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

package source

import (
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"reflect"
	"regexp"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

type testConfig struct {
	propagatorsPattern string
	fieldsPattern      string
	sanitizerPattern   string
}

func (c *testConfig) IsSanitizer(call *ssa.Call) bool {
	match, _ := regexp.MatchString(c.sanitizerPattern, call.String())
	return match
}

func (c *testConfig) IsPropagator(call *ssa.Call) bool {
	match, _ := regexp.MatchString(c.propagatorsPattern, call.String())
	return match
}

func (c *testConfig) IsSourceFieldAddr(field *ssa.FieldAddr) bool {
	match, _ := regexp.MatchString(c.fieldsPattern, utils.FieldName(field))
	return match
}

type analyzerResult struct {
	allocations []*ssa.Alloc
	calls       []*ssa.Call
	fieldAddr   []*ssa.FieldAddr
	store       []*ssa.Store
}

var testAnalyzer = &analysis.Analyzer{
	Name:       "source",
	Run:        runTest,
	Doc:        "test harness for the logic related to sources",
	Requires:   []*analysis.Analyzer{buildssa.Analyzer},
	ResultType: reflect.TypeOf(analyzerResult{}),
}

func runTest(pass *analysis.Pass) (interface{}, error) {
	in := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	var result analyzerResult
	for _, fn := range in.SrcFuncs {
		for _, b := range fn.Blocks {
			for _, i := range b.Instrs {
				switch v := i.(type) {
				case *ssa.Alloc:
					result.allocations = append(result.allocations, v)
				case *ssa.Call:
					result.calls = append(result.calls, v)
				case *ssa.FieldAddr:
					result.fieldAddr = append(result.fieldAddr, v)
				case *ssa.Store:
					result.store = append(result.store, v)
				}
			}
		}
	}

	return result, nil
}

func TestSource(t *testing.T) {
	dir := analysistest.TestData()
	config := &testConfig{
		propagatorsPattern: "propagator",
		sanitizerPattern:   "sanitizer",
		fieldsPattern:      "name",
	}

	testCases := []struct {
		pattern                      string
		config                       *testConfig
		wantCallConnectionIdx        []int
		wantFieldAccessConnectionIdx []int
		wantStoreConnectionIdx       []int
	}{
		{
			pattern:                      "allocation",
			config:                       config,
			wantStoreConnectionIdx:       []int{0},
			wantFieldAccessConnectionIdx: []int{0},
		},
		{
			pattern:                      "propagation",
			config:                       config,
			wantCallConnectionIdx:        []int{0},
			wantFieldAccessConnectionIdx: []int{0},
		},
		{
			pattern:                      "sanitization",
			config:                       config,
			wantFieldAccessConnectionIdx: []int{0},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.pattern, func(t *testing.T) {
			r := analysistest.Run(t, dir, testAnalyzer, tt.pattern)
			if len(r) != 1 {
				t.Fatalf("Got len(result) == %d, want 1", len(r))
			}

			a, ok := r[0].Result.(analyzerResult)
			if !ok {
				t.Fatalf("Got result of type %T, wanted analyzerResult", a)
			}

			if len(a.allocations) == 0 {
				t.Fatal("Got 0 allocations want at least one")
			}

			src := New(a.allocations[0], tt.config)
			t.Logf("Testing source:\n%v", src)

			for i := 0; i < len(tt.wantCallConnectionIdx); i++ {
				if !src.HasPathTo(a.calls[i]) {
					t.Errorf("Expected\n%v to have a path to %v", src, a.calls[i])
				}
			}

			for i := 0; i < len(tt.wantStoreConnectionIdx); i++ {
				if !src.HasPathTo(a.store[i]) {
					t.Errorf("Expected\n%v to have a path to %v", src, a.store[i])
				}
			}

			for i := 0; i < len(tt.wantFieldAccessConnectionIdx); i++ {
				if !src.HasPathTo(a.fieldAddr[i]) {
					t.Fatalf("Expected\n%v to have a path to %v", src, a.fieldAddr[i])
				}
			}
		})
	}
}
