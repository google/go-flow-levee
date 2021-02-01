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
	"testing"

	"github.com/google/go-flow-levee/internal/pkg/config/regexp"
	"sigs.k8s.io/yaml"
)

func TestFuncMatcherUnmarshalErrorCases(t *testing.T) {
	testCases := []struct {
		desc, yaml string
	}{
		{
			desc: "Unmarshaling is strict",
			yaml: `
Blahblah: foo
PackageRE: bar`,
		},
		{
			desc: "Malformed YAML errors gracefully",
			yaml: `
PackageRE: "No ending quote`,
		},
		{
			desc: "Malformed regexp errors gracefully",
			yaml: `
PackageRE: "(?:NoEndingParen"`,
		},
		{
			desc: "Do not permit both Package and PackageRE",
			yaml: `
Package: foo
PackageRE: bar`,
		},
		{
			desc: "Do not permit both Receiver and ReceiverRE",
			yaml: `
Receiver: foo
ReceiverRE: bar`,
		},
		{
			desc: "Do not permit both Field and FieldRE",
			yaml: `
Method: foo
MethodRE: bar`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			fm := funcMatcher{}
			err := yaml.UnmarshalStrict([]byte(tc.yaml), &fm)

			if err == nil {
				t.Error("got err = nil, want error")
			}
		})
	}
}

func TestFuncMatcherMatching(t *testing.T) {
	testCases := []struct {
		desc, yaml       string
		path, recv, name string
		shouldMatch      bool
	}{
		{
			desc: "Literal foo.bar should match foo.bar; no receiver arg",
			yaml: `
Package: foo
Method: bar`,
			path:        "foo",
			recv:        "",
			name:        "bar",
			shouldMatch: true,
		},
		{
			desc: "Literal foo.bar should match foo.(baz).bar; no receiver arg",
			yaml: `
Package: foo
Method: bar`,
			path:        "foo",
			recv:        "baz",
			name:        "bar",
			shouldMatch: true,
		},
		{
			desc: "Literal foo.bar should NOT match foodstuff.bar; no receiver arg",
			yaml: `
Package: foo
Method: bar`,
			path:        "foodstuff",
			recv:        "",
			name:        "bar",
			shouldMatch: false,
		},
		{
			desc: "Mixed regexp and literal matchers are permitted - positive case",
			yaml: `
PackageRE: foo
Receiver: baz
Method: bar`,
			path:        "foodstuff",
			recv:        "baz",
			name:        "bar",
			shouldMatch: true,
		},
		{
			desc: "Mixed regexp and literal matchers are permitted - negative case",
			yaml: `
PackageRE: foo
Receiver: baz
Method: bar`,
			path:        "foodstuff",
			recv:        "bazinga",
			name:        "bar",
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			fm := funcMatcher{}
			if err := yaml.UnmarshalStrict([]byte(tc.yaml), &fm); err != nil {
				t.Errorf("unexpected error unmarshalling funcMatcher: %v", err)
			}

			if tc.shouldMatch != fm.MatchFunction(tc.path, tc.recv, tc.name) {
				t.Errorf("MatchFunction(%q, %q, %q) got %v, want %v; ", tc.path, tc.recv, tc.name, !tc.shouldMatch, tc.shouldMatch)
			}
		})
	}
}

func TestSourceMatcherUnmarshalingErrorCases(t *testing.T) {
	testCases := []struct {
		desc, yaml string
	}{
		{
			desc: "Unmarshaling is strict",
			yaml: `
Blahblah: foo
PackageRE: bar`,
		},
		{
			desc: "Malformed YAML errors gracefully",
			yaml: `
PackageRE: "No ending quote`,
		},
		{
			desc: "Malformed regexp errors gracefully",
			yaml: `
PackageRE: "(?:NoEndingParen"`,
		},
		{
			desc: "Do not permit both Package and PackageRE",
			yaml: `
Package: foo
PackageRE: bar`,
		},
		{
			desc: "Do not permit both Type and TypeRE",
			yaml: `
Type: foo
TypeRE: bar`,
		},
		{
			desc: "Do not permit both Field and FieldRE",
			yaml: `
Field: foo
FieldRE: bar`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			sm := sourceMatcher{}
			err := yaml.UnmarshalStrict([]byte(tc.yaml), &sm)

			if err == nil {
				t.Errorf("got err = nil, want err != nil")
			}
		})
	}
}

