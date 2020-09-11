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
func noop(args ...interface{}) {}

func TestSourceDeclarations() {
	var varZeroVal Source                         // want "source identified"
	declZeroVal := Source{}                       // want "source identified"
	populatedVal := Source{Data: "secret", ID: 0} // want "source identified"

	// We do not want a "source identified" here, since this is nil
	// and gets optimized out when the SSA is built.
	var constPtr *Source

	var ptr *Source
	// We do want a "source identified" here.
	// ptr does not get optimized out because it gets assigned.
	ptr = &Source{}                                       // want "source identified"
	newPtr := new(Source)                                 // want "source identified"
	ptrToDeclZero := &Source{}                            // want "source identified"
	ptrToDeclPopulataed := &Source{Data: "secret", ID: 1} // want "source identified"

	noop(varZeroVal, declZeroVal, populatedVal, constPtr, ptr, newPtr, ptrToDeclZero, ptrToDeclPopulataed)
}

// A report should be emitted for each parameter.
func TestSourceParameters(val Source, ptr *Source) { // want "source identified" "source identified"

}

func TestSourceExtracts() {
	s, err := CreateSource()                     // want "source identified"
	sptr, err := NewSource()                     // want "source identified"
	mapSource, ok := map[string]Source{}[""]     // want "source identified"
	mapSourcePtr, ok := map[string]*Source{}[""] // want "source identified"
	chanSource, ok := <-(make(chan Source))      // want "source identified"
	chanSourcePtr, ok := <-(make(chan *Source))  // want "source identified"
	_, _, _, _, _, _, _, _ = s, sptr, mapSource, chanSource, mapSourcePtr, chanSourcePtr, err, ok
}

func CreateSource() (Source, error) {
	return Source{}, nil // want "source identified"
}

func NewSource() (*Source, error) {
	return &Source{}, nil // want "source identified"
}
