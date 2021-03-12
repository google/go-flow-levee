package propagation

import (
	"go/types"
	"strings"

	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/ssa"
)

// taintStdlibCall propagates taint through a static call to a standard
// library function, provided that the function's taint propagation behavior
// is known (i.e. the function has a summary).
func (prop *Propagation) taintStdlibCall(call *ssa.Call, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	summ, ok := funcSummaries[funcName(call)]
	if !ok {
		return
	}

	prop.taintFromSummary(summ, call, call.Call.Args, maxInstrReached, lastBlockVisited)
}

// taintStdlibInterfaceCall propagates taint through a static call to a standard
// library function, provided that the function's taint propagation behavior
// is known (i.e. the function has a summary).
func (prop *Propagation) taintStdlibInterfaceCall(call *ssa.Call, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	summ, ok := interfaceFuncSummaries[funcKey{funcNameWithoutReceiver(call), sigTypeString(call.Call.Signature())}]
	if !ok {
		return
	}

	var args []ssa.Value
	if call.Call.IsInvoke() {
		args = append(args, call.Call.Value)
	}
	args = append(args, call.Call.Args...)

	prop.taintFromSummary(summ, call, args, maxInstrReached, lastBlockVisited)
}

func (prop *Propagation) taintFromSummary(summ summary, call *ssa.Call, args []ssa.Value, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	tainted := int64(0)
	for i, a := range args {
		if prop.tainted[a.(ssa.Node)] {
			tainted |= 1 << i
		}
	}
	if (tainted & summ.ifTainted) == 0 {
		return
	}

	for _, i := range summ.taintedArgs {
		prop.taint(args[i].(ssa.Node), maxInstrReached, lastBlockVisited, false)
	}

	if call.Referrers() == nil {
		return
	}

	if call.Call.Signature().Results().Len() == 1 {
		if len(summ.taintedRets) > 0 {
			prop.taintReferrers(call, maxInstrReached, lastBlockVisited)
		}
		return
	}

	indexToExtract := map[int]*ssa.Extract{}
	for _, r := range *call.Referrers() {
		e := r.(*ssa.Extract)
		indexToExtract[e.Index] = e
	}
	for i := range summ.taintedRets {
		prop.taint(indexToExtract[i], maxInstrReached, lastBlockVisited, true)
	}
}

func sigTypeString(sig *types.Signature) string {
	var b strings.Builder
	paramsPtr := sig.Params()
	b.WriteByte('(')
	if paramsPtr != nil {
		params := *paramsPtr
		for i := 0; i < params.Len(); i++ {
			p := params.At(i)
			b.WriteString(utils.UnqualifiedName(p.Type()))
			if i+1 != params.Len() {
				b.WriteByte(',')
			}
		}
	}
	b.WriteByte(')')
	resultsPtr := sig.Results()
	b.WriteByte('(')
	if resultsPtr != nil {
		results := *resultsPtr
		for i := 0; i < results.Len(); i++ {
			p := results.At(i)
			b.WriteString(utils.UnqualifiedName(p.Type()))
			if i+1 != results.Len() {
				b.WriteByte(',')
			}
		}
	}
	b.WriteByte(')')
	return b.String()
}

func funcName(call *ssa.Call) string {
	cc := call.Call
	if cc.IsInvoke() {
		return cc.Method.Name()
	}
	if sc := cc.StaticCallee(); sc != nil {
		return sc.RelString(call.Parent().Pkg.Pkg)
	}
	return ""
}

func funcNameWithoutReceiver(call *ssa.Call) string {
	cc := call.Call
	if cc.IsInvoke() {
		return cc.Method.Name()
	}
	if sc := cc.StaticCallee(); sc != nil {
		return sc.Name()
	}
	return ""
}
