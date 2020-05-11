package matcher

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
	"google.com/go-flow-levee/internal/pkg/config/regexp"
	"google.com/go-flow-levee/internal/pkg/utils"
)

// A SourceMatcher defines what types are or contain sources.
// Within a given type, specific field access can be specified as the actual source data
// via the fieldRE.
type SourceMatcher struct {
	PackageRE regexp.Regexp
	TypeRE    regexp.Regexp
	FieldRE   regexp.Regexp
}

func (s SourceMatcher) Match(n *types.Named) bool {
	if types.IsInterface(n) {
		// In our context, both sources and sanitizers are concrete types.
		return false
	}

	return s.PackageRE.MatchString(n.Obj().Pkg().Path()) && s.TypeRE.MatchString(n.Obj().Name())
}

type FieldPropagatorMatcher struct {
	Receiver   string
	AccessorRE regexp.Regexp
}

func (f FieldPropagatorMatcher) Match(call *ssa.Call) bool {
	if call.Call.StaticCallee() == nil {
		return false
	}

	recv := call.Call.Signature().Recv()
	if recv == nil {
		return false
	}

	if f.Receiver != utils.Dereference(recv.Type()).String() {
		return false
	}

	return f.AccessorRE.MatchString(call.Call.StaticCallee().Name())
}

type TransformingPropagatorMatcher struct {
	PackageName string
	MethodRE    regexp.Regexp
}

func (t TransformingPropagatorMatcher) Match(call *ssa.Call) bool {
	if call.Call.StaticCallee() == nil ||
		call.Call.StaticCallee().Pkg == nil ||
		call.Call.StaticCallee().Pkg.Pkg.Path() != t.PackageName {
		return false
	}

	return t.MethodRE.MatchString(call.Call.StaticCallee().Name())
}

type ArgumentPropagatorMatcher struct {
	ArgumentTypeRE regexp.Regexp
}

type PackageMatcher struct {
	PackageNameRE regexp.Regexp
}

func (pm PackageMatcher) Match(pkg *types.Package) bool {
	return pm.PackageNameRE.MatchString(pkg.Path())
}

type NameMatcher struct {
	PackageRE regexp.Regexp
	TypeRE    regexp.Regexp
	MethodRE  regexp.Regexp
}

func (r NameMatcher) MatchPackage(p *types.Package) bool {
	return r.PackageRE.MatchString(p.Path())
}

func (r NameMatcher) MatchMethodName(c *ssa.Call) bool {
	if c.Call.StaticCallee() == nil || c.Call.StaticCallee().Pkg == nil {
		return false
	}

	return r.MatchPackage(c.Call.StaticCallee().Pkg.Pkg) &&
		r.MethodRE.MatchString(c.Call.StaticCallee().Name())
}

func (r NameMatcher) matchNamedType(n *types.Named) bool {
	if types.IsInterface(n) {
		// In our context, both sources and sanitizers are concrete types.
		return false
	}

	return r.PackageRE.MatchString(n.Obj().Pkg().Path()) &&
		r.TypeRE.MatchString(n.Obj().Name())
}
