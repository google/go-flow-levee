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

	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/utils"

	"github.com/google/go-flow-levee/internal/pkg/source"
	"golang.org/x/tools/go/ssa"
)

// Bounded traversal of an EAR heap.
type heapTraversal struct {
	heap         *Partitions
	callees      map[*ssa.Function]bool // the functions containing the references of interest
	visited      ReferenceSet           // the visited references during the traversal
	isTaintField func(named *types.Named, index int) bool
}

func (ht *heapTraversal) isWithinCallees(ref Reference) bool {
	if fn := ref.Value().Parent(); fn != nil {
		return ht.callees[fn]
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
		result[rep] = true // the current struct object is tainted
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
			// Skip the non-taint fields.
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

// For a function, transitively get the functions called within this function.
// Argument "depth" controls the depth of the call chain.
// For example, return {g1,g2,g3} for "func f(){ g1(); g2() }, func g1(){ g3() }".
func calleeFunctions(fn *ssa.Function, result map[*ssa.Function]bool, depth uint) {
	if depth <= 0 {
		return
	}
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			if call, ok := instr.(*ssa.Call); ok {
				// TODO(#317): use more advanced call graph.
				// skip empty, unlinked, or visited functions
				if callee := call.Call.StaticCallee(); callee != nil && len(callee.Blocks) > 0 && !result[callee] {
					result[callee] = true
					calleeFunctions(callee, result, depth-1)
				}
			}
		}
	}
}

func boundedDepthCallees(fn *ssa.Function, depth uint) map[*ssa.Function]bool {
	result := make(map[*ssa.Function]bool)
	result[fn] = true
	calleeFunctions(fn, result, depth)
	return result
}

// Obtain the references which are aliases of a taint source, with field sensitivity.
// Argument "heap" is an immutable EAR heap containing alias information;
// "callees" is used to bound the searching of source references in the heap.
func srcAliasRefs(src *source.Source, isTaintField func(named *types.Named, index int) bool,
	heap *Partitions, callees map[*ssa.Function]bool) ReferenceSet {

	val, ok := src.Node.(ssa.Value)
	if !ok {
		return nil
	}
	rep := heap.Representative(MakeLocalWithEmptyContext(val))
	refs := make(ReferenceSet)
	ht := &heapTraversal{heap: heap, callees: callees, visited: make(ReferenceSet), isTaintField: isTaintField}
	ht.srcRefs(rep, val.Type(), refs)
	return refs
}

// Check whether a taint sink can be reached by a taint source reference by
// examining the heap alias information.
// Argument "heap" is an immutable EAR state same as the one in "srcAliasRefs()";
// "callees" is used to bound the searching of sink references in the heap.
func canReach(sink ssa.Instruction, srcRefs ReferenceSet, heap *Partitions, callees map[*ssa.Function]bool) bool {
	ht := &heapTraversal{heap: heap, callees: callees, visited: make(ReferenceSet)}
	sinkedRefs := make(map[Reference]bool)
	// All sub-fields of a sink object are considered.
	// For example, for heap "{t0}: [0->t1(taint), 1->t2]", return true for
	// sink call "sinkf(t0)" since t0 contains a taint field t1.
	for _, op := range sink.Operands(nil) {
		v := *op
		if isLocal(v) || isGlobal(v) {
			ref := MakeLocalWithEmptyContext(v)
			ht.fieldRefs(ref, sinkedRefs)
		}
	}
	// Match each sink with any possible source.
	for sink := range sinkedRefs {
		members := heap.PartitionMembers(sink)
		for _, m := range members {
			if srcRefs[m] {
				return true
			}
		}
	}
	return false
}

type SourceSinkTrace struct {
	Src       *source.Source
	Sink      ssa.Instruction
	Callstack []ssa.Call
}

// Look for the pairs of <source, sink> examining the heap alias information.
func SourcesToSinks(funcSources source.ResultType, isTaintField func(named *types.Named, index int) bool,
	heap *Partitions, conf *config.Config) []*SourceSinkTrace {

	var traces []*SourceSinkTrace
	for fn, sources := range funcSources {
		// Transitively get the set of functions called within "fn".
		// This set is used to narrow down the set of references needed to be
		// considered during EAR heap traversal. It can also help reducing the
		// false positives and boosting the performance.
		callees := boundedDepthCallees(fn, conf.EARTaintCallSpan)
		srcRefs := make(map[*source.Source]ReferenceSet)
		for _, s := range sources {
			srcRefs[s] = srcAliasRefs(s, isTaintField, heap, callees)
		}
		// Return a source if it can reach the given sink.
		reachAnySource := func(sink ssa.Instruction) *source.Source {
			for _, src := range sources {
				if canReach(sink, srcRefs[src], heap, callees) {
					return src
				}
			}
			return nil
		}
		// Traverse all the callee functions (not just the ones with sink sources)
		for member := range callees {
			for _, b := range member.Blocks {
				for _, instr := range b.Instrs {
					switch v := instr.(type) {
					case *ssa.Call:
						sink := instr
						// TODO(#317): use more advanced call graph.
						callee := v.Call.StaticCallee()
						if callee != nil && conf.IsSink(utils.DecomposeFunction(callee)) {
							if src := reachAnySource(instr); src != nil {
								traces = append(traces, &SourceSinkTrace{Src: src, Sink: sink})
								break
							}
						}
					case *ssa.Panic:
						if conf.AllowPanicOnTaintedValues {
							continue
						}
						sink := instr
						if src := reachAnySource(sink); src != nil {
							traces = append(traces, &SourceSinkTrace{Src: src, Sink: sink})
							break
						}

					}
				}
			}
		}
	}
	return traces
}
