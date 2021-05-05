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

package earpointer

import (
	"fmt"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// An abstract reference is a local variable (register), global variable,
// or a synthetic element, possibly annotated with some kind of context
// (call stack, allocation site etc).

// Reference is the base interface for local, global, and synthetic references.
type Reference interface {
	fmt.Stringer

	Type() types.Type // Return the data type of the underlying element
	Value() ssa.Value // Return the underlying local or global
}

// Context represents the calling context of a reference.
// Typically, it contains a stack of call instructions.
type Context []ssa.Instruction

// Local is a heap partition that represents all abstract
// objects that a register of reference type can point to in a specific context.
type Local struct {
	context *Context
	reg     ssa.Value
}

func (l Local) Type() types.Type {
	return l.reg.Type()
}

func (l Local) Value() ssa.Value {
	return l.reg
}

func (l Local) String() string {
	// Customize the printing of a value of pointer receiver.
	printReceiverType := func(t types.Type) string {
		switch t := t.(type) {
		case *types.Named:
			return t.Obj().Name()
		case *types.Pointer:
			if et, ok := t.Elem().(*types.Named); ok {
				return "*" + et.Obj().Name()
			}
		}
		return ""
	}
	reg := l.reg
	if f := reg.Parent(); f != nil {
		if recv := f.Signature.Recv(); recv != nil {
			return fmt.Sprintf("%s%s:%s.%s",
				l.context, printReceiverType(recv.Type()), f.Name(), reg.Name())
		}
		return fmt.Sprintf("%s%s.%s", l.context, f.Name(), reg.Name())
	}
	return l.context.String() + reg.Name()
}

// Global is a heap partition that represents all abstract
// objects that a global of reference type can point to.
type Global struct {
	global *ssa.Global
}

func (g Global) Type() types.Type {
	return g.global.Type()
}

func (g Global) Value() ssa.Value {
	return g.global
}

func (g Global) String() string {
	return g.global.Name()
}

type SyntheticKind int

const (
	SyntheticValueOf SyntheticKind = iota
	SyntheticField
)

// Synthetic is for references created internally with an operator
// and an operand reference. It is for creating references not directly associated
// with an IR element. For example,
// (1) *r0 can be represented by using operator SyntheticValueOf (*);
// (2) a unknown field can be represented by using operator SyntheticField.
//     For example, when a struct is cloned, some fields may not appear in the IR,
//     these "unused" fields can be synthesized and stored in the heap.
type Synthetic struct {
	kind SyntheticKind
	ref  Reference
}

func (s Synthetic) Type() types.Type {
	return s.ref.Type()
}

func (s Synthetic) Value() ssa.Value {
	return s.ref.Value()
}

func (s Synthetic) String() string {
	if s.kind == SyntheticValueOf {
		return "*" + s.ref.String()
	}
	return s.ref.String() + "[.]"
}

// Field can be (1) a struct field linked to an IR field (Var);
// (2) or one with a name but not linked to any IR element (irField = nil).
type Field struct {
	Name    string
	irField *types.Var
}

// Helper to get the pseudo-field that is used to denote looking up an
// array/slice/map index. For example, consider a register x of type T[], where
// T is a struct type. The contents of the array/slice, namely x[i], is
// over-approximated as the heap partition pointed-to by a pseudo-field named
// anyIndexField.
var anyIndexField Field = Field{Name: "AnyField"}

// A pseudo-field to denote the direct points-to relation. For example, r1 = &r0
// is modeled as r1[directPointToField] = r0.
var directPointToField Field = Field{Name: "->"}

// Commonly used data structures.

// ReferenceSet is a hash set of references.
type ReferenceSet map[Reference]bool

// FieldMap maps from Fields to a reference handle.
// It is used for maintaining points-to relationships for the fields of a
// reference partition. The number of fields is not expected to
// be high on average, so a tree map may be used instead to save memory.
type FieldMap map[Field]Reference

// Constructs Reference for local "reg" in context "context".
// TODO: hashconst the object to save memory.
func MakeLocal(context *Context, reg ssa.Value) Local {
	return Local{context: context, reg: reg}
}

func MakeLocalWithEmptyContext(reg ssa.Value) Local {
	return Local{context: &emptyContext, reg: reg}
}

func MakeGlobal(global *ssa.Global) Global {
	return Global{global: global}
}

func MakeReference(context *Context, reg ssa.Value) Reference {
	if global, ok := reg.(*ssa.Global); ok {
		return Global{global: global}
	}
	return Local{context: context, reg: reg}
}

// Constructs synthetic reference.
func MakeSynthetic(kind SyntheticKind, ref Reference) Synthetic {
	return Synthetic{kind: kind, ref: ref}
}

// Returns whether a value of this type may share an object pointed by other
// values. It is used for identifying copy-by-reference objects. For example, it
// returns false for any integer type, and returns true for pointer type and
// struct type.
func typeMayShareObject(tp types.Type) bool {
	switch tp := tp.(type) {
	case *types.Pointer,
		*types.Struct,
		*types.Chan,
		*types.Interface,
		*types.Slice,
		*types.Signature,
		*types.Array,
		*types.Map:
		return true
	case *types.Basic:
		if tp.Kind() == types.String || tp.Kind() == types.UnsafePointer {
			return true
		}
	case *types.Named:
		return typeMayShareObject(tp.Underlying())
	}
	return false
}

// emptyContext defines an empty context handle to be shared by local references.
var emptyContext Context

func (ctx Context) String() string {
	if len(ctx) == 0 {
		return ""
	}
	s := make([]string, len(ctx))
	for i, ins := range ctx {
		s[i] = ins.String()
	}
	return "[" + strings.Join(s, "; ") + "]"
}
