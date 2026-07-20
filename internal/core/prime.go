package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// PrimeOptions parameterizes the KB digest. MinPriority is the eligibility
// threshold (the CLI defaults it to high); Limit caps the shown entries (<=0 = no cap).
type PrimeOptions struct {
	MinPriority mtt.Priority
	Limit       int
}

// PrimeEntry is one important note in the digest (a pointer — no Body).
type PrimeEntry struct {
	Slug      mtt.NoteSlug
	Title     string
	Tags      []string
	Priority  mtt.Priority
	Backlinks int
}

// Prime is the pure derived KB digest (like Roadmap; not in the pkg/mtt contract).
// Eligible ⇔ the note has an EXPLICIT priority (Priority != "") whose Rank() is at or
// above MinPriority; unset notes are NEVER primed (the opt-in safety model). The order
// is priority band (Rank asc), then backlink-count desc, then recency. The second
// return is total — the eligible count BEFORE the Limit cap (the footer's M).
func Prime(notes []mtt.Note, bl Backlinks, opts PrimeOptions) ([]PrimeEntry, int) {
	threshold := opts.MinPriority.Rank()
	type scored struct {
		n  mtt.Note
		bc int
	}
	var elig []scored
	for _, n := range notes {
		if n.Priority == "" || n.Priority.Rank() > threshold {
			continue
		}
		elig = append(elig, scored{n: n, bc: len(bl.To(mtt.RefNote, string(n.Slug)))})
	}
	sort.SliceStable(elig, func(i, j int) bool {
		ri, rj := elig[i].n.Priority.Rank(), elig[j].n.Priority.Rank()
		if ri != rj {
			return ri < rj
		}
		if elig[i].bc != elig[j].bc {
			return elig[i].bc > elig[j].bc // more-referenced first
		}
		return lessNotesByRecency(elig[i].n, elig[j].n, SortCreated)
	})
	total := len(elig)
	if opts.Limit > 0 && len(elig) > opts.Limit {
		elig = elig[:opts.Limit]
	}
	out := make([]PrimeEntry, 0, len(elig))
	for _, s := range elig {
		out = append(out, PrimeEntry{Slug: s.n.Slug, Title: s.n.Title, Tags: s.n.Tags, Priority: s.n.Priority, Backlinks: s.bc})
	}
	return out, total
}
