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

// Package callcommon contains tests for Defer and Go instructions,
// which should be treated similarly to Call instructions.
package callcommon

import (
	"fmt"
	"io"
	"levee_analysistest/example/core"
)

func TestGoStdlib(w io.Writer, s core.Source) {
	go fmt.Fprint(w, s.Data)
	core.Sink(w) // want "a source has reached a sink"
}

func TestGoUnknownFunction(i *core.Innocuous, s core.Source) {
	go baz(i, s)
	core.Sink(i)
}

func baz(a, b interface{}) {}
