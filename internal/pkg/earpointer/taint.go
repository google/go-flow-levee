// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package earpointer

import (
	"go/types"

	"golang.org/x/tools/go/callgraph"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/utils"

	"github.com/google/go-flow-levee/internal/pkg/source"
	"golang.org/x/tools/go/ssa"
)

// Bounded traversal of an EAR heap.
type heapTraversal struct {
	heap *Partitions
	// The reachable functions containing the references of interest.
	reachableFns map[*ssa.Function]bool
	// The visited references during the traversal.
	visited      ReferenceSet
	isTaintField func(named *types.Named, index int) bool
}

func (ht *heapTraversal) isWithinCallees(ref Reference) bool {
	if fn := ref.Value().Parent(); fn != nil {
		return ht.reachableFns[fn]
	}
	// Globals and Builtins have no parents.
	return true
}

// Obtain the references associated with a taint source, with field sensitivity.
// Composite types are examined recursively to identify the taint elements, e.g.
// (1) when a map contains taint elements, these elements are examined to identify
//    taint sources; and
// (2) when a struct object contains taint fields, only these fields
// 	  and all their subfields are included in taint sources, and the struct object
//    itself is a taint source. Other fields will not be tainted.
func (ht *heapTraversal) srcRefs(rep Reference, tp types.Type, result ReferenceSet) {
	heap := ht.heap
	switch tp := tp.(type) {
	case *types.Named:
		// Consider object of type "struct {x: *T, y *T}" where x is a
		// taint field, and this object's heap is "{t0,t1}: [x->t2, y->t3],
		// {t2,t4} --> t5, {t3} --> t6", then {t0,t1,t2,t4,t5} are taint sources.
		tt, ok := tp.Underlying().(*types.Struct)
		if !ok {
			ht.srcRefs(rep, tp.Underlying(), result)
			return
		}
		// Mark the current struct object as tainted.
		result[rep] = true
		// Look for the taint fields.
		for i := 0; i < tt.NumFields(); i++ {
			f := tt.Field(i)
			if ht.isTaintField(tp, i) {
				for fd, fref := range heap.PartitionFieldMap(rep) {
					if fd.Name == f.Name() {
						result[fref] = true
						// Mark all the subfields to be tainted.
						ht.fieldRefs(fref, result)
					}
				}
			}
		}
	case *types.Pointer:
		if r := heap.PartitionFieldMap(rep)[directPointToField]; r != nil {
			ht.srcRefs(r, tp.Elem(), result)
		} else {
			ht.srcRefs(rep, tp.Elem(), result)
		}
	case *types.Array:
		result[rep] = true
		for _, r := range heap.PartitionFieldMap(rep) {
			ht.srcRefs(r, tp.Elem(), result)
		}
	case *types.Slice:
		result[rep] = true
		for _, r := range heap.PartitionFieldMap(rep) {
			ht.srcRefs(r, tp.Elem(), result)
		}
	case *types.Chan:
		result[rep] = true
		for _, r := range heap.PartitionFieldMap(rep) {
			ht.srcRefs(r, tp.Elem(), result)
		}
	case *types.Map:
		result[rep] = true
		for _, r := range heap.PartitionFieldMap(rep) {
			ht.srcRefs(r, tp.Elem(), result)
		}
	case *types.Basic, *types.Tuple, *types.Interface, *types.Signature:
		// These types do not currently represent possible source types
	}
}

// Obtains all the field references and their aliases for "ref".
// For example, return {t0,t5,t1,t3,t4} for "{t0,t5}: [0->t1, 1->t3], {t3} --> t4".
func (ht *heapTraversal) fieldRefs(ref Reference, result ReferenceSet) {
	ht.visited[ref] = true
	h := ht.heap
	for _, m := range h.PartitionMembers(ref) {
		if ht.isWithinCallees(m) {
			result[m] = true
		}
	}
	rep := h.Representative(ref)
	for _, r := range h.PartitionFieldMap(rep) {
		if _, ok := ht.visited[r]; !ok {
			ht.fieldRefs(r, result)
		}
	}
}

// Return any of the sources if it can reach the taint; otherwise return nil.
// Argument "srcRefs" maps a source to its alias references.
func (ht *heapTraversal) canReach(sink ssa.Instruction, sources []*source.Source, srcRefs map[*source.Source]ReferenceSet) *source.Source {
	// Obtain the alias references of a sink.
	// All sub-fields of a sink object are considered.
	// For example, for heap "{t0}: [0->t1(taint), 1->t2]", return true for
	// sink call "sinkf(t0)" since t0 contains a taint field t1.
	sinkedRefs := make(map[Reference]bool)
	for _, op := range sink.Operands(nil) {
		// Use a separate heapTraversal to search for the sink references.
		sinkHT := &heapTraversal{heap: ht.heap, reachableFns: ht.reachableFns, visited: make(ReferenceSet)}
		v := *op
		if isLocal(v) || isGlobal(v) {
			ref := MakeLocalWithEmptyContext(v)
			sinkHT.fieldRefs(ref, sinkedRefs)
		}
	}
	// Match each sink with any possible source.
	for sink := range sinkedRefs {
		members := ht.heap.PartitionMembers(sink)
		for _, m := range members {
			for _, src := range sources {
				if srcRefs[src][m] {
					return src
				}
			}
		}
	}
	return nil
}

