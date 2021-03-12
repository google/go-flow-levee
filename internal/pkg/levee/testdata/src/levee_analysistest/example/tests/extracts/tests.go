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

package extracts

import (
	"levee_analysistest/example/core"
)

func CreateSource() (core.Source, error) {
	return core.Source{}, nil
}

func TryUpdateSource(s core.Source) (core.Source, bool) {
	return s, true
}

func CreateSourceFlipped() (error, core.Source) {
	return nil, core.Source{}
}

func TestOnlySourceExtractIsTaintedFromCall() {
	s, ok := TryUpdateSource(core.Source{})
	core.Sink(s) // want "a source has reached a sink"
	core.Sink(ok)
}

func TestOnlySourceExtractIsTaintedFromTypeAssert(s core.Source) {
	var p interface{} = s
	s, ok := p.(core.Source)
	core.Sink(s) // want "a source has reached a sink"
	core.Sink(ok)
}

func TestOnlySourceExtractIsTaintedFromLookup() {
	s, ok := map[string]core.Source{}[""]
	core.Sink(s) // want "a source has reached a sink"
	core.Sink(ok)
}

func TestOnlySourceExtractIsTaintedFromChanRecv() {
	s, ok := <-make(chan core.Source)
	core.Sink(s) // want "a source has reached a sink"
	core.Sink(ok)
}

func TestOnlySourceExtractIsTaintedInstructionOrder() {
	s, err := CreateSource()
	core.Sink(err)
	core.Sink(s) // want "a source has reached a sink"
}

func TestOnlySourceExtractIsTaintedFlipped() {
	err, s := CreateSourceFlipped()
	core.Sink(s) // want "a source has reached a sink"
	core.Sink(err)
}

func TestOnlySourceExtractIsTaintedInstructionOrderFlipped() {
	err, s := CreateSourceFlipped()
	core.Sink(err)
	core.Sink(s) // want "a source has reached a sink"
}

func NewSource() (*core.Source, error) {
	return &core.Source{}, nil
}

func TestNewSource() {
	s, err := NewSource()
	core.Sink(s) // want "a source has reached a sink"
	core.Sink(err)
}

func TestCopy() {
	s := core.Source{}
	cpy, err := s.Copy()
	core.Sink(cpy) // want "a source has reached a sink"
	core.Sink(err)
}

func TestCopyPointer() {
	s := core.Source{}
	cpy, err := s.CopyPointer()
	core.Sink(cpy) // want "a source has reached a sink"
	core.Sink(err)
}
