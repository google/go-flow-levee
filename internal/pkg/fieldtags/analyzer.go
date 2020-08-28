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
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"strings"

	"github.com/google/go-flow-levee/internal/pkg/utils"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/ssa"
)

type keyValue struct {
	key   string
	value string
}

type sourcePatterns []keyValue

var patterns sourcePatterns = []keyValue{
	{"levee", "source"},
}

// TaggedFields is named type representing a slice of TaggedFields.
type TaggedFields []TaggedField

// ResultType is a slice of TaggedFields.
type ResultType = TaggedFields

// A TaggedField contains the necessary information to identify a tagged struct field.
type TaggedField struct {
	pkg, typ, field string
}

var Analyzer = &analysis.Analyzer{
	Name: "fieldtags",
	Doc:  "This analyzer identifies Source fields based on their tags. Tags are expected to satisfy the `go vet -structtag` format.",
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
	ResultType: reflect.TypeOf(new(ResultType)).Elem(),
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspectResult := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	var taggedFields TaggedFields

	nodeFilter := []ast.Node{
		(*ast.TypeSpec)(nil),
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
		for _, f := range (*s).Fields.List {
			if patterns.isSource(f) && len(f.Names) > 0 {
				tf := TaggedField{
					pkg:   pass.Pkg.Name(),
					typ:   t.Name.Name,
					field: f.Names[0].Name,
				}
				taggedFields = append(taggedFields, tf)
				pass.Reportf(f.Pos(), "tagged field: %s", tf)
			}
		}
	})
	return taggedFields, nil
}

func (sp *sourcePatterns) isSource(field *ast.Field) bool {
	if field.Tag == nil {
		return false
	}

	tag := field.Tag.Value

	// TODO: consider refactoring this logic into a regex matcher
	i := 1 // skip initial quote
	j := 1
	for j < len(tag) {
		for j < len(tag) && tag[j] != ':' {
			j++
		}
		key := tag[i:j]
		if key == "" {
			return false
		}

		i = j + 1 // skip colon
		if i >= len(tag) {
			return false
		}
		if tag[i] == '\\' {
			i++
		}
		i++ // skip quote

		j = i
		for j < len(tag) && tag[j] != '"' {
			// skip escape character
			if tag[j] == '\\' {
				j++
			}
			j++
		}
		value := tag[i:j]
		if value == "" {
			return false
		}

		for _, p := range *sp {
			// value may be a list of comma separated values
			if p.key == key && strings.Contains(value, p.value) {
				return true
			}
		}

		i = j + 2 // skip closing quote and space
		j = i
	}

	return false
}

// IsSource determines whether a FieldAddr is a source, that is whether it refers to a field previously identified as a source.
func (t TaggedFields) IsSource(f *ssa.FieldAddr) bool {
	n, ok := namedType(f)
	if !ok {
		return false
	}
	fName := utils.FieldName(f)
	for _, tf := range t {
		if tf.matches(n.String(), fName) {
			return true
		}
	}
	return false
}

func namedType(f *ssa.FieldAddr) (*types.Named, bool) {
	structType := f.X.Type()
	deref := utils.Dereference(structType)
	named, ok := deref.(*types.Named)
	return named, ok
}

func (t TaggedField) matches(typeName, fieldName string) bool {
	return fmt.Sprintf("%s.%s", t.pkg, t.typ) == typeName && t.field == fieldName
}

func (t TaggedField) String() string {
	return fmt.Sprintf("%s.%s.%s", t.pkg, t.typ, t.field)
}
