package core

import (
	"errors"
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// ErrDanglingRefs is returned by the check command when the sweep finds >=1
// dangling reference (the CLI maps it to exit 7).
var ErrDanglingRefs = errors.New("mtt: dangling references")

// RefKey is a reference's natural key (its target).
type RefKey struct {
	Kind   mtt.RefKind
	Target string
}

// Referent is one carrier that points at a target (a computed backlink entry).
type Referent struct {
	Carrier mtt.RefKind // RefTask or RefNote
	ID      string      // task id or note slug
	Label   string      // the forward ref's own label
}

// Backlinks is the computed inverse index target->referents (never stored).
type Backlinks map[RefKey][]Referent

// NewBacklinks builds the inverse index from a task+note snapshot. Referents are
// sorted (carrier kind, then id) for determinism.
func NewBacklinks(tasks []mtt.Task, notes []mtt.Note) Backlinks {
	b := Backlinks{}
	add := func(carrier mtt.RefKind, id string, refs []mtt.Ref) {
		for _, r := range refs {
			k := RefKey{Kind: r.Kind, Target: r.ID}
			b[k] = append(b[k], Referent{Carrier: carrier, ID: id, Label: r.Label})
		}
	}
	for _, t := range tasks {
		add(mtt.RefTask, string(t.ID), t.Refs)
	}
	for _, n := range notes {
		add(mtt.RefNote, string(n.Slug), n.Refs)
	}
	for k := range b {
		refs := b[k]
		sort.SliceStable(refs, func(i, j int) bool {
			if refs[i].Carrier != refs[j].Carrier {
				return refs[i].Carrier < refs[j].Carrier
			}
			return refs[i].ID < refs[j].ID
		})
	}
	return b
}

// To returns the referents pointing at (kind,target), or nil.
func (b Backlinks) To(kind mtt.RefKind, target string) []Referent {
	return b[RefKey{Kind: kind, Target: target}]
}

// CheckFinding is one non-ok ref discovered by the sweep.
type CheckFinding struct {
	CarrierKind mtt.RefKind
	CarrierID   string
	Ref         mtt.Ref
	Status      RefStatus
}

// CheckRefs sweeps every carrier's refs and returns the non-ok findings (dangling
// and unverified), in a deterministic order (carrier kind, carrier id, then
// (ref kind, ref id)). kbWired controls note verifiability (D5); the existence sets
// are built from the same snapshot.
func CheckRefs(tasks []mtt.Task, notes []mtt.Note, kbWired bool) []CheckFinding {
	taskSet := make(map[mtt.TaskID]bool, len(tasks))
	for _, t := range tasks {
		taskSet[t.ID] = true
	}
	noteSet := make(map[mtt.NoteSlug]bool, len(notes))
	for _, n := range notes {
		noteSet[n.Slug] = true
	}
	taskExists := func(id mtt.TaskID) bool { return taskSet[id] }
	var noteExists func(mtt.NoteSlug) bool
	if kbWired {
		noteExists = func(s mtt.NoteSlug) bool { return noteSet[s] }
	}
	var out []CheckFinding
	sweep := func(ck mtt.RefKind, id string, refs []mtt.Ref) {
		for _, r := range refs {
			st := VerifyRef(r, taskExists, noteExists)
			if st == RefOK {
				continue
			}
			out = append(out, CheckFinding{CarrierKind: ck, CarrierID: id, Ref: r, Status: st})
		}
	}
	for _, t := range tasks {
		sweep(mtt.RefTask, string(t.ID), t.Refs)
	}
	for _, n := range notes {
		sweep(mtt.RefNote, string(n.Slug), n.Refs)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.CarrierKind != b.CarrierKind {
			return a.CarrierKind < b.CarrierKind
		}
		if a.CarrierID != b.CarrierID {
			return a.CarrierID < b.CarrierID
		}
		if a.Ref.Kind != b.Ref.Kind {
			return a.Ref.Kind < b.Ref.Kind
		}
		return a.Ref.ID < b.Ref.ID
	})
	return out
}
