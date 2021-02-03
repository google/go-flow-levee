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

package position

import (
	"levee_analysistest/example/core"
)

func TestSourcePointerExtract() {
	s, _ := NewSource()
	core.Sink(s) // want "a source has reached a sink\n source: .*tests.go:22:19"
}

// In order for the SSA to contain a FieldAddr, the EmbedsSource instance's fields have to be addressable.
// We use a local variable here so that the fields will be addressable.
// The field is extracted on its own line so we can differentiate between the struct's position
// and the field's position.
// The Source in this function is created by a FieldAddr that is not represented explicitly in the code.
// Indeed, es.Data is actually es.Source.Data.
// Because of this, we expect the report to be produced at the struct's position.
func TestEmbeddedSourceFieldAddr() {
	es := EmbedsSource{}
	d := es.Data
	core.Sink(d) // want "a source has reached a sink\n source: .*tests.go:34:2"
}

// In order for the SSA to contain a Field, the EmbedsSource instance's fields must not be addressable.
// One way to do this is to create a literal and to access the field directly, as part of the same expression.
func TestEmbeddedSourceField() {
	core.Sink(EmbedsSource{}.Data) // want "a source has reached a sink\n source: .*tests.go:42:24"
}

type EmbedsSource struct {
	core.Source
}

func NewSource() (*core.Source, error) {
	return &core.Source{}, nil
}
