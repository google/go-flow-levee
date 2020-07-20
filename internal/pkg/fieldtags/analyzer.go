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

// Package fieldtags defines an analyzer that identifies struct fields tagged with datapol annotations
// indicating that they contain SPII.
package fieldtags

import (
	"go/ast"
	"reflect"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/ast/inspector"
)

type keyValue struct {
	key   string
	value string
}

type sourcePatterns []keyValue

var patterns sourcePatterns = []keyValue{
	{"levee", "source"},
}

var Analyzer = &analysis.Analyzer{
	Name: "fieldtags",
	Doc:  "This analyzer identifies Source fields based on their tags.",
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
		structtag.Analyzer, // required to ensure that tags are properly formatted
	},
	ResultType: reflect.TypeOf(new([]*ast.Field)).Elem(),
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspector_ := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	var results []*ast.Field

	nodeFilter := []ast.Node{
		(*ast.Field)(nil),
	}

	inspector_.Preorder(nodeFilter, func(n ast.Node) {
		field, ok := n.(*ast.Field)
		if !ok {
			return
		}
		if patterns.isSource(field) {
			results = append(results, field)
			pass.Reportf(field.Pos(), "tagged field")
		}
	})
	return results, nil
}

func (ps *sourcePatterns) isSource(field *ast.Field) bool {
	if field.Tag == nil {
		return false
	}

	tag := field.Tag.Value

	// TODO: consider refactoring this logic into a regex matcher
	i := 1 // skip backtick
	j := 1
	for j < len(tag) {
		for j < len(tag) && tag[j] != ':' {
			j++
		}
		key := tag[i:j]
		if key == "" {
			return false
		}

		i = j + 2 // skip colon and opening quote
		j = i
		for j < len(tag) && tag[j] != '"' {
			j++
		}
		value := tag[i:j]
		if value == "" {
			return false
		}

		for _, p := range *ps {
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
