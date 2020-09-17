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
	"go/token"
	"go/types"
	"log"
	"strings"

	"github.com/google/go-flow-levee/internal/pkg/sanitizer"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

type classifier interface {
	IsSource(types.Type) bool
	IsSanitizer(*ssa.Call) bool
	IsSourceFieldAddr(*ssa.FieldAddr) bool
	IsSinkFunction(fn *ssa.Function) bool
}

// Source represents a Source in an SSA call tree.
// It is based on ssa.Node, with the added functionality of computing the recursive graph of
// its referrers.
// Source.sanitized notes sanitizer calls that sanitize this Source
type Source struct {
	node             ssa.Node
	marked           map[ssa.Node]bool
	preOrder         []ssa.Node
	sanitizers       []*sanitizer.Sanitizer
	config           classifier
	maxInstrReached  map[*ssa.BasicBlock]int
	lastBlockVisited *ssa.BasicBlock
}

// Pos returns the token position of the SSA Node associated with the Source.
func (s *Source) Pos() token.Pos {
	// Extracts don't have a registered position in the source code,
	// so we need to use the position of their related Tuple.
	if e, ok := s.node.(*ssa.Extract); ok {
		return e.Tuple.Pos()
	}
	return s.node.Pos()
}

// New constructs a Source
func New(in ssa.Node, config classifier) *Source {
	s := &Source{
		node:            in,
		marked:          make(map[ssa.Node]bool),
		config:          config,
		maxInstrReached: map[*ssa.BasicBlock]int{},
	}
	s.dfs(in)
	return s
}

// dfs performs Depth-First-Search on the def-use graph of the input Source.
// While traversing the graph we also look for potential sanitizers of this Source.
// If the Source passes through a sanitizer, dfs does not continue through that Node.
func (s *Source) dfs(n ssa.Node) {
	if s.marked[n] {
		return
	}

	if instr, ok := n.(ssa.Instruction); ok {
		if !s.reachableFromSource(instr) {
			return
		}
		s.record(instr)
	}
	s.preOrder = append(s.preOrder, n)
	s.marked[n.(ssa.Node)] = true

	s.visitReferrers(n)

	operands := n.Operands(nil)
	if operands != nil {
		s.visitOperands(n, operands)
	}
}

func (s *Source) record(target ssa.Instruction) {
	b := target.Block()
	s.lastBlockVisited = b
	i, ok := indexInBlock(target)
	if !ok {
		return
	}
	if s.maxInstrReached[b] < i {
		s.maxInstrReached[b] = i
	}
}

func (s *Source) visitReferrers(n ssa.Node) {
	referrers := s.referrersToVisit(n)

	for _, r := range referrers {
		switch v := r.(type) {
		case *ssa.Call:
			if s.config.IsSanitizer(v) {
				s.sanitizers = append(s.sanitizers, &sanitizer.Sanitizer{Call: v})
			}
		}

		s.dfs(r.(ssa.Node))
	}
}

// referrersToVisit produces a filtered list of Referrers for an ssa.Node.
// Specifically, we want to avoid referrers that shouldn't be visited, e.g.
// because they would not be reachable in an actual execution of the program.
func (s *Source) referrersToVisit(n ssa.Node) (referrers []ssa.Instruction) {
	if n.Referrers() == nil {
		return
	}
	for _, r := range *n.Referrers() {
		// If the referrer is in a different block from the one we last visited,
		// and it can't be reached from the block we are visiting, then stop visiting.
		if rb := r.Block(); s.lastBlockVisited != nil &&
			rb != s.lastBlockVisited &&
			!s.canReach(s.lastBlockVisited, rb) {
			continue
		}

		if c, ok := r.(*ssa.Call); ok {
			// This is to avoid attaching calls where the source is the receiver, ex:
			// core.Sinkf("Source id: %v", wrapper.Source.GetID())
			if recv := c.Call.Signature().Recv(); recv != nil && s.config.IsSource(utils.Dereference(recv.Type())) {
				continue
			}

			// If this call's index is lower than the highest in its block,
			// then this call is "in the past" and we should stop traversing.
			i, ok := indexInBlock(r)
			if !ok {
				continue
			}
			if i < s.maxInstrReached[r.Block()] {
				continue
			}
		}

		if fa, ok := r.(*ssa.FieldAddr); ok && !s.config.IsSourceFieldAddr(fa) {
			continue
		}

		referrers = append(referrers, r)
	}
	return referrers
}

