package mtt

import (
	"errors"
	"testing"
)

func TestRefKindValid(t *testing.T) {
	for _, k := range []RefKind{RefNote, RefTask, RefComment, RefURL} {
		if !k.Valid() {
			t.Fatalf("%q should be valid", k)
		}
	}
	if RefKind("bogus").Valid() {
		t.Fatal("bogus should be invalid")
	}
}

func TestErrNotFound(t *testing.T) {
	if ErrNotFound == nil || !errors.Is(ErrNotFound, ErrNotFound) {
		t.Fatal("ErrNotFound must be a usable sentinel")
	}
}

// compile-time: a Task carries the reserved collections.
var _ = Task{Refs: []Ref{{Kind: RefTask}}, Comments: []Comment{{Replies: nil}}, History: []HistoryEntry{{Checks: []Check{{Exit: 0}}}}}
