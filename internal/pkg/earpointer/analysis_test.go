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
	"testing"

	"github.com/google/go-flow-levee/internal/pkg/earpointer"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

// Compiles the code and then runs the EAR pointer analysis.
func runCode(code string) (*earpointer.Partitions, error) {
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
	// Run the analysis.
	analyzer := earpointer.Analyzer
	partitions, err := analyzer.Run(&pass)
	if err != nil {
		return nil, fmt.Errorf("analyzer run failed: %v", ssainput)
	}
	return partitions.(*earpointer.Partitions), nil
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
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{*f.a}: [x->f.t0], {f.a}: --> *f.a, {f.b}: [], {f.t0}: --> f.b"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
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
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	// t2's field x points to t3
	want := "{*f.t0}: [x->f.t1], {f.a}: [], {f.t0}: --> *f.t0, {f.t1}: --> f.a, {f.t2[.],f.t3}: --> f.a, {f.t2}: [x->f.t3]"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
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
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.a}: [1->f.t0], {f.b}: [AnyField->f.t1], {f.t0}: --> f.t2, {f.t1}: --> f.t2, {f.t2}: []"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
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
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	// t2's field 0 points to t3
	want := "{f.a}: [], {f.t0}: --> f.t2, {f.t1}: --> f.a, {f.t2}: [0->f.t3], {f.t3}: []"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
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
	*/
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.a,f.b,f.t1}: []"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
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
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want :=
		`{*f.a}: [x->f.t0, y->f.t2], {f.a}: --> *f.a, {f.b}: [10->f.t3], {f.t0}: --> f.t1, {f.t1}: [], {f.t2}: --> f.t4, {f.t3}: --> f.t4, {f.t4}: [], {t.z}: --> f.t1`
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
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
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.m}: [0->f.t0, 1->f.t2, AnyField->f.t2], {f.t0}: [], {f.t1,f.t2}: [], {t.t}: --> f.t0"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
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
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.a}: [10->f.t1], {f.t1}: --> f.t3, {f.t3}: []"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
	}
}

func TestConvert(t *testing.T) {
	code := `package p
	func f(a []byte) {
		_ = (string)(a)
	}
	`
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.a,f.t0}: []"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
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
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.a,f.t1,f.t3}: []"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
	}
}

func TestChangeInterfaceOrType(t *testing.T) {
	code := `package p
	type I interface{}
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
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.a,f.t3}: [], {f.t0}: --> f.t1, {f.t1,f.t2}: []"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
	}
}

func TestBinOp(t *testing.T) {
	code := `package p
	func f(a, b string) {
	_ = a + b
	}
	`
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.a,f.b,f.t0}: []"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
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
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.a}: [AnyField->f.t4], {f.t3}: [], {f.t4}: []"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
	}
}

func TestChannel(t *testing.T) {
	code := `package p
	func f(ch chan string, z string) {
	ch <- z   // Send v to channel ch.
	_ = <-ch  // Receive from ch.
	}
	`
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.ch}: [AnyField->f.t0], {f.t0,f.z}: []"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
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
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.s}: [], {f.t0,f.t1,f.t2}: --> f.s"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
	}
}

func TestMakeInterface(t *testing.T) {
	code := `package p
	func f(s *int) interface{} {
		return s
	}
	`
	state, err := runCode(code)
	if err != nil {
		t.Fatal(err)
	}
	want := "{f.s,f.t0}: []"
	if got := state.String(); got != want {
		t.Errorf("got: %s\n want: %s", got, want)
	}
}
