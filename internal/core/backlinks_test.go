package core

import (
	"reflect"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func refFixtures() ([]mtt.Task, []mtt.Note) {
	tasks := []mtt.Task{
		{ID: "t1"},
		{ID: "t2", Refs: []mtt.Ref{{Kind: mtt.RefTask, ID: "t1", Label: "blocks"}, {Kind: mtt.RefTask, ID: "t9"}}},
	}
	notes := []mtt.Note{
		{Slug: "a", Refs: []mtt.Ref{{Kind: mtt.RefTask, ID: "t1"}, {Kind: mtt.RefURL, ID: "https://x/"}}},
	}
	return tasks, notes
}

func TestBacklinksTo(t *testing.T) {
	tasks, notes := refFixtures()
	bl := NewBacklinks(tasks, notes)
	got := bl.To(mtt.RefTask, "t1")
	want := []Referent{{Carrier: mtt.RefNote, ID: "a"}, {Carrier: mtt.RefTask, ID: "t2", Label: "blocks"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("backlinks to t1: got %+v want %+v", got, want)
	}
	if len(bl.To(mtt.RefTask, "t9")) != 1 {
		t.Fatal("t9 should have one backlink (from t2)")
	}
	if bl.To(mtt.RefNote, "nobody") != nil {
		t.Fatal("absent target -> nil")
	}
}

func TestCheckRefs(t *testing.T) {
	tasks, notes := refFixtures()
	got := CheckRefs(tasks, notes, true)
	// dangling: t2->t9 ; unverified: a->url
	var dangling, unverified int
	for _, f := range got {
		switch f.Status {
		case RefDangling:
			dangling++
		case RefUnverified:
			unverified++
		default:
			t.Fatalf("ok refs must not appear: %+v", f)
		}
	}
	if dangling != 1 || unverified != 1 {
		t.Fatalf("got dangling=%d unverified=%d in %+v", dangling, unverified, got)
	}
}

func TestCheckRefsNoKB(t *testing.T) {
	tasks := []mtt.Task{{ID: "t1", Refs: []mtt.Ref{{Kind: mtt.RefNote, ID: "a"}}}}
	got := CheckRefs(tasks, nil, false) // kb not wired
	if len(got) != 1 || got[0].Status != RefUnverified {
		t.Fatalf("note ref with no KB must be unverified: %+v", got)
	}
}
