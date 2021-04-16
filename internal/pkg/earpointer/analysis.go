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
	"log"
	"reflect"
	"strconv"

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

// transformer processes the instructions in a function and perform unifications
// for all reachable contexts for that function. Both the intra-procedural
// instructions and inter-procedural instructions (i.e. CallInstruction) are processed.
type transformer struct {
	state    *state // mutable state
	contexts []*Context
}

func run(pass *analysis.Pass) (interface{}, error) {
	ssainput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	p := transformer{state: NewState(), contexts: []*Context{&emptyContext}}
	initReferences(ssainput, p.state)
	for _, fn := range ssainput.SrcFuncs {
		for _, blk := range fn.Blocks {
			for _, instr := range blk.Instrs {
				p.visitInstruction(instr)
			}
		}
	}
	return p.state.ToPartitions(), nil
}

// Insert into the state all the local references and global references.
func initReferences(ssainput *buildssa.SSA, state *state) {
	for _, member := range ssainput.Pkg.Members {
		if m, ok := member.(*ssa.Global); ok {
			if typeMayShareObject(m.Type()) && m.Name() != "init$guard" {
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

func (p *transformer) visitInstruction(instr ssa.Instruction) {
	switch i := instr.(type) {
	case *ssa.FieldAddr:
		p.visitFieldAddr(i)
	case *ssa.IndexAddr:
		p.visitIndexAddr(i)
	case *ssa.Field:
		p.visitField(i)
	case *ssa.Index:
		p.visitIndex(i)
	case *ssa.Phi:
		p.visitPhi(i)
	case *ssa.Store:
		p.visitStore(i)
	case *ssa.MapUpdate:
		p.visitMapUpdate(i)
	case *ssa.Lookup:
		p.visitLookup(i)
	case *ssa.Convert:
		p.visitConvert(i)
	case *ssa.TypeAssert:
		p.visitTypeAssert(i)
	case *ssa.UnOp:
		p.visitUnOp(i)
	case *ssa.BinOp:
		p.visitBinOp(i)
	case *ssa.Extract:
		p.visitExtract(i)
	case *ssa.Range:
		p.visitRange(i)
	case *ssa.Send:
		p.visitSend(i)
	case *ssa.Slice:
		p.visitSlice(i)
	case *ssa.MakeInterface:
		p.visitMakeInterface(i)

	case *ssa.Call:
		p.visitCall(i.Call, instr)
	case *ssa.Go:
		p.visitCall(i.Call, instr)
	case *ssa.Defer:
		p.visitCall(i.Call, instr)
	}
}

func (p *transformer) visitFieldAddr(addr *ssa.FieldAddr) {
	// "t1 = t0.x" is implemented by t0[x -> t1].
	field := &Field{Name: getFieldName(addr.X.Type(), addr.Field)}
	// "t1 = t0.x" is handled through address unification.
	if !typeMayShareObject(addr.Type()) {
		return
	}
	// unify instance field
	for _, c := range p.contexts {
		p.unifyFieldAddress(c, addr.X, field, addr)
	}
}

func (p *transformer) visitField(field *ssa.Field) {
	// "t1 = t0.x" is implemented by t0[x -> t1].
	fd := Field{Name: getFieldName(field.X.Type(), field.Field)}
	p.processHeapAccess(field, field.X, fd /*unifyByValue=*/, false)
}

func (p *transformer) visitIndexAddr(addr *ssa.IndexAddr) {
	// "t2 = &t1[t0]" is handled through address unification.
	p.processMemoryIndexAccess(addr, addr.X, addr.Index /*unifyByValue=*/, false)
}

func (p *transformer) visitIndex(index *ssa.Index) {
	p.processMemoryIndexAccess(index, index.X, index.Index /*unifyByValue=*/, false)
}

func (p *transformer) visitLookup(lookup *ssa.Lookup) {
	if !lookup.CommaOk {
		p.processMemoryIndexAccess(lookup, lookup.X, lookup.Index,
			/*unify_by_value=*/ true)
	}
	// The comma case will be processed by the subsequent extract instruction.
}

func (p *transformer) visitStore(store *ssa.Store) {
	// The Address can be a (memory) address, a global variable, or a free
	// variable.
	// Here store instruction "*address = data" is implemented by a
	// semantics-equivalent (w.r.t. unification) load instruction "data =
	// *address".
	if mayShareObject(store.Val) {
		p.processAddressToValue(store.Addr, store.Val /*unifyByValue=*/, false)
	}
}

func (p *transformer) visitPhi(phi *ssa.Phi) {
	if !typeMayShareObject(phi.Type()) {
		return
	}
	for _, e := range phi.Edges {
		p.unifyLocals(phi, e)
	}
}

func (p *transformer) visitMapUpdate(update *ssa.MapUpdate) {
	p.processMemoryIndexAccess(update.Value, update.Map,
		update.Key /*unifyByValue=*/, false)
}

func (p *transformer) visitConvert(convert *ssa.Convert) {
	p.unifyLocals(convert, convert.X)
}

func (p *transformer) visitUnOp(op *ssa.UnOp) {
	switch op.Op {
	case token.ARROW: // channel read
		// Use array read to simulate channel read.
		p.processHeapAccess(op, op.X, anyIndexField /*unifyByValue=*/, false)
	case token.MUL:
		{ // v = *addr
			// The LHS is a register; the RHS can be a (memory) address, a global
			// variable, or a free variable.
			// t1 = *addr is implemented as "addr[directPointToField] = t1",
			// i.e., address addr points to value t1.
			p.processAddressToValue(op.X, op /*unifyByValue=*/, false)
		}
	}
}

func (p *transformer) visitBinOp(op *ssa.BinOp) {
	p.unifyLocals(op, op.X)
	p.unifyLocals(op, op.Y)
}

func (p *transformer) visitTypeAssert(assert *ssa.TypeAssert) {
	if !assert.CommaOk {
		// t1 = typeassert t0.(*int)
		p.unifyLocals(assert, assert.X)
	}
	// The comma case will be processed by the subsequent extract instruction.
}

func (p *transformer) visitExtract(extract *ssa.Extract) {
	switch base := extract.Tuple.(type) {
	case *ssa.TypeAssert:
		// t1 = typeassert,ok t0.(*int)
		// t2 = extract t1 0
		if base.CommaOk && extract.Index == 0 {
			p.unifyLocals(extract, base.X)
		}
	case *ssa.Next:
		// t1 = range t0
		// t2 = next t1   // return (ok bool, k Key, v Value)
		// t3 = extract t2 ?
		if rng, ok := base.Iter.(*ssa.Range); ok {
			if mayShareObject(extract) && extract.Index == 2 { // extract the value v
				// The value from the map flows into the field: m[*] = v.
				p.processHeapAccess(extract, rng.X, anyIndexField,
					/*unify_by_value=*/ false)
			}
		}
	case *ssa.Lookup:
		// t2 = t0[t1],ok
		// t3 = extract t2 ?
		if base.CommaOk && extract.Index == 0 {
			p.processHeapAccess(extract, base.X, Field{Name: "0"},
				/*unify_by_value=*/ false)
		}
	default: // Other cases: model "r1 = extract r0 #n" as "r0["n"] = r1".
		field := Field{Name: strconv.Itoa(extract.Index)}
		p.processHeapAccess(extract, base, field,
			/*unify_by_value=*/ false)
	}
}

func (p *transformer) visitNext(next *ssa.Next) {
	// The next instruction will be processed by the subsequent extract
	// instruction.
}

func (p *transformer) visitSlice(slice *ssa.Slice) {
	p.unifyLocals(slice, slice.X)
}

func (p *transformer) visitMakeInterface(makeInterface *ssa.MakeInterface) {
	if mayShareObject(makeInterface.X) {
		p.unifyLocals(makeInterface, makeInterface.X)
	}
}

func (p *transformer) visitSend(send *ssa.Send) {
	p.processHeapAccess(send.X, send.Chan,
		anyIndexField /*unify_by_value=*/, false)
}

func (p *transformer) visitRange(rng *ssa.Range) {
	// t1 = range t0: unify t0 and t1.
	p.unifyLocals(rng, rng.X)
}

func (p *transformer) VisitSelect() {
	// TODO: to be implemented.
}

func (p *transformer) visitCall(call ssa.CallCommon, instr ssa.Instruction) {
	// TODO: to be added.
}

// Process a load/store using an index (which can be constant).
func (p *transformer) processMemoryIndexAccess(data ssa.Value, base ssa.Value, index ssa.Value, unifyByValue bool) {
	// If the index is a constant, its string name is used as the field name;
	// otherwise the predefined field name is used.
	var field Field
	if c, ok := index.(*ssa.Const); ok {
		field = Field{Name: c.Name()}
	} else {
		field = anyIndexField
	}
	p.processHeapAccess(data, base, field, unifyByValue)
}

// Process a load/store from data_reg to (base, index), where base can be a register
// or global variable. Here heap access can be through unify-by-reference or
// unifyByValue simulation.
func (p *transformer) processHeapAccess(data ssa.Value, base ssa.Value, fd Field, unifyByValue bool) {
	// Skip unification if lhs is not a sharable type.
	if !mayShareObject(data) {
		return
	}
	// unify instance field
	for _, context := range p.contexts {
		p.unifyField(context, base, fd, data, unifyByValue)
	}
}

// Process memory operator & and * by making the address pointing to the value.
func (p *transformer) processAddressToValue(addr ssa.Value,
	value ssa.Value, unifyByValue bool) {
	if !typeMayShareObject(addr.Type()) || !typeMayShareObject(value.Type()) {
		return
	}
	state := p.state
	for _, context := range p.contexts {
		caddr := state.Insert(MakeReference(context, addr))
		cvalue := state.Insert(MakeReference(context, value))
		fmap := state.PartitionFieldMap(state.representative(caddr))
		field := directPointToField
		// Add or unify the field.
		if v, ok := fmap[field]; ok {
			if (!unifyByValue) ||
				isUnifyByReference(value.Type()) { // unify-by-reference
				p.unify(v, cvalue)
			} else {
				p.unifyByValue(cvalue, v)
			}
		} else {
			fmap[field] = cvalue
		}
	}
}

// Unify local regs "v1" and "v2" in all contexts.
func (p *transformer) unifyLocals(v1 ssa.Value, v2 ssa.Value) {
	if !mayShareObject(v1) || !mayShareObject(v2) {
		return
	}
	for _, context := range p.contexts {
		p.unify(MakeReference(context, v1), MakeReference(context, v2))
	}
}

// Unify "v1.field" and "v2" in "context".
func (p *transformer) unifyField(context *Context, v1 ssa.Value, fd Field, v2 ssa.Value, unifyByValue bool) {
	// This generates the unification constraint v1.f = v2
	if !typeMayShareObject(v2.Type()) {
		return
	}
	state := p.state
	cv1 := MakeReference(context, v1)
	fmap := state.PartitionFieldMap(state.representative(cv1))
	if fmap == nil {
		log.Fatal("field map is nil")
	}
	// if v1 is an address pointing to a value, use the value instead.
	if v, ok := fmap[directPointToField]; ok {
		cv1 = v
	}
	cv2 := state.Insert(MakeReference(context, v2))
	// Add mapping from field to v2. If v1's partition already has a mapping for
	// this field, unify v2 with the suitable partition,
	if !unifyByValue || isUnifyByReference(cv2.Type()) { // unify the two references
		// Add or unify the field.
		if v, ok := fmap[fd]; ok {
			state.Unify(v, cv2)
		} else {
			fmap[fd] = cv2
		}
	} else { // unify by value
		if v, ok := fmap[fd]; ok {
			p.unifyByValue(v, cv2)
		}
	}
}

// Unify "&v1.field" and "v2" in "context".
func (p *transformer) unifyFieldAddress(context *Context, v1 ssa.Value, field *Field, v2 ssa.Value) {
	// This generates the unification constraint &v1.f = v2
	if !typeMayShareObject(v1.Type()) {
		return
	}
	state := p.state
	addr := p.state.Insert(MakeReference(context, v1))
	cv1 := p.getValueReference(addr)
	fmap := state.PartitionFieldMap(state.representative(cv1))
	if fmap == nil {
		log.Fatalf("reference [%s] has no field map", cv1)
	}
	cv2 := state.Insert(MakeReference(context, v2))
	// If v1's partition already has a mapping for this field, unify v2 with the
	// suitable partition. Otherwise set v2 as the ref.
	if v, ok := fmap[*field]; ok {
		state.Unify(v, cv2)
	} else {
		fmap[*field] = cv2
	}
}

// Return the reference holding the value *r of an address r (which can be
// either a Register or a GlobalVariable). Create the value reference if
// it doesn't exist. In the heap, the address points to the value, i.e., r --> *r.
func (p *transformer) getValueReference(addr Reference) Reference {
	state := p.state
	arep := state.representative(addr)
	// if ref is an address pointing to a value, return the value instead.
	pval := state.valueReferenceOrNil(arep)
	if pval != nil {
		return pval
	}
	// The value doesn't exist.
	val := MakeSynthetic(0 /*VALUEOF*/, arep)
	state.Insert(val)
	fmap := state.PartitionFieldMap(arep)
	fmap[directPointToField] = val
	return val
}

// Unify two abstract references "ref1" and "ref2".
func (p *transformer) unify(ref1 Reference, ref2 Reference) {
	state := p.state
	state.unifyReps(state.Insert(ref1), state.Insert(ref2))
}

// Unify a value with another value pointed by an address.
// For example, unifying value v2 and address addr --> v1 results in addr ->
// {v1, v2}.
func (p *transformer) unifyFieldValueToAddress(toAddr Reference, fromVal Reference) {
	state := p.state
	toFmap := state.PartitionFieldMap(toAddr)
	if toFmap == nil {
		log.Fatal("field maps are nil")
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
func (p *transformer) unifyFields(ref1 Reference, ref2 Reference, addressable bool) (Reference, Reference) {
	// A function to return a pair of references to be unified.
	toBeUnified := func(r1 Reference, r2 Reference) (Reference, Reference) {
		if typeMayShareObject(r1.Type()) && typeMayShareObject(r2.Type()) {
			return r1, r2
		}
		return nil, nil
	}

	state := p.state
	if !addressable {
		// unify directly for non-addressable fields.
		if isUnifyByReference(ref1.Type()) { // unify-by-reference
			state.Unify(ref1, ref2)
		} else { // unify-by-value
			return toBeUnified(ref1, ref2)
		}
	}
	rep1, rep2 := state.representative(ref1), state.representative(ref2)
	// The addressable case: &r1 -> *r1 and &r2 -> *r2, unify *r1 and *r2.
	if val2 := state.valueReferenceOrNil(rep2); val2 != nil {
		if isUnifyByReference(val2.Type()) {
			p.unifyFieldValueToAddress(rep1, val2)
		} else { // unify-by-value
			return toBeUnified(val2, rep1)
		}
	} else if val1 := state.valueReferenceOrNil(rep1); val1 != nil {
		if isUnifyByReference(val1.Type()) {
			p.unifyFieldValueToAddress(rep2, val1)
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
func (p *transformer) unifyByValue(ref1 Reference, ref2 Reference) {
	// A field may be addressable or non-addressable:
	// (1) for the non-addressable case such that r1 -> r1.x and r2 -> r2.x, unify
	// r1.x and r2.x;
	// (2) for the addressable case such as r1 -> &r1.x -> *r1.x and r2 -> &r2.x
	// -> *r2.x, unify *r1.x and *r2.x, and keep &r1.x and &r2.x intact.
	state := p.state
	rep1, rep2 := state.representative(ref1), state.representative(ref2)
	fmap1, fmap2 := state.PartitionFieldMap(rep1), state.PartitionFieldMap(rep2)
	if fmap1 == nil || fmap2 == nil {
		log.Fatal("field maps are nil")
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
			addr1 = state.Insert(MakeSynthetic(1 /*FIELD*/, rep1))
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
		addr2 := state.Insert(MakeSynthetic(1 /*FIELD*/, rep2))
		fmap2[k] = addr2
		toUnified[addr1] = addr2
	}
	// Perform further unifications.
	addressable := isFieldAddressable(rep2.Type())
	for addr1, addr2 := range toUnified {
		p.unifyFields(addr1, addr2, addressable)
	}
}

// Return whether a type can use the unify-by-reference semantics. If not then
// it is assumed to be unifyByValue.
func isUnifyByReference(tp types.Type) bool {
	switch t := tp.(type) {
	case *types.Struct,
		*types.Array:
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
	case *types.Struct,
		*types.Slice,
		*types.Array:
		return true
	case *types.Named:
		return isFieldAddressable(t.Underlying())
	}
	return false
}

// Return the name of a field in a struct.
// It the field is not found, then it returns the string format of the index.
//
// For example, for struct T {x int, y int), getFieldName(*T, 1) returns "y".
func getFieldName(tp types.Type, index int) string {
	if pt, ok := tp.(*types.Pointer); ok {
		tp = pt.Elem()
	}
	if named, ok := tp.(*types.Named); ok {
		tp = named.Underlying()
	}
	if stp, ok := tp.(*types.Struct); ok {
		return stp.Field(index).Name()
	}
	return fmt.Sprintf("%d", index)
}

// Return whether a value is a local variable  such as a paramter or a register instruction.
func isLocal(v ssa.Value) bool {
	switch l := v.(type) {
	case *ssa.Parameter,
		*ssa.FreeVar:
		return true
	case ssa.Instruction:
		// Consider only register instruction.
		_, ok := l.(ssa.Value)
		return ok
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
