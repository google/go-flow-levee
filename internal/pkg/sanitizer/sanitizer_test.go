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

package sanitizer

import (
	"reflect"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

var testAnalyzer = &analysis.Analyzer{
	Name:       "domination",
	Run:        run,
	Doc:        "test harness for domination logic",
	Requires:   []*analysis.Analyzer{buildssa.Analyzer},
	ResultType: reflect.TypeOf([]*ssa.Call{}),
}

func run(pass *analysis.Pass) (interface{}, error) {
	in := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	var calls []*ssa.Call
	for _, fn := range in.SrcFuncs {
		for _, b := range fn.Blocks {
			for _, i := range b.Instrs {
				if c, ok := i.(*ssa.Call); ok {
					calls = append(calls, c)
				}
			}
		}
	}

	return calls, nil
}

func TestDomination(t *testing.T) {
	dir := analysistest.TestData()
	r := analysistest.Run(t, dir, testAnalyzer)
	if len(r) != 1 {
		t.Fatalf("Got %d results, wanted one", len(r))
	}
	calls, ok := r[0].Result.([]*ssa.Call)
	if !ok {
		t.Fatalf("Got result of type %T, wanted []*ssa.Call", calls)
	}

	/*
	    Call Sequence:

			call time.Now()
			call (time.Time).Weekday(t0)
			call scrub("P@ssword1":string) // Sanitizer - index 2
			call log.Print(t7...) // Dominated sink
			call log.Print(t13...) // Non dominated sink due to an if statement.

	*/

	sanitizer := &Sanitizer{calls[2]}
	dominatedSink := calls[3]
	nonDominatedSink := calls[4]

	if !sanitizer.Dominates(dominatedSink) {
		t.Fatalf("Sanitizer %v is dominating the leak %v due to no if statements", sanitizer.Call, dominatedSink)
	}

	if sanitizer.Dominates(nonDominatedSink) {
		t.Fatalf("Sanitizer %v is not dominating the leak %v due to an if statement", sanitizer.Call, nonDominatedSink)
	}
}
