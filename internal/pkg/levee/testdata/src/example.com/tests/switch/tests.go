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

package _switch

import (
	"example.com/core"
)

func TestTypeSwitch(i interface{}) {
	switch t := i.(type) {
	case core.Innocuous:
		core.Sink(i)
		core.Sink(t)
	case *core.Source:
		core.Sink(i) // TODO/discuss - do we want "a source has reached a sink"
		core.Sink(t) // want "a source has reached a sink"
	case core.Source:
		core.Sink(i) // TODO/discuss - do we want "a source has reached a sink"
		core.Sink(t) // want "a source has reached a sink"
	default:
		core.Sink(i)
		core.Sink(t)
	}
}

func TestTypeSwitchInline(i interface{}) {
	switch i.(type) {
	case core.Innocuous, *core.Source, core.Source:
		core.Sink(i) // TODO/discuss - do we want "a source has reached a sink"
	default:
		// do nothing
	}
}
