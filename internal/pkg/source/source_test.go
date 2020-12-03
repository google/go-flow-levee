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
	"regexp"
	"testing"

	"github.com/google/go-flow-levee/internal/pkg/fieldtags"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/buildssa"
)

type testConfig struct {
	sourcePattern    string
	fieldsPattern    string
	sanitizerPattern string
	sinkPattern      string
}

func (c *testConfig) IsSourceType(path, typename string) bool {
	match, _ := regexp.MatchString(c.sourcePattern, typename)
	return match
}

func (c *testConfig) IsSanitizer(path, recv, name string) bool {
	match, _ := regexp.MatchString(c.sanitizerPattern, name)
	return match
}

func (c *testConfig) IsSourceField(path, typeName, fieldName string) bool {
	match, _ := regexp.MatchString(c.fieldsPattern, fieldName)
	return match
}

func (c *testConfig) IsSink(path, recv, name string) bool {
	match, _ := regexp.MatchString(c.sinkPattern, name)
	return match
}

func (c *testConfig) IsExcluded(path, recv, name string) bool {
	return false
}

var testAnalyzer = &analysis.Analyzer{
	Name:     "source",
	Run:      runTest,
	Doc:      "test harness for the logic related to sources",
	Requires: []*analysis.Analyzer{buildssa.Analyzer, fieldtags.Analyzer},
}

func runTest(pass *analysis.Pass) (interface{}, error) {
	in := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	taggedFields := pass.ResultOf[fieldtags.Analyzer].(fieldtags.ResultType)
	config := &testConfig{
		sourcePattern:    "foo",
		sanitizerPattern: "sanitizer",
		fieldsPattern:    "name",
		sinkPattern:      "sink",
	}

	sm := identify(config, in, taggedFields)
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
