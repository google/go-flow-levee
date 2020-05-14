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

	"github.com/google/go-flow-levee/internal/pkg/config"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/google/go-flow-levee/internal/pkg/source"
)

var Analyzer = &analysis.Analyzer{
	Name:     "levee",
	Run:      run,
	Flags:    config.FlagSet,
	Doc:      "reports attempts to source data to sinks",
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

// varargs represents a variable length argument.
// Concretely, it abstract over the fact that the varargs internally are represented by an ssa.Slice
// which contains the underlying values of for vararg members.
// Since many sink functions (ex. log.Info, fmt.Errorf) take a vararg argument, being able to
// get the underlying values of the vararg members is important for this analyzer.
type varargs struct {
	slice   *ssa.Slice
	sources []*source.Source
}

// newVarargs constructs varargs. SSA represents varargs as an ssa.Slice.
func newVarargs(s *ssa.Slice, sources []*source.Source) *varargs {
	a, ok := s.X.(*ssa.Alloc)
	if !ok || a.Comment != "varargs" {
		return nil
	}
	var (
		referredSources []*source.Source
	)

	for _, r := range *a.Referrers() {
		idx, ok := r.(*ssa.IndexAddr)
		if !ok {
			continue
		}

		if idx.Referrers() != nil && len(*idx.Referrers()) != 1 {
			continue
		}

		// IndexAddr and Store instructions are inherently linked together.
		// IndexAddr returns an address of an element within a Slice, which is followed by
		// a Store instructions to place a value into the address provided by IndexAddr.
		store := (*idx.Referrers())[0].(*ssa.Store)

		for _, s := range sources {
			if s.HasPathTo(store.Val.(ssa.Node)) {
				referredSources = append(referredSources, s)
				break
			}
		}
	}

	return &varargs{
		slice:   s,
		sources: referredSources,
	}
}

func (v *varargs) referredByCallWithPattern(patterns []config.NameMatcher) *ssa.Call {
	if v.slice.Referrers() == nil || len(*v.slice.Referrers()) != 1 {
		return nil
	}

	c, ok := (*v.slice.Referrers())[0].(*ssa.Call)
	if !ok || c.Call.StaticCallee() == nil {
		return nil
	}

	for _, p := range patterns {
		if p.MatchMethodName(c) {
			return c
		}
	}

	return nil
}

func run(pass *analysis.Pass) (interface{}, error) {
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}
	// TODO: respect configuration scope

	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

	sourcesMap := source.Identify(conf, ssaInput)

	// Only examine functions that have sources
	for fn, sources := range sourcesMap {
		//log.V(2).Infof("Processing function %v", fn)

		for _, b := range fn.Blocks {
			if b == fn.Recover {
				// TODO Handle calls to sinks in a recovery block.
				continue // skipping Recover since it does not have instructions, rather a single block.
			}

			for _, instr := range b.Instrs {
				//log.V(2).Infof("Inst: %v %T", instr, instr)
				switch v := instr.(type) {
				case *ssa.Call:
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
						if a := sendsToIOWriter(conf, v); a != nil {
							sources = append(sources, source.New(a, conf))
						} else {
							//log.V(2).Infof("Adding source: %v %T", v.Value(), v.Value())
							sources = append(sources, source.New(v, conf))
						}

					case conf.IsSink(v):
						// TODO Only variadic sink arguments are currently detected.
						if v.Call.Signature().Variadic() && len(v.Call.Args) > 0 {
							lastArg := v.Call.Args[len(v.Call.Args)-1]
							if varargs, ok := lastArg.(*ssa.Slice); ok {
								if sinkVarargs := newVarargs(varargs, sources); sinkVarargs != nil {
									for _, s := range sinkVarargs.sources {
										if !s.IsSanitizedAt(v) {
											report(pass, s, v)
										}
									}
								}
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

func sendsToIOWriter(c *config.Config, call *ssa.Call) ssa.Node {
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
