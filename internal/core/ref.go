package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// RefStatus is the derived resolution state of a reference (never stored).
type RefStatus string

// The three reference resolution states.
const (
	RefOK         RefStatus = "ok"
	RefDangling   RefStatus = "dangling"
	RefUnverified RefStatus = "unverified"
)

// canonicalRefs returns refs deduped by (Kind,ID) — the natural key — keeping the
// LAST occurrence (so an upsert's appended value wins), sorted by (Kind,ID).
func canonicalRefs(refs []mtt.Ref) []mtt.Ref {
	last := make(map[[2]string]mtt.Ref, len(refs))
	order := make([][2]string, 0, len(refs))
	for _, r := range refs {
		k := [2]string{string(r.Kind), r.ID}
		if _, seen := last[k]; !seen {
			order = append(order, k)
		}
		last[k] = r
	}
	out := make([]mtt.Ref, 0, len(order))
	for _, k := range order {
		out = append(out, last[k])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// upsertRef adds r to refs by its natural key. An existing key keeps its label
// unless setLabel is true (then r.Label overwrites); a new key is appended. The
// result is canonicalized.
func upsertRef(refs []mtt.Ref, r mtt.Ref, setLabel bool) []mtt.Ref {
	out := make([]mtt.Ref, 0, len(refs)+1)
	found := false
	for _, e := range refs {
		if e.Kind == r.Kind && e.ID == r.ID {
			found = true
			if setLabel {
				e.Label = r.Label
			}
		}
		out = append(out, e)
	}
	if !found {
		out = append(out, r)
	}
	return canonicalRefs(out)
}

// removeRef drops the (kind,id) entry; the bool reports whether it was present.
func removeRef(refs []mtt.Ref, kind mtt.RefKind, id string) ([]mtt.Ref, bool) {
	out := make([]mtt.Ref, 0, len(refs))
	found := false
	for _, e := range refs {
		if e.Kind == kind && e.ID == id {
			found = true
			continue
		}
		out = append(out, e)
	}
	return out, found
}

// VerifyRef resolves a ref's status capability-aware. taskExists is always
// available; noteExists is nil when no KnowledgeStore is wired (then a note ref is
// unverified). url is external — always unverified. Any other kind (e.g. an
// unreachable comment) is unverified.
func VerifyRef(r mtt.Ref, taskExists func(mtt.TaskID) bool, noteExists func(mtt.NoteSlug) bool) RefStatus {
	switch r.Kind {
	case mtt.RefTask:
		if taskExists(mtt.TaskID(r.ID)) {
			return RefOK
		}
		return RefDangling
	case mtt.RefNote:
		if noteExists == nil {
			return RefUnverified
		}
		if noteExists(mtt.NoteSlug(r.ID)) {
			return RefOK
		}
		return RefDangling
	default: // url, and any not-yet-supported kind
		return RefUnverified
	}
}
