package core

import (
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestPrimeThresholdOrderCap(t *testing.T) {
	notes := []mtt.Note{
		{Slug: "h1", Priority: mtt.PriorityHigh, Created: time.Unix(10, 0)},
		{Slug: "h2", Priority: mtt.PriorityHigh, Created: time.Unix(20, 0)},
		{Slug: "m1", Priority: mtt.PriorityMedium, Created: time.Unix(30, 0)},
		{Slug: "un", Created: time.Unix(40, 0)}, // unset — never primed
	}
	// h1 referenced by two carriers, h2 by none -> h1 first on the backlink tiebreak
	tasks := []mtt.Task{
		{ID: "t1", Refs: []mtt.Ref{{Kind: mtt.RefNote, ID: "h1"}}},
		{ID: "t2", Refs: []mtt.Ref{{Kind: mtt.RefNote, ID: "h1"}}},
	}
	bl := NewBacklinks(tasks, notes)

	got, total := Prime(notes, bl, PrimeOptions{MinPriority: mtt.PriorityHigh, Limit: 0})
	if total != 2 || len(got) != 2 {
		t.Fatalf("high total/shown: total=%d got=%+v", total, got)
	}
	if got[0].Slug != "h1" || got[0].Backlinks != 2 || got[1].Slug != "h2" {
		t.Fatalf("backlink tiebreak: %+v", got)
	}
	got, total = Prime(notes, bl, PrimeOptions{MinPriority: mtt.PriorityMedium, Limit: 0})
	if total != 3 {
		t.Fatalf("medium total: %d", total)
	}
	for _, e := range got {
		if e.Slug == "un" {
			t.Fatal("unset note must never be primed")
		}
	}
	got, total = Prime(notes, bl, PrimeOptions{MinPriority: mtt.PriorityHigh, Limit: 1})
	if len(got) != 1 || total != 2 {
		t.Fatalf("cap: shown=%d total=%d", len(got), total)
	}
}

func TestPrimeCorruptPriority(t *testing.T) {
	notes := []mtt.Note{{Slug: "g", Priority: mtt.Priority("garbage")}}
	bl := NewBacklinks(nil, notes)
	if got, total := Prime(notes, bl, PrimeOptions{MinPriority: mtt.PriorityHigh}); total != 0 || len(got) != 0 {
		t.Fatalf("corrupt excluded at high: total=%d got=%+v", total, got)
	}
	if got, total := Prime(notes, bl, PrimeOptions{MinPriority: mtt.PriorityMedium}); total != 1 || len(got) != 1 {
		t.Fatalf("corrupt included at medium: total=%d got=%+v", total, got)
	}
}
