// Copyright 2021 Google LLC
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

// Package earpointer introduces a pointer analysis based on unifying
// equivalent abstract references (EAR), which implements Steensgaard's algorithm.
// Extensions include adding context sensitivity and field sensitivity.
// Abstract references defines the data structure to represent abstract heap.
package earpointer

import (
	"log"
	"sort"
	"strings"

	"golang.org/x/tools/go/callgraph"
)

// parentMap maps a reference to its representative (i.e. the parent
// in the union-find tree).
type parentMap map[Reference]Reference

// partitionInfo maintains information at the partition level (which
// consists of a set of references). It maintains the size of the
// partition (used for weighted join in union-find) and a FieldMap,
// which is a mapping from values F (instance fields, array contents etc) to
// references R.
type partitionInfo struct {
	numMembers uint     // number of members
	fieldRefs  FieldMap // field mapping
}

// partitionInfoMap maps a reference to its partition.
type partitionInfoMap map[Reference]partitionInfo

// state is Equivalent Abstract references (EAR) state, which maintains
// an abstract heap information. It contains data structures that
// may be mutated during heap construction.
type state struct {
	// Map from a ref (a partition representative) to information about
	// this partition.
	partitions partitionInfoMap
	// Map from a ref to its parent abstract ref.
	parents parentMap
}

// NewState creates an empty abstract state.
func NewState() *state {
	return &state{
		partitions: make(partitionInfoMap),
		parents:    make(parentMap),
	}
}

// references gets all references.
func (state *state) references() ReferenceSet {
	refs := make(ReferenceSet)
	for k := range state.parents {
		refs[k] = true
	}
	return refs
}

// representatives gets all references that are partition representatives.
//lint:ignore U1000 ignore dead code for now
func (state *state) representatives() ReferenceSet {
	reps := make(ReferenceSet)
	for k, v := range state.parents {
		if k == v {
			reps[k] = true
		}
	}
	return reps
}

// representative gets the partition representative of reference "ref"
// ("ref" must belong to this state).
func (state *state) representative(ref Reference) Reference {
	failureCallback := func(ref Reference) Reference {
		// Some global variables has no declarations in the SSA;
		// Insert their references dynamically.
		state.Insert(ref)
		return ref
	}
	return state.lookupPartitionRep(ref, failureCallback)
}

// PartitionFieldMap gets the partition-level field map for partition
// representative "ref". The result maps a field to a partition representative.
// Note "ref" must be a partition representative.
func (state *state) PartitionFieldMap(ref Reference) FieldMap {
	return state.partitionInfo(ref).fieldRefs
}

// Pretty printer
func (state *state) String() string {
	refs := state.references()
	members := make(map[Reference][]Reference)
	for ref := range refs {
		rep := state.representative(ref)
		members[rep] = append(members[rep], ref)
	}

	var pstrs []string
	for rep, mems := range members {
		mstrs := make([]string, len(mems))
		for i, v := range mems {
			mstrs[i] = v.String()
		}
		sort.Strings(mstrs)
		pstr := "{" + strings.Join(mstrs, ",") + "}: " +
			state.PartitionFieldMap(rep).String()
		pstrs = append(pstrs, pstr)
	}
	sort.Strings(pstrs)
	return strings.Join(pstrs, ", ")
}

// Pretty prints a field map.
func (fmap FieldMap) String() string {
	if v, ok := fmap[directPointToField]; ok {
		return "--> " + v.String()
	}
	var fstrs []string
	for k, v := range fmap {
		fstrs = append(fstrs, k.Name+"->"+v.String())
	}
	sort.Strings(fstrs)
	return "[" + strings.Join(fstrs, ", ") + "]"
}

// Various state mutation operation

// Insert inserts reference "ref" to the state and returns the current
// partition representative of "ref".
func (state *state) Insert(ref Reference) Reference {
	// Lookup failure will create new entry in the parent table.
	failureCallback := func(ref Reference) Reference {
		state.parents[ref] = ref
		state.partitions[ref] = partitionInfo{numMembers: 1, fieldRefs: make(FieldMap)}
		return ref
	}
	return state.lookupPartitionRep(ref, failureCallback)
}

// Unify unifies the references "ref1" and "ref2"
func (state *state) Unify(ref1 Reference, ref2 Reference) {
	state.unifyReps(state.representative(ref1), state.representative(ref2))
}

