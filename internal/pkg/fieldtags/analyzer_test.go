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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestFieldTagsAnalysis(t *testing.T) {
	testdata := analysistest.TestData()

	results := analysistest.Run(t, testdata, Analyzer, "tests")

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	want := []string{
		"password",
		"creds",
		"secret",
		"another",
	}

	var got []string
	for o := range results[0].Result.(ResultType) {
		got = append(got, o.Name())
	}

	if diff := cmp.Diff(want, got, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
		t.Errorf("Tagged Fields diff (-want +got):\n%s", diff)
	}
}
