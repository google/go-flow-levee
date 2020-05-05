// Package fields contains tests related to fields accessors
package fields

import (
	"example.com/core"
)

func TestFieldAccessors(s core.Source, ptr *core.Source) {
	core.Sinkf("Data: %v", s.GetData()) // want "a source has reached a sink"
	core.Sinkf("ID: %v", s.GetID())

	core.Sinkf("Data: %v", ptr.GetData()) // want "a source has reached a sink"
	core.Sinkf("ID: %v", ptr.GetID())
}

func TestDirectFieldAccess(c *core.Source) {
	core.Sinkf("Data: %v", c.Data) // want "a source has reached a sink"
	core.Sinkf("ID: %v", c.ID)
}

func TestProtoStyleFieldAccessorSanitizedPII(c *core.Source) {
	core.Sinkf("Source data: %v", core.Sanitize(c.GetData()))
}

func TestProtoStyleFieldAccessorPIISecondLevel(wrapper struct{ *core.Source }) {
	core.Sinkf("Source data: %v", wrapper.Source.GetData()) // want "a source has reached a sink"
	core.Sinkf("Source id: %v", wrapper.Source.GetID())
}

func tesDirectFieldAccessorPIISecondLevel(wrapper struct{ *core.Source }) {
	core.Sinkf("Source data: %v", wrapper.Source.Data) // want "a source has reached a sink"
	core.Sinkf("Source id: %v", wrapper.Source.ID)
}
