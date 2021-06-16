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

// buildSSA parses the source code, convert it to SSA form, and return the SSA package.
// The filename for source is always "test.go", and the package is "t" at an empty path.
func buildSSA(src string) (*ssa.Package, error) {
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
	code := `package p
	type T struct {}
	var g1 *int
	var g2 *int
	var g3 T
	`
	pkg, err := buildSSA(code)
	if err != nil {
		t.Fatalf("compilation failed: %s", code)
	}
	state := earpointer.NewState()
	getGlobal := func(name string) earpointer.Reference {
		return earpointer.MakeGlobal(pkg.Members[name].(*ssa.Global))
	}
	refs := map[string]earpointer.Reference{
		"g1": getGlobal("g1"),
		"g2": getGlobal("g2"),
		"g3": getGlobal("g3"),
	}
	for _, r := range refs {
		state.Insert(r)
	}

	want := "{g1}: [], {g2}: [], {g3}: []"
	if got := state.String(); got != want {
		t.Errorf("initial:\n got: %s\n want: %s", got, want)
	}

	state.Unify(refs["g1"], refs["g2"])
	want = "{g1,g2}: [], {g3}: []"
	if got := state.String(); got != want {
		t.Errorf("after unifying g1 and g2:\n got: %s\n want: %s", got, want)
	}

	// Test the Partitions.
	// The members information is built after the state is finalized.
	partitions := state.ToPartitions()
	if !partitions.Has(refs["g1"]) {
		t.Errorf("g1 should be in the partitions")
	}
	if len(partitions.References()) != 3 {
		t.Errorf("partitions should contain 3 references")
	}
	if partitions.NumPartitions() != 2 {
		t.Errorf("should have 2 partitions")
	}
	members := partitions.PartitionMembers(refs["g1"])
	if len(members) != 2 {
		t.Errorf("g1's partition should have 2 members")
	}
	mstrs := []string{members[0].String(), members[1].String()}
	sort.Strings(mstrs)
	if mstrs[0] != "g1" || mstrs[1] != "g2" {
		t.Errorf("g1's partition should contain [g1, g2]")
	}
	want = "{g1,g2}: [], {g3}: []"
	if got := partitions.String(); got != want {
		t.Errorf("after unifying g1 and g2:\n got: %s\n want: %s", got, want)
	}
}

// Test field unification on global variables.
func TestGlobalField(t *testing.T) {
	code := `package p
	type T struct { x *int; y *int }
	var g1 *T
	var g2 *T
	var g3 *T
	var g4 *T
	`
	pkg, err := buildSSA(code)
	if err != nil {
		t.Fatalf("compilation failed: %s", code)
	}
	state := earpointer.NewState()
	getGlobal := func(name string) earpointer.Reference {
		return earpointer.MakeGlobal(pkg.Members[name].(*ssa.Global))
	}
	refs := map[string]earpointer.Reference{
		"g1": getGlobal("g1"),
		"g2": getGlobal("g2"),
		"g3": getGlobal("g3"),
		"g4": getGlobal("g4"),
	}
	for _, r := range refs {
		state.Insert(r)
	}

	fm1 := state.PartitionFieldMap(refs["g1"])
	fm1[earpointer.Field{Name: "x"}] = refs["g3"]
	want := "{g1}: [x->g3], {g2}: [], {g3}: [], {g4}: []"
	if got := state.String(); got != want {
		t.Errorf("initial:\n got: %s\n want: %s", got, want)
	}

	// Unify "g3" and "g4", which will also unify "g1" and "g2"
	fm3 := state.PartitionFieldMap(refs["g3"])
	fm3[earpointer.Field{Name: "y"}] = refs["g1"]
	fm4 := state.PartitionFieldMap(refs["g4"])
	fm4[earpointer.Field{Name: "y"}] = refs["g2"]
	state.Unify(refs["g3"], refs["g4"])
	want = "{g1,g2}: [x->g3], {g3,g4}: [y->g2]"
	if got := state.String(); got != want {
		t.Errorf("after unifying g3 and g4:\n got: %s\n want: %s", got, want)
	}

	// Test the Partitions.
	partitions := state.ToPartitions()
	want = "{g1,g2}: [x->g4], {g3,g4}: [y->g2]"
	if got := partitions.String(); got != want {
		t.Errorf("partitions:\n got: %s\n want: %s", got, want)
	}
}

// Test field unification on local variables including parameters and registers.
func TestLocalField(t *testing.T) {
	code := `package p
	type T struct { x *int; y *int }
	func f(a *T, b *T) {
		var c T
		_ = c
	}
	`
	pkg, err := buildSSA(code)
	if err != nil {
		t.Fatalf("compilation failed: %s", code)
	}
	state := earpointer.NewState()
	f := pkg.Members["f"].(*ssa.Function)
	var emptyContext earpointer.Context
	getLocal := func(param ssa.Value) earpointer.Reference {
		r := earpointer.MakeLocal(&emptyContext, param)
		state.Insert(r)
		return r
	}
	refa := getLocal(f.Params[0])
	refb := getLocal(f.Params[1])
	refc := getLocal(f.Locals[0])

	want := "{f.a}: [], {f.b}: [], {f.t0}: []"
	if got := state.String(); got != want {
		t.Errorf("initial:\n got: %s\n want: %s", got, want)
	}

	fma := state.PartitionFieldMap(refa)
	fma[earpointer.Field{Name: "x"}] = refc
	fma[earpointer.Field{Name: "y"}] = refb
	fmc := state.PartitionFieldMap(refc)
	fmc[earpointer.Field{Name: "y"}] = refa
	want = "{f.a}: [x->f.t0, y->f.b], {f.b}: [], {f.t0}: [y->f.a]"
	if got := state.String(); got != want {
		t.Errorf("after setting fields:\n got: %s\n want: %s", got, want)
	}

	// unify parameter "a" and local "c".
	state.Unify(refa, refc)
	want = "{f.a,f.b,f.t0}: [x->f.t0, y->f.a]"
	if got := state.String(); got != want {
		t.Errorf("after unifying 'a' and 'b':\n got: %s\n want: %s", got, want)
	}

	// Test the Partitions.
	partitions := state.ToPartitions()
	want = "{f.a,f.b,f.t0}: [x->f.t0, y->f.t0]"
	if got := partitions.String(); got != want {
		t.Errorf("partitions:\n got: %s\n want: %s", got, want)
	}
}

func TestSyntheticReference(t *testing.T) {
	code := `package p
	var g1 *int
	`
	pkg, err := buildSSA(code)
	if err != nil {
		t.Fatalf("compilation failed: %s", code)
	}
	g1 := pkg.Members["g1"].(*ssa.Global)
	ref1 := earpointer.MakeGlobal(g1)

	i1 := earpointer.MakeSynthetic(earpointer.SyntheticValueOf, ref1)
	i2 := earpointer.MakeSynthetic(earpointer.SyntheticValueOf, ref1)
	if i1 != i2 {
		t.Errorf("[%s] != [%s]", i1, i2)
	}
	want := "*g1"
	if got := i1.String(); got != want {
		t.Errorf("String():\n got: %s\n want: %s", got, want)
	}

	i3 := earpointer.MakeSynthetic(earpointer.SyntheticField, ref1)
	if i1 == i3 {
		t.Errorf("[%s] == [%s]", i1, i3)
	}
	want = "g1[.]"
	if got := i3.String(); got != want {
		t.Errorf("String():\n got: %s\n want: %s", got, want)
	}
}
