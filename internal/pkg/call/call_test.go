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

package call

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/ioutil"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type fakeReferrer struct {
	hasPathTo bool
}

func (f fakeReferrer) RefersTo(node ssa.Node) bool { return f.hasPathTo }

func TestRegularCallReferredBy(t *testing.T) {
	source := readFromTestData(t, "test.go")
	cases := []struct {
		desc           string
		r              Referrer
		parentFuncName string
		want           bool
	}{
		{"call without args", fakeReferrer{true}, "CallNoArgs", false},
		{"call with one arg that is referred to by a referrer", fakeReferrer{true}, "CallOneArg", true},
		{"call with one arg that isn't referred to", fakeReferrer{false}, "CallOneArg", false},
	}
	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			ssaCall := getCall(t, source, tt.parentFuncName)
			call := Regular(ssaCall)
			got := call.ReferredBy(tt.r)
			if got != tt.want {
				t.Errorf("call.ReferredBy(%v) == %v, want %v", call, got, tt.want)
			}
		})
	}
}

func readFromTestData(t *testing.T, filename string) string {
	t.Helper()

	filePath := filepath.Join("testdata", filename)
	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes)
}

// taken from golang.org/x/tools/go/ssa/example_test.go
func getCall(t *testing.T, source, parentFuncName string) *ssa.Call {
	t.Helper()

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	files := []*ast.File{f}

	pkg := types.NewPackage("test", "")
	ssaPkg, _, err := ssautil.BuildPackage(
		&types.Config{Importer: importer.Default()}, fset, pkg, files, ssa.SanityCheckFunctions)
	if err != nil {
		t.Fatal(err)
	}

	fun := ssaPkg.Func(parentFuncName)
	if fun == nil {
		t.Fatal(fmt.Sprintf("did not find function named %s", parentFuncName))
	}

	for _, i := range fun.Blocks[0].Instrs {
		call, ok := i.(*ssa.Call)
		if ok {
			return call
		}
	}

	t.Fatal(fmt.Sprintf("did not find call instruction in function named %s", parentFuncName))
	return nil // unreachable
}
