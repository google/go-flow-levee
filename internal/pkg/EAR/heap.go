package EAR

import (
	"go/types"
	"golang.org/x/tools/go/ssa"
	"strings"
)

// An abstract reference is a local (register) or global variable, possibly
// annotated with some kind of context (call stack, allocation site etc).

// AbsReference is the base class for local and global AbsReferences.
type AbsReference interface {
	DataType() types.Type // Return the data type of the underlying local or global
	String() string
}

// AbsReferenceContext contains a stack of call instructions.
type AbsReferenceContext []ssa.Instruction

// AbsReferenceLocal is a heap partition that represents all abstract
// objects that a register of reference type can point to in a specific context.
type AbsReferenceLocal struct {
	context *AbsReferenceContext
	reg     ssa.Value
}

func (ref AbsReferenceLocal) DataType() types.Type {
	return ref.reg.Type()
}

func (ref AbsReferenceLocal) String() string {
	return ref.context.String() + ref.reg.Name()
}

// AbsReferenceGlobal is a heap partition that represents all abstract
// objects that a global of reference type can point to.
type AbsReferenceGlobal struct {
	global *ssa.Global
}

func (ref AbsReferenceGlobal) DataType() types.Type {
	return ref.global.Type()
}

func (ref AbsReferenceGlobal) String() string {
	return ref.global.String()
}

// AbsReferenceInternal is for references created internally with an operator
// and an operand reference. It is for creating references not directly associated
// with an IR element. For example,
// (1) *r0 can be represented by using operator VALUEOF (*);
// (2) a unknown field can be represented by using operator FIELD.
type AbsReferenceInternal struct {
	kind uint8 // 0 for VALUEOF, 1 for FIELD
	ref  AbsReference
}

func (ref AbsReferenceInternal) DataType() types.Type {
	return ref.ref.DataType()
}

func (ref AbsReferenceInternal) String() string {
	if ref.kind == 0 {
		return "*" + ref.ref.String()
	} else {
		return ref.ref.String() + "[.]"
	}
	return "unknown"
}

// Commonly used data structures.

// AbsReferenceSet is a hash set of abstract references.
type AbsReferenceSet map[AbsReference]bool

// Field can be (1) a struct field linked to an IR field (Var);
// (2) or one with a name but not linked to any IR element (irField = nil).
type Field struct {
	name    string
	irField *types.Var
}

// AbsReferenceFieldMap maps from Fields to an abstract reference handle. It
// is used for maintaining points-to relationships for the fields of an abstract
// reference partition. The number of fields is not expected to
// be high on average, so O(n) or O(log(n)) lookup time is acceptable.
type AbsReferenceFieldMap map[Field]AbsReference

// Helper to get the pseudo-field that is used to denote looking up an
// array/slice/map index. For example, consider a register x of type T[], where
// T is a struct type. The contents of the array/slice, namely x[i], is
// over-approximated as the heap partition pointed-to by a pseudo-field named
// getIndexField().
func getIndexField() Field {
	return Field{name: "AnyField"}
}

// A pseudo-field to denote the direct points-to relation. For example, r1 = &r0
// is modeled as r1[getDirectPointToField()] = r0.
func getDirectPointToField() Field {
	return Field{name: "->"}
}

// Constructs AbsReference for local "reg" in context "context".
// TODO: hashconst the object to save memory.
func getAbsReferenceForLocal(context *AbsReferenceContext, reg ssa.Value) AbsReference {
	return AbsReferenceLocal{context: context, reg: reg}
}

// Constructs AbsReference for "global".
func getAbsReferenceForGlobal(global *ssa.Global) AbsReference {
	return AbsReferenceGlobal{global: global}
}

// Constructs internal AbsReference.
func getAbsReferenceForInternal(kind uint8, ref AbsReference) AbsReference {
	return AbsReferenceInternal{kind: kind, ref: ref}
}

// Returns whether a value of this type may share an object pointed by other
// values. It is used for identifying copy-by-reference objects. For example, it
// returns false for any integer type, and returns true for pointer type and
// struct type.
func typeMayShareObject(tp types.Type) bool {
	switch tp.(type) {
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
	}
	return false
}

// emptyContext keeps a copy of empty context handle.
var emptyContext = make(AbsReferenceContext, 0)

func (ctx AbsReferenceContext) String() string {
	if len(ctx) == 0 {
		return ""
	}
	var s []string
	for _, ins := range ctx {
		s = append(s, ins.String())
	}
	return "[" + strings.Join(s, "; ") + "]"
}
