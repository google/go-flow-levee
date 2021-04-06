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

package stdlib

import (
	"context"
	"io"
	"levee_analysistest/example/core"
	"strings"
)

func TestPropagateToAndFromConcreteReceiver(b *strings.Builder, s core.Source) {
	b.WriteString(s.Data)
	core.Sink(b)          // want "a source has reached a sink"
	core.Sink(b.String()) // want "a source has reached a sink"
}

func TestPropagateToAndFromAbstractReceiver(w io.ReadWriter, b []byte, s core.Source) {
	w.Write([]byte(s.Data))
	w.Read(b)
	core.Sink(w) // want "a source has reached a sink"
	core.Sink(b) // want "a source has reached a sink"
}

func TestPropagationInvolvingFuncWithInterfaceParameter(rf io.ReaderFrom, s core.Source) {
	rf.ReadFrom(strings.NewReader(s.Data))
	core.Sink(rf) // want "a source has reached a sink"
}

func TestPropagateThroughContext(c context.Context, s core.Source) {
	cc := context.WithValue(c, "data", s.Data)
	core.Sink(cc.Err())         // want "a source has reached a sink"
	core.Sink(cc.Value("data")) // want "a source has reached a sink"
}
