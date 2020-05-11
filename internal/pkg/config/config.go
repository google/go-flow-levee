package config

import (
	"encoding/json"
	"fmt"
	"go/types"
	"io/ioutil"
	"sync"

	"golang.org/x/tools/go/ssa"
	"google.com/go-flow-levee/internal/pkg/matcher"
	"google.com/go-flow-levee/internal/pkg/utils"
)

// config contains matchers and analysis scope information
type Config struct {
	Sources                 []matcher.SourceMatcher
	Sinks                   []matcher.NameMatcher
	Sanitizers              []matcher.NameMatcher
	FieldPropagators        []matcher.FieldPropagatorMatcher
	TransformingPropagators []matcher.TransformingPropagatorMatcher
	PropagatorArgs          matcher.ArgumentPropagatorMatcher
	Whitelist               []matcher.PackageMatcher
	AnalysisScope           []matcher.PackageMatcher
}

// shouldSkip returns true for any function that is outside analysis scope,
// that is whitelisted,
// whose containing package imports "testing"
// or whose containing package does not import any package containing a source or a sink.
func (c Config) shouldSkip(pkg *types.Package) bool {
	if isTestPkg(pkg) || !c.isInScope(pkg) || c.isWhitelisted(pkg) {
		return true
	}

	// TODO Does this skip packages that own sources/sinks but don't import others?
	for _, im := range pkg.Imports() {
		for _, s := range c.Sinks {
			if s.MatchPackage(im) {
				return false
			}
		}

		for _, s := range c.Sources {
			if s.PackageRE.MatchString(im.Path()) {
				return false
			}
		}
	}

	return true
}

func (c Config) IsSink(call *ssa.Call) bool {
	for _, p := range c.Sinks {
		if p.MatchMethodName(call) {
			return true
		}
	}

	return false
}

func (c Config) IsSanitizer(call *ssa.Call) bool {
	for _, p := range c.Sanitizers {
		if p.MatchMethodName(call) {
			return true
		}
	}

	return false
}

func (c Config) IsSource(t types.Type) bool {
	n, ok := t.(*types.Named)
	if !ok {
		return false
	}

	for _, p := range c.Sources {
		if p.Match(n) {
			return true
		}
	}
	return false
}

func (c Config) IsSourceFieldAddr(fa *ssa.FieldAddr) bool {
	// fa.Type() refers to the accessed field's type.
	// fa.X.Type() refers to the surrounding struct's type.

	deref := utils.Dereference(fa.X.Type())
	st, ok := deref.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	fieldName := st.Field(fa.Field).Name()

	for _, p := range c.Sources {
		if n, ok := deref.(*types.Named); ok &&
			p.Match(n) && p.FieldRE.MatchString(fieldName) {
			return true
		}
	}
	return false
}

func (c Config) IsPropagator(call *ssa.Call) bool {
	return c.isFieldPropagator(call) || c.isTransformingPropagator(call)
}

func (c Config) isFieldPropagator(call *ssa.Call) bool {
	recv := call.Call.Signature().Recv()
	if recv == nil {
		return false
	}

	for _, p := range c.FieldPropagators {
		if p.Match(call) {
			return true
		}
	}

	return false
}

func (c Config) isTransformingPropagator(call *ssa.Call) bool {
	for _, p := range c.TransformingPropagators {
		if !p.Match(call) {
			continue
		}

		for _, a := range call.Call.Args {
			// TODO Handle ChangeInterface case.
			switch t := a.(type) {
			case *ssa.MakeInterface:
				if c.IsSource(utils.Dereference(t.X.Type())) {
					return true
				}
			case *ssa.Parameter:
				if c.IsSource(utils.Dereference(t.Type())) {
					return true
				}
			}
		}
	}

	return false
}

func (c Config) SendsToIOWriter(call *ssa.Call) ssa.Node {
	if call.Call.Signature().Params().Len() == 0 {
		return nil
	}

	firstArg := call.Call.Signature().Params().At(0)
	if c.PropagatorArgs.ArgumentTypeRE.MatchString(firstArg.Type().String()) {
		if a, ok := call.Call.Args[0].(*ssa.MakeInterface); ok {
			return a.X.(ssa.Node)
		}
	}

	return nil
}

func (c Config) isWhitelisted(pkg *types.Package) bool {
	for _, w := range c.Whitelist {
		if w.Match(pkg) {
			return true
		}
	}
	return false
}

func (c Config) isInScope(pkg *types.Package) bool {
	for _, s := range c.AnalysisScope {
		if s.Match(pkg) {
			return true
		}
	}
	return false
}

func isTestPkg(p *types.Package) bool {
	for _, im := range p.Imports() {
		if im.Name() == "testing" {
			return true
		}
	}
	return false
}

var readFileOnce sync.Once
var readConfigCached *Config
var readConfigCachedErr error

func FromFile(path string) (*Config, error) {
	loadedFromCache := true
	readFileOnce.Do(func() {
		loadedFromCache = false
		c := new(Config)
		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			readConfigCachedErr = fmt.Errorf("error reading analysis config: %v", err)
			return
		}

		if err := json.Unmarshal(bytes, c); err != nil {
			readConfigCachedErr = err
			return
		}
		readConfigCached = c
	})
	_ = loadedFromCache
	return readConfigCached, readConfigCachedErr
}
