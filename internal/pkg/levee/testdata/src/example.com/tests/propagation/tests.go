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

package propagation

import (
	"example.com/core"
)

func main() {
	TestSinkWrapper(core.Source{})
	TestSinkWrapperWrapper(core.Source{})
	TestReturnsFive(core.Source{})
}

func TestSinkWrapperWrapper(s core.Source) {
	SinkWrapperWrapper(s) // want "a source has reached a sink"
}

func SinkWrapperWrapper(arg interface{}) {
	SinkWrapper(arg)
}

func TestSinkWrapper(s core.Source) {
	SinkWrapper(s) // want "a source has reached a sink"
}

func SinkWrapper(arg interface{}) {
	core.Sink(arg)
}

func TestReturnsFive(s core.Source) {
	five := ReturnsFive(s)
	core.Sink(five)
}

func ReturnsFive(arg interface{}) interface{} {
	return 5
}
