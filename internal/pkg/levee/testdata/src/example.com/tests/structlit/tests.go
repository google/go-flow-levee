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

// Package structlit contains tests related to struct literals.
package structlit

import (
	"example.com/core"
)

type Holder struct {
	s core.Source
	i core.Innocuous
}

type PointerHolder struct {
	s *core.Source
	i *core.Innocuous
}

func TestStructHoldingSourceAndInnocIsTainted(s core.Source, i core.Innocuous) {
	h := Holder{
		s,
		i,
	}
	core.Sink(h) // want "a source has reached a sink"
}

func TestStructHoldingSourceAndInnocIsTaintedReverseFieldOrder(s core.Source, i core.Innocuous) {
	h := Holder{
		i: i,
		s: s,
	}
	core.Sink(h) // want "a source has reached a sink"
}

func TestStructHoldingSourceAndInnocPointersIsTainted(s *core.Source, i *core.Innocuous) {
	h := PointerHolder{
		s,
		i,
	}
	core.Sink(h) // want "a source has reached a sink"
}

func TestStructHoldingSourceAndInnocPointersIsTaintedReverseFieldOrder(s *core.Source, i *core.Innocuous) {
	h := PointerHolder{
		i: i,
		s: s,
	}
	core.Sink(h) // want "a source has reached a sink"
}
