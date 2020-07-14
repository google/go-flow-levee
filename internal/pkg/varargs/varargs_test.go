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

package varargs

import (
	"go/types"
	"reflect"
	"testing"

	"github.com/google/go-flow-levee/internal/pkg/source"

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

func (c *testConfig) IsSource(t types.Type) bool {
	return true
}

func (c *testConfig) IsSanitizer(call *ssa.Call) bool {
	return false
}

func (c *testConfig) IsPropagator(call *ssa.Call) bool {
	return false
}

func (c *testConfig) IsSourceFieldAddr(field *ssa.FieldAddr) bool {
	return false
}

type analyzerResult struct {
	allocations []*ssa.Alloc
	calls       []*ssa.Call
	store       []*ssa.Store
}

var testAnalyzer = &analysis.Analyzer{
	Name:       "varargs",
	Run:        run,
	Doc:        "test harness for varargs",
	Requires:   []*analysis.Analyzer{buildssa.Analyzer},
	ResultType: reflect.TypeOf(analyzerResult{}),
}

func run(pass *analysis.Pass) (interface{}, error) {
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
				case *ssa.Store:
					result.store = append(result.store, v)
				}
			}
		}
	}

	return result, nil
}

func TestVarargs(t *testing.T) {
	var testCases = []struct {
		pattern               string
		expectVararg          bool
		callUnderTestIdx      int
		allocUnderTestIdx     int
		wantConnectionToAlloc bool
		wantStores            int
	}{
		{
			pattern:               "base",
			wantConnectionToAlloc: true,
			wantStores:            1,
		},
		{
			pattern:               "empty",
			wantConnectionToAlloc: true,
			wantStores:            1,
		},
		{
			pattern:               "multiple",
			wantConnectionToAlloc: true,
			wantStores:            2,
		},
		{
			pattern:    "no-connection-to-source",
			wantStores: 1,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.pattern, func(t *testing.T) {
			dir := analysistest.TestData()
			r := analysistest.Run(t, dir, testAnalyzer, tt.pattern)
			if len(r) != 1 {
				t.Fatalf("Got %d results, wanted one", len(r))
			}

			a, ok := r[0].Result.(analyzerResult)
			if !ok {
				t.Fatalf("Got result of type %T, wanted analyzerResult", a)
			}

			got := New(a.calls[tt.callUnderTestIdx])
			if got == nil && tt.expectVararg {
				t.Fatal("Got nil wanted varargs")
			}

			if got == nil {
				return
			}

			for i := 0; i < tt.wantStores; i++ {
				if got.stores[i] != a.store[i] {
					t.Fatalf("Expected %v == %v for store #%d. Referres: %v", got.stores[i], a.store[i], i, got.stores[i].Referrers())
				}
			}

			s := source.New(a.allocations[tt.allocUnderTestIdx], &testConfig{})
			if s == nil {
				t.Fatal("Expected a source got got nil at a.allocations[0]")
			}

			if !got.ReferredBy(s) && tt.wantConnectionToAlloc {
				t.Fatalf("Expected source %v to refer to vararg %v", s, got)
			}
		})
	}
}
