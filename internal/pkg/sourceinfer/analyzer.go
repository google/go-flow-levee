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

// Package infer defines an analyzer that identifies Sources that either
// 1. are defined from a Source type, or
// 2. have a Source type as a field
package infer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/fieldtags"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// ResultType is a set of types.Object that are inferred Sources.
type ResultType map[types.Object]bool

type inferredSourceFact struct{}

func (i inferredSourceFact) AFact() {}

func (i inferredSourceFact) String() string {
	return "inferred source"
}

var Analyzer = &analysis.Analyzer{
	Name: "sourceinfer",
	Doc: `This analyzer infers named source types from typedefs and fields.

Suppose Foo has been configured to be a source type.

Then, the following type definition would cause Bar to be identified
as a source, because its underlying type is Foo:
type Bar Foo
Indeed, if Foo contains sensitive data, then so does Bar, because they have
the same fields.

Similarly, the following type definition would cause Baz to be identified
as a source, because it holds a field of type Foo:
type Baz struct {
  f Foo
}
Indeed, if Foo contains sensitive data, and Bar contains Foo, Bar also
contains that data, via Foo.

Types identified as sources during analysis are used to evaluate
the sourceyness of types that depend on them. For example, in the
below type definitions, both Qux and Quux will be identified as sources:
type Qux Foo
type Quux Qux
`,
	Run: run,
	Requires: []*analysis.Analyzer{
		fieldtags.Analyzer,
		inspect.Analyzer,
	},
	ResultType: reflect.TypeOf(new(ResultType)).Elem(),
	FactTypes:  []analysis.Fact{new(inferredSourceFact)},
}

type objectGraph map[types.Object][]types.Object

func run(pass *analysis.Pass) (interface{}, error) {
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	ft := pass.ResultOf[fieldtags.Analyzer].(fieldtags.ResultType)

	objectGraph := createObjectGraph(pass, ins)

	inferredSources := inferSources(pass, conf, ft, objectGraph)

	return inferredSources, nil
}

func createObjectGraph(pass *analysis.Pass, ins *inspector.Inspector) objectGraph {
	objGraph := objectGraph{}

	typeSpecFilter := []ast.Node{
		(*ast.TypeSpec)(nil),
	}

	ins.Preorder(typeSpecFilter, func(n ast.Node) {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return
		}

		// type alias, e.g. type A = B
		// we don't need to handle these, since the type will be inlined
		if ts.Assign != token.NoPos {
			return
		}

		typeBeingDefined := pass.TypesInfo.ObjectOf(ts.Name)

		for n := range findNamedTypes(pass.TypesInfo.TypeOf(ts.Type)) {
			objGraph[n.Obj()] = append(objGraph[n.Obj()], typeBeingDefined)
		}
	})

	return objGraph
}

func findNamedTypes(t types.Type) map[*types.Named]bool {
	namedTypes := map[*types.Named]bool{}

	var find func(t types.Type)
	find = func(t types.Type) {
		deref := utils.Dereference(t)
		switch tt := deref.(type) {
		case *types.Named:
			namedTypes[tt] = true
		case *types.Array:
			find(tt.Elem())
		case *types.Slice:
			find(tt.Elem())
		case *types.Chan:
			find(tt.Elem())
		case *types.Map:
			find(tt.Key())
			find(tt.Elem())
		case *types.Struct:
			for i := 0; i < tt.NumFields(); i++ {
				find(tt.Field(i).Type())
			}
		case *types.Basic, *types.Tuple, *types.Interface, *types.Signature:
			// these types cannot hold named types
		case *types.Pointer:
			// this should be unreachable due to the dereference above
		default:
			// The above should be exhaustive.  Reaching this default case is an error.
			fmt.Printf("unexpected type received: %T %v; please report this issue\n", tt, tt)
		}
	}

	find(t)

	return namedTypes
}

func inferSources(pass *analysis.Pass, conf *config.Config, ft fieldtags.ResultType, objGraph objectGraph) ResultType {
	inferredSources := ResultType{}

	order := topoSort(objGraph)

	seen := map[types.Object]bool{}
	for i := len(order) - 1; i >= 0; i-- {
		o := order[i]
		if seen[o] {
			continue
		}
		if !(isSourceType(conf, o.Type()) || ft.IsSource(o) || pass.ImportObjectFact(o, &inferredSourceFact{})) {
			continue
		}

		seen[o] = true
		stack := []types.Object{o}
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if current.Pkg() == pass.Pkg && current != o {
				pass.ExportObjectFact(current, &inferredSourceFact{})
				inferredSources[current] = true
			}
			for _, n := range objGraph[current] {
				if seen[n] {
					continue
				}
				seen[n] = true
				stack = append(stack, n)
			}
		}
	}

	return inferredSources
}

// topoSort produces the topological order of a given graph.
// If the graph contains cycles, the order within a cycle is not specified,
// but the order of nodes outside the cycle will not be affected.
func topoSort(graph objectGraph) []types.Object {
	var order []types.Object
	visited := map[types.Object]bool{}
	var visit func(o types.Object)
	visit = func(o types.Object) {
		if visited[o] {
			return
		}
		visited[o] = true
		for _, n := range graph[o] {
			visit(n)
		}
		order = append(order, o)
	}
	for o := range graph {
		visit(o)
	}
	return order
}

func isSourceType(c *config.Config, t types.Type) bool {
	for nt := range findNamedTypes(t) {
		if c.IsSource(nt) {
			return true
		}
	}
	return false
}