// unifyReps unifies the references "ref1" and "ref2", assuming both are
// partition representative. This allows to avoid calling representative
// unnecesarilly. Using this function with non-representative
// references will raise an error.
func (state *state) unifyReps(ref1 Reference, ref2 Reference) {
	// If they are in the same partition already, we are done.
	if ref1 == ref2 {
		return
	}
	// Create local copies to use swap (this improves the readability).
	prep1, prep2 := ref1, ref2
	// Find partition-level info for "ref1" and "ref2"
	pinfo1 := state.partitionInfo(prep1)
	pinfo2 := state.partitionInfo(prep2)
	// swap so that "ref1" is always the smaller partition
	// if their sizes are the same, use source position information to
	// make the choice deterministic.
	n1, n2 := pinfo1.numMembers, pinfo2.numMembers
	if n1 > n2 || (n1 == n2 && prep1.Value().Pos() > prep2.Value().Pos()) {
		prep1, prep2 = prep2, prep1
		pinfo1, pinfo2 = pinfo2, pinfo1
	}

	// Create state by having "prep1" point to "prep2" as parent and
	// by erasing "prep1" as a partition rep. We then call MergeFieldMap()
	// to merge field maps (which can trigger further unification). This process
	// will converge since the number of partitions is guaranteed to decrease at
	// every unification and is lower-bounded by 1.

	// Update parent pointer and partition info map for ref1 while retaining
	// its field map in fmap1.
	state.parents[prep1] = prep2
	fmap1 := pinfo1.fieldRefs
	pinfo1.fieldRefs = nil
	delete(state.partitions, prep1)

	// Update partition info for ref2 and merge the field map fmap1 in-place.
	pinfo2.numMembers += pinfo1.numMembers
	state.mergeFieldMap(fmap1, pinfo2.fieldRefs)
}

// Helper to find a partition representative. It calls onfailure callback
// to delegate the action on lookup failure.
func (state *state) lookupPartitionRep(ref Reference, onfailure func(abs Reference) Reference) Reference {
	rep, ok := state.parents[ref]
	if !ok {
		res := onfailure(ref)
		return res
	}
	// If we found the partition rep, return right away.
	if rep == ref {
		return rep
	}
	// Else recurse. We use crashing callback here, because when recursing
	// we expect the recursion argument to be always in the abstract state.
	failureCallback := func(ref Reference) Reference {
		log.Printf("lookupPartitionRep: Reference [%s] not found in state", ref)
		return nil
	}
	prep := state.lookupPartitionRep(rep, failureCallback)
	// Perform path compression before returning to caller.
	state.parents[ref] = prep
	return prep
}

// mergeFieldMap sets the partition info for "rep" as a merge of the field
// maps in "oldMap" and "newMap" (the last is an out-parameter to save
// on memory copies).
func (state *state) mergeFieldMap(oldMap FieldMap, newMap FieldMap) {
	// Merge field maps of "oldMap" and update "newMap" suitably. For any
	// common fields, we unify the pointers, else we copy over the field mapping
	// as such.  We first compute the list of references to unify, update the
	// field map, and then perform unification.
	toUnify := make(map[Reference]Reference)
	// Merging old fmap to a new fmap. We iterate over oldMap and insert all elements
	// to newMap. If a field already exists in the newMap, we unify the corresponding heap
	// partitions. This takes linear time.
	for k, v := range oldMap {
		w, ok := newMap[k]
		if !ok {
			newMap[k] = v
		} else {
			toUnify[v] = w
		}
	}
	// unify all necessary field partitions.
	for k, v := range toUnify {
		state.Unify(k, v)
	}
}

// Helper to obtain the partition info for "ref" (which should be a partition
// representative)
func (state *state) partitionInfo(ref Reference) *partitionInfo {
	v, ok := state.partitions[ref]
	if !ok {
		log.Fatalf("Reference [%s] is not a partition rep", ref)
	}
	return &v
}

// valueReferenceOrNil returns the reference holding the value *r of an address r
// which is a partition representative.
// Return nil if such a value reference doesn't exists.
//lint:ignore U1000 ignore dead code for now
func (state *state) valueReferenceOrNil(addr Reference) Reference {
	// In the heap, r --> *r is implemented as r[directPointToField] = *r.
	fmap := state.PartitionFieldMap(addr)
	if v, ok := fmap[directPointToField]; ok {
		return v
	}
	return nil
}

// Partitions represents the state with the partitions fully constructed, after which
// no more mutation operations will be performed. Its internal data structures are
// optimized for lookups only.
type Partitions struct {
	// Inherit "parents" and "partitions" from state.

	// Map from a ref to its parent abstract ref.
	parents parentMap
	// Map from a ref (a partition representative) to information about
	// this partition, esp. field maps.
	partitions partitionInfoMap

	// Extra data structures for partitions.

	// Mapping from a partition representative to its members. This map is
	// initialize in finalize method, which guarantees to avoid duplication.
	// Users are accessing this information via PartitionMembers method,
	// which does not allow any modification.
	members map[Reference][]Reference

	// Map from each field reference to its parent references. For example, record
	// r->o for "o.x = r". This map is for accelerating external queries on
	// fields, and is not required by the EAR analysis itself.
	// It is constructed separately using the "ConstructFieldParentMap()"
	// at the final phase.
	revFields map[Reference][]Reference

	// The call graph used to unify callers and callees.
	cg *callgraph.Graph
}

