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

package sourcetest

// source container
type Source struct {
	Data string // source field
	ID   int    // public
}

// This function allows us to consume multiple arguments in a single line so this file can compile
func Noop(args ...interface{}) {}

func TestSourceDeclarations() {
	var varZeroVal Source                         // want "source identified"
	declZeroVal := Source{}                       // want "source identified"
	populatedVal := Source{Data: "secret", ID: 0} // want "source identified"

	var ptr *Source                                       // TODO want "source identified"
	newPtr := new(Source)                                 // want "source identified"
	ptrToDeclZero := &Source{}                            // want "source identified"
	ptrToDeclPopulataed := &Source{Data: "secret", ID: 1} // want "source identified"

	Noop(varZeroVal, declZeroVal, populatedVal, ptr, newPtr, ptrToDeclZero, ptrToDeclPopulataed)
}

// A report should be emitted for each parameter.
func TestSourceParameters(val Source, ptr *Source) { // want "source identified" "source identified"

}
