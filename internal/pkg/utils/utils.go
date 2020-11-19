// Copyright 2020 Google LLC
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

// Package utils contains various utility functions.
package utils

import (
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// Dereference returns the underlying type of a pointer.
// If the input is not a pointer, then the type of the input is returned.
func Dereference(t types.Type) types.Type {
	for {
		tt, ok := t.Underlying().(*types.Pointer)
		if !ok {
			return t
		}
		t = tt.Elem()
	}
}

// FieldName returns the name of the field identified by the FieldAddr.
// It is the responsibility of the caller to ensure that the returned value is a non-empty string.
func FieldName(fa *ssa.FieldAddr) string {
	// fa.Type() refers to the accessed field's type.
	// fa.X.Type() refers to the surrounding struct's type.
	d := Dereference(fa.X.Type())
	st, ok := d.Underlying().(*types.Struct)
	if !ok {
		return ""
	}
	return st.Field(fa.Field).Name()
}

// DecomposeType returns the path and name of a Named type
// Returns empty strings if the type is not *types.Named
func DecomposeType(t types.Type) (path, name string) {
	n, ok := t.(*types.Named)
	if !ok {
		return
	}

	if pkg := n.Obj().Pkg(); pkg != nil {
		path = pkg.Path()
	}

	return path, n.Obj().Name()
}

func UnqualifiedName(v *types.Var) string {
	packageQualifiedName := v.Type().String()
	dotPos := strings.LastIndexByte(packageQualifiedName, '.')
	if dotPos == -1 {
		return packageQualifiedName
	}
	return packageQualifiedName[dotPos+1:]
}

// DecomposeFunction returns the path, receiver, and name strings of a ssa.Function.
// For functions that have no receiver, returns an empty string for recv.
// For shared functions (wrappers and error.Error), returns an empty string for path.
// Panics if provided a nil argument.
func DecomposeFunction(f *ssa.Function) (path, recv, name string) {
	if f.Pkg != nil {
		path = f.Pkg.Pkg.Path()
	}
	name = f.Name()
	if recvVar := f.Signature.Recv(); recvVar != nil {
		recv = UnqualifiedName(recvVar)
	}
	return
}
