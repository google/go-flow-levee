# How to Contribute

We'd love to accept your patches and contributions to this project. There are
just a few small guidelines you need to follow.

## Contributor License Agreement

Contributions to this project must be accompanied by a Contributor License
Agreement. You (or your employer) retain the copyright to your contribution;
this simply gives us permission to use and redistribute your contributions as
part of the project. Head over to <https://cla.developers.google.com/> to see
your current agreements on file or to sign a new one.

You generally only need to submit a CLA once, so if you've already submitted one
(even if it was for a different project), you probably don't need to do it
again.

## Code reviews

All submissions, including submissions by project members, require review. We
use GitHub pull requests for this purpose. Consult
[GitHub Help](https://help.github.com/articles/about-pull-requests/) for more
information on using pull requests.

## Community Guidelines

This project follows [Google's Open Source Community
Guidelines](https://opensource.google/conduct/).

## Project Conventions

### Use Go Modules in `testdata`

The analyzer testing package `analysistest` executes a build of the Go code it tests.
This treats the `testdata` folder as a synthetic GOPATH, and Go code tested within will look for imported packages in `testdata/src/...`.
Unfortunately, this tends to break IDE linking, making test development difficult.

To ameliorate this issue, we group a given test's `testdata` into a synthetic root package and use Go modules to identify it for the IDE.
This allows your IDE to correctly link imports while adhering to the expectation of `analysistest`.

Specifically, `testdata` should be grouped under `testdata/src/NAME_analysistest`, where `NAME` is the name of the analyzer being tested.
Initialize a trivial Go module file with `go mod init NAME_analysistest`.
Any imports used within this `testdata` will begin with `NAME_analysistest`.
See [`levee_analysistest`](internal/pkg/levee/testdata/src/levee_analysistest) for an example. 
