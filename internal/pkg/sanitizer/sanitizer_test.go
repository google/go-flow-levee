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
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

var testAnalyzer = &analysis.Analyzer{
	Name:     "domination",
	Run:      run,
	Doc:      "test harness for domination logic",
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	in := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	var sanitizers []Sanitizer
	var sinks []*ssa.Call
	for _, fn := range in.SrcFuncs {
		for _, b := range fn.Blocks {
			for _, i := range b.Instrs {
				if c, ok := i.(*ssa.Call); ok {
					name := c.Call.StaticCallee().Name()
					switch name {
					case "scrub":
						sanitizers = append(sanitizers, Sanitizer{c})
					case "Print":
						sinks = append(sinks, c)
					}
				}
			}
		}
	}

	for _, sink := range sinks {
		for _, san := range sanitizers {
			if san.Dominates(sink) {
				pass.Reportf(sink.Pos(), "dominated")
			}
		}
	}
	return nil, nil
}

func TestDomination(t *testing.T) {
	dir := analysistest.TestData()
	analysistest.Run(t, dir, testAnalyzer)
}
