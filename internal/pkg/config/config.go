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
	"reflect"
	"strconv"
	"strings"
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

type Matcher interface {
	MatchPkg(path string) bool
	MatchType(path, typeName string) bool
	MatchField(path, typeName, fieldName string) bool
	MatchFunction(path, receiver, name string) bool
}

// config contains matchers and analysis scope information
type Config struct {
	Sources    []sourceMatcher
	Sinks      []funcMatcher
	Sanitizers []funcMatcher
	FieldTags  []fieldTagMatcher
	Exclude    []funcMatcher
}

type fieldTagMatcher struct {
	Key string
	Val string
}

// IsSourceFieldTag determines whether a field tag made up of a key and value
// is a Source.
func (c Config) IsSourceFieldTag(tag string) bool {
	if unq, err := strconv.Unquote(tag); err == nil {
		tag = unq
	}
	st := reflect.StructTag(tag)

	// built in
	if st.Get("levee") == "source" {
		return true
	}
	// configured
	for _, ft := range c.FieldTags {
		val := st.Get(ft.Key)
		for _, v := range strings.Split(val, ",") {
			if v == ft.Val {
				return true
			}
		}
	}
	return false
}

// IsExcluded determines if a function matches one of the exclusion patterns.
func (c Config) IsExcluded(path string, recv string, name string) bool {
	for _, pm := range c.Exclude {
		if pm.MatchFunction(path, recv, name) {
			return true
		}
	}
	return false
}

func (c Config) IsSink(path, recv, name string) bool {
	for _, p := range c.Sinks {
		if p.MatchFunction(path, recv, name) {
			return true
		}
	}
	return false
}

func (c Config) IsSanitizer(path, recv, name string) bool {
	for _, p := range c.Sanitizers {
		if p.MatchFunction(path, recv, name) {
			return true
		}
	}
	return false
}

// DecompoeType returns the path, typename, and indicators for if the Type is Named or an Interface
// Returns empty strings if the type is not *types.Named
func DecomposeType(t types.Type) (path, name string) {
	n, ok := t.(*types.Named)
	if !ok {
		return
	}

	return n.Obj().Pkg().Path(), n.Obj().Name()
}

func (c Config) IsSource(path string, name string) bool {
	for _, p := range c.Sources {
		if p.MatchType(path, name) {
			return true
		}
	}
	return false
}

func (c Config) IsSourceField(typ types.Type, fld *types.Var) bool {
	n, ok := typ.(*types.Named)
	if !ok || types.IsInterface(n) {
		return false
	}

	path, typeName, fieldName := n.Obj().Pkg().Path(), n.Obj().Name(), fld.Name()
	for _, p := range c.Sources {
		if p.MatchField(path, typeName, fieldName) {
			return true
		}
	}
	return false
}

func (c Config) IsSourceFieldAddr(fa *ssa.FieldAddr) bool {
	// fa.Type() refers to the accessed field's type.
	// fa.X.Type() refers to the surrounding struct's type.
	deref := utils.Dereference(fa.X.Type())
	n, ok := deref.(*types.Named)
	if !ok || types.IsInterface(n) {
		return false
	}
	path, typeName := n.Obj().Pkg().Path(), n.Obj().Name()
	fieldName := utils.FieldName(fa)

	for _, p := range c.Sources {
		if p.MatchField(path, typeName, fieldName) {
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

func (s sourceMatcher) MatchPkg(path string) bool {
	return s.PackageRE.MatchString(path)
}

func (s sourceMatcher) MatchType(path, typeName string) bool {
	return s.MatchPkg(path) && s.TypeRE.MatchString(typeName)
}

func (s sourceMatcher) MatchField(path, typeName, fieldName string) bool {
	return s.MatchType(path, typeName) && s.FieldRE.MatchString(fieldName)
}

// sourceMatchers do not match functions
func (s sourceMatcher) MatchFunction(path, receiver, name string) bool {
	return false
}

type funcMatcher struct {
	PackageRE  regexp.Regexp
	ReceiverRE regexp.Regexp
	MethodRE   regexp.Regexp
}

func (fm funcMatcher) MatchPkg(path string) bool {
	return fm.PackageRE.MatchString(path)
}

func (fm funcMatcher) MatchType(path, typeName string) bool {
	return fm.MatchPkg(path) && fm.ReceiverRE.MatchString(typeName)
}

func (fm funcMatcher) MatchField(path, typeName, fieldName string) bool {
	return false
}

// sourceMatchers do not match functions
func (fm funcMatcher) MatchFunction(path, receiver, name string) bool {
	return fm.MatchType(path, receiver) && fm.MethodRE.MatchString(name)
}

func unqualifiedName(v *types.Var) string {
	packageQualifiedName := v.Type().String()
	dotPos := strings.LastIndexByte(packageQualifiedName, '.')
	if dotPos == -1 {
		return packageQualifiedName
	}
	return packageQualifiedName[dotPos+1:]
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

// DecomposeFunction returns the path, receiver, and name strings of a ssa.Function.
// For functions that have no receiver, returns an empty string for recv.
// If f is nil, returns empty strings for all return values.
func DecomposeFunction(f *ssa.Function) (path, recv, name string) {
	if f == nil {
		return
	}

	path = f.Pkg.Pkg.Path()
	name = f.Name()
	recvVar := f.Signature.Recv()
	if recvVar != nil {
		recv = unqualifiedName(recvVar)
	}
	return
}
