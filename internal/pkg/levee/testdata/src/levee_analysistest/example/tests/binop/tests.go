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

package binop

import (
	"fmt"

	"levee_analysistest/example/core"
)

func TestConcatenatingTaintedAndNonTaintedStrings(prefix string) {
	s := core.Source{Data: "password1234"}
	message := fmt.Sprintf("source: %v", s)
	fullMessage := prefix + message
	core.Sink(prefix)
	core.Sink(message)     // want "a source has reached a sink"
	core.Sink(fullMessage) // want "a source has reached a sink"
}
