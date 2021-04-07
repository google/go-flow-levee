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
	"flag"
	"testing"

	"github.com/google/go-flow-levee/internal/pkg/debug"
	"golang.org/x/tools/go/analysis/analysistest"
)

var debugging *bool = flag.Bool("debug", false, "run the debug analyzer")

func TestLevee(t *testing.T) {
	dataDir := analysistest.TestData()
	if err := Analyzer.Flags.Set("config", dataDir+"/test-config.yaml"); err != nil {
		t.Error(err)
	}
	if *debugging {
		Analyzer.Requires = append(Analyzer.Requires, debug.Analyzer)
	}
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/example/...")
}

func TestLeveeDoesNotCreateReportsForPanicIfPanicingOnTaintedValuesIsAllowed(t *testing.T) {
	dataDir := analysistest.TestData()
	if err := Analyzer.Flags.Set("config", dataDir+"/allowpanicontaintedvalues-config.yaml"); err != nil {
		t.Error(err)
	}
	if *debugging {
		Analyzer.Requires = append(Analyzer.Requires, debug.Analyzer)
	}
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/nopanic.com/...")
}

func TestFormattingWithCustomReportMessage(t *testing.T) {
	dataDir := analysistest.TestData()
	if err := Analyzer.Flags.Set("config", dataDir+"/with-custom-message.yaml"); err != nil {
		t.Error(err)
	}
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/custom.message.com/withcustom")
}

func TestFormattingWithoutCustomReportMessage(t *testing.T) {
	dataDir := analysistest.TestData()
	if err := Analyzer.Flags.Set("config", dataDir+"/no-custom-message.yaml"); err != nil {
		t.Error(err)
	}
	analysistest.Run(t, dataDir, Analyzer, "./src/levee_analysistest/custom.message.com/nocustom")
}
