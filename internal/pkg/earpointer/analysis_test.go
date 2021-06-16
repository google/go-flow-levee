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
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-flow-levee/internal/pkg/earpointer"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

// Compiles the code and then runs the EAR pointer analysis with a specific context K.
func runCodeWithContext(code string, contextK int) (*earpointer.Partitions, error) {
	pkg, err := buildSSA(code)
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %s :\n %s", err, code)
	}
	// Synthesize an SSA input and an analysis pass.
	var srcFuncs []*ssa.Function
	for _, m := range pkg.Members {
		if f, ok := m.(*ssa.Function); ok {
			srcFuncs = append(srcFuncs, f)
		}
	}
	ssainput := buildssa.SSA{Pkg: pkg, SrcFuncs: srcFuncs}
	pass := analysis.Pass{ResultOf: map[*analysis.Analyzer]interface{}{buildssa.Analyzer: &ssainput}}
	earpointer.Analyzer.Flags.Set("contextK", strconv.Itoa(contextK))
	// Run the analysis.
	partitions, err := earpointer.Analyzer.Run(&pass)
	if err != nil {
		return nil, fmt.Errorf("analyzer run failed: %v", ssainput)
	}
	return partitions.(*earpointer.Partitions), nil
}

// A shortcut with context K=0.
func runCodeK0(code string) (*earpointer.Partitions, error) {
	return runCodeWithContext(code, 0)
}

// Concatenates the "members -> fields" map items into a string.
func concat(m map[string]string) string {
	var pstrs []string
	for k, v := range m {
		pstrs = append(pstrs, k+": "+v)
	}
	sort.Strings(pstrs)
	return strings.Join(pstrs, ", ")
}

