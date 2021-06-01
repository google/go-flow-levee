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

// For a function, transitively get the functions called within this function.
// Argument "depth" controls the depth of the call chain.
// For example, return {g1,g2,g3} for "func f(){ g1(); g2() }, func g1(){ g3() }".
func GetCalleeFunctions(fn *ssa.Function, result map[*ssa.Function]bool, depth int) {
	if depth <= 0 {
		return
	}
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			switch v := instr.(type) {
			case *ssa.Call:
				// TODO: the call graph can be CHA, RTA, VTA, etc.
				if callee := v.Call.StaticCallee(); callee != nil &&
					len(callee.Blocks) > 0 {
					if _, ok := result[callee]; !ok {
						result[callee] = true
						GetCalleeFunctions(callee, result, depth-1)
					}
				}
			}
		}
	}
}

func isWithinFunctions(ref Reference, callees map[*ssa.Function]bool) bool {
	fn := ref.Value().Parent()
	if fn != nil {
		_, ok := callees[fn]
		return ok
	}
	return true
}

// Obtain the references associated with taint sources, with field sensitivity.
// When a struct object contains taint fields, these fields and all their
// subfields are included in taint sources, but the struct object itself is
// not a taint source. However, if this object has no fields, then this object
// is considered as a taint source.
//
// For example, consider object of type "struct {x: *T, y *T}" where x is a
// taint field, and this object's heap is "{t0,t1}: [x:int->t2, y:int->t3],
// {t2, t4} --> t5", then {t2, t4, t5} are taint sources.
func GetSrcRefs(src *source.Source, state *Partitions, isTaintField func(named *types.Named, index int) bool,
	callees map[*ssa.Function]bool) ReferenceSet {
	refs := make(ReferenceSet)
	val, ok := src.Node.(ssa.Value)
	if !ok {
		return nil
	}
	rep := state.Representative(MakeLocalWithEmptyContext(val))
	var getStructFieldRefs func(ref Reference, tp types.Type)
	getStructFieldRefs = func(ref Reference, tp types.Type) {
		switch tp := tp.(type) {
		case *types.Named:
			if tt, ok := tp.Underlying().(*types.Struct); ok {
				// Look for the taint fields.
				hasTaintField := false
				for i := 0; i < tt.NumFields(); i++ {
					f := tt.Field(i)
					if isTaintField(tp, i) {
						for fd, fref := range state.PartitionFieldMap(ref) {
							if fd.Name == f.Name() {
								refs[fref] = true
								// Mark all the subfields to be tainted.
								getFieldReferences(fref, refs, state, callees, make(ReferenceSet))
								hasTaintField = true
							}
						}
					}
				}
				if !hasTaintField {
					getFieldReferences(ref, refs, state, callees, make(ReferenceSet))
				}
			}
		case *types.Pointer:
			if r := state.PartitionFieldMap(rep)[directPointToField]; r != nil {
				getStructFieldRefs(r, tp.Elem())
			} else {
				getStructFieldRefs(rep, tp.Elem())
			}
		case *types.Array:
			for _, r := range state.PartitionFieldMap(rep) {
				getStructFieldRefs(r, tp.Elem())
			}
		case *types.Slice:
			for _, r := range state.PartitionFieldMap(rep) {
				getStructFieldRefs(r, tp.Elem())
			}
		case *types.Chan:
			for _, r := range state.PartitionFieldMap(rep) {
				getStructFieldRefs(r, tp.Elem())
			}
		case *types.Map:
			for _, r := range state.PartitionFieldMap(rep) {
				getStructFieldRefs(r, tp.Elem())
			}
		case *types.Basic, *types.Tuple, *types.Interface, *types.Signature:
			// These types do not currently represent possible source types
		}
	}

	getStructFieldRefs(rep, val.Type())
	return refs
}

// Obtains all the field references and their aliases for "ref".
// For example, return {t0, t5, t1, t3, t4} for "{t0,t5}: [0:int->t1, 1:int->t3], {t3} --> t4".
func getFieldReferences(ref Reference, result ReferenceSet, state *Partitions, callees map[*ssa.Function]bool, visited ReferenceSet) {
	visited[ref] = true
	for _, m := range state.PartitionMembers(ref) {
		if isWithinFunctions(m, callees) {
			result[m] = true
		}
	}
	rep := state.Representative(ref)
	result[rep] = true
	for _, r := range state.PartitionFieldMap(rep) {
		if _, ok := visited[r]; !ok {
			getFieldReferences(r, result, state, callees, visited)
		}
	}
}

// Checks whether a taint sink can be reached by a taint source by examining the alias information.
func IsEARTainted(sink ssa.Instruction, srcRefs ReferenceSet, state *Partitions, callees map[*ssa.Function]bool) bool {
	sinks := make(map[Reference]bool)
	for _, op := range sink.Operands(nil) {
			v := *op
			if isLocal(v) || isGlobal(v) {
				ref := MakeLocalWithEmptyContext(v)
				getFieldReferences(ref, sinks, state, callees, make(ReferenceSet))
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
