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
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/static"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

var (
	contextK = 0 // the number of call sites in each context
)

func init() {
	Analyzer.Flags.IntVar(&contextK, "contextK", 0,
		`the K value (default=0) in context sensitivity.`)
}

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

// visitor traverse the instructions in a function and perform unifications
// for all reachable contexts for that function. Both the intra-procedural
// instructions and inter-procedural instructions are handled.
type visitor struct {
	state    *state                              // mutable state
	callees  map[*ssa.CallCommon][]*ssa.Function // callee functions at each callsite
	contexts map[*ssa.Function][]*Context        // for context sensitive analysis
	contextK int
}

func run(pass *analysis.Pass) (interface{}, error) {
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}
	if !conf.UseEAR {
		return &Partitions{}, nil
	}
	ssainput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	p := analyze(ssainput)
	return p, nil
}

// Analyzes an SSA program and build the partition information.
func analyze(ssainput *buildssa.SSA) *Partitions {
	prog := ssainput.Pkg.Prog
	// Use the call graph to initialize the contexts.
	// TODO: the call graph can be CHA, RTA, VTA, etc.
	cg := static.CallGraph(prog)
	vis := visitor{state: NewState(), callees: mapCallees(cg)}
	vis.initContexts(cg)
	vis.initGlobalReferences(ssainput.Pkg)
	// Analyze all the functions and methods in the package,
	// not just those in ssainput.SrcFuncs.
	fns := ssautil.AllFunctions(prog)
	for _, fn := range ssainput.SrcFuncs {
		fns[fn] = true
	}
	for fn := range fns {
		vis.initFunction(fn)
	}
	for fn := range fns {
		for _, blk := range fn.Blocks {
			for _, i := range blk.Instrs {
				vis.visitInstruction(i)
			}
		}
	}
	p := vis.state.ToPartitions()
	p.cg = cg
	return p
}

// Builds the calling context set for each function.
func (vis *visitor) initContexts(cg *callgraph.Graph) {
	vis.contextK = contextK
	vis.contexts = make(map[*ssa.Function][]*Context)
	for fn, node := range cg.Nodes {
		if fn == nil {
			continue
		}
		// Create the contexts for this function.
		vis.contexts[fn] = collectContext(node, vis.contextK)
	}
}

// Collect the contexts from the node backward up to k edges.
func collectContext(node *callgraph.Node, k int) []*Context {
	// This implementation can be optimized in two ways:
	// (1) Reuse the k-1, k-2, ... contexts when calculating k so as to
	//     avoid duplicate computations.
	// (2) Make the contexts share memory since they may overlap a lot.
	// However, we may use only small k values so the performance
	// and memory consumption issue is not a concern in practice.
	if k <= 0 || len(node.In) == 0 {
		return []*Context{&emptyContext}
	}
	var kContexts []*Context
	for _, in := range node.In {
		for _, prev := range collectContext(in.Caller, k-1) {
			cur := append(*prev, in.Site)
			kContexts = append(kContexts, &cur)
		}
	}
	return kContexts
}

// Insert into the state all the global references.
func (vis *visitor) initGlobalReferences(pkg *ssa.Package) {
	state := vis.state
	for _, member := range pkg.Members {
		if g, ok := member.(*ssa.Global); ok {
			if typeMayShareObject(g.Type()) &&
				// skip some synthetic variables
				!strings.HasPrefix(g.Name(), "init$") {
				state.Insert(MakeGlobal(g))
			}
		}
		// Global constants are not within the scope of the pointer analysis.
	}
}

