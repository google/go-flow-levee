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
	"go/token"
	"go/types"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/go-flow-levee/internal/pkg/config"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

// Analyzer traverses the packages and constructs an EAR partitions
// unifying all the IR elements in these packages.
var Analyzer = &analysis.Analyzer{
	Name:       "earpointer",
	Doc:        "EAR pointer analysis",
	Flags:      config.FlagSet,
	Run:        run,
	ResultType: reflect.TypeOf(new(Partitions)),
	Requires:   []*analysis.Analyzer{buildssa.Analyzer},
}

// visitor processes the instructions in a function and perform unifications
// for all reachable contexts for that function. Both the intra-procedural
// instructions and inter-procedural instructions (i.e. CallInstruction) are processed.
type visitor struct {
	state    *state     // mutable state
	contexts []*Context // for context sensitive analysis
}

func run(pass *analysis.Pass) (interface{}, error) {
	ssainput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	vis := visitor{state: NewState(), contexts: []*Context{&emptyContext}}
	vis.initReferences(ssainput)
	for _, fn := range ssainput.SrcFuncs {
		for _, blk := range fn.Blocks {
			for _, i := range blk.Instrs {
				vis.visitInstruction(i)
			}
		}
	}
	return vis.state.ToPartitions(), nil
}

// Insert into the state all the local references and global references.
func (vis *visitor) initReferences(ssainput *buildssa.SSA) {
	state := vis.state
	for _, member := range ssainput.Pkg.Members {
		if m, ok := member.(*ssa.Global); ok {
			if typeMayShareObject(m.Type()) &&
				!strings.HasPrefix(m.Name(), "init$") { // skip some synthetic variables
				state.Insert(MakeGlobal(m))
			}
		}
	}
	for _, fn := range ssainput.SrcFuncs {
		for _, param := range fn.Params {
			if typeMayShareObject(param.Type()) {
				state.Insert(MakeLocal(&emptyContext, param))
			}
		}
		for _, fv := range fn.FreeVars {
			if typeMayShareObject(fv.Type()) {
				state.Insert(MakeLocal(&emptyContext, fv))
			}
		}
		for _, blk := range fn.Blocks {
			for _, instr := range blk.Instrs {
				if v, ok := instr.(ssa.Value); ok && typeMayShareObject(v.Type()) {
					state.Insert(MakeLocal(&emptyContext, v))
				}
			}
		}
	}
}

func (vis *visitor) visitInstruction(instr ssa.Instruction) {
	switch i := instr.(type) {
	case *ssa.FieldAddr:
		vis.visitFieldAddr(i)
	case *ssa.IndexAddr:
		vis.visitIndexAddr(i)
	case *ssa.Field:
		vis.visitField(i)
	case *ssa.Index:
		vis.visitIndex(i)
	case *ssa.Phi:
		vis.visitPhi(i)
	case *ssa.Store:
		vis.visitStore(i)
	case *ssa.MapUpdate:
		vis.visitMapUpdate(i)
	case *ssa.Lookup:
		vis.visitLookup(i)
	case *ssa.Convert:
		vis.visitConvert(i)
	case *ssa.ChangeInterface:
		vis.visitChangeInterface(i)
	case *ssa.ChangeType:
		vis.visitChangeType(i)
	case *ssa.TypeAssert:
		vis.visitTypeAssert(i)
	case *ssa.UnOp:
		vis.visitUnOp(i)
	case *ssa.BinOp:
		vis.visitBinOp(i)
	case *ssa.Extract:
		vis.visitExtract(i)
	case *ssa.Send:
		vis.visitSend(i)
	case *ssa.Slice:
		vis.visitSlice(i)
	case *ssa.MakeInterface:
		vis.visitMakeInterface(i)
	case *ssa.Select:
		vis.visitSelect(i)

	case *ssa.Call, *ssa.Go, *ssa.Defer:
		vis.visitCall(i.(ssa.CallInstruction).Common(), instr)

	case *ssa.Alloc,
		*ssa.MakeChan,
		*ssa.MakeClosure,
		*ssa.MakeMap,
		*ssa.MakeSlice:
		// Local variables are created in the beginning of the pass by initReferences().
		// These instructions won't be handled after that.
	case *ssa.If,
		*ssa.Jump:
		// The analysis is flow insensitive.
	case *ssa.Range:
		// t1 = range t0: unify t0 and t1.
		// It will be processed by the subsequent extract instruction that also handles the relevant Next.
	case *ssa.Next:
		// The next instruction will be processed by the subsequent extract instruction.
	case *ssa.Return:
		// Return is handled in visitCall().
	case *ssa.RunDefers:
		// To be implemented.
	case *ssa.Panic:
		// To be implemented.
	case *ssa.DebugRef:
		// Not related.
	default:
		fmt.Printf("unsupported instruction: %s; please report this issue\n", i)
	}
}

