# Developing Go Flow Levee

## TODOs

If you introduce a new `TODO`, please use `TODO(#issue number)` instead of a plain `TODO` or even `TODO(username)`. Having an issue number gives visibility to the TODO and makes it less likely that it will be forgotten.

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
