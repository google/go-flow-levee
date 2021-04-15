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

func TestSourceMapIsSource() {
	core.Sink(map[string]string{})
	core.Sink(map[core.Source]string{})      // want "a source has reached a sink"
	core.Sink(map[string]core.Source{})      // want "a source has reached a sink"
	core.Sink(map[core.Source]core.Source{}) // want "a source has reached a sink"
}

func TestSourceArrayIsSource() {
	core.Sink([1]string{})
	core.Sink([1]core.Source{}) // want "a source has reached a sink"
}

func TestSourceSliceIsSource() {
	core.Sink([]string{})
	core.Sink([]core.Source{}) // want "a source has reached a sink"
}

func TestSourceChanIsSource() {
	core.Sink(make(chan string))
	core.Sink(make(chan core.Source)) // want "a source has reached a sink"
}

func TestSourcePointerCollectionIsSource() {
	core.Sink([]*core.Source{}) // want "a source has reached a sink"
}

func TestSourceCollectionParamIsSource(ss []core.Source) {
	core.Sink(ss) // want "a source has reached a sink"
}

func TestSourceCollectionReturnedFromFunctionCallIsSource() {
	core.Sink(GetSourcesSlice()) // want "a source has reached a sink"
	core.Sink(GetSourcesMap())   // want "a source has reached a sink"
}

func TestSourceCollectionReturnedFromMethodCallIsSource() {
	core.Sink(SourcesHolder{}.GetSourcesSlice()) // want "a source has reached a sink"
	core.Sink(SourcesHolder{}.GetSourcesMap())   // want "a source has reached a sink"
}

func TestSourceCollectionInStructFieldIsSource() {
	core.Sink(SourcesHolder{}.Sources)    // want "a source has reached a sink"
	core.Sink(SourcesHolder{}.SourcesMap) // want "a source has reached a sink"
}

type SourceSlice []core.Source

func TestAliasedSourceCollectionIsSource() {
	core.Sink(SourceSlice{}) // want "a source has reached a sink"
}

type DeeplyNested map[string][]map[string]map[string][][]map[string]core.Source

func TestDeeplyNestedSourceCollectionIsSource() {
	core.Sink(DeeplyNested{}) // want "a source has reached a sink"
}

func GetSourcesSlice() []core.Source {
	return nil
}

func GetSourcesMap() map[string]core.Source {
	return nil
}

type SourcesHolder struct {
	Sources    []core.Source
	SourcesMap map[string]core.Source
}

func (sh SourcesHolder) GetSourcesSlice() []core.Source {
	return sh.Sources
}

func (sh SourcesHolder) GetSourcesMap() map[string]core.Source {
	return sh.SourcesMap
}
