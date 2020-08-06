package node

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// CanonicalName produces a canonical string representation for an SSA node.
func CanonicalName(n ssa.Node) string {
	var target, op string
	value, isValue := n.(ssa.Value)
	instr, isInstr := n.(ssa.Instruction)
	if isInstr {
		op = instr.String()
	}
	if isValue {
		if isInstr {
			target = fmt.Sprintf("%s = ", value.Name())
		} else {
			target = value.Name()
		}
	}
	return target + op
}

// TrimmedType returns the type of a node without the "*.ssa" prefix.
func TrimmedType(n ssa.Node) string {
	t := fmt.Sprintf("%T", n)
	return strings.TrimPrefix(t, "*ssa.")
}
