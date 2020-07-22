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

// Package varargs contains functionality for dealing with variable length arguments.
package varargs

import (
	"github.com/google/go-flow-levee/internal/pkg/call"
	"golang.org/x/tools/go/ssa"
)

// Varargs represents a variable length argument.
// Concretely, it abstract over the fact that the varargs internally are represented by an ssa.Slice
// which contains the underlying values of for vararg members.
// Since many sink functions (ex. log.Info, fmt.Errorf) take a vararg argument, being able to
// get the underlying values of the vararg members is important for the analysis.
type Varargs struct {
	stores []*ssa.Store
}

// New constructs varargs. SSA represents varargs as an ssa.Slice.
func New(s *ssa.Call) *Varargs {
	if !s.Call.Signature().Variadic() {
		return nil
	}
	if len(s.Call.Args) == 0 {
		return nil
	}

	lastArg := s.Call.Args[len(s.Call.Args)-1]
	sl, ok := lastArg.(*ssa.Slice)
	if !ok {
		return nil
	}

	a, ok := sl.X.(*ssa.Alloc)
	if !ok || (a.Comment != "varargs" && a.Comment != "slicelit") {
		return nil
	}

	var stores []*ssa.Store

	for _, r := range *a.Referrers() {
		idx, ok := r.(*ssa.IndexAddr)
		if !ok || idx.Referrers() == nil {
			continue
		}

		// IndexAddr and Store instructions are inherently linked together.
		// IndexAddr returns an address of an element within a Slice, which is followed by
		// a Store instructions to place a value into the address provided by IndexAddr.
		stores = append(stores, (*idx.Referrers())[0].(*ssa.Store))
	}

	return &Varargs{
		stores: stores,
	}
}

// ReferredBy determines if the supplied node refers the Vararg in question via a store instruction.
func (v *Varargs) ReferredBy(r call.Referrer) bool {
	for _, s := range v.stores {
		if r.RefersTo(s.Val.(ssa.Node)) {
			return true
		}
	}

	return false
}
