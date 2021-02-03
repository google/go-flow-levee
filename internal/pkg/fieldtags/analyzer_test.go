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

package fieldtags

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-flow-levee/internal/pkg/config"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestFieldTagsAnalysis(t *testing.T) {
	testdata := analysistest.TestData()

	if err := config.FlagSet.Set("config", filepath.Join(testdata, "test-config.yaml")); err != nil {
		t.Error(err)
	}

	results := analysistest.Run(t, testdata, Analyzer, "fieldtags_analysistest/core", "fieldtags_analysistest/crosspkg")

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	var gotCoreResults []string
	for obj := range results[0].Result.(ResultType) {
		gotCoreResults = append(gotCoreResults, obj.Name())
	}
	wantCoreResults := []string{
		"adminSecret",
		"another",
		"creds",
		"hasCustomFieldTag",
		"hasTagWithMultipleValues",
		"password",
		"secret",
	}
	if diff := cmp.Diff(wantCoreResults, gotCoreResults, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
		t.Errorf("core results diff (-want +got):\n%s", diff)
	}

	var gotCrosspkgResults []string
	for obj := range results[1].Result.(ResultType) {
		gotCrosspkgResults = append(gotCrosspkgResults, obj.Name())
	}
	wantCrosspkgResults := append(wantCoreResults, "crossField")
	if diff := cmp.Diff(wantCrosspkgResults, gotCrosspkgResults, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
		t.Errorf("crosspkg results diff (-want +got):\n%s", diff)
	}
}