// Insert into the state all the local references.
func (vis *visitor) initFunction(fn *ssa.Function) {
	state := vis.state
	// A function to insert a local into the state under all related contexts.
	initLocal := func(v ssa.Value) {
		for _, c := range vis.getContexts(v) {
			state.Insert(MakeLocal(c, v))
		}
	}
	for _, param := range fn.Params {
		if typeMayShareObject(param.Type()) {
			initLocal(param)
		}
	}
	for _, fv := range fn.FreeVars {
		if typeMayShareObject(fv.Type()) {
			initLocal(fv)
		}
	}
	// Usual tuple-type registers are skipped since they will be processed
	// by subsequent Extract instructions, but function return is a special
	// case where the return tuple is needed.
	returnTuple := func(v ssa.Value) bool {
		if _, ok := v.(ssa.CallInstruction); ok {
			// In SSA, a call's return is always of a tuple type.
			// If the call has no return, then the tuple is empty.
			if tuple, ok := v.Type().(*types.Tuple); ok {
				return tuple.Len() > 1
			}
		}
		return false
	}
	for _, blk := range fn.Blocks {
		for _, instr := range blk.Instrs {
			if v, ok := instr.(ssa.Value); ok {
				if returnTuple(v) || typeMayShareObject(v.Type()) {
					initLocal(v)
				}
			}
		}
	}
	// Recursively process the embedded functions
	for _, anon := range fn.AnonFuncs {
		vis.initFunction(anon)
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
	case *ssa.MakeClosure:
		vis.visitMakeClosure(i)
	case *ssa.Select:
		vis.visitSelect(i)

	case *ssa.Call, *ssa.Go, *ssa.Defer:
		vis.visitCall(i.(ssa.CallInstruction).Common(), instr)

	case *ssa.Alloc,
		*ssa.MakeChan,
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
	for _, c := range vis.getContexts(addr) {
		vis.unifyFieldAddress(c, obj, field, addr)
	}
}

func (vis *visitor) visitField(field *ssa.Field) {
	// "t1 = t0.x" is implemented by t0[x -> addr], addr --> t1,
	// where addr is the reference to the field address.
	obj := field.X
	fd := makeField(obj.Type(), field.Field)
	state := vis.state
	for _, c := range vis.getContexts(field) {
		objRef := MakeReference(c, obj)
		fmap := state.PartitionFieldMap(state.representative(objRef))
		fref := MakeReference(c, field)
		// If obj's partition already has a mapping for this field, unify fref
		// with the suitable partition. Otherwise set fref as the ref.
		if oldRef, ok := fmap[*fd]; ok {
			// Implement "fmap[fd -> addr], addr --> fref".
			state.Unify(vis.getPointee(oldRef), fref)
		} else {
			fmap[*fd] = fref // over-approximation
		}
	}
}

func (vis *visitor) visitIndexAddr(addr *ssa.IndexAddr) {
	// "t2 = &t1[t0]" is handled through address unification.
	obj := addr.X
	if c, ok := addr.Index.(*ssa.Const); ok {
		field := Field{Name: c.Value.String()}
		for _, cxt := range vis.getContexts(addr) {
			vis.unifyFieldAddress(cxt, obj, &field, addr)
		}
	}
	// Also update the AnyField information.
	// Note that base[c -> data] implies base[AnyField -> data]
	for _, c := range vis.getContexts(addr) {
		vis.unifyFieldAddress(c, obj, &anyIndexField, addr)
	}
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
	if op.Op != token.ADD {
		return
	}
	// for r0 = r1 + r2, unifying r0, r1 and r2 may cause too many
	// over-approximations. Here fields are introduced to make:
	//     {r0}[0->r1, 1->r2]
	// which avoids unifying r1 and r2.
	if tp, ok := op.X.Type().(*types.Basic); ok && tp.Kind() == types.String {
		// unify instance field
		for _, c := range vis.getContexts(op) {
			if mayShareObject(op.X) {
				vis.unifyField(c, op, Field{Name: "left"}, op.X)
			}
			if mayShareObject(op.Y) {
				vis.unifyField(c, op, Field{Name: "right"}, op.Y)
			}
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
	case *ssa.UnOp:
		// Handle the channel read case with comma.
		// Other UnOp cases are handled within "visitUnOp()".
		// t1 = <-t0,ok
		// t2 = extract t1 0
		if base.Op == token.ARROW && base.CommaOk && extract.Index == 0 { // channel read
			// Use array read to simulate channel read.
			vis.processHeapAccess(extract, base.X, anyIndexField)
		}
	case *ssa.Select:
		// t1 = select blocking [c1<-t0, <-c2]  // (index int, recvOk bool, r_0 T_0, ... r_n-1 T_n-1)
		// t2 = extract t1 #n
		if extract.Index <= 1 {
			break
		}
		k := -1 // for locating the (extract.Index-2)^{th} receive
		for _, st := range base.States {
			if st.Dir == types.RecvOnly {
				if k++; k == extract.Index-2 && mayShareObject(st.Chan) {
					// Extract the receive value from the channel.
					vis.processHeapAccess(extract, st.Chan, anyIndexField)
					break
				}
			}
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

func (vis *visitor) visitMakeClosure(closure *ssa.MakeClosure) {
	if fn, ok := closure.Fn.(*ssa.Function); ok {
		// For a closure, add for free variable bindings.
		for i, bind := range closure.Bindings {
			if mayShareObject(bind) {
				fv := fn.FreeVars[i]
				vis.unifyLocals(fv, bind)
			}
		}
	}
}

func (vis *visitor) visitSend(send *ssa.Send) {
	vis.processHeapAccess(send.X, send.Chan, anyIndexField)
}

func (vis *visitor) visitSelect(s *ssa.Select) {
	for _, st := range s.States {
		if st.Send != nil {
			vis.processHeapAccess(st.Send, st.Chan, anyIndexField)
		}
	}
}

func (vis *visitor) visitCall(call *ssa.CallCommon, callsite ssa.Instruction) {
	if builtin, ok := call.Value.(*ssa.Builtin); ok {
		// special handling of builtin functions
		vis.visitBuiltin(builtin, callsite)
		return
	}
	// Collect unification constraints using the call graph.
	for _, fn := range vis.callees[call] {
		if fn == nil {
			continue
		}
		if vis.visitKnownFunction(fn, callsite) {
			// special handling of some known functions
			continue
		}

		paramCstrs, retCstrs := vis.collectCalleeConstraints(call, fn, callsite)
		// Unify caller arguments and callee parameters in matching contexts.
		for arg, params := range paramCstrs {
			for _, param := range params {
				// The copy-by-value semantics is implemented by the SSA call.
				// For example:
				//   type S struct { ... }
				//   var v S
				//   g(v)
				//   func g(a S) { ... }
				// SSA creates a copy of v and passes it to g. Hence we directly
				// unify this copy and g's argument "a".
				if mayShareObject(arg) {
					vis.unifyCallWithContexts(arg, param, callsite)
				}
			}
		}
		// Handle return value unification.
		for callerRet, rets := range retCstrs {
			for _, calleeRet := range rets {
				if len(calleeRet) == 1 { // non-tuple case
					param := calleeRet[0]
					if mayShareObject(param) {
						vis.unifyCallWithContexts(callerRet, param, callsite)
					}
					continue
				}
				// tuple case
				// For call "return_r = f(...)", if the callee returns a tuple
				// <r0, r1, ...>, then the i^{th} element in the tuple is
				// connected to the caller's return_r by making
				// "return_r[i] = r{i}".
				for k := 0; k < len(calleeRet); k++ {
					retV := calleeRet[k]
					if !mayShareObject(retV) {
						continue
					}
					// Implement "return_r[i] = r{i}".
					field := Field{Name: strconv.Itoa(k)}
					// unify instance field
					for _, c := range vis.getContexts(callerRet) {
						callerCxt := append(*c, callsite)
						for _, calleeCxt := range vis.getContexts(retV) {
							if vis.contextKEqual(callerCxt, *calleeCxt) {
								vis.unifyField(c, callerRet, field, retV)
							}
						}
					}
				}
			}
		}
	}
}

// Unify the argument in the caller with the parameter in the callee under all contexts
func (vis *visitor) unifyCallWithContexts(arg ssa.Value, param ssa.Value, callsite ssa.Instruction) {
	state := vis.state
	// The caller may have multiple contexts;
	// perform the unification for each context.
	for _, c := range vis.getContexts(arg) {
		// For each context of the callee, match it with the caller context
		// plus the call site. Unify the caller' arg and the callee's param
		// for each matched context.
		// For example, assume K=1,
		//   func f(x, y *T) {
		//       g(x)
		//       g(y)
		//   }
		//   func g(a *T) {}
		// g() is called at contexts [g(x)] and [g(y)], so g.a is unified
		// with f.x and f.y w.r.t. these contexts, resulting in two
		// partitions {[g(x)]g.a, f.x} and {[g(y)]g.a, f.y}.
		argCxt := append(*c, callsite)
		argRef := MakeReference(c, arg)
		for _, paramCxt := range vis.getContexts(param) {
			if vis.contextKEqual(argCxt, *paramCxt) {
				state.Unify(argRef, MakeReference(paramCxt, param))
				break
			}
		}
	}
}

// Handle calls to builtin functions: https://golang.org/pkg/builtin/.
func (vis *visitor) visitBuiltin(builtin *ssa.Builtin, instr ssa.Instruction) {
	// TODO(#312): support more library functions
	switch builtin.Name() {
	case "append": // func append(slice []Type, elems ...Type) []Type
		// Propagage the arguments to the return value.
		if dst, ok := instr.(ssa.Value); ok {
			ops := instr.Operands(nil)
			// ops[0] is the function itself.
			for i := 1; i < len(ops); i++ {
				// Unify-by-reference the dst and the op for all contexts.
				vis.unifyLocals(dst, *ops[i])
			}
		}
	case "copy": // "func copy(dst, src []Type) int"
		{
			// Propagage the source to the destination through unify-by-value.
			ops := instr.Operands(nil)
			// ops[0] is the function itself.
			dst, src := *ops[1], *ops[2]
			if mayShareObject(dst) && mayShareObject(src) {
				for _, c := range vis.getContexts(dst) {
					vis.unifyByValue(MakeLocal(c, dst), MakeLocal(c, src))
				}
			}
		}
	}
}

// Handle some known functions, e.g. in package "fmt".
func (vis *visitor) visitKnownFunction(fn *ssa.Function, instr ssa.Instruction) bool {
	// TODO(#312): Handle standard library functions.
	// Add an operand reference to a field of the "dst", i.e. dst[index -> op].
	addField := func(dst ssa.Value, op ssa.Value, index int) {
		fd := Field{Name: strconv.Itoa(index)}
		if mayShareObject(op) {
			for _, c := range vis.getContexts(dst) {
				vis.unifyField(c, dst, fd, op)
			}
		}
	}
	fname := fn.Name()
	switch {
	case strings.HasPrefix(fname, "Sprint"), strings.HasPrefix(fname, "Errorf"):
		// Propagate the arguments to the return value using fields.
		if dst, ok := instr.(ssa.Value); ok {
			ops := instr.Operands(nil)
			// ops[0] is the function itself.
			for i := 1; i < len(ops); i++ {
				addField(dst, *ops[i], i)
			}
		}
		return true
	case strings.HasPrefix(fname, "Fprint"):
		// Propagate the arguments to the first argument using fields.
		ops := instr.Operands(nil)
		// ops[0] is the function itself.
		first := *ops[1] // the io stream
		for i := 2; i < len(ops); i++ {
			addField(first, *ops[i], i)
		}
		return true
	default:
		return false
	}
}

// Collect unification constraints corresponding to a call.
// This generates constraints for unifying parameters, free variables, and return values.
func (vis *visitor) collectCalleeConstraints(common *ssa.CallCommon, fn *ssa.Function, instr ssa.Instruction) (map[ssa.Value][]ssa.Value, map[ssa.Value][][]ssa.Value) {
	// Handle undefined/unlinked functions.
	if fn == nil || len(fn.Blocks) == 0 {
		return nil, nil
	}
	// TODO: in some rare cases, SSA may generate an imported function with no arguments while this function actually
	//  takes arguments. Skip such functions here.
	if len(fn.Params) != len(common.Args) {
		return nil, nil
	}

	paramCstrs := make(map[ssa.Value][]ssa.Value)
	// Add caller_arg -> {callee_parameter} constraints for parameters.
	// For example, for g(a, b) and func g(x, y), add <a, g.x> and
	// <b, g.y> into the constraints.
	for i := 0; i < len(common.Args); i++ {
		arg, param := common.Args[i], fn.Params[i]
		if mayShareObject(arg) && typeMayShareObject(param.Type()) {
			paramCstrs[arg] = append(paramCstrs[arg], param)
		}
	}
	// For a closure call, add constraints for free variable bindings from
	// caller to callee.
	if clos, ok := common.Value.(*ssa.MakeClosure); ok {
		for i, bind := range clos.Bindings {
			if mayShareObject(bind) {
				fv := fn.FreeVars[i]
				paramCstrs[fv] = append(paramCstrs[fv], bind)
			}
		}
	}

	// Skip generating constraint if the return variable doesn't exist.
	callerRet, ok := instr.(ssa.Value)
	if !ok {
		return paramCstrs, nil
	}
	// Collect unification constraint for return registers.
	// For example, for t0 = g(a), where g returns tuple (a,b),
	// add <t0, (a,b)> into the constraints.
	retCstrs := make(map[ssa.Value][][]ssa.Value)
	for _, blk := range fn.Blocks {
		if ret, ok := blk.Instrs[len(blk.Instrs)-1].(*ssa.Return); ok {
			if len(ret.Results) > 0 {
				retCstrs[callerRet] = append(retCstrs[callerRet], ret.Results)
			}
		}
	}
	return paramCstrs, retCstrs
}

// Process a load/store using an index (which can be constant).
func (vis *visitor) processIndex(data ssa.Value, base ssa.Value, index ssa.Value) {
	// If the index is a constant, its string name is used as the field name;
	// otherwise the predefined field name "AnyField" is used.
	var field Field
	if c, ok := index.(*ssa.Const); ok {
		field = Field{Name: c.Value.String()}
		vis.processHeapAccess(data, base, field)
		// base[c -> data] implies base[AnyField -> data]
		vis.processHeapAccess(data, base, anyIndexField)
	} else {
		vis.processHeapAccess(data, base, anyIndexField)
	}
}

// Process a load/store from data_reg to (base, index), where base can be a register
// or global variable. Here heap access is through unify-by-reference simulation.
func (vis *visitor) processHeapAccess(data ssa.Value, base ssa.Value, fd Field) {
	// Skip unification if lhs is not a sharable type.
	if !typeMayShareObject(data.Type()) {
		return
	}
	// unify instance field
	for _, context := range vis.getContexts(base) {
		vis.unifyField(context, base, fd, data)
	}
}

// Process memory operator & and * by making the address pointing to the value.
func (vis *visitor) processAddressToValue(addr ssa.Value, value ssa.Value) {
	if !typeMayShareObject(addr.Type()) || !typeMayShareObject(value.Type()) {
		return
	}
	state := vis.state
	for _, context := range vis.getContexts(addr) {
		caddr := state.Insert(MakeReference(context, addr))
		cvalue := state.Insert(MakeReference(context, value))
		fmap := state.PartitionFieldMap(state.representative(caddr))
		// Add or unify the field.
		if v, ok := fmap[directPointToField]; ok {
			if isUnifyByReference(value.Type()) { // unify-by-reference
				state.Unify(v, cvalue)
			} else {
				// TODO(#314): handle the unify-by-value semantics
				state.Unify(v, cvalue)
			}
		} else {
			fmap[directPointToField] = cvalue
		}
	}
}

// Unify local regs "u" and "v" in all contexts.
func (vis *visitor) unifyLocals(u ssa.Value, v ssa.Value) {
	if !mayShareObject(u) || !mayShareObject(v) {
		return
	}
	state := vis.state
	for _, context := range vis.getContexts(u) {
		state.Unify(MakeReference(context, u), MakeReference(context, v))
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
	// TODO(#314): handle the unify-by-value semantics.
	// unify the two references
	// Add or unify the field.
	if v, ok := fmap[fd]; ok {
		state.Unify(v, tr)
	} else {
		fmap[fd] = tr
	}
}

// Unify "&obj.field" and "target" in "context".
// For example, for "t1 = &t0.x", after the unification, t0 has a field x pointing to t1.
func (vis *visitor) unifyFieldAddress(context *Context, obj ssa.Value, field *Field, target ssa.Value) {
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
		// TODO(#314): Recursively call "UnifyByValue" here.
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

// Return all the calling contexts of the function to which a value belongs.
func (vis *visitor) getContexts(v ssa.Value) []*Context {
	if fn := v.Parent(); fn != nil {
		if ctxs, ok := vis.contexts[fn]; ok {
			return ctxs
		}
	}
	// Otherwise return an empty context, e.g. for a Global.
	return []*Context{&emptyContext}
}

// Return whether the last K sites of two contexts are equal.
func (vis *visitor) contextKEqual(c1 Context, c2 Context) bool {
	l1, l2 := len(c1), len(c2)
	for i := 0; i < vis.contextK; i++ {
		if i >= l1 || i >= l2 { // no more to compare
			return true
		}
		if c1[l1-i-1] != c2[l2-i-1] {
			return false
		}
	}
	// All the last K sites in the two contexts match.
	return true
}

// Maps a callsite to the set of candidate callee functions.
func mapCallees(cg *callgraph.Graph) map[*ssa.CallCommon][]*ssa.Function {
	callees := make(map[*ssa.CallCommon][]*ssa.Function)
	for f, node := range cg.Nodes {
		if f == nil {
			continue
		}
		for _, in := range node.In {
			common := in.Site.Common()
			callees[common] = append(callees[common], in.Callee.Func)
		}
	}
	return callees
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
	return &Field{Name: strconv.Itoa(index)}
}
