package internal

import (
	"fmt"
	"go/types"
	"log"

	"github.com/google/go-flow-levee/internal/pkg/sanitizer"
	"github.com/google/go-flow-levee/internal/pkg/source"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/ssa"
)

type DFSTools source.Source

// dfs performs Depth-First-Search on the def-use graph of the input Source.
// While traversing the graph we also look for potential sanitizers of this Source.
// If the Source passes through a sanitizer, dfs does not continue through that Node.
func (s *DFSTools) Dfs(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock, isReferrer bool) {
	if s.shouldNotVisit(n, maxInstrReached, lastBlockVisited, isReferrer) {
		return
	}
	s.PreOrder = append(s.PreOrder, n)
	s.Marked[n] = true

	mirCopy := map[*ssa.BasicBlock]int{}
	for m, i := range maxInstrReached {
		mirCopy[m] = i
	}

	if instr, ok := n.(ssa.Instruction); ok {
		instrIndex, ok := IndexInBlock(instr)
		if !ok {
			return
		}

		if mirCopy[instr.Block()] < instrIndex {
			mirCopy[instr.Block()] = instrIndex
		}

		lastBlockVisited = instr.Block()
	}

	s.visit(n, mirCopy, lastBlockVisited)
}

func (s *DFSTools) shouldNotVisit(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock, isReferrer bool) bool {
	if s.Marked[n] {
		return true
	}

	if !source.HasTaintableType(n) {
		return true
	}

	if instr, ok := n.(ssa.Instruction); ok {
		instrIndex, ok := IndexInBlock(instr)
		if !ok {
			return true
		}

		// If the referrer is in a different block from the one we last visited,
		// and it can't be reached from the block we are visiting, then stop visiting.
		if lastBlockVisited != nil && instr.Block() != lastBlockVisited && !s.canReach(lastBlockVisited, instr.Block()) {
			return true
		}

		// If this call's index is lower than the highest seen so far in its block,
		// then this call is "in the past". If this call is a referrer,
		// then we would be propagating taint backwards in time, so stop traversing.
		// (If the call is an operand, then it is being used as a value, so it does
		// not matter when the call occurred.)
		if _, ok := instr.(*ssa.Call); ok && instrIndex < maxInstrReached[instr.Block()] && isReferrer {
			return true
		}
	}

	return false
}

