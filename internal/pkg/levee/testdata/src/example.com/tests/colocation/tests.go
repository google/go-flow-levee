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

package colocation

import (
	"encoding/json"
	"reflect"

	"example.com/core"
)

func colocateString(core.Source, string) {}

func TestBasicTypeIsNotTainted(s core.Source, str string) {
	colocateString(s, str)
	core.Sink(str)
}

func colocateStringPointer(core.Source, *string) {}

func TestBasicPointerTypeIsTainted(s core.Source, strptr *string) {
	colocateStringPointer(s, strptr)
	core.Sink(strptr) // want "a source has reached a sink"
}

func TestPointerToBasicTypeIsTainted(s core.Source, str string) {
	// This test is failing because &x introduces an Alloc,
	// and we don't traverse through non-array Allocs
	colocateStringPointer(s, &str)
	core.Sink(str) // TODO want "a source has reached a sink"
}

func colocateInnoc(core.Source, core.Innocuous) {}

func TestNamedStructTypeIsNotTainted(s core.Source, i core.Innocuous) {
	colocateInnoc(s, i)
	core.Sink(i)
}

func colocateInnocPtr(core.Source, *core.Innocuous) {}

func TestNamedStructPointerIsTainted(s core.Source, i *core.Innocuous) {
	colocateInnocPtr(s, i)
	core.Sink(i) // want "a source has reached a sink"
}

func TestPointerToNamedStructIsTainted(s core.Source, i core.Innocuous) {
	// This test is failing because &x introduces an Alloc,
	// and we don't traverse through non-array Allocs
	colocateInnocPtr(s, &i)
	core.Sink(i) // TODO want "a source has reached a sink"
}

type PointerHolder struct{ ptr *core.Source }

func colocatePointerHolder(core.Source, PointerHolder) {}

func TestNamedStructPointerHolderIsTainted(s core.Source, ph PointerHolder) {
	// This test is failing because ph is created by an Alloc,
	// and we don't traverse through non-array Allocs
	colocatePointerHolder(s, ph)
	core.Sink(ph) // TODO want "a source has reached a sink"
}

func colocateArrOfValues(core.Source, [1]string) {}

func TestArrOfValuesIsNotTainted(s core.Source, arr [1]string) {
	colocateArrOfValues(s, arr)
	core.Sink(arr)
}

func colocateArrOfPointers(core.Source, [1]*string) {}

func TestArrOfPointersIsTainted(s core.Source, arr [1]*string) {
	colocateArrOfPointers(s, arr)
	core.Sink(arr) // want "a source has reached a sink"
}

func colocateReferenceCollections(core.Source, map[string]string, chan string, []string) {}

func TestReferenceCollectionsAreTainted(s core.Source) {
	m := make(map[string]string)
	c := make(chan string)
	sl := make([]string, 0)
	colocateReferenceCollections(s, m, c, sl)
	core.Sink(m)  // want "a source has reached a sink"
	core.Sink(c)  // want "a source has reached a sink"
	core.Sink(sl) // want "a source has reached a sink"
}

func colocateReflectValue(core.Source, reflect.Value) {}

func TestReflectValuesAreTainted(s core.Source, r reflect.Value) {
	// This test is failing because ph is created by an Alloc,
	// and we don't traverse through non-array Allocs
	colocateReflectValue(s, r)
	core.Sink(r) // TODO want "a source has reached a sink"
}

func colocateEface(s core.Source, taintees ...interface{}) {}

func TestTaintedEface(s core.Source, i interface{}) {
	colocateEface(s, i)
	core.Sink(i) // want "a source has reached a sink"

}

func TestTaintedThroughEface(s core.Source, str string, i core.Innocuous) {
	colocateEface(s, str, i)
	// Ideally, we wouldn't want reports for either of those values, because they
	// are not pointers.
	// However, because they are passed to the call via an interface type,
	// we have no easy way to know that these values can't actually be tainted.
	core.Sink(str) // want "a source has reached a sink"
	core.Sink(i)   // TODO want "a source has reached a sink"
}

func TestPointerTaintedThroughEface(s core.Source, str string, i core.Innocuous) {
	colocateEface(s, &str, &i)
	// These tests are failing because &x introduces an Alloc,
	// and we don't traverse through non-array Allocs
	core.Sink(str) // TODO want "a source has reached a sink"
	core.Sink(i)   // TODO want "a source has reached a sink"
}

// CVE-2020-8564
func TestTaintIsPropagatedToDataBeingUnmarshalled(contents []byte) (src core.Source, err error) {
	if err = json.Unmarshal(contents, &src); err != nil {
		core.Sink(src)      // want "a source has reached a sink"
		core.Sink(contents) // want "a source has reached a sink"
		return
	}
	core.Sink(src)      // want "a source has reached a sink"
	core.Sink(contents) // want "a source has reached a sink"
	return
}
