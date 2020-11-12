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
	"io/ioutil"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"sigs.k8s.io/yaml"

	"github.com/google/go-flow-levee/internal/pkg/config/regexp"
)

// FlagSet should be used by analyzers to reuse -config flag.
var FlagSet flag.FlagSet
var configFile string

func init() {
	FlagSet.StringVar(&configFile, "config", "config.yaml", "path to analysis configuration file")
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
func (c Config) IsExcluded(path, recv, name string) bool {
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

func (c Config) IsSourceType(path, name string) bool {
	for _, p := range c.Sources {
		if p.MatchType(path, name) {
			return true
		}
	}
	return false
}

func (c Config) IsSourceField(path, typeName, fieldName string) bool {
	for _, p := range c.Sources {
		if p.MatchField(path, typeName, fieldName) {
			return true
		}
	}
	return false
}

type stringMatcher interface {
	MatchString(string) bool
}

type literalMatcher string

func (lm literalMatcher) MatchString(s string) bool {
	return string(lm) == s
}

type vacuousMatcher struct{}

func (vacuousMatcher) MatchString(s string) bool {
	return true
}

// Returns the first non-nil matcher.  If all are nil, returns a vacuousMatcher.
func matcherFrom(lm *literalMatcher, r *regexp.Regexp) stringMatcher {
	switch {
	case lm != nil:
		return lm
	case r != nil:
		return r
	default:
		return vacuousMatcher{}
	}
}

// A sourceMatcher matches by package, type, and field.
// Matching may be done against string literals Package, Type, Field,
// or against regexp PackageRE, TypeRE, FieldRE.
type sourceMatcher struct {
	Package stringMatcher
	Type    stringMatcher
	Field   stringMatcher
}

// this type uses the default unmarshaler and mirrors configuration key-value pairs
type rawSourceMatcher struct {
	Package   *literalMatcher
	Type      *literalMatcher
	Field     *literalMatcher
	PackageRE *regexp.Regexp
	TypeRE    *regexp.Regexp
	FieldRE   *regexp.Regexp
}

func (s *sourceMatcher) UnmarshalJSON(bytes []byte) error {
	raw := rawSourceMatcher{}
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return err
	}

	// validation: do not double-specify any attribute with literal and regexp
	if raw.Package != nil && raw.PackageRE != nil {
		return fmt.Errorf("expected only one of Package, PackageRE to be configured")
	}
	if raw.Type != nil && raw.TypeRE != nil {
		return fmt.Errorf("expected only one of Type, TypeRE to be configured")
	}
	if raw.Field != nil && raw.FieldRE != nil {
		return fmt.Errorf("expected only one of Field, FieldRE to be configured")
	}

	// Unpack raw object into sourceMatcher
	*s = sourceMatcher{
		Package: matcherFrom(raw.Package, raw.PackageRE),
		Type:    matcherFrom(raw.Type, raw.TypeRE),
		Field:   matcherFrom(raw.Field, raw.FieldRE),
	}
	return nil
}

func (s sourceMatcher) MatchType(path, typeName string) bool {
	return s.Package.MatchString(path) && s.Type.MatchString(typeName)
}

func (s sourceMatcher) MatchField(path, typeName, fieldName string) bool {
	return s.Package.MatchString(path) && s.Type.MatchString(typeName) && s.Field.MatchString(fieldName)
}

type funcMatcher struct {
	Package  stringMatcher
	Receiver stringMatcher
	Method   stringMatcher
}

// this type uses the default unmarshaler and mirrors configuration key-value pairs
type rawFuncMatcher struct {
	Package    *literalMatcher
	Receiver   *literalMatcher
	Method     *literalMatcher
	PackageRE  *regexp.Regexp
	ReceiverRE *regexp.Regexp
	MethodRE   *regexp.Regexp
}

func (fm *funcMatcher) UnmarshalJSON(bytes []byte) error {
	raw := rawFuncMatcher{}
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return err
	}

	// validation: do not double-specify any attribute with literal and regexp
	if raw.Package != nil && raw.PackageRE != nil {
		return fmt.Errorf("expected at most one of Package, PackageRE to be configured")
	}
	if raw.Receiver != nil && raw.ReceiverRE != nil {
		return fmt.Errorf("expected at most one of Receiver, ReceiverRE to be configured")
	}
	if raw.Method != nil && raw.MethodRE != nil {
		return fmt.Errorf("expected at most one of Method, MethodRE to be configured")
	}

	// Unpack raw object into funcMatcher
	*fm = funcMatcher{
		Package:  matcherFrom(raw.Package, raw.PackageRE),
		Receiver: matcherFrom(raw.Receiver, raw.ReceiverRE),
		Method:   matcherFrom(raw.Method, raw.MethodRE),
	}
	return nil
}

func (fm funcMatcher) MatchFunction(path, receiver, name string) bool {
	return fm.Package.MatchString(path) && fm.Receiver.MatchString(receiver) && fm.Method.MatchString(name)
}

var readFileOnce sync.Once
var readConfigCached *Config
var readConfigCachedErr error

func ReadConfig() (*Config, error) {
	readFileOnce.Do(func() {
		c := new(Config)
		bytes, err := ioutil.ReadFile(configFile)
		if err != nil {
			readConfigCachedErr = fmt.Errorf("error reading analysis config: %v", err)
			return
		}

		if err := yaml.UnmarshalStrict(bytes, c); err != nil {
			readConfigCachedErr = err
			return
		}
		readConfigCached = c
	})
	return readConfigCached, readConfigCachedErr
}
