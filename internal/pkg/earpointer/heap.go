package earpointer

import (
	"go/types"
	"golang.org/x/tools/go/ssa"
	"strings"
)

// An abstract reference is a local (register) or global variable, possibly
// annotated with some kind of context (call stack, allocation site etc).

// Reference is the base class for local and global references.
type Reference interface {
	Type() types.Type // Return the data type of the underlying local or global
	String() string
	Node() ssa.Value
}

// ReferenceContext represents the calling context of a reference.
// Typically, it contains a stack of call instructions.
type ReferenceContext []ssa.Instruction

// ReferenceLocal is a heap partition that represents all abstract
// objects that a register of reference type can point to in a specific context.
type ReferenceLocal struct {
	context *ReferenceContext
	reg     ssa.Value
}

func (ref ReferenceLocal) Type() types.Type {
	return ref.reg.Type()
}

func (ref ReferenceLocal) Node() ssa.Value {
	return ref.reg
}

func (ref ReferenceLocal) String() string {
	reg := ref.reg
	if reg.Parent() != nil {
		return ref.context.String() + reg.Parent().Name() + "." + reg.Name()
	} else {
		return ref.context.String() + reg.Name()
	}
}

// ReferenceGlobal is a heap partition that represents all abstract
// objects that a global of reference type can point to.
type ReferenceGlobal struct {
	global *ssa.Global
}

func (ref ReferenceGlobal) Type() types.Type {
	return ref.global.Type()
}

func (ref ReferenceGlobal) Node() ssa.Value {
	return ref.global
}

func (ref ReferenceGlobal) String() string {
	return ref.global.String()
}

type InternalKind int

const (
	VALUEOF InternalKind = iota
	FIELD
)

// ReferenceInternal is for references created internally with an operator
// and an operand reference. It is for creating references not directly associated
// with an IR element. For example,
// (1) *r0 can be represented by using operator VALUEOF (*);
// (2) a unknown field can be represented by using operator FIELD.
type ReferenceInternal struct {
	kind InternalKind
	ref  Reference
}

func (ref ReferenceInternal) Type() types.Type {
	return ref.ref.Type()
}

func (ref ReferenceInternal) Node() ssa.Value {
	return ref.ref.Node()
}

func (ref ReferenceInternal) String() string {
	if ref.kind == VALUEOF {
		return "*" + ref.ref.String()
	}
	return ref.ref.String() + "[.]"
}

// Field can be (1) a struct field linked to an IR field (Var);
// (2) or one with a name but not linked to any IR element (irField = nil).
type Field struct {
	Name string
	//lint:ignore U1000 ignore dead code for now
	irField *types.Var
}

// Helper to get the pseudo-field that is used to denote looking up an
// array/slice/map index. For example, consider a register x of type T[], where
// T is a struct type. The contents of the array/slice, namely x[i], is
// over-approximated as the heap partition pointed-to by a pseudo-field named
// getIndexField().
//lint:ignore U1000 ignore dead code for now
func getIndexField() Field {
	return Field{Name: "AnyField"}
}

// A pseudo-field to denote the direct points-to relation. For example, r1 = &r0
// is modeled as r1[getDirectPointToField()] = r0.
func getDirectPointToField() Field {
	return Field{Name: "->"}
}

// Commonly used data structures.

// ReferenceSet is a hash set of references.
type ReferenceSet map[Reference]bool

// ReferenceFieldMap maps from Fields to a reference handle. It
// is used for maintaining points-to relationships for the fields of a
// reference partition. The number of fields is not expected to
// be high on average, so O(n) or O(log(n)) lookup time is acceptable.
type ReferenceFieldMap map[Field]Reference

// Constructs Reference for local "reg" in context "context".
// TODO: hashconst the object to save memory.
func GetReferenceForLocal(context *ReferenceContext, reg ssa.Value) Reference {
	return ReferenceLocal{context: context, reg: reg}
}

func GetReferenceForLocalWithEmptyContext(reg ssa.Value) Reference {
	return ReferenceLocal{context: &emptyContext, reg: reg}
}

func GetReferenceForGlobal(global *ssa.Global) Reference {
	return ReferenceGlobal{global: global}
}

func GetReference(context *ReferenceContext, reg ssa.Value) Reference {
	if global, ok := reg.(*ssa.Global); ok {
		return ReferenceGlobal{global: global}
	}
	return ReferenceLocal{context: context, reg: reg}
}

// Constructs internal Reference.
func GetReferenceForInternal(kind InternalKind, ref Reference) Reference {
	return ReferenceInternal{kind: kind, ref: ref}
}

// Returns whether a value of this type may share an object pointed by other
// values. It is used for identifying copy-by-reference objects. For example, it
// returns false for any integer type, and returns true for pointer type and
// struct type.
//lint:ignore U1000 ignore dead code for now
func typeMayShareObject(tp types.Type) bool {
	switch tp := tp.(type) {
	case *types.Pointer:
		return true
	case *types.Struct:
		return true
	case *types.Chan:
		return true
	case *types.Interface:
		return true
	case *types.Slice:
		return true
	case *types.Array:
		return true
	case *types.Map:
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
var emptyContext = make(ReferenceContext, 0)

func (ctx ReferenceContext) String() string {
	if len(ctx) == 0 {
		return ""
	}
	var s []string
	for _, ins := range ctx {
		s = append(s, ins.String())
	}
	return "[" + strings.Join(s, "; ") + "]"
}
