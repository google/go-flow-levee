package EAR

import (
	"reflect"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name:       "ear pointer",
	Doc:        "apply pointer analysis",
	Run:        run,
	ResultType: reflect.TypeOf(new(AbsState)),
	Requires:   []*analysis.Analyzer{buildssa.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	ssainput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	state := NewAbsState()
	for _, fn := range ssainput.SrcFuncs {
		if err := runFunc(pass, fn, state); err != nil {
			return nil, err
		}
	}
	return &state, nil
}

//// isLocalVariable returns whether a value is a local variable
//// such as a paramter or a register instruction.
//func isLocalVariable(v ssa.Value) bool {
//	if _, ok := v.(*ssa.Parameter); ok {
//		return true
//	}
//	if ins, ok := v.(ssa.Instruction); ok {
//		// Consider only register instruction.
//		if _, ok := (ins).(ssa.Value); ok {
//			return true
//		}
//	}
//	return false
//}

func runFunc(pass *analysis.Pass, fn *ssa.Function, state *AbsState) error {
	blks := fn.Blocks
	for _, blk := range blks {
		for _, instr := range blk.Instrs {
			visitInstruction(instr, state)
		}
	}
	return nil
}

func visitInstruction(instr ssa.Instruction, state *AbsState) {

}


func VisitPhiInstruction(phi *ssa.Phi, state *AbsState) {

}

func VisitStoreAddressInstruction(store *ssa.Store) {

}

func VisitMapUpdateInstruction(update *ssa.MapUpdate) {

}

func VisitLoadFieldInstruction(addr *ssa.FieldAddr) {

}

func VisitLoadIndexInstruction(addr *ssa.IndexAddr) {

}


func VisitLookupInstruction(lookup *ssa.Lookup) {

}

func VisitCastInstruction(convert *ssa.Convert) {

}

func VisitUnaryOpInstruction(op *ssa.UnOp) {

}

func VisitTypeAssertInstruction(assert *ssa.TypeAssert) {

}

func VisitExtractInstruction(extract *ssa.Extract) {

}

func VisitNextInstruction(next *ssa.Next) {

}

func VisitSliceInstruction(slice *ssa.Slice) {

}

func VisitSendInstruction(send *ssa.Send) {

}

// func VisitSelectInstruction() { }

func VisitCallInstruction(call *ssa.Call) {
}

func VisitGoInstruction(p *ssa.Go) {
}

func VisitDeferInstruction(p *ssa.Defer) {
}

// Process calls to builtin functions; return true if the callee is a builtin
// function.
func ProcessBuiltinCall(instruction ssa.Instruction) {

}