func (vis *visitor) visitFieldAddr(addr *ssa.FieldAddr) {
	// "t1 = t0.x" is implemented by t0[x -> t1] through address unification.
	obj := addr.X
	field := makeField(obj.Type(), addr.Field)
	for _, c := range vis.contexts {
		vis.unifyFieldAddress(c, obj, field, addr)
	}
}

func (vis *visitor) visitField(field *ssa.Field) {
	// "t1 = t0.x" is implemented by t0[x -> t1].
	obj := field.X
	fd := makeField(obj.Type(), field.Field)
	vis.processHeapAccess(field, obj, *fd)
}

func (vis *visitor) visitIndexAddr(addr *ssa.IndexAddr) {
	// "t2 = &t1[t0]" is handled through address unification.
	vis.processIndex(addr, addr.X, addr.Index)
}

func (vis *visitor) visitIndex(index *ssa.Index) {
	vis.processIndex(index, index.X, index.Index)
}

func (vis *visitor) visitLookup(lookup *ssa.Lookup) {
	if !lookup.CommaOk {
		vis.processIndex(lookup, lookup.X, lookup.Index)
	}
	// The comma case will be processed by the subsequent extract instruction.
}

func (vis *visitor) visitStore(store *ssa.Store) {
	// Here store instruction "*address = data" is implemented by a
	// semantics-equivalent (w.r.t. unification) load instruction "data =
	// *address".
	// The address can be a (memory) address, a global variable, or a free
	// variable.
	if mayShareObject(store.Val) {
		vis.processAddressToValue(store.Addr, store.Val)
	}
}

func (vis *visitor) visitPhi(phi *ssa.Phi) {
	if !typeMayShareObject(phi.Type()) {
		return
	}
	for _, e := range phi.Edges {
		vis.unifyLocals(phi, e)
	}
}

func (vis *visitor) visitMapUpdate(update *ssa.MapUpdate) {
	vis.processIndex(update.Value, update.Map, update.Key)
}

func (vis *visitor) visitConvert(convert *ssa.Convert) {
	vis.unifyLocals(convert, convert.X)
}

func (vis *visitor) visitChangeInterface(change *ssa.ChangeInterface) {
	vis.unifyLocals(change, change.X)
}

func (vis *visitor) visitChangeType(change *ssa.ChangeType) {
	vis.unifyLocals(change, change.X)
}

func (vis *visitor) visitUnOp(op *ssa.UnOp) {
	switch op.Op {
	case token.ARROW: // channel read
		// Use array read to simulate channel read.
		vis.processHeapAccess(op, op.X, anyIndexField)
	case token.MUL:
		// v = *addr
		// The LHS is a register; the RHS can be a (memory) address, a global
		// variable, or a free variable.
		// t1 = *addr is implemented as "addr[directPointToField] = t1",
		// i.e., address addr points to value t1.
		vis.processAddressToValue(op.X, op)
	}
}

func (vis *visitor) visitBinOp(op *ssa.BinOp) {
	// Numeric binop will be skipped.
	// The only binop that matters now is "+" that concatenates strings.
	if op.Op == token.ADD {
		if tp, ok := op.X.Type().(*types.Basic); ok && tp.Kind() == types.String {
			vis.unifyLocals(op, op.X)
			vis.unifyLocals(op, op.Y)
		}
	}
}

func (vis *visitor) visitTypeAssert(assert *ssa.TypeAssert) {
	if !assert.CommaOk {
		// t1 = typeassert t0.(*int)
		vis.unifyLocals(assert, assert.X)
	}
	// The comma case will be processed by the subsequent extract instruction.
}

