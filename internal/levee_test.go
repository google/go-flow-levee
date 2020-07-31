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
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

var patterns = []string{
	"example.com/tests/arguments",
	"example.com/tests/collections",
	"example.com/tests/declarations",
	"example.com/tests/dominance",
	"example.com/tests/fields",
	"example.com/tests/receivers",
	"example.com/tests/sinks",
}

func TestLevee(t *testing.T) {
	dir := analysistest.TestData()
	if err := Analyzer.Flags.Set("config", dir+"/test-config.json"); err != nil {
		t.Error(err)
	}
	analysistest.Run(t, dir, Analyzer, patterns...)
}
