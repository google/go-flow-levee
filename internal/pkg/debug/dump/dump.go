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

// Package dump contains functions for writing a function's SSA as SSA or DOT source to a file.
package dump

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/go-flow-levee/internal/pkg/debug/render"
	"golang.org/x/tools/go/ssa"
)

// SSA dumps a function's SSA to a file.
func SSA(fileName string, f *ssa.Function) {
	save(fileName, f.Name(), render.SSA(f), "ssa")
}

// DOT dumps DOT source representing the function's SSA graph to a file.
func DOT(fileName string, f *ssa.Function) {
	save(fileName, f.Name(), render.DOT(f), "dot")
}

func save(fileName, funcName, s, ending string) {
	outFile := strings.TrimSuffix(fileName, ".go") + "_" + funcName + "." + ending
	ioutil.WriteFile(filepath.Join(outDir(), outFile), []byte(s), 0666)
}

func outDir() string {
	_, currentFile, _, _ := runtime.Caller(0)
	d := filepath.Join(filepath.Dir(currentFile), "../../../../output")
	os.MkdirAll(d, 0755)
	return d
}
