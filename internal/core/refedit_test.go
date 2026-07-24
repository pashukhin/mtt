package core

import (
	"errors"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestRefEditorAddRemove(t *testing.T) {
	store := newMemStore(mtt.Task{ID: "t1", Updated: time.Unix(0, 0)})
	e := NewRefEditor(store, testClock, nil)
	got, err := e.AddRef("t1", mtt.Ref{Kind: mtt.RefTask, ID: "t2", Label: "blocks"}, true, EventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Refs) != 1 || got.Refs[0].ID != "t2" || !got.Updated.Equal(testClock()) {
		t.Fatalf("add: %+v", got)
	}
	got, err = e.RemoveRef("t1", mtt.RefTask, "t2", EventOptions{})
	if err != nil || len(got.Refs) != 0 {
		t.Fatalf("remove: %+v err=%v", got, err)
	}
	// idempotent absent-key remove: no error
	if _, err := e.RemoveRef("t1", mtt.RefTask, "gone", EventOptions{}); err != nil {
		t.Fatalf("absent remove must be no-op: %v", err)
	}
	// carrier not found -> ErrNotFound
	if _, err := e.AddRef("t9", mtt.Ref{Kind: mtt.RefTask, ID: "t2"}, false, EventOptions{}); err == nil {
		t.Fatal("missing carrier must error")
	}
}

func TestNoteRefEditorAddRemove(t *testing.T) {
	kb := newFakeKB()
	_, _ = kb.CreateNote(mtt.Note{Slug: "a", Updated: time.Unix(0, 0)})
	e := NewNoteRefEditor(kb, testClock, nil)
	got, err := e.AddRef("a", mtt.Ref{Kind: mtt.RefTask, ID: "t2"}, false, EventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Refs) != 1 || !got.Updated.Equal(testClock()) {
		t.Fatalf("add: %+v", got)
	}
	got, err = e.RemoveRef("a", mtt.RefTask, "t2", EventOptions{})
	if err != nil || len(got.Refs) != 0 {
		t.Fatalf("remove: %+v err=%v", got, err)
	}
	// idempotent absent-key remove
	if _, err := e.RemoveRef("a", mtt.RefTask, "gone", EventOptions{}); err != nil {
		t.Fatalf("absent remove must be no-op: %v", err)
	}
	// missing note -> ErrNotFound
	if _, err := e.AddRef("z", mtt.Ref{Kind: mtt.RefTask, ID: "t2"}, false, EventOptions{}); err == nil {
		t.Fatal("missing note must error")
	}
}

func TestNoteRefEditorNoRunPreflight(t *testing.T) {
	kb := newFakeKB()
	if _, err := NewNoteAdder(kb, testClock, nil).Add(NoteParams{Slug: "a", Title: "x"}); err != nil {
		t.Fatal(err)
	}
	bare := EventOptions{NoRun: true}
	e := NewNoteRefEditor(kb, testClock, nil)
	if _, err := e.AddRef("a", mtt.Ref{Kind: mtt.RefTask, ID: "t2"}, false, bare); !errors.Is(err, ErrMissingAttribution) {
		t.Fatalf("AddRef preflight = %v", err)
	}
	if _, err := e.RemoveRef("a", mtt.RefTask, "t2", bare); !errors.Is(err, ErrMissingAttribution) {
		t.Fatalf("RemoveRef preflight = %v", err)
	}
	if n, _ := kb.GetNote("a"); len(n.Refs) != 0 {
		t.Fatal("store mutated despite failed preflight")
	}
}
