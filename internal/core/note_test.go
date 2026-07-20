package core

import (
	"errors"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// fakeKB is an in-memory mtt.KnowledgeStore for usecase tests.
type fakeKB struct {
	notes map[mtt.NoteSlug]mtt.Note
}

func newFakeKB() *fakeKB { return &fakeKB{notes: map[mtt.NoteSlug]mtt.Note{}} }

func (f *fakeKB) CreateNote(n mtt.Note) (mtt.Note, error) {
	if _, ok := f.notes[n.Slug]; ok {
		return mtt.Note{}, errors.New("exists")
	}
	f.notes[n.Slug] = n
	return n, nil
}
func (f *fakeKB) GetNote(slug mtt.NoteSlug) (mtt.Note, error) {
	n, ok := f.notes[slug]
	if !ok {
		return mtt.Note{}, mtt.ErrNotFound
	}
	return n, nil
}
func (f *fakeKB) ListNotes() ([]mtt.Note, error) {
	out := make([]mtt.Note, 0, len(f.notes))
	for _, n := range f.notes {
		out = append(out, n)
	}
	return out, nil
}
func (f *fakeKB) UpdateNote(n mtt.Note) (mtt.Note, error) {
	if _, ok := f.notes[n.Slug]; !ok {
		return mtt.Note{}, mtt.ErrNotFound
	}
	f.notes[n.Slug] = n
	return n, nil
}
func (f *fakeKB) DeleteNote(slug mtt.NoteSlug) error {
	if _, ok := f.notes[slug]; !ok {
		return mtt.ErrNotFound
	}
	delete(f.notes, slug)
	return nil
}

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestNoteAdder(t *testing.T) {
	kb := newFakeKB()
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got, err := NewNoteAdder(kb, fixedClock(ts)).Add(NoteParams{Slug: "auth-design", Title: "Auth", Tags: []string{"Design", "design", "auth"}, Body: "b"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !got.Created.Equal(ts) || !got.Updated.Equal(ts) {
		t.Errorf("clock not applied: %+v", got)
	}
	// canonicalTags: deduped + sorted + lowercased.
	if len(got.Tags) != 2 || got.Tags[0] != "auth" || got.Tags[1] != "design" {
		t.Errorf("tags not canonical: %v", got.Tags)
	}
	// Invalid slug rejected.
	if _, err := NewNoteAdder(kb, fixedClock(ts)).Add(NoteParams{Slug: "../x"}); err == nil {
		t.Error("add invalid slug: want error")
	}
}

func TestNoteEditor(t *testing.T) {
	kb := newFakeKB()
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	kb.notes["auth-design"] = mtt.Note{Slug: "auth-design", Title: "Old", Tags: []string{"a"}, Body: "old", Created: created, Updated: created}
	later := created.Add(time.Hour)

	title := "New"
	got, err := NewNoteEditor(kb, fixedClock(later)).Edit("auth-design", NoteEditParams{Title: &title})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if got.Title != "New" || got.Body != "old" { // only title changed
		t.Errorf("edit applied wrong fields: %+v", got)
	}
	if !got.Created.Equal(created) || !got.Updated.Equal(later) {
		t.Errorf("created must be kept, updated bumped: %+v", got)
	}
	// Tags provided -> whole set replaced.
	tags := []string{"x", "y"}
	got, _ = NewNoteEditor(kb, fixedClock(later)).Edit("auth-design", NoteEditParams{Tags: &tags})
	if len(got.Tags) != 2 || got.Tags[0] != "x" {
		t.Errorf("tags not replaced: %v", got.Tags)
	}
	// Nothing to edit -> error.
	if _, err := NewNoteEditor(kb, fixedClock(later)).Edit("auth-design", NoteEditParams{}); err == nil {
		t.Error("empty edit: want error")
	}
	// Missing note -> ErrNotFound.
	if _, err := NewNoteEditor(kb, fixedClock(later)).Edit("missing", NoteEditParams{Title: &title}); !errors.Is(err, mtt.ErrNotFound) {
		t.Errorf("edit missing: want ErrNotFound, got %v", err)
	}
}

func TestSelectNotes(t *testing.T) {
	older := time.Unix(100, 0).UTC()
	newer := time.Unix(200, 0).UTC()
	notes := []mtt.Note{
		{Slug: "b", Tags: []string{"design"}, Created: older},
		{Slug: "a", Tags: []string{"design"}, Created: newer},
		{Slug: "c", Tags: []string{"ops"}, Created: newer},
	}
	// Empty filter -> all, Created desc then slug asc.
	all := SelectNotes(notes, NoteFilter{})
	if len(all) != 3 || all[0].Slug != "a" || all[1].Slug != "c" || all[2].Slug != "b" {
		t.Fatalf("order: %v", slugs(all))
	}
	// Tag filter -> intersection.
	design := SelectNotes(notes, NoteFilter{Tags: []string{"design"}})
	if len(design) != 2 || design[0].Slug != "a" || design[1].Slug != "b" {
		t.Fatalf("tag filter: %v", slugs(design))
	}
}

func slugs(ns []mtt.Note) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = string(n.Slug)
	}
	return out
}

func TestNoteAdderRefs(t *testing.T) {
	kb := newFakeKB()
	got, err := NewNoteAdder(kb, testClock).Add(NoteParams{Slug: "a", Refs: []mtt.Ref{
		{Kind: mtt.RefTask, ID: "t2"}, {Kind: mtt.RefTask, ID: "t2"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Refs) != 1 { // deduped
		t.Fatalf("refs: %+v", got.Refs)
	}
}
