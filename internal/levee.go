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
	"google.com/go-flow-levee/internal/pkg/matcher"

	"github.com/eapache/queue"

	"google.com/go-flow-levee/internal/pkg/sanitizer"
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

// source represents a source in an SSA call tree.
// It is based on ssa.Node, with the added functionality of computing the recursive graph of
// its referrers.
// source.sanitized notes sanitizer calls that sanitize this source
type source struct {
	node       ssa.Node
	marked     map[ssa.Node]bool
	sanitizers []*sanitizer.Sanitizer
}

func newSource(in ssa.Node, config *config.Config) *source {
	a := &source{
		node:   in,
		marked: make(map[ssa.Node]bool),
	}
	a.bfs(config)
	return a
}

// bfs performs Breadth-First-Search on the def-use graph of the input source.
// While traversing the graph we also look for potential sanitizers of this source.
// If the source passes through a sanitizer, bfs does not continue through that Node.
func (a *source) bfs(config *config.Config) {
	q := queue.New()
	q.Add(a.node)
	a.marked[a.node] = true

	for q.Length() > 0 {
		e := q.Remove().(ssa.Node)

		if c, ok := e.(*ssa.Call); ok && config.IsSanitizer(c) {
			a.sanitizers = append(a.sanitizers, &sanitizer.Sanitizer{Call: c})
			continue
		}

		if e.Referrers() == nil {
			continue
		}

		for _, r := range *e.Referrers() {
			if _, ok := a.marked[r.(ssa.Node)]; ok {
				continue
			}
			a.marked[r.(ssa.Node)] = true

			// Need to stay within the scope of the function under analysis.
			if call, ok := r.(*ssa.Call); ok && !config.IsPropagator(call) {
				continue
			}

			// Do not follow innocuous field access (e.g. Cluster.Zone)
			if addr, ok := r.(*ssa.FieldAddr); ok && !config.IsSourceFieldAddr(addr) {
				continue
			}

			q.Add(r)
		}
	}
}

// hasPathTo returns true when a Node is part of declaration-use graph.
func (a *source) hasPathTo(n ssa.Node) bool {
	return a.marked[n]
}

func (a *source) isSanitizedAt(call ssa.Instruction) bool {
	for _, s := range a.sanitizers {
		if s.Dominates(call) {
			return true
		}
	}

	return false
}

// varargs represents a variable length argument.
// Concretely, it abstract over the fact that the varargs internally are represented by an ssa.Slice
// which contains the underlying values of for vararg members.
// Since many sink functions (ex. log.Info, fmt.Errorf) take a vararg argument, being able to
// get the underlying values of the vararg members is important for this analyzer.
type varargs struct {
	slice   *ssa.Slice
	sources []*source
}

// newVarargs constructs varargs. SSA represents varargs as an ssa.Slice.
func newVarargs(s *ssa.Slice, sources []*source) *varargs {
	a, ok := s.X.(*ssa.Alloc)
	if !ok || a.Comment != "varargs" {
		return nil
	}
	var (
		referredSources []*source
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
			if s.hasPathTo(store.Val.(ssa.Node)) {
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

func (v *varargs) referredByCallWithPattern(patterns []matcher.NameMatcher) *ssa.Call {
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
							sources = append(sources, newSource(a, conf))
						} else {
							//log.V(2).Infof("Adding source: %v %T", v.Value(), v.Value())
							sources = append(sources, newSource(v, conf))
						}

					case conf.IsSink(v):
						// TODO Only variadic sink arguments are currently detected.
						if v.Call.Signature().Variadic() && len(v.Call.Args) > 0 {
							lastArg := v.Call.Args[len(v.Call.Args)-1]
							if varargs, ok := lastArg.(*ssa.Slice); ok {
								if sinkVarargs := newVarargs(varargs, sources); sinkVarargs != nil {
									for _, s := range sinkVarargs.sources {
										if !s.isSanitizedAt(v) {
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

func identifySources(conf *config.Config, ssaInput *buildssa.SSA) map[*ssa.Function][]*source {
	sourceMap := make(map[*ssa.Function][]*source)

	for _, fn := range ssaInput.SrcFuncs {
		var sources []*source
		sources = append(sources, sourcesFromParams(fn, conf)...)
		sources = append(sources, sourcesFromClosure(fn, conf)...)
		sources = append(sources, sourcesFromBlocks(fn, conf)...)

		if len(sources) > 0 {
			sourceMap[fn] = sources
		}
	}
	return sourceMap
}

func sourcesFromParams(fn *ssa.Function, conf *config.Config) []*source {
	var sources []*source
	for _, p := range fn.Params {
		switch t := p.Type().(type) {
		case *types.Pointer:
			if n, ok := t.Elem().(*types.Named); ok && conf.IsSource(n) {
				sources = append(sources, newSource(p, conf))
			}
			// TODO Handle the case where sources arepassed by value: func(c sourceType)
			// TODO Handle cases where PII is wrapped in struct/slice/map
		}
	}
	return sources
}

func sourcesFromClosure(fn *ssa.Function, conf *config.Config) []*source {
	var sources []*source
	for _, p := range fn.FreeVars {
		switch t := p.Type().(type) {
		case *types.Pointer:
			// FreeVars (variables from a closure) appear as double-pointers
			// Hence, the need to dereference them recursively.
			if s, ok := utils.Dereference(t).(*types.Named); ok && conf.IsSource(s) {
				sources = append(sources, newSource(p, conf))
			}
		}
	}
	return sources
}

func sourcesFromBlocks(fn *ssa.Function, conf *config.Config) []*source {
	var sources []*source
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
					sources = append(sources, newSource(v, conf))
				}

				// Handling the case where PII may be in a receiver
				// (ex. func(b *something) { log.Info(something.PII) }
			case *ssa.FieldAddr:
				if conf.IsSource(utils.Dereference(v.Type())) {
					sources = append(sources, newSource(v, conf))
				}
			}
		}
	}
	return sources
}

func report(pass *analysis.Pass, source *source, sink ssa.Node) {
	var b strings.Builder
	b.WriteString("a source has reached a sink")
	fmt.Fprintf(&b, ", source: %v", pass.Fset.Position(source.node.Pos()))
	pass.Reportf(sink.Pos(), b.String())
}
