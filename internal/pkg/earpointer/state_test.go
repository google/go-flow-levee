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

package earpointer_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"sort"
	"testing"

	"github.com/google/go-flow-levee/internal/pkg/earpointer"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// parseSourceCode parses the source code, convert it to SSA form, and return the SSA package.
// The filename for source is always "test.go", and the package is "t" at an empty path.
func parseSourceCode(src string) (*ssa.Package, error) {
	// Parse the source files.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	files := []*ast.File{f}

	// Create the type-checker's package.
	pkg := types.NewPackage("t", "")
	// Type-check the package, load dependencies.
	// Create and build the SSA program.
	ssaPkg, _, err := ssautil.BuildPackage(
		&types.Config{Importer: importer.Default()}, fset, pkg, files, ssa.SanityCheckFunctions)
	if err != nil {
		return nil, err
	}
	return ssaPkg, nil
}

// Test basic state operations using single global variables.
// No fields are involved.
func TestBasic(t *testing.T) {
	code := `package p;
	type T struct {}
	var g1 *int
	var g2 *int
	var g3 T
	`
	pkg, err := parseSourceCode(code)
	if err != nil {
		t.Fatalf("compilation failed: %s", code)
	}
	state := earpointer.NewState()
	g1 := pkg.Members["g1"].(*ssa.Global)
	ref1 := earpointer.GetReferenceForGlobal(g1)
	g2 := pkg.Members["g2"].(*ssa.Global)
	ref2 := earpointer.GetReferenceForGlobal(g2)
	g3 := pkg.Members["g3"].(*ssa.Global)
	ref3 := earpointer.GetReferenceForGlobal(g3)
	state.Insert(ref1)
	state.Insert(ref2)
	state.Insert(ref3)

	got := state.String()
	want := "{t.g1}: [], {t.g2}: [], {t.g3}: []"
	if got != want {
		t.Errorf("initial state:\n got: %s\n want: %s", got, want)
	}

	state.Unify(ref1, ref2)
	got = state.String()
	want = "{t.g1,t.g2}: [], {t.g3}: []"
	if got != want {
		t.Errorf("after unifying %s and %s:\n got: %s\n want: %s", ref1, ref2, got, want)
	}

	reps := state.AllPartitionReps()
	if len(reps) != 2 {
		t.Errorf("after unification, the number of representatives should be 2")
	}

	// The members information is built after the state is finalized.
	state.Finalize()
	if state.GetPartitionRep(ref1) != state.GetPartitionRep(ref2) {
		t.Errorf("after unification, g1 and g2 should have the same representative")
	}
	members := state.GetPartitionMembers(ref1)
	if len(members) != 2 {
		t.Errorf("the number of members in g1's partition should be 2")
	}
	mstrs := []string{members[0].String(), members[1].String()}
	sort.Strings(mstrs)
	if mstrs[0] != "t.g1" || mstrs[1] != "t.g2" {
		t.Errorf("the member set of g1's partition should be [t.g1, t.g2]")
	}
}

// Test field unification on global variables.
func TestGlobalField(t *testing.T) {
	code := `package p;
	type T struct { x *int; y *int }
	var g1 *T
	var g2 *T
	var g3 *T
	var g4 *T
	`
	pkg, err := parseSourceCode(code)
	if err != nil {
		t.Fatalf("compilation failed: %s", code)
	}
	state := earpointer.NewState()
	g1 := pkg.Members["g1"].(*ssa.Global)
	ref1 := earpointer.GetReferenceForGlobal(g1)
	g2 := pkg.Members["g2"].(*ssa.Global)
	ref2 := earpointer.GetReferenceForGlobal(g2)
	g3 := pkg.Members["g3"].(*ssa.Global)
	ref3 := earpointer.GetReferenceForGlobal(g3)
	g4 := pkg.Members["g4"].(*ssa.Global)
	ref4 := earpointer.GetReferenceForGlobal(g4)
	state.Insert(ref1)
	state.Insert(ref2)
	state.Insert(ref3)
	state.Insert(ref4)

	fm1 := state.GetPartitionFieldMap(ref1)
	fm1[earpointer.Field{Name: "x"}] = ref3
	got := state.String()
	want := "{t.g1}: [x->t.g3], {t.g2}: [], {t.g3}: [], {t.g4}: []"
	if got != want {
		t.Errorf("initial state:\n got: %s\n want: %s", got, want)
	}

	// Unify "g1" and "g2"
	state.Unify(ref1, ref2)
	got = state.String()
	want = "{t.g1,t.g2}: [x->t.g3], {t.g3}: [], {t.g4}: []"
	if got != want {
		t.Errorf("after unifying %s and %s:\n got: %s\n want: %s", ref1, ref2, got, want)
	}

	// Further unify "g3" and "g4"
	fm3 := state.GetPartitionFieldMap(ref3)
	fm3[earpointer.Field{Name: "y"}] = ref1
	fm4 := state.GetPartitionFieldMap(ref4)
	fm4[earpointer.Field{Name: "y"}] = ref2
	state.Unify(ref3, ref4)
	got = state.String()
	want = "{t.g1,t.g2}: [x->t.g3], {t.g3,t.g4}: [y->t.g2]"
	if got != want {
		t.Errorf("after unifying %s and %s:\n got: %s\n want: %s", ref3, ref4, got, want)
	}
}

