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

	"levee_analysistest/example/core"
)

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