func TestFieldAddr(t *testing.T) {
	code := `package p
	type T struct { x *int; y *int }
	func f(a *T, b *int) {
		a.x = b
	}
	`
	/*
		func f(a *T, b *int):
		0:                               entry P:0 S:0
		t0 = &a.x [#0]                   **int
		*t0 = b
		return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := concat(map[string]string{
		"{f.a}":  "--> *f.a",
		"{*f.a}": "[x->f.t0]",
		"{f.t0}": "--> f.b",
		"{f.b}":  "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestFieldAddr2(t *testing.T) {
	code := `package p
	type T struct { x *int; y *int }
	func f(a *T, b, c *int) {
		a.x = b
		a.x = c
	}
	`
	/*
		func f(a *T, b *int, c *int):
		0:                                                entry P:0 S:0
			t0 = &a.x [#0]                                **int
			*t0 = b
			t1 = &a.x [#0]                                **int
			*t1 = c
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := concat(map[string]string{
		"{f.a}":       "--> *f.a",
		"{*f.a}":      "[x->f.t1]",
		"{f.t0,f.t1}": "--> f.c",
		"{f.b,f.c}":   "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestField(t *testing.T) {
	code := `package p
	type T struct { x *int }
	func f(a *int) {
		_ = T{x: a}.x
	}
	`
	/*
		func f(a *int):
		0:                                  entry P:0 S:0
			t0 = local T (complit)          *T
			t1 = &t0.x [#0]                 **int
			*t1 = a
			t2 = *t0                        T
			t3 = t2.x [#0]                  *int
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	// t2's field x points to t3
	want := concat(map[string]string{
		"{f.t0}":       "--> f.t2",
		"{*f.t0,f.t2}": "[x->f.t1]",
		"{f.t1}":       "--> f.t3",
		"{f.a,f.t3}":   "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestEmbeddedField(t *testing.T) {
	code := `package p
	type T1 struct { }
	type T2 struct { t T1 }
	func f(i *int) {
		_ = T2{t: T1{}}.t
	}
	`
	/*
		func f(i *int):
		0:                                  entry P:0 S:0
			t0 = local T2 (complit)         *T2
			t1 = &t0.t [#0]                 *T1
			t2 = *t0                        T2
			t3 = t2.t [#0]                  T1
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	// t3 is not unified with t2.t since they are structs.
	want := concat(map[string]string{
		"{f.i}":        "[]",
		"{*f.t0,f.t2}": "[t->f.t1]",
		"{f.t0}":       "--> f.t2",
		"{*f.t1,f.t3}": "[]",
		"{f.t1}":       "--> f.t3",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestStructCopy(t *testing.T) { // mainly for coverage test
	code := `package p
	type T1 struct { }
	type T2 struct { x *T1; y *int }
	func f(v1, v2 T2, i *T1, j *int) {
		v1.x = i
		v1.y = j
		v2.x = i
		v2 = v1
	}
	`
	/*
		func f(v1 T2, v2 T2, i *T1, j *int):
		0:                                   entry P:0 S:0
			t0 = local T2 (v1)               *T2
			*t0 = v1
			t1 = local T2 (v2)               *T2
			*t1 = v2
			t2 = &t0.x [#0]                  **T1
			*t2 = i
			t3 = &t0.y [#1]                  **int
			*t3 = j
			t4 = &t1.x [#0]                  **T1
			*t4 = i
			t5 = *t0                         T2
			*t1 = t5
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := concat(map[string]string{
		"{f.i}":            "[]",
		"{f.j}":            "[]",
		"{f.t0}":           "--> f.t5",
		"{f.t1}":           "--> f.t5",
		"{f.t2,f.t4}":      "--> f.i",
		"{f.t3}":           "--> f.j",
		"{f.t5,f.v1,f.v2}": "[x->f.t4, y->f.t3]",
	})
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s\n", got, want)
	}
}

func TestEmbeddedFieldClone(t *testing.T) {
	code := `package p
	type T1 struct { x *int}
	type T2 struct { x T1 }
	func f(i T1, v1, v2 T2) {
		v2.x = i
		v1 = v2
	}
	`
	/*
		func f(i T1, v1 T2, v2 T2):
		0:                                                   entry P:0 S:0
			t0 = local T1 (i)                                *T1
			*t0 = i
			t1 = local T2 (v1)                               *T2
			*t1 = v1
			t2 = local T2 (v2)                               *T2
			*t2 = v2
			t3 = &t2.x [#0]                                  *T1
			t4 = *t0                                         T1
			*t3 = t4
			t5 = *t2                                         T2
			*t1 = t5
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := concat(map[string]string{
		"{f.i,f.t4}":       "[]",
		"{f.t0}":           "--> f.t4",
		"{f.t1}":           "--> f.t5",
		"{f.t2}":           "--> f.t5",
		"{f.t3}":           "--> f.t4",
		"{f.t5,f.v1,f.v2}": "[x->f.t3]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestIndexAddr(t *testing.T) {
	code := `package p
	func f(a, b []*int, i int) {
		a[1] = b[i]
	}
	`
	/*
		func f(a []*int, b []*int, i int):
		0:                       entry P:0 S:0
			t0 = &a[1:int]       **int
			t1 = &b[i]           **int
			t2 = *t1             *int
			*t0 = t2
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := concat(map[string]string{
		"{f.a}":  "--> *f.a",
		"{*f.a}": "[1->f.t0, AnyField->f.t0]",
		"{f.b}":  "--> *f.b",
		"{*f.b}": "[AnyField->f.t1]",
		"{f.t0}": "--> f.t2",
		"{f.t1}": "--> f.t2",
		"{f.t2}": "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestIndex(t *testing.T) {
	code := `package p
	func f(a *int) {
		_ = [10]*int{a}[0]
	}
	`
	/*
		func f(a *int):
		0:                                    entry P:0 S:0
			t0 = local [10]*int (complit)     *[10]*int
			t1 = &t0[0:int]                   **int
			*t1 = a
			t2 = *t0                          [10]*int
			t3 = t2[0:int]                    *int
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	// t2's field 0 points to t3
	want := concat(map[string]string{
		"{f.a}":        "[]",
		"{f.t0}":       "--> f.t2",
		"{f.t1,f.t3}":  "--> f.a",
		"{*f.t0,f.t2}": "[0->f.t3, AnyField->f.t3]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestPhi(t *testing.T) {
	code := `package p
	func f(a, b *int, i int) {
		c := a
		if i > 0 {
		    c = b
	    }
		print(c)

		d := 10  // non-pointer type
		if i > 0 {
		    d = i
	    }
		print(d)
	}
	`
	/*
		func f(a *int, b *int, i int):
		0:                                   entry P:0 S:2
			t0 = i > 0:int                   bool
			if t0 goto 1 else 2
		1:                                   if.then P:1 S:1
			jump 2
		2:                                   if.done P:2 S:0
			t1 = phi [0: a, 1: b] #c         *int
			t2 = print(t1)                   ()
		...
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.a,f.b,f.t1}: []"
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestStore(t *testing.T) {
	code := `package p
	type A struct { x *int; y *int }
	var z *int
	func f(a *A, b []*int) {
		a.x = z
		a.y = b[10]
	}
	`
	/*
		func f(a *A, b []*int):
		0:                            entry P:0 S:0
			t0 = &a.x [#0]            **int
			t1 = *z                   *int
			*t0 = t1
			t2 = &a.y [#1]            **int
			t3 = &b[10:int]           **int
			t4 = *t3                  *int
			*t2 = t4
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := concat(map[string]string{
		"{f.a}":  "--> *f.a",
		"{*f.a}": "[x->f.t0, y->f.t2]",
		"{f.b}":  "--> *f.b",
		"{*f.b}": "[10->f.t3, AnyField->f.t3]",
		"{f.t0}": "--> f.t1",
		"{f.t1}": "[]",
		"{f.t2}": "--> f.t4",
		"{f.t3}": "--> f.t4",
		"{f.t4}": "[]",
		"{z}":    "--> f.t1",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestDereference(t *testing.T) {
	code := `package p
	var x *int
	func f(y, z **int) {
		x = *y
		*z = x
	}
	`
	/*
		func f(y **int, z **int):
		0:                                  entry P:0 S:0
			t0 = *y                         *int
			*x = t0
			t1 = *z                         *int
			*x = t1
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := concat(map[string]string{
		"{f.t0,f.t1}": "[]",
		"{f.y}":       "--> f.t1",
		"{f.z}":       "--> f.t1",
		"{x}":         "--> f.t1",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestMapAccess(t *testing.T) {
	code := `package p
	type T struct { }
	var t *T
	func f(m map[int]*T, i int) {
		m[0] = t
		m[i] = m[1]
		_ = m[i]
	}
	`
	/*
		func f(m map[int]*T, i int):
		0:                        entry P:0 S:0
			t0 = *t               *T
			m[0:int] = t0
			t1 = m[1:int]         *T
			m[i] = t1
			t2 = m[i]             *T
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := concat(map[string]string{
		"{f.m}":            "[0->f.t2, 1->f.t2, AnyField->f.t2]",
		"{f.t0,f.t1,f.t2}": "[]",
		"{t}":              "--> f.t2",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestLookUp(t *testing.T) {
	code := `package p
	func f(a map[int]**int) {
		v, ok := a[10]
		_ = *v
		_ = ok
	}
	`
	/*
		func f(a map[int]**int):
		0:                                      entry P:0 S:0
			t0 = a[10:int],ok                   (**int, bool)
			t1 = extract t0 #0                  **int
			t2 = extract t0 #1                  bool
			t3 = *t1                            *int
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := concat(map[string]string{
		"{f.a}":  "[10->f.t1, AnyField->f.t1]",
		"{f.t1}": "--> f.t3",
		"{f.t3}": "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestConvert(t *testing.T) {
	code := `package p
	func f(a []byte) {
		_ = (string)(a)
	}
	`
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.a,f.t0}: []"
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestTypeAssert(t *testing.T) {
	code := `package p
	func f(a interface{}) {
	b,_ := a.(*bool)
		_ = b
		_ = a.(*int)
	}
	`
	/*
		func f(a interface{}):
		0:                                    entry P:0 S:0
			t0 = typeassert,ok a.(*bool)      (value *bool, ok bool)
			t1 = extract t0 #0                *bool
			t2 = extract t0 #1                bool
			t3 = typeassert a.(*int)          *int
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.a,f.t1,f.t3}: []"
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestChangeInterfaceOrType(t *testing.T) {
	code := `package p
	type I interface{ f() }
	type T1 struct {}
	type T2 struct {}
	func f(a I) interface{} {
		var t1 T1
		_ = (T2)(t1)
		return a
	}
	`
	/*
		func f(a I) interface{}:
		0:                                            entry P:0 S:0
			t0 = local T1 (t1)                        *T1
			t1 = *t0                                  T1
			t2 = changetype T2 <- T1 (t1)             T2
			t3 = changetype interface{} <- I (a)      interface{}
			return t3
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := concat(map[string]string{
		"{f.a,f.t3}":  "[]",
		"{f.t0}":      "--> f.t1",
		"{f.t1,f.t2}": "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestBinOp(t *testing.T) {
	code := `package p
	func f(a, b string) {
		_ = a + b
	}
	`
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.a}: [], {f.b}: [], {f.t0}: [left->f.a, right->f.b]"
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestRange(t *testing.T) {
	code := `package p
	func f(a map[string]*int) {
		for i, v := range a {
			_ = i
			_ = v
		}
	}
	`
	/*
		func f(a map[string]*int):
		0:                                   entry P:0 S:1
			t0 = range a                     iter
			jump 1
		1:                                   rangeiter.loop P:2 S:2
			t1 = next t0                     (ok bool, k string, v *int)
			t2 = extract t1 #0               bool
			if t2 goto 2 else 3
		2:                                   rangeiter.body P:1 S:1
			t3 = extract t1 #1               string
			t4 = extract t1 #2               *int
			jump 1
		3:                                   rangeiter.done P:1 S:0
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.a}: [AnyField->f.t4], {f.t3}: [], {f.t4}: []"
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestChannel(t *testing.T) {
	code := `package p
	func f(ch chan string, z string) {
		ch <- z   // Send v to channel ch.
		_ = <-ch  // Receive from ch.
	}
	`
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.ch}: [AnyField->f.t0], {f.t0,f.z}: []"
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestSelect(t *testing.T) {
	code := `package p
	func f(c1, c2 chan string, m1, m2 string) {
		select {
        case c1 <- m1:
			//
        case m := <- c2:
			print(m)
        case c1 <- m2:
			//
        }
	}
	`
	/*
		func f(c1 chan string, c2 chan string, m1 string, m2 string):
		0:                                                                entry P:0 S:2
			t0 = select blocking [c1<-m1, <-c2, c1<-m2] (index int, ok bool, string, string)
			t1 = extract t0 #0                                                  int
			t2 = t1 == 0:int                                                   bool
			if t2 goto 1 else 2
		1:                                               select.done P:4 S:0
			return
		2:                                               select.next P:1 S:2
			t3 = t1 == 1:int                                            bool
			if t3 goto 3 else 4
		3:                                               select.body P:1 S:1
			t4 = extract t0 #2                                        string
			t5 = print(t4)                                                ()
			jump 1
		4:                                               select.next P:1 S:2
			t6 = t1 == 2:int                                            bool
			if t6 goto 1 else 5
		5:                                               select.next P:1 S:2
			t7 = make interface{} <- string ("blocking select m...":string) interface{}
			panic t7
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := concat(map[string]string{
		"{f.c1}":      "[AnyField->f.m2]",
		"{f.c2}":      "[AnyField->f.t4]",
		"{f.m1,f.m2}": "[]",
		"{f.t7}":      "[]",
		"{f.t4}":      "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestSlice(t *testing.T) {
	code := `package p
	func f(s [6]int) {
		t1 := s[1:4]
		_ = t1[2:]
	}
	`
	/*
		func f(s [6]int):
		0:                                 entry P:0 S:0
			t0 = new [6]int (s)            *[6]int
			*t0 = s
			t1 = slice t0[1:int:4:int]     []int
			t2 = slice t1[2:int:]          []int
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.s}: [], {f.t0,f.t1,f.t2}: --> f.s"
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestMakeInterface(t *testing.T) {
	code := `package p
	func f(s *int) interface{} {
		return s
	}
	`
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.s,f.t0}: []"
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestUnimplemented(t *testing.T) {
	code := `package p
	func f() {
		c1 := make(chan string)
		c2 := make(chan string)
		select {
	        case <-c1: { print("c1") }
	        case <-c2: { print("c2") }
		}

		panic("test")
	}
	`
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.t0}: [], {f.t1}: [], {f.t5}: [], {f.t9}: []"
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestSimpleCall(t *testing.T) {
	code := `package p
	func f(x *int, y *int) *int {
		return g(x, nil)
	}
	func g(a *int, b *int) *int {
		return a
	}
	`
	/*
		func f(x *int, y *int) *int:
		0:                                             entry P:0 S:0
			t0 = g(x, nil:*int)                        *int
			return t0

		func g(a *int, b *int) *int:
		0:                                             entry P:0 S:0
			return a
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	// f.x and g.a are unified due to argument passing;
	// f.t0 and g.a are unified due to g's return.
	want := "{f.t0,f.x,g.a}: [], {f.y}: [], {g.b}: []"
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestCallWithTupleReturn(t *testing.T) {
	code := `package p
	type T struct { }
	var x T
	func f(a *T) *T {
		t0,_ := g(a)
		return t0
	}
	func g(a *T) (*T,T) {
		return a, x
	}
	`
	/*
		func f(a *T) *T:
		0:                                               entry P:0 S:0
			t0 = g(a)                                    (*T, T)
			t1 = extract t0 #0                           *T
			t2 = extract t0 #1                           T
			return t1

		func g(a *T) (*T, T):
		0:                                               entry P:0 S:0
			t0 = *x                                      T
			return a, t0
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	// f.a and g.a are unified due to the call;
	// f.t1 and g.a are unified due to the return of this call.
	// Here (f.t1, f.t2) == f.t0 == (g.a, g.t0).
	// Note that g.t0 and f.t2 are unified due to an approximation.
	want := concat(map[string]string{
		"{f.t0}":         "[0->g.a, 1->g.t0]",
		"{f.a,f.t1,g.a}": "[]",
		"{f.t2,g.t0}":    "[]",
		"{x}":            "--> g.t0",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestClosureCall(t *testing.T) {
	code := `package p
	func f(i, j int) {
		g := func() (*int,*int, int) {
			return &i, &j, 10
		}
		_,_,_ = g()
	}
	`
	/*
		func f(i int, j int):
		0:                                               entry P:0 S:0
			t0 = new int (i)                             *int
			*t0 = i
			t1 = new int (j)                             *int
			*t1 = j
			t2 = make closure f$1 [t0, t1]               func() (*int, *int)
			t3 = t2()                                    (*int, *int)
			t4 = extract t3 #0                           *int
			t5 = extract t3 #1                           *int
			t6 = extract t3 #2                           int
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	// Free variable f$1.i is unified with f.t0 due to argument passing,
	// and with f.t4 due to the return of the closure function.
	want := concat(map[string]string{
		"{f$1.i,f.t0,f.t4}": "[]",
		"{f$1.j,f.t1,f.t5}": "[]",
		"{f.t2}":            "[]",
		"{f.t3}":            "[0->f.t0, 1->f.t1]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestRecursiveCall(t *testing.T) {
	code := `package p
	func f(i int, y *int) *int {
		if i > 10 {
			y = nil
		}
		return f(i-1, y)
	}
	`
	/*
		func f(i int, y *int) *int:
		0:                                       entry P:0 S:2
			t0 = i > 10:int                      bool
			if t0 goto 1 else 2
		1:                                       if.then P:1 S:1
			jump 2
		2:                                       if.done P:2 S:0
			t1 = phi [0: y, 1: nil:*int] #y      *int
			t2 = i - 1:int                       int
			t3 = f(t2, t1)                       *int
			return t3
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.t1,f.y}: [], {f.t3}: []"
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestMethod(t *testing.T) {
	code := `package p
	type A struct {}
	func (a A) g(x *int) *int {
		return x
	}
	func f(x *int, a *A) *int {
		return a.g(x)
	}
	`
	/*
		func (a A) g(x *int) *int:
			0:                                          entry P:0 S:0
				t0 = local A (a)                        *A
				*t0 = a
				return x
		func (a *t.A) g(x *int) *int:
		0:                                              entry P:0 S:0
			t0 = ssa:wrapnilchk(a, "t.A":string, "g":string)
			t1 = *t0                                    t.A
			t2 = (t.A).g(t1, x)                         *int
			return t2

		func f(x *int, a *A) *int:
			0:                                          entry P:0 S:0
				t0 = *a                                 A
				t1 = (A).g(t0, x)                       *int
				return t1
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	// f.t1, f.x and g.x are unified due to calling g().
	// Note g.a is a struct receiver that won't be unified.
	want := concat(map[string]string{
		"{f.a}":                           "--> A:g.a",
		"{*A:g.t2,*A:g.x,A:g.x,f.t1,f.x}": "[]",
		"{*A:g.a}":                        "[]",
		"{A:g.t0}":                        "--> A:g.a",
		"{*A:g.t0}":                       "--> A:g.a",
		"{*A:g.t1,A:g.a,f.t0}":            "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestGoDefer(t *testing.T) {
	code := `package p
	func g(i *int) {
		go func() {
			print(*i)
		}()
		defer func(k *int) {
			print(*k)
		}(i)
	}
	`
	/*
		func g(i *int):
		0:                                       entry P:0 S:0
			t0 = new *int (i)                    **int
			*t0 = i
			t1 = make closure g$1 [t0]           func()
			go t1()
			t2 = *t0                             *int
			defer g$2(t2)
			rundefers
			return
		1:                                       recover P:0 S:0
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	// Free variable g$1.i is unified with g.t0 due to closure binding;
	// Arguments g$2.k and g.i are unified due to the defer call.
	want := concat(map[string]string{
		"{g$1.i,g.t0}":            "--> g.t2",
		"{g$1.t0,g$2.k,g.i,g.t2}": "[]",
		"{g.t1}":                  "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestCallBuiltin(t *testing.T) {
	code := `package p
	func f(s []*int, x *int, d1 []*int) {
		d2 := append(s, x)
		copy(d1, s)
		print(d1, d2)
	}
	`
	/*
		func f(s []*int, x *int, d1 []*int):
		0:                                             entry P:0 S:0
			t0 = new [1]*int (varargs)                 *[1]*int
			t1 = &t0[0:int]                            **int
			*t1 = x
			t2 = slice t0[:]                           []*int
			t3 = append(s, t2...)                      []*int
			t4 = copy(d1, s)                           int
			t5 = print(d1, t3)                         ()
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	// f.s and f.d1 are separate due to the "copy".
	// f.d1 contains a field pointing at f.x due to the "copy".
	// f.s, f.t2 and f.r3 ("d2") are unified due to the "append".
	want := concat(map[string]string{
		"{f.d1}":               "--> *f.t0",
		"{*f.t0,f.d1[.]}":      "[0->f.t1, AnyField->f.t1]",
		"{f.s,f.t0,f.t2,f.t3}": "--> *f.t0",
		"{f.t1}":               "--> f.x",
		"{f.x}":                "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestKnownCall(t *testing.T) {
	code := `package p
	import (
		"fmt"
		"os"
	)
	func f(x string) {
		_ = fmt.Sprintf(x)
		fmt.Fprintf(os.Stderr, x)
	}
	`
	/*
		func f(x string):
		0:                                                   entry P:0 S:0
			t0 = fmt.Sprintf(x, nil:[]interface{}...)        string
			t1 = *os.Stderr                                  *os.File
			t2 = make io.Writer <- *os.File (t1)             io.Writer
			t3 = fmt.Fprintf(t2, x, nil:[]interface{}...)   (n int, err error)
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	// f.x is pointed by a field of f.t0,
	// and by a field of f.t1 (i.e. os.Stderr).
	want := concat(map[string]string{
		"{Stderr}":    "--> f.t1",
		"{f.t0}":      "[1->f.x]",
		"{f.t1,f.t2}": "[2->f.x]",
		"{f.t3}":      "[]",
		"{f.x}":       "[]",
	})
	// Exclude the references in packages "fmt" and "os".
	if !strings.Contains(state.String(), want) {
		t.Errorf("want:\n%s", want)
	}
}

func TestVariadicCall(t *testing.T) {
	code := `package p
	func g(ks ...*int) *int {
		return ks[0]
	}
	func f(a *int, b *int) *int {
		g(a)
		return g(a, b)
	}
	`
	/*
		func f(a *int, b *int) *int:
		0:                                                     entry P:0 S:0
			t0 = new [1]*int (varargs)                              *[1]*int
			t1 = &t0[0:int]                                                **int
			*t1 = a
			t2 = slice t0[:]                                              []*int
			t3 = g(t2...)                                                   *int
			t4 = new [2]*int (varargs)                                  *[2]*int
			t5 = &t4[0:int]                                                **int
			*t5 = a
			t6 = &t4[1:int]                                                **int
			*t6 = b
			t7 = slice t4[:]                                              []*int
			t8 = g(t7...)                                                   *int
			return t8

			func g(ks ...*int) *int:
			0:                                                     entry P:0 S:0
				t0 = &ks[0:int]                                    **int
				t1 = *t0                                           *int
				return t1
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	// Handle the non-determinism due to the processing order of "f" and "g".
	// Synthetic reference *g.ks may or may not be created: when "f" is
	// processed first, "*g.ks" will not be created.
	want1 := concat(map[string]string{
		"{f.a,f.b,f.t3,f.t8,g.t1}": "[]",
		// non-determinism: synthetic *g.ks is not created.
		"{*f.t0,*f.t4}":              "[0->g.t0, 1->g.t0, AnyField->g.t0]",
		"{f.t0,f.t2,f.t4,f.t7,g.ks}": "--> *f.t4", // non-determinism
		"{f.t1,f.t5,f.t6,g.t0}":      "--> f.t8",
	})
	want2 := concat(map[string]string{
		"{f.a,f.b,f.t3,f.t8,g.t1}": "[]",
		// non-determinism: synthetic *g.ks is created.
		"{*f.t0,*f.t4,*g.ks}":        "[0->g.t0, 1->g.t0, AnyField->g.t0]",
		"{f.t0,f.t2,f.t4,f.t7,g.ks}": "--> *f.t4",
		"{f.t1,f.t5,f.t6,g.t0}":      "--> f.t8",
	})
	diff1 := cmp.Diff(want1, state.String())
	diff2 := cmp.Diff(want2, state.String())
	if diff1 != "" && diff2 != "" {
		t.Errorf("diff (-want +got):\n%s", diff1+diff2)
	}
}

func TestMethodInvoke(t *testing.T) {
	code := `package p
	type T1 struct {}
	func (*T1) f(x *int) {}

	type T2 struct {
		T1
	}
	func g(x2 *T2, x *int) {
		x2.f(x)
	}
	`
	/*
		func (arg0 (T1) f(x *int):
		0:                                          entry P:0 S:0
				return

		func (arg0 *t.T2) f(x *int):
		0:                                          entry P:0 S:0
			t0 = &arg0.T1 [#0]                      *t.T1
			t1 = (*t.T1).f(t0, x)                   ()
			return

		func g(x2 *T2, x *int):
		0:                                      	entry P:0 S:0
			t0 = &x2.T1 [#0]                        *T1
			t1 = (*T1).f(t0, x)                     ()
			return
	*/
	state, err := runCodeK0(code)
	if err != nil {
		t.Fatal(err)
	}
	// *T1.f.x and g.x are unified due to calling f().
	// There are two separate f.arg0s w.r.t. T1.f and T2.f respectively.
	// Note that in "**T2", the first "*" is the synthesized ValueOf operator,
	// and "*T2" is the receiver type.
	want := concat(map[string]string{
		"{**T2:f.arg0}":              "[T1->*T1:f.arg0]",
		"{*T1:f.arg0,*T2:f.t0,g.t0}": "[]",
		"{*T1:f.x,*T2:f.x,g.x}":      "[]",
		"{*T2:f.arg0}":               "--> **T2:f.arg0",
		"{*g.x2}":                    "[T1->*T1:f.arg0]",
		"{g.x2}":                     "--> *g.x2",
	})
	if got := state.String(); got != want {
		t.Errorf("\n  got: %s\n want: %s", got, want)
	}
}

