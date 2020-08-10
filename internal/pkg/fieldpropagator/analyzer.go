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

// Package fieldpropagator implements identification of field propagators.
// A field propagator is a function that returns a source field.
package fieldpropagator

import (
	"go/types"
	"reflect"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

// ResultType is a map telling whether an object (really a function) is a field propagator.
type ResultType = map[types.Object]bool

type isFieldPropagator struct{}

func (i isFieldPropagator) AFact() {}

func (i isFieldPropagator) String() string {
	return "field propagator identified"
}

// For testing purposes.
// Specifically, this makes the analyzer report field propagators it finds,
// which allows us to use analysistest "want" expectations.
var reporting bool

var Analyzer = &analysis.Analyzer{
	Name:       "fieldpropagator",
	Doc:        "This analyzer identifies field propagators.",
	Flags:      config.FlagSet,
	Run:        run,
	Requires:   []*analysis.Analyzer{buildssa.Analyzer},
	ResultType: reflect.TypeOf(new(ResultType)).Elem(),
	FactTypes:  []analysis.Fact{new(isFieldPropagator)},
}

func run(pass *analysis.Pass) (interface{}, error) {
	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	ssaProg := ssaInput.Pkg.Prog
	for _, mem := range ssaInput.Pkg.Members {
		ssaType, ok := mem.(*ssa.Type)
		if !ok || !conf.IsSource(ssaType.Type()) {
			continue
		}
		methods := ssaProg.MethodSets.MethodSet(ssaType.Type())
		for i := 0; i < methods.Len(); i++ {
			meth := ssaProg.MethodValue(methods.At(i))
			analyzeBlocks(pass, conf, meth)
		}
	}

	isFieldPropagator := map[types.Object]bool{}
	for _, f := range pass.AllObjectFacts() {
		isFieldPropagator[f.Object] = true
	}
	return isFieldPropagator, nil
}

func analyzeBlocks(pass *analysis.Pass, conf *config.Config, meth *ssa.Function) {
	// Function does not return anything
	if res := meth.Signature.Results(); res == nil || (*res).Len() == 0 {
		return
	}
	for _, b := range meth.Blocks {
		if len(b.Instrs) == 0 {
			continue
		}
		lastInstr := b.Instrs[len(b.Instrs)-1]
		ret, ok := lastInstr.(*ssa.Return)
		if !ok {
			continue
		}
		analyzeResults(pass, conf, meth, ret.Results)
	}
}

func analyzeResults(pass *analysis.Pass, conf *config.Config, meth *ssa.Function, results []ssa.Value) {
	for _, r := range results {
		fa, ok := fieldAddr(r)
		if !ok {
			continue
		}
		if conf.IsSourceFieldAddr(fa) {
			pass.ExportObjectFact(meth.Object(), &isFieldPropagator{})
			if reporting {
				pass.Reportf(meth.Pos(), "field propagator identified")
			}
		}
	}
}

func fieldAddr(x ssa.Value) (*ssa.FieldAddr, bool) {
	switch t := x.(type) {
	case *ssa.FieldAddr:
		return t, true
	case *ssa.UnOp:
		return fieldAddr(t.X)
	}
	return nil, false
}
