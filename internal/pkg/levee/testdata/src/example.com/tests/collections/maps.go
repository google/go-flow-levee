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
	"example.com/core"
)

func TestMapLiteralContainingSourceKeyIsTainted(s core.Source) {
	m := map[core.Source]string{s: "source"}
	core.Sink(m)                // want "a source has reached a sink"
	core.Sink(m[s])             // want "a source has reached a sink"
	core.Sink(m[core.Source{}]) // want "a source has reached a sink"
}

func TestMapLiteralContainingSourceValueIsTainted(s core.Source) {
	m := map[string]core.Source{"source": s}
	core.Sink(m)              // want "a source has reached a sink"
	core.Sink(m["source"])    // want "a source has reached a sink"
	core.Sink(m["innocuous"]) // want "a source has reached a sink"
}

func TestMapIsTaintedWhenSourceIsInserted(s core.Source) {
	m := map[string]core.Source{}
	m["source"] = s
	core.Sink(m)           // want "a source has reached a sink"
	core.Sink(m["source"]) // want "a source has reached a sink"
}

func TestTaintIsNotPropagatedwhenMapIsOverwritten(s core.Source) {
	m := map[string]core.Source{"source": s}
	core.Sink(m) // want "a source has reached a sink"
	m = nil
	core.Sink(m)
}
