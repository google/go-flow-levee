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

package sanitization

import (
	"time"

	"levee_analysistest/example/core"
)

func TestSanitizedSourceDoesNotTriggerFinding(s *core.Source) {
	sanitized := core.Sanitize(s)[0]
	core.Sinkf("Sanitized %v", sanitized)
}

func TestSanitizedSourceDoesNotTriggerFindingWhenTypeAsserted(s *core.Source) {
	sanitized := core.Sanitize(s)[0].(*core.Source)
	core.Sinkf("Sanitized %v", sanitized)
}

func TestSanitizedSourceDoesNotTriggerFindingWithTypedSanitizer(s core.Source) {
	sanitized := core.SanitizeSource(s)
	core.Sinkf("Sanitized %v", sanitized)
}

func TestNotGuaranteedSanitization(s *core.Source) {
	p := s
	if time.Now().Weekday() == time.Monday {
		p = core.Sanitize(s)[0].(*core.Source)
	}
	core.Sinkf("Sometimes sanitized: %v", p) // want "a source has reached a sink"
}

func TestPointerSanitization(s *core.Source) {
	core.SanitizePtr(s)
	core.Sink(s)
}

func TestSanitizationByReference(s core.Source) {
	core.SanitizePtr(&s)
	core.Sink(s)
}

func TestIncorrectSanitizationByValue(s core.Source) {
	core.Sanitize(s)
	core.Sink(s) // TODO(#105): want "a source has reached a sink"
}

func TestOnlySanitizedIfLoopIsTaken() {
	var e interface{} = core.Source{}
	for false {
		e = core.Sanitize(e)[0]
	}
	core.Sink(e) // want "a source has reached a sink"
}

func TestTaintedInLoopAndSanitizedAfterLoop() {
	var e interface{}
	for false {
		e = core.Source{}
	}
	e = core.Sanitize(e)[0]
	core.Sink(e)
}

func TestMaybeTaintedInLoopButSanitizedBeforeLoopExit() {
	var e interface{}
	for false {
		if false {
			e = core.Source{}
		}
		e = core.Sanitize(e)[0]
	}
	core.Sink(e)
}

func TestTaintedInIfButSanitizedBeforeIfExit() {
	var e interface{}
	if false {
		e = core.Source{}
		e = core.Sanitize(e)[0]
	}
	core.Sink(e)
}

func TestPointerTaintedInIfButSanitizedBeforeIfExit() {
	var e interface{}
	if false {
		s := &core.Source{}
		core.SanitizePtr(s)
		e = s
	}
	// TODO(#155) want no report here
	core.Sink(e) // want "a source has reached a sink"
}

func TestSanitizedBeforeSinkInLoop() {
	var e interface{}
	for false {
		e = core.Source{}
		e = core.Sanitize(e)[0]
		core.Sink(e)
	}
}

func TestSanitizedBeforeMaybeSinkingMaybeTaintedValue() {
	var obj interface{}
	if false {
		obj = core.Source{}
	} else {
		obj = 10
	}

	obj = core.Sanitize(obj)[0]

	if false {
		core.Sink(obj)
	}
}

func TestSanitizedAfterSink() {
	s := core.Source{}
	core.Sink(s) // want "a source has reached a sink"
	core.SanitizePtr(&s)
}
