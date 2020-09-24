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

	"golang.org/x/tools/go/ssa"
)

// CFG renders DOT source representing the function's control flow graph (CFG).
func CFG(f *ssa.Function) string {
	blockIndex := map[*ssa.BasicBlock]int{}
	for i, b := range f.Blocks {
		blockIndex[b] = i
	}

	var b strings.Builder
	b.WriteString("digraph {\n")
	for _, block := range f.Blocks {
		b.WriteString(fmt.Sprintf("\t%q\n", blockLabel(block, blockIndex)))
		for _, succ := range block.Succs {
			b.WriteString(fmt.Sprintf("\t%q -> %q;\n", blockLabel(block, blockIndex), blockLabel(succ, blockIndex)))
		}
	}
	b.WriteString("}")
	return b.String()
}

func blockLabel(b *ssa.BasicBlock, blockIndex map[*ssa.BasicBlock]int) string {
	return fmt.Sprintf("%d %s", blockIndex[b], b.Comment)
}
