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
	for i, blk := range f.Blocks {
		b.WriteString(fmt.Sprintf("%d: %s\n", i, blk.Comment))
		for j, instr := range blk.Instrs {
			s := node.CanonicalName(instr.(ssa.Node))
			b.WriteByte('\t')
			b.WriteString(strconv.Itoa(j))
			b.WriteString(fmt.Sprintf("(%-20T): ", instr))
			b.WriteString(s)
			b.WriteByte('\n')
		}
	}
	return b.String()
}
