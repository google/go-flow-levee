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
	"github.com/google/go-flow-levee/internal/pkg/fieldtags"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

// ResultType is a map telling whether an object (really a function) is a field propagator.
type ResultType = FieldPropagators

type isFieldPropagator struct{}

func (i isFieldPropagator) AFact() {}

func (i isFieldPropagator) String() string {
	return "field propagator identified"
}

var Analyzer = &analysis.Analyzer{
	Name: "fieldpropagator",
	Doc: `This analyzer identifies field propagators.

A field propagator is a function that returns a source field.`,
	Flags:      config.FlagSet,
	Run:        run,
	Requires:   []*analysis.Analyzer{buildssa.Analyzer, fieldtags.Analyzer},
	ResultType: reflect.TypeOf(new(ResultType)).Elem(),
	FactTypes:  []analysis.Fact{new(isFieldPropagator)},
}

type analysisState struct {
	pass     *analysis.Pass
	conf     *config.Config
	ssaInput *buildssa.SSA
	tf       fieldtags.TaggedFields
}

func run(pass *analysis.Pass) (interface{}, error) {
	taggedFields := pass.ResultOf[fieldtags.Analyzer].(fieldtags.ResultType)
	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	a := &analysisState{
		pass:     pass,
		conf:     conf,
		ssaInput: ssaInput,
		tf:       taggedFields,
	}
	a.run()

	isFieldPropagator := map[types.Object]bool{}
	for _, f := range pass.AllObjectFacts() {
		isFieldPropagator[f.Object] = true
	}
	return FieldPropagators(isFieldPropagator), nil
}

func (a *analysisState) run() {
	ssaProg := a.ssaInput.Pkg.Prog
	for _, mem := range a.ssaInput.Pkg.Members {
		ssaType, ok := mem.(*ssa.Type)
		if !ok || !a.conf.IsSource(ssaType.Type()) {
			continue
		}
		methods := ssaProg.MethodSets.MethodSet(ssaType.Type())
		for i := 0; i < methods.Len(); i++ {
			meth := ssaProg.MethodValue(methods.At(i))
			a.analyzeBlocks(meth)
		}
	}
}

func (a *analysisState) analyzeBlocks(meth *ssa.Function) {
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
		a.analyzeResults(meth, ret.Results)
	}
}

func (a *analysisState) analyzeResults(meth *ssa.Function, results []ssa.Value) {
	for _, r := range results {
		fa, ok := fieldAddr(r)
		if !ok {
			continue
		}
		if a.conf.IsSourceFieldAddr(fa) || a.tf.IsSource(fa) {
			a.pass.ExportObjectFact(meth.Object(), &isFieldPropagator{})
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

type FieldPropagators map[types.Object]bool

// Contains determines whether a call is a field propagator, i.e. whether it is contained by the FieldPropagators map.
func (f FieldPropagators) Contains(c *ssa.Call) bool {
	cf, ok := c.Call.Value.(*ssa.Function)
	return ok && f[cf.Object()]
}
