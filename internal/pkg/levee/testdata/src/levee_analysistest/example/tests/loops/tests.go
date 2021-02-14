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

package loops

import (
	"levee_analysistest/example/core"
)

func TestTaintInThenBlockInLoopSinkAfterLoop() {
	var e interface{}
	for true {
		if true {
			e = core.Source{}
		} else {
			e = nil
		}
	}
	core.Sink(e) // want "a source has reached a sink"
}

func TestTaintInElseBlockInLoopSinkAfterLoop() {
	var e interface{}
	for true {
		if true {
			e = nil
		} else {
			e = core.Source{}
		}
	}
	core.Sink(e) // want "a source has reached a sink"
}

func TestTaintInThenBlockSinkInElseBlockInLoop() {
	var e interface{}
	for true {
		if true {
			e = core.Source{}
		} else {
			core.Sink(e) // want "a source has reached a sink"
		}
	}
}

func TestTaintInElseBlockSinkInThenBlockInLoop() {
	var e interface{}
	for true {
		if true {
			e = core.Source{}
		} else {
			core.Sink(e) // want "a source has reached a sink"
		}
	}
}

func TestTaintInNestedConditionalInLoop() {
	var e interface{}
	for true {
		if true {
			if true {
				e = nil
			} else {
				e = core.Source{}
			}
		} else {
			e = nil
		}
	}
	core.Sink(e) // want "a source has reached a sink"
}

func TestTaintPropagationOverMultipleIterations() {
	var e1 interface{}
	var e2 interface{}
	for true {
		if true {
			e1 = core.Source{}
		} else {
			e2 = e1
		}
	}
	core.Sink(e1) // want "a source has reached a sink"
	core.Sink(e2) // want "a source has reached a sink"
}

func TestTaintPropagationOverMultipleIterationsWithNestedConditionals() {
	var e1 interface{}
	var e2 interface{}
	var e3 interface{}
	var e4 interface{}
	for true {
		if true {
			e1 = core.Source{}
		} else {
			if true {
				e4 = e3
			} else {
				e3 = e2
			}
			e2 = e1
		}
	}
	core.Sink(e1) // want "a source has reached a sink"
	core.Sink(e2) // want "a source has reached a sink"
	core.Sink(e3) // want "a source has reached a sink"
	core.Sink(e4) // want "a source has reached a sink"
}

func TestSourceOverwrittenBeforeLoopExit() {
	var e interface{}
	for true {
		if true {
			e = nil
		} else {
			e = core.Source{}
		}
		e = nil
	}
	core.Sink(e)
}