func (s *Source) reachableFromSource(target ssa.Instruction) bool {
	// If the Source isn't produced by an instruction, be conservative and
	// assume the target instruction is reachable.
	sInstr, ok := s.node.(ssa.Instruction)
	if !ok {
		return true
	}

	// If these calls fail, be conservative and assume the target
	// instruction is reachable.
	sIndex, sOk := indexInBlock(sInstr)
	targetIndex, targetOk := indexInBlock(target)
	if !sOk || !targetOk {
		return true
	}

	if sInstr.Block() == target.Block() && sIndex > targetIndex {
		return false
	}

	if !s.canReach(sInstr.Block(), target.Block()) {
		return false
	}

	return true
}

func (s *Source) canReach(start *ssa.BasicBlock, dest *ssa.BasicBlock) bool {
	if start.Dominates(dest) {
		return true
	}

	stack := stack([]*ssa.BasicBlock{start})
	seen := map[*ssa.BasicBlock]bool{start: true}
	for len(stack) > 0 {
		current := stack.pop()
		if current == dest {
			return true
		}
		for _, s := range current.Succs {
			if seen[s] {
				continue
			}
			seen[s] = true
			stack.push(s)
		}
	}
	return false
}

func (s *Source) visitOperands(n ssa.Node, operands []*ssa.Value) {
	_, visitingFromExtract := n.(*ssa.Extract)

	for _, o := range operands {
		n, ok := (*o).(ssa.Node)
		if !ok {
			continue
		}

		// Do not visit a Call if the current node is an Extract.
		// This is to avoid incorrectly tainting non-Source values that are
		// returned from a call that has a Source among its return values,
		// e.g. a call to a function with a signature like:
		// func CreateSource() (core.Source, error)
		if _, ok := (*o).(*ssa.Call); visitingFromExtract && ok {
			continue
		}

		// An Alloc represents the allocation of space for a variable. If a Node is an Alloc,
		// and the thing being allocated is not an array, then either:
		// a) it is a Source value, in which case it will get its own traversal when sourcesFromBlocks
		//    finds this Alloc
		// b) it is not a Source value, in which case we should not visit it.
		// However, if the Alloc is an array, then that means the source that we are visiting from
		// is being placed into an array, slice or varags, so we do need to keep visiting.
		if al, isAlloc := (*o).(*ssa.Alloc); isAlloc {
			if _, isArray := utils.Dereference(al.Type()).(*types.Array); !isArray {
				return
			}
		}
		s.dfs(n)
	}
}

// compress removes the elements from the graph that are not required by the
// taint-propagation analysis. Concretely, only propagators, sanitizers and
// sinks should constitute the output. Since, we already know what the source
// is, it is also removed.
func (s *Source) compress() []ssa.Node {
	var compressed []ssa.Node
	for _, n := range s.preOrder {
		switch n.(type) {
		case *ssa.Call:
			compressed = append(compressed, n)
		}
	}

	return compressed
}

func (s *Source) RefersTo(n ssa.Node) bool {
	return s.HasPathTo(n)
}

// HasPathTo returns true when a Node is part of declaration-use graph.
func (s *Source) HasPathTo(n ssa.Node) bool {
	return s.marked[n]
}

// IsSanitizedAt returns true when the Source is sanitized by the supplied instruction.
func (s *Source) IsSanitizedAt(call ssa.Instruction) bool {
	for _, san := range s.sanitizers {
		if san.Dominates(call) {
			return true
		}
	}

	return false
}

