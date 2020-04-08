// Copyright 2019 Google LLC
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

package internal

import (
	"encoding/json"
	"fmt"
	"go/types"
	"io/ioutil"
	"math"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/eapache/queue"
)

var configFile string

func init() {
	Analyzer.Flags.StringVar(&configFile, "config", "config.json", "path to analysis configuration file")
}

// config contains matchers and analysis scope information
type config struct {
	Sources                 []sourceMatcher
	Sinks                   []nameMatcher
	Sanitizers              []nameMatcher
	FieldPropagators        []fieldPropagatorMatcher
	TransformingPropagators []transformingPropagatorMatcher
	PropagatorArgs          argumentPropagatorMatcher
	Whitelist               []packageMatcher
	AnalysisScope           []packageMatcher
}

// shouldSkip returns true for any function that is outside analysis scope,
// that is whitelisted,
// whose containing package imports "testing"
// or whose containing package does not import any package containing a source or a sink.
func (c config) shouldSkip(pkg *types.Package) bool {
	if isTestPkg(pkg) || !c.isInScope(pkg) || c.isWhitelisted(pkg) {
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

func (c config) isSink(call *ssa.Call) bool {
	for _, p := range c.Sinks {
		if p.matchMethodName(call) {
			return true
		}
	}

	return false
}

func (c config) isSanitizer(call *ssa.Call) bool {
	for _, p := range c.Sanitizers {
		if p.matchMethodName(call) {
			return true
		}
	}

	return false
}

func (c config) isSource(t types.Type) bool {
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

func (c config) isSourceFieldAddr(fa *ssa.FieldAddr) bool {
	// fa.Type() refers to the accessed field's type.
	// fa.X.Type() refers to the surrounding struct's type.

	deref := dereferenceRecursive(fa.X.Type())
	st, ok := deref.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	fieldName := st.Field(fa.Field).Name()

	for _, p := range c.Sources {
		if n, ok := deref.(*types.Named); ok &&
			p.match(n) && p.FieldRE.MatchString(fieldName) {
			return true
		}
	}
	return false
}

func (c config) isPropagator(call *ssa.Call) bool {
	return c.isFieldPropagator(call) || c.isTransformingPropagator(call)
}

func (c config) isFieldPropagator(call *ssa.Call) bool {
	recv := call.Call.Signature().Recv()
	if recv == nil {
		return false
	}

	for _, p := range c.FieldPropagators {
		if p.match(call) {
			return true
		}
	}

	return false
}

func (c config) isTransformingPropagator(call *ssa.Call) bool {
	for _, p := range c.TransformingPropagators {
		if !p.match(call) {
			continue
		}

		for _, a := range call.Call.Args {
			// TODO Handle ChangeInterface case.
			switch t := a.(type) {
			case *ssa.MakeInterface:
				if c.isSource(dereferenceRecursive(t.X.Type())) {
					return true
				}
			case *ssa.Parameter:
				if c.isSource(dereferenceRecursive(t.Type())) {
					return true
				}
			}
		}
	}

	return false
}

func (c config) sendsToIOWriter(call *ssa.Call) ssa.Node {
	if call.Call.Signature().Params().Len() == 0 {
		return nil
	}

	firstArg := call.Call.Signature().Params().At(0)
	if c.PropagatorArgs.ArgumentTypeRE.MatchString(firstArg.Type().String()) {
		if a, ok := call.Call.Args[0].(*ssa.MakeInterface); ok {
			return a.X.(ssa.Node)
		}
	}

	return nil
}

func (c config) isWhitelisted(pkg *types.Package) bool {
	for _, w := range c.Whitelist {
		if w.match(pkg) {
			return true
		}
	}
	return false
}

func (c config) isInScope(pkg *types.Package) bool {
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
	PackageRE configRegexp
	TypeRE    configRegexp
	FieldRE   configRegexp
}

func (s sourceMatcher) match(n *types.Named) bool {
	if types.IsInterface(n) {
		// In our context, both sources and sanitizers are concrete types.
		return false
	}

	return s.PackageRE.MatchString(n.Obj().Pkg().Path()) && s.TypeRE.MatchString(n.Obj().Name())
}

type fieldPropagatorMatcher struct {
	Receiver   string
	AccessorRE configRegexp
}

func (f fieldPropagatorMatcher) match(call *ssa.Call) bool {
	if call.Call.StaticCallee() == nil {
		return false
	}

	recv := call.Call.Signature().Recv()
	if recv == nil {
		return false
	}

	if f.Receiver != dereferenceRecursive(recv.Type()).String() {
		return false
	}

	return f.AccessorRE.MatchString(call.Call.StaticCallee().Name())
}

type transformingPropagatorMatcher struct {
	PackageName string
	MethodRE    configRegexp
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
	ArgumentTypeRE configRegexp
}

type packageMatcher struct {
	PackageNameRE configRegexp
}

func (pm packageMatcher) match(pkg *types.Package) bool {
	return pm.PackageNameRE.MatchString(pkg.Path())
}

// configRegexp delegates to a Regexp while enabling unmarshalling.
// Any unspecified / nil matcher will return vacuous truth in MatchString
type configRegexp struct {
	r *regexp.Regexp
}

func (mr *configRegexp) MatchString(s string) bool {
	return mr.r == nil || mr.r.MatchString(s)
}

func (mr *configRegexp) UnmarshalJSON(data []byte) error {
	var matcher string
	if err := json.Unmarshal(data, &matcher); err != nil {
		return err
	}

	var err error
	if mr.r, err = regexp.Compile(matcher); err != nil {
		return err
	}
	return nil
}

type nameMatcher struct {
	PackageRE configRegexp
	TypeRE    configRegexp
	MethodRE  configRegexp
}

func (r nameMatcher) matchPackage(p *types.Package) bool {
	return r.PackageRE.MatchString(p.Path())
}

func (r nameMatcher) matchMethodName(c *ssa.Call) bool {
	if c.Call.StaticCallee() == nil || c.Call.StaticCallee().Pkg == nil {
		return false
	}

	return r.matchPackage(c.Call.StaticCallee().Pkg.Pkg) &&
		r.MethodRE.MatchString(c.Call.StaticCallee().Name())
}

func (r nameMatcher) matchNamedType(n *types.Named) bool {
	if types.IsInterface(n) {
		// In our context, both sources and sanitizers are concrete types.
		return false
	}

	return r.PackageRE.MatchString(n.Obj().Pkg().Path()) &&
		r.TypeRE.MatchString(n.Obj().Name())
}

var Analyzer = &analysis.Analyzer{
	Name:     "levee",
	Run:      run,
	Doc:      "reports attempts to source data to sinks",
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

// source represents a source in an SSA call tree.
// It is based on ssa.Node, with the added functionality of computing the recursive graph of
// its referrers.
// source.sanitized notes sanitizer calls that sanitize this source
type source struct {
	node       ssa.Node
	marked     map[ssa.Node]bool
	sanitizers []*sanitizer
}

func newSource(in ssa.Node, config *config) *source {
	a := &source{
		node:   in,
		marked: make(map[ssa.Node]bool),
	}
	a.bfs(config)
	return a
}

// bfs performs Breadth-First-Search on the def-use graph of the input source.
// While traversing the graph we also look for potential sanitizers of this source.
// If the source passes through a sanitizer, bfs does not continue through that Node.
func (a *source) bfs(config *config) {
	q := queue.New()
	q.Add(a.node)
	a.marked[a.node] = true

	for q.Length() > 0 {
		e := q.Remove().(ssa.Node)

		if c, ok := e.(*ssa.Call); ok && config.isSanitizer(c) {
			a.sanitizers = append(a.sanitizers, &sanitizer{call: c})
			continue
		}

		if e.Referrers() == nil {
			continue
		}

		for _, r := range *e.Referrers() {
			if _, ok := a.marked[r.(ssa.Node)]; ok {
				continue
			}
			a.marked[r.(ssa.Node)] = true

			// Need to stay within the scope of the function under analysis.
			if call, ok := r.(*ssa.Call); ok && !config.isPropagator(call) {
				continue
			}

			// Do not follow innocuous field access (e.g. Cluster.Zone)
			if addr, ok := r.(*ssa.FieldAddr); ok && !config.isSourceFieldAddr(addr) {
				continue
			}

			q.Add(r)
		}
	}
}

// hasPathTo returns true when a Node is part of declaration-use graph.
func (a *source) hasPathTo(n ssa.Node) bool {
	return a.marked[n]
}

func (a *source) isSanitizedAt(call ssa.Instruction) bool {
	for _, s := range a.sanitizers {
		if s.dominates(call) {
			return true
		}
	}

	return false
}

type sanitizer struct {
	call *ssa.Call
}

// dominates returns true if the sanitizer dominates the supplied instruction.
// In the context of SSA, domination implies that
// if instructions X executes and X dominates Y, then Y is guaranteed to execute and to be
// executed after X.
func (s sanitizer) dominates(target ssa.Instruction) bool {
	if s.call.Parent() != target.Parent() {
		// Instructions are in different functions.
		return false
	}

	if !s.call.Block().Dominates(target.Block()) {
		return false
	}

	if s.call.Block() == target.Block() {
		parentIdx := math.MaxInt64
		childIdx := 0
		for i, instr := range s.call.Block().Instrs {
			if instr == s.call {
				parentIdx = i
			}
			if instr == target {
				childIdx = i
				break
			}
		}
		return parentIdx < childIdx
	}

	for _, d := range s.call.Block().Dominees() {
		if target.Block() == d {
			return true
		}
	}

	return false
}

// varargs represents a variable length argument.
// Concretely, it abstract over the fact that the varargs internally are represented by an ssa.Slice
// which contains the underlying values of for vararg members.
// Since many sink functions (ex. log.Info, fmt.Errorf) take a vararg argument, being able to
// get the underlying values of the vararg members is important for this analyzer.
type varargs struct {
	slice   *ssa.Slice
	sources []*source
}

// newVarargs constructs varargs. SSA represents varargs as an ssa.Slice.
func newVarargs(s *ssa.Slice, sources []*source) *varargs {
	a, ok := s.X.(*ssa.Alloc)
	if !ok || a.Comment != "varargs" {
		return nil
	}
	var (
		referredSources []*source
	)

	for _, r := range *a.Referrers() {
		idx, ok := r.(*ssa.IndexAddr)
		if !ok {
			continue
		}

		if idx.Referrers() != nil && len(*idx.Referrers()) != 1 {
			continue
		}

		// IndexAddr and Store instructions are inherently linked together.
		// IndexAddr returns an address of an element within a Slice, which is followed by
		// a Store instructions to place a value into the address provided by IndexAddr.
		store := (*idx.Referrers())[0].(*ssa.Store)

		for _, s := range sources {
			if s.hasPathTo(store.Val.(ssa.Node)) {
				referredSources = append(referredSources, s)
				break
			}
		}
	}

	return &varargs{
		slice:   s,
		sources: referredSources,
	}
}

func (v *varargs) referredByCallWithPattern(patterns []nameMatcher) *ssa.Call {
	if v.slice.Referrers() == nil || len(*v.slice.Referrers()) != 1 {
		return nil
	}

	c, ok := (*v.slice.Referrers())[0].(*ssa.Call)
	if !ok || c.Call.StaticCallee() == nil {
		return nil
	}

	for _, p := range patterns {
		if p.matchMethodName(c) {
			return c
		}
	}

	return nil
}

func run(pass *analysis.Pass) (interface{}, error) {
	conf, err := readConfig(configFile)
	if err != nil {
		return nil, err
	}
	// TODO: respect configuration scope

	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)

	sourcesMap := identifySources(conf, ssaInput)

	// Only examine functions that have sources
	for fn, sources := range sourcesMap {
		//log.V(2).Infof("Processing function %v", fn)

		for _, b := range fn.Blocks {
			if b == fn.Recover {
				// TODO Handle calls to sinks in a recovery block.
				continue // skipping Recover since it does not have instructions, rather a single block.
			}

			for _, instr := range b.Instrs {
				//log.V(2).Infof("Inst: %v %T", instr, instr)
				switch v := instr.(type) {
				case *ssa.Call:
					switch {
					case conf.isPropagator(v):
						// Handling the case where sources are propagated to io.Writer
						// (ex. proto.MarshalText(&buf, c)
						// In such cases, "buf" becomes a source, and not the return value of the propagator.
						// TODO Do not hard-code logging sinks usecase
						// TODO  Handle case of os.Stdout and os.Stderr.
						// TODO  Do not hard-code the position of the argument, instead declaratively
						//  specify the position of the propagated source.
						// TODO  Consider turning propagators that take io.Writer into sinks.
						if a := conf.sendsToIOWriter(v); a != nil {
							sources = append(sources, newSource(a, conf))
						} else {
							//log.V(2).Infof("Adding source: %v %T", v.Value(), v.Value())
							sources = append(sources, newSource(v, conf))
						}

					case conf.isSink(v):
						// TODO Only variadic sink arguments are currently detected.
						if v.Call.Signature().Variadic() && len(v.Call.Args) > 0 {
							lastArg := v.Call.Args[len(v.Call.Args)-1]
							if varargs, ok := lastArg.(*ssa.Slice); ok {
								sinkVarargs := newVarargs(varargs, sources)
								for _, s := range sinkVarargs.sources {
									if !s.isSanitizedAt(v) {
										report(pass, s, v)
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return nil, nil
}

var readFileOnce sync.Once
var readConfigCached *config
var readConfigCachedErr error

func readConfig(path string) (*config, error) {
	loadedFromCache := true
	readFileOnce.Do(func() {
		loadedFromCache = false
		c := new(config)
		bytes, err := ioutil.ReadFile(path)
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

func identifySources(conf *config, ssaInput *buildssa.SSA) map[*ssa.Function][]*source {
	sourceMap := make(map[*ssa.Function][]*source)

	for _, fn := range ssaInput.SrcFuncs {
		var sources []*source
		sources = append(sources, sourcesFromParams(fn, conf)...)
		sources = append(sources, sourcesFromClosure(fn, conf)...)
		sources = append(sources, sourcesFromBlocks(fn, conf)...)

		if len(sources) > 0 {
			sourceMap[fn] = sources
		}
	}
	return sourceMap
}

func sourcesFromParams(fn *ssa.Function, conf *config) []*source {
	var sources []*source
	for _, p := range fn.Params {
		switch t := p.Type().(type) {
		case *types.Pointer:
			if n, ok := t.Elem().(*types.Named); ok && conf.isSource(n) {
				sources = append(sources, newSource(p, conf))
			}
			// TODO Handle the case where sources arepassed by value: func(c sourceType)
			// TODO Handle cases where PII is wrapped in struct/slice/map
		}
	}
	return sources
}

func sourcesFromClosure(fn *ssa.Function, conf *config) []*source {
	var sources []*source
	for _, p := range fn.FreeVars {
		switch t := p.Type().(type) {
		case *types.Pointer:
			// FreeVars (variables from a closure) appear as double-pointers
			// Hence, the need to dereference them recursively.
			if s, ok := dereferenceRecursive(t).(*types.Named); ok && conf.isSource(s) {
				sources = append(sources, newSource(p, conf))
			}
		}
	}
	return sources
}

func sourcesFromBlocks(fn *ssa.Function, conf *config) []*source {
	var sources []*source
	for _, b := range fn.Blocks {
		if b == fn.Recover {
			// TODO Handle calls to log in a recovery block.
			continue
		}

		for _, instr := range b.Instrs {
			switch v := instr.(type) {
			// Looking for sources of PII allocated within the body of a function.
			case *ssa.Alloc:
				if conf.isSource(dereferenceRecursive(v.Type())) {
					sources = append(sources, newSource(v, conf))
				}

				// Handling the case where PII may be in a receiver
				// (ex. func(b *something) { log.Info(something.PII) }
			case *ssa.FieldAddr:
				if conf.isSource(dereferenceRecursive(v.Type())) {
					sources = append(sources, newSource(v, conf))
				}
			}
		}
	}
	return sources
}

func report(pass *analysis.Pass, source *source, sink ssa.Node) {
	var b strings.Builder
	b.WriteString("a source has reached a sink")
	fmt.Fprintf(&b, ", source: %v", pass.Fset.Position(source.node.Pos()))
	pass.Reportf(sink.Pos(), b.String())
}

func dereferenceRecursive(t types.Type) types.Type {
	for {
		tt, ok := t.Underlying().(*types.Pointer)
		if !ok {
			return t
		}
		t = tt.Elem()
	}
}

func isTestPkg(p *types.Package) bool {
	for _, im := range p.Imports() {
		if im.Name() == "testing" {
			return true
		}
	}
	return false
}
