// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/types"
	"io/ioutil"
	"sync"

	"github.com/google/go-flow-levee/internal/pkg/config/regexp"
	"github.com/google/go-flow-levee/internal/pkg/utils"
	"golang.org/x/tools/go/ssa"
)

// FlagSet should be used by analyzers to reuse -config flag.
var FlagSet flag.FlagSet
var configFile string

func init() {
	FlagSet.StringVar(&configFile, "config", "config.json", "path to analysis configuration file")
}

// config contains matchers and analysis scope information
type Config struct {
	Sources                 []sourceMatcher
	Sinks                   []callMatcher
	Sanitizers              []callMatcher
	TransformingPropagators []transformingPropagatorMatcher
	PropagatorArgs          argumentPropagatorMatcher
	Allowlist               []packageMatcher
	AnalysisScope           []packageMatcher
}

// shouldSkip returns true for any function that is outside analysis scope,
// that is allowlisted,
// whose containing package imports "testing"
// or whose containing package does not import any package containing a source or a sink.
func (c Config) shouldSkip(pkg *types.Package) bool {
	if isTestPkg(pkg) || !c.isInScope(pkg) || c.isAllowlisted(pkg) {
		return true
	}

	// TODO Does this skip packages that own sources/sinks but don't import others?
	for _, im := range pkg.Imports() {
		for _, s := range c.Sinks {
			if s.matchPackage(im) {
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
		if p.Match(call) {
			return true
		}
	}

	return false
}

func (c Config) IsSinkFunction(f *ssa.Function) bool {
	for _, p := range c.Sinks {
		if p.MethodRE.MatchString(f.Name()) {
			return true
		}
	}

	return false
}

func (c Config) IsSanitizer(call *ssa.Call) bool {
	for _, p := range c.Sanitizers {
		if p.Match(call) {
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
		if p.match(n) {
			return true
		}
	}
	return false
}

func (c Config) IsSourceField(typ types.Type, fld *types.Var) bool {
	n, ok := typ.(*types.Named)
	if !ok {
		return false
	}

	for _, p := range c.Sources {
		if p.match(n) && p.FieldRE.MatchString(fld.Name()) {
			return true
		}
	}
	return false
}

func (c Config) IsSourceFieldAddr(fa *ssa.FieldAddr) bool {
	// fa.Type() refers to the accessed field's type.
	// fa.X.Type() refers to the surrounding struct's type.
	deref := utils.Dereference(fa.X.Type())
	fieldName := utils.FieldName(fa)

	n, ok := deref.(*types.Named)
	if !ok {
		return false
	}

	for _, p := range c.Sources {
		if p.match(n) && p.FieldRE.MatchString(fieldName) {
			return true
		}
	}
	return false
}

func (c Config) IsPropagator(call *ssa.Call) bool {
	return c.isTransformingPropagator(call)
}

// A call is a transforming propagator if its name matches a pattern in the config
// and at least one of its arguments is a Source.
func (c Config) isTransformingPropagator(call *ssa.Call) bool {
	for _, p := range c.TransformingPropagators {
		if !p.match(call) {
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

func (c Config) isAllowlisted(pkg *types.Package) bool {
	for _, w := range c.Allowlist {
		if w.match(pkg) {
			return true
		}
	}
	return false
}

func (c Config) isInScope(pkg *types.Package) bool {
	for _, s := range c.AnalysisScope {
		if s.match(pkg) {
			return true
		}
	}
	return false
}

// A sourceMatcher defines what types are or contain sources.
// Within a given type, specific field access can be specified as the actual source data
// via the fieldRE.
type sourceMatcher struct {
	PackageRE regexp.Regexp
	TypeRE    regexp.Regexp
	FieldRE   regexp.Regexp
}

func (s sourceMatcher) match(n *types.Named) bool {
	if types.IsInterface(n) {
		// In our context, both sources and sanitizers are concrete types.
		return false
	}

	return s.PackageRE.MatchString(n.Obj().Pkg().Path()) && s.TypeRE.MatchString(n.Obj().Name())
}

type transformingPropagatorMatcher struct {
	PackageName string
	MethodRE    regexp.Regexp
}

func (t transformingPropagatorMatcher) match(call *ssa.Call) bool {
	if call.Call.StaticCallee() == nil ||
		call.Call.StaticCallee().Pkg == nil ||
		call.Call.StaticCallee().Pkg.Pkg.Path() != t.PackageName {
		return false
	}

	return t.MethodRE.MatchString(call.Call.StaticCallee().Name())
}

type argumentPropagatorMatcher struct {
	ArgumentTypeRE regexp.Regexp
}

type packageMatcher struct {
	PackageNameRE regexp.Regexp
}

func (pm packageMatcher) match(pkg *types.Package) bool {
	return pm.PackageNameRE.MatchString(pkg.Path())
}

type callMatcher struct {
	PackageRE  regexp.Regexp
	ReceiverRE regexp.Regexp
	MethodRE   regexp.Regexp
}

func (r callMatcher) matchPackage(p *types.Package) bool {
	return r.PackageRE.MatchString(p.Path())
}

// Match matches methods based on package, method, and receiver regexp.
// To explicitly match a method with no receiver (i.e., a top-level function),
// provide the ReceiverRE regexp `^$`.
func (r callMatcher) Match(c *ssa.Call) bool {
	callee := c.Call.StaticCallee()
	if callee == nil || callee.Pkg == nil {
		return false
	}

	if !r.matchPackage(callee.Pkg.Pkg) || !r.MethodRE.MatchString(callee.Name()) {
		return false
	}

	recv := c.Call.Signature().Recv()
	var recvName string
	if recv != nil {
		recvName = recv.Type().String()
	}

	return r.ReceiverRE.MatchString(recvName)
}

var readFileOnce sync.Once
var readConfigCached *Config
var readConfigCachedErr error

func ReadConfig() (*Config, error) {
	loadedFromCache := true
	readFileOnce.Do(func() {
		loadedFromCache = false
		c := new(Config)
		bytes, err := ioutil.ReadFile(configFile)
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

func isTestPkg(p *types.Package) bool {
	for _, im := range p.Imports() {
		if im.Name() == "testing" {
			return true
		}
	}
	return false
}
