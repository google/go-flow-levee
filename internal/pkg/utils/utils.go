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
