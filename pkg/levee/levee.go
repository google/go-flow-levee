// Package levee exports the levee Analyzer.
package levee

import internal "github.com/google/go-flow-levee/internal/pkg/levee"

// Analyzer reports instances of source data reaching a sink.
var Analyzer = internal.Analyzer
