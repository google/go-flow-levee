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

package fieldpropagator

import (
	"path/filepath"
	"testing"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestFieldPropagatorAnalysis(t *testing.T) {
	testdata := analysistest.TestData()
	if err := config.FlagSet.Set("config", filepath.Join(testdata, "test-config.yaml")); err != nil {
		t.Error(err)
	}
	analysistest.Run(t, testdata, Analyzer, "source", "test")
}
