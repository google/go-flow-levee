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

package phi

import (
	"levee_analysistest/example/core"
)

func TestPhiNodeDoesntPropagateTaintToOperands(i *core.Innocuous) {
	for {
		s := core.Source{}
		var ss, ii interface{} = s, i
		if true {
			ss = ii
		}
		core.Sink(ii)
		core.Sink(i)
		core.Sink(ss) // want "a source has reached a sink"
	}
}

func TestPhiNodeIsTaintedByTaintedOperand(s core.Source, i *core.Innocuous) {
	var ss, ii interface{} = s, i
	if true {
		ii = ss
	}
	core.Sink(ss) // want "a source has reached a sink"
	core.Sink(ii) // want "a source has reached a sink"
	core.Sink(i)
}
