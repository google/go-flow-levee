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

// Package call contains functionality for determining whether a function
// call refers to a given referrer.
package call

import (
	"golang.org/x/tools/go/ssa"
)

// Call is a wrapper for *ssa.Call instructions, allowing Variadic and
// non-Variadic calls to be handled polymorphically
type Call interface {
	// ReferredBy determines if the call is referred to by the referrer
	ReferredBy(r Referrer) bool
}

type Referrer interface {
	HasPathTo(node ssa.Node) bool
}

type RegularCall struct {
	call *ssa.Call
}

func Regular(c *ssa.Call) *RegularCall {
	return &RegularCall{c}
}

// ReferredBy determines if the supplied referrer refers one of the Call's arguments.
func (c *RegularCall) ReferredBy(r Referrer) bool {
	for _, a := range c.call.Call.Args {
		if r.HasPathTo(a.(ssa.Node)) {
			return true
		}
	}

	return false
}
