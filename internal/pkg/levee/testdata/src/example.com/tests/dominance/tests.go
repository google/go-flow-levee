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

// Package dominance contains test-cases for testing PII sanitization.
package dominance

import (
	"time"

	"example.com/core"
)

func TestSanitizedSourceDoesNotTriggerFinding(c *core.Source) {
	sanitized := core.Sanitize(c)
	core.Sinkf("Sanitized %v", sanitized)
}

func TestSanitizedSourceDoesNotTriggerFindingWhenTypeAsserted(c *core.Source) {
	sanitized := core.Sanitize(c)[0].(*core.Source)
	core.Sinkf("Sanitized %v", sanitized)
}

func TestSanitizedSourceDoesNotTriggerFindingWithTypedSanitizer(c core.Source) {
	sanitized := core.SanitizeSource(c)
	core.Sinkf("Sanitized %v", sanitized)
}

func TestNotGuaranteedSanitization(c *core.Source) {
	p := c
	if time.Now().Weekday() == time.Monday {
		p = core.Sanitize(c)[0].(*core.Source)
	}
	core.Sinkf("Sometimes sanitized: %v", p) // want "a source has reached a sink"
}

func TestSanitizationByPointer(c core.Source) {
	core.SanitizePtr(&c)
	core.Sink(c)
}
