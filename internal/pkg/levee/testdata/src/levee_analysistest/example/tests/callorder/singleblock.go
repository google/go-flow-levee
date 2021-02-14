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

func TestTaintBeforeSinking(s core.Source, w io.Writer) {
	_, _ = fmt.Fprintf(w, "%v", s)
	core.Sink(w) // want "a source has reached a sink"
}

func TestSinkBeforeTainting(s core.Source, w io.Writer) {
	core.Sink(w)
	_, _ = fmt.Fprintf(w, "%v", s)
}

func TestSinkBeforeAndAfterTainting(s core.Source, w io.Writer) {
	core.Sink(w)
	_, _ = fmt.Fprintf(w, "%v", s)
	core.Sink(w) // want "a source has reached a sink"
}
