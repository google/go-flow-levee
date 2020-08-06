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

package render

import (
	"fmt"
	"strings"

	"github.com/google/go-flow-levee/internal/pkg/debug/graph"
	"github.com/google/go-flow-levee/internal/pkg/debug/node"
	"golang.org/x/tools/go/ssa"
)

// DOT produces DOT source code representing the SSA graph for a function.
func DOT(f *ssa.Function) string {
	return renderDOT(graph.New(f))
}

func renderDOT(g graph.Graph) string {
	return (&renderer{strings.Builder{}, g}).Render()
}

type renderer struct {
	strings.Builder
	graph.Graph
}

func (r *renderer) Render() string {
	r.init()
	r.writeSubgraphs()
	r.writeEdges()
	r.finish()
	return r.String()
}

func (r *renderer) init() {
	r.WriteString("digraph {\n")
}

func (r *renderer) writeSubgraphs() {
	for bi, b := range r.F.Blocks {
		r.WriteString(fmt.Sprintf("subgraph cluster_%d {\ncolor=black;\nlabel=%q;\n", bi, b.Comment))
		for _, i := range b.Instrs {
			n := i.(ssa.Node)
			r.WriteString(fmt.Sprintf("%q [shape=%s];\n", renderNode(n), nodeShape(n)))
		}
		r.WriteString("}\n")
	}
}

func (r *renderer) writeEdges() {
	for from, neighbors := range r.Neighbors {
		for _, to := range neighbors {
			switch to.R {
			case graph.Referrer:
				r.addReferrer(from, to.N)
			case graph.Operand:
				r.addOperand(from, to.N)
			}
		}
	}
}

func (r *renderer) addReferrer(n ssa.Node, ref ssa.Node) {
	// TODO: document this somewhere?
	// Red as in R-eferrer
	r.addEdge(n, ref, "red")
}

func (r *renderer) addOperand(n ssa.Node, op ssa.Node) {
	// TODO: document this somewhere?
	// Orange as in O-perand
	r.addEdge(n, op, "orange")
}

func (r *renderer) addEdge(from ssa.Node, to ssa.Node, color string) {
	r.WriteString(fmt.Sprintf("%q -> %q [color=%s];\n", renderNode(from), renderNode(to), color))
}

func renderNode(n ssa.Node) string {
	return fmt.Sprintf("%s\n(%s)", node.CanonicalName(n), node.TrimmedType(n))
}

func (r *renderer) finish() {
	r.WriteString("}\n")
}

func nodeShape(n ssa.Node) string {
	_, isValue := n.(ssa.Value)
	_, isInstr := n.(ssa.Instruction)
	// TODO: document this somewhere?
	switch {
	case isValue && isInstr:
		return "diamond"
	case isInstr:
		return "square"
	case isValue:
		return "ellipse"
	}
	panic(fmt.Sprintf("nodeShape: node %v is neither a Value nor an Instruction (impossible)", n))
}
