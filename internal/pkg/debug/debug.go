package debug

import (
	"github.com/google/go-flow-levee/internal/pkg/debug/dump"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
)

var Analyzer = &analysis.Analyzer{
	Name:     "debug",
	Run:      run,
	Doc:      "dumps SSA and DOT source for every function",
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

	for _, f := range ssaInput.SrcFuncs {
		pkgName := f.Pkg.Pkg.Name()
		dump.SSA(pkgName, f)
		dump.DOT(pkgName, f)
	}

	return nil, nil
}
