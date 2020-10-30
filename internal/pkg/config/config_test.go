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

package config

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

var testAnalyzer = &analysis.Analyzer{
	Name:     "config",
	Run:      runTest,
	Doc:      "test harness for the logic related to config",
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func runTest(pass *analysis.Pass) (interface{}, error) {
	in := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

	conf, err := ReadConfig()
	if err != nil {
		return nil, err
	}

	for _, f := range in.SrcFuncs {
		if conf.IsSink(DecomposeFunction(f)) {
			pass.Reportf(f.Pos(), "sink")
		}
		if conf.IsExcluded(DecomposeFunction(f)) {
			pass.Reportf(f.Pos(), "excluded")
		}
		for _, b := range f.Blocks {
			for _, i := range b.Instrs {
				if c, ok := i.(*ssa.Call); ok && conf.IsSink(DecomposeFunction(c.Call.StaticCallee())) {
					pass.Reportf(i.Pos(), "sink call")
				}
			}
		}
	}

	return nil, nil
}

func TestConfig(t *testing.T) {
	testdata := analysistest.TestData()
	if err := FlagSet.Set("config", filepath.Join(testdata, "test-config.json")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"core", "crosspkg", "exclusion", "notcore"} {
		analysistest.Run(t, testdata, testAnalyzer, filepath.Join(testdata, "src/example.com", p))
	}
	for _, p := range []string{"core", "exclusion"} {
		analysistest.Run(t, testdata, testAnalyzer, filepath.Join(testdata, "src/notexample.com", p))
	}
}
