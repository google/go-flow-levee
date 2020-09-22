// Copyright 2019 Google LLC
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

// Package interp defines an analyzer that performs taint analysis on
// functions by interpreting their ssa code.
package interp

import (
	"go/token"
	"go/types"
	"reflect"

	"github.com/google/go-flow-levee/internal/pkg/config"
	"github.com/google/go-flow-levee/internal/pkg/fieldpropagator"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

// A Result contains the information necessary to report that
// a Source has reached a Sink. Currently this is just the
// source code positions of the Source and Sink.
type Result struct {
	SourcePos, SinkPos token.Pos
}

// A blocksKey is used to determine whether the current block has already
// been visited from the pred(ecessor) block.
type blocksKey struct {
	current, pred *ssa.BasicBlock
}

// A val represents a Value that is tainted by a source.
type val struct {
	source ssa.Value
}

// A ptr represents a Value that points to another Value. This implies that
// taint needs to be propagated to and from the pointed to value.
type ptr struct {
	pointsTo ssa.Value
}

// An interpreterState is a mapping from ssa Values to either a val or a ptr.
type interpreterState map[ssa.Value]interface{}

// An interpreter is used to perform taint analysis on a function by
// stepping through its instructions. When a source reaches a sink,
// a result is added to its slice of Results.
type interpreter struct {
	conf             *config.Config
	fieldpropagators fieldpropagator.ResultType
	results          []Result
}

// ResultType is a slice of Results.
type ResultType = []Result

// Analyzer is the analysis.Analyzer for the interp package.
var Analyzer = &analysis.Analyzer{
	Name: "interp",
	Doc:  "Performs taint propagation analysis by stepping through SSA instructions.",
	Run:  run,
	Requires: []*analysis.Analyzer{
		buildssa.Analyzer, fieldpropagator.Analyzer,
	},
	ResultType: reflect.TypeOf(new(ResultType)).Elem(),
}

func run(pass *analysis.Pass) (interface{}, error) {
	conf, err := config.ReadConfig()
	if err != nil {
		return nil, err
	}

	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	fieldPropagators := pass.ResultOf[fieldpropagator.Analyzer].(fieldpropagator.ResultType)

	results := []Result{}
	for _, f := range ssaInput.SrcFuncs {
		results = append(results, analyzeFunction(conf, fieldPropagators, f)...)
	}

	return results, nil
}

// analyzeFunction performs taint analysis on the given function and
// returns a slice of Results.
func analyzeFunction(conf *config.Config, fieldpropagators fieldpropagator.ResultType, f *ssa.Function) []Result {
	// nothing to analyze
	if len(f.Blocks) == 0 {
		return nil
	}

	state := interpreterState{}
	// populate the initial state with the function's tainted parameters
	for _, p := range f.Params {
		if conf.IsSource(utils.Dereference(p.Type())) {
			state[p] = val{source: p}
		}
	}

	interpreter := interpreter{
		conf:             conf,
		fieldpropagators: fieldpropagators,
	}

	analyzed := map[blocksKey]bool{}
	interpreter.analyzeBlock(f.Blocks[0], nil, state, analyzed)

	return interpreter.results
}

// analyzeBlock analyzes a given block and recursively analyzes its successors.
// We copy the state before analyzing the block's instructions to avoid affecting
// other branches of the analysis.
// After analyzing the instructions in the block, the used state is provided as
// input to its successors. This is so the analysis of the successors will know
// about any changes in state caused by analyzing the block.
// If a block has many predecessors, in an execution of the program it could be
// visited through any of them so we need to analyze it for each predecessor.
// To avoid repeating work and getting stuck in cycles, we want to avoid
// analyzing a block coming from the same predecessor twice.
func (in *interpreter) analyzeBlock(block, pred *ssa.BasicBlock, startState interpreterState, analyzed map[blocksKey]bool) {
	if analyzed[blocksKey{block, pred}] {
		return
	}
	analyzed[blocksKey{block, pred}] = true

	state := startState.copy()
	in.analyzeInstructions(block, pred, state)
	for _, succ := range block.Succs {
		in.analyzeBlock(succ, block, state, analyzed)
	}
}

// analyzeInstructions performs the actual work of analyzing a block by
// stepping through its instructions.
// Each instruction type is handled via a case in a switch.
func (in *interpreter) analyzeInstructions(block, pred *ssa.BasicBlock, state interpreterState) {
	for _, instr := range block.Instrs {
		switch t := instr.(type) {
		case *ssa.Alloc:
			if in.conf.IsSource(utils.Dereference(t.Type())) {
				state[t] = val{source: t}
			}

		case *ssa.Call:
			in.handleCall(state, t)

		case *ssa.ChangeInterface:
			state[t] = state[t.X]

		case *ssa.Extract:
			// an Extracted value is derived from a Tuple
			in.handleDerivedValue(state, t, t.Tuple)

		case *ssa.Field:
			// the field itself is a Source
			if in.conf.IsSource(utils.Dereference(t.Type())) {
				state[t] = val{source: t}
				continue
			}
			// the field is a source field on a Source type
			if under, ok := t.X.Type().Underlying().(*types.Struct); ok {
				fld := under.Field(t.Field)
				if in.conf.IsSourceField(utils.Dereference(t.X.Type()), fld) {
					state[t] = val{source: t.X}
				}
			}

		case *ssa.FieldAddr:
			if in.conf.IsSource(utils.Dereference(t.Type())) || in.conf.IsSourceFieldAddr(t) {
				state[t] = val{source: t}
				state[t.X] = val{source: t.X}
			}

		case *ssa.IndexAddr:
			state[t] = ptr{pointsTo: t.X}

		case *ssa.Lookup:
			// a Looked up value is derived from a String or Map
			in.handleDerivedValue(state, t, t.X)

		case *ssa.MakeClosure:
			closure := t.Fn.(*ssa.Function)
			// record the state of the closure's free variables
			// using the values being bound
			for i := 0; i < len(closure.FreeVars); i++ {
				state[closure.FreeVars[i]] = state[t.Bindings[i]]
			}
			in.analyzeBlock(closure.Blocks[0], block, state, map[blocksKey]bool{})

		case *ssa.MakeInterface:
			switch {
			case pointer.CanPoint(t.X.Type()):
				state[t] = ptr{pointsTo: t.X}
			default:
				state[t] = state[t.X]
			}

		case *ssa.MapUpdate:
			// propagate taint from key if key is tainted
			if kv, ok := state[t.Key].(val); ok && kv.source != nil {
				state[t.Map] = kv
			}
			// propagate taint from valueif value is tainted
			if vv, ok := state[t.Value].(val); ok && vv.source != nil {
				state[t.Map] = vv
			}

		case *ssa.Phi:
			pi := findPredIndex(block.Preds, pred)
			if ev, ok := state[t.Edges[pi]].(val); ok && ev.source != nil {
				state[t] = ev
			}

		case *ssa.Send:
			state[t.Chan] = state[t.X]

		case *ssa.Slice:
			state[t] = state[t.X]

		case *ssa.Store:
			switch addr := state[t.Addr].(type) {
			case val:
				state[t.Addr] = state[t.Val]
			case ptr:
				// need this check so we don't remove existing taint
				if x := dereference(state, t.Val); x.source != nil {
					state[addr.pointsTo] = state[t.Val]
				}
			}

		case *ssa.TypeAssert:
			state[t] = state[t.X]

		case *ssa.UnOp:
			switch t.Op {
			// dereferencing a pointer, e.g. *ptr
			case token.MUL:
				switch tx := state[t.X].(type) {
				case val:
					if tx.source != nil {
						state[t] = tx
					}
				case ptr:
					state[t] = state[tx.pointsTo]
					if in.conf.IsSource(utils.Dereference(t.X.Type())) {
						state[t] = val{source: t.X}
					}
				}
			// receiving from a chan, e.g. <-chan
			case token.ARROW:
				// a Received value is derived from a Chan
				in.handleDerivedValue(state, t, t.X)
			}

		}
	}
}

func (in *interpreter) handleCall(state map[ssa.Value]interface{}, c *ssa.Call) {
	switch {
	case in.conf.IsSinkCall(c):
		in.handleSinkCall(state, c)
		return
	case in.conf.IsSanitizer(c):
		in.handleSanitizerCall(state, c)
		return
	}

	// method
	if sc := c.Call.StaticCallee(); sc != nil && sc.Signature.Recv() != nil {
		recv := c.Call.Args[0]
		if in.fieldpropagators.IsFieldPropagator(c) {
			state[c] = val{source: recv}
			return
		}
		if in.conf.IsSource(recv.Type()) {
			return
		}
	}

	v := val{}

	// receive taint from arguments
	for _, o := range c.Operands(nil) {
		if o == nil {
			continue
		}
		x := dereference(state, *o)
		if x.source != nil {
			v = x
		}
	}

	// taint colocated arguments
	for _, o := range c.Operands(nil) {
		if o == nil {
			continue
		}
		if _, ok := (*o).(*ssa.Function); ok {
			continue
		}
		if pointer.CanPoint((*o).Type()) && v.source != nil {
			switch so := state[*o].(type) {
			case ptr:
				state[so.pointsTo] = v
			default:
				state[*o] = v
			}
		}
	}

	// call returns a Source
	if v.source == nil && in.conf.IsSource(utils.Dereference(c.Type())) {
		v.source = c
	}

	if v.source != nil {
		state[c] = v
	}
}

func (in *interpreter) handleSinkCall(state map[ssa.Value]interface{}, c *ssa.Call) {
	for _, o := range c.Operands(nil) {
		if o == nil {
			continue
		}
		x := dereference(state, *o)
		if x.source != nil {
			in.results = append(in.results, createResult(x.source, c))
		}
	}
}

func (in *interpreter) handleSanitizerCall(state map[ssa.Value]interface{}, c *ssa.Call) {
	// sanitize the return value
	state[c] = val{}

	// sanitize values passed by pointer
	for _, o := range c.Operands(nil) {
		if o == nil {
			continue
		}
		if _, ok := (*o).(*ssa.Function); ok {
			continue
		}
		if pointer.CanPoint((*o).Type()) {
			switch so := state[*o].(type) {
			case val:
				state[*o] = val{}
			case ptr:
				state[so.pointsTo] = val{}
			}
		}
	}
}

func (in *interpreter) handleDerivedValue(state map[ssa.Value]interface{}, derived ssa.Value, from ssa.Value) {
	// the value itself is a Source
	if in.conf.IsSource(utils.Dereference(derived.Type())) {
		state[derived] = val{source: derived}
	}
	// the value from which the value is derived is a Source
	if c, ok := state[from].(val); ok && c.source != nil {
		state[derived] = c
	}
}

func createResult(source, sink ssa.Value) Result {
	sourcePos := source.Pos()
	if e, ok := source.(*ssa.Extract); ok {
		sourcePos = e.Tuple.Pos()
	}
	if fa, ok := source.(*ssa.FieldAddr); ok && sourcePos == token.NoPos {
		sourcePos = fa.X.Pos()
	}
	return Result{SourcePos: sourcePos, SinkPos: sink.Pos()}
}

func dereference(state map[ssa.Value]interface{}, v ssa.Value) val {
	if v == nil {
		return val{}
	}
	switch t := state[v].(type) {
	case val:
		return t
	case ptr:
		return dereference(state, t.pointsTo)
	}
	return val{}
}

func findPredIndex(preds []*ssa.BasicBlock, pred *ssa.BasicBlock) int {
	for i, p := range preds {
		if p == pred {
			return i
		}

	}
	panic("this can only occur due to a bug in the ssa package")
}

func (is interpreterState) copy() interpreterState {
	s := interpreterState{}
	for k, v := range is {
		s[k] = v
	}
	return s
}
