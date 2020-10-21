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
	"github.com/google/go-flow-levee/internal/pkg/config/regexp"
)

// This type marks intended future work
type NotImplemented = interface{}

// ConfigV2 is a more generic config.
type ConfigV2 struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind       string

	Metadata metaSpec

	Sources    []specifier
	Sinks      []specifier
	Sanitizers []specifier
	Allowlist  []specifier
}

type metaSpec struct {
	ValidateConfig NotImplemented `yaml:"validateConfig"`
	Scope          NotImplemented
}

type valueSpec struct {
	Id            string           // Identifier for this spec
	TypeSpec      `yaml:",inline"` // Match source type by path and type name
	FieldSpec     `yaml:",inline"` // Match field of above type by name or tags
	Scope         NotImplemented   // Match by local/param/global
	IsReference   NotImplemented   // Match by pointer/slice/map
	MatchConstant NotImplemented   // Match explicit value, e.g. "PASSWORD"
	Context       NotImplemented   // Match invocation context
	Unless        []valueSpec      // Exclusion matchers
}

type callSpec struct {
	Id        string           // Identifier for this spec
	TypeSpec  `yaml:",inline"` // Match by package and optional receiver
	Function  string           // Match function/method by name
	Arguments NotImplemented   // Match function invocation by arguments
	Context   NotImplemented   // Match invocation context
	Unless    []callSpec       // Exclusion matchers
}

type specifier struct {
	Value *valueSpec `yaml:",omitempty"`
	Call  *callSpec  `yaml:",omitempty"`
}

type FieldSpec struct {
	Field     regexp.Regexp
	Fieldtags []fieldTagMatcher
}

func (fs FieldSpec) MatchName(name string) bool {
	return fs.Field.MatchString(name)
}

func (fs FieldSpec) MatchTag(key, value string) bool {
	for _, ftm := range fs.Fieldtags {
		if ftm.Key == key && ftm.Val == value {
			return true
		}
	}
	return false
}

type TypeSpec struct {
	Package regexp.Regexp
	Type    regexp.Regexp
}

func (ts TypeSpec) Match(pkg, typ string) bool {
	return ts.Package.MatchString(pkg) && ts.Type.MatchString(typ)
}
