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

## User Guide

See `guides/` for guided introductions on:
* [How to configure and run the analyzer](guides/quickstart.md)
* More topics coming soon!

## Motivation

Much data should not be freely shared.
For instance, secrets (e.g, OAuth tokens, passwords),
  personally identifiable information (e.g., name, email or mailing address),
  and other sensitive information (e.g., user payment info, information regulated by law)
  should typically be serialized only when necessary and should almost never be logged.
However, as a program's type hierarchy becomes more complex or
  as program logic grows to warrant increasingly detailed logging,
  it is easy to overlook when a class might contain these sensitive data and
  which log statements might accidentally expose them.

## Technical design

See [design/](design/README.md).

## Configuration

See [configuration/](configuration/README.md) for configuration details.

## Reporting bugs

Static taint propagation analysis is a hard problem. In fact, it is [undecidable](https://en.wikipedia.org/wiki/Rice%27s_theorem). Concretely, this means two things:
* False negatives: the analyzer may fail to recognize that a piece of code is unsafe.
* False positives: the analyzer may incorrectly claim that a safe piece of code is unsafe. 

Since taint propagation is often used as a security safeguard, we care more deeply about false negatives. If you discover unsafe code that the analyzer is not recognizing as unsafe, please open an issue [here](https://github.com/google/go-flow-levee/issues/new?template=false-negative.md). Conversely, false positives waste developer time and should also be addressed. If the analyzer produces a report for code that you consider to be safe, please open an issue [here](https://github.com/google/go-flow-levee/issues/new?template=false-positive.md).

For general bug reports (e.g. crashes), please open an issue [here](https://github.com/google/go-flow-levee/issues/new?template=bug_report.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## Developing

See [DEVELOPING.md](DEVELOPING.md) for details.

## Disclaimer

This is not an officially supported Google product.
