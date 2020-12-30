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
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

// ResultType is a map telling whether an object (really a function) is a field propagator.
type ResultType map[types.Object]bool

// IsFieldPropagator determines whether a call is a field propagator.
func (r ResultType) IsFieldPropagator(c *ssa.Call) bool {
	cf, ok := c.Call.Value.(*ssa.Function)
	return ok && r[cf.Object()]
}

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

func run(pass *analysis.Pass) (interface{}, error) {
	taggedFields := pass.ResultOf[fieldtags.Analyzer].(fieldtags.ResultType)
	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	ssaProg := ssaInput.Pkg.Prog
	for _, mem := range ssaInput.Pkg.Members {
		ssaType, ok := mem.(*ssa.Type)
		if !ok || !conf.IsSourceType(utils.DecomposeType(ssaType.Type())) {
			continue
		}
		for _, meth := range methods(ssaProg, ssaType.Type()) {
			analyzeBlocks(pass, conf, taggedFields, meth)
		}
	}

	isFieldPropagator := map[types.Object]bool{}
	for _, f := range pass.AllObjectFacts() {
		isFieldPropagator[f.Object] = true
	}
	return ResultType(isFieldPropagator), nil
}

func methods(ssaProg *ssa.Program, t types.Type) []*ssa.Function {
	var methods []*ssa.Function
	// The method sets of T and *T are disjoint.
	// In Go code, a variable of type T has access to all
	// the methods of type *T due to an implicit address operation.
	methods = append(methods, methodValues(ssaProg, t)...)
	return append(methods, methodValues(ssaProg, types.NewPointer(t))...)
}

func methodValues(ssaProg *ssa.Program, t types.Type) []*ssa.Function {
	var methodValues []*ssa.Function
	mset := ssaProg.MethodSets.MethodSet(t)
	for i := 0; i < mset.Len(); i++ {
		if meth := ssaProg.MethodValue(mset.At(i)); meth != nil {
			methodValues = append(methodValues, meth)
		}
	}
	return methodValues
}

func analyzeBlocks(pass *analysis.Pass, conf *config.Config, tf fieldtags.ResultType, meth *ssa.Function) {
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
		analyzeResults(pass, conf, tf, meth, ret.Results)
	}
}

func analyzeResults(pass *analysis.Pass, conf *config.Config, tf fieldtags.ResultType, meth *ssa.Function, results []ssa.Value) {
	for _, r := range results {
		fa, ok := fieldAddr(r)
		if !ok {
			continue
		}

		xt, field := fa.X.Type(), fa.Field
		if conf.IsSourceField(utils.DecomposeField(xt, field)) || tf.IsSourceField(xt, field) {
			pass.ExportObjectFact(meth.Object(), &isFieldPropagator{})
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
