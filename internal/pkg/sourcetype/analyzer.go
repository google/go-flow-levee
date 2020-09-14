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

// Package sourcetpye handles identification of source types and fields at the type declaration.
// This can be consumed downstream, e.g., by the sources package to identify source data at instantiation.
// This package concerns itself with ssa.Member and types.Object, as opposed to ssa.Value and ssa.Instruction more typically used in other analysis packages.
package sourcetype

import (
	"fmt"
	"go/types"
	"reflect"
	"sort"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/fieldtags"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

type typeDeclFact struct{}

func (t typeDeclFact) AFact() {
}

func (t typeDeclFact) String() string {
	return "source type"
}

type fieldDeclFact struct{}

func (f fieldDeclFact) AFact() {
}

func (f fieldDeclFact) String() string {
	return "source field"
}

type sourceClassifier struct {
	passObjFacts []analysis.ObjectFact
}

func (s sourceClassifier) IsSource(named *types.Named) bool {
	if named == nil {
		return false
	}

	for _, fct := range s.passObjFacts {
		if fct.Object == named.Obj() {
			return true
		}
	}
	return false
}

func (s sourceClassifier) IsSourceField(v *types.Var) bool {
	for _, fct := range s.passObjFacts {
		if fct.Object == v {
			return true
		}
	}
	return false
}

var Analyzer = &analysis.Analyzer{
	Name:       "sourcetypes",
	Doc:        "This analyzer identifies types.Types values which contain dataflow sources.",
	Flags:      config.FlagSet,
	Run:        run,
	Requires:   []*analysis.Analyzer{buildssa.Analyzer, fieldtags.Analyzer},
	ResultType: reflect.TypeOf(new(sourceClassifier)),
	FactTypes:  []analysis.Fact{new(typeDeclFact), new(fieldDeclFact)},
}

var Report bool

func run(pass *analysis.Pass) (interface{}, error) {
	taggedFields := pass.ResultOf[fieldtags.Analyzer].(fieldtags.ResultType)
	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	// Members contains all named entities
	for _, mem := range ssaInput.Pkg.Members {
		if ssaType, ok := mem.(*ssa.Type); ok &&
			(conf.IsSource(ssaType.Type()) || hasTaggedField(mem, taggedFields)) {
			exportSourceFacts(pass, ssaType, conf, taggedFields)
		}
	}

	classifier := &sourceClassifier{pass.AllObjectFacts()}
	if Report {
		makeReport(classifier, pass)
	}

	return classifier, nil
}

func hasTaggedField(mem ssa.Member, taggedFields fieldtags.ResultType) bool {
	n, ok := mem.Type().(*types.Named)
	if !ok {
		return false
	}
	s, ok := n.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	var has bool
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		has = has || taggedFields.IsSource(f)
	}
	return has
}

func exportSourceFacts(pass *analysis.Pass, ssaType *ssa.Type, conf *config.Config, taggedFields fieldtags.ResultType) {
	pass.ExportObjectFact(ssaType.Object(), &typeDeclFact{})
	if under, ok := ssaType.Type().Underlying().(*types.Struct); ok {
		for i := 0; i < under.NumFields(); i++ {
			fld := under.Field(i)
			if fld.Pkg() != pass.Pkg {
				continue
			}
			if conf.IsSourceField(ssaType.Type(), fld) || taggedFields.IsSource(fld) {
				pass.ExportObjectFact(fld, &fieldDeclFact{})
			}
		}
	}
}

func makeReport(classifier *sourceClassifier, pass *analysis.Pass) {
	// Aggregate diagnostics first in order to sort report by position.
	var diags []analysis.Diagnostic
	for _, objFact := range classifier.passObjFacts {
		// A pass should only report within its package.
		if objFact.Object.Pkg() == pass.Pkg {
			diags = append(diags, analysis.Diagnostic{
				Pos:     objFact.Object.Pos(),
				Message: fmt.Sprintf("%v: %v", objFact.Fact, objFact.Object.Name()),
			})
		}
	}
	sort.Slice(diags, func(i, j int) bool { return diags[i].Pos < diags[j].Pos })
	for _, d := range diags {
		pass.Reportf(d.Pos, d.Message)
	}
}
