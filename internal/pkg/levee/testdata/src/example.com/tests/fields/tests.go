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

// Package fields contains tests related to fields accessors
package fields

import (
	"example.com/core"
)

func TestFieldAccessors(s core.Source, ptr *core.Source) {
	core.Sinkf("Data: %v", s.GetData()) // want "a source has reached a sink"
	core.Sinkf("ID: %v", s.GetID())

	core.Sinkf("Data: %v", ptr.GetData()) // want "a source has reached a sink"
	core.Sinkf("ID: %v", ptr.GetID())
}

func TestDirectFieldAccess(c *core.Source) {
	core.Sinkf("Data: %v", c.Data) // want "a source has reached a sink"
	core.Sinkf("ID: %v", c.ID)
}

func TestInlinedDirectFieldAccess() {
	core.Sinkf("Data: %v", core.Source{}.Data) // want "a source has reached a sink"
	core.Sinkf("ID: %v", core.Source{}.ID)
}

func TestProtoStyleFieldAccessorSanitizedPII(c *core.Source) {
	core.Sinkf("Source data: %v", core.Sanitize(c.GetData()))
}

func TestProtoStyleFieldAccessorPIISecondLevel(wrapper struct{ *core.Source }) {
	core.Sinkf("Source data: %v", wrapper.Source.GetData()) // want "a source has reached a sink"
	core.Sinkf("Source id: %v", wrapper.Source.GetID())
}

func TestDirectFieldAccessorPIISecondLevel(wrapper struct{ *core.Source }) {
	core.Sinkf("Source data: %v", wrapper.Source.Data) // want "a source has reached a sink"
	core.Sinkf("Source id: %v", wrapper.Source.ID)
}
