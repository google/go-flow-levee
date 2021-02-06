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

// Package suppression defines an analyzer that identifies calls
// suppressed by a comment.
package suppression

import (
	"go/ast"
	"reflect"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// ResultType is a set of nodes that are suppressed by a comment.
type ResultType map[ast.Node]bool

func (rt ResultType) IsSuppressed(n ast.Node) bool {
	_, ok := rt[n]
	return ok
}

var Analyzer = &analysis.Analyzer{
	Name:       "suppression",
	Doc:        "This analyzer identifies ast nodes that are suppressed by comments.",
	Run:        run,
	ResultType: reflect.TypeOf(new(ResultType)).Elem(),
}

func run(pass *analysis.Pass) (interface{}, error) {
	result := map[ast.Node]bool{}

	for _, f := range pass.Files {
		for node, commentGroups := range ast.NewCommentMap(pass.Fset, f, f.Comments) {
			for _, cg := range commentGroups {
				if isSuppressingCommentGroup(cg) {
					result[node] = true
					// for testing
					pass.Reportf(node.Pos(), "suppressed")
				}
			}
		}
	}

	return ResultType(result), nil
}

func isSuppressingCommentGroup(commentGroup *ast.CommentGroup) bool {
	for _, line := range strings.Split(commentGroup.Text(), "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "//"), "/*"))
		if strings.HasPrefix(trimmed, doNotReport) {
			return true
		}
	}
	return false
}

const doNotReport = "levee.DoNotReport"
