package main

import (
	"github.com/google/go-flow-levee/pkg/levee"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(levee.Analyzer)
}
