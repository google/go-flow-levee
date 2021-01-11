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
	"fmt"

	"example.com/core"
)

func Identity(arg interface{}) interface{} {
	return arg
}

func TestIdentityPropagator(s core.Source) {
	i := Identity(s)
	core.Sink(i)           // want "a source has reached a sink"
	core.Sink(Identity(s)) // want "a source has reached a sink"
}

func ToString(arg interface{}) string {
	return fmt.Sprintf("%v", arg)
}

func TestToStringPropagator(s core.Source) {
	v := ToString(s)
	core.Sink(v) // want "a source has reached a sink"
}

func TestPropagationViaSourceMethod(s core.Source) {
	tainted := s.Propagate(s.Data)
	core.Sink(tainted) // want "a source has reached a sink"
}
