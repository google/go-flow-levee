// Copyright 2021 Google LLC
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

package call

import (
	"levee_analysistest/example/core"
)

func sinkf1(i interface{}) {
	core.Sink(i) // want "a source has reached a sink"
}

func f1(s string) {
	sinkf1(s)
}

func createData() string {
	src := &core.Source{}
	return src.Data
}

// Test the case where the source is introduced in callees.
func TestSrcInCallees() {
	s := createData()
	core.Sink(s) // want "a source has reached a sink"
	f1(s)
}
