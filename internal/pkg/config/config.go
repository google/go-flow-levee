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

var (
	// FlagSet should be used by analyzers to reuse -config flag.
	FlagSet    flag.FlagSet
	configFile string
)

func init() {
	FlagSet.StringVar(&configFile, "config", "config.yaml", "path to analysis configuration file")
}

// Config contains matchers and analysis scope information.
type Config struct {
	ReportMessage             string
	Sources                   []sourceMatcher
	Sinks                     []funcMatcher
	Sanitizers                []funcMatcher
	FieldTags                 []fieldTagMatcher
	Exclude                   []funcMatcher
	AllowPanicOnTaintedValues bool
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
	for _, exc := range c.Exclude {
		if exc.MatchFunction(path, recv, name) {
			return true
		}
	}
	return false
}

// IsSink determines whether a function is a sink.
func (c Config) IsSink(path, recv, name string) bool {
	for _, sink := range c.Sinks {
		if sink.MatchFunction(path, recv, name) {
			return true
		}
	}
	return false
}

// IsSanitizer determines whether a function is a sanitizer.
func (c Config) IsSanitizer(path, recv, name string) bool {
	for _, san := range c.Sanitizers {
		if san.MatchFunction(path, recv, name) {
			return true
		}
	}
	return false
}

// IsSourceType determines whether a type is a source.
func (c Config) IsSourceType(path, name string) bool {
	for _, source := range c.Sources {
		if source.MatchType(path, name) {
			return true
		}
	}
	return false
}

// IsSourceField determines whether a field is a source.
func (c Config) IsSourceField(path, typeName, fieldName string) bool {
	for _, source := range c.Sources {
		if source.MatchField(path, typeName, fieldName) {
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

func (vacuousMatcher) MatchString(string) bool {
	return true
}

type fieldTagMatcher struct {
	Key string
	Val string
}

func (ft *fieldTagMatcher) UnmarshalJSON(bytes []byte) error {
	raw := map[string]string{}
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return err
	}
	for key, val := range raw {
		switch strings.ToLower(key) {
		case "key":
			if ft.Key != "" {
				return fmt.Errorf("Multiple values given for 'key'.")
			}
			ft.Key = val
		case "val", "value":
			if ft.Key != "" {
				return fmt.Errorf("Multiple values given for 'value'.")
			}
			ft.Val = val
		default:
			return fmt.Errorf("got unexpected key %q in field tag configuration", key)
		}
	}

	if ft.Key == "" || ft.Val == "" {
		// TODO test error case
		return fmt.Errorf("expected nonempty key and value, got key %q and value %q", ft.Key, ft.Val)
	}
	return nil
}

// A sourceMatcher matches by package, type, and field.
// Matching may be done against string literals Package, Type, Field,
// or against regexp PackageRE, TypeRE, FieldRE.
type sourceMatcher struct {
	Package stringMatcher
	Type    stringMatcher
	Field   stringMatcher
}

func (s *sourceMatcher) UnmarshalJSON(bytes []byte) error {
	// Unspecified matchers match vacuously
	s.Package = vacuousMatcher{}
	s.Type = vacuousMatcher{}
	s.Field = vacuousMatcher{}

	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return err
	}

	for key, msg := range raw {
		var matcher stringMatcher
		matcher, err := s.makeMatcher(key, msg)
		if err != nil {
			return err
		}

		switch strings.ToLower(key) {
		case "package", "packagere":
			if _, isUnset := s.Package.(vacuousMatcher); !isUnset {
				return fmt.Errorf("only one \"package\", \"packageRE\" of should be provided to configuration")
			}
			s.Package = matcher
		case "type", "typere":
			if _, isUnset := s.Type.(vacuousMatcher); !isUnset {
				return fmt.Errorf("only one \"type\", \"typeRE\" of should be provided to configuration")
			}
			s.Type = matcher
		case "field", "fieldre":
			if _, isUnset := s.Field.(vacuousMatcher); !isUnset {
				return fmt.Errorf("only one \"field\", \"fieldRE\" of should be provided to configuration")
			}
			s.Field = matcher
		default:
			return fmt.Errorf("got unexpected key %q in source matcher configuration", key)
		}
	}

	// TODO any additional validation checks?  No full-vacuous matchers?
	return nil
}

// makeMatcher instantiates a Regexp matcher for "*RE" keys and literalMatcher for other valid keys.  Keys are not case sensitive.
func (s *sourceMatcher) makeMatcher(key string, msg json.RawMessage) (stringMatcher, error) {
	switch strings.ToLower(key) {
	case "package", "type", "field":
		var l literalMatcher
		if err := json.Unmarshal(msg, &l); err != nil {
			return nil, err
		}
		return l, nil
	case "packagere", "typere", "fieldre":
		var r *regexp.Regexp
		if err := json.Unmarshal(msg, &r); err != nil {
			return nil, err
		}
		return r, nil
	default:
		return nil, fmt.Errorf("got unexpected key %q in source matcher configuration", key)
	}
}

func (s sourceMatcher) MatchType(path, typeName string) bool {
	return s.Package.MatchString(path) && s.Type.MatchString(typeName)
}

func (s sourceMatcher) MatchField(path, typeName, fieldName string) bool {
	return s.MatchType(path, typeName) && s.Field.MatchString(fieldName)
}

type funcMatcher struct {
	Package  stringMatcher
	Receiver stringMatcher
	Method   stringMatcher
}

func (fm *funcMatcher) UnmarshalJSON(bytes []byte) error {
	// Initialize all fields to vacuous matchers
	fm.Package = vacuousMatcher{}
	fm.Receiver = vacuousMatcher{}
	fm.Method = vacuousMatcher{}

	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return err
	}

	for key, msg := range raw {
		matcher, err := fm.makeMatcher(key, msg)
		if err != nil {
			return err
		}

		switch strings.ToLower(key) {
		case "package", "packagere":
			if _, isUnset := fm.Package.(vacuousMatcher); !isUnset {
				return fmt.Errorf("only one \"package\", \"packageRE\" of should be provided to configuration")
			}
			fm.Package = matcher
		case "receiver", "receiverre":
			if _, isUnset := fm.Receiver.(vacuousMatcher); !isUnset {
				return fmt.Errorf("only one \"receiver\", \"receiverRE\" of should be provided to configuration")
			}
			fm.Receiver = matcher
		case "method", "methodre":
			if _, isUnset := fm.Method.(vacuousMatcher); !isUnset {
				return fmt.Errorf("only one \"method\", \"methodRE\" of should be provided to configuration")
			}
			fm.Method = matcher
		default:
			return fmt.Errorf("got unexpected key %q in (funcMatcher).UnmarshalJSON", key)
		}
	}

	// TODO any additional validation checks?  No full-vacuous matchers?
	return nil
}

