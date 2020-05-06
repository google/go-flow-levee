// Package receivers contains test-cases for testing PII leak detection when sources are introduced via receivers.
package receivers

import (
	"example.com/core"
)

type sourceBuilder struct {
	sourcePtr *core.Source
	sourceVal core.Source
}

func (b *sourceBuilder) buildP() {
	core.Sinkf("Building cluster %v", b.sourcePtr) // want "a source has reached a sink"
	core.Sinkf("Building cluster %v", b.sourceVal) // want "a source has reached a sink"
}

func (b sourceBuilder) buildV() {
	core.Sinkf("Building cluster %v", b.sourcePtr) // want "a source has reached a sink"
	core.Sinkf("Building cluster %v", b.sourceVal) // want "a source has reached a sink"
}
