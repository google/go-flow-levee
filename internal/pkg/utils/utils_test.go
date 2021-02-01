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
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/buildssa"
)

type testAnalyzerResult struct {
	makeInterface []*ssa.MakeInterface
	fieldAddr     []*ssa.FieldAddr
}

var testAnalyzer = &analysis.Analyzer{
	Name:       "domination",
	Run:        run,
	Doc:        "test harness for de-referencing",
	Requires:   []*analysis.Analyzer{buildssa.Analyzer},
	ResultType: reflect.TypeOf(testAnalyzerResult{}),
}

func run(pass *analysis.Pass) (interface{}, error) {
	in := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	var result testAnalyzerResult
	for _, fn := range in.SrcFuncs {
		for _, b := range fn.Blocks {
			for _, i := range b.Instrs {
				switch v := i.(type) {
				case *ssa.MakeInterface:
					result.makeInterface = append(result.makeInterface, v)
				case *ssa.FieldAddr:
					result.fieldAddr = append(result.fieldAddr, v)
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

			mis, ok := r[0].Result.(testAnalyzerResult)
			if !ok {
				t.Fatalf("Got result of type %T, wanted testAnalyzerResult", mis)
			}

			got := Dereference(mis.makeInterface[0].X.Type())
			if !strings.HasSuffix(got.String(), tt.want) {
				t.Fatalf("Got %s, want it to have a suffix of: %s", got, tt.want)
			}
		})
	}
}

func TestDecomposeField(t *testing.T) {
	dir := analysistest.TestData()

	testCases := []struct {
		pattern   string
		typePath  string
		typeName  string
		fieldName string
	}{
		{
			pattern:   "regular",
			typePath:  "fields/regular",
			typeName:  "foo",
			fieldName: "name",
		},
		{
			pattern:   "embedded",
			typePath:  "fields/embedded",
			typeName:  "bar",
			fieldName: "foo",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.pattern, func(t *testing.T) {
			r := analysistest.Run(t, dir, testAnalyzer, fmt.Sprintf("fields/%s", tt.pattern))

			if len(r) != 1 {
				t.Fatalf("Got len(result) == %d, want 1", len(r))
			}

			if r[0].Err != nil {
				t.Fatalf("Got unexpected error: %s", r[0].Err)
			}

			res := r[0].Result.(testAnalyzerResult)

			fa := res.fieldAddr[0]

			typePath, typeName, fieldName := DecomposeField(fa.X.Type(), fa.Field)
			if typePath != tt.typePath {
				t.Fatalf("Got typePath %s, want %s", typePath, tt.typePath)
			}
			if typeName != tt.typeName {
				t.Fatalf("Got typeName %s, want %s", typeName, tt.typeName)
			}
			if fieldName != tt.fieldName {
				t.Fatalf("Got fieldName %s, want %s", fieldName, tt.fieldName)
			}
		})
	}
}
