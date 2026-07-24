package core

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// NoteAdder is the note-create usecase: validate the slug, canonicalize tags, stamp
// Created/Updated from the injected clock, persist via KnowledgeStore.CreateNote.
type NoteAdder struct {
	store mtt.KnowledgeStore
	now   func() time.Time
	ev    *EventEmitter
}

// NewNoteAdder builds a NoteAdder; ev fires the note create event (nil = none).
func NewNoteAdder(store mtt.KnowledgeStore, now func() time.Time, ev *EventEmitter) *NoteAdder {
	return &NoteAdder{store: store, now: now, ev: ev}
}

// NoteParams carries the note-create inputs (already parsed at the CLI boundary).
type NoteParams struct {
	Slug     mtt.NoteSlug
	Title    string
	Tags     []string
	Priority mtt.Priority // importance axis (t51; unset = medium in ordering)
	Body     string
	Refs     []mtt.Ref    // informational references set at creation (canonicalized; not verified here)
	Events   EventOptions // lifecycle-event bypass + attribution (t66)
}

// Add validates and creates the note. A *PostActionError return carries the
// PERSISTED note (the create happened; only the event finalization failed).
func (a *NoteAdder) Add(p NoteParams) (mtt.Note, error) {
	if err := p.Events.Preflight(); err != nil {
		return mtt.Note{}, err
	}
	if !p.Slug.Valid() {
		return mtt.Note{}, fmt.Errorf("invalid note slug %q", string(p.Slug))
	}
	ts := a.now().UTC()
	var refs []mtt.Ref
	if len(p.Refs) > 0 {
		refs = canonicalRefs(p.Refs)
	}
	created, err := a.store.CreateNote(mtt.Note{
		Slug:     p.Slug,
		Title:    p.Title,
		Tags:     canonicalTags(p.Tags),
		Priority: p.Priority,
		Body:     p.Body,
		Refs:     refs,
		Created:  ts,
		Updated:  ts,
	})
	if err != nil {
		return mtt.Note{}, err
	}
	return created, a.ev.NoteEvent(mtt.EventCreate, created, "note add", p.Events)
}

// NoteEditor is the note-edit usecase: load, apply only the provided fields, bump
// Updated (keep Created), persist via KnowledgeStore.UpdateNote.
type NoteEditor struct {
	store mtt.KnowledgeStore
	now   func() time.Time
	ev    *EventEmitter
}

// NewNoteEditor builds a NoteEditor; ev fires the note update event (nil = none).
func NewNoteEditor(store mtt.KnowledgeStore, now func() time.Time, ev *EventEmitter) *NoteEditor {
	return &NoteEditor{store: store, now: now, ev: ev}
}

// NoteEditParams carries the note-edit inputs; a nil pointer means "unchanged".
// Tags, when non-nil, REPLACES the whole set (declarative, not additive).
type NoteEditParams struct {
	Title    *string
	Tags     *[]string
	Body     *string
	Priority *mtt.Priority
	Events   EventOptions // lifecycle-event bypass + attribution (t66)
}

// Edit applies the provided fields and persists the note. A *PostActionError
// return carries the PERSISTED note.
func (e *NoteEditor) Edit(slug mtt.NoteSlug, p NoteEditParams) (mtt.Note, error) {
	if err := p.Events.Preflight(); err != nil {
		return mtt.Note{}, err
	}
	if !slug.Valid() {
		return mtt.Note{}, fmt.Errorf("invalid note slug %q", string(slug))
	}
	if p.Title == nil && p.Tags == nil && p.Body == nil && p.Priority == nil {
		return mtt.Note{}, errors.New("nothing to edit (provide --title, --tag, --body, --file, or --priority)")
	}
	n, err := e.store.GetNote(slug)
	if err != nil {
		return mtt.Note{}, err
	}
	if p.Title != nil {
		n.Title = *p.Title
	}
	if p.Tags != nil {
		n.Tags = canonicalTags(*p.Tags)
	}
	if p.Body != nil {
		n.Body = *p.Body
	}
	if p.Priority != nil {
		n.Priority = *p.Priority
	}
	n.Updated = e.now().UTC()
	up, err := e.store.UpdateNote(n)
	if err != nil {
		return mtt.Note{}, err
	}
	return up, e.ev.NoteEvent(mtt.EventUpdate, up, "note edit", p.Events)
}

// NoteFilter filters and orders a note list. Tags is OR-within (a note matches if it
// carries any filter tag; empty matches all; pre-normalized via CLI toTags).
// Priorities matches the STORED label (unset matches only when the filter is empty —
// mirrors the task ListFilter). Sort selects the ordering (SortPriority orders by
// Rank asc; else the default recency order).
type NoteFilter struct {
	Tags       []string
	Priorities []mtt.Priority
	Sort       SortKey
}

// SelectNotes filters notes and orders them (default: Created desc, slug tiebreak;
// SortPriority: Rank asc then that recency order). Pure — no store, no clock
// (mirrors core.Select / lessByPriority).
func SelectNotes(notes []mtt.Note, f NoteFilter) []mtt.Note {
	out := make([]mtt.Note, 0, len(notes))
	for _, n := range notes {
		if anyOrEmptyIntersect(f.Tags, n.Tags) && anyOrEmpty(f.Priorities, n.Priority) {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if f.Sort == SortPriority {
			if ri, rj := out[i].Priority.Rank(), out[j].Priority.Rank(); ri != rj {
				return ri < rj
			}
			return lessNotesByRecency(out[i], out[j], SortCreated) // priority tiebreak: created desc
		}
		return lessNotesByRecency(out[i], out[j], f.Sort)
	})
	return out
}

// lessNotesByRecency orders by the chosen timestamp descending (Updated when
// key == SortUpdated, else Created), tie-broken by slug — the note analogue of
// lessByRecency. Extracted so the priority sort can fall back to it.
func lessNotesByRecency(a, b mtt.Note, key SortKey) bool {
	ta, tb := a.Created, b.Created
	if key == SortUpdated {
		ta, tb = a.Updated, b.Updated
	}
	if !ta.Equal(tb) {
		return ta.After(tb)
	}
	return a.Slug < b.Slug
}
