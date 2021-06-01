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

package levee

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis/passes/buildssa"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/earpointer"
	"github.com/google/go-flow-levee/internal/pkg/fieldtags"
	"github.com/google/go-flow-levee/internal/pkg/propagation"
	"github.com/google/go-flow-levee/internal/pkg/source"
	"github.com/google/go-flow-levee/internal/pkg/suppression"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/ssa"
)

// Whether to use EAR pointer analysis as the taint propagation engine.
var useEAR bool

func init() {
	config.FlagSet.BoolVar(&useEAR, "useEAR", false, "Use EAR pointer analysis as the taint propagation engine (default=false)")
}

var Analyzer = &analysis.Analyzer{
	Name:  "levee",
	Run:   run,
	Flags: config.FlagSet,
	Doc:   "reports attempts to source data to sinks",
	Requires: []*analysis.Analyzer{
		fieldtags.Analyzer,
		source.Analyzer,
		suppression.Analyzer,
		buildssa.Analyzer,
	},
}

func run(pass *analysis.Pass) (interface{}, error) {
	if useEAR {
		ssainput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
		// Note: some ssainput.SrcFuncs are not within ssautil.AllFunctions(ssa.prog)
		p := earpointer.Analyze(ssainput)
		if p == nil {
			return nil, fmt.Errorf("no valid EAR partitions")
		}
		return runEAR(pass, p) // Use the EAR-pointer based taint analysis
	} else {
		return runPropagation(pass) // Use the propagation based taint analysis
	}
}

func runPropagation(pass *analysis.Pass) (interface{}, error) {
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}
	funcSources := pass.ResultOf[source.Analyzer].(source.ResultType)
	taggedFields := pass.ResultOf[fieldtags.Analyzer].(fieldtags.ResultType)
	suppressedNodes := pass.ResultOf[suppression.Analyzer].(suppression.ResultType)

	for fn, sources := range funcSources {
		propagations := make(map[*source.Source]propagation.Propagation, len(sources))
		for _, s := range sources {
			propagations[s] = propagation.Taint(s.Node, conf, taggedFields)
		}

		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				switch v := instr.(type) {
				case *ssa.Call:
					if callee := v.Call.StaticCallee(); callee != nil && conf.IsSink(utils.DecomposeFunction(callee)) {
						reportSourcesReachingSink(conf, pass, suppressedNodes, propagations, instr)
					}
				case *ssa.Panic:
					if conf.AllowPanicOnTaintedValues {
						continue
					}
					reportSourcesReachingSink(conf, pass, suppressedNodes, propagations, instr)
				}
			}
		}
	}

	return nil, nil
}

// Use the EAR pointer analysis as the propagation engine
func runEAR(pass *analysis.Pass, state *earpointer.Partitions) (interface{}, error) {
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}
	funcSources := pass.ResultOf[source.Analyzer].(source.ResultType)
	taggedFields := pass.ResultOf[fieldtags.Analyzer].(fieldtags.ResultType)
	suppressedNodes := pass.ResultOf[suppression.Analyzer].(suppression.ResultType)
	// Return whether a field is tainted.
	isTaintField := func(named *types.Named, index int) bool {
		if tt, ok := named.Underlying().(*types.Struct); ok {
			return conf.IsSourceField(utils.DecomposeField(named, index)) ||
				taggedFields.IsSourceField(tt, index)
		}
		return false
	}
	// Collect all the taint sources
	for fn, sources := range funcSources {
		// Transitively get the set of functions called within "fn".
		// This set is used to narrow down the set of references needed to be
		// considered during EAR heap traversal. It can also help reducing the
		// false positives.
		callees := make(map[*ssa.Function]bool)
		callees[fn] = true
		earpointer.GetCalleeFunctions(fn, callees, 7)

		var fnSrcs []*source.Source
		srcRefs := make(map[*source.Source]earpointer.ReferenceSet)
		for _, s := range sources {
			fnSrcs = append(fnSrcs, s)
			srcRefs[s] = earpointer.GetSrcRefs(s, state, isTaintField, callees)
		}
		// Traverse all the callee functions (not just the ones with sink sources)
		for member := range callees {
			for _, b := range member.Blocks {
				for _, instr := range b.Instrs {
					switch v := instr.(type) {
					case *ssa.Call:
						if callee := v.Call.StaticCallee(); callee != nil &&
							conf.IsSink(utils.DecomposeFunction(callee)) {
							sink := instr
							for _, src := range fnSrcs {
								if earpointer.IsEARTainted(sink, srcRefs[src], state, callees) &&
									!isSuppressed(sink.Pos(), suppressedNodes, pass) {
									report(conf, pass, src, sink.(ssa.Node))
									break
								}
							}
						}
					case *ssa.Panic:
						if conf.AllowPanicOnTaintedValues {
							continue
						}
						sink := instr
						for _, src := range fnSrcs {
							if earpointer.IsEARTainted(sink, srcRefs[src], state, callees) &&
								!isSuppressed(sink.Pos(), suppressedNodes, pass) {
								report(conf, pass, src, sink.(ssa.Node))
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

func reportSourcesReachingSink(conf *config.Config, pass *analysis.Pass, suppressedNodes suppression.ResultType, propagations map[*source.Source]propagation.Propagation, sink ssa.Instruction) {
	for src, prop := range propagations {
		if prop.IsTainted(sink) && !isSuppressed(sink.Pos(), suppressedNodes, pass) {
			report(conf, pass, src, sink.(ssa.Node))
			break
		}
	}
}

func isSuppressed(pos token.Pos, suppressedNodes suppression.ResultType, pass *analysis.Pass) bool {
	for _, f := range pass.Files {
		if pos < f.Pos() || f.End() < pos {
			continue
		}
		// astutil.PathEnclosingInterval produces the list of nodes that enclose the provided
		// position, from the leaf node that directly contains it up to the ast.File node
		path, _ := astutil.PathEnclosingInterval(f, pos, pos)
		if len(path) < 2 {
			return false
		}
		// Given the position of a call, path[0] holds the ast.CallExpr and
		// path[1] holds the ast.ExprStmt. A suppressing comment may be associated
		// with the name of the function being called (Ident, SelectorExpr), with the
		// call itself (CallExpr), or with the entire expression (ExprStmt).
		if ce, ok := path[0].(*ast.CallExpr); ok {
			switch t := ce.Fun.(type) {
			case *ast.Ident:
				/*
					Sink( // levee.DoNotReport
				*/
				if suppressedNodes.IsSuppressed(t) {
					return true
				}
			case *ast.SelectorExpr:
				/*
					core.Sink( // levee.DoNotReport
				*/
				if suppressedNodes.IsSuppressed(t.Sel) {
					return true
				}
			}
		} else {
			fmt.Printf("unexpected node received: %v (type %T); please report this issue\n", path[0], path[0])
		}
		return suppressedNodes.IsSuppressed(path[0]) || suppressedNodes.IsSuppressed(path[1])
	}
	return false
}

func report(conf *config.Config, pass *analysis.Pass, source *source.Source, sink ssa.Node) {
	var b strings.Builder
	b.WriteString("a source has reached a sink")
	fmt.Fprintf(&b, "\n source: %v", pass.Fset.Position(source.Pos()))
	if conf.ReportMessage != "" {
		fmt.Fprintf(&b, "\n %v", conf.ReportMessage)
	}
	pass.Reportf(sink.Pos(), b.String())
}
