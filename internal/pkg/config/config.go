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

	"github.com/google/go-flow-levee/internal/pkg/config/regexp"
)

// FlagSet should be used by analyzers to reuse -config flag.
var FlagSet flag.FlagSet
var configFile string

func init() {
	FlagSet.StringVar(&configFile, "config", "config.json", "path to analysis configuration file")
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

func (c Config) IsSourceType(path string, name string) bool {
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

// A sourceMatcher defines what types are or contain sources.
// Within a given type, specific field access can be specified as the actual source data
// via the fieldRE.
type sourceMatcher struct {
	PackageRE regexp.Regexp
	TypeRE    regexp.Regexp
	FieldRE   regexp.Regexp
}

func (s sourceMatcher) MatchType(path, typeName string) bool {
	return s.PackageRE.MatchString(path) && s.TypeRE.MatchString(typeName)
}

func (s sourceMatcher) MatchField(path, typeName, fieldName string) bool {
	return s.PackageRE.MatchString(path) && s.TypeRE.MatchString(typeName) && s.FieldRE.MatchString(fieldName)
}

type funcMatcher struct {
	PackageRE  regexp.Regexp
	ReceiverRE regexp.Regexp
	MethodRE   regexp.Regexp
}

func (fm funcMatcher) MatchFunction(path, receiver, name string) bool {
	return fm.PackageRE.MatchString(path) && fm.ReceiverRE.MatchString(receiver) && fm.MethodRE.MatchString(name)
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
