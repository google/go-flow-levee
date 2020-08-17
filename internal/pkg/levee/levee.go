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

const sourceReachedSinkMessage = "a source has reached a sink"

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
							reportAtSink(pass, s.Node(), v)
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

	pointerConf := &pointer.Config{
		Mains: []*ssa.Package{pkg},
	}

	vs := callValues(c)
	if len(vs) == 0 {
		return false
	}

	for _, v := range vs {
		if !pointer.CanPoint(v.Type()) {
			continue
		}
		pointerConf.AddQuery(v)
	}

	result, err := pointer.Analyze(pointerConf)
	if err != nil {
		return false
	}

	isSafeCall = true
	for _, pSet := range result.Queries {
		labels := pSet.PointsTo().Labels()
		if len(labels) == 0 {
			continue
		}
		for _, lab := range labels {
			v := lab.Value()
			switch t := v.(type) {
			case *ssa.MakeInterface:
				v = t.X
			}
			if !analysisConf.IsSource(utils.Dereference(v.Type())) {
				continue
			}
			isSafeCall = false
			reportAtSource(pass, v.(ssa.Node), (ssa.Node)(c))
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

func reportAtSink(pass *analysis.Pass, source ssa.Node, sink ssa.Node) {
	var b strings.Builder
	b.WriteString(sourceReachedSinkMessage)
	fmt.Fprintf(&b, ", source: %v", pass.Fset.Position(source.Pos()))
	pass.Reportf(sink.Pos(), b.String())
}

func reportAtSource(pass *analysis.Pass, source ssa.Node, sink ssa.Node) {
	var b strings.Builder
	b.WriteString(sourceReachedSinkMessage)
	fmt.Fprintf(&b, ", sink: %v", pass.Fset.Position(sink.Pos()))
	pass.Reportf(source.Pos(), b.String())
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

// callValues collects the ssa.Values that are arguments to an ssa.Call.
// in particular, it handles variadic arguments to some extent.
// We don't want to analyze what the variadic argument points to (a slice), but we do
// want to analyze the values *within* the slice.
func callValues(c *ssa.Call) (values []ssa.Value) {
	for _, a := range c.Call.Args {
		slice, ok := a.(*ssa.Slice)
		if !ok {
			values = append(values, a)
			continue
		}
		refs := *slice.X.Referrers()
		// the last referrer is the slice
		for i := 0; i < len(refs)-1; i++ {
			indexAddr := refs[i].(*ssa.IndexAddr)
			val := (*indexAddr.Referrers())[0].(*ssa.Store).Val
			values = append(values, val)
		}
	}
	return
}
