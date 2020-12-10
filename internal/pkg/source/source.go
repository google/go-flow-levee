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
	"golang.org/x/tools/go/pointer"
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
	Node         ssa.Node
	Marked       map[ssa.Node]bool
	PreOrder     []ssa.Node
	Sanitizers   []*sanitizer.Sanitizer
	Config       classifier
	TaggedFields fieldtags.ResultType
}

// Pos returns the token position of the SSA Node associated with the Source.
func (s *Source) Pos() token.Pos {
	// Extracts don't have a registered position in the source code,
	// so we need to use the position of their related Tuple.
	if e, ok := s.Node.(*ssa.Extract); ok {
		return e.Tuple.Pos()
	}
	// Fields don't *always* have a registered position in the source code,
	// e.g. when accessing an embedded field.
	if f, ok := s.Node.(*ssa.Field); ok && f.Pos() == token.NoPos {
		return f.X.Pos()
	}
	// FieldAddrs don't *always* have a registered position in the source code,
	// e.g. when accessing an embedded field.
	if f, ok := s.Node.(*ssa.FieldAddr); ok && f.Pos() == token.NoPos {
		return f.X.Pos()
	}
	return s.Node.Pos()
}

// New constructs a Source
func New(in ssa.Node, config classifier, taggedFields fieldtags.ResultType) *Source {
	return &Source{
		Node:         in,
		Marked:       make(map[ssa.Node]bool),
		Config:       config,
		TaggedFields: taggedFields,
	}
}

// compress removes the elements from the graph that are not required by the
// taint-propagation analysis. Concretely, only propagators, sanitizers and
// sinks should constitute the output. Since, we already know what the source
// is, it is also removed.
func (s *Source) compress() []ssa.Node {
	var compressed []ssa.Node
	for _, n := range s.PreOrder {
		switch n.(type) {
		case *ssa.Call:
			compressed = append(compressed, n)
		}
	}

	return compressed
}

// HasPathTo returns true when a Node is part of declaration-use graph.
func (s *Source) HasPathTo(n ssa.Node) bool {
	return s.Marked[n]
}

// IsSanitizedAt returns true when the Source is sanitized by the supplied instruction.
func (s *Source) IsSanitizedAt(call ssa.Instruction) bool {
	for _, san := range s.Sanitizers {
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
		if IsSourceType(conf, taggedFields, p.Type()) {
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
			if IsSourceType(conf, taggedFields, t) {
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
				if _, ok := t.(*types.Pointer); ok && IsSourceType(conf, taggedFields, t) {
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
			if v := instr.(ssa.Value); IsSourceType(conf, taggedFields, v.Type()) {
				sources = append(sources, New(v.(ssa.Node), conf, taggedFields))
			}
		}
	}
	return sources
}

func HasTaintableType(n ssa.Node) bool {
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

func IsSourceType(c classifier, tf fieldtags.ResultType, t types.Type) bool {
	deref := utils.Dereference(t)
	switch tt := deref.(type) {
	case *types.Named:
		return c.IsSourceType(utils.DecomposeType(tt)) || IsSourceType(c, tf, tt.Underlying())
	case *types.Array:
		return IsSourceType(c, tf, tt.Elem())
	case *types.Slice:
		return IsSourceType(c, tf, tt.Elem())
	case *types.Chan:
		return IsSourceType(c, tf, tt.Elem())
	case *types.Map:
		key := IsSourceType(c, tf, tt.Key())
		elem := IsSourceType(c, tf, tt.Elem())
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

// A type can be tainted by a call if it is itself a pointer or pointer-like type (according to
// pointer.CanPoint), or it is an array/struct that holds an element that can be tainted by
// a call.
func CanBeTaintedByCall(t types.Type) bool {
	if pointer.CanPoint(t) {
		return true
	}

	switch tt := t.(type) {
	case *types.Array:
		return CanBeTaintedByCall(tt.Elem())

	case *types.Struct:
		for i := 0; i < tt.NumFields(); i++ {
			// this cannot cause an infinite loop, because a struct
			// type cannot refer to itself except through a pointer
			if CanBeTaintedByCall(tt.Field(i).Type()) {
				return true
			}
		}
		return false
	}

	return false
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
func IndexInBlock(target ssa.Instruction) (int, bool) {
	for i, instr := range target.Block().Instrs {
		if instr == target {
			return i, true
		}
	}
	// we can only hit this return if there is a bug in the ssa package
	// i.e. an instruction does not appear within its parent block
	return 0, false
}

type Stack []*ssa.BasicBlock

func (s *Stack) Pop() *ssa.BasicBlock {
	if len(*s) == 0 {
		log.Println("tried to pop from empty stack")
	}
	popped := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	return popped
}

func (s *Stack) Push(b *ssa.BasicBlock) {
	*s = append(*s, b)
}
