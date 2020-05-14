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
	"go/types"
	"strings"

	"github.com/eapache/queue"
	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/sanitizer"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis/passes/buildssa"
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

func Identify(conf *config.Config, ssaInput *buildssa.SSA) map[*ssa.Function][]*Source {
	sourceMap := make(map[*ssa.Function][]*Source)

	for _, fn := range ssaInput.SrcFuncs {
		var sources []*Source
		sources = append(sources, sourcesFromParams(fn, conf)...)
		sources = append(sources, sourcesFromClosure(fn, conf)...)
		sources = append(sources, sourcesFromBlocks(fn, conf)...)

		if len(sources) > 0 {
			sourceMap[fn] = sources
		}
	}
	return sourceMap
}

func sourcesFromParams(fn *ssa.Function, conf *config.Config) []*Source {
	var sources []*Source
	for _, p := range fn.Params {
		switch t := p.Type().(type) {
		case *types.Pointer:
			if n, ok := t.Elem().(*types.Named); ok && conf.IsSource(n) {
				sources = append(sources, New(p, conf))
			}
			// TODO Handle the case where sources arepassed by value: func(c sourceType)
			// TODO Handle cases where PII is wrapped in struct/slice/map
		}
	}
	return sources
}

func sourcesFromClosure(fn *ssa.Function, conf *config.Config) []*Source {
	var sources []*Source
	for _, p := range fn.FreeVars {
		switch t := p.Type().(type) {
		case *types.Pointer:
			// FreeVars (variables from a closure) appear as double-pointers
			// Hence, the need to dereference them recursively.
			if s, ok := utils.Dereference(t).(*types.Named); ok && conf.IsSource(s) {
				sources = append(sources, New(p, conf))
			}
		}
	}
	return sources
}

func sourcesFromBlocks(fn *ssa.Function, conf *config.Config) []*Source {
	var sources []*Source
	for _, b := range fn.Blocks {
		if b == fn.Recover {
			// TODO Handle calls to log in a recovery block.
			continue
		}

		for _, instr := range b.Instrs {
			switch v := instr.(type) {
			// Looking for sources of PII allocated within the body of a function.
			case *ssa.Alloc:
				if conf.IsSource(utils.Dereference(v.Type())) {
					sources = append(sources, New(v, conf))
				}

				// Handling the case where PII may be in a receiver
				// (ex. func(b *something) { log.Info(something.PII) }
			case *ssa.FieldAddr:
				if conf.IsSource(utils.Dereference(v.Type())) {
					sources = append(sources, New(v, conf))
				}
			}
		}
	}
	return sources
}

