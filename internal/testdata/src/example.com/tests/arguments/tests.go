package arguments

import (
	"example.com/core"
)

func TestSourceFromParamByReference(s *core.Source) {
	core.Sink("Source in the parameter %v", s) // want "a source has reached a sink"
}

func TestSourceMethodFromParamByReference(s *core.Source) {
	core.Sink("Source in the parameter %v", s.Data) // want "a source has reached a sink"
}

func TestSourceFromParamByReferenceInfo(s *core.Source) {
	core.Sink(s) // want "a source has reached a sink"
}

func TestSourceFromParamByValue(s core.Source) {
	core.Sink("Source in the parameter %v", s) // want "a source has reached a sink"
}

func TestUpdatedSource(s *core.Source) {
	s.Data = "updated"
	core.Sink("Updated %v", s) // want "a source has reached a sink"
}

func TestSourceFromAPointerCopy(s *core.Source) {
	cp := s
	core.Sink("Pointer copy of the source %v", cp) // want "a source has reached a sink"
}
