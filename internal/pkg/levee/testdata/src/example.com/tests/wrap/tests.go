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

package wrap

import (
	"example.com/core"
)

func CreateSource() core.Source {
	return core.Source{}
}

func TestWrappedCall() {
	core.Sink(CreateSource()) // want "a source has reached a sink"
}

func TestNonWrappedCall() {
	s := CreateSource()
	core.Sink(s) // want "a source has reached a sink"
}

func TestWrappedRecv(sources <-chan core.Source) {
	core.Sink(<-sources) // want "a source has reached a sink"
}

func TestNonWrappedRecv(sources <-chan core.Source) {
	s := <-sources
	core.Sink(s) // want "a source has reached a sink"
}

func TestWrappedArray(sources [1]core.Source) {
	core.Sink(sources[0]) // want "a source has reached a sink"
}

func TestNonWrappedArray(sources [1]core.Source) {
	s := sources[0]
	core.Sink(s) // want "a source has reached a sink"
}

func TestWrappedMap(sources map[string]core.Source) {
	core.Sink(sources["source"]) // want "a source has reached a sink"
}

func TestNonWrappedMap(sources map[string]core.Source) {
	s := sources["source"]
	core.Sink(s) // want "a source has reached a sink"
}
