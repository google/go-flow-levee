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

	"example.com/core"
)

func taintString(core.Source, string) {}
func TestBasicTypeIsNotTainted(s core.Source, str string) {
	taintString(s, str)
	core.Sink(str)
}

func taintStringPointer(core.Source, *string) {}
func TestBasicPointerTypeIsTainted(s core.Source, strptr *string) {
	taintStringPointer(s, strptr)
	core.Sink(strptr) // want "a source has reached a sink"
}
func TestPointerToBasicTypeIsTainted(s core.Source, str string) {
	taintStringPointer(s, &str)
	core.Sink(str) // want "a source has reached a sink"
}

func taintInnoc(core.Source, core.Innocuous) {}
func TestNamedStructTypeIsNotTainted(s core.Source, i core.Innocuous) {
	taintInnoc(s, i)
	core.Sink(i)

}

func taintInnocPtr(core.Source, *core.Innocuous) {}
func TestNamedStructPointerIsTainted(s core.Source, i *core.Innocuous) {
	taintInnocPtr(s, i)
	core.Sink(i) // want "a source has reached a sink"
}
func TestPointerToNamedStructIsTainted(s core.Source, i core.Innocuous) {
	taintInnocPtr(s, &i)
	core.Sink(i) // want "a source has reached a sink"
}

type PointerHolder struct{ ptr *core.Source }

func taintPointerHolder(core.Source, PointerHolder) {}
func TestNamedStructPointerHolderIsTainted(s core.Source, ph PointerHolder) {
	taintPointerHolder(s, ph)
	core.Sink(ph) // want "a source has reached a sink"
}

type InnocSlice []core.Innocuous

func taintInnocSlice(core.Source, InnocSlice) {}
func TestNamedSliceTypeIsTainted(s core.Source, is InnocSlice) {
	taintInnocSlice(s, is)
	core.Sink(is) // want "a source has reached a sink"
}

func taintArrOfValues(core.Source, [1]string) {}
func TestArrOfValuesIsNotTainted(s core.Source, arr [1]string) {
	taintArrOfValues(s, arr)
	core.Sink(arr)
}

func taintArrOfPointers(core.Source, [1]*string) {}
func TestArrOfPointersIsTainted(s core.Source, arr [1]*string) {
	taintArrOfPointers(s, arr)
	core.Sink(arr) // want "a source has reached a sink"
}

func taintFunc(core.Source, func()) {}
func TestFuncIsNotTainted(s core.Source, f func()) {
	taintFunc(s, f)
	core.Sink(f)
}

func taintReferenceCollections(core.Source, map[string]string, chan string, []string) {}
func TestReferenceCollectionsAreTainted(s core.Source) {
	m := make(map[string]string)
	c := make(chan string)
	sl := make([]string, 0)
	taintReferenceCollections(s, m, c, sl)
	core.Sink(m)  // want "a source has reached a sink"
	core.Sink(c)  // want "a source has reached a sink"
	core.Sink(sl) // want "a source has reached a sink"
}

func taintEface(s core.Source, taintees ...interface{}) {}
func TestTaintedEface(s core.Source, i interface{}) {
	taintEface(s, i)
	core.Sink(i) // want "a source has reached a sink"

}
func TestTaintedThroughEface(s core.Source, str string, i core.Innocuous) {
	taintEface(s, str, i)
	core.Sink(str) // want "a source has reached a sink"
	core.Sink(i)   // want "a source has reached a sink"
}
func TestPointerTaintedThroughEface(s core.Source, str string, i core.Innocuous) {
	taintEface(s, &str, &i)
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
