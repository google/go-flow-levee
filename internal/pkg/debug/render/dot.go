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

	"github.com/google/go-flow-levee/internal/pkg/debug/node"
	"golang.org/x/tools/go/ssa"
)

// DOT produces DOT source code representing the SSA graph for a function.
func DOT(f *ssa.Function) string {
	return (&renderer{strings.Builder{}, f}).Render()
}

type renderer struct {
	strings.Builder
	f *ssa.Function
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
	for bi, b := range r.f.Blocks {
		r.WriteString(fmt.Sprintf("\tsubgraph cluster_%d {\n\t\tcolor=black;\n\t\tlabel=%q;\n", bi, b.Comment))
		for _, i := range b.Instrs {
			n := i.(ssa.Node)
			r.WriteString(fmt.Sprintf("\t\t%q [shape=%s];\n", renderNode(n), nodeShape(n)))
		}
		r.WriteString("\t}\n")
	}
}

func (r *renderer) writeEdges() {
	for _, b := range r.f.Blocks {
		for _, i := range b.Instrs {
			// we only need to write the operands, since as per the ssa package docs,
			// the referrers relation is a subset of the operands relation
			r.writeOperands(i.(ssa.Node))
		}
	}
}

func (r *renderer) writeOperands(n ssa.Node) {
	for _, o := range n.Operands(nil) {
		if *o == nil {
			continue
		}
		// Orange as in O-perand
		r.writeEdge((*o).(ssa.Node), n, "orange")
	}
}

func (r *renderer) writeEdge(from ssa.Node, to ssa.Node, color string) {
	r.WriteString(fmt.Sprintf("\t%q -> %q [color=%s];\n", renderNode(from), renderNode(to), color))
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
	switch {
	case isValue && isInstr:
		return "rectangle"
	case isInstr:
		return "diamond"
	default:
		return "ellipse"
	}
}
