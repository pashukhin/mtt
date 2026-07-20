package core

import (
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestRefEditorAddRemove(t *testing.T) {
	store := newMemStore(mtt.Task{ID: "t1", Updated: time.Unix(0, 0)})
	e := NewRefEditor(store, testClock)
	got, err := e.AddRef("t1", mtt.Ref{Kind: mtt.RefTask, ID: "t2", Label: "blocks"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Refs) != 1 || got.Refs[0].ID != "t2" || !got.Updated.Equal(testClock()) {
		t.Fatalf("add: %+v", got)
	}
	got, err = e.RemoveRef("t1", mtt.RefTask, "t2")
	if err != nil || len(got.Refs) != 0 {
		t.Fatalf("remove: %+v err=%v", got, err)
	}
	// idempotent absent-key remove: no error
	if _, err := e.RemoveRef("t1", mtt.RefTask, "gone"); err != nil {
		t.Fatalf("absent remove must be no-op: %v", err)
	}
	// carrier not found -> ErrNotFound
	if _, err := e.AddRef("t9", mtt.Ref{Kind: mtt.RefTask, ID: "t2"}, false); err == nil {
		t.Fatal("missing carrier must error")
	}
}

func TestNoteRefEditorAddRemove(t *testing.T) {
	kb := newFakeKB()
	_, _ = kb.CreateNote(mtt.Note{Slug: "a", Updated: time.Unix(0, 0)})
	e := NewNoteRefEditor(kb, testClock)
	got, err := e.AddRef("a", mtt.Ref{Kind: mtt.RefTask, ID: "t2"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Refs) != 1 || !got.Updated.Equal(testClock()) {
		t.Fatalf("add: %+v", got)
	}
	got, err = e.RemoveRef("a", mtt.RefTask, "t2")
	if err != nil || len(got.Refs) != 0 {
		t.Fatalf("remove: %+v err=%v", got, err)
	}
	// idempotent absent-key remove
	if _, err := e.RemoveRef("a", mtt.RefTask, "gone"); err != nil {
		t.Fatalf("absent remove must be no-op: %v", err)
	}
	// missing note -> ErrNotFound
	if _, err := e.AddRef("z", mtt.Ref{Kind: mtt.RefTask, ID: "t2"}, false); err == nil {
		t.Fatal("missing note must error")
	}
}
