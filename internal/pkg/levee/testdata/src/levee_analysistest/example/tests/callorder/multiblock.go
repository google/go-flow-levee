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

package callorder

import (
	"fmt"
	"io"

	"levee_analysistest/example/core"
)

func TestSinkInIfBeforeTaint(s core.Source, w io.Writer) {
	if true {
		core.Sink(w)
	}
	fmt.Fprintf(w, "%v", s)
}

func TestTaintInIfBeforeSink(s core.Source, w io.Writer) {
	if true {
		fmt.Fprintf(w, "%v", s)
	}
	core.Sink(w) // want "a source has reached a sink"
}

func TestSinkAndTaintInDifferentIfBranches(s core.Source, w io.Writer) {
	if true {
		fmt.Fprintf(w, "%v", s)
	} else {
		core.Sink(w)
	}
}

func TestSinkInIfBeforeTaintInIf(s core.Source, w io.Writer) {
	if true {
		core.Sink(w)
	}
	if true {
		fmt.Fprintf(w, "%v", s)
	}
}

func TestTaintInIfBeforeSinkInIf(s core.Source, w io.Writer) {
	if true {
		fmt.Fprintf(w, "%v", s)
	}
	if true {
		core.Sink(w) // want "a source has reached a sink"
	}
}

func TestSinkBeforeTaintInSameIfBlock(s core.Source, w io.Writer) {
	if true {
		core.Sink(w)
		fmt.Fprintf(w, "%v", s)
	}
}

func TestTaintBeforeSinkInSameIfBlock(s core.Source, w io.Writer) {
	if true {
		fmt.Fprintf(w, "%v", s)
		core.Sink(w) // want "a source has reached a sink"
	}
}

func TestSinkInNestedIfBeforeTaint(s core.Source, w io.Writer) {
	if true {
		if true {
			core.Sink(w)
		}
	}
	fmt.Fprintf(w, "%v", s)
}

func TestTaintInNestedIfBeforeSink(s core.Source, w io.Writer) {
	if true {
		if true {
			fmt.Fprintf(w, "%v", s)
			core.Sink(w) // want "a source has reached a sink"
		}
		core.Sink(w) // want "a source has reached a sink"
	}
	core.Sink(w) // want "a source has reached a sink"
}

func TestSinkAndTaintInSeparateSwitchCases(s core.Source, w io.Writer) {
	switch "true" {
	case "true":
		core.Sink(w)
	case "false":
		fmt.Fprintf(w, "%v", s)
	}
}

func TestSinkAfterTaintInSwitch(s core.Source, w io.Writer) {
	switch "true" {
	case "true":
		fmt.Fprintf(w, "%v", s)
	}
	core.Sink(w) // want "a source has reached a sink"
}

func TestSinkAfterTaintInFor(sources []core.Source, w io.Writer) {
	for i := 0; i < len(sources); i++ {
		fmt.Fprintf(w, "%v", sources[i])
	}
	core.Sink(w) // want "a source has reached a sink"
}
