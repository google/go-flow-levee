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
	"strconv"

	"levee_analysistest/example/core"
)

func TestFieldAccessors(s core.Source, ptr *core.Source) {
	core.Sinkf("Data: %v", s.GetData()) // want "a source has reached a sink"
	core.Sinkf(s.ShowData())            // want "a source has reached a sink"
	core.Sinkf("ID: %v", s.GetID())

	core.Sinkf("Data: %v", ptr.GetData()) // want "a source has reached a sink"
	core.Sinkf(ptr.ShowData())            // want "a source has reached a sink"
	core.Sinkf("ID: %v", ptr.GetID())
}

func TestDirectFieldAccess(c *core.Source) {
	core.Sinkf("Data: %v", c.Data) // want "a source has reached a sink"
	core.Sinkf("ID: %v", c.ID)
}

func TestInlinedDirectFieldAccess() {
	// This pattern is unlikely to occur in real code.
	// The intent is to get Field instructions in the SSA
	// so that we can validate that those are handled correctly.
	core.Sinkf("Data: %v", core.Source{Data: "password1234", ID: 1234}.Data) // want "a source has reached a sink"
	core.Sinkf("ID: %v", core.Source{Data: "password1234", ID: 1234}.ID)
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

func TestTaggedStruct(s core.TaggedSource) {
	core.Sink(s) // want "a source has reached a sink"
}

func TestTaggedAndNonTaggedFields(s core.TaggedSource) {
	core.Sink(s.Data) // want "a source has reached a sink"
	core.Sink(s.ID)
}

func TestTaintFieldOnNonSourceStruct(s core.Source, i *core.Innocuous) {
	i.Data = s.Data
	core.Sink(i)      // TODO(#228) want "a source has reached a sink"
	core.Sink(i.Data) // TODO(#228) want "a source has reached a sink"
}

func TestTaintNonSourceFieldOnSourceType(s core.Source, i *core.Innocuous) {
	s.ID, _ = strconv.Atoi(s.Data)
	core.Sink(s.ID) // TODO(#228) want "a source has reached a sink"
}

type Headers struct {
	Name  string
	Auth  map[string]string `levee:"source"`
	Other map[string]string
}

func fooByPtr(h *Headers) {}

func foo(h Headers) {}

func TestCallWithStructReferenceTaintsEveryField(h Headers) {
	fooByPtr(&h)       // without interprocedural assessment, foo can do anything, so this call should taint every field on h
	core.Sink(h.Name)  // TODO(#229) want "a source has reached a sink"
	core.Sink(h.Other) // TODO(#229) want "a source has reached a sink"
}

func TestCallWithStructValueDoesNotTaintNonReferenceFields(h Headers) {
	foo(h) // h is passed by value, so only its reference-like fields should be tainted
	core.Sink(h.Name)
	core.Sink(h.Other) // TODO(#229) want "a source has reached a sink"
}
