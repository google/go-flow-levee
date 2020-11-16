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
	"unsafe"

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
	colocateInnocPtr(s, &i)
	core.Sink(i) // TODO want "a source has reached a sink"
}

func colocateUnsafePointer(core.Source, unsafe.Pointer) {}
func TestUnsafePointerIsTainted(s core.Source, up unsafe.Pointer) {
	colocateUnsafePointer(s, up)
	core.Sink(up) // want "a source has reached a sink"
}

type PointerHolder struct{ ptr *core.Source }

func colocatePointerHolder(core.Source, PointerHolder) {}
func TestNamedStructPointerHolderIsTainted(s core.Source, ph PointerHolder) {
	colocatePointerHolder(s, ph)
	core.Sink(ph) // TODO want "a source has reached a sink"
}

type InnocSlice []core.Innocuous

func colocateInnocSlice(core.Source, InnocSlice) {}
func TestNamedSliceTypeIsTainted(s core.Source, is InnocSlice) {
	colocateInnocSlice(s, is)
	core.Sink(is) // want "a source has reached a sink"
}

func colocateArrOfValues(core.Source, [1]string) {}
func TestArrOfValuesIsNotTainted(s core.Source, arr [1]string) {
	colocateArrOfValues(s, arr)
	core.Sink(arr)
}

func colocateArrOfPointers(core.Source, [1]*string) {}
func TestArrOfPointersIsTainted(s core.Source, arr [1]*string) {
	colocateArrOfPointers(s, arr)
	// XXX
	core.Sink(arr) // want "a source has reached a sink"
}

func colocateFunc(core.Source, func()) {}
func TestFuncIsNotTainted(s core.Source, f func()) {
	colocateFunc(s, f)
	core.Sink(f)
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

func colocateEface(s core.Source, taintees ...interface{}) {}
func TestTaintedEface(s core.Source, i interface{}) {
	colocateEface(s, i)
	// XXX: we have no idea what i is hiding, so we have to assume the worst
	core.Sink(i) // want "a source has reached a sink"

}
func TestTaintedThroughEface(s core.Source, str string, i core.Innocuous) {
	colocateEface(s, str, i)
	// XXX: it is true that we don't want reports here, but I don't really believe it...
	core.Sink(str)
	core.Sink(i)
}
func TestPointerTaintedThroughEface(s core.Source, str string, i core.Innocuous) {
	colocateEface(s, &str, &i)
	// XXX: this is probably failing because of Allocs
	core.Sink(str) // want "a source has reached a sink"
	core.Sink(i)   // want "a source has reached a sink"
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
