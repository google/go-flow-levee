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

package sourcetype

import (
	"go/types"
	"reflect"

	"github.com/google/go-flow-levee/internal/pkg/config"
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
	Requires:   []*analysis.Analyzer{buildssa.Analyzer},
	ResultType: reflect.TypeOf(new(sourceClassifier)),
	FactTypes:  []analysis.Fact{new(typeDeclFact), new(fieldDeclFact)},
}

func run(pass *analysis.Pass) (interface{}, error) {
	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	// Members contains all named entities
	for _, mem := range ssaInput.Pkg.Members {
		if ssaType, ok := mem.(*ssa.Type); ok {
			if conf.IsSource(ssaType.Type()) {
				pass.ExportObjectFact(ssaType.Object(), &typeDeclFact{})
				if under, ok := ssaType.Type().Underlying().(*types.Struct); ok {
					for i := 0; i < under.NumFields(); i++ {
						fld := under.Field(i)
						if conf.IsSourceField(ssaType.Type(), fld) {
							if fld.Pkg() == pass.Pkg {
								pass.ExportObjectFact(fld, &fieldDeclFact{})
							}
						}
					}
				}
			}
		}
	}

	classifier := &sourceClassifier{pass.AllObjectFacts()}
	return classifier, nil
}
