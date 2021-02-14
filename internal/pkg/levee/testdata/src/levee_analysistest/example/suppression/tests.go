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

// Package suppression contains tests for false positive suppression.
package suppression

import (
	"levee_analysistest/example/core"
)

func TestNotSuppressed(s core.Source) {
	core.Sink(s) // want "a source has reached a sink"
}

func TestOnelineLineComment(s core.Source) {
	// levee.DoNotReport
	core.Sink(s)
}

func TestOnelineGeneralComment(s core.Source) {
	/* levee.DoNotReport */
	core.Sink(s)
}

func TestInlineLineComment(s core.Source) {
	core.Sink(s) // levee.DoNotReport
}

func TestInlineGeneralComment(s core.Source) {
	core.Sink(s) /* levee.DoNotReport */
}

func TestMultilineLineComment(s core.Source) {
	// Line 1
	// levee.DoNotReport
	// Line 3
	core.Sink(s)
}

func TestMultilineGeneralComment(s core.Source) {
	/*
		Line 1
		levee.DoNotReport
		Line 3
	*/
	core.Sink(s)
}

func TestAdjacentReports(s core.Source) {
	core.Sink(s) // levee.DoNotReport
	core.Sink(s) // want "a source has reached a sink"
}

func TestReportsSeparatedByLineComment(s core.Source) {
	core.Sink(s) // want "a source has reached a sink"
	// levee.DoNotReport
	core.Sink(s)
}

func TestReportsSeparatedByGeneralComment(s core.Source) {
	core.Sink(s) /*
		levee.DoNotReport
	*/
	core.Sink(s) // want "a source has reached a sink"
}

func TestLineCommentBeforeGeneralComment(s core.Source) {
	// levee.DoNotReport
	/*
		The line comment above and this comment
		are part of the same comment group.
	*/
	core.Sink(s)
}
