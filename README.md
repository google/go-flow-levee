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

### Glossary
| Term             | Definition |
|------------------|------------|
| source           | A variable of concern, such as PII that should not be logged
| source container | A type which is expected to contain a source
| source producer  | A function or method which instantiates a source or source container
| propagator       | A function or method which accepts a source or source container as input, returning data which may be "tainted" with source data.  For instance, `fmt.Sprintf("%v", source)` may return a tainted string containing PII.
| taint            | A variable which may contain source data due to a propagator
| sink             | A function or method which should not be called on source or taint arguments.
| sanitizer        | A function or method which "sanitizes" source data, allowing it to safely pass to a sink.

### Configuration

See [configuration/](configuration/README.md) for configuration details.

### How it works

// TODO Describe SSA, taint propagation, pointer analysis

### Automatic source container identification

// TODO -- describe

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
