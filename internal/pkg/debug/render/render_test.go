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

package render

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestDOT(t *testing.T) {
	testGoldenFiles(t, DOT, "DOT", ".dot")
}

func TestSSA(t *testing.T) {
	testGoldenFiles(t, SSA, "SSA", ".ssa")
}

// testGoldenFiles tests the output of a rendering function against the golden files under testdata.
func testGoldenFiles(t *testing.T, fn func(f *ssa.Function) string, fnName, ext string) {
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	testfile := filepath.Join(testdata, "tests.go")
	ssaFuncs := extractSSAFuncs(t, testfile)
	for _, f := range ssaFuncs {
		goldenFilename := filepath.Join(testdata, f.Name()) + ext
		bytes, err := ioutil.ReadFile(goldenFilename)
		if err != nil {
			t.Fatal(err)
		}
		want := string(bytes)

		got := fn(f)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("render.%s(%s) diff (-want +got):\n%s", fnName, f.Name(), diff)
		}
	}
}

func extractSSAFuncs(t *testing.T, testfile string) []*ssa.Function {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, testfile, nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	files := []*ast.File{file}

	pkg := types.NewPackage(file.Name.Name, "")
	ssaPkg, _, err := ssautil.BuildPackage(
		&types.Config{Importer: importer.Default()}, fset, pkg, files, ssa.SanityCheckFunctions)
	if err != nil {
		t.Fatal(err)
	}

	var functions []*ssa.Function
	for _, m := range ssaPkg.Members {
		if f, ok := m.(*ssa.Function); ok && !strings.HasPrefix(f.Name(), "init") {
			functions = append(functions, f)
		}
	}
	return functions
}
