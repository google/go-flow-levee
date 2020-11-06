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

	"github.com/google/go-flow-levee/internal/pkg/config"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/ssa"
)

type ResultType struct {
	sources      map[types.Object]bool
	taggedFields map[types.Object]bool
}

// hasTaggedFields is a Fact that indicates that a type is identified
// as a Source due to having at least one tagged field.
type hasTaggedFields struct{}

func (hasTaggedFields) AFact() {}
func (hasTaggedFields) String() string {
	return "source"
}

type isTaggedField struct{}

func (isTaggedField) AFact() {}
func (isTaggedField) String() string {
	return "tagged field"
}

var Analyzer = &analysis.Analyzer{
	Name: "fieldtags",
	Doc:  "This analyzer identifies Sources based on their field tags. Tags are expected to satisfy the `go vet -structtag` format.",
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
	ResultType: reflect.TypeOf(new(ResultType)).Elem(),
	FactTypes:  []analysis.Fact{new(hasTaggedFields), new(isTaggedField)},
}

func run(pass *analysis.Pass) (interface{}, error) {
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	inspectResult := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.TypeSpec)(nil),
	}

	rt := ResultType{
		sources:      map[types.Object]bool{},
		taggedFields: map[types.Object]bool{},
	}

	inspectResult.Preorder(nodeFilter, func(n ast.Node) {
		t, ok := n.(*ast.TypeSpec)
		if !ok {
			return
		}
		s, ok := t.Type.(*ast.StructType)
		if !ok || s.Fields == nil {
			return
		}
		isSource := false
		for _, f := range (*s).Fields.List {
			if f.Tag == nil || len(f.Names) == 0 || !conf.IsSourceFieldTag(f.Tag.Value) {
				continue
			}
			isSource = true
			for _, ident := range f.Names {
				pass.ExportObjectFact(pass.TypesInfo.ObjectOf(ident), &isTaggedField{})
			}
		}
		if isSource {
			pass.ExportObjectFact(pass.TypesInfo.ObjectOf(t.Name), &hasTaggedFields{})
		}

	})

	for _, f := range pass.AllObjectFacts() {
		switch f.Fact.(type) {
		case *isTaggedField:
			rt.taggedFields[f.Object] = true
		case *hasTaggedFields:
			rt.sources[f.Object] = true
		}
	}

	return rt, nil
}

func (rt ResultType) IsSource(o types.Object) bool {
	return rt.sources[o]
}

// IsSource determines whether a types.Var is a source, that is whether it refers to a field previously identified as a source.
func (rt ResultType) IsSourceField(field *types.Var) bool {
	return rt.taggedFields[field]
}

// IsSourceFieldAddr determines whether a ssa.FieldAddr is a source, that is whether it refers to a field previously identified as a source.
func (rt ResultType) IsSourceFieldAddr(fa *ssa.FieldAddr) bool {
	// incantation plundered from the docstring for ssa.FieldAddr.Field
	field := fa.X.Type().Underlying().(*types.Pointer).Elem().Underlying().(*types.Struct).Field(fa.Field)
	return rt.IsSourceField(field)
}
