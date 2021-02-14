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

func TestArrayLiteralContainingSourceIsTainted(s core.Source) {
	tainted := [1]core.Source{s}
	core.Sink(tainted) // want "a source has reached a sink"
}

func TestArrayIsTaintedWhenSourceIsInserted(s core.Source) {
	arr := [2]interface{}{nil, nil}
	arr[0] = s
	core.Sink(arr) // want "a source has reached a sink"
}

func TestValueObtainedFromTaintedArrayIsTainted(s core.Source) {
	arr := [2]interface{}{nil, nil}
	arr[0] = s
	core.Sink(arr[1]) // want "a source has reached a sink"
}

func TestArrayRemainsTaintedWhenSourceIsOverwritten(s core.Source) {
	arr := [2]interface{}{s, nil}
	arr[0] = nil
	core.Sink(arr) // want "a source has reached a sink"
}

func TestRangeOverArray() {
	sources := [1]core.Source{core.Source{Data: "password1234"}}
	for i, s := range sources {
		core.Sink(s) // want "a source has reached a sink"
		core.Sink(i)
	}
}
