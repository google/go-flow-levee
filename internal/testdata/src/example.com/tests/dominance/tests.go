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
	// TODO This should not trigger
	core.Sinkf("Sanitized %v", sanitized) // want "a source has reached a sink,"
}

func TestNotGuaranteedSanitization(c *core.Source) {
	p := c
	if time.Now().Weekday() == time.Monday {
		p = core.Sanitize(c)[0].(*core.Source)
	}
	core.Sinkf("Sometimes sanitized: %v", p) // want "a source has reached a sink"
}
