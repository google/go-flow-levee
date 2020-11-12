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

package colocation

import (
	"encoding/json"

	"example.com/core"
)

func TestTaintIsPropagatedToColocatedPointerArguments(s core.Source, i core.Innocuous, ip *core.Innocuous) {
	taintColocated(s, &i, ip)
	core.Sink(s)  // want "a source has reached a sink"
	core.Sink(i)  // TODO want "a source has reached a sink"
	core.Sink(ip) // want "a source has reached a sink"
}

func TestTaintIsPropagatedToColocatedPointerArgumentsThroughEface(s core.Source, i core.Innocuous, ip *core.Innocuous) {
	taintColocatedEface(s, &i, ip)
	core.Sink(s)  // want "a source has reached a sink"
	core.Sink(i)  // TODO want "a source has reached a sink"
	core.Sink(ip) // want "a source has reached a sink"
}

// CVE-2020-8564
func TestTaintIsPropagatedToDataBeingUnmarshalled(contents []byte) (src core.Source, err error) {
	if err = json.Unmarshal(contents, &src); err != nil {
		core.Sink(src)      // want "a source has reached a sink"
		core.Sink(contents) // want "a source has reached a sink"
		return
	}
	core.Sink(src)      // want "a source has reached a sink"
	core.Sink(contents) // want "a source has reached a sink"
	return
}

func taintColocated(s core.Source, i *core.Innocuous, ip *core.Innocuous) {
}

func taintColocatedEface(a, b, c interface{}) {
}
