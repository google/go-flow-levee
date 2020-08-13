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

// Package node contains utility functions for working with SSA nodes.
package node

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// CanonicalName produces a canonical string representation for an SSA node.
func CanonicalName(n ssa.Node) string {
	value, isValue := n.(ssa.Value)
	instr, isInstr := n.(ssa.Instruction)
	switch {
	case isValue && isInstr:
		return fmt.Sprintf("%s = %s", value.Name(), instr.String())
	case isValue:
		return value.Name()
	case isInstr:
		return instr.String()
	default:
		member, isMember := n.(ssa.Member)
		if !isMember {
			return ""
		}
		return member.Name()
	}
}

// TrimmedType returns the type of a node without the "*.ssa" prefix.
func TrimmedType(n ssa.Node) string {
	t := fmt.Sprintf("%T", n)
	return strings.TrimPrefix(t, "*ssa.")
}
