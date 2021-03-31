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
	"github.com/google/go-flow-levee/internal/pkg/propagation/summary"
	"golang.org/x/tools/go/ssa"
)

// taintStdlibCall propagates taint through a static call to a standard
// library function, or through an implementation of a standard library
// interface function, provided that the function's taint propagation behavior
// is known (i.e. the function has a summary).
func (prop *Propagation) taintStdlibCall(call *ssa.Call, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	summ := summary.For(call)
	if summ == nil {
		return
	}

	var args []ssa.Value
	// For "invoke" calls, Value is the receiver
	if call.Call.IsInvoke() {
		args = append(args, call.Call.Value)
	}
	args = append(args, call.Call.Args...)

	// Determine whether we need to propagate taint.
	// Specifically: is at least one argument tainted?
	tainted := int64(0)
	for i, a := range args {
		if prop.tainted[a.(ssa.Node)] {
			tainted |= 1 << i
		}
	}
	if (tainted & summ.IfTainted) == 0 {
		return
	}

	// Taint call arguments.
	for _, i := range summ.TaintedArgs {
		prop.taint(args[i].(ssa.Node), maxInstrReached, lastBlockVisited, false)
	}

	// Taint call referrers, if there are any.
	if call.Referrers() == nil {
		return
	}

	// If the call has a single return value, the return value is the call
	// instruction itself.
	if call.Call.Signature().Results().Len() == 1 {
		if len(summ.TaintedRets) > 0 {
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
	for i := range summ.TaintedRets {
		prop.taint(indexToExtract[i], maxInstrReached, lastBlockVisited, true)
	}
}
