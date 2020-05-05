package sinks

import (
	"io"

	"example.com/core"
)

func TestSinks(s core.Source, writer io.Writer) {
	core.Sink(s)                  // want "a source has reached a sink"
	core.Sinkf("a source: %v", s) // want "a source has reached a sink"
	core.FSinkf(writer, s)        // want "a source has reached a sink"
	core.OneArgSink(s)            // TODO want "a source has reached a sink"

	core.Sink([]interface{}{s, s, s}...) // TODO want "a source has reached a sink"
	core.Sink([]interface{}{s, s, s})    // TODO want "a source has reached a sink"
}

func TestSinksWithRef(s *core.Source, writer io.Writer) {
	core.Sink(s)                  // want "a source has reached a sink"
	core.Sinkf("a source: %v", s) // want "a source has reached a sink"
	core.FSinkf(writer, s)        // want "a source has reached a sink"
	core.OneArgSink(s)            // TODO want "a source has reached a sink"

	core.Sink([]interface{}{s, s, s}...) // TODO want "a source has reached a sink"
	core.Sink([]interface{}{s, s, s})    // TODO want "a source has reached a sink"
}

func TestSinksInnocuous(innoc core.Innocuous, writer io.Writer) {
	core.Sink(innoc)
	core.Sinkf("a source: %v", innoc)
	core.FSinkf(writer, innoc)
	core.OneArgSink(innoc)

	core.Sink([]interface{}{innoc, innoc, innoc}...)
	core.Sink([]interface{}{innoc, innoc, innoc})
}

func TestSinksWithInnocuousRef(innoc *core.Innocuous, writer io.Writer) {
	core.Sink(innoc)
	core.Sinkf("a source: %v", innoc)
	core.FSinkf(writer, innoc)
	core.OneArgSink(innoc)

	core.Sink([]interface{}{innoc, innoc, innoc}...)
	core.Sink([]interface{}{innoc, innoc, innoc})
}
