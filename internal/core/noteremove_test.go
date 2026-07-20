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
	r := NewNoteRemover(kb, audit, testClock)

	// referenced + no force -> refuse, not deleted
	if err := r.Remove("a", []string{"t2"}, false, "", ""); err == nil {
		t.Fatal("referenced note must refuse without --force")
	}
	// force without who/why -> ErrMissingAttribution
	if err := r.Remove("a", []string{"t2"}, true, "", ""); !errors.Is(err, ErrMissingAttribution) {
		t.Fatalf("force needs who/why: %v", err)
	}
	// force with who/why -> audited + deleted
	if err := r.Remove("a", []string{"t2"}, true, "me", "cleanup"); err != nil {
		t.Fatal(err)
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "note rm --force" {
		t.Fatalf("audit: %+v", audit.entries)
	}

	// unreferenced -> plain delete, no force
	kb2 := newFakeKB()
	_, _ = kb2.CreateNote(mtt.Note{Slug: "b"})
	if err := NewNoteRemover(kb2, &fakeAudit{}, testClock).Remove("b", nil, false, "", ""); err != nil {
		t.Fatal(err)
	}
	// missing note -> ErrNotFound
	if err := NewNoteRemover(newFakeKB(), &fakeAudit{}, testClock).Remove("z", nil, false, "", ""); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}
