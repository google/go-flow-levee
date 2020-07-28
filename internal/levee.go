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
	"strings"

	"github.com/google/go-flow-levee/internal/pkg/utils"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"

	"github.com/google/go-flow-levee/internal/pkg/source"
	"github.com/google/go-flow-levee/internal/pkg/varargs"
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

	// TODO: integrate this with the rest of the code

	for fn, _ := range sourcesMap {
		if fn.Name() == "TestSlices" {
			graph := newGraph()
			for _, b := range fn.Blocks {
				for _, inst := range b.Instrs {
					dst, hasDst := inst.(ssa.Value)
					if !hasDst {
						t, ok := inst.(*ssa.Store)
						if !ok {
							continue
						}
						graph.addEdgeFromTo(t.Val.Name(), t.Addr.Name())
						continue
					}
					dstName := dst.Name()
					switch t := inst.(type) {
					case *ssa.Alloc:
						if conf.IsSource(utils.Dereference(t.Type())) {
							graph.addSource(source.New(t, conf))
						}
					case *ssa.IndexAddr:
						src, _ := t.X.(ssa.Value)
						srcName := src.Name()
						// this looks backwards, but we want to taint the whole array/slice
						graph.addEdgeFromTo(dstName, srcName)
					case *ssa.UnOp:
						src, _ := t.X.(ssa.Value)
						srcName := src.Name()
						graph.addEdgeFromTo(dstName, srcName)
					case *ssa.Slice:
						src, _ := t.X.(ssa.Value)
						srcName := src.Name()
						graph.addEdgeFromTo(dstName, srcName)
					case *ssa.MakeInterface:
						src, _ := t.X.(ssa.Value)
						srcName := src.Name()
						graph.addEdgeFromTo(dstName, srcName)
					case *ssa.Call:
						parameters := t.Call.Args
						var parameterNames []string
						for _, p := range parameters {
							parameterNames = append(parameterNames, p.Name())
						}
						graph.addSink(*t, parameterNames...)
					}
				}
			}
			graph.report(pass)
			continue
		}
	}

	// Only examine functions that have sources
	for fn, sources := range sourcesMap {
		for _, b := range fn.Blocks {
			if b == fn.Recover {
				// TODO Handle calls to sinks in a recovery block.
				continue // skipping Recover since it does not have instructions, rather a single block.
			}

			for _, instr := range b.Instrs {
				v, ok := instr.(*ssa.Call)
				if !ok {
					continue
				}
				switch {
				case conf.IsPropagator(v):
					// Handling the case where sources are propagated to io.Writer
					// (ex. proto.MarshalText(&buf, c)
					// In such cases, "buf" becomes a source, and not the return value of the propagator.
					// TODO Do not hard-code logging sinks usecase
					// TODO  Handle case of os.Stdout and os.Stderr.
					// TODO  Do not hard-code the position of the argument, instead declaratively
					//  specify the position of the propagated source.
					// TODO  Consider turning propagators that take io.Writer into sinks.
					if a := getArgumentPropagator(conf, v); a != nil {
						sources = append(sources, source.New(a, conf))
					} else {
						sources = append(sources, source.New(v, conf))
					}

				case conf.IsSink(v):
					// TODO Only variadic sink arguments are currently detected.
					if sinkVarargs := varargs.New(v); sinkVarargs != nil {
						for _, s := range sources {
							if sinkVarargs.ReferredBy(s) && !s.IsSanitizedAt(v) {
								report(pass, s, v)
								break
							}
						}
					}
				}
			}
		}
	}

	return nil, nil
}

func report(pass *analysis.Pass, source *source.Source, sink ssa.Node) {
	var b strings.Builder
	b.WriteString("a source has reached a sink")
	fmt.Fprintf(&b, ", source: %v", pass.Fset.Position(source.Node().Pos()))
	pass.Reportf(sink.Pos(), b.String())
}

func getArgumentPropagator(c *config.Config, call *ssa.Call) ssa.Node {
	if call.Call.Signature().Params().Len() == 0 {
		return nil
	}

	firstArg := call.Call.Signature().Params().At(0)
	if c.PropagatorArgs.ArgumentTypeRE.MatchString(firstArg.Type().String()) {
		if a, ok := call.Call.Args[0].(*ssa.MakeInterface); ok {
			return a.X.(ssa.Node)
		}
	}

	return nil
}

type Graph struct {
	edges   map[string]map[string]bool
	sources []*source.Source
	sinks   []ssa.Call
}

func newGraph() Graph {
	return Graph{
		edges: map[string]map[string]bool{},
	}
}

func (g *Graph) addEdgeFromTo(src, dst string) {
	if _, ok := g.edges[src]; !ok {
		g.edges[src] = map[string]bool{}
	}
	g.edges[src][dst] = true
}

func (g *Graph) addSource(s *source.Source) {
	g.sources = append(g.sources, s)
}

func (g *Graph) addSink(sink ssa.Call, args ...string) {
	g.sinks = append(g.sinks, sink)
	for _, a := range args {
		g.addEdgeFromTo(a, sink.Call.StaticCallee().Name())
	}
}

func (g *Graph) report(pass *analysis.Pass) {
	seen := map[string]bool{}
	for _, source := range g.sources {
		var stack []string
		name := g.getName(source.Node())
		stack = append(stack, name)
		seen[name] = true
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, sink := range g.sinks {
				if sink.Call.StaticCallee().Name() == "Sink" {
					report(pass, source, &sink)
				}
			}
			for n, _ := range g.edges[current] {
				if !seen[n] {
					seen[n] = true
					stack = append(stack, n)
				}
			}
		}
	}

}

func (g *Graph) getName(s ssa.Node) string {
	val, _ := s.(ssa.Value)
	return val.Name()
}
