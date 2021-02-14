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

package sinks

import (
	"io"

	"levee_analysistest/example/core"
)

func TestSinks(s core.Source, writer io.Writer) {
	core.Sinker{}.Sink(s)         // want "a source has reached a sink"
	core.Sink(s)                  // want "a source has reached a sink"
	core.Sinkf("a source: %v", s) // want "a source has reached a sink"
	core.FSinkf(writer, s)        // want "a source has reached a sink"
	core.OneArgSink(s)            // want "a source has reached a sink"
}

func TestSinksWithRef(s *core.Source, writer io.Writer) {
	core.Sinker{}.Sink(s)         // want "a source has reached a sink"
	core.Sink(s)                  // want "a source has reached a sink"
	core.Sinkf("a source: %v", s) // want "a source has reached a sink"
	core.FSinkf(writer, s)        // want "a source has reached a sink"
	core.OneArgSink(s)            // want "a source has reached a sink"
}

func TestSinksInnocuous(innoc core.Innocuous, writer io.Writer) {
	core.Sinker{}.Sink(innoc)
	core.Sink(innoc)
	core.Sinkf("a source: %v", innoc)
	core.FSinkf(writer, innoc)
	core.OneArgSink(innoc)

	core.Sink([]interface{}{innoc, innoc, innoc}...)
	core.Sink([]interface{}{innoc, innoc, innoc})
}

func TestSinksWithInnocuousRef(innoc *core.Innocuous, writer io.Writer) {
	core.Sinker{}.Sink(innoc)
	core.Sink(innoc)
	core.Sinkf("a source: %v", innoc)
	core.FSinkf(writer, innoc)
	core.OneArgSink(innoc)

	core.Sink([]interface{}{innoc, innoc, innoc}...)
	core.Sink([]interface{}{innoc, innoc, innoc})
}

// TestSinksSourceAndInnocuous covers the situations in which both a Source and
// a non-Source value are in scope, as well as variadic calls involving both
// Source and non-Source arguments.
func TestSinksSourceAndInnocuous(source core.Source, innoc core.Innocuous) {
	core.Sink(source)        // want "a source has reached a sink"
	core.OneArgSink(source)  // want "a source has reached a sink"
	core.Sink(innoc, source) // want "a source has reached a sink"
	core.Sink(source, innoc) // want "a source has reached a sink"
	core.Sink(innoc)
	core.OneArgSink(innoc)
}
