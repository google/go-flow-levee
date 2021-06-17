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

// A data structure to assist the traversal of an EAR heap.
type heapTraversal struct {
	partitions *Partitions
	callees    map[*ssa.Function]bool // the functions containing the references of interest
	visited    ReferenceSet           // the visited references during the traversal
}

// For a function, transitively get the functions called within this function.
// Argument "depth" controls the depth of the call chain.
// For example, return {g1,g2,g3} for "func f(){ g1(); g2() }, func g1(){ g3() }".
func getCalleeFunctions(fn *ssa.Function, result map[*ssa.Function]bool, depth uint) {
	if depth <= 0 {
		return
	}
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			if call, ok := instr.(*ssa.Call); ok {
				// TODO: the call graph can be CHA, RTA, VTA, etc.
				if callee := call.Call.StaticCallee(); callee != nil && len(callee.Blocks) > 0 {
					if _, ok := result[callee]; !ok {
						result[callee] = true
						getCalleeFunctions(callee, result, depth-1)
					}
				}
			}
		}
	}
}

func GetCalleeFunctions(fn *ssa.Function, depth uint) map[*ssa.Function]bool {
	result := make(map[*ssa.Function]bool)
	result[fn] = true
	getCalleeFunctions(fn, result, depth)
	return result
}

func (ht *heapTraversal) isWithinFunctions(ref Reference) bool {
	if fn := ref.Value().Parent(); fn != nil {
		_, ok := ht.callees[fn]
		return ok
	}
	// Globals and Builtins have no parents.
	return true
}

// Obtain the references associated with a taint source, with field sensitivity.
// Consider only the references located with the "callees".
func GetSrcRefs(src *source.Source, isTaintField func(named *types.Named, index int) bool,
	state *Partitions, callees map[*ssa.Function]bool) ReferenceSet {

	val, ok := src.Node.(ssa.Value)
	if !ok {
		return nil
	}
	rep := state.Representative(MakeLocalWithEmptyContext(val))
	refs := make(ReferenceSet)
	ht := &heapTraversal{partitions: state, callees: callees, visited: make(ReferenceSet)}
	ht.getStructFieldRefs(rep, val.Type(), refs, isTaintField)
	return refs
}

// Obtain the references associated with a taint source, with field sensitivity.
// Composite types are examined recursively to identify the taint elements, e.g.
// (1) when a map contains taint elements, these elements are examined to identify
//     taint sources; and
// (2) when a struct object contains taint fields, only these fields
// 	   and all their subfields are included in taint sources, and the struct object
//     itself is a taint source. Other fields will not be tainted.
func (ht *heapTraversal) getStructFieldRefs(rep Reference, tp types.Type, result ReferenceSet, isTaintField func(named *types.Named, index int) bool) {
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
				if isTaintField(tp, i) {
					for fd, fref := range state.PartitionFieldMap(rep) {
						if fd.Name == f.Name() {
							result[fref] = true
							// Mark all the subfields to be tainted.
							ht.getFieldReferences(fref, result)
						}
					}
				}
				// Skip the non-taint fields.
			}
		} else {
			ht.getStructFieldRefs(rep, tp.Underlying(), result, isTaintField)
		}
	case *types.Pointer:
		if r := state.PartitionFieldMap(rep)[directPointToField]; r != nil {
			ht.getStructFieldRefs(r, tp.Elem(), result, isTaintField)
		} else {
			ht.getStructFieldRefs(rep, tp.Elem(), result, isTaintField)
		}
	case *types.Array:
		result[rep] = true
		for _, r := range state.PartitionFieldMap(rep) {
			ht.getStructFieldRefs(r, tp.Elem(), result, isTaintField)
		}
	case *types.Slice:
		result[rep] = true
		for _, r := range state.PartitionFieldMap(rep) {
			ht.getStructFieldRefs(r, tp.Elem(), result, isTaintField)
		}
	case *types.Chan:
		result[rep] = true
		for _, r := range state.PartitionFieldMap(rep) {
			ht.getStructFieldRefs(r, tp.Elem(), result, isTaintField)
		}
	case *types.Map:
		result[rep] = true
		for _, r := range state.PartitionFieldMap(rep) {
			ht.getStructFieldRefs(r, tp.Elem(), result, isTaintField)
		}
	case *types.Basic, *types.Tuple, *types.Interface, *types.Signature:
		// These types do not currently represent possible source types
	}
}

// Obtains all the field references and their aliases for "ref".
// For example, return {t0,t5,t1,t3,t4} for "{t0,t5}: [0->t1, 1->t3], {t3} --> t4".
func (ht *heapTraversal) getFieldReferences(ref Reference, result ReferenceSet) {
	ht.visited[ref] = true
	p := ht.partitions
	for _, m := range p.PartitionMembers(ref) {
		if ht.isWithinFunctions(m) {
			result[m] = true
		}
	}
	rep := p.Representative(ref)
	for _, r := range p.PartitionFieldMap(rep) {
		if _, ok := ht.visited[r]; !ok {
			ht.getFieldReferences(r, result)
		}
	}
}

// Checks whether a taint sink can be reached by a taint source by examining the alias information.
func IsEARTainted(sink ssa.Instruction, srcRefs ReferenceSet, state *Partitions, callees map[*ssa.Function]bool) bool {
	ht := &heapTraversal{partitions: state, callees: callees, visited: make(ReferenceSet)}
	sinks := make(map[Reference]bool)
	// All sub-fields of a sink object are considered.
	// For example, for heap "{t0}: [0->t1(taint), 1->t2]", return true for
	// sink call "sinkf(t0)" since t0 contains a taint field t1.
	for _, op := range sink.Operands(nil) {
		v := *op
		if isLocal(v) || isGlobal(v) {
			ref := MakeLocalWithEmptyContext(v)
			ht.getFieldReferences(ref, sinks)
		}
	}
	// Match each sink with any possible source.
	for sink := range sinks {
		members := state.PartitionMembers(sink)
		for _, m := range members {
			if _, ok := srcRefs[m]; ok {
				return true
			}
		}
	}
	return false
}
