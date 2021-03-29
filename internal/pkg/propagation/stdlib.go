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

// taintStdlibInterfaceCall propagates taint through a call to a function that
// implements an interface function from the standard library, provided that the
// function's taint propagation behavior is known (i.e. the function has a summary).
// This can be a static call or a dynamic call.
func (prop *Propagation) taintStdlibInterfaceCall(call *ssa.Call, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	summ, ok := interfaceFuncSummaries[funcKey{funcNameWithoutReceiver(call), sigTypeString(call.Call.Signature())}]
	if !ok {
		return
	}

	var args []ssa.Value
	// For "invoke" calls, Value is the receiver
	if call.Call.IsInvoke() {
		args = append(args, call.Call.Value)
	}
	args = append(args, call.Call.Args...)

	prop.taintFromSummary(summ, call, args, maxInstrReached, lastBlockVisited)
}

func (prop *Propagation) taintFromSummary(summ summary, call *ssa.Call, args []ssa.Value, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	// Determine whether we need to propagate taint.
	// Specifically: is at least one argument tainted?
	tainted := int64(0)
	for i, a := range args {
		if prop.tainted[a.(ssa.Node)] {
			tainted |= 1 << i
		}
	}
	if (tainted & summ.ifTainted) == 0 {
		return
	}

	// Taint call arguments.
	for _, i := range summ.taintedArgs {
		prop.taint(args[i].(ssa.Node), maxInstrReached, lastBlockVisited, false)
	}

	// Taint call referrers, if there are any.
	if call.Referrers() == nil {
		return
	}

	// If the call has a single return value, the return value is the call
	// instruction itself.
	if call.Call.Signature().Results().Len() == 1 {
		if len(summ.taintedRets) > 0 {
			prop.taintReferrers(call, maxInstrReached, lastBlockVisited)
		}
		return
	}

	// If the call has more than one return value, the call's Referrers will
	// contain one Extract for each returned value. There is no guarantee that
	// these will appear in order, so we create a map from the index of
	// each returned value to the corresponding Extract (the extracted value).
	indexToExtract := map[int]*ssa.Extract{}
	for _, r := range *call.Referrers() {
		e := r.(*ssa.Extract)
		indexToExtract[e.Index] = e
	}
	for i := range summ.taintedRets {
		prop.taint(indexToExtract[i], maxInstrReached, lastBlockVisited, true)
	}
}

// sigTypeString produces a stripped version of a function's signature, containing
// just the types of the arguments and return values.
// The receiver's type is not included.
// For a function such as:
//   WriteTo(w Writer) (n int64, err error)
// The result is:
//   (Writer)(int64,error)
func sigTypeString(sig *types.Signature) string {
	var b strings.Builder

	b.WriteByte('(')
	paramsPtr := sig.Params()
	if paramsPtr != nil {
		params := *paramsPtr
		for i := 0; i < params.Len(); i++ {
			p := params.At(i)
			b.WriteString(utils.UnqualifiedName(p))
			if i+1 != params.Len() {
				b.WriteByte(',')
			}
		}
	}
	b.WriteByte(')')

	b.WriteByte('(')
	resultsPtr := sig.Results()
	if resultsPtr != nil {
		results := *resultsPtr
		for i := 0; i < results.Len(); i++ {
			p := results.At(i)
			b.WriteString(utils.UnqualifiedName(p))
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
