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

	"github.com/google/go-flow-levee/internal/pkg/call"
	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/fieldpropagator"
	"github.com/google/go-flow-levee/internal/pkg/varargs"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"

	"github.com/google/go-flow-levee/internal/pkg/source"
)

var Analyzer = &analysis.Analyzer{
	Name:     "levee",
	Run:      run,
	Flags:    config.FlagSet,
	Doc:      "reports attempts to source data to sinks",
	Requires: []*analysis.Analyzer{buildssa.Analyzer, source.Analyzer, fieldpropagator.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}
	// TODO: respect configuration scope

	// a call is safe if its arguments have been analyzed by pointer analysis and found not to point to Sources
	isSafeCall := map[*ssa.Call]bool{}
	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	for _, f := range ssaInput.SrcFuncs {
		for _, b := range f.Blocks {
			for _, instr := range b.Instrs {
				c, ok := instr.(*ssa.Call)
				if !ok || !conf.IsSink(c) {
					continue
				}
				isSafeCall[c] = doPointerAnalysis(pass, conf, c)
			}
		}
	}

	sourcesMap := pass.ResultOf[source.Analyzer].(source.ResultType)
	fieldPropagators := pass.ResultOf[fieldpropagator.Analyzer].(fieldpropagator.ResultType)
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
				case fieldPropagators.Contains(v):
					sources = append(sources, source.New(v, conf))

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
					if isSafeCall[v] {
						continue
					}

					c := makeCall(v)
					for _, s := range sources {
						if c.ReferredBy(s) && !s.IsSanitizedAt(v) {
							report(pass, s, v)
							break
						}
					}
				}
			}
		}
	}

	return nil, nil
}

func doPointerAnalysis(pass *analysis.Pass, analysisConf *config.Config, c *ssa.Call) (isSafeCall bool) {
	pkg := c.Parent().Pkg
	main := pkg.Func("main")
	if main == nil {
		return false
	}

	var pointed []ssa.Value
	conf := &pointer.Config{
		Mains: []*ssa.Package{pkg},
	}

	slice, ok := c.Call.Args[0].(*ssa.Slice)
	if !ok {
		return false
	}

	indexAddr := (*slice.X.Referrers())[0].(*ssa.IndexAddr)
	refs := *indexAddr.Referrers()
	addr := refs[0].(*ssa.Store).Val

	conf.AddQuery(addr)
	pointed = append(pointed, addr)

	result, err := pointer.Analyze(conf)
	if err != nil {
		return false
	}

	isSafeCall = true
	for _, p := range pointed {
		pSet := result.Queries[p]
		if pSet.PointsTo().String() != "[]" {
			for _, lab := range pSet.PointsTo().Labels() {
				v := lab.Value()
				x := v.(*ssa.MakeInterface).X
				if analysisConf.IsSource(utils.Dereference(x.Type())) {
					isSafeCall = false
					pass.Reportf(x.Pos(), "a source has reached a sink")
				}
			}
		} else {
			fmt.Println(p, "has empty pset")
		}
	}
	return isSafeCall
}

func makeCall(c *ssa.Call) call.Call {
	if sinkVarargs := varargs.New(c); sinkVarargs != nil {
		return sinkVarargs
	}
	return call.Regular(c)
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

// for _, a := range c.Call.Args {
// 	if !pointer.CanPoint(a.Type()) {
// 		continue
// 	}
// 	// DFS TO FIND THINGS TO POINT TO
// 	conf.AddExtendedQuery(a, "x[0]")
// 	pointed = append(pointed, a)
// }
