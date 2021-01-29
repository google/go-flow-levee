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

package arguments

import (
	"levee_analysistest/example/core"
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
