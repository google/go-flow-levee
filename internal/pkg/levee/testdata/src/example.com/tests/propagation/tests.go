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

package propagation

import (
	"example.com/core"
)

func main() {
	TestSinkWrapperWrapper(core.Source{})
	TestSinkWrapper(core.Source{})
	TestSinkWrapperTwoArgs(core.Source{})
	TestOneArgSinkWrapper(core.Source{})
	TestReturnsFive(core.Source{})
	TestSinkWrapperSlice(core.Source{})
	TestSinkWrapperSpread(core.Source{})
}

func TestSinkWrapperWrapper(s core.Source) {
	SinkWrapperWrapper(s) // want "a source has reached a sink"
}

func SinkWrapperWrapper(arg interface{}) {
	SinkWrapper(arg)
}

func TestSinkWrapper(s core.Source) {
	SinkWrapper(s) // want "a source has reached a sink"
}

func SinkWrapper(arg interface{}) {
	core.Sink(arg)
}

func TestSinkWrapperTwoArgs(s core.Source) {
	SinkWrapperTwoArgs("not a source", s) // want "a source has reached a sink"
}

func SinkWrapperTwoArgs(a1 interface{}, a2 interface{}) {
	core.Sink(a1, a2)
}

func TestOneArgSinkWrapper(s core.Source) {
	OneArgSinkWrapper(s) // want "a source has reached a sink"
}

func OneArgSinkWrapper(arg interface{}) {
	core.OneArgSink(arg)
}

func TestReturnsFive(s core.Source) {
	five := ReturnsFive(s)
	core.Sink(five)
}

func ReturnsFive(arg interface{}) interface{} {
	return 5
}

func TestSinkWrapperSlice(s core.Source) {
	// This fails because SinkWrapperSlice receives a slice of interface{}, which it then passes
	// to core.Sink. Since core.Sink is variadic, the ssa code creates a slice, puts the first slice
	// into it, then calls core.Sink with that. Unforunately, the first slice is represented as an
	// `ssa.Parameter`, which is rather opaque. In particular, its only Referrer is the Store instruction,
	// and it has no Operands because it is an ssa.Value.
	SinkWrapperSlice("not a source", s, 0) // TODO want "a source has reached a sink"
}

func SinkWrapperSlice(args ...interface{}) {
	core.Sink(args)
}

func TestSinkWrapperSpread(s core.Source) {
	// This fails because in SinkWrapperSpread, core.Sink receives `args` as an ssa.Parameter.
	// Unfortunately, ssa.Parameter is rather opaque: its only Referrer is the call to core.Sink,
	// and it has no Operands.
	SinkWrapperSpread("not a source", s, 0) // TODO want "a source has reached a sink"
}

func SinkWrapperSpread(args ...interface{}) {
	core.Sink(args...)
}
