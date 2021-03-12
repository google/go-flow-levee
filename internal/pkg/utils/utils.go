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

// DecomposeField returns the decomposed type of the
// struct containing the field, as well as the field's name.
// If the referenced struct's type is not a named type,
// the type path and name will both be empty strings.
func DecomposeField(t types.Type, field int) (typePath, typeName, fieldName string) {
	deref := Dereference(t)
	typePath, typeName = DecomposeType(deref)
	fieldName = deref.Underlying().(*types.Struct).Field(field).Name()
	return
}

func UnqualifiedName(t types.Type) string {
	packageQualifiedName := t.String()
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
		recv = UnqualifiedName(recvVar.Type())
	}
	return
}
