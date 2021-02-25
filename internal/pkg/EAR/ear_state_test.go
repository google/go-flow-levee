package EAR

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"sort"
	"testing"
)

// ParseSourceCode parses the source code, convert it to SSA form, and return the SSA package.
// The filename for source is always "test.go", and the package is "t" at an empty path.
func ParseSourceCode(src string) (*ssa.Package, error) {
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

func TestBasic(t *testing.T) {
	code := `package p;
	type T struct {}
	var g1 *int
	var g2 *int
	var g3 T
	`
	pkg, err := ParseSourceCode(code)
	if err != nil {
		t.Fatalf("compilation failed: %s", code)
	}
	state := NewAbsState()
	g1 := pkg.Members["g1"].(*ssa.Global)
	ref1 := getAbsReferenceForGlobal(g1)
	g2 := pkg.Members["g2"].(*ssa.Global)
	ref2 := getAbsReferenceForGlobal(g2)
	g3 := pkg.Members["g3"].(*ssa.Global)
	ref3 := getAbsReferenceForGlobal(g3)
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

	reps := make(AbsReferenceSet)
	state.GetAllPartitionReps(reps)
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
	mstrs := []string{ members[0].String(), members[1].String() }
	sort.Strings(mstrs)
	if mstrs[0] != "t.g1" || mstrs[1] != "t.g2" {
		t.Errorf("the member set of g1's partition should be [t.g1, t.g2]")
	}
}

func TestGlobalField(t *testing.T) {
	code := `package p;
	type T struct { x *int; y *int }
	var g1 *T
	var g2 *T
	var g3 *T
	var g4 *T
	`
	pkg, err := ParseSourceCode(code)
	if err != nil {
		t.Fatalf("compilation failed: %s", code)
	}
	state := NewAbsState()
	g1 := pkg.Members["g1"].(*ssa.Global)
	ref1 := getAbsReferenceForGlobal(g1)
	g2 := pkg.Members["g2"].(*ssa.Global)
	ref2 := getAbsReferenceForGlobal(g2)
	g3 := pkg.Members["g3"].(*ssa.Global)
	ref3 := getAbsReferenceForGlobal(g3)
	g4 := pkg.Members["g4"].(*ssa.Global)
	ref4 := getAbsReferenceForGlobal(g4)
	state.Insert(ref1)
	state.Insert(ref2)
	state.Insert(ref3)
	state.Insert(ref4)

	fm1 := state.GetPartitionFieldMap(ref1)
	fm1[Field{name: "x"}] = ref3
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
	fm3[Field{name: "y"}] = ref1
	fm4 := state.GetPartitionFieldMap(ref4)
	fm4[Field{name: "y"}] = ref2
	state.Unify(ref3, ref4)
	got = state.String()
	want = "{t.g1,t.g2}: [x->t.g3], {t.g3,t.g4}: [y->t.g2]"
	if got != want {
		t.Errorf("after unifying %s and %s:\n got: %s\n want: %s", ref3, ref4, got, want)
	}
}

func TestLocalField(t *testing.T) {
	code := `package p;
	type T struct { x *int; y *int }
	func f(a *T, b *T) {
		var c T
		_ = c
	}
	`
	pkg, err := ParseSourceCode(code)
	if err != nil {
		t.Fatalf("compilation failed: %s", code)
	}
	state := NewAbsState()
	f := pkg.Members["f"].(*ssa.Function)
	refa := getAbsReferenceForLocal(&emptyContext, f.Params[0])
	state.Insert(refa)
	refb := getAbsReferenceForLocal(&emptyContext, f.Params[1])
	state.Insert(refb)
	refc := getAbsReferenceForLocal(&emptyContext, f.Locals[0])
	state.Insert(refc)

	got := state.String()
	want := "{a}: [], {b}: [], {t0}: []"
	if got != want {
		t.Errorf("initial state:\n got: %s\n want: %s", got, want)
	}

	fma := state.GetPartitionFieldMap(refa)
	fma[Field{name: "x"}] = refc
	fma[Field{name: "y"}] = refb
	fmc := state.GetPartitionFieldMap(refc)
	fmc[Field{name: "y"}] = refa
	got = state.String()
	want = "{a}: [x->t0, y->b], {b}: [], {t0}: [y->a]"
	if got != want {
		t.Errorf("after setting fields:\n got: %s\n want: %s", got, want)
	}

	// Unify parameter "a" and local "c".
	state.Unify(refa, refc)
	got = state.String()
	want = "{a,b,t0}: [x->t0, y->a]"
	if got != want {
		t.Errorf("after unifying %s and %s:\n got: %s\n want: %s", refa, refc, got, want)
	}
}