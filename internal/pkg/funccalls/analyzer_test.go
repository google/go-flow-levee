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

package funccalls

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestLevee(t *testing.T) {
	testData := analysistest.TestData()
	if err := Analyzer.Flags.Set("config", filepath.Join(testData, "test-config.yaml")); err != nil {
		t.Error(err)
	}
	analysistest.Run(t, testData, Analyzer, "./...")
}
