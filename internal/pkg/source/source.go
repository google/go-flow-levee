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

// Package source contains the logic related to the concept of the source which may be tainted.
package source

import (
	"fmt"
	"strings"

	"github.com/eapache/queue"
	"github.com/google/go-flow-levee/internal/pkg/sanitizer"
	"golang.org/x/tools/go/ssa"
)

type classifier interface {
	IsSanitizer(*ssa.Call) bool
	IsPropagator(*ssa.Call) bool
	IsSourceFieldAddr(*ssa.FieldAddr) bool
}

// Source represents a Source in an SSA call tree.
// It is based on ssa.Node, with the added functionality of computing the recursive graph of
// its referrers.
// Source.sanitized notes sanitizer calls that sanitize this Source
type Source struct {
	node       ssa.Node
	marked     map[ssa.Node]bool
	sanitizers []*sanitizer.Sanitizer
	config     classifier
}

// Node returns the underlying ssa.Node of the Source.
func (a *Source) Node() ssa.Node {
	return a.node
}

// New constructs a Source
func New(in ssa.Node, config classifier) *Source {
	a := &Source{
		node:   in,
		marked: make(map[ssa.Node]bool),
		config: config,
	}
	a.bfs()
	return a
}

// bfs performs Breadth-First-Search on the def-use graph of the input Source.
// While traversing the graph we also look for potential sanitizers of this Source.
// If the Source passes through a sanitizer, bfs does not continue through that Node.
func (a *Source) bfs() {
	q := queue.New()
	q.Add(a.node)
	a.marked[a.node] = true

	for q.Length() > 0 {
		e := q.Remove().(ssa.Node)

		if e.Referrers() == nil {
			continue
		}

		for _, r := range *e.Referrers() {
			if _, ok := a.marked[r.(ssa.Node)]; ok {
				continue
			}

			if c, ok := r.(*ssa.Call); ok && a.config.IsSanitizer(c) {
				a.sanitizers = append(a.sanitizers, &sanitizer.Sanitizer{Call: c})
				continue
			}

			// Need to stay within the scope of the function under analysis.
			if call, ok := r.(*ssa.Call); ok && !a.config.IsPropagator(call) {
				continue
			}

			// Do not follow innocuous field access.
			if addr, ok := r.(*ssa.FieldAddr); ok && !a.config.IsSourceFieldAddr(addr) {
				continue
			}

			a.marked[r.(ssa.Node)] = true
			q.Add(r)
		}
	}
}

// HasPathTo returns true when a Node is part of declaration-use graph.
func (a *Source) HasPathTo(n ssa.Node) bool {
	return a.marked[n]
}

// IsSanitizedAt returns true when the Source is sanitized by the supplied instruction.
func (a *Source) IsSanitizedAt(call ssa.Instruction) bool {
	for _, s := range a.sanitizers {
		if s.Dominates(call) {
			return true
		}
	}

	return false
}

// String implements Stringer interface.
func (a *Source) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%v\n", a.node.String())

	fmt.Fprint(&b, "Connections:\n")
	for k := range a.marked {
		fmt.Fprintf(&b, "\t%v : %T\n", k.String(), k)
	}

	fmt.Fprint(&b, "Referrers:\n")
	for _, k := range *a.node.Referrers() {
		fmt.Fprintf(&b, "\t%v : %T\n", k.String(), k)
	}

	fmt.Fprint(&b, "Sanitizers:\n")
	for _, k := range a.sanitizers {
		fmt.Fprintf(&b, "\t%v : %T\n", k.Call, k)
	}

	return b.String()
}
