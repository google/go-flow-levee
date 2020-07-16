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

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name:     "fieldtags",
	Doc:      "This analyzer identifies fields tagged with SPII datapol annotations.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.Field)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		field, ok := n.(*ast.Field)
		if !ok {
			return
		}
		if isTaggedPII(field) {
			pass.Reportf(field.Pos(), "tagged field")
		}
	})
	return nil, nil
}

func isTaggedPII(field *ast.Field) bool {
	tag := field.Tag
	if tag == nil {
		return false
	}

	value := tag.Value
	if value == "`datapol:ST_ACCOUNT_CRENDENTIAL`" {
		return true
	}
	return false
}
