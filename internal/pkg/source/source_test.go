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
	"go/types"
	"regexp"
	"testing"

	"github.com/google/go-flow-levee/internal/pkg/utils"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

type testConfig struct {
	sourcePattern    string
	fieldsPattern    string
	sanitizerPattern string
	sinkPattern      string
}

func (c *testConfig) IsSource(t types.Type) bool {
	d := utils.Dereference(t)
	_, ok := d.(*types.Named)
	if !ok {
		return false
	}

	match, _ := regexp.MatchString(c.sourcePattern, d.String())
	return match
}

func (c *testConfig) IsSanitizer(call *ssa.Call) bool {
	match, _ := regexp.MatchString(c.sanitizerPattern, call.String())
	return match
}

func (c *testConfig) IsSourceFieldAddr(field *ssa.FieldAddr) bool {
	match, _ := regexp.MatchString(c.fieldsPattern, utils.FieldName(field))
	return match
}

func (c *testConfig) IsSinkFunction(f *ssa.Function) bool {
	match, _ := regexp.MatchString(c.sinkPattern, f.Name())
	return match
}

func (c *testConfig) IsExcluded(path string, recv string, name string) bool {
	return false
}

var testAnalyzer = &analysis.Analyzer{
	Name:     "source",
	Run:      runTest,
	Doc:      "test harness for the logic related to sources",
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func runTest(pass *analysis.Pass) (interface{}, error) {
	in := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	config := &testConfig{
		sourcePattern:    `\.foo`,
		sanitizerPattern: "sanitizer",
		fieldsPattern:    "name",
		sinkPattern:      "sink",
	}

	sm := identify(config, in)
	for _, f := range sm {
		for _, s := range f {
			if s.String() != "" {
				pass.Reportf(s.node.Pos(), s.String())
			}
		}
	}

	return nil, nil
}

func TestSource(t *testing.T) {
	dir := analysistest.TestData()
	testCases := []struct {
		pattern string
	}{
		{
			pattern: "allocation",
		},
		{
			pattern: "propagation",
		},
		{
			pattern: "sanitization",
		},
		{
			pattern: "domination",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.pattern, func(t *testing.T) {
			analysistest.Run(t, dir, testAnalyzer, tt.pattern)
		})
	}
}
