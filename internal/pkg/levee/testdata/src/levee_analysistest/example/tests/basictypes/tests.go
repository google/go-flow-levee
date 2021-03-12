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

package basic

import (
	"levee_analysistest/example/core"
)

func BooleansDontPropagateTaint(s *core.Source) {
	is := isSourcey(s)
	core.Sink(is)
}

func isSourcey(x interface{}) bool {
	// not taking any chances
	return true
}

func IntegersDontPropagateTaint(sources []core.Source) {
	core.Sink(len(sources))
}
