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
	"example.com/core"
)

func CreateSource() (core.Source, error) {
	return core.Source{}, nil
}

func CreateSourceRetvalsFlipped() (error, core.Source) {
	return nil, core.Source{}
}

func TakeSource(s core.Source) (string, int, interface{}) {
	return "", 0, nil
}

func TestOnlySourceExtractIsTainted() {
	s, err := CreateSource()
	core.Sink(s) // want "a source has reached a sink"
	core.Sink(err)
}

func TestOnlySourceExtractIsTaintedInstructionOrder() {
	s, err := CreateSource()
	core.Sink(err)
	core.Sink(s) // want "a source has reached a sink"
}

func TestOnlySourceExtractIsTaintedInstructionOrderFlipped() {
	err, s := CreateSourceRetvalsFlipped()
	core.Sink(s) // want "a source has reached a sink"
	core.Sink(err)
}

func TestExtractsFlipped() {
	err, s := CreateSourceRetvalsFlipped()
	core.Sink(err)
	core.Sink(s) // want "a source has reached a sink"
}

func TestExtractsFromCallWithSourceArg(s core.Source) {
	str, i, e := TakeSource(s)
	core.Sink(str) // want "a source has reached a sink"
	core.Sink(i)   // want "a source has reached a sink"
	core.Sink(e)   // want "a source has reached a sink"
}
