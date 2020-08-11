# Go Flow Levee

This static analysis tool works to ensure your program's data flow does not spill beyond its banks.

An input program's data flow is explored using a combination of pointer analysis,
 static single assignment analysis, and taint analysis.
"Sources" must not reach "sinks" without first passing through a "sanitizer."
Additionally, source data can "taint" neighboring variables during a "propagation" function call,
 such as writing a source to a string.
Such tainted variables also must not reach any sink.

Such analysis can be used to prevent the accidental logging of credentials or personally identifying information,
 defend against maliciously constructed user input, and enforce data communication restrictions between processes.

### Motivation

Much data should not be freely shared.
For instance, secrets (e.g, OAuth tokens, passwords),
  personally identifiable information (e.g., name, email or mailing address),
  and other sensitive information (e.g., user payment info, information regulated by law)
  should typically be serialized only when necessary and should almost never be logged.
However, as a program's type hierarchy becomes more complex or
  as program logic grows to warrant increasingly detailed logging,
  it is easy to overlook when a class might contain these sensitive data and
  which log statements might accidentally expose them.

### Technical design

See [design/](design/README.md).

### Configuration

See [configuration/](configuration/README.md) for configuration details.

### Debugging

The main analyzer depends heavily on the SSA package. Being able to read the SSA code and visualize its graph can be very useful for debugging. In order to generate the SSA code and DOT (graphviz) source for every function in a test, run `go test levee_test.go -debug`. Results are written to the `output` directory. You can generate a PDF from the DOT source using `dot -Tpdf <file> -o "$(basename <file> .dot).pdf"`.

Currently, debugging is only supported for `levee_test.go`. In order to add support for debugging in a new test, first add a debugging flag:
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
* An **orange** edges points to an **Operand** of an `ssa.Node`
* **Diamond**-shaped nodes represent `ssa.Node`s that are both `ssa.Instruction`s and `ssa.Value`s
* **Square**-shaped node reprsent `ssa.Node`s that are only `ssa.Instruction`s

## Source Code Headers

Every file containing source code must include copyright and license
information. This includes any JS/CSS files that you might be serving out to
browsers. (This is to help well-intentioned people avoid accidental copying that
doesn't comply with the license.)

Apache header:

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

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## Disclaimer

This is not an officially supported Google product.
