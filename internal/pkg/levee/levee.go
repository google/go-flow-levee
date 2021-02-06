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
	"github.com/google/go-flow-levee/internal/pkg/fieldtags"
	"github.com/google/go-flow-levee/internal/pkg/propagation"
	"github.com/google/go-flow-levee/internal/pkg/source"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name:     "levee",
	Run:      run,
	Flags:    config.FlagSet,
	Doc:      "reports attempts to source data to sinks",
	Requires: []*analysis.Analyzer{source.Analyzer, fieldtags.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}
	funcSources := pass.ResultOf[source.Analyzer].(source.ResultType)
	taggedFields := pass.ResultOf[fieldtags.Analyzer].(fieldtags.ResultType)

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
						reportSourcesReachingSink(conf, pass, propagations, instr)
					}
				case *ssa.Panic:
					if conf.AllowPanicOnTaintedValues {
						continue
					}
					reportSourcesReachingSink(conf, pass, propagations, instr)
				}
			}
		}
	}

	return nil, nil
}

func reportSourcesReachingSink(conf *config.Config, pass *analysis.Pass, propagations map[*source.Source]propagation.Propagation, sink ssa.Instruction) {
	for src, prop := range propagations {
		if prop.IsTainted(sink) {
			report(conf, pass, src, sink.(ssa.Node), prop)
			break
		}
	}
}

func report(conf *config.Config, pass *analysis.Pass, source *source.Source, sink ssa.Node, prop propagation.Propagation) {
	var b strings.Builder
	b.WriteString("a source has reached a sink")
	fmt.Fprintf(&b, "\n source: %v", pass.Fset.Position(source.Node.Pos()))

	// TODO make toggle in config
	if printTrace := true; printTrace {
		// TODO validation
		path, _ := prop.TaintPathTo(sink)

		// TODO discuss what a good first iteration of tracing is
		// Here, we only display the position of nodes with a Pos().
		// Additionally, we filter out positions that have the same file and linenumber as the previous.
		prevPos := pass.Fset.Position(source.Node.Pos())
		for i := len(path) - 2; i > 0; i-- {
			if path[i].Pos() == 0 {
				continue
			}
			position := pass.Fset.Position(path[i].Pos())
			if prevPos.Filename == position.Filename &&
				prevPos.Line == position.Line {
				continue
			}

			fmt.Fprintf(&b, "\n taints: %v", position)
			prevPos = position
		}
	}

	fmt.Fprintf(&b, "\n sinks at: %v", pass.Fset.Position(sink.Pos()))
	if conf.ReportMessage != "" {
		fmt.Fprintf(&b, "\n %v", conf.ReportMessage)
	}
	s := b.String()
	pass.Reportf(sink.Pos(), s)
}
