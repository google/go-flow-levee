// Package crosspkg is used to test whether evaluating reachability of sinks
// from sources using pointer analysis works across packages.
package crosspkg

import (
	"example.com/core"
	"example.com/tests/propagators"
)

func main() {
	TestCrossPackageSinkWrapper(core.Source{})
}

func TestCrossPackageSinkWrapper(s core.Source) {
	propagators.SinkWrapper(s) // TODO want "a source has reached a sink, sink:"
}
