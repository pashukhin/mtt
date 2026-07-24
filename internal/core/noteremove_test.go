package core

import (
	"errors"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestNoteRemoverGuard(t *testing.T) {
	kb := newFakeKB()
	_, _ = kb.CreateNote(mtt.Note{Slug: "a"})
	audit := &fakeAudit{}
	r := NewNoteRemover(kb, audit, testClock, nil)

	// referenced + no force -> refuse, not deleted
	if err := r.Remove("a", []string{"t2"}, false, "", "", false); err == nil {
		t.Fatal("referenced note must refuse without --force")
	}
	// force without who/why -> ErrMissingAttribution
	if err := r.Remove("a", []string{"t2"}, true, "", "", false); !errors.Is(err, ErrMissingAttribution) {
		t.Fatalf("force needs who/why: %v", err)
	}
	// force with who/why -> audited + deleted
	if err := r.Remove("a", []string{"t2"}, true, "me", "cleanup", false); err != nil {
		t.Fatal(err)
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "note rm --force" {
		t.Fatalf("audit: %+v", audit.entries)
	}

	// unreferenced -> plain delete, no force
	kb2 := newFakeKB()
	_, _ = kb2.CreateNote(mtt.Note{Slug: "b"})
	if err := NewNoteRemover(kb2, &fakeAudit{}, testClock, nil).Remove("b", nil, false, "", "", false); err != nil {
		t.Fatal(err)
	}
	// missing note -> ErrNotFound
	if err := NewNoteRemover(newFakeKB(), &fakeAudit{}, testClock, nil).Remove("z", nil, false, "", "", false); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestNoteRemoveNoRunPreflight(t *testing.T) {
	kb := newFakeKB()
	if _, err := NewNoteAdder(kb, testClock, nil).Add(NoteParams{Slug: "a", Title: "x"}); err != nil {
		t.Fatal(err)
	}
	err := NewNoteRemover(kb, &fakeAudit{}, testClock, nil).Remove("a", nil, false, "", "", true)
	if !errors.Is(err, ErrMissingAttribution) {
		t.Fatalf("want ErrMissingAttribution, got %v", err)
	}
	if _, gerr := kb.GetNote("a"); gerr != nil {
		t.Fatal("preflight must run BEFORE any deletion")
	}
}

func TestNoteRemoveForceNoRunOneRecord(t *testing.T) {
	kb := newFakeKB()
	if _, err := NewNoteAdder(kb, testClock, nil).Add(NoteParams{Slug: "a", Title: "x"}); err != nil {
		t.Fatal(err)
	}
	audit := &fakeAudit{}
	var ev mtt.Events
	ev.Note.Delete = mtt.EventHook{Post: strCmds([]string{"echo hi"})}
	runner := &fakeRunner{}
	r := NewNoteRemover(kb, audit, testClock, NewEventEmitter(eventCfg(ev), runner, audit, testClock))
	if err := r.Remove("a", nil, true, "me", "sign", true); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if runner.called {
		t.Fatal("--no-run must skip the pipeline")
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "note rm --force --no-run" {
		t.Fatalf("want ONE record (pin iii), got %+v", audit.entries)
	}
}
