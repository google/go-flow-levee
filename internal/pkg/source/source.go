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

	"github.com/google/go-flow-levee/internal/pkg/fieldtags"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

type Classifier interface {
	IsSourceType(path, typeName string) bool
	IsSourceField(path, typeName, fieldName string) bool
	IsSanitizer(path, recv, name string) bool
	IsSink(path, recv, name string) bool
	IsExcluded(path, recv, name string) bool
}

// Source represents a Source in an SSA call tree.
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

// New constructs a Source
func New(in ssa.Node) *Source {
	return &Source{
		Node: in,
	}
}

func identify(conf Classifier, ssaInput *buildssa.SSA, taggedFields fieldtags.ResultType) map[*ssa.Function][]*Source {
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

func sourcesFromParams(fn *ssa.Function, conf Classifier, taggedFields fieldtags.ResultType) []*Source {
	var sources []*Source
	for _, p := range fn.Params {
		if IsSourceType(conf, taggedFields, p.Type()) {
			sources = append(sources, New(p))
		}
	}
	return sources
}

func sourcesFromClosure(fn *ssa.Function, conf Classifier, taggedFields fieldtags.ResultType) []*Source {
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
func sourcesFromBlocks(fn *ssa.Function, conf Classifier, taggedFields fieldtags.ResultType) []*Source {
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
			case *ssa.TypeAssert:
				if !v.CommaOk && IsSourceType(conf, taggedFields, v.AssertedType) {
					sources = append(sources, New(v))
				}

			// An Extract is used to obtain a value from an instruction that returns multiple values.
			// If the Extract is used to get a Pointer, create a Source, otherwise the Source won't
			// have an Alloc and we'll miss it.
			case *ssa.Extract:
				t := v.Tuple.Type().(*types.Tuple).At(v.Index).Type()
				if _, ok := t.(*types.Pointer); ok && IsSourceType(conf, taggedFields, t) {
					sources = append(sources, New(v))
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
				sources = append(sources, New(v.(ssa.Node)))
			}
		}
	}
	return sources
}

func IsSourceType(c Classifier, tf fieldtags.ResultType, t types.Type) bool {
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

func isProducedBySanitizer(v ssa.Value, conf Classifier) bool {
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