func (vis *visitor) visitExtract(extract *ssa.Extract) {
	switch base := extract.Tuple.(type) {
	case *ssa.TypeAssert:
		// t1 = typeassert,ok t0.(*int)
		// t2 = extract t1 0
		if base.CommaOk && extract.Index == 0 {
			vis.unifyLocals(extract, base.X)
		}
	case *ssa.Next:
		// t1 = range t0
		// t2 = next t1   // return (ok bool, k Key, v Value)
		// t3 = extract t2 ?
		if rng, ok := base.Iter.(*ssa.Range); ok {
			if mayShareObject(extract) && extract.Index == 2 { // extract the value v
				// The value from the map t0 flows into the field: t0[*] = v.
				vis.processHeapAccess(extract, rng.X, anyIndexField)
			}
		}
	case *ssa.Lookup:
		// t2 = t0[t1],ok
		// t3 = extract t2 ?
		if base.CommaOk && extract.Index == 0 {
			vis.processIndex(extract, base.X, base.Index)
		}
	default: // Other cases: model "r1 = extract r0 #n" as "r0["n"] = r1".
		field := Field{Name: strconv.Itoa(extract.Index)}
		vis.processHeapAccess(extract, base, field)
	}
}

func (vis *visitor) visitSlice(slice *ssa.Slice) {
	vis.unifyLocals(slice, slice.X)
}

func (vis *visitor) visitMakeInterface(makeInterface *ssa.MakeInterface) {
	vis.unifyLocals(makeInterface, makeInterface.X)
}

func (vis *visitor) visitSend(send *ssa.Send) {
	vis.processHeapAccess(send.X, send.Chan, anyIndexField)
}

func (vis *visitor) visitSelect(s *ssa.Select) {
	// TODO: to be implemented.
}

func (vis *visitor) visitCall(call *ssa.CallCommon, instr ssa.Instruction) {
	// TODO: to be added.
}

// Process a load/store using an index (which can be constant).
func (vis *visitor) processIndex(data ssa.Value, base ssa.Value, index ssa.Value) {
	// If the index is a constant, its string name is used as the field name;
	// otherwise the predefined field name "AnyField" is used.
	var field Field
	if c, ok := index.(*ssa.Const); ok {
		field = Field{Name: c.Value.String()}
	} else {
		field = anyIndexField
	}
	vis.processHeapAccess(data, base, field)
}

// Process a load/store from data_reg to (base, index), where base can be a register
// or global variable. Here heap access is through unify-by-reference simulation.
func (vis *visitor) processHeapAccess(data ssa.Value, base ssa.Value, fd Field) {
	// Skip unification if lhs is not a sharable type.
	if !typeMayShareObject(data.Type()) {
		return
	}
	// unify instance field
	for _, context := range vis.contexts {
		vis.unifyField(context, base, fd, data)
	}
}

// Process memory operator & and * by making the address pointing to the value.
func (vis *visitor) processAddressToValue(addr ssa.Value, value ssa.Value) {
	if !typeMayShareObject(addr.Type()) || !typeMayShareObject(value.Type()) {
		return
	}
	state := vis.state
	for _, context := range vis.contexts {
		caddr := state.Insert(MakeReference(context, addr))
		cvalue := state.Insert(MakeReference(context, value))
		fmap := state.PartitionFieldMap(state.representative(caddr))
		// Add or unify the field.
		if v, ok := fmap[directPointToField]; ok {
			if isUnifyByReference(value.Type()) { // unify-by-reference
				state.Unify(v, cvalue)
			} else {
				vis.unifyByValue(v, cvalue)
			}
		} else {
			fmap[directPointToField] = cvalue
		}
	}
}

// Unify local regs "v1" and "v2" in all contexts.
func (vis *visitor) unifyLocals(v1 ssa.Value, v2 ssa.Value) {
	if !mayShareObject(v1) || !mayShareObject(v2) {
		return
	}
	state := vis.state
	for _, context := range vis.contexts {
		state.Unify(MakeReference(context, v1), MakeReference(context, v2))
	}
}

// Unify "obj.field" and "target" in "context".
func (vis *visitor) unifyField(context *Context, obj ssa.Value, fd Field, target ssa.Value) {
	if !typeMayShareObject(target.Type()) {
		return
	}
	// This generates the unification constraint obj.field = target
	state := vis.state
	objr := state.representative(MakeReference(context, obj))
	fmap := state.PartitionFieldMap(objr)
	if fmap == nil { // Should not happen
		fmt.Printf("unexpected nil field map: %s; please report this issue\n", objr)
		return
	}
	tr := state.Insert(MakeReference(context, target))
	// Add mapping from field to target. If obj's partition already has a mapping for
	// this field, unify target with the suitable partition,
	if isUnifyByReference(tr.Type()) { // unify the two references
		// Add or unify the field.
		if v, ok := fmap[fd]; ok {
			state.Unify(v, tr)
		} else {
			fmap[fd] = tr
		}
	} else { // unify by value
		if v, ok := fmap[fd]; ok {
			vis.unifyByValue(v, tr)
		}
	}
}

