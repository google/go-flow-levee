// Copyright 2021 Google LLC
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
	"testing"

	"github.com/google/go-flow-levee/internal/pkg/debug"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestLeveeEAR(t *testing.T) {
	dataDir := analysistest.TestData()
	if err := Analyzer.Flags.Set("config", dataDir+"/test-ear-config.yaml"); err != nil {
		t.Error(err)
	}
	if *debugging {
		Analyzer.Requires = append(Analyzer.Requires, debug.Analyzer)
	}

	// TODO(#318): separate the unit-tests for the two taint analyses.
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/arguments")
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/basictypes")
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/binop")
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/booleans")
	// analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/callorder")  // TODO: flow-sensitive
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/closures")
	// analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/collections") // TODO: map key
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/declarations")
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/eface")
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/embedding")
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/excludedpackage")
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/extracts")
	// analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/fields")  // TODO: FP has been fixed?
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/includedpackage")
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/inlining")
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/loops")

	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/namedreturn")
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/panic")

	// analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/phi")  // TODO: flow sensitive
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/pointers")
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/position")
	// analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/propagation") // TODO: flow sensitive
	// analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/propagation/builtins.go") // TODO: flow sensitive

	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/receivers")
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/recover")
	// analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/sanitization")  // TODO: santizers are not handled yet
	// analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/sinks")  // TODO
	// analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/store")  // TODO: flow sensitive
	// analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/structlit")  // TODO: NP have been fixed?
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/tests/typealias")
}

func TestLeveeEARInter(t *testing.T) {
	dataDir := analysistest.TestData()
	if err := Analyzer.Flags.Set("config", dataDir+"/test-ear-config.yaml"); err != nil {
		t.Error(err)
	}

	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/ear/tests/...")
}
