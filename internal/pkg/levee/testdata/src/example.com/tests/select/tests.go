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

package sel

import (
	"example.com/core"
)

func TestSelectSourceSingleCase(sources <-chan core.Source) {
	// In the ssa, a single case select is equivalent to not having a select at all.
	select {
	case s := <-sources:
		core.Sink(s) // want "a source has reached a sink"
	}
}

func TestSelectSourceDefault(sources <-chan core.Source) {
	select {
	case s := <-sources:
		core.Sink(s) // want "a source has reached a sink"
	default:
		return
	}
}

func TestSelectSourcePointer(sources <-chan *core.Source) {
	select {
	case s := <-sources:
		core.Sink(s) // want "a source has reached a sink"
	default:
		return
	}
}

func TestTaintedInOneSelectSinkedInTheNext(objects chan interface{}) {
	select {
	case objects <- core.Source{}:
	default:
	}

	select {
	case s := <-objects:
		core.Sink(s) // want "a source has reached a sink"
	default:
	}
}

func TestTaintedAndSinkedInDifferentBranches(objects chan interface{}) {
	select {
	case objects <- core.Source{}:
	case s := <-objects:
		// TODO want no report here, because objects is only tainted if the
		// other branch is taken, and only one branch can be taken
		core.Sink(s) // want "a source has reached a sink"
	}
}

func TestTaintedAndSinkedInDifferentBranchesInLoop(objects chan interface{}) {
	for {
		select {
		case objects <- core.Source{}:
		case s := <-objects:
			core.Sink(s) // want "a source has reached a sink"
		}
	}
}

func TestSelectSourceAndInnoc(sources <-chan core.Source, innocs <-chan core.Innocuous) {
	select {
	case s := <-sources:
		core.Sink(s) // want "a source has reached a sink"
	case i := <-innocs:
		core.Sink(i)
	}
}

func TestSelectRecvIntoInterface(sources <-chan core.Source, innocs <-chan core.Innocuous) {
	var empty interface{}
	select {
	case empty = <-sources:
	case empty = <-innocs:
	}
	core.Sink(empty) // want "a source has reached a sink"
}

func TestTaintedInForkedClosure(objects chan interface{}) {
	go func() {
		objects <- core.Source{}
	}()

	select {
	case s := <-objects:
		core.Sink(s) // TODO want "a source has reached a sink"
	default:
		return
	}
}

func TestTaintedInForkedCall(objects chan interface{}) {
	go PutSource(objects)

	select {
	case s := <-objects:
		core.Sink(s) // TODO want "a source has reached a sink"
	default:
		return
	}
}

func PutSource(objects chan<- interface{}) {
	objects <- core.Source{}
}