func (s *DFSTools) visit(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	if s.Node == n {
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		return
	}

	switch t := n.(type) {
	case *ssa.Alloc:
		// An Alloc represents the allocation of space for a variable. If a Node is an Alloc,
		// and the thing being allocated is not an array, then either:
		// a) it is a Source value, in which case it will get its own traversal when sourcesFromBlocks
		//    finds this Alloc
		// b) it is not a Source value, in which case we should not visit it.
		// However, if the Alloc is an array, then that means the source that we are visiting from
		// is being placed into an array, slice or varags, so we do need to keep visiting.
		if _, isArray := utils.Dereference(t.Type()).(*types.Array); isArray {
			s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		}

	case *ssa.Call:
		if callee := t.Call.StaticCallee(); callee != nil && s.Config.IsSanitizer(utils.DecomposeFunction(callee)) {
			s.Sanitizers = append(s.Sanitizers, &sanitizer.Sanitizer{Call: t})
		}

		// This is to avoid attaching calls where the source is the receiver, ex:
		// core.Sinkf("Source id: %v", wrapper.Source.GetID())
		if recv := t.Call.Signature().Recv(); recv != nil && source.IsSourceType(s.Config, s.TaggedFields, recv.Type()) {
			return
		}

		s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		for _, a := range t.Call.Args {
			if source.CanBeTaintedByCall(a.Type()) {
				s.Dfs(a.(ssa.Node), maxInstrReached, lastBlockVisited, false)
			}
		}

	case *ssa.FieldAddr:
		deref := utils.Dereference(t.X.Type())
		typPath, typName := utils.DecomposeType(deref)
		fieldName := utils.FieldName(t)
		if !s.Config.IsSourceField(typPath, typName, fieldName) && !s.TaggedFields.IsSourceFieldAddr(t) {
			return
		}
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		s.visitOperands(n, maxInstrReached, lastBlockVisited)

	// Everything but the actual integer Index should be visited.
	case *ssa.Index:
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		s.Dfs(t.X.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// Everything but the actual integer Index should be visited.
	case *ssa.IndexAddr:
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		s.Dfs(t.X.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// Only the Addr (the Value that is being written to) should be visited.
	case *ssa.Store:
		s.Dfs(t.Addr.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// Only the Map itself can be tainted by an Update.
	// The Key can't be tainted.
	// The Value can propagate taint to the Map, but not receive it.
	// MapUpdate has no referrers, it is only an Instruction, not a Value.
	case *ssa.MapUpdate:
		s.Dfs(t.Map.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// The only Operand that can be tainted by a Send is the Chan.
	// The Value can propagate taint to the Chan, but not receive it.
	// Send has no referrers, it is only an Instruction, not a Value.
	case *ssa.Send:
		s.Dfs(t.Chan.(ssa.Node), maxInstrReached, lastBlockVisited, false)

	// These nodes' operands should not be visited, because they can only receive
	// taint from their operands, not propagate taint to them.
	case *ssa.BinOp, *ssa.ChangeInterface, *ssa.ChangeType, *ssa.Convert, *ssa.Extract, *ssa.MakeChan, *ssa.MakeMap, *ssa.MakeSlice, *ssa.Phi, *ssa.Range:
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)

	// These nodes don't have operands; they are Values, not Instructions.
	case *ssa.Const, *ssa.FreeVar, *ssa.Global, *ssa.Lookup, *ssa.Parameter:
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)

	// These nodes don't have referrers; they are Instructions, not Values.
	case *ssa.Go:
		s.visitOperands(n, maxInstrReached, lastBlockVisited)

	// These nodes are both Instructions and Values, and currently have no special restrictions.
	case *ssa.Field, *ssa.MakeInterface, *ssa.Select, *ssa.Slice, *ssa.TypeAssert, *ssa.UnOp:
		s.visitReferrers(n, maxInstrReached, lastBlockVisited)
		s.visitOperands(n, maxInstrReached, lastBlockVisited)

	// These nodes cannot propagate taint.
	case *ssa.Builtin, *ssa.DebugRef, *ssa.Defer, *ssa.Function, *ssa.If, *ssa.Jump, *ssa.MakeClosure, *ssa.Next, *ssa.Panic, *ssa.Return, *ssa.RunDefers:

	default:
		fmt.Printf("unexpected node received: %T %v; please report this issue\n", n, n)
	}
}

func (s *DFSTools) visitReferrers(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	if n.Referrers() == nil {
		return
	}
	for _, r := range *n.Referrers() {
		s.Dfs(r.(ssa.Node), maxInstrReached, lastBlockVisited, true)
	}
}

func (s *DFSTools) visitOperands(n ssa.Node, maxInstrReached map[*ssa.BasicBlock]int, lastBlockVisited *ssa.BasicBlock) {
	for _, o := range n.Operands(nil) {
		if *o == nil {
			continue
		}
		s.Dfs((*o).(ssa.Node), maxInstrReached, lastBlockVisited, false)
	}
}

func (s *DFSTools) canReach(start *ssa.BasicBlock, dest *ssa.BasicBlock) bool {
	if start.Dominates(dest) {
		return true
	}

	stack := Stack([]*ssa.BasicBlock{start})
	seen := map[*ssa.BasicBlock]bool{start: true}
	for len(stack) > 0 {
		current := stack.Pop()
		if current == dest {
			return true
		}
		for _, s := range current.Succs {
			if seen[s] {
				continue
			}
			seen[s] = true
			stack.Push(s)
		}
	}
	return false
}

type Stack []*ssa.BasicBlock

func (s *Stack) Pop() *ssa.BasicBlock {
	if len(*s) == 0 {
		log.Println("tried to pop from empty stack")
	}
	popped := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	return popped
}

func (s *Stack) Push(b *ssa.BasicBlock) {
	*s = append(*s, b)
}

// indexInBlock returns this instruction's index in its parent block.
func IndexInBlock(target ssa.Instruction) (int, bool) {
	for i, instr := range target.Block().Instrs {
		if instr == target {
			return i, true
		}
	}
	// we can only hit this return if there is a bug in the ssa package
	// i.e. an instruction does not appear within its parent block
	return 0, false
}
