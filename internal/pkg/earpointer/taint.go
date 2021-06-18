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

	"github.com/google/go-flow-levee/internal/pkg/source"
	"golang.org/x/tools/go/ssa"
)

// Bounded traversal of an EAR heap.
type heapTraversal struct {
	partitions   *Partitions
	callees      map[*ssa.Function]bool // the functions containing the references of interest
	visited      ReferenceSet           // the visited references during the traversal
	isTaintField func(named *types.Named, index int) bool
}

func (ht *heapTraversal) isWithinCallees(ref Reference) bool {
	if fn := ref.Value().Parent(); fn != nil {
		_, ok := ht.callees[fn]
		return ok
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
	state := ht.partitions
	switch tp := tp.(type) {
	case *types.Named:
		// Consider object of type "struct {x: *T, y *T}" where x is a
		// taint field, and this object's heap is "{t0,t1}: [x->t2, y->t3],
		// {t2,t4} --> t5, {t3} --> t6", then {t0,t1,t2,t4,t5} are taint sources.
		if tt, ok := tp.Underlying().(*types.Struct); ok {
			result[rep] = true // the current struct object is tainted
			// Look for the taint fields.
			for i := 0; i < tt.NumFields(); i++ {
				f := tt.Field(i)
				if ht.isTaintField(tp, i) {
					for fd, fref := range state.PartitionFieldMap(rep) {
						if fd.Name == f.Name() {
							result[fref] = true
							// Mark all the subfields to be tainted.
							ht.fieldRefs(fref, result)
						}
					}
				}
				// Skip the non-taint fields.
			}
		} else {
			ht.srcRefs(rep, tp.Underlying(), result)
		}
	case *types.Pointer:
		if r := state.PartitionFieldMap(rep)[directPointToField]; r != nil {
			ht.srcRefs(r, tp.Elem(), result)
		} else {
			ht.srcRefs(rep, tp.Elem(), result)
		}
	case *types.Array:
		result[rep] = true
		for _, r := range state.PartitionFieldMap(rep) {
			ht.srcRefs(r, tp.Elem(), result)
		}
	case *types.Slice:
		result[rep] = true
		for _, r := range state.PartitionFieldMap(rep) {
			ht.srcRefs(r, tp.Elem(), result)
		}
	case *types.Chan:
		result[rep] = true
		for _, r := range state.PartitionFieldMap(rep) {
			ht.srcRefs(r, tp.Elem(), result)
		}
	case *types.Map:
		result[rep] = true
		for _, r := range state.PartitionFieldMap(rep) {
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
	p := ht.partitions
	for _, m := range p.PartitionMembers(ref) {
		if ht.isWithinCallees(m) {
			result[m] = true
		}
	}
	rep := p.Representative(ref)
	for _, r := range p.PartitionFieldMap(rep) {
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
				if callee := call.Call.StaticCallee(); callee != nil {
					if len(callee.Blocks) > 0 { // skip empty or unlinked functions
						if _, ok := result[callee]; !ok {
							result[callee] = true
							calleeFunctions(callee, result, depth-1)
						}
					}
				}
			}
		}
	}
}

func BoundedDepthCallees(fn *ssa.Function, depth uint) map[*ssa.Function]bool {
	result := make(map[*ssa.Function]bool)
	result[fn] = true
	calleeFunctions(fn, result, depth)
	return result
}

// Obtain the references associated with a taint source, with field sensitivity.
// Consider only the references located with the "callees".
func SrcRefs(src *source.Source, isTaintField func(named *types.Named, index int) bool,
	state *Partitions, callees map[*ssa.Function]bool) ReferenceSet {

	val, ok := src.Node.(ssa.Value)
	if !ok {
		return nil
	}
	rep := state.Representative(MakeLocalWithEmptyContext(val))
	refs := make(ReferenceSet)
	ht := &heapTraversal{partitions: state, callees: callees, visited: make(ReferenceSet), isTaintField: isTaintField}
	ht.srcRefs(rep, val.Type(), refs)
	return refs
}

// Checks whether a taint sink can be reached by a taint source by
// examining the heap alias information. "callees" are used to bound the heap traversal.
func IsEARTainted(sink ssa.Instruction, srcRefs ReferenceSet, state *Partitions, callees map[*ssa.Function]bool) bool {
	ht := &heapTraversal{partitions: state, callees: callees, visited: make(ReferenceSet)}
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
		members := state.PartitionMembers(sink)
		for _, m := range members {
			if _, ok := srcRefs[m]; ok {
				return true
			}
		}
	}
	return false
}