// makeMatcher instantiates a Regexp matcher for "*RE" keys and literalMatcher for other valid keys.  Keys are not case sensitive.
func (fm *funcMatcher) makeMatcher(key string, msg json.RawMessage) (stringMatcher, error) {
	switch strings.ToLower(key) {
	case "package", "receiver", "method":
		var l literalMatcher
		if err := json.Unmarshal(msg, &l); err != nil {
			return nil, err
		}
		return l, nil
	case "packagere", "receiverre", "methodre":
		var r *regexp.Regexp
		if err := json.Unmarshal(msg, &r); err != nil {
			return nil, err
		}
		return r, nil
	default:
		return nil, fmt.Errorf("got unexpected key %q in function matcher configuration", key)
	}
}

func (fm funcMatcher) MatchFunction(path, receiver, name string) bool {
	return fm.Package.MatchString(path) && fm.Receiver.MatchString(receiver) && fm.Method.MatchString(name)
}

// ReadConfig fetches configuration from the config cache.
// The cache reads, parses, and validates config file if necessary.
func ReadConfig() (*Config, error) {
	return cache.read(configFile)
}

// configCacheElement reduces disk access across multiple ReadConfig calls.
type configCacheElement struct {
	once       sync.Once
	conf       *Config
	err        error
	sourceFile string
}

func (r *configCacheElement) readOnce() (*Config, error) {
	r.once.Do(func() {
		c := new(Config)
		bytes, err := ioutil.ReadFile(r.sourceFile)
		if err != nil {
			r.err = fmt.Errorf("error reading analysis config: %v", err)
			return
		}

		if err := yaml.UnmarshalStrict(bytes, c); err != nil {
			r.err = err
			return
		}
		r.conf = c
	})

	return r.conf, r.err
}

// configCache safely stores a configCacheElement per source file for concurrent access.
type configCache struct {
	mu    sync.Mutex
	cache map[string]*configCacheElement
}

// Instantiates configCacheElement if file has not yet been loaded
func (w *configCache) read(file string) (*Config, error) {
	return w.getCacheForFile(file).readOnce()
}

func (w *configCache) getCacheForFile(file string) *configCacheElement {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, ok := w.cache[file]; !ok {
		w.cache[file] = &configCacheElement{sourceFile: file}
	}
	return w.cache[file]
}

var cache = configCache{
	cache: make(map[string]*configCacheElement),
}
