// Package declarations contains test-cases for testing PII leak detection when sources are introduced via declarations.
package declarations

import (
	"example.com/core"
)

func TestSourceDeclaredInBody() {
	s := &core.Source{}
	core.Sinkf("%v", s) // want "a source has reached a sink"

	i := &core.Innocuous{}
	core.Sinkf("%v", i)
}

func TestSourceViaClosure() func() {
	s := &core.Source{}
	return func() {
		core.Sinkf("%v", s) // want "a source has reached a sink"
	}
}