// For a function, transitively get the functions reachable from this function
// according to the call graph. Both callers and callees are considered.
// Argument "depth" controls the depth of the call chain, and  "result" is
// to store the set of reachable functions. For example,
//   func f(){ g1(); g2() }
//   func g1(){ g3() }
// for input "g1", result = {f,g1,g2,g3} if depth>1, and result = {f, g1} if depth=1.
func boundedReachableFunctions(fn *ssa.Function, cg *callgraph.Graph, depth uint, result map[*ssa.Function]bool) {
	if depth <= 0 || result[fn] {
		return
	}
	result[fn] = true
	node := cg.Nodes[fn]
	if node == nil {
		return
	}
	// Visit the callees within "fn".
	for _, out := range node.Out {
		boundedReachableFunctions(out.Callee.Func, cg, depth-1, result)
	}
	// Visit the callers of "fn".
	for _, in := range node.In {
		boundedReachableFunctions(in.Caller.Func, cg, depth-1, result)
	}
}

// Obtain the references which are aliases of a taint source, with field sensitivity.
// Argument "heap" is an immutable EAR heap containing alias information;
// "reachable" is used to bound the searching of source references in the heap.
func srcAliasRefs(src *source.Source, isTaintField func(named *types.Named, index int) bool,
	heap *Partitions, reachable map[*ssa.Function]bool) ReferenceSet {

	val, ok := src.Node.(ssa.Value)
	if !ok {
		return nil
	}
	rep := heap.Representative(MakeLocalWithEmptyContext(val))
	refs := make(ReferenceSet)
	ht := &heapTraversal{heap: heap, reachableFns: reachable, visited: make(ReferenceSet), isTaintField: isTaintField}
	ht.srcRefs(rep, val.Type(), refs)
	return refs
}

type SourceSinkTrace struct {
	Src       *source.Source
	Sink      ssa.Instruction
	Callstack []ssa.Call
}

// Look for <source, sink> pairs by examining the heap alias information.
func SourcesToSinks(funcSources source.ResultType, isTaintField func(named *types.Named, index int) bool,
	heap *Partitions, conf *config.Config) map[ssa.Instruction]*SourceSinkTrace {

	// A map from a callsite to its possible callees.
	calleeMap := mapCallees(heap.cg)
	traces := make(map[ssa.Instruction]*SourceSinkTrace)
	for fn, sources := range funcSources {
		// Transitively get the set of functions reachable from "fn".
		// This set is used to narrow down the set of references needed to be
		// considered during EAR heap traversal. It can also help reducing the
		// false positives and boosting the performance.
		// For example,
		//   func f1(){ f2(); g4() }
		//   func f2(){ g1(); g2() }
		//   func g1(){ g3() }
		// g1 can reach f2 through the caller, and then f1 similarly,
		// and then g4 through the callee.
		// g1's full reachable set is {f1,f2,g1,g2,g3,g4}.
		reachable := make(map[*ssa.Function]bool)
		boundedReachableFunctions(fn, heap.cg, conf.EARTaintCallSpan, reachable)
		// Start from the set of taint sources.
		srcRefs := make(map[*source.Source]ReferenceSet)
		for _, s := range sources {
			srcRefs[s] = srcAliasRefs(s, isTaintField, heap, reachable)
		}
		// Traverse all the reachable functions (not just the ones with sink sources)
		// in search for connected sinks.
		ht := &heapTraversal{heap: heap, reachableFns: reachable, visited: make(ReferenceSet)}
		for member := range reachable {
			for _, b := range member.Blocks {
				for _, instr := range b.Instrs {
					switch v := instr.(type) {
					case *ssa.Call:
						callees := calleeMap[&v.Call]
						sink := instr
						for _, callee := range callees {
							if conf.IsSink(utils.DecomposeFunction(callee)) {
								if src := ht.canReach(sink, sources, srcRefs); src != nil {
									// If a previous source has been found, be in favor of the source within the same
									// function. This can be extended to be in favor of the source closest to the sink.
									if _, ok := traces[instr]; !ok || src.Node.Parent() == sink.Parent() {
										traces[sink] = &SourceSinkTrace{Src: src, Sink: sink}
									}
								}
							}
						}
					case *ssa.Panic:
						if conf.AllowPanicOnTaintedValues {
							continue
						}
						sink := instr
						if src := ht.canReach(sink, sources, srcRefs); src != nil {
							traces[sink] = &SourceSinkTrace{Src: src, Sink: sink}
						}

					}
				}
			}
		}
	}
	return traces
}