// String implements Stringer interface.
func (s *Source) String() string {
	var b strings.Builder
	for _, n := range s.compress() {
		b.WriteString(fmt.Sprintf("%v ", n))
	}

	return b.String()
}

func identify(conf classifier, ssaInput *buildssa.SSA) map[*ssa.Function][]*Source {
	sourceMap := make(map[*ssa.Function][]*Source)

	for _, fn := range ssaInput.SrcFuncs {
		// no need to analyze the body of sinks
		if conf.IsSinkFunction(fn) {
			continue
		}

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

func sourcesFromParams(fn *ssa.Function, conf classifier) []*Source {
	var sources []*Source
	for _, p := range fn.Params {
		switch t := p.Type().(type) {
		case *types.Pointer:
			if n, ok := t.Elem().(*types.Named); ok && conf.IsSource(n) {
				sources = append(sources, New(p, conf))
			}
		}
	}
	return sources
}

func sourcesFromClosure(fn *ssa.Function, conf classifier) []*Source {
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

// sourcesFromBlocks finds Source values created by instructions within a function's body.
func sourcesFromBlocks(fn *ssa.Function, conf classifier) []*Source {
	var sources []*Source
	for _, b := range fn.Blocks {
		if b == fn.Recover {
			continue
		}

		for _, instr := range b.Instrs {
			// This type switch is used to catch instructions that could produce sources.
			// All instructions that do not match one of the cases will hit the "default"
			// and they will not be examined any further.
			switch v := instr.(type) {
			// drop anything that doesn't match one of the following cases
			default:
				continue

			// source defined as a local variable or returned from a call
			case *ssa.Alloc, *ssa.Call:
				// Allocs and Calls are values
				if isProducedBySanitizer(v.(ssa.Value), conf) {
					continue
				}

			// An Extract is used to obtain a value from an instruction that returns multiple values.
			// If the Extract is used to get a Pointer, create a Source, otherwise the Source won't
			// have an Alloc and we'll miss it.
			case *ssa.Extract:
				t := v.Tuple.Type().(*types.Tuple).At(v.Index).Type()
				if _, ok := t.(*types.Pointer); ok && conf.IsSource(utils.Dereference(t)) {
					sources = append(sources, New(v, conf))
				}
				continue

			// source received from chan
			case *ssa.UnOp:
				// not a <-chan operation
				if v.Op != token.ARROW {
					continue
				}

			// source obtained through a field or an index operation
			case *ssa.FieldAddr, *ssa.IndexAddr, *ssa.Lookup:
			}

			// all of the instructions that the switch lets through are values as per ssa/doc.go
			if v := instr.(ssa.Value); conf.IsSource(utils.Dereference(v.Type())) {
				sources = append(sources, New(v.(ssa.Node), conf))
			}
		}
	}
	return sources
}

func isProducedBySanitizer(v ssa.Value, conf classifier) bool {
	for _, instr := range *v.Referrers() {
		store, ok := instr.(*ssa.Store)
		if !ok {
			continue
		}
		call, ok := store.Val.(*ssa.Call)
		if !ok {
			continue
		}
		if conf.IsSanitizer(call) {
			return true
		}
	}
	return false
}

// indexInBlock returns this instruction's index in its parent block.
func indexInBlock(target ssa.Instruction) (int, bool) {
	for i, instr := range target.Block().Instrs {
		if instr == target {
			return i, true
		}
	}
	// we can only hit this return if there is a bug in the ssa package
	// i.e. an instruction does not appear within its parent block
	return 0, false
}

type stack []*ssa.BasicBlock

func (s *stack) pop() *ssa.BasicBlock {
	if len(*s) == 0 {
		log.Println("tried to pop from empty stack")
	}
	popped := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	return popped
}

func (s *stack) push(b *ssa.BasicBlock) {
	*s = append(*s, b)
}