func TestMethodCallContextSensitive(t *testing.T) {
	code := `package p
	type A struct {}
	func f(x *int) *int {
		var a A
		return a.g(x)
	}
	func (a *A) g(x *int) *int {
		return x
	}
	`
	/*
		func f(x *int) *int:
		0:                                               entry P:0 S:0
			t0 = new A (a)                               *A
			t1 = (*A).g(t0, x)                           *int
			return t1
	*/
	state, err := runCodeWithContext(code, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := concat(map[string]string{
		"{[(*A).g(t0, x)]*A:g.a,f.t0}":     "[]",
		"{[(*A).g(t0, x)]*A:g.x,f.t1,f.x}": "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestCallContextSensitive(t *testing.T) {
	code := `package p
	func f(x, y *int, i int) *int {
		if i > 0 {
			return g(x)
		}
		return g(y)
	}
	func g(a *int) *int {
		return a
	}
	`
	/*
		func f(x *int, y *int, i int) *int:
		0:                                           entry P:0 S:2
			t0 = i > 0:int                           bool
			if t0 goto 1 else 2
		1:                                           if.then P:1 S:0
			t1 = g(x)                                *int
			return t1
		2:                                           if.done P:1 S:0
			t2 = g(y)                                *int
			return t2
	*/
	state, err := runCodeWithContext(code, 1)
	if err != nil {
		t.Fatal(err)
	}
	// g() is called at two call sites. f.x and f.y won't be unified.
	want := concat(map[string]string{
		"{[g(x)]g.a,f.t1,f.x}": "[]",
		"{[g(y)]g.a,f.t2,f.y}": "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestEmbeddedCallContextSensitive(t *testing.T) {
	code := `package p
	func f(x, y *int) {
		g(x)
		g(y)
	}
	func g(a *int) {
		h(a)
	}
	func h(b *int) { }
	`
	/*
		func f(x *int, y *int):
		0:                                           entry P:0 S:0
			t0 = g(x)                                ()
			t1 = g(y)                                ()
			return

		func g(a *int):
		0:                                           entry P:0 S:0
			t0 = h(a)                                ()
			return
	*/
	state, err := runCodeWithContext(code, 1)
	if err != nil {
		t.Fatal(err)
	}
	// When K=1, h.b will be unified with g.a regardless of the context of g.a.
	want := concat(map[string]string{
		"{[g(x)]g.a,[g(y)]g.a,[h(a)]h.b,f.x,f.y}": "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}

	state, _ = runCodeWithContext(code, 2)
	// When K=2, g(x)'s calling h(a) is distinguished from g(y)'s calling h(a),
	// i.e. h(a) is called in two different contexts: [g(x); h(a)] and [g(y); h(a)].
	want = concat(map[string]string{
		"{[g(x); h(a)]h.b,[g(x)]g.a,f.x}": "[]",
		"{[g(y); h(a)]h.b,[g(y)]g.a,f.y}": "[]",
	})
	//	g() is called at two call sites.
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}

func TestCallReturnContextSensitive(t *testing.T) {
	code := `package p
	func f(x, y, z *int) *int {
		g(x)
		h(y)
		return g(z)
	}
	func g(a *int) *int {
		return h(a)
	}
	func h(b *int) *int {
		return b
	}
	`
	/*
		func f(x *int, y *int, z *int) *int:
		0:                                                                entry P:0 S:0
			t0 = g(x)                                                          *int
			t1 = h(y)                                                          *int
			t2 = g(z)                                                          *int
			return t2

		func g(a *int) *int:
		0:                                                                entry P:0 S:0
			t0 = h(a)                                                          *int
			return t0
	*/
	state, err := runCodeWithContext(code, 1)
	if err != nil {
		t.Fatal(err)
	}
	// When K=1, h.b will be unified with g.a regardless of the context of g.a,
	// which also resulting in the unfications of f.x, f.y and f.z.
	want := concat(map[string]string{
		"{[g(x)]g.a,[g(x)]g.t0,[g(z)]g.a,[g(z)]g.t0,[h(a)]h.b,f.t0,f.t2,f.x,f.z}": "[]",
		"{[h(y)]h.b,f.t1,f.y}": "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}

	state, _ = runCodeWithContext(code, 2)
	// When K=2, less unifications are performed. Now the two calls to h(b) are separate.
	want = concat(map[string]string{
		"{[g(x); h(a)]h.b,[g(x)]g.a,[g(x)]g.t0,f.t0,f.x}": "[]",
		"{[g(z); h(a)]h.b,[g(z)]g.a,[g(z)]g.t0,f.t2,f.z}": "[]",
		"{[h(y)]h.b,f.t1,f.y}":                            "[]",
	})
	if diff := cmp.Diff(want, state.String()); diff != "" {
		t.Errorf("diff (-want +got):\n%s", diff)
	}
}