// Unify "&obj.field" and "target" in "context".
// For example, for "t1 = &t0.x", after the unification, t0 has a field x pointing to t1.
func (vis *visitor) unifyFieldAddress(context *Context, obj ssa.Value, field *Field, target *ssa.FieldAddr) {
	// This generates the unification constraint &obj.field = target
	state := vis.state
	objv := vis.getPointee(MakeReference(context, obj))
	fmap := state.PartitionFieldMap(state.representative(objv))
	if fmap == nil { // Should not happen
		fmt.Printf("unexpected nil field map: %s; please report this issue\n", objv)
		return
	}
	tv := MakeReference(context, target)
	// If obj's partition already has a mapping for this field, unify v with the
	// suitable partition. Otherwise set v as the ref.
	if v, ok := fmap[*field]; ok {
		state.Unify(v, tv)
	} else {
		fmap[*field] = tv
	}
}

// Return the reference holding the value *r of an address r (which can be
// either a Local or a Global). Create the value reference if
// it doesn't exist. In the heap, the address points to the value, i.e., r --> *r.
func (vis *visitor) getPointee(addr Reference) Reference {
	state := vis.state
	arep := state.representative(addr)
	// if ref is an address pointing to a value, return the value instead.
	pval := state.valueReferenceOrNil(arep)
	if pval != nil {
		return pval
	}
	// The value doesn't exist; synthesize a reference for it.
	val := MakeSynthetic(SyntheticValueOf, arep)
	state.Insert(val)
	state.PartitionFieldMap(arep)[directPointToField] = val
	return val
}

// Unify a value with another value pointed by an address.
// For example, unifying value "v2" and address "addr --> v1"
// results in "addr -> {v1, v2}".
func (vis *visitor) unifyFieldValueToAddress(toAddr Reference, fromVal Reference) {
	state := vis.state
	toFmap := state.PartitionFieldMap(toAddr)
	if toFmap == nil { // Should not happen
		fmt.Printf("unexpected nil field map: %s; please report this issue\n", toAddr)
		return
	}
	if toVal, ok := toFmap[directPointToField]; ok {
		state.Unify(toVal, fromVal)
	} else {
		toFmap[directPointToField] = fromVal
	}
}

// Unify two field references w.r.t. the unify-by-value semantics, and return
// the remaining references to be further unified-by-value.
//
// Distinguish addressable fields from non-addressable fields:
// (1) for the non-addressable case such that r1 and r2, unify r1 and r2 if
// they are references, otherwise return {r1, r2};
// (2) for the addressable case such as &r1 --> *r1 and &r2 --> *r2,
// keep &r1 and &r2 intact, but unify *r1 and *r2 if they are references,
// otherwise return {*r1, *r2}.
func (vis *visitor) unifyFields(ref1 Reference, ref2 Reference, addressable bool) (Reference, Reference) {
	// A function to return a pair of references to be unified.
	toBeUnified := func(r1 Reference, r2 Reference) (Reference, Reference) {
		if typeMayShareObject(r1.Type()) && typeMayShareObject(r2.Type()) {
			return r1, r2
		}
		return nil, nil
	}

	state := vis.state
	if !addressable {
		// TODO: check whether this is possible,
		//  i.e. non-addressable fields (e.g. map elements) may have been unified before reaching here.
		// unify directly for non-addressable fields.
		if isUnifyByReference(ref1.Type()) { // unify-by-reference
			state.Unify(ref1, ref2)
			return nil, nil
		} else { // unify-by-value
			return toBeUnified(ref1, ref2)
		}
	}
	rep1, rep2 := state.representative(ref1), state.representative(ref2)
	// The addressable case: &r1 -> *r1 and &r2 -> *r2, unify *r1 and *r2.
	if val2 := state.valueReferenceOrNil(rep2); val2 != nil {
		if isUnifyByReference(val2.Type()) {
			vis.unifyFieldValueToAddress(rep1, val2)
		} else { // unify-by-value
			return toBeUnified(val2, rep1)
		}
	} else if val1 := state.valueReferenceOrNil(rep1); val1 != nil {
		if isUnifyByReference(val1.Type()) {
			vis.unifyFieldValueToAddress(rep2, val1)
		} else { // unify-by-value
			return toBeUnified(val1, rep2)
		}
	}
	return nil, nil // no further unify-by-value
}

