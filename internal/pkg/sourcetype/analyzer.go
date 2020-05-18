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

	"github.com/google/go-flow-levee/internal/pkg/config"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

type sourceClassifier struct {
	// ssaTypes maps the package ssa.Member to the corresponding types.Type
	// This may be many-to-one if the Member is a type alias
	ssaTypes map[*ssa.Type]types.Type

	// sources maps the source type to its respective source fields.
	sources map[types.Type][]*types.Var
}

func (sc *sourceClassifier) add(ssaType *ssa.Type, sourceFields []*types.Var) {
	sc.ssaTypes[ssaType] = ssaType.Type()
	sc.sources[ssaType.Type()] = sourceFields
}

func (sc *sourceClassifier) makeReport(pass *analysis.Pass) {
	// Alias types can double-report field identification.
	// Don't report the same underlying types.Type more than once.
	alreadyReported := make(map[types.Type]bool)

	for ssaType, typ := range sc.ssaTypes {
		pass.Reportf(ssaType.Pos(), "source type declaration identified: %v", typ)
		if !alreadyReported[typ] {
			alreadyReported[typ] = true
			for _, f := range sc.sources[typ] {
				pass.Reportf(f.Pos(), "source field declaration identified: %v", f)
			}

		}
	}
}

func newSourceClassifier() *sourceClassifier {
	return &sourceClassifier{
		sources:  make(map[types.Type][]*types.Var),
		ssaTypes: make(map[*ssa.Type]types.Type),
	}
}

var Analyzer = &analysis.Analyzer{
	Name:     "sourcetypes",
	Doc:      "This analyzer identifies types.Types values which contain dataflow sources.",
	Flags:    config.FlagSet,
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	sc := newSourceClassifier()
	// Members contains all named entities
	for _, mem := range ssaInput.Pkg.Members {
		if ssaType, ok := mem.(*ssa.Type); ok {
			if conf.IsSource(ssaType.Type()) {
				var sourceFields []*types.Var
				// If the mamber names a struct declaration, examine the fields for additional tagging.
				if under, ok := ssaType.Type().Underlying().(*types.Struct); ok {
					for i := 0; i < under.NumFields(); i++ {
						fld := under.Field(i)
						if conf.IsSourceField(ssaType.Type(), fld) {
							sourceFields = append(sourceFields, fld)
						}
					}
				}

				sc.add(ssaType, sourceFields)
			}
		}
	}

	sc.makeReport(pass)

	return nil, nil
}
