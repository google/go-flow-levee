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

package discussion

import "fmt"

// This sink is identified by name
func Sink(args ...interface{}) {
	fmt.Println(args...)
}

// Calls to Sink here are exempted from identification by the configuration's .unless block.
// This context has been blessed by a security expert
func SimpleSinkPermitted(t Token) {
	Sink(t) // not a sink, so no issue reported
}

// All SinkType functions are sinks
type SinkType func(...interface{})

type SinkHolder struct {
	Sinker func(...interface{}) // Identified as sink by name
}

type TaggedSinkHolder struct {
	Sinker func(...interface{}) `myTag:"sink"` // Identified as sink by tag
}
