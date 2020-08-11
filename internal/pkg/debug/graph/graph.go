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

// Package graph defines an abstraction of the SSA graph that facilitates rendering.
package graph

import (
	"log"

	"golang.org/x/tools/go/ssa"
)

type relationship int

const (
	// Referrer represents a node that is referred to as per ssa.Value.Referrers
	Referrer relationship = iota
	// Operand represents a node that is an operand of an instruction as per ssa.Instr.Operands
	Operand
)

// Node represents a node in the SSA graph, along with its relationship to a parent node.
type Node struct {
	// N is the ssa.Node wrapped by this type.
	N ssa.Node
	// R is the relationship between this node and its parent in the SSA graph.
	R relationship
}

// FuncGraph represents the SSA graph for an ssa.Function.
type FuncGraph struct {
	// F is the function whose graph this is
	F *ssa.Function
	// Children is a mapping from each node to its children (referrers + operands)
	Children map[ssa.Node][]Node
	// visited is used while creating the graph to avoid needlessly revisiting nodes
	visited map[ssa.Node]bool
	// stack is used to perform a DFS on F's SSA graph
	s stack
}

// New returns a new Graph constructed from a given function.
func New(f *ssa.Function) *FuncGraph {
	g := FuncGraph{
		F:        f,
		Children: map[ssa.Node][]Node{},
		visited:  map[ssa.Node]bool{},
	}
	g.visitBlocks()
	return &g
}

func (g *FuncGraph) visitBlocks() {
	for _, b := range g.F.Blocks {
		g.visit(b)
	}
}

func (g *FuncGraph) visit(b *ssa.BasicBlock) {
	// according to the ssa package docs, this should not happen,
	// but we don't want a panic
	if len(b.Instrs) == 0 {
		return
	}

	n := b.Instrs[0].(ssa.Node)
	g.visited[n] = true
	g.s.push(n)
	for len(g.s) > 0 {
		current := g.s.pop()
		if current == nil {
			break
		}
		g.visitOperands(current)
		g.visitReferrers(current)
	}
}

func (g *FuncGraph) visitOperands(n ssa.Node) {
	var operands []*ssa.Value
	operands = n.Operands(operands)
	if operands == nil {
		return
	}
	for _, o := range operands {
		on, ok := (*o).(ssa.Node)
		if !ok {
			continue
		}
		g.addOperand(n, on)
		g.visitNode(on)
	}
}

func (g *FuncGraph) visitReferrers(n ssa.Node) {
	if n.Referrers() == nil {
		return
	}
	for _, ref := range *n.Referrers() {
		rn := ref.(ssa.Node)
		g.addReferrer(n, rn)
		g.visitNode(rn)
	}
}

func (g *FuncGraph) visitNode(n ssa.Node) {
	if g.visited[n] {
		return
	}
	g.visited[n] = true
	g.s.push(n)
}

func (g *FuncGraph) addReferrer(current, referrer ssa.Node) {
	g.Children[current] = append(g.Children[current], Node{N: referrer, R: Referrer})
}

func (g *FuncGraph) addOperand(current, operand ssa.Node) {
	g.Children[current] = append(g.Children[current], Node{N: operand, R: Operand})
}

type stack []ssa.Node

func (s *stack) pop() ssa.Node {
	if len(*s) == 0 {
		log.Println("tried to pop from empty stack")
	}
	popped := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	return popped
}

func (s *stack) push(n ssa.Node) {
	*s = append(*s, n)
}
