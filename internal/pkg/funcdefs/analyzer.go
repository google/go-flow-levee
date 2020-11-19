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

// Package funcdefs identifies function definitions that define sinks or sanitizers,
// or functions that are excluded from analysis.
package funcdefs

import (
	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name:     "funcdefs",
	Doc:      `The funcdefs analyzer identifies sinks and sanitizers that match the provided configuration.`,
	Flags:    config.FlagSet,
	Run:      run,
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	builtSSA := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

	for _, f := range builtSSA.SrcFuncs {
		switch {
		case conf.IsExcluded(utils.DecomposeFunction(f)):
			reportDef(pass, f, "excluded from analysis")
		case conf.IsSink(utils.DecomposeFunction(f)):
			reportDef(pass, f, "a sink")
		case conf.IsSanitizer(utils.DecomposeFunction(f)):
			reportDef(pass, f, "a sanitizer")
		}
	}

	return nil, nil
}

func reportDef(pass *analysis.Pass, f *ssa.Function, what string) {
	pass.Reportf(f.Pos(), "function %s is %s", f.RelString(pass.Pkg), what)
}
