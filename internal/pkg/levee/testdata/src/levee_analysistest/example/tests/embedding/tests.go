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

package embedding

import (
	"levee_analysistest/example/core"
)

type EmbedsSource struct {
	core.Source
}

type EmbedsSourcePointer struct {
	*core.Source
}

func TestStructThatEmbedsSourceIsSource() {
	core.Sink(EmbedsSource{}) // TODO(#96) want "a source has reached a sink"
}

func TestStructThatEmbedsSourcePointerIsSource() {
	core.Sink(EmbedsSourcePointer{}) // TODO(#96) want "a source has reached a sink"
}

func TestEmbeddedSourceIsSource() {
	core.Sink(EmbedsSource{}.Source) // want "a source has reached a sink"
}

func TestEmbeddedSourcePointerIsSource() {
	core.Sink(EmbedsSource{}.Source) // want "a source has reached a sink"
}

func TestEmbeddedSourceFieldIsSourceField() {
	core.Sink(EmbedsSource{}.Data) // want "a source has reached a sink"
}

func TestEmbeddedSourcePointerFieldIsSourceField() {
	core.Sink(EmbedsSourcePointer{}.Data) // want "a source has reached a sink"
}
