package main

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func main() {
	source := `package test
type Source struct {
	Data string
}
func TestField(s Source) {
	data := s.Data
	_ = data
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", []byte(source), parser.ParseComments)
	if err != nil {
		panic(err)
	}
	files := []*ast.File{file}

	pkg := types.NewPackage("test", "")
	ssaPkg, _, err := ssautil.BuildPackage(&types.Config{Importer: importer.Default()}, fset, pkg, files, ssa.SanityCheckFunctions)
	// TODO: look at final SSA result
	if err != nil {
		panic(err)
	}

	_ = ssaPkg
}
