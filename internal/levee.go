// Copyright 2019 Google LLC
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

package internal

import (
	"fmt"
	"go/types"
	"strconv"
	"strings"

	"github.com/google/go-flow-levee/internal/pkg/utils"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/source"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name:     "levee",
	Run:      run,
	Flags:    config.FlagSet,
	Doc:      "reports attempts to source data to sinks",
	Requires: []*analysis.Analyzer{source.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}
	// TODO: respect configuration scope

	sourcesMap := pass.ResultOf[source.Analyzer].(source.ResultType)

	for fn := range sourcesMap {
		analyzeFn(fn, conf, pass)
	}

	return nil, nil
}

func analyzeFn(fn *ssa.Function, conf *config.Config, pass *analysis.Pass) {
	graph := newGraph()

	for _, p := range fn.Params {
		if conf.IsSource(utils.Dereference(p.Type())) {
			graph.addSource(p.Name(), p)
		}
	}

	// this is needed to capture variables referred to by closures
	for _, p := range fn.FreeVars {
		switch t := p.Type().(type) {
		case *types.Pointer:
			if s, ok := utils.Dereference(t).(*types.Named); ok && conf.IsSource(s) {
				graph.addSource(p.Name(), p)
			}
		}
	}

	for _, b := range fn.Blocks {
		for _, inst := range b.Instrs {
			value, _ := inst.(ssa.Value)

			switch t := inst.(type) {
			case *ssa.Alloc:
				if conf.IsSource(utils.Dereference(t.Type())) && !source.IsProducedBySanitizer(t, conf) {
					graph.addSource(value.Name(), t)
				}

			case *ssa.Call:
				parameters := t.Call.Args
				var parameterNames []string
				for _, p := range parameters {
					parameterNames = append(parameterNames, p.Name())
				}
				switch {
				case conf.IsSink(t):
					graph.addSink(t, parameterNames)
				case conf.IsPropagator(t):
					graph.addSource(value.Name(), t)
				case conf.IsSanitizer(t):
					graph.addSanitizer(t)
				default:
					graph.addCallEdges(*t, parameterNames)
				}

			case *ssa.FieldAddr:
				if conf.IsSourceFieldAddr(t) {
					graph.addSource(value.Name(), t)
				}

			case *ssa.IndexAddr:
				x, _ := t.X.(ssa.Value)
				// example instruction: t1 = &t0[0:int]
				// the edge needs to be inverted since we are storing
				// a reference; in effect, t1 is now an alias for t0
				graph.addEdge(value.Name(), x.Name())

			case *ssa.MakeInterface:
				x := t.X.(ssa.Value)
				graph.addEdge(x.Name(), value.Name())

			case *ssa.Phi:
				for _, e := range t.Edges {
					graph.addEdge(e.Name(), value.Name())
				}

			case *ssa.Slice:
				x, _ := t.X.(ssa.Value)
				graph.addEdge(x.Name(), value.Name())

			case *ssa.Store:
				graph.addEdge(t.Val.Name(), t.Addr.Name())

			case *ssa.UnOp:
				x, _ := t.X.(ssa.Value)
				graph.addEdge(x.Name(), value.Name())
			}
		}
	}

	reachabilities := graph.detectSinksReachableFromSources()
	for _, r := range reachabilities {
		r.report(pass)
	}
}

type namedNode struct {
	name string
	node ssa.Node
}

type sink struct {
	name string
	call *ssa.Call
}

func newSink(call *ssa.Call, i int) sink {
	return sink{
		name: call.Call.StaticCallee().Name() + strconv.Itoa(i),
		call: call,
	}
}

type graph struct {
	edges      map[string]map[string]bool
	sources    []namedNode
	sinks      []sink
	sanitizers []string
}

func newGraph() graph {
	return graph{
		edges: map[string]map[string]bool{},
	}
}

func (g *graph) addEdge(from, to string) {
	if _, ok := g.edges[from]; !ok {
		g.edges[from] = map[string]bool{}
	}
	g.edges[from][to] = true
}

func (g *graph) addSource(name string, node ssa.Node) {
	g.sources = append(g.sources, namedNode{name: name, node: node})
}

func (g *graph) addCallEdges(call ssa.Call, args []string) {
	for _, a := range args {
		g.addEdge(a, call.Call.StaticCallee().Name())
	}
}

func (g *graph) addSink(call *ssa.Call, args []string) {
	sink := newSink(call, len(g.sinks))
	g.sinks = append(g.sinks, sink)
	for _, a := range args {
		g.addEdge(a, sink.name)
	}
}

func (g *graph) addSanitizer(call *ssa.Call) {
	g.sanitizers = append(g.sanitizers, call.Call.StaticCallee().Name())
}

func (g *graph) detectSinksReachableFromSources() []reachability {
	seen := map[string]bool{}
	var reachabilities []reachability

	for _, source := range g.sources {
		var stack []string
		name := source.name
		stack = append(stack, name)
		seen[name] = true
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			for _, sanitizer := range g.sanitizers {
				if current == sanitizer {
					continue
				}

			}

			for _, sink := range g.sinks {
				if current == sink.name {
					reachabilities = append(reachabilities, reachability{source: source.node, sink: sink.call})
					break
				}
			}

			for n := range g.edges[current] {
				if !seen[n] {
					seen[n] = true
					stack = append(stack, n)
				}
			}
		}
	}

	return reachabilities
}

type reachability struct {
	source ssa.Node
	sink   *ssa.Call
}

func (r *reachability) report(pass *analysis.Pass) {
	var b strings.Builder
	b.WriteString("a source has reached a sink")
	fmt.Fprintf(&b, ", source: %v", pass.Fset.Position(r.source.Pos()))
	pass.Reportf(r.sink.Pos(), b.String())
}
