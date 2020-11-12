package config

import (
	"testing"

	"sigs.k8s.io/yaml"
)

func TestFuncMatcher(t *testing.T) {
	testCases := []struct {
		desc, yaml        string
		path, recv, name  string
		shouldErrorOnLoad bool
		shouldMatch       bool
	}{
		{
			desc: "Garbage in garbage out",
			yaml: `
Blahblah: foo
PackageRE: bar`,
			shouldErrorOnLoad: false, // TODO true
		},
		{
			desc: "Do not permit both Package and PackageRE",
			yaml: `
Package: foo
PackageRE: bar`,
			shouldErrorOnLoad: true,
		},
		{
			desc: "Do not permit both Receiver and ReceiverRE",
			yaml: `
Receiver: foo
ReceiverRE: bar`,
			shouldErrorOnLoad: true,
		},
		{
			desc: "Do not permit both Field and FieldRE",
			yaml: `
Method: foo
MethodRE: bar`,
			shouldErrorOnLoad: true,
		},
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
			err := yaml.UnmarshalStrict([]byte(tc.yaml), &fm)

			if tc.shouldErrorOnLoad {
				if err == nil {
					t.Errorf("Expected yaml to fail on load, got err = nil")
				}
				return
			}

			if !tc.shouldErrorOnLoad && err != nil {
				t.Error(err)
				return
			}

			if tc.shouldMatch != fm.MatchFunction(tc.path, tc.recv, tc.name) {
				t.Errorf("MatchFunction(%q, %q, %q) = %v; got %v", tc.path, tc.recv, tc.name, tc.shouldMatch, !tc.shouldMatch)
			}
		})
	}
}

func TestSourceMatcher(t *testing.T) {
	testCases := []struct {
		desc, yaml           string
		path, typ, fieldName string
		shouldErrorOnLoad    bool
		shouldMatchType      bool
		shouldMatchField     bool
	}{
		{
			desc: "Garbage in garbage out",
			yaml: `
Blahblah: foo
PackageRE: bar`,
			shouldErrorOnLoad: false, // TODO true
		},
		{
			desc: "Do not permit both Package and PackageRE",
			yaml: `
Package: foo
PackageRE: bar`,
			shouldErrorOnLoad: true,
		},
		{
			desc: "Do not permit both Type and TypeRE",
			yaml: `
Type: foo
TypeRE: bar`,
			shouldErrorOnLoad: true,
		},
		{
			desc: "Do not permit both Field and FieldRE",
			yaml: `
Field: foo
FieldRE: bar`,
			shouldErrorOnLoad: true,
		},
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
			err := yaml.UnmarshalStrict([]byte(tc.yaml), &sm)

			if tc.shouldErrorOnLoad {
				if err == nil {
					t.Errorf("Expected yaml to fail on load, got err = nil")
				}
				return
			}

			if !tc.shouldErrorOnLoad && err != nil {
				t.Error(err)
				return
			}

			if tc.shouldMatchType != sm.MatchType(tc.path, tc.typ) {
				t.Errorf("MatchType(%q, %q) = %v; got %v", tc.path, tc.typ, tc.shouldMatchType, !tc.shouldMatchType)
			}
			if tc.shouldMatchField != sm.MatchField(tc.path, tc.typ, tc.fieldName) {
				t.Errorf("MatchField(%q, %q, %q) = %v; got %v", tc.path, tc.typ, tc.fieldName, tc.shouldMatchType, !tc.shouldMatchType)
			}
		})
	}
}
