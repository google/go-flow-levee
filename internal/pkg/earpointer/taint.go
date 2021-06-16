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

type heapTraversalState struct {
	partitions *Partitions
	callees    map[*ssa.Function]bool
	visited    ReferenceSet
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
			switch v := instr.(type) {
			case *ssa.Call:
				// TODO: the call graph can be CHA, RTA, VTA, etc.
				if callee := v.Call.StaticCallee(); callee != nil && len(callee.Blocks) > 0 {
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

func isWithinFunctions(ref Reference, callees map[*ssa.Function]bool) bool {
	if fn := ref.Value().Parent(); fn != nil {
		_, ok := callees[fn]
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
	tState := &heapTraversalState{partitions: state, callees: callees, visited: make(ReferenceSet)}
	getStructFieldRefs(rep, val.Type(), refs, isTaintField, tState)
	return refs
}

// Obtain the references associated with a taint source, with field sensitivity.
// When a struct object contains taint fields, only these fields and all their
// subfields are included in taint sources, and the struct object itself is
// a taint source. Other fields will not be tainted.
//
// For example, consider object of type "struct {x: *T, y *T}" where x is a
// taint field, and this object's heap is "{t0,t1}: [x->t2, y->t3],
// {t2,t4} --> t5, {t3} --> t6", then {t0,t1,t2,t4,t5} are taint sources.
func getStructFieldRefs(rep Reference, tp types.Type, refs ReferenceSet,
	isTaintField func(named *types.Named, index int) bool, tState *heapTraversalState) {

	state := tState.partitions
	switch tp := tp.(type) {
	case *types.Named:
		if tt, ok := tp.Underlying().(*types.Struct); ok {
			refs[rep] = true // the current struct object is tainted
			// Look for the taint fields.
			for i := 0; i < tt.NumFields(); i++ {
				f := tt.Field(i)
				if isTaintField(tp, i) {
					for fd, fref := range state.PartitionFieldMap(rep) {
						if fd.Name == f.Name() {
							refs[fref] = true
							// Mark all the subfields to be tainted.
							getFieldReferences(fref, refs, tState)
						}
					}
				}
				// Skip the non-taint fields.
			}
		} else {
			getStructFieldRefs(rep, tp.Underlying(), refs, isTaintField, tState)
		}
	case *types.Pointer:
		if r := state.PartitionFieldMap(rep)[directPointToField]; r != nil {
			getStructFieldRefs(r, tp.Elem(), refs, isTaintField, tState)
		} else {
			getStructFieldRefs(rep, tp.Elem(), refs, isTaintField, tState)
		}
	case *types.Array:
		refs[rep] = true
		for _, r := range state.PartitionFieldMap(rep) {
			getStructFieldRefs(r, tp.Elem(), refs, isTaintField, tState)
		}
	case *types.Slice:
		refs[rep] = true
		for _, r := range state.PartitionFieldMap(rep) {
			getStructFieldRefs(r, tp.Elem(), refs, isTaintField, tState)
		}
	case *types.Chan:
		refs[rep] = true
		for _, r := range state.PartitionFieldMap(rep) {
			getStructFieldRefs(r, tp.Elem(), refs, isTaintField, tState)
		}
	case *types.Map:
		refs[rep] = true
		for _, r := range state.PartitionFieldMap(rep) {
			getStructFieldRefs(r, tp.Elem(), refs, isTaintField, tState)
		}
	case *types.Basic, *types.Tuple, *types.Interface, *types.Signature:
		// These types do not currently represent possible source types
	}
}

// Obtains all the field references and their aliases for "ref".
// For example, return {t0, t5, t1, t3, t4} for "{t0,t5}: [0:int->t1, 1:int->t3], {t3} --> t4".
func getFieldReferences(ref Reference, result ReferenceSet, traversalState *heapTraversalState) {
	traversalState.visited[ref] = true
	state := traversalState.partitions
	for _, m := range state.PartitionMembers(ref) {
		if isWithinFunctions(m, traversalState.callees) {
			result[m] = true
		}
	}
	rep := state.Representative(ref)
	for _, r := range state.PartitionFieldMap(rep) {
		if _, ok := traversalState.visited[r]; !ok {
			getFieldReferences(r, result, traversalState)
		}
	}
}

// Checks whether a taint sink can be reached by a taint source by examining the alias information.
func IsEARTainted(sink ssa.Instruction, srcRefs ReferenceSet, state *Partitions, callees map[*ssa.Function]bool) bool {
	traversalState := &heapTraversalState{partitions: state, callees: callees, visited: make(ReferenceSet)}
	sinks := make(map[Reference]bool)
	for _, op := range sink.Operands(nil) {
		v := *op
		if isLocal(v) || isGlobal(v) {
			ref := MakeLocalWithEmptyContext(v)
			getFieldReferences(ref, sinks, traversalState)
		}
	}

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
