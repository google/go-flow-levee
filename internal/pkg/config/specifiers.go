package config

import (
	"fmt"

	"github.com/google/go-flow-levee/internal/pkg/config/regexp"
)

// This type marks intended future work
type NotImplemented = interface{}

type FieldSpec struct {
	typeSpec `yaml:",inline"`
	Field    regexp.Regexp
	Tags     []fieldTagMatcher
}

func (fs FieldSpec) MatchName(name string) bool {
	return fs.Field.MatchString(name)
}

func (fs FieldSpec) MatchTag(key, value string) bool {
	for _, ftm := range fs.Tags {
		if ftm.Key == key && ftm.Val == value {
			return true
		}
	}
	return false
}

type typeSpec struct {
	Package regexp.Regexp
	Type    regexp.Regexp
}

func (ts typeSpec) Match(pkg, typ string) bool {
	return ts.Package.MatchString(pkg) && ts.Type.MatchString(typ)
}

type valueSpec struct {
	FieldSpec     `yaml:",inline"` // Match according to field name or tags
	Id            string
	Unless        []valueSpec
	Scope         NotImplemented
	IsReference   NotImplemented
	MatchConstant NotImplemented
}

type callSpec struct {
	typeSpec     // patch package and optional receiver
	Id           string
	FunctionName string
	Arguments    NotImplemented
}

type metaSpec struct {
	ValidateConfig NotImplemented `yaml:"validate-config"`
	Scope          NotImplemented
}

type Specifier struct {
	Value *valueSpec `yaml:"value,omitempty"`
	Call  *callSpec  `yaml:"call,omitempty"`
}

func (s Specifier) validate() error {
	switch {
	case s.Value != nil && s.Call != nil:
		return fmt.Errorf("a specifier should include only a value or call specification")
	case s.Value == nil && s.Call == nil:
		return fmt.Errorf("specifier includes neither a value or call specification")
	}
	return nil
}

// ConfigV2 is a more generic config
type ConfigV2 struct {
	Apiversion string
	Kind       string

	Metadata metaSpec

	Source    []Specifier
	Sink      []Specifier
	Sanitizer []Specifier
	Allowlist []Specifier
}
