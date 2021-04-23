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

package summary

import (
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

func Test(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyzer, "./...")
}

var analyzer = &analysis.Analyzer{
	Name:     "summarytest",
	Doc:      "This analyzer is a test harness for functions in the summary package.",
	Run:      run,
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	builtSSA := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

	// produce reports for sigTypeString test cases
	for _, f := range builtSSA.SrcFuncs {
		if f.Name() == "testStaticFuncName" || f.Name() == "testFuncNameWithoutReceiver" {
			continue
		}
		sig := f.Type().(*types.Signature)
		pass.Reportf(f.Pos(), sigTypeString(sig))
	}

	// produce reports for testFuncName test cases
	for _, f := range builtSSA.SrcFuncs {
		if f.Name() != "testStaticFuncName" {
			continue
		}
		for _, instr := range f.Blocks[0].Instrs {
			if call, ok := instr.(*ssa.Call); ok {
				pass.Reportf(call.Pos(), staticFuncName(call))
			}
		}
	}

	// produce reports for testFuncNameWithoutReceiver test cases
	for _, f := range builtSSA.SrcFuncs {
		if f.Name() != "testFuncNameWithoutReceiver" {
			continue
		}
		for _, instr := range f.Blocks[0].Instrs {
			if call, ok := instr.(*ssa.Call); ok {
				pass.Reportf(call.Pos(), methodNameWithoutReceiver(call))
			}
		}
	}

	return nil, nil
}
