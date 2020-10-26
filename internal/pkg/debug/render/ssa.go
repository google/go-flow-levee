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
	"strconv"
	"strings"

	"github.com/google/go-flow-levee/internal/pkg/debug/node"
	"golang.org/x/tools/go/ssa"
)

// SSA produces a human-readable representation of the SSA code for a function.
func SSA(f *ssa.Function) string {
	var b strings.Builder

	b.WriteString("func ")
	b.WriteString(f.Name())
	b.WriteString(strings.TrimPrefix(f.Signature.String(), "func"))
	b.WriteByte('\n')

	maxBlockLength := 1
	for _, blk := range f.Blocks {
		length := len(strconv.Itoa(len(blk.Instrs)))
		if length > maxBlockLength {
			maxBlockLength = length
		}
	}

	for i, blk := range f.DomPreorder() {
		renderBlock(&b, i, blk, maxBlockLength)
	}

	for _, af := range f.AnonFuncs {
		b.WriteString(SSA(af))
	}

	return b.String()
}

func renderBlock(b *strings.Builder, i int, blk *ssa.BasicBlock, instructionIndexWidth int) {
	b.WriteString(fmt.Sprintf("%d: %s\n", i, blk.Comment))
	for j, instr := range blk.Instrs {
		s := node.CanonicalName(instr.(ssa.Node))
		b.WriteByte('\t')
		b.WriteString(fmt.Sprintf("%"+strconv.Itoa(instructionIndexWidth)+"d", j))
		b.WriteString(fmt.Sprintf("(%-20T): ", instr))
		b.WriteString(s)
		b.WriteByte('\n')
	}
}
