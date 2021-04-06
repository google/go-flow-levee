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

func TestTaintedColocatedArgumentDoesNotReachSinkThatPrecedesColocation(w io.Writer, src core.Source) {
	if true {
		core.Sink(w)
	}
	fmt.Fprint(w, src)
}

func TestTaintedColocatedArgumentReachesSinkThatFollowsColocation(w io.Writer, src core.Source) {
	if _, err := fmt.Fprint(w, src); err != nil {
		core.Sink(w) // want "a source has reached a sink"
	}
}

func TestAvoidingIncorrectPropagationFromColocationDoesNotPreventCorrectReport(w io.Writer, src core.Source) {
	_, err := fmt.Fprint(w, src)
	if err != nil {
		core.Sink(w) // want "a source has reached a sink"
	}

	if true {
		fmt.Fprint(w, src)
	}
}
