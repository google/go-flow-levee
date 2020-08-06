package graph

import (
	"fmt"

	"github.com/google/go-flow-levee/internal/pkg/debug/node"
	"golang.org/x/tools/go/ssa"
)

type NodeRelationship int

const (
	Referrer NodeRelationship = iota
	Operand
)

type RelatedNode struct {
	N ssa.Node
	R NodeRelationship
}

type Graph struct {
	// the function whose graph this is
	F *ssa.Function
	// a mapping from each node to its neighbors (referrers + operands)
	Neighbors map[ssa.Node][]RelatedNode
	// this is used while creating the graph to avoid needlessly revisiting nodes
	visited map[ssa.Node]bool
}

// New returns a new Graph constructed from a given function.
func New(f *ssa.Function) Graph {
	g := Graph{
		F:         f,
		Neighbors: map[ssa.Node][]RelatedNode{},
		visited:   map[ssa.Node]bool{},
	}
	g.visitBlocks()
	return g
}

func (g *Graph) visitBlocks() {
	for _, b := range g.F.Blocks {
		g.visit(b)
	}
}

func (g *Graph) visit(b *ssa.BasicBlock) {
	n := b.Instrs[0].(ssa.Node)

	g.visited[n] = true

	stack := []ssa.Node{n}
	for len(stack) > 0 {
		current := stack[len(stack)-1]

		if node.CanonicalName(current) == "c" {
			fmt.Println("referrers", len(*current.Referrers()))
			fmt.Println("operands", len(current.Operands([]*ssa.Value{})))
		}

		stack = stack[:len(stack)-1]
		var operands []*ssa.Value
		operands = current.Operands(operands)
		if operands != nil {
			for _, o := range operands {
				on, ok := (*o).(ssa.Node)
				if !ok {
					continue
				}
				g.addOperand(current, on)
				if g.visited[on] {
					continue
				}
				g.visited[on] = true
				stack = append(stack, on)
			}
		}
		if current.Referrers() != nil {
			for _, ref := range *current.Referrers() {
				rn := ref.(ssa.Node)
				g.addReferrer(current, rn)
				if g.visited[rn] {
					continue
				}
				g.visited[rn] = true
				stack = append(stack, rn)
			}
		}
	}
}

func (g *Graph) addReferrer(current, referrer ssa.Node) {
	g.Neighbors[current] = append(g.Neighbors[current], RelatedNode{N: referrer, R: Referrer})
}

func (g *Graph) addOperand(current, operand ssa.Node) {
	g.Neighbors[current] = append(g.Neighbors[current], RelatedNode{N: operand, R: Operand})
}
