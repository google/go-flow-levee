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

func TestMapLiteralContainingSourceKeyIsTainted(s core.Source) {
	m := map[core.Source]string{s: "source"}
	core.Sink(m) // want "a source has reached a sink"
}

func TestMapLiteralContainingSourceValueIsTainted(s core.Source) {
	m := map[string]core.Source{"source": s}
	core.Sink(m) // want "a source has reached a sink"
}

func TestMapIsTaintedWhenSourceIsInserted(s core.Source) {
	m := map[core.Source]core.Source{}
	m[s] = s
	core.Sink(m) // want "a source has reached a sink"
}

func TestTaintIsNotPropagatedwhenMapIsOverwritten(s core.Source) {
	m := map[string]interface{}{"source": s}
	core.Sink(m) // want "a source has reached a sink"
	m = nil
	core.Sink(m)
}

func TestValueObtainedFromTaintedMapIsTainted(s core.Source) {
	m := map[interface{}]string{s: "source"}
	v := m[0]
	core.Sink(v) // want "a source has reached a sink"
}

func TestMapRemainsTaintedWhenSourceIsDeleted(s core.Source) {
	m := map[interface{}]string{s: "source"}
	delete(m, s)
	core.Sink(m) // want "a source has reached a sink"
}

func TestDeletingFromTaintedMapDoesNotTaintKey(key *string, sources map[*string]core.Source) {
	// The key needs to be a pointer parameter, because we don't traverse to non-pointer
	// arguments of a call, and we don't traverse to Allocs.
	delete(sources, key)
	core.Sink(key)
}

func TestMapUpdateWithTaintedValueDoesNotTaintTheKey(key string, value core.Source, sources map[string]core.Source) {
	sources[key] = value
	core.Sink(key)
}

func TestMapUpdateWithTaintedKeyDoesNotTaintTheValue(key core.Source, value string, sources map[core.Source]string) {
	sources[key] = value
	core.Sink(value)
}

func TestRangeOverMapWithSourceAsValue() {
	m := map[string]core.Source{"secret": core.Source{Data: "password1234"}}
	for k, s := range m {
		core.Sink(s) // want "a source has reached a sink"
		core.Sink(k)
	}
}

func TestRangeOverMapWithSourceAsKey() {
	m := map[core.Source]string{core.Source{Data: "password1234"}: "don't sink me"}
	for src, str := range m {
		core.Sink(src) // want "a source has reached a sink"
		core.Sink(str)
	}
}
