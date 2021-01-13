# Developing Go Flow Levee

## Testing Your Changes Against a Large Codebase

When creating a pull request, you must verify that your changes are safe by running them on a large codebase such as [https://github.com/kubernetes/kubernetes](https://github.com/kubernetes/kubernetes). This is not intended as a replacement for proper automated testing. Running without error on a large codebase provides an additional level of confidence, since a large codebase is likely to contain edge cases that you may not have considered. Indeed, such edge cases have caused failures in the past ([#74](https://github.com/google/go-flow-levee/pull/74), [#143](https://github.com/google/go-flow-levee/pull/143)).

In addition we recommend running master and feature branch versions of levee against k8s and diffing the results to see if your changes have led to some new findings. To make this easier we have created a script that will run both master and your feature branch version of levee against k8s and print the diff:

```bash
./hack/verify-kubernetes.sh
```

Note: The script makes the following assumptions
1. The kubernetes repository on your machine is at `$(GOPATH)/src/k8s.io/kubernetes`.
2. You are running the script from the root of the go-flow-levee repository and have your feature branch checked out.

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

## TODOs

If you introduce a new TODO, please use `TODO(#issue number)` instead of a plain `TODO` or even `TODO(username)`. Having an issue number gives visibility to the TODO and makes it less likely that it will be forgotten.

## Debugging

The main analyzer depends heavily on the [golang.org/x/tools/ssa](https://pkg.go.dev/golang.org/x/tools/ssa) package. Being able to read the SSA code and visualize its graph can be very useful for debugging.

Currently, debugging is only available in the tests for the `levee` package. To obtain the debugging output, run this command: `go test ./... -debug` from the `internal/pkg/levee` directory. The output files are written to a directory named `output` in the root of the repository.

Given a function named `MyFunc` in a package named `mypack`, running `levee`'s tests with `-debug` produces the following files:
* `mypack_MyFunc.ssa`: the SSA code, in a similar fashion to `golang.org/x/tools/cmd/ssadump`
* `mypack_MyFunc.dot`: the DOT ([graphviz](https://graphviz.org/)) code for the function's SSA Operands and Referrers graph
* `mypack_MyFunc-cfg.dot`: the DOT code for the function's [control-flow graph (CFG)](https://en.wikipedia.org/wiki/Control-flow_graph)

You can generate a PDF from the DOT source using `dot -Tpdf <file> -o "$(basename <file> .dot).pdf"`.

In the graph:
* An **orange** edge captures an **operand** relationship. The source node is an **operand** of the destination node. Therefore, you may read an edge from "A" to "B" as "A is an Operand of B".
* Referrer relationships are not explicitly shown in the graph, since they are redundant. Indeed, as per the ssa package documentation for the `Operands` method, the referrers relation is a subset of the operands relation.
* **Rectangle**-shaped nodes represent `ssa.Node`s that are both `ssa.Instruction`s and `ssa.Value`s
* **Diamond**-shaped nodes represent `ssa.Node`s that are only `ssa.Instruction`s
* **Ellipse**-shaped nodes represent `ssa.Node`s that are either only `ssa.Value`s, or are `ssa.Member`s.

In order to add support for debugging in a new package, first add a debugging flag:
```go
var debugging bool = flag.Bool("debug", false, "run the debug analyzer")
```

Then add `debug.Analyzer` as a dependency of the analyzer being tested:
```go
if *debugging {
	Analyzer.Requires = append(Analyzer.Requires, debug.Analyzer)
}
```

## Git workflow

Please follow our preferred [git workflow](GIT_WORKFLOW.md).

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
