package core

import (
	"reflect"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestCanonicalRefsDedupSort(t *testing.T) {
	in := []mtt.Ref{
		{Kind: mtt.RefTask, ID: "t2"},
		{Kind: mtt.RefNote, ID: "a"},
		{Kind: mtt.RefTask, ID: "t2", Label: "new"}, // dup key, last label wins
	}
	got := canonicalRefs(in)
	want := []mtt.Ref{{Kind: mtt.RefNote, ID: "a"}, {Kind: mtt.RefTask, ID: "t2", Label: "new"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestUpsertRef(t *testing.T) {
	refs := []mtt.Ref{{Kind: mtt.RefTask, ID: "t2", Label: "old"}}
	// re-add with label -> overwrite
	got := upsertRef(refs, mtt.Ref{Kind: mtt.RefTask, ID: "t2", Label: "new"}, true)
	if got[0].Label != "new" {
		t.Fatalf("label overwrite: %+v", got)
	}
	// re-add without label -> unchanged (idempotent)
	got = upsertRef(refs, mtt.Ref{Kind: mtt.RefTask, ID: "t2"}, false)
	if got[0].Label != "old" {
		t.Fatalf("no-label re-add must not clear: %+v", got)
	}
	// new key -> appended
	got = upsertRef(refs, mtt.Ref{Kind: mtt.RefURL, ID: "https://x/"}, false)
	if len(got) != 2 {
		t.Fatalf("append: %+v", got)
	}
}

func TestRemoveRef(t *testing.T) {
	refs := []mtt.Ref{{Kind: mtt.RefTask, ID: "t2"}, {Kind: mtt.RefNote, ID: "a"}}
	got, found := removeRef(refs, mtt.RefTask, "t2")
	if !found || len(got) != 1 || got[0].Kind != mtt.RefNote {
		t.Fatalf("remove: %+v found=%v", got, found)
	}
	if _, found := removeRef(refs, mtt.RefTask, "nope"); found {
		t.Fatal("absent key must report not-found")
	}
}

func TestVerifyRef(t *testing.T) {
	taskExists := func(id mtt.TaskID) bool { return id == "t2" }
	noteExists := func(s mtt.NoteSlug) bool { return s == "a" }
	cases := []struct {
		r    mtt.Ref
		ne   func(mtt.NoteSlug) bool
		want RefStatus
	}{
		{mtt.Ref{Kind: mtt.RefTask, ID: "t2"}, noteExists, RefOK},
		{mtt.Ref{Kind: mtt.RefTask, ID: "t9"}, noteExists, RefDangling},
		{mtt.Ref{Kind: mtt.RefNote, ID: "a"}, noteExists, RefOK},
		{mtt.Ref{Kind: mtt.RefNote, ID: "z"}, noteExists, RefDangling},
		{mtt.Ref{Kind: mtt.RefNote, ID: "a"}, nil, RefUnverified}, // no KB wired
		{mtt.Ref{Kind: mtt.RefURL, ID: "https://x/"}, noteExists, RefUnverified},
	}
	for _, c := range cases {
		if got := VerifyRef(c.r, taskExists, c.ne); got != c.want {
			t.Fatalf("%+v: got %q want %q", c.r, got, c.want)
		}
	}
}
