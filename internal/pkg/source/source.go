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

// Package source can be used to identify SSA values that are Sources.
package source

import (
	"fmt"
	"go/token"
	"go/types"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/fieldpropagator"
	"github.com/google/go-flow-levee/internal/pkg/fieldtags"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

// A Source is a node in the SSA graph that is used as a
// starting point in a propagation analysis.
type Source struct {
	Node ssa.Node
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

// New constructs a new Source.
func New(in ssa.Node) *Source {
	return &Source{
		Node: in,
	}
}

// identify individually examines each Function in the SSA code looking for Sources.
// It produces a map relating a Function to the Sources it contains.
// If a Function contains no Sources, it does not appear in the map.
func identify(conf *config.Config, ssaInput *buildssa.SSA, taggedFields fieldtags.ResultType, propagators fieldpropagator.ResultType) map[*ssa.Function][]*Source {
	sourceMap := make(map[*ssa.Function][]*Source)

	for _, fn := range ssaInput.SrcFuncs {
		// no need to analyze the body of sinks, nor of excluded functions
		path, recv, name := utils.DecomposeFunction(fn)
		if conf.IsSink(path, recv, name) || conf.IsExcluded(path, recv, name) {
			continue
		}

		var sources []*Source
		sources = append(sources, sourcesFromParams(fn, conf, taggedFields)...)
		sources = append(sources, sourcesFromClosures(fn, conf, taggedFields)...)
		sources = append(sources, sourcesFromBlocks(fn, conf, taggedFields, propagators)...)

		if len(sources) > 0 {
			sourceMap[fn] = sources
		}
	}
	return sourceMap
}

// sourcesFromParams identifies Sources that appear within a Function's parameters.
func sourcesFromParams(fn *ssa.Function, conf *config.Config, taggedFields fieldtags.ResultType) []*Source {
	var sources []*Source
	for _, p := range fn.Params {
		if IsSourceType(conf, taggedFields, p.Type()) {
			sources = append(sources, New(p))
		}
	}
	return sources
}

// sourcesFromClosures identifies Source values which are captured by closures.
// A value that is captured by a closure will appear as a Free Variable in the
// closure. In the SSA, a Free Variable is represented as a Pointer, distinct
// from the original value.
func sourcesFromClosures(fn *ssa.Function, conf *config.Config, taggedFields fieldtags.ResultType) []*Source {
	var sources []*Source
	for _, p := range fn.FreeVars {
		switch t := p.Type().(type) {
		case *types.Pointer:
			if IsSourceType(conf, taggedFields, t) {
				sources = append(sources, New(p))
			}
		}
	}
	return sources
}

// sourcesFromBlocks finds Source values created by instructions within a function's body.
func sourcesFromBlocks(fn *ssa.Function, conf *config.Config, taggedFields fieldtags.ResultType, propagators fieldpropagator.ResultType) []*Source {
	var sources []*Source
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			// all of the instructions that the switch lets through are values as per ssa/doc.go
			if foo(instr, conf, propagators, taggedFields) {
				sources = append(sources, New(instr.(ssa.Node)))
			}
		}
	}
	return sources
}

func foo(instr ssa.Instruction, conf *config.Config, propagators fieldpropagator.ResultType, taggedFields fieldtags.ResultType) bool {
	// This type switch is used to catch instructions that could produce sources.
	// All instructions that do not match one of the cases will hit the "default"
	// and they will not be examined any further.
	switch v := instr.(type) {
	// drop anything that doesn't match one of the following cases
	default:
		return false
	// Values produced by sanitizers are not sources.
	case *ssa.Alloc:
		if isProducedBySanitizer(v, conf) {
			return false
		}

	// Values produced by sanitizers are not sources.
	// Values produced by field propagators are.
	case *ssa.Call:
		if isProducedBySanitizer(v, conf) {
			return false
		}

		if propagators.IsFieldPropagator(v) {
			return true
		}

	// A panicky type assert like s := e.(*core.Source) does not result in ssa.Extract
	// so we need to create a source if the type assert is panicky i.e. CommaOk is false
	// and the type being asserted is a source type.
	case *ssa.TypeAssert:
		if !v.CommaOk && IsSourceType(conf, taggedFields, v.AssertedType) {
			return true
		}

	// An Extract is used to obtain a value from an instruction that returns multiple values.
	// If the extracted value is a Pointer to a Source, it won't have an Alloc, so we need to
	// identify the Source from the Extract.
	case *ssa.Extract:
		t := v.Tuple.Type().(*types.Tuple).At(v.Index).Type()
		if _, ok := t.(*types.Pointer); ok && IsSourceType(conf, taggedFields, t) {
			return true
		}
		return false

	// value received from chan
	case *ssa.UnOp:
		// not a <-chan operation
		if v.Op != token.ARROW {
			return false
		}

	// value obtained through a field or an index operation
	case *ssa.Field, *ssa.FieldAddr, *ssa.Index, *ssa.IndexAddr, *ssa.Lookup:

	// chan or map value (arrays and slices are created using Allocs)
	case *ssa.MakeMap, *ssa.MakeChan:
	}

	return IsSourceType(conf, taggedFields, instr.(ssa.Value).Type())
}

// IsSourceType determines whether a Type is a Source Type.
// A Source Type is either:
// - A Named Type that is classified as a Source
// - A composite type that contains a Source Type
// - A Struct Type that contains a tagged field
func IsSourceType(c *config.Config, tf fieldtags.ResultType, t types.Type) bool {
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

func hasTaggedField(taggedFields fieldtags.ResultType, s *types.Struct) bool {
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if taggedFields.IsSource(f) {
			return true
		}
	}
	return false
}

func isProducedBySanitizer(v ssa.Value, conf *config.Config) bool {
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
