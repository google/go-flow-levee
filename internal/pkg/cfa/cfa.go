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

// Package cfa implements cross-function analysis. The analyzer
// defined in this package looks at every function in the transitive
// dependencies of the program being analyzed and creates an abstraction
// for each one that can be used to determine what the function does with
// each of its arguments.

package cfa

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// A Function is an abstraction for a Go function.
// It can be queried about what it does with its arguments.
type Function interface {
	// Does this function's nth argument reach a sink?
	Sinks(arg int) bool

	// If this argument is tainted, which return values are tainted?
	Taints(arg int) []int
}

type sink struct{}

func (s sink) Sinks(_ int) bool {
	return true
}

func (s sink) Taints(_ int) (tainted []int) {
	return
}

func (s sink) String() string {
	return "sink"
}

type sanitizer struct{}

func (s sanitizer) Sinks(_ int) bool {
	return false
}

func (s sanitizer) Taints(_ int) (tainted []int) {
	return
}

func (s sanitizer) String() string {
	return "sanitizer"
}

type genericFunc struct {
	sinks   []bool
	taints  [][]int
	results int
}

func newGenericFunc(f *ssa.Function) genericFunc {
	params := f.Signature.Params().Len()
	return genericFunc{
		sinks:  make([]bool, params),
		taints: make([][]int, params),
	}
}

func (g genericFunc) Sinks(arg int) bool {
	return g.sinks[arg]
}

func (g genericFunc) Taints(arg int) (tainted []int) {
	return g.taints[arg]
}

func (g genericFunc) String() string {
	var b strings.Builder
	b.WriteString("genericFunc{ sinks: [")
	var reached []string
	for i, reachesSink := range g.sinks {
		if reachesSink {
			reached = append(reached, strconv.Itoa(i))
		}
	}
	b.WriteString(strings.Join(reached, " "))
	b.WriteString("], taints: [")
	for i, s := range g.taints {
		sort.Ints(s)
		b.WriteString(fmt.Sprintf("%v", s))
		if i < len(g.sinks)-1 {
			b.WriteByte(' ')
		}
	}
	b.WriteString("] }")
	return b.String()
}

// Results returns the number of return values that this function has.
func (g genericFunc) Results() int {
	return g.results
}
