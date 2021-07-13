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

// Package receivers contains test-cases for testing PII leak detection when sources are introduced via receivers.
package receivers

import (
	"levee_analysistest/example/core"
)

type SourceBuilder struct {
	sourcePtr *core.Source
	sourceVal core.Source
}

func (b *SourceBuilder) buildP() {
	core.Sinkf("Building cluster %v", b.sourcePtr) // want "a source has reached a sink"
	core.Sinkf("Building cluster %v", b.sourceVal) // want "a source has reached a sink"
}

func (b SourceBuilder) buildV() {
	core.Sinkf("Building cluster %v", b.sourcePtr) // want "a source has reached a sink"
	core.Sinkf("Building cluster %v", b.sourceVal) // want "a source has reached a sink"
}
