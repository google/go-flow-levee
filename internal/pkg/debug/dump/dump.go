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
