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

package call

import (
	"reflect"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

func TestRegularCallReferredBy(t *testing.T) {
	cases := []struct {
		desc    string
		pattern string
		r       Referrer
		want    bool
	}{
		{
			"call without args",
			"noarg",
			fakeReferrer{true},
			false,
		},
		{
			"call with one arg that is referred to by a referrer",
			"onearg",
			fakeReferrer{true},
			true,
		},
		{
			"call with one arg that isn't referred to",
			"onearg",
			fakeReferrer{false},
			false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			dir := analysistest.TestData()
			r := analysistest.Run(t, dir, testAnalyzer, tt.pattern)
			call := Regular(getCall(t, r))
			got := call.ReferredBy(tt.r)
			if got != tt.want {
				t.Errorf("call.ReferredBy(%v) == %v, want %v", call, got, tt.want)
			}
		})
	}
}

func getCall(t *testing.T, r []*analysistest.Result) *ssa.Call {
	t.Helper()
	if len(r) != 1 {
		t.Fatalf("Got %d results, want one", len(r))
	}
	a, ok := r[0].Result.(analyzerResult)
	if !ok {
		t.Fatalf("Got result of type %T, want analyzerResult", a)
	}
	if len(a.calls) != 1 {
		t.Fatalf("Got %d calls, want one", len(a.calls))
	}
	return a.calls[0]
}

type fakeReferrer struct {
	hasPathTo bool
}

func (f fakeReferrer) RefersTo(node ssa.Node) bool { return f.hasPathTo }

var testAnalyzer = &analysis.Analyzer{
	Name:       "call",
	Run:        run,
	Doc:        "test harness for call",
	Requires:   []*analysis.Analyzer{buildssa.Analyzer},
	ResultType: reflect.TypeOf(analyzerResult{}),
}

type analyzerResult struct {
	calls []*ssa.Call
}

func run(pass *analysis.Pass) (interface{}, error) {
	in := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	var result analyzerResult
	for _, fn := range in.SrcFuncs {
		for _, b := range fn.Blocks {
			for _, i := range b.Instrs {
				switch v := i.(type) {
				case *ssa.Call:
					result.calls = append(result.calls, v)
				}
			}
		}
	}
	return result, nil
}
