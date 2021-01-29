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

package deletethis

import (
	"levee_analysistest/example/core"
)

func TestTaintingInterfaceValueDoesNotTaintContainedValue(s *core.Source, str string) {
	var x interface{} = str
	colocate(s, x)
	// TODO(#209): no report should be produced below
	core.Sink(str) // want "a source has reached a sink"
}

func colocate(s *core.Source, x interface{}) {}
