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

// Package crosspkg is used to test whether evaluating reachability of sinks
// from sources using pointer analysis works across packages.
package crosspkg

import (
	"example.com/core"
	"example.com/tests/propagators"
)

func main() {
	TestCrossPackageSinkWrapper(core.Source{})
}

func TestCrossPackageSinkWrapper(s core.Source) {
	propagators.SinkWrapper(s) // TODO want "a source has reached a sink, sink:"
}
