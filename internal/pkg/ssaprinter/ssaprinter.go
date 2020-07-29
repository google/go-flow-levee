// Copyright 2020 Google LLC
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

package ssaprinter

import (
	"bytes"
	"fmt"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

type SSAPrinter struct {
	b bytes.Buffer
}

// New returns a new ssaPrinter.
// An ssaPrinter is used to produce a string representation
// of a function's SSA.
func New(fn *ssa.Function) *SSAPrinter {
	s := SSAPrinter{}
	s.writeSignature(fn)
	s.b.WriteString(":\n")
	return &s
}

// Adapted from tools/go/ssa/func.go
func (s *SSAPrinter) writeSignature(fn *ssa.Function) {
	s.b.WriteString("func ")
	sig := fn.Signature
	params := fn.Params
	if recv := sig.Recv(); recv != nil {
		s.b.WriteString("(")
		if n := params[0].Name(); n != "" {
			s.b.WriteString(n)
			s.b.WriteString(" ")
		}
		types.WriteType(&s.b, params[0].Type(), types.RelativeTo(fn.Pkg.Pkg))
		s.b.WriteString(") ")
	}
	s.b.WriteString(fn.Name())
	types.WriteSignature(&s.b, sig, types.RelativeTo(fn.Pkg.Pkg))
}

func (s *SSAPrinter) WriteBlock(blockIndex int, block *ssa.BasicBlock) {
	s.b.WriteString(fmt.Sprintf("\t%d: %s\n", blockIndex, block.Comment))

}

func (s *SSAPrinter) WriteInstr(instrIndex int, instr ssa.Instruction, dst ssa.Value, hasDst bool) {
	var assg string
	if hasDst {
		assg = fmt.Sprintf("%s = ", dst.Name())

	}
	s.b.WriteString(fmt.Sprintf("\t\t%d(%-20T): %s%v\n", instrIndex, instr, assg, instr.String()))
}

func (s *SSAPrinter) String() string {
	return s.b.String()
}
