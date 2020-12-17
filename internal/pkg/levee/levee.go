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
	"github.com/google/go-flow-levee/internal/pkg/levee/propagation"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"

	"github.com/google/go-flow-levee/internal/pkg/source"
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
	sourcesMap := pass.ResultOf[source.Analyzer].(source.ResultType)
	taggedFields := pass.ResultOf[fieldtags.Analyzer].(fieldtags.ResultType)

	propagations := map[*source.Source]propagation.Propagation{}

	for _, sources := range sourcesMap {
		for _, s := range sources {
			propagations[s] = propagation.Dfs(s.Node, conf, taggedFields)
		}
	}

	for fn := range sourcesMap {
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				v, ok := instr.(*ssa.Call)
				if !ok {
					continue
				}

				callee := v.Call.StaticCallee()
				switch {
				case callee != nil && conf.IsSink(utils.DecomposeFunction(callee)):
					for s, prop := range propagations {
						if prop.HasPathTo(instr.(ssa.Node)) && !prop.IsSanitizedAt(v) {
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

func report(pass *analysis.Pass, source *source.Source, sink ssa.Node) {
	var b strings.Builder
	b.WriteString("a source has reached a sink")
	fmt.Fprintf(&b, ", source: %v", pass.Fset.Position(source.Pos()))
	pass.Reportf(sink.Pos(), b.String())
}
