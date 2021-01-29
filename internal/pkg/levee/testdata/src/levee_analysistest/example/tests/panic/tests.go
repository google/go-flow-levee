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

package panic

import (
	"levee_analysistest/example/core"
)

func TestPanicIsASink(source core.Source) {
	panic(source) // want "a source has reached a sink"
}

func TestPanicIsASinkSanitized(source core.Source) {
	sanitized := core.Sanitize(source)[0]
	panic(sanitized)
}

func TestGoPanicIsASink(source core.Source) {
	go panic(source) // TODO(#231): want "a source has reached a sink"
}

func TestDeferPanicIsASink(source core.Source) {
	defer panic(source) // TODO(#231): want "a source has reached a sink"
}

func TestPanicOnNonSourceDoesNotProduceReport(source core.Source) {
	y := 42
	panic(y)
}
