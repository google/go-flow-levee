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
	"levee_analysistest/example/core"
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
		// TODO(#211) want no report here, because objects is only tainted if the
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
		core.Sink(s) // TODO(#99) want "a source has reached a sink"
	default:
		return
	}
}

func TestTaintedInForkedCall(objects chan interface{}) {
	go PutSource(objects)

	select {
	case s := <-objects:
		core.Sink(s) // TODO(#99) want "a source has reached a sink"
	default:
		return
	}
}

func PutSource(objects chan<- interface{}) {
	objects <- core.Source{}
}

func TestRecvFromTaintedAndNonTaintedChans(sources <-chan *core.Source, innocs <-chan *core.Innocuous) {
	select {
	case s := <-sources:
		core.Sink(sources) // want "a source has reached a sink"
		core.Sink(s)       // want "a source has reached a sink"
	case i := <-innocs:
		core.Sink(innocs)
		core.Sink(i)
	}
	core.Sink(sources) // want "a source has reached a sink"
	core.Sink(innocs)
}

func TestSendOnTaintedAndNonTaintedChans(i1 chan<- interface{}, i2 chan<- interface{}) {
	select {
	case i1 <- core.Source{}:
		core.Sink(i1) // want "a source has reached a sink"
	case i2 <- core.Innocuous{}:
		core.Sink(i2)
	}
	core.Sink(i1) // want "a source has reached a sink"
	core.Sink(i2)
}

func TestDaisyChain(srcs chan core.Source, i1, i2, i3, i4 chan interface{}) {
	select {
	case s := <-srcs:
		i1 <- s
	case z := <-i1:
		i2 <- z
	case y := <-i2:
		i3 <- y
	case x := <-i3:
		i4 <- x
	}
	core.Sink(srcs) // want "a source has reached a sink"
	core.Sink(i1)   // want "a source has reached a sink"
	core.Sink(i2)
	core.Sink(i3)
	core.Sink(i4)
}

func TestDaisyChainCasesInReverseOrder(srcs chan core.Source, i1, i2, i3, i4 chan interface{}) {
	select {
	case x := <-i3:
		i4 <- x
	case y := <-i2:
		i3 <- y
	case z := <-i1:
		i2 <- z
	case s := <-srcs:
		i1 <- s
	}
	core.Sink(srcs) // want "a source has reached a sink"
	core.Sink(i1)   // want "a source has reached a sink"
	core.Sink(i2)
	core.Sink(i3)
	core.Sink(i4)
}

func TestDaisyChainInLoop(srcs chan core.Source, i1, i2, i3, i4 chan interface{}) {
	for i := 0; i < 4; i++ {
		select {
		case s := <-srcs:
			i1 <- s
		case z := <-i1:
			i2 <- z
		case y := <-i2:
			i3 <- y
		case x := <-i3:
			i4 <- x
		}
	}
	core.Sink(srcs) // want "a source has reached a sink"
	core.Sink(i1)   // want "a source has reached a sink"
	core.Sink(i2)   // want "a source has reached a sink"
	core.Sink(i3)   // want "a source has reached a sink"
	core.Sink(i4)   // want "a source has reached a sink"
}
