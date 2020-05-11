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

package utils

import (
	"fmt"
	"golang.org/x/tools/go/ssa"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/buildssa"
)

var testAnalyzer = &analysis.Analyzer{
	Name:       "domination",
	Run:        run,
	Doc:        "test harness for de-referencing",
	Requires:   []*analysis.Analyzer{buildssa.Analyzer},
	ResultType: reflect.TypeOf([]*ssa.MakeInterface{}),
}

func run(pass *analysis.Pass) (interface{}, error) {
	in := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	var result []*ssa.MakeInterface
	for _, fn := range in.SrcFuncs {
		for _, b := range fn.Blocks {
			for _, i := range b.Instrs {
				if mi, ok := i.(*ssa.MakeInterface); ok {
					result = append(result, mi)
				}
			}
		}
	}

	return result, nil
}

func TestDereference(t *testing.T) {
	dir := analysistest.TestData()

	testCases := []struct {
		desc    string
		pattern string
		want    string
	}{
		{
			desc:    "pointer to a struct",
			pattern: "pointer_to_struct",
			want:    "foo",
		},
		{
			desc:    "struct",
			pattern: "struct",
			want:    "foo",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			r := analysistest.Run(t, dir, testAnalyzer, fmt.Sprintf("dereference/%s", tt.pattern))
			if len(r) != 1 {
				t.Fatalf("Got len(result) == %d, want 1", len(r))
			}

			mis, ok := r[0].Result.([]*ssa.MakeInterface)
			if !ok {
				t.Fatalf("Got result of type %T, wanted []*ssa.MakeInterface", mis)
			}

			got := Dereference(mis[0].X.Type())
			if !strings.HasSuffix(got.String(), tt.want) {
				t.Fatalf("Got %s, want it to have a suffix of: %s", got, tt.want)
			}
		})
	}
}
