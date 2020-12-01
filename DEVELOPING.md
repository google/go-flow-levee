# Developing Go Flow Levee

## Recommended Reading

Having some knowledge of the following packages is helpful when developing Go Flow Levee:

* [golang.org/x/tools/go/analysis](https://golang.org/x/tools/go/analysis)

The `analysis` package provides a framework for developing Go static analyzers. A good overview of the framework is provided in the `doc.go` file. For concrete examples, the `passes` directory contains many analyzers. In that directory, the `findcall` analyzer is intended as a simple example. The `nilness` analyzer is particularly interesting for our purposes, since it uses the `ssa` package.

* [golang.org/x/tools/go/ssa](https://golang.org/x/tools/go/ssa)

The `ssa` package provides a [Static Single Assignment (SSA)](https://en.wikipedia.org/wiki/Static_single_assignment_form) form for Go code. This is the main code representation used by Go Flow Levee, as it provides a way to track the flow of data in a program. The `doc.go` file provides a good overview of the main concepts. The main types (e.g., instructions, values) are defined in `ssa.go`.

* [golang.org/go/types](https://golang.org/pkg/go/types)

The `types` package provides information about types in a Go program. The most relevant files for our uses are `api.go` and `object.go`. For a deeper understanding, there is a nice tutorial about using the `types` package [here](https://github.com/golang/example/tree/master/gotypes).

* [golang.org/go/ast](https://golang.org/pkg/go/ast)

The `ast` package provides the [Abstract Syntax Tree (AST)](https://en.wikipedia.org/wiki/Abstract_syntax_tree) of Go code. We use this package to answer questions about the structure of the code, e.g. "does this struct definition have tagged fields" (in the `fieldtags` analyzer). You can view a Go program's AST with `gotype -ast <file>`.

Outside of those packages, we often refer to the [Go Spec](https://golang.org/ref/spec) to determine the proper way to handle various Go constructs.

## Debugging

The main analyzer depends heavily on the [golang.org/x/tools/ssa](https://pkg.go.dev/golang.org/x/tools/ssa) package. Being able to read the SSA code and visualize its graph can be very useful for debugging. In order to generate the SSA code and DOT ([graphviz](https://graphviz.org/)) source for every function in a test, run `go test <package> -debug`. Results are written to the `output` directory. You can generate a PDF from the DOT source using `dot -Tpdf <file> -o "$(basename <file> .dot).pdf"`.

Currently, debugging is only supported for the `levee` analyzer. In order to add support for debugging in a new test, first add a debugging flag:
```go
var debugging bool = flag.Bool("debug", false, "run the debug analyzer")
```
Then add `debug.Analyzer` as a dependency of the analyzer being tested:
```go
if *debugging {
	Analyzer.Requires = append(Analyzer.Requires, debug.Analyzer)
}
```

In the `dot` output:
* A **red** edge points to a **Referrer** of an `ssa.Node`
* An **orange** edge points to an **Operand** of an `ssa.Node`
* **Rectangle**-shaped nodes represent `ssa.Node`s that are both `ssa.Instruction`s and `ssa.Value`s
* **Diamond**-shaped nodes represent `ssa.Node`s that are only `ssa.Instruction`s
* **Ellipse**-shaped nodes represent `ssa.Node`s that are either only `ssa.Value`s, or are `ssa.Member`s.

The function's control-flow graph (CFG) is also produced and written in a file named `<function-name>-cfg.dot`.

## Source Code Headers

Every file containing source code or data (e.g., YAML configuration files) must include
the Apache header:

    Copyright 2020 Google LLC

    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at

        https://www.apache.org/licenses/LICENSE-2.0

    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.
