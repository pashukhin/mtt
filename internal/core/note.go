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
}

// NewNoteAdder builds a NoteAdder.
func NewNoteAdder(store mtt.KnowledgeStore, now func() time.Time) *NoteAdder {
	return &NoteAdder{store: store, now: now}
}

// NoteParams carries the note-create inputs (already parsed at the CLI boundary).
type NoteParams struct {
	Slug  mtt.NoteSlug
	Title string
	Tags  []string
	Body  string
}

// Add validates and creates the note.
func (a *NoteAdder) Add(p NoteParams) (mtt.Note, error) {
	if !p.Slug.Valid() {
		return mtt.Note{}, fmt.Errorf("invalid note slug %q", string(p.Slug))
	}
	ts := a.now().UTC()
	return a.store.CreateNote(mtt.Note{
		Slug:    p.Slug,
		Title:   p.Title,
		Tags:    canonicalTags(p.Tags),
		Body:    p.Body,
		Created: ts,
		Updated: ts,
	})
}

// NoteEditor is the note-edit usecase: load, apply only the provided fields, bump
// Updated (keep Created), persist via KnowledgeStore.UpdateNote.
type NoteEditor struct {
	store mtt.KnowledgeStore
	now   func() time.Time
}

// NewNoteEditor builds a NoteEditor.
func NewNoteEditor(store mtt.KnowledgeStore, now func() time.Time) *NoteEditor {
	return &NoteEditor{store: store, now: now}
}

// NoteEditParams carries the note-edit inputs; a nil pointer means "unchanged".
// Tags, when non-nil, REPLACES the whole set (declarative, not additive).
type NoteEditParams struct {
	Title *string
	Tags  *[]string
	Body  *string
}

// Edit applies the provided fields and persists the note.
func (e *NoteEditor) Edit(slug mtt.NoteSlug, p NoteEditParams) (mtt.Note, error) {
	if !slug.Valid() {
		return mtt.Note{}, fmt.Errorf("invalid note slug %q", string(slug))
	}
	if p.Title == nil && p.Tags == nil && p.Body == nil {
		return mtt.Note{}, errors.New("nothing to edit (provide --title, --tag, --body, or --file)")
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
	n.Updated = e.now().UTC()
	return e.store.UpdateNote(n)
}

// NoteFilter filters a note list. Tags is OR-within (a note matches if it carries any
// filter tag; an empty filter matches all). Filter tags are pre-normalized (CLI toTags).
type NoteFilter struct {
	Tags []string
}

// SelectNotes filters notes and orders them Created desc, tie-broken by slug (opaque
// string) for determinism. Pure — no store, no clock (mirrors core.Select).
func SelectNotes(notes []mtt.Note, f NoteFilter) []mtt.Note {
	out := make([]mtt.Note, 0, len(notes))
	for _, n := range notes {
		if anyOrEmptyIntersect(f.Tags, n.Tags) {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Created.Equal(out[j].Created) {
			return out[i].Created.After(out[j].Created)
		}
		return out[i].Slug < out[j].Slug
	})
	return out
}
