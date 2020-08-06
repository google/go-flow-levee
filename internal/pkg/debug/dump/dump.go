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

package dump

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-flow-levee/internal/pkg/debug/render"
	"golang.org/x/tools/go/ssa"
)

// SSA dumps a function's SSA to a file.
func SSA(fileName string, f *ssa.Function) {
	saveSSA(fileName, f.Name(), render.SSA(f))
}

func saveSSA(fileName, funcName, s string) {
	save(fileName, funcName, s, "ssa")
}

// DOT dumps DOT source representing the function's SSA graph to a file.
func DOT(fileName string, f *ssa.Function) {
	saveDOT(fileName, f.Name(), render.DOT(f))
}

func saveDOT(fileName, funcName, s string) {
	save(fileName, funcName, s, "dot")
}

func save(fileName, funcName, s, ending string) {
	outName := strings.TrimSuffix(fileName, ".go") + "_" + funcName + "." + ending
	ioutil.WriteFile(filepath.Join(ensureExists("debug_output"), outName), []byte(s), 0666)
}

func ensureExists(dirName string) string {
	os.MkdirAll(dirName, 0755)
	return dirName
}
