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

package crosspkg

import "analyzertest/sourcetest"

func TestSourceDeclarations() {
	var varZeroVal sourcetest.Source                         // want "source identified"
	declZeroVal := sourcetest.Source{}                       // want "source identified"
	populatedVal := sourcetest.Source{Data: "secret", ID: 0} // want "source identified"

	var ptr *sourcetest.Source                                       // TODO want "source identified"
	newPtr := new(sourcetest.Source)                                 // want "source identified"
	ptrToDeclZero := &sourcetest.Source{}                            // want "source identified"
	ptrToDeclPopulataed := &sourcetest.Source{Data: "secret", ID: 1} // want "source identified"

	sourcetest.Noop(varZeroVal, declZeroVal, populatedVal, ptr, newPtr, ptrToDeclZero, ptrToDeclPopulataed)
}

func TestSourceParameterValue(val sourcetest.Source) { // want "source identified"

}

func TestSourceParameterPtr(ptr *sourcetest.Source) { // want "source identified"

}
