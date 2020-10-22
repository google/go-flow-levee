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
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/ssa"
)

// ResultType is a set of types.Object that are inferred Sources.
type ResultType map[types.Object]bool

type inferredSource struct{}

func (i inferredSource) AFact() {}

func (i inferredSource) String() string {
	return "inferred source"
}

var Analyzer = &analysis.Analyzer{
	Name: "sourceinfer",
	Doc: `This analyzer infers source types from typedefs and fields.

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

The analysis is recursive: types identified as sources during analysis
will be used to evaluate the sourceyness of types that depend on them.
For example, in the below type definitions, both Qux and Quux will be
identified as sources:
type Qux Foo
type Quux Qux
`,
	Run: run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
		buildssa.Analyzer,
	},
	ResultType: reflect.TypeOf(new(ResultType)).Elem(),
	FactTypes:  []analysis.Fact{new(inferredSource)},
}

type ObjectGraph map[types.Object][]types.Object

func run(pass *analysis.Pass) (interface{}, error) {
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	typeGraph := createTypeDefGraph(pass, ins)

	builtSSA := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	addFieldEdges(builtSSA, typeGraph)

	inferredSources := inferSources(pass, conf, typeGraph)

	return inferredSources, nil
}

func createTypeDefGraph(pass *analysis.Pass, ins *inspector.Inspector) ObjectGraph {
	defGraph := ObjectGraph{}

	genDeclFilter := []ast.Node{
		(*ast.TypeSpec)(nil),
	}

	ins.Preorder(genDeclFilter, func(n ast.Node) {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return
		}
		// type alias, e.g. type A = B
		// we don't need to handle these, since the type will be inlined
		if ts.Assign != token.NoPos {
			return
		}

		// struct definition, fields will be checked later in the analysis
		if _, ok := ts.Type.(*ast.StructType); ok {
			return
		}

		typeBeingDefined := pass.TypesInfo.ObjectOf(ts.Name)

		selectorFinder := &selectorFinder{nil, map[*ast.Ident]bool{}}
		ast.Walk(selectorFinder, (ast.Node)(ts.Type))
		for _, sel := range selectorFinder.foundSelectors {
			named, ok := pass.TypesInfo.TypeOf(sel).(*types.Named)
			if !ok {
				continue
			}
			obj := named.Obj()
			if len(defGraph[obj]) == 0 {
				defGraph[obj] = make([]types.Object, 0, 1)
			}
			defGraph[obj] = append(defGraph[obj], typeBeingDefined)
		}

		idFinder := &identFinder{}
		ast.Walk(idFinder, (ast.Node)(ts.Type))
		for _, id := range idFinder.foundIdentifiers {
			// identifier is part of a SelectorExpr and has already been handled
			if selectorFinder.foundIdentifiers[id] {
				continue
			}
			obj := pass.TypesInfo.ObjectOf(id)
			if len(defGraph[obj]) == 0 {
				defGraph[obj] = make([]types.Object, 0, 1)
			}
			defGraph[obj] = append(defGraph[obj], typeBeingDefined)
		}
	})

	return defGraph
}

type selectorFinder struct {
	foundSelectors   []*ast.SelectorExpr
	foundIdentifiers map[*ast.Ident]bool
}

func (sf *selectorFinder) Visit(n ast.Node) ast.Visitor {
	sel, ok := n.(*ast.SelectorExpr)
	if !ok {
		return sf
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}
	sf.foundSelectors = append(sf.foundSelectors, sel)
	sf.foundIdentifiers[pkg] = true
	sf.foundIdentifiers[sel.Sel] = true
	return nil
}

type identFinder struct {
	foundIdentifiers []*ast.Ident
}

func (i *identFinder) Visit(n ast.Node) ast.Visitor {
	if id, ok := n.(*ast.Ident); ok {
		i.foundIdentifiers = append(i.foundIdentifiers, id)
		return nil
	}
	return i
}

func addFieldEdges(builtSSA *buildssa.SSA, typeGraph ObjectGraph) {
	for _, m := range builtSSA.Pkg.Members {
		t, ok := m.(*ssa.Type)
		if !ok {
			continue
		}
		n, ok := t.Type().(*types.Named)
		if !ok {
			continue
		}
		s, ok := n.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < s.NumFields(); i++ {
			namedTypes := map[*types.Named]bool{}
			findNamedTypes(s.Field(i).Type(), namedTypes)
			for nt := range namedTypes {
				obj := nt.Obj()
				if len(typeGraph[obj]) == 0 {
					typeGraph[obj] = make([]types.Object, 0, 1)
				}
				typeGraph[obj] = append(typeGraph[obj], n.Obj())
			}
		}
	}
}

func findNamedTypes(t types.Type, namedTypes map[*types.Named]bool) {
	deref := utils.Dereference(t)
	switch tt := deref.(type) {
	case *types.Named:
		namedTypes[tt] = true
	case *types.Array:
		findNamedTypes(tt.Elem(), namedTypes)
	case *types.Slice:
		findNamedTypes(tt.Elem(), namedTypes)
	case *types.Chan:
		findNamedTypes(tt.Elem(), namedTypes)
	case *types.Map:
		findNamedTypes(tt.Elem(), namedTypes)
		findNamedTypes(tt.Key(), namedTypes)
	case *types.Basic, *types.Struct, *types.Tuple, *types.Interface, *types.Signature:
		// these types cannot hold named types
	case *types.Pointer:
		// this should be unreachable due to the dereference above
	default:
		// The above should be exhaustive.  Reaching this default case is an error.
		fmt.Printf("unexpected type received: %T %v; please report this issue\n", tt, tt)
	}
}

func inferSources(pass *analysis.Pass, conf *config.Config, typeGraph ObjectGraph) ResultType {
	inferredSources := ResultType{}

	order := topoSort(typeGraph)

	seen := map[types.Object]bool{}
	for i := len(order) - 1; i >= 0; i-- {
		o := order[i]
		if seen[o] {
			continue
		}
		if !(isSourceType(conf, o.Type()) || pass.ImportObjectFact(o, &inferredSource{})) {
			continue
		}
		seen[o] = true
		stack := []types.Object{o}
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if current.Pkg() == pass.Pkg && current != o {
				pass.ExportObjectFact(current, &inferredSource{})
				inferredSources[current] = true
			}
			for _, n := range typeGraph[current] {
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
func topoSort(graph ObjectGraph) []types.Object {
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
	deref := utils.Dereference(t)
	switch tt := deref.(type) {
	case *types.Named:
		return c.IsSource(tt) || isSourceType(c, tt.Underlying())
	case *types.Array:
		return isSourceType(c, tt.Elem())
	case *types.Slice:
		return isSourceType(c, tt.Elem())
	case *types.Chan:
		return isSourceType(c, tt.Elem())
	case *types.Map:
		key := isSourceType(c, tt.Key())
		elem := isSourceType(c, tt.Elem())
		return key || elem
	case *types.Basic, *types.Struct, *types.Tuple, *types.Interface, *types.Signature:
		// These types do not currently represent possible source types
		return false
	case *types.Pointer:
		// This should be unreachable due to the dereference above
		return false
	default:
		// The above should be exhaustive.  Reaching this default case is an error.
		fmt.Printf("unexpected type received: %T %v; please report this issue\n", tt, tt)
		return false
	}
}
