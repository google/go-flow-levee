package source

import (
	"github.com/eapache/queue"
	"golang.org/x/tools/go/ssa"
	"google.com/go-flow-levee/internal/pkg/config"
	"google.com/go-flow-levee/internal/pkg/matcher"
	"google.com/go-flow-levee/internal/pkg/sanitizer"
)

// source represents a source in an SSA call tree.
// It is based on ssa.Node, with the added functionality of computing the recursive graph of
// its referrers.
// source.sanitized notes sanitizer calls that sanitize this source
type Source struct {
	Node       ssa.Node
	marked     map[ssa.Node]bool
	sanitizers []*sanitizer.Sanitizer
}

func New(in ssa.Node, config *config.Config) *Source {
	a := &Source{
		Node:   in,
		marked: make(map[ssa.Node]bool),
	}
	a.bfs(config)
	return a
}

// bfs performs Breadth-First-Search on the def-use graph of the input source.
// While traversing the graph we also look for potential sanitizers of this source.
// If the source passes through a sanitizer, bfs does not continue through that Node.
func (a *Source) bfs(config *config.Config) {
	q := queue.New()
	q.Add(a.Node)
	a.marked[a.Node] = true

	for q.Length() > 0 {
		e := q.Remove().(ssa.Node)

		if c, ok := e.(*ssa.Call); ok && config.IsSanitizer(c) {
			a.sanitizers = append(a.sanitizers, &sanitizer.Sanitizer{Call: c})
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
			if call, ok := r.(*ssa.Call); ok && !config.IsPropagator(call) {
				continue
			}

			// Do not follow innocuous field access (e.g. Cluster.Zone)
			if addr, ok := r.(*ssa.FieldAddr); ok && !config.IsSourceFieldAddr(addr) {
				continue
			}

			q.Add(r)
		}
	}
}

// hasPathTo returns true when a Node is part of declaration-use graph.
func (a *Source) hasPathTo(n ssa.Node) bool {
	return a.marked[n]
}

func (a *Source) IsSanitizedAt(call ssa.Instruction) bool {
	for _, s := range a.sanitizers {
		if s.Dominates(call) {
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
type Varargs struct {
	slice   *ssa.Slice
	Sources []*Source
}

// newVarargs constructs varargs. SSA represents varargs as an ssa.Slice.
func NewVarargs(s *ssa.Slice, sources []*Source) *Varargs {
	a, ok := s.X.(*ssa.Alloc)
	if !ok || a.Comment != "varargs" {
		return nil
	}
	var (
		referredSources []*Source
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

	return &Varargs{
		slice:   s,
		Sources: referredSources,
	}
}

func (v *Varargs) referredByCallWithPattern(patterns []matcher.NameMatcher) *ssa.Call {
	if v.slice.Referrers() == nil || len(*v.slice.Referrers()) != 1 {
		return nil
	}

	c, ok := (*v.slice.Referrers())[0].(*ssa.Call)
	if !ok || c.Call.StaticCallee() == nil {
		return nil
	}

	for _, p := range patterns {
		if p.MatchMethodName(c) {
			return c
		}
	}

	return nil
}

