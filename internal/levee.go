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
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
	"google.com/go-flow-levee/internal/pkg/config"
	"google.com/go-flow-levee/internal/pkg/source"

	"google.com/go-flow-levee/internal/pkg/utils"
)

var configFile string

func init() {
	Analyzer.Flags.StringVar(&configFile, "config", "config.json", "path to analysis configuration file")
}

var Analyzer = &analysis.Analyzer{
	Name:     "levee",
	Run:      run,
	Doc:      "reports attempts to source data to sinks",
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	conf, err := config.FromFile(configFile)
	if err != nil {
		return nil, err
	}
	// TODO: respect configuration scope

	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

	sourcesMap := identifySources(conf, ssaInput)

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
						if a := conf.SendsToIOWriter(v); a != nil {
							sources = append(sources, source.NewSource(a, conf))
						} else {
							//log.V(2).Infof("Adding source: %v %T", v.Value(), v.Value())
							sources = append(sources, source.NewSource(v, conf))
						}

					case conf.IsSink(v):
						// TODO Only variadic sink arguments are currently detected.
						if v.Call.Signature().Variadic() && len(v.Call.Args) > 0 {
							lastArg := v.Call.Args[len(v.Call.Args)-1]
							if varargs, ok := lastArg.(*ssa.Slice); ok {
								if sinkVarargs := source.NewVarargs(varargs, sources); sinkVarargs != nil {
									for _, s := range sinkVarargs.Sources {
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

func identifySources(conf *config.Config, ssaInput *buildssa.SSA) map[*ssa.Function][]*source.Source {
	sourceMap := make(map[*ssa.Function][]*source.Source)

	for _, fn := range ssaInput.SrcFuncs {
		var sources []*source.Source
		sources = append(sources, sourcesFromParams(fn, conf)...)
		sources = append(sources, sourcesFromClosure(fn, conf)...)
		sources = append(sources, sourcesFromBlocks(fn, conf)...)

		if len(sources) > 0 {
			sourceMap[fn] = sources
		}
	}
	return sourceMap
}

func sourcesFromParams(fn *ssa.Function, conf *config.Config) []*source.Source {
	var sources []*source.Source
	for _, p := range fn.Params {
		switch t := p.Type().(type) {
		case *types.Pointer:
			if n, ok := t.Elem().(*types.Named); ok && conf.IsSource(n) {
				sources = append(sources, source.NewSource(p, conf))
			}
			// TODO Handle the case where sources arepassed by value: func(c sourceType)
			// TODO Handle cases where PII is wrapped in struct/slice/map
		}
	}
	return sources
}

func sourcesFromClosure(fn *ssa.Function, conf *config.Config) []*source.Source {
	var sources []*source.Source
	for _, p := range fn.FreeVars {
		switch t := p.Type().(type) {
		case *types.Pointer:
			// FreeVars (variables from a closure) appear as double-pointers
			// Hence, the need to dereference them recursively.
			if s, ok := utils.Dereference(t).(*types.Named); ok && conf.IsSource(s) {
				sources = append(sources, source.NewSource(p, conf))
			}
		}
	}
	return sources
}

func sourcesFromBlocks(fn *ssa.Function, conf *config.Config) []*source.Source {
	var sources []*source.Source
	for _, b := range fn.Blocks {
		if b == fn.Recover {
			// TODO Handle calls to log in a recovery block.
			continue
		}

		for _, instr := range b.Instrs {
			switch v := instr.(type) {
			// Looking for sources of PII allocated within the body of a function.
			case *ssa.Alloc:
				if conf.IsSource(utils.Dereference(v.Type())) {
					sources = append(sources, source.NewSource(v, conf))
				}

				// Handling the case where PII may be in a receiver
				// (ex. func(b *something) { log.Info(something.PII) }
			case *ssa.FieldAddr:
				if conf.IsSource(utils.Dereference(v.Type())) {
					sources = append(sources, source.NewSource(v, conf))
				}
			}
		}
	}
	return sources
}

func report(pass *analysis.Pass, source *source.Source, sink ssa.Node) {
	var b strings.Builder
	b.WriteString("a source has reached a sink")
	fmt.Fprintf(&b, ", source: %v", pass.Fset.Position(source.Node.Pos()))
	pass.Reportf(sink.Pos(), b.String())
}
