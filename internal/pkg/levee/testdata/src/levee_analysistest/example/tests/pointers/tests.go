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

package pointers

import (
	"levee_analysistest/example/core"
)

func TestNew() {
	s := new(core.Source)
	core.Sink(s) // want "a source has reached a sink"
}

func TestDoublePointer(s **core.Source) {
	core.Sink(s)   // want "a source has reached a sink"
	core.Sink(*s)  // want "a source has reached a sink"
	core.Sink(**s) // want "a source has reached a sink"
}

func TestDoubleReference(s core.Source) {
	ref := &s
	core.Sink(ref) // want "a source has reached a sink"
	refRef := &ref
	core.Sink(refRef) // want "a source has reached a sink"
}
