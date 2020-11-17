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
		// TODO: refactor
		case conf.IsExcluded(utils.DecomposeFunction(f)):
			pass.Reportf(f.Pos(), "%s is excluded from analysis", f.Name())
		case conf.IsSink(utils.DecomposeFunction(f)):
			pass.Reportf(f.Pos(), "%s is a sink", f.Name())
		case conf.IsSanitizer(utils.DecomposeFunction(f)):
			pass.Reportf(f.Pos(), "%s is a sanitizer", f.Name())
		}
	}

	return nil, nil
}
