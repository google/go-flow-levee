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

package graphprinter

import (
	"bytes"
	"fmt"
)

// Print renders a graph as DOT source code.
func Print(graph map[string]map[string]bool, isSource, isSanitizer, isSink func(string) bool) string {
	var b bytes.Buffer

	b.WriteString("digraph {\n")

	for src, neighbors := range graph {
		if isSource(src) {
			b.WriteString(fmt.Sprintf("%q [style=filled fillcolor=red];\n", src))
		}
		for dst := range neighbors {
			if isSanitizer(dst) {
				b.WriteString(fmt.Sprintf("%q [style=filled fillcolor=green];\n", dst))
			}
			if isSink(dst) {
				b.WriteString(fmt.Sprintf("%q [style=filled fillcolor=blue];\n", dst))

			}
			b.WriteString(fmt.Sprintf("%q -> %q;\n", src, dst))
		}
	}

	b.WriteString("}\n")

	return b.String()
}