// Test field unification on local variables including parameters and registers.
func TestLocalField(t *testing.T) {
	code := `package p;
	type T struct { x *int; y *int }
	func f(a *T, b *T) {
		var c T
		_ = c
	}
	`
	pkg, err := parseSourceCode(code)
	if err != nil {
		t.Fatalf("compilation failed: %s", code)
	}
	state := earpointer.NewState()
	f := pkg.Members["f"].(*ssa.Function)
	var emptyContext earpointer.ReferenceContext
	refa := earpointer.GetReferenceForLocal(&emptyContext, f.Params[0])
	state.Insert(refa)
	refb := earpointer.GetReferenceForLocal(&emptyContext, f.Params[1])
	state.Insert(refb)
	refc := earpointer.GetReferenceForLocal(&emptyContext, f.Locals[0])
	state.Insert(refc)

	got := state.String()
	want := "{f.a}: [], {f.b}: [], {f.t0}: []"
	if got != want {
		t.Errorf("initial state:\n got: %s\n want: %s", got, want)
	}

	fma := state.GetPartitionFieldMap(refa)
	fma[earpointer.Field{Name: "x"}] = refc
	fma[earpointer.Field{Name: "y"}] = refb
	fmc := state.GetPartitionFieldMap(refc)
	fmc[earpointer.Field{Name: "y"}] = refa
	got = state.String()
	want = "{f.a}: [x->f.t0, y->f.b], {f.b}: [], {f.t0}: [y->f.a]"
	if got != want {
		t.Errorf("after setting fields:\n got: %s\n want: %s", got, want)
	}

	// Unify parameter "a" and local "c".
	state.Unify(refa, refc)
	got = state.String()
	want = "{f.a,f.b,f.t0}: [x->f.t0, y->f.a]"
	if got != want {
		t.Errorf("after unifying %s and %s:\n got: %s\n want: %s", refa, refc, got, want)
	}
}

func TestInternalReference(t *testing.T) {
	code := `package p;
	var g1 *int
	`
	pkg, err := parseSourceCode(code)
	if err != nil {
		t.Fatalf("compilation failed: %s", code)
	}
	g1 := pkg.Members["g1"].(*ssa.Global)
	ref1 := earpointer.GetReferenceForGlobal(g1)

	i1 := earpointer.GetReferenceForInternal(earpointer.VALUEOF, ref1)
	i2 := earpointer.GetReferenceForInternal(earpointer.VALUEOF, ref1)
	if i1 != i2 {
		t.Errorf("internal reference [%s] should be equal to [%s]", i1, i2)
	}
	got := i1.String()
	if want := "*t.g1"; got != want {
		t.Errorf("internal reference:\n got: %s\n want: %s", got, want)
	}

	i3 := earpointer.GetReferenceForInternal(earpointer.FIELD, ref1)
	if i1 == i3 {
		t.Errorf("internal reference [%s] should not be equal to [%s]", i1, i2)
	}
	got = i3.String()
	if want := "t.g1[.]"; got != want {
		t.Errorf("internal reference:\n got: %s\n want: %s", got, want)
	}
}
