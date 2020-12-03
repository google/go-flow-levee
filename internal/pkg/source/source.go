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

	"github.com/google/go-flow-levee/internal/pkg/fieldtags"
	"github.com/google/go-flow-levee/internal/pkg/sanitizer"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

type classifier interface {
	IsSourceType(path, typeName string) bool
	IsSourceField(path, typeName, fieldName string) bool
	IsSanitizer(path, recv, name string) bool
	IsSink(path, recv, name string) bool
	IsExcluded(path, recv, name string) bool
}

// Source represents a Source in an SSA call tree.
// It is based on ssa.Node, with the added functionality of computing the recursive graph of
// its referrers.
// Source.sanitized notes sanitizer calls that sanitize this Source
type Source struct {
	node         ssa.Node
	marked       map[ssa.Node]bool
	preOrder     []ssa.Node
	sanitizers   []*sanitizer.Sanitizer
	config       classifier
	taggedFields fieldtags.ResultType
}

// Pos returns the token position of the SSA Node associated with the Source.
func (s *Source) Pos() token.Pos {
	// Extracts don't have a registered position in the source code,
	// so we need to use the position of their related Tuple.
	if e, ok := s.node.(*ssa.Extract); ok {
		return e.Tuple.Pos()
	}
	// Fields don't *always* have a registered position in the source code,
	// e.g. when accessing an embedded field.
	if f, ok := s.node.(*ssa.Field); ok && f.Pos() == token.NoPos {
		return f.X.Pos()
	}
	// FieldAddrs don't *always* have a registered position in the source code,
	// e.g. when accessing an embedded field.
	if f, ok := s.node.(*ssa.FieldAddr); ok && f.Pos() == token.NoPos {
		return f.X.Pos()
	}
	return s.node.Pos()
}

// New constructs a Source
func New(in ssa.Node, config classifier, taggedFields fieldtags.ResultType) *Source {
	s := &Source{
		node:         in,
		marked:       make(map[ssa.Node]bool),
		config:       config,
		taggedFields: taggedFields,
	}
	s.dfs(in, map[*ssa.BasicBlock]int{}, nil, false)
	return s
}

// dfs performs Depth-First-Search on the def-use graph of the input Source.
// While traversing the graph we also look for potential sanitizers of this Source.
// If the Source passes through a sanitizer, dfs does not continue through that Node.
func (s *Source) dfs(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock, isReferrer bool) {
	if s.shouldNotVisit(n, maxInstrReached, lastBlockVisited, isReferrer) {
		return
	}
	s.preOrder = append(s.preOrder, n)
	s.marked[n] = true

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

	s.visit(n, mirCopy, lastBlockVisited)
}

