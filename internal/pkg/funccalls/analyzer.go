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

// Package funcs contains an analyzer that performs identification of
// sink and sanitizer function definitions and calls. It also identifies
// excluded functions.
package funccalls

import (
	"reflect"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name:  "funccalls",
	Run:   runTest,
	Flags: config.FlagSet,
	Doc: `The funccalls analyzer finds calls to sink and sanitizer functions.
Found calls are returned as a result that can be consumed by other analyzers.`,
	Requires:   []*analysis.Analyzer{buildssa.Analyzer},
	ResultType: reflect.TypeOf(new(ResultType)).Elem(),
}

// ResultType can be used to check if a given call is a call to a sink or sanitizer.
type ResultType struct {
	sinkCalls      map[*ssa.Call]bool
	sanitizerCalls map[*ssa.Call]bool
}

func (rt ResultType) IsSink(c *ssa.Call) bool {
	return rt.sinkCalls[c]
}

func (rt ResultType) IsSanitizer(c *ssa.Call) bool {
	return rt.sanitizerCalls[c]
}

func runTest(pass *analysis.Pass) (interface{}, error) {
	in := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	rt := ResultType{
		sinkCalls:      map[*ssa.Call]bool{},
		sanitizerCalls: map[*ssa.Call]bool{},
	}

	for _, f := range in.SrcFuncs {
		if conf.IsExcluded(utils.DecomposeFunction(f)) {
			continue
		}
		for _, b := range f.Blocks {
			for _, i := range b.Instrs {
				c, ok := i.(*ssa.Call)
				if !ok {
					continue
				}
				callee := c.Call.StaticCallee()
				if callee == nil {
					continue
				}
				switch {
				case conf.IsSink(utils.DecomposeFunction(callee)):
					rt.sinkCalls[c] = true
					reportCall(pass, c, callee, "sink")
				case conf.IsSanitizer(utils.DecomposeFunction(callee)):
					rt.sanitizerCalls[c] = true
					reportCall(pass, c, callee, "sanitizer")
				}
			}
		}
	}

	return rt, nil
}

func reportCall(pass *analysis.Pass, c *ssa.Call, f *ssa.Function, kind string) {
	pass.Reportf(c.Pos(), "call to %s function %s", kind, f.RelString(pass.Pkg))
}
