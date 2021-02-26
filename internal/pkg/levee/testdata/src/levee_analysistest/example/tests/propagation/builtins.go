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
	"levee_analysistest/example/core"
)

func TestCopyPropagatesTaintFromSrcToDst(s core.Source) {
	b := make([]byte, len(s.Data))
	bytesCopied := copy(b, s.Data)
	core.Sink(bytesCopied)
	core.Sink(b) // want "a source has reached a sink"
}

func TestCopyDoesNotPropagateTaintFromDstToSrc(s core.Source) {
	data := []byte(s.Data)
	redacted := []byte("<redacted>")
	copy(data, redacted)
	core.Sink(redacted)
}

func TestCopyDoesNotPropagateTaintToReturnedCount(s core.Source) {
	var b []byte
	count := copy(b, s.Data)
	core.Sink(count)
}

func TestAppendPropagatesTaintFromInputValueToInputAndOutputSlices(s core.Source, in, out []string) {
	out = append(in, s.Data)
	core.Sink(in)  // want "a source has reached a sink"
	core.Sink(out) // want "a source has reached a sink"
}

func TestAppendPropagatesTaintFromInputSliceToOutputSlice(s core.Source, safe interface{}, out []interface{}) {
	in := []interface{}{s.Data}
	out = append(in, safe)
	core.Sink(out) // want "a source has reached a sink"
	core.Sink(safe)
}

func TestSpreadIntoAppendPropagatesTaintFromValueToSlices(s core.Source, in, out []byte) {
	out = append(in, s.Data...)
	core.Sink(in)  // want "a source has reached a sink"
	core.Sink(out) // want "a source has reached a sink"
}
