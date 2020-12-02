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

// Package fieldtags defines an analyzer that identifies struct fields identified
// as sources via a field tag.
package fieldtags

import (
	"go/ast"
	"go/types"
	"reflect"
	"strings"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/ssa"
)

// ResultType is a map from types.Object to bool.
// It can be used to determine whether a field is a tagged Source field.
type ResultType map[types.Object]bool

var Analyzer = &analysis.Analyzer{
	Name: "fieldtags",
	Doc:  "This analyzer identifies Source fields based on their tags.",
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
	ResultType: reflect.TypeOf(new(ResultType)).Elem(),
	FactTypes:  []analysis.Fact{new(isTaggedField)},
}

type isTaggedField struct{}

func (i isTaggedField) AFact() {}

func (i isTaggedField) String() string {
	return "tagged field"
}

func run(pass *analysis.Pass) (interface{}, error) {
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	inspectResult := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	taggedFields := map[types.Object]bool{}

	nodeFilter := []ast.Node{
		(*ast.StructType)(nil),
	}

	inspectResult.Preorder(nodeFilter, func(n ast.Node) {
		s, ok := n.(*ast.StructType)
		if !ok || s.Fields == nil {
			return
		}
		for _, f := range (*s).Fields.List {
			if f.Tag == nil || len(f.Names) == 0 || !conf.IsSourceFieldTag(f.Tag.Value) {
				continue
			}
			fNames := make([]string, len(f.Names))
			for i, ident := range f.Names {
				fNames[i] = ident.Name
				taggedFields[pass.TypesInfo.ObjectOf(ident)] = true
				pass.ExportObjectFact(pass.TypesInfo.ObjectOf(ident), &isTaggedField{})
			}
			pass.Reportf(f.Pos(), "tagged field: %s", strings.Join(fNames, ", "))
		}
	})

	// return all facts accumulated down the current path in the dependency graph
	result := map[types.Object]bool{}
	for _, f := range pass.AllObjectFacts() {
		result[f.Object] = true
	}

	return ResultType(result), nil
}

// IsSourceFieldAddr determines whether a ssa.FieldAddr is a source, that is whether it refers to a field previously identified as a source.
func (r ResultType) IsSourceFieldAddr(fa *ssa.FieldAddr) bool {
	// incantation plundered from the docstring for ssa.FieldAddr.Field
	field := fa.X.Type().Underlying().(*types.Pointer).Elem().Underlying().(*types.Struct).Field(fa.Field)
	return r.IsSource(field)
}

// IsSource determines whether a types.Var is a source, that is whether it refers to a field previously identified as a source.
func (r ResultType) IsSource(field *types.Var) bool {
	return r[(types.Object)(field)]
}
