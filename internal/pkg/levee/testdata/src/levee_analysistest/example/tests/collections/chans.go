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

package collections

import (
	"levee_analysistest/example/core"
)

func TestSourceReceivedFromChannelIsTainted(sources <-chan core.Source) {
	s := <-sources
	core.Sink(s) // want "a source has reached a sink"
}

func TestChannelIsTaintedWhenSourceIsPlacedOnIt(sources chan<- core.Source) {
	sources <- core.Source{}
	core.Sink(sources) // want "a source has reached a sink"
}

func TestValueObtainedFromTaintedChannelIsTainted(c chan interface{}) {
	c <- core.Source{}
	s := <-c
	core.Sink(s) // want "a source has reached a sink"
}

func TestChannelIsNoLongerTaintedWhenNilledOut(sources chan core.Source) {
	sources <- core.Source{}
	sources = nil
	core.Sink(sources)
}

func TestRangeOverChan(sources chan core.Source) {
	for s := range sources {
		core.Sink(s) // want "a source has reached a sink"
	}
}