func TestSourceMatcherMatching(t *testing.T) {
	testCases := []struct {
		desc, yaml           string
		path, typ, fieldName string
		shouldMatchType      bool
		shouldMatchField     bool
	}{
		{
			desc: "Literal foo.bar (no field args) should match foo.bar and foo.bar.baz",
			yaml: `
Package: foo
Type: bar`,
			path:             "foo",
			typ:              "bar",
			fieldName:        "baz",
			shouldMatchType:  true,
			shouldMatchField: true,
		},
		{
			desc: "Literal foo.bar (no field args) should NOT match foodstuff.bar or foodstuff.bar.baz",
			yaml: `
Package: foo
Type: bar`,
			path:             "foodstuff",
			typ:              "bar",
			fieldName:        "baz",
			shouldMatchType:  false,
			shouldMatchField: false,
		},
		{
			desc: "Regexp foo.bar (no field args) should match foodstuff.bar and foodstuff.bar.baz",
			yaml: `
PackageRE: foo
TypeRE: bar`,
			path:             "foodstuff",
			typ:              "bar",
			fieldName:        "baz",
			shouldMatchType:  true,
			shouldMatchField: true,
		},
		{
			desc: "Mixed regexp and literal matchers are permitted",
			yaml: `
PackageRE: foo
Type: bar
Field: baz`,
			path:             "foodstuff",
			typ:              "bar",
			fieldName:        "baz",
			shouldMatchType:  true,
			shouldMatchField: true,
		},
		{
			desc: "Literal foo.bar.qux should MatchType foo.bar but not MatchField foo.bar.baz",
			yaml: `
PackageRE: foo
Type: bar
Field: qux`,
			path:             "foodstuff",
			typ:              "bar",
			fieldName:        "baz",
			shouldMatchType:  true,
			shouldMatchField: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			sm := sourceMatcher{}
			if err := yaml.UnmarshalStrict([]byte(tc.yaml), &sm); err != nil {
				t.Errorf("Unexpected error unmarshalling sourceMatcher: %v", err)
			}

			if tc.shouldMatchType != sm.MatchType(tc.path, tc.typ) {
				t.Errorf("MatchType(%q, %q) got %v, want %v", tc.path, tc.typ, !tc.shouldMatchType, tc.shouldMatchType)
			}
			if tc.shouldMatchField != sm.MatchField(tc.path, tc.typ, tc.fieldName) {
				t.Errorf("MatchField(%q, %q, %q) got %v, want %v", tc.path, tc.typ, tc.fieldName, !tc.shouldMatchType, tc.shouldMatchType)
			}
		})
	}
}

func TestFieldTagMatcherUnmarshalling(t *testing.T) {
	testCases := []struct {
		desc, yaml string
		wantErr    bool
	}{
		{
			desc:    "missing value",
			yaml:    "key: foo",
			wantErr: true,
		},
		{
			desc:    "missing key",
			yaml:    "value: foo",
			wantErr: true,
		},
		{
			desc: "unknown field is not allowed",
			yaml: `
Key: good
Value: good
UnknownField: bad`,
			wantErr: true,
		},
		{
			desc: "val should be value",
			yaml: `
key: good
val: "two letters short"`,
			wantErr: true,
		},
		{
			desc: "valid field tag config, lowercase",
			yaml: `
key: foo
value: bar`,
			wantErr: false,
		},
		{
			desc: "valid field tag config, titlecase",
			yaml: `
Key: foo
Value: bar`,
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ftm := fieldTagMatcher{}
			err := yaml.UnmarshalStrict([]byte(tc.yaml), &ftm)

			if (err != nil) != tc.wantErr {
				t.Errorf("got err = %v, expect err = %v", err, tc.wantErr)
			}
		})
	}
}

func TestMatcherTypes(t *testing.T) {
	testCases := []struct {
		desc        string
		matcher     stringMatcher
		s           string
		shouldMatch bool
	}{
		{
			desc:        "literal matcher foo == foo",
			matcher:     literalMatcher("foo"),
			s:           "foo",
			shouldMatch: true,
		},
		{
			desc:        "literal matcher foo != food",
			matcher:     literalMatcher("foo"),
			s:           "food",
			shouldMatch: false,
		},
		{
			desc:        "literal matcher foo != bar",
			matcher:     literalMatcher("foo"),
			s:           "bar",
			shouldMatch: false,
		},
		{
			desc:        "regexp matcher /foo/ matches foo",
			matcher:     func() stringMatcher { r, _ := regexp.New("foo"); return r }(),
			s:           "foo",
			shouldMatch: true,
		},
		{
			desc:        "regexp matcher /foo/ matches food",
			matcher:     func() stringMatcher { r, _ := regexp.New("foo"); return r }(),
			s:           "food",
			shouldMatch: true,
		},
		{
			desc:        "regexp matcher /foo/ does not match bar",
			matcher:     func() stringMatcher { r, _ := regexp.New("foo"); return r }(),
			s:           "bar",
			shouldMatch: false,
		},
		{
			desc:        "vacuous matcher matches foo",
			matcher:     vacuousMatcher{},
			s:           "foo",
			shouldMatch: true,
		},
		{
			desc:        "vacuous matcher matches food",
			matcher:     vacuousMatcher{},
			s:           "food",
			shouldMatch: true,
		},
		{
			desc:        "vacuous matcher matches bar",
			matcher:     vacuousMatcher{},
			s:           "bar",
			shouldMatch: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.matcher.MatchString(tc.s) != tc.shouldMatch {
				t.Errorf("matcher (%T) %v returned MatchString(%q) == %v, want %v, ", tc.matcher, tc.matcher, tc.s, !tc.shouldMatch, tc.shouldMatch)
			}
		})
	}
}