func (state *state) ToPartitions() *Partitions {
	p := &Partitions{
		parents:    state.parents,
		partitions: state.partitions,
		members:    make(map[Reference][]Reference),
		revFields:  make(map[Reference][]Reference),
	}
	p.finalize()
	return p
}

// finalize indicates that no more mutation operations will be performed.
// After this call, internal data structures are optimized for lookups only,
// and no mutations should be made.
func (p *Partitions) finalize() {
	// (1) Construct a new parent map that maps a reference to its representative
	//     so as to accelerate representative lookup.
	// For example, supposed the old parent map contains r1->r2, r2->r3,
	// r3->r4(representative), then the new parent map contains r1->r4.
	// "r4" can be returned immediately without chasing the union-find relation
	// in the old map.
	optParents := make(parentMap) // optimized parents
	for ref, parent := range p.parents {
		if ref == parent {
			optParents[ref] = ref
		} else {
			rep := parent
			for up := p.parents[rep]; up != rep; {
				rep = up
			}
			// Otherwise, get ref's partition representative and update members.
			optParents[ref] = rep
		}
	}
	p.parents = optParents
	// (2) Construct the members. members maps partitions representatives to the
	// members (references) of partitions.
	for ref, parent := range p.parents {
		if ref == parent {
			// If ref is equal to its parent, then ref is partition
			// representative. No need to call representative.
			p.members[ref] = append(p.members[ref], ref)
		} else {
			rep := p.Representative(ref)
			// Otherwise, get ref's partition representative and update members.
			p.members[rep] = append(p.members[rep], ref)
		}
	}
	// (3) Normalize fieldRefs table in a way that it only contains partition
	// representatives.
	for _, pinfo := range p.partitions {
		for fd, entry := range pinfo.fieldRefs {
			pinfo.fieldRefs[fd] = p.Representative(entry)
		}
	}
	// (4) Construct the map from field references to their parents.
	p.constructFieldParentMap()
}

// Has returns true if "ref" is a reference present in the partitions.
func (p *Partitions) Has(ref Reference) bool {
	_, ok := p.parents[ref]
	return ok
}

// References gets all references.
func (p *Partitions) References() ReferenceSet {
	refs := make(ReferenceSet)
	for k := range p.parents {
		refs[k] = true
	}
	return refs
}

// Representatives gets all references that are partition representatives.
func (p *Partitions) Representatives() ReferenceSet {
	reps := make(ReferenceSet)
	for k := range p.members {
		reps[k] = true
	}
	return reps
}

// Representative gets the partition representative of reference "ref"
// ("ref" must belong to this partitions).
func (p *Partitions) Representative(ref Reference) Reference {
	return p.parents[ref]
}

// PartitionFieldMap gets the partition-level field map for partition
// representative "ref". The result maps a field (ConstTermHandle type)
// to a reference that is a partition representative.
// Note "ref" must be a partition representative.
func (p *Partitions) PartitionFieldMap(ref Reference) FieldMap {
	return p.partitions[ref].fieldRefs
}

// Pretty printer
func (p *Partitions) String() string {
	state := state{partitions: p.partitions, parents: p.parents}
	return state.String()
}

// FieldParents returns the parents of a field reference. Return nil if ref has no parent.
// For example, return o for r if "o[x -> r]" is in partitions. This shall be
// called only after the partitions have been finalized.
func (p *Partitions) FieldParents(ref Reference) []Reference {
	return p.revFields[ref]
}

// constructFieldParentMap constructs (from scratch) a map from each field reference
// to its parent reference. For example, record r->o for "o.x = r".
func (p *Partitions) constructFieldParentMap() {
	reps := p.Representatives()
	for rep := range reps {
		if fmap := p.PartitionFieldMap(rep); fmap != nil {
			for _, fref := range fmap {
				p.revFields[fref] = append(p.revFields[fref], rep)
			}
		}
	}
}

// NumPartitions returns the number of representatives
// (i.e., the size of the partition).
func (p *Partitions) NumPartitions() int {
	return len(p.members)
}

// MembersForRep gets the list of members in the partition
// for which "prep" is the representative.
// Precondition: "prep" must be a partition representative.
func (p *Partitions) MembersForRep(rep Reference) []Reference {
	members, ok := p.members[rep]
	if !ok {
		log.Printf("Warning: reference [%s] is not a partition rep", rep)
		return nil
	}
	return members
}

// PartitionMembers gets the list of members in the partition to which "ref" belongs.
// Precondition: "ref" must be a reference present in the partitions.
func (p *Partitions) PartitionMembers(ref Reference) []Reference {
	return p.MembersForRep(p.Representative(ref))
}
