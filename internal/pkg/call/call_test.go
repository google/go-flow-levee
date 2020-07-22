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
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const source = `package test
func NoArgs() {}
func OneArg(one int) {}
func CallNoArgs() {
	NoArgs()
}
func CallOneArg() {
	OneArg(0)
}
`

type fakeReferrer struct {
	hasPathTo bool
}

func (f fakeReferrer) HasPathTo(node ssa.Node) bool { return f.hasPathTo }

func TestRegularCallWithNoArgsIsNotReferredBy(t *testing.T) {
	ssaCall := getCall(source, "CallNoArgs")
	call := Regular(ssaCall)
	got := call.ReferredBy(fakeReferrer{true})
	want := false
	if got != want {
		t.Errorf("call.ReferredBy(%v) == %v, want %v", call, got, want)
	}
}

func TestRegularCallReferredBy(t *testing.T) {
	cases := []struct {
		r    Referrer
		want bool
	}{
		{fakeReferrer{true}, true},
		{fakeReferrer{false}, false},
	}
	for _, c := range cases {
		ssaCall := getCall(source, "CallOneArg")
		call := Regular(ssaCall)
		got := call.ReferredBy(c.r)
		if got != c.want {
			t.Errorf("call.ReferredBy(%v) == %v, want %v", call, got, c.want)
		}
	}
}

// taken from golang.org/x/tools/go/ssa/example_test.go
func getCall(source, parentFuncName string) *ssa.Call {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	files := []*ast.File{f}

	pkg := types.NewPackage("test", "")
	ssaPkg, _, err := ssautil.BuildPackage(
		&types.Config{Importer: importer.Default()}, fset, pkg, files, ssa.SanityCheckFunctions)
	if err != nil {
		panic(err)
	}

	fun := ssaPkg.Func(parentFuncName)
	if fun == nil {
		panic("did not find function named Call")
	}

	for _, i := range fun.Blocks[0].Instrs {
		call, ok := i.(*ssa.Call)
		if ok {
			return call
		}
	}

	panic("did not find call instruction")
}
