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

package debug

import (
	"io/ioutil"
	"path/filepath"
)

func writeOut(filename string, content []byte) {
	ioutil.WriteFile(filepath.Join("out", filename), content, 0666)
}

// WriteSSA writes out the ssa from an ssaPrinter to a file.
func WriteSSA(fnName, source string) {
	writeOut(fnName+".ssa", []byte(source))
}

// WriteGraph writes out the DOT source representing a graph to a file.
func WriteGraph(fnName, source string) {
	writeOut(fnName+".dot", []byte(source))
}
