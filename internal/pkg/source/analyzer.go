// Copyright 2020 Google LLC
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

package source

import (
	"reflect"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/sourcetype"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

type ResultType = map[*ssa.Function][]*Source

var Analyzer = &analysis.Analyzer{
	Name:       "source",
	Doc:        "This analyzer identifies ssa.Values as dataflow sources.",
	Flags:      config.FlagSet,
	Run:        run,
	Requires:   []*analysis.Analyzer{buildssa.Analyzer, sourcetype.Analyzer},
	ResultType: reflect.TypeOf(new(ResultType)).Elem(),
}

func run(pass *analysis.Pass) (interface{}, error) {
	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	stc := pass.ResultOf[sourcetype.Analyzer].(config.SourceTypeClassifier)
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	sourceMap := identify(stc, ssaInput)

	// TODO source.bfs() logic should be performed downstream, as it is not part of source identification
	for _, srcs := range sourceMap {
		for _, s := range srcs {
			s.config = conf
			s.marked = make(map[ssa.Node]bool)
			s.bfs()
		}
	}

	for _, srcs := range sourceMap {
		for _, s := range srcs {
			pass.Reportf(s.node.Pos(), "source identified")
		}
	}

	return sourceMap, nil
}
