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

package typeassert

import (
	"levee_analysistest/example/core"
)

func TestTypeSwitch(i interface{}) {
	switch t := i.(type) {
	case core.Innocuous:
		core.Sink(i)
		core.Sink(t)
	case *core.Source:
		// The type of i is definitively known within this block
		core.Sink(i) // TODO(#161) want "a source has reached a sink"
		core.Sink(t) // want "a source has reached a sink"
	case core.Source:
		// The type of i is definitively known within this block
		core.Sink(i) // TODO(#161) want "a source has reached a sink"
		core.Sink(t) // want "a source has reached a sink"
	default:
		core.Sink(i)
		core.Sink(t)
	}
}

func TestTypeSwitchInline(i interface{}) {
	switch i.(type) {
	case core.Innocuous, *core.Source, core.Source:
		// While not definitively known, the type of i may be asserted to be a source type
		core.Sink(i) // TODO(#161) want "a source has reached a sink"
	default:
		// do nothing
	}
}

func TestPanicTypeAssertSource(i interface{}) {
	s := i.(core.Source)
	_ = s
	// The dominating type assertion would panic if i were not a source type.
	core.Sink(i) // want "a source has reached a sink"
}

func TestPanicTypeAssertInnocuous(i interface{}) {
	// TypeAssert instructions should not cause reports by themselves.
	innoc := i.(core.Innocuous)
	_ = innoc
	core.Sink(i)
}

func TestOkayTypeAssert(i interface{}) {
	s, ok := i.(core.Source)
	_, _ = s, ok
	// The dominating type assertion will not panic.
	core.Sink(i)
}
