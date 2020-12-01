# Developing Go Flow Levee

## Debugging

The main analyzer depends heavily on the [golang.org/x/tools/ssa](https://pkg.go.dev/golang.org/x/tools/ssa) package. Being able to read the SSA code and visualize its graph can be very useful for debugging.

Currently, debugging is only available in the tests for the `levee` package. To obtain the debugging output, run this command: `go test ./... -debug` from the `internal/pkg/levee` directory. The output files are written to a directory named `output` in the root of the repository.

Given a function named `MyFunc` in a package named `mypack`, running `levee`'s tests with `-debug` produces the following files:
* `mypack_MyFunc.ssa`: the SSA code, in a similar fashion to `golang.org/x/tools/cmd/ssadump`
* `mypack_MyFunc.dot`: the DOT ([graphviz](https://graphviz.org/)) code for the function's SSA Operands and Referrers graph
* `mypack_MyFunc-cfg.dot`: the DOT code for the function's [control-flow graph (CFG)](https://en.wikipedia.org/wiki/Control-flow_graph)

You can generate a PDF from the DOT source using `dot -Tpdf <file> -o "$(basename <file> .dot).pdf"`.

In the graph:
* An **orange** edge points to an **Operand** of an `ssa.Node`
* A **red** edge points to a **Referrer** of an `ssa.Node`
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
