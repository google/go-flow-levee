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

package inlining

import (
	"levee_analysistest/example/core"
)

func NewSource() *core.Source {
	return &core.Source{}
}

func MaybeSource() (core.Source, error) {
	return core.Source{}, nil
}

func MaybeSourcePtr() (*core.Source, error) {
	return &core.Source{}, nil
}

func TestInlinedCall() {
	core.Sink(NewSource()) // want "a source has reached a sink"
}

func TestInlinedTupleCall() {
	core.Sink(MaybeSource())    // want "a source has reached a sink"
	core.Sink(MaybeSourcePtr()) // want "a source has reached a sink"
}

func TestInlinedRecv(sources <-chan core.Source) {
	core.Sink(<-sources) // want "a source has reached a sink"
}

func TestInlinedArrayIndex(sources [1]core.Source) {
	core.Sink(sources[0]) // want "a source has reached a sink"
}

func TestInlinedMapKey(sources map[string]core.Source) {
	core.Sink(sources["source"]) // want "a source has reached a sink"
}
