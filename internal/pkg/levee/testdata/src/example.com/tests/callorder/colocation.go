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
	"example.com/core"
)

func TestTaintedColocatedArgumentDoesNotReachSinkThatPrecedesColocation() {
	s := core.Source{}
	i := newInnocuous()
	if err := fail(i); err != nil {
		core.Sink(err)
	}
	taintColocated(s, i)
}

func TestTaintedColocatedArgumentReachesSinkThatFollowsColocation() {
	s := core.Source{}
	i := newInnocuous()
	taintColocated(s, i)
	if err := fail(i); err != nil {
		core.Sink(err) // want "a source has reached a sink"
	}
}

func fail(x interface{}) error {
	return nil
}

func taintColocated(a, b interface{}) {
}

func newInnocuous() *core.Innocuous {
	return &core.Innocuous{}
}
