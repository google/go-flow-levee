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

// type alias
type Alias = Source

// type definition where the underlying type is a Source
type Definition Source

type FuncField struct {
	flipCoin func() bool
	producer func() Source
	ptr      func() *Source
}

// This test exercises isSourceType for *types.Signature and related interactions.
func TestFunctionFields() {
	bar := FuncField{
		flipCoin: func() bool {
			return true // It's a trick coin
		},
		producer: func() Source {
			return Source{} // want "source identified"
		},
		ptr: func() *Source {
			return &Source{} // want "source identified"
		},
	}

	// When the assigned-to type is a Source, then we expect two Allocs, and therefore two Source identifications.
	// This Alloc doesn't occur if the assigned-to type is a pointer or interface
	var i interface{} = bar.producer() // want "source identified"
	s := bar.producer()                // want "source identified" "source identified"
	ptr := bar.ptr()                   // want "source identified"

	noop(bar, s, i, ptr)
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
	ptr = &Source{}                                      // want "source identified"
	newPtr := new(Source)                                // want "source identified"
	ptrToDeclZero := &Source{}                           // want "source identified"
	ptrToDeclPopulated := &Source{Data: "secret", ID: 1} // want "source identified"

	alias := Alias{} // want "source identified"
	def := Definition{}

	noop(varZeroVal, declZeroVal, populatedVal, constPtr, ptr, newPtr, ptrToDeclZero, ptrToDeclPopulated, alias, def)
}

// A report should be emitted for each parameter, as well as the (implicit) Alloc for val.
func TestSourceParameters(val Source, ptr *Source) { // want "source identified" "source identified" "source identified"

}

// A report should be emitted for val, because it will have an Alloc in the body.
// A report should *not* be emitted for ptr, because it will not, by itself, lead to
// the creation of a Source value.
func TestNamedReturnValues() (val Source, ptr *Source) { // want "source identified"
	return
}

func TestSourceExtracts() {
	// We expect two reports for this case, because creating s
	// will require an Extract and an Alloc.
	s, err := CreateSource() // want "source identified" "source identified"
	sptr, err := NewSource() // want "source identified"

	// We expect three reports for this case, because:
	// 1. the map is a Source
	// 2. there is an Extract for the mapSource value
	// 3. there is an Alloc for the mapSource value
	mapSource, ok := map[string]Source{}[""] // want "source identified" "source identified" "source identified"

	// We expect two reports here, for the map and the Extract.
	// (There won't be an Alloc because mapSourcePtr is a pointer.)
	mapSourcePtr, ok := map[string]*Source{}[""] // want "source identified" "source identified"

	// These two cases are similar to the map cases above.
	// The reasoning behind the number of expected reports is the same.
	chanSource, ok := <-(make(chan Source))     // want "source identified" "source identified" "source identified"
	chanSourcePtr, ok := <-(make(chan *Source)) // want "source identified" "source identified"

	_, _, _, _, _, _, _, _ = s, sptr, mapSource, chanSource, mapSourcePtr, chanSourcePtr, err, ok
}

func TestCollections(ss []Source) { // want "source identified"
	_ = map[Source]string{} // want "source identified"
	_ = map[string]Source{} // want "source identified"
	_ = map[Source]Source{} // want "source identified"
	_ = [1]Source{}         // want "source identified"
	_ = []Source{}          // want "source identified"
	_ = make(chan Source)   // want "source identified"
	_ = []*Source{}         // want "source identified"
	_ = SourceSlice{}       // want "source identified"
	_ = DeeplyNested{}      // want "source identified"
}

type SourceSlice []Source
type DeeplyNested map[string][]map[string]map[string][][]map[string]Source

func TestTaggedSourceIdentification() {
	_ = TaggedSource{} // want "source identified"
}

func TestNamedInterface(x SourceInterface) { // want "source identified"
}