func (s *Source) shouldNotVisit(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock, isReferrer bool) bool {
	if s.marked[n] {
		return true
	}

	// booleans can't meaningfully be tainted
	if isBoolean(n) {
		return true
	}

	if instr, ok := n.(ssa.Instruction); ok {
		instrIndex, ok := indexInBlock(instr)
		if !ok {
			return true
		}

		// If the referrer is in a different block from the one we last visited,
		// and it can't be reached from the block we are visiting, then stop visiting.
		if lastBlockVisited != nil && instr.Block() != lastBlockVisited && !s.canReach(lastBlockVisited, instr.Block()) {
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

func (s *Source) visit(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	if s.node == n {
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		return
	}

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
			s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		}

	case *ssa.Call:
		if callee := t.Call.StaticCallee(); callee != nil && s.config.IsSanitizer(utils.DecomposeFunction(callee)) {
			s.sanitizers = append(s.sanitizers, &sanitizer.Sanitizer{Call: t})
		}

		// This is to avoid attaching calls where the source is the receiver, ex:
		// core.Sinkf("Source id: %v", wrapper.Source.GetID())
		if recv := t.Call.Signature().Recv(); recv != nil && isSourceType(s.config, s.taggedFields, recv.Type()) {
			return
		}

		s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		s.visitOperands(n, maxInstrReached, lastBlockVisited)

	case *ssa.FieldAddr:
		deref := utils.Dereference(t.X.Type())
		typPath, typName := utils.DecomposeType(deref)
		fieldName := utils.FieldName(t)
		if !s.config.IsSourceField(typPath, typName, fieldName) && !s.taggedFields.IsSourceFieldAddr(t) {
			return
		}
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		s.visitOperands(n, maxInstrReached, lastBlockVisited)

	// Everything but the actual integer Index should be visited.
	case *ssa.Index:
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		s.dfs(t.X.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// The actual integer Index should not be visited.
	// Everything but the actual integer Index should be visited.
	case *ssa.IndexAddr:
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		s.dfs(t.X.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// Only the Addr (the Value that is being written to) should be visited.
	case *ssa.Store:
		s.dfs(t.Addr.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// Only the Map itself can be tainted by an Update.
	// The Key can't be tainted.
	// The Value can propagate taint to the Map, but not receive it.
	// MapUpdate has no referrers, it is only an Instruction, not a Value.
	case *ssa.MapUpdate:
		s.dfs(t.Map.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// The only Operand that can be tainted by a Send is the Chan.
	// The Value can propagate taint to the Chan, but not receive it.
	// Send has no referrers, it is only an Instruction, not a Value.
	case *ssa.Send:
		s.dfs(t.Chan.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// These nodes' operands should not be visited, because they can only receive
	// taint from their operands, not propagate taint to them.
	case *ssa.BinOp, *ssa.ChangeInterface, *ssa.ChangeType, *ssa.Convert, *ssa.Extract, *ssa.MakeChan, *ssa.MakeMap, *ssa.MakeSlice, *ssa.Phi, *ssa.Range, *ssa.Slice, *ssa.UnOp:
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)

	// These nodes don't have operands; they are Values, not Instructions.
	case *ssa.Const, *ssa.FreeVar, *ssa.Global, *ssa.Lookup, *ssa.Parameter:
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)

	// These nodes don't have referrers; they are Instructions, not Values.
	case *ssa.Go:
		s.visitOperands(n, maxInstrReached, lastBlockVisited)

	// These nodes are both Instructions and Values, and currently have no special restrictions.
	case *ssa.Field, *ssa.MakeInterface, *ssa.Select, *ssa.TypeAssert:
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		s.visitOperands(n, maxInstrReached, lastBlockVisited)

	// These nodes cannot propagate taint.
	case *ssa.Builtin, *ssa.DebugRef, *ssa.Defer, *ssa.Function, *ssa.If, *ssa.Jump, *ssa.MakeClosure, *ssa.Next, *ssa.Panic, *ssa.Return, *ssa.RunDefers:

	default:
		fmt.Printf("unexpected node received: %T %v; please report this issue\n", n, n)
	}
}

func (s *Source) visitReferrers(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	if n.Referrers() == nil {
		return
	}
	for _, r := range *n.Referrers() {
		s.dfs(r.(ssa.Node), maxInstrReached, lastBlockVisited, true)
	}
}

func (s *Source) visitOperands(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	for _, o := range n.Operands(nil) {
		if *o == nil {
			continue
		}
		s.dfs((*o).(ssa.Node), maxInstrReached, lastBlockVisited, false)
	}
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

func identify(conf classifier, ssaInput *buildssa.SSA, taggedFields fieldtags.ResultType) map[*ssa.Function][]*Source {
	sourceMap := make(map[*ssa.Function][]*Source)

	for _, fn := range ssaInput.SrcFuncs {
		// no need to analyze the body of sinks, nor of excluded functions
		path, recv, name := utils.DecomposeFunction(fn)
		if conf.IsSink(path, recv, name) || conf.IsExcluded(path, recv, name) {
			continue
		}

		var sources []*Source
		sources = append(sources, sourcesFromParams(fn, conf, taggedFields)...)
		sources = append(sources, sourcesFromClosure(fn, conf, taggedFields)...)
		sources = append(sources, sourcesFromBlocks(fn, conf, taggedFields)...)

		if len(sources) > 0 {
			sourceMap[fn] = sources
		}
	}
	return sourceMap
}

func sourcesFromParams(fn *ssa.Function, conf classifier, taggedFields fieldtags.ResultType) []*Source {
	var sources []*Source
	for _, p := range fn.Params {
		if isSourceType(conf, taggedFields, p.Type()) {
			sources = append(sources, New(p, conf, taggedFields))
		}
	}
	return sources
}

func sourcesFromClosure(fn *ssa.Function, conf classifier, taggedFields fieldtags.ResultType) []*Source {
	var sources []*Source
	for _, p := range fn.FreeVars {
		switch t := p.Type().(type) {
		case *types.Pointer:
			if isSourceType(conf, taggedFields, t) {
				sources = append(sources, New(p, conf, taggedFields))
			}
		}
	}
	return sources
}

// sourcesFromBlocks finds Source values created by instructions within a function's body.
func sourcesFromBlocks(fn *ssa.Function, conf classifier, taggedFields fieldtags.ResultType) []*Source {
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
				if _, ok := t.(*types.Pointer); ok && isSourceType(conf, taggedFields, t) {
					sources = append(sources, New(v, conf, taggedFields))
				}
				continue

			// source received from chan
			case *ssa.UnOp:
				// not a <-chan operation
				if v.Op != token.ARROW {
					continue
				}

			// source obtained through a field or an index operation
			case *ssa.Field, *ssa.FieldAddr, *ssa.Index, *ssa.IndexAddr, *ssa.Lookup:

			// source chan or map (arrays and slices have regular Allocs)
			case *ssa.MakeMap, *ssa.MakeChan:
			}

			// all of the instructions that the switch lets through are values as per ssa/doc.go
			if v := instr.(ssa.Value); isSourceType(conf, taggedFields, v.Type()) {
				sources = append(sources, New(v.(ssa.Node), conf, taggedFields))
			}
		}
	}
	return sources
}

func isBoolean(n ssa.Node) bool {
	if v, ok := n.(ssa.Value); ok {
		if basic, ok := v.Type().(*types.Basic); ok && basic.Info() == types.IsBoolean {
			return true
		}
	}
	return false
}

func isSourceType(c classifier, tf fieldtags.ResultType, t types.Type) bool {
	deref := utils.Dereference(t)
	switch tt := deref.(type) {
	case *types.Named:
		return c.IsSourceType(utils.DecomposeType(tt)) || isSourceType(c, tf, tt.Underlying())
	case *types.Array:
		return isSourceType(c, tf, tt.Elem())
	case *types.Slice:
		return isSourceType(c, tf, tt.Elem())
	case *types.Chan:
		return isSourceType(c, tf, tt.Elem())
	case *types.Map:
		key := isSourceType(c, tf, tt.Key())
		elem := isSourceType(c, tf, tt.Elem())
		return key || elem
	case *types.Struct:
		return hasTaggedField(tf, tt)
	case *types.Basic, *types.Tuple, *types.Interface, *types.Signature:
		// These types do not currently represent possible source types
		return false
	case *types.Pointer:
		// This should be unreachable due to the dereference above
		return false
	default:
		// The above should be exhaustive.  Reaching this default case is an error.
		fmt.Printf("unexpected type received: %T %v; please report this issue\n", tt, tt)
		return false
	}
}

func hasTaggedField(taggedFields fieldtags.ResultType, s *types.Struct) bool {
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if taggedFields.IsSource(f) {
			return true
		}
	}
	return false
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
		if callee := call.Call.StaticCallee(); callee != nil && conf.IsSanitizer(utils.DecomposeFunction(callee)) {
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
