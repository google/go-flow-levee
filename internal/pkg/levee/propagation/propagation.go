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

// Package propagation implements the core taint propagation analysis that
// can be used to determine what ssa Nodes can be reached from a given Node.
package propagation

import (
	"fmt"
	"go/types"
	"log"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/fieldtags"
	"github.com/google/go-flow-levee/internal/pkg/sanitizer"
	"github.com/google/go-flow-levee/internal/pkg/source"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

// Propagation represents the information that is used by, and collected
// during, a taint propagation analysis.
type Propagation struct {
	root         ssa.Node
	tainted      map[ssa.Node]bool
	preOrder     []ssa.Node
	sanitizers   []*sanitizer.Sanitizer
	config       *config.Config
	taggedFields fieldtags.ResultType
}

// Taint performs a depth-first search of the graph formed by SSA Referrers and
// Operands relationships, beginning at the given root node.
func Taint(n ssa.Node, conf *config.Config, taggedFields fieldtags.ResultType) Propagation {
	prop := Propagation{
		root:         n,
		tainted:      make(map[ssa.Node]bool),
		config:       conf,
		taggedFields: taggedFields,
	}
	maxInstrReached := map[*ssa.BasicBlock]int{}

	prop.taintReferrers(n, maxInstrReached, nil)
	prop.taint(n, maxInstrReached, nil, false)

	return prop
}

// taint performs a depth-first search of the graph formed by SSA Referrers and
// Operands relationships. Along the way, visited nodes are marked and stored
// in a slice which captures the visitation order. Sanitizers are also recorded.
// maxInstrReached and lastBlockVisited are used to give the traversal some
// degree of flow sensitivity. Specifically:
// - maxInstrReached records the highest index of an instruction visited
//   in each block. This is used to avoid visiting past instructions, e.g.
//   a call to a sink where the argument was tainted after the call happened.
// - lastBlockVisited is used to determine whether the next instruction to visit
//   can be reached from the current instruction.
func (prop *Propagation) taint(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock, isReferrer bool) {
	if prop.shouldNotTaint(n, maxInstrReached, lastBlockVisited, isReferrer) {
		return
	}
	prop.preOrder = append(prop.preOrder, n)
	prop.tainted[n] = true

	mirCopy := map[*ssa.BasicBlock]int{}
	for m, i := range maxInstrReached {
		mirCopy[m] = i
	}

	if instr, ok := n.(ssa.Instruction); ok {
		instrIndex, ok := indexInBlock(instr)
		if !ok {
			return
		}

		if mirCopy[instr.Block()] < instrIndex {
			mirCopy[instr.Block()] = instrIndex
		}

		lastBlockVisited = instr.Block()
	}

	prop.taintNeighbors(n, mirCopy, lastBlockVisited)
}

func (prop *Propagation) shouldNotTaint(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock, isReferrer bool) bool {
	if prop.tainted[n] {
		return true
	}

	if !hasTaintableType(n) {
		return true
	}

	if instr, ok := n.(ssa.Instruction); ok {
		instrIndex, ok := indexInBlock(instr)
		if !ok {
			return true
		}

		// If the referrer is in a different block from the one we last visited,
		// and it can't be reached from the block we are visiting, then stop visiting.
		if lastBlockVisited != nil && instr.Block() != lastBlockVisited && !prop.canReach(lastBlockVisited, instr.Block()) {
			return true
		}

		// If this call's index is lower than the highest seen so far in its block,
		// then this call is "in the past". If this call is a referrer,
		// then we would be propagating taint backwards in time, so stop traversing.
		// (If the call is an operand, then it is being used as a value, so it does
		// not matter when the call occurred.)
		if _, ok := instr.(*ssa.Call); ok && instrIndex < maxInstrReached[instr.Block()] && isReferrer {
			return true
		}
	}

	return false
}

func (prop *Propagation) taintNeighbors(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	switch t := n.(type) {
	case *ssa.Alloc:
		// An Alloc represents the allocation of space for a variable. If a Node is an Alloc,
		// and the thing being allocated is not an array, then either:
		// a) it is a Source value, in which case it will get its own traversal when sourcesFromBlocks
		//    finds this Alloc
		// b) it is not a Source value, in which case we should not visit it.
		// However, if the Alloc is an array, then that means the source that we are visiting from
		// is being placed into an array, slice or varags, so we do need to keep visiting.
		if _, isArray := utils.Dereference(t.Type()).(*types.Array); isArray {
			prop.taintReferrers(n, maxInstrReached, lastBlockVisited)
		}

	case *ssa.Call:
		// The builtin delete(m map[Type]Type1, key Type) func does not propagate a taint.
		if builtin, ok := t.Call.Value.(*ssa.Builtin); ok && builtin.Name() == "delete" {
			return
		}

		if callee := t.Call.StaticCallee(); callee != nil && prop.config.IsSanitizer(utils.DecomposeFunction(callee)) {
			prop.sanitizers = append(prop.sanitizers, &sanitizer.Sanitizer{Call: t})
		}

		// This is to avoid attaching calls where the source is the receiver, ex:
		// core.Sinkf("Source id: %v", wrapper.Source.GetID())
		if recv := t.Call.Signature().Recv(); recv != nil && source.IsSourceType(prop.config, prop.taggedFields, recv.Type()) {
			return
		}

		prop.taintReferrers(n, maxInstrReached, lastBlockVisited)
		for _, a := range t.Call.Args {
			if canBeTaintedByCall(a.Type()) {
				prop.taint(a.(ssa.Node), maxInstrReached, lastBlockVisited, false)
			}
		}

	case *ssa.FieldAddr:
		deref := utils.Dereference(t.X.Type())
		typPath, typName := utils.DecomposeType(deref)
		fieldName := utils.FieldName(t)
		if !prop.config.IsSourceField(typPath, typName, fieldName) && !prop.taggedFields.IsSourceFieldAddr(t) {
			return
		}
		prop.taintReferrers(n, maxInstrReached, lastBlockVisited)
		prop.taintOperands(n, maxInstrReached, lastBlockVisited)

	// Everything but the actual integer Index should be visited.
	case *ssa.Index:
		prop.taintReferrers(n, maxInstrReached, lastBlockVisited)
		prop.taint(t.X.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// Everything but the actual integer Index should be visited.
	case *ssa.IndexAddr:
		prop.taintReferrers(n, maxInstrReached, lastBlockVisited)
		prop.taint(t.X.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// Only the Addr (the Value that is being written to) should be visited.
	case *ssa.Store:
		prop.taint(t.Addr.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// Only the Map itself can be tainted by an Update.
	// The Key can't be tainted.
	// The Value can propagate taint to the Map, but not receive it.
	// MapUpdate has no referrers, it is only an Instruction, not a Value.
	case *ssa.MapUpdate:
		prop.taint(t.Map.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// The only Operand that can be tainted by a Send is the Chan.
	// The Value can propagate taint to the Chan, but not receive it.
	// Send has no referrers, it is only an Instruction, not a Value.
	case *ssa.Send:
		prop.taint(t.Chan.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// These nodes' operands should not be visited, because they can only receive
	// taint from their operands, not propagate taint to them.
	case *ssa.BinOp, *ssa.ChangeInterface, *ssa.ChangeType, *ssa.Convert, *ssa.Extract, *ssa.MakeChan, *ssa.MakeMap, *ssa.MakeSlice, *ssa.Phi, *ssa.Range:
		prop.taintReferrers(n, maxInstrReached, lastBlockVisited)

	// These nodes don't have operands; they are Values, not Instructions.
	case *ssa.Const, *ssa.FreeVar, *ssa.Global, *ssa.Lookup, *ssa.Parameter:
		prop.taintReferrers(n, maxInstrReached, lastBlockVisited)

	// These nodes don't have referrers; they are Instructions, not Values.
	case *ssa.Go:
		prop.taintOperands(n, maxInstrReached, lastBlockVisited)

	// These nodes are both Instructions and Values, and currently have no special restrictions.
	case *ssa.Field, *ssa.MakeInterface, *ssa.Select, *ssa.Slice, *ssa.TypeAssert, *ssa.UnOp:
		prop.taintReferrers(n, maxInstrReached, lastBlockVisited)
		prop.taintOperands(n, maxInstrReached, lastBlockVisited)

	// These nodes cannot propagate taint.
	case *ssa.Builtin, *ssa.DebugRef, *ssa.Defer, *ssa.Function, *ssa.If, *ssa.Jump, *ssa.MakeClosure, *ssa.Next, *ssa.Panic, *ssa.Return, *ssa.RunDefers:

	default:
		fmt.Printf("unexpected node received: %T %v; please report this issue\n", n, n)
	}
}

func (prop *Propagation) taintReferrers(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	if n.Referrers() == nil {
		return
	}
	for _, r := range *n.Referrers() {
		prop.taint(r.(ssa.Node), maxInstrReached, lastBlockVisited, true)
	}
}

func (prop *Propagation) taintOperands(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	for _, o := range n.Operands(nil) {
		if *o == nil {
			continue
		}
		prop.taint((*o).(ssa.Node), maxInstrReached, lastBlockVisited, false)
	}
}

func (prop *Propagation) canReach(start *ssa.BasicBlock, dest *ssa.BasicBlock) bool {
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

// HasPathTo determines whether a Node can be reached
// from the Propagation's root.
func (prop Propagation) HasPathTo(n ssa.Node) bool {
	return prop.tainted[n]
}

// IsSanitizedAt determines whether the Propagation's root is sanitized
// when it reaches the given instruction.
func (prop Propagation) IsSanitizedAt(instr ssa.Instruction) bool {
	for _, san := range prop.sanitizers {
		if san.Dominates(instr) {
			return true
		}
	}

	return false
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

func hasTaintableType(n ssa.Node) bool {
	if v, ok := n.(ssa.Value); ok {
		switch t := v.Type().(type) {
		case *types.Basic:
			return t.Info() != types.IsBoolean
		case *types.Signature:
			return false
		}
	}
	return true
}

// A type can be tainted by a call if it is itself a pointer or pointer-like type (according to
// pointer.CanPoint), or it is an array/struct that holds an element that can be tainted by
// a call.
func canBeTaintedByCall(t types.Type) bool {
	if pointer.CanPoint(t) {
		return true
	}

	switch tt := t.(type) {
	case *types.Array:
		return canBeTaintedByCall(tt.Elem())

	case *types.Struct:
		for i := 0; i < tt.NumFields(); i++ {
			// this cannot cause an infinite loop, because a struct
			// type cannot refer to itself except through a pointer
			if canBeTaintedByCall(tt.Field(i).Type()) {
				return true
			}
		}
		return false
	}

	return false
}
