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

package collections

import (
	"levee_analysistest/example/core"
)

func TestSlices(s core.Source) {
	slice := []core.Source{s}
	core.Sink(slice)                      // want "a source has reached a sink"
	core.Sink([]core.Source{s})           // want "a source has reached a sink"
	core.Sink([]interface{}{s})           // want "a source has reached a sink"
	core.Sink([]interface{}{0, "", s})    // want "a source has reached a sink"
	core.Sink([]interface{}{0, "", s}...) // want "a source has reached a sink"
}

func TestRangeOverSlice() {
	sources := []core.Source{core.Source{Data: "password1234"}}
	for i, s := range sources {
		core.Sink(s) // want "a source has reached a sink"
		core.Sink(i)
	}
}

func TestRangeOverInterfaceSlice() {
	for i, s := range []interface{}{core.Source{Data: "password1235"}} {
		core.Sink(s) // want "a source has reached a sink"
		core.Sink(i)
	}
}

func TestSliceBoundariesAreNotTainted(lo, hi, max int) {
	sources := [1]core.Source{{Data: "secret"}}
	slice := sources[lo:hi:max]
	core.Sink(lo)
	core.Sink(hi)
	core.Sink(max)
	_ = slice
}

func TestSlicedArrayIsTainted() {
	innocs := [1]interface{}{nil}
	slice := innocs[:]
	slice[0] = core.Source{Data: "secret"}
	core.Sink(innocs) // want "a source has reached a sink"
	_ = slice
}