// For non-reference types, unify-by-value semantics is used such that reference
// r1 and r2 are not unified, but their sub-fields of reference types are
// unified recursively.
//
// Example:
//   type S struct {x *int, y int, z S};
//   var v2 S = ...;
//   v1 := v2;
// After the unification, the partitions are:
// { v1.x, v2.x }, {v1.z.x, v2.z.x }, ...
func (vis *visitor) unifyByValue(ref1 Reference, ref2 Reference) {
	// A field may be addressable or non-addressable:
	// (1) for the non-addressable case such that r1 -> r1.x and r2 -> r2.x, unify
	// r1.x and r2.x;
	// (2) for the addressable case such as r1 -> &r1.x -> *r1.x and r2 -> &r2.x
	// -> *r2.x, unify *r1.x and *r2.x, and keep &r1.x and &r2.x intact.
	state := vis.state
	rep1, rep2 := state.representative(ref1), state.representative(ref2)
	fmap1, fmap2 := state.PartitionFieldMap(rep1), state.PartitionFieldMap(rep2)
	if fmap1 == nil || fmap2 == nil { // Should not happen
		fmt.Printf("unexpected nil field map: %s or %s; please report this issue\n", rep1, rep2)
		return
	}
	if &fmap1 == &fmap2 { // already unified
		return
	}
	// The to-be-unified field pairs.
	toUnified := make(map[Reference]Reference)
	// "Copy" the fields from ref2 to ref1: copy only the field values rather than
	// the field addresses.
	for k, addr2 := range fmap2 {
		var addr1 Reference
		if fd, ok := fmap1[k]; ok {
			addr1 = fd
		} else {
			addr1 = state.Insert(MakeSynthetic(SyntheticField, rep1))
			fmap1[k] = addr1
		}
		toUnified[addr1] = addr2
		// TODO: Recursively call "UnifyByValue" here.
	}
	// "Copy" the remaining fields in ref1 to ref2. Only copy unify-by-reference
	// field values.
	for k, addr1 := range fmap1 {
		if _, ok := fmap2[k]; ok {
			continue
		}
		addr2 := state.Insert(MakeSynthetic(SyntheticField, rep2))
		fmap2[k] = addr2
		toUnified[addr1] = addr2
	}
	// Perform further unifications.
	addressable := isFieldAddressable(rep2.Type())
	for addr1, addr2 := range toUnified {
		vis.unifyFields(addr1, addr2, addressable)
	}
}

// Return whether a type can use the unify-by-reference semantics. If not then
// it is assumed to be unifyByValue.
func isUnifyByReference(tp types.Type) bool {
	switch t := tp.(type) {
	case *types.Struct, *types.Array:
		return false
	case *types.Named:
		return isUnifyByReference(t.Underlying())
	}
	// "MayShareObject" determines whether the value may share an object
	// pointed by other values. It includes structs and arrays, whose elements may be shared.
	return typeMayShareObject(tp)
}

// Return whether the fields of a type can be accessed through operator &
// (https://golang.org/ref/spec#Address_operators).
// The Go frontend and SSA handle an addressable field by introducing an
// intermediate register for the address. For example, for struct A, "A.x = 1"
// is compiled into "t1 = &A.x; *t1 = 1".
func isFieldAddressable(tp types.Type) bool {
	switch t := tp.(type) {
	case *types.Struct, *types.Slice, *types.Array:
		return true
	case *types.Named:
		return isFieldAddressable(t.Underlying())
	}
	return false
}

// Return whether a value is a local variable  such as a paramter or a register instruction.
func isLocal(v ssa.Value) bool {
	switch v.(type) {
	case *ssa.Parameter, *ssa.FreeVar, ssa.Instruction:
		return true
	}
	return false
}

func isGlobal(v ssa.Value) bool {
	_, ok := v.(*ssa.Global)
	return ok
}

// Return whether the value is a local variable or a global variable,
// and its type allows object sharing.
func mayShareObject(v ssa.Value) bool {
	return typeMayShareObject(v.Type()) && (isLocal(v) || isGlobal(v))
}

// Construct a field from a type and a field index.
// For example, for struct T {x int, y int), makeField(*T, 1) returns Field{Name:"y", IrField:y}.
func makeField(tp types.Type, index int) *Field {
	if pt, ok := tp.(*types.Pointer); ok {
		tp = pt.Elem()
	}
	if named, ok := tp.(*types.Named); ok {
		tp = named.Underlying()
	}
	if stp, ok := tp.(*types.Struct); ok {
		fvar := stp.Field(index)
		return &Field{Name: fvar.Name(), irField: fvar}
	}
	return &Field{Name: fmt.Sprintf("%d", index)}
}
