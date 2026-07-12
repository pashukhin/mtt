package core

import (
	"reflect"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestCanonicalTags(t *testing.T) {
	// Merge groups: normalize, dedup, sort. Invalid values dropped.
	got := canonicalTags([]string{"Urgent", "#auth"}, []string{"auth", "bad tag", "backend"})
	want := []string{"auth", "backend", "urgent"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("canonicalTags = %v; want %v", got, want)
	}
	if canonicalTags(nil, nil) != nil {
		t.Fatal("empty canonicalTags must be nil")
	}
}

func TestAddUnionsExplicitAndHashtags(t *testing.T) {
	fs := &fakeStore{retID: "e1"}
	got, err := NewAdder(fs, cfg(), fixed).Add(AddParams{
		Title: "fix #auth", Description: "see #backend", TypeName: "epic", Tags: []string{"urgent", "auth"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// explicit {urgent, auth} ∪ text {auth, backend} -> sorted, deduped.
	want := []string{"auth", "backend", "urgent"}
	if !reflect.DeepEqual(got.Tags, want) {
		t.Fatalf("Add Tags = %v; want %v", got.Tags, want)
	}
	// text is not rewritten.
	if got.Title != "fix #auth" {
		t.Fatalf("title rewritten: %q", got.Title)
	}
}

func TestEditReconcileDropsRemovedHashtag(t *testing.T) {
	orig := mtt.Task{ID: "e1", Type: "epic", Title: "fix #auth", Status: "tbd",
		Tags: []string{"auth", "urgent"}, Created: fixed(), Updated: fixed()}
	got, err := NewEditor(&editStore{get: orig}, later).Edit("e1", EditParams{Title: strptr("fix login")})
	if err != nil {
		t.Fatal(err)
	}
	// #auth left the text -> auth dropped; the manual tag urgent survives.
	if !reflect.DeepEqual(got.Tags, []string{"urgent"}) {
		t.Fatalf("Tags = %v; want [urgent]", got.Tags)
	}
}

func TestEditReconcileAddsNewHashtag(t *testing.T) {
	orig := mtt.Task{ID: "e1", Type: "epic", Title: "plain", Status: "tbd",
		Tags: []string{"urgent"}, Created: fixed(), Updated: fixed()}
	got, err := NewEditor(&editStore{get: orig}, later).Edit("e1", EditParams{Title: strptr("plain #api")})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Tags, []string{"api", "urgent"}) {
		t.Fatalf("Tags = %v; want [api urgent]", got.Tags)
	}
}

func TestEditReconcileDescriptionScanned(t *testing.T) {
	orig := mtt.Task{ID: "e1", Type: "epic", Title: "plain", Description: "old #db", Status: "tbd",
		Tags: []string{"db", "urgent"}, Created: fixed(), Updated: fixed()}
	got, err := NewEditor(&editStore{get: orig}, later).Edit("e1", EditParams{Description: strptr("new #cache")})
	if err != nil {
		t.Fatal(err)
	}
	// #db left the description -> dropped; #cache entered -> added; manual urgent kept.
	if !reflect.DeepEqual(got.Tags, []string{"cache", "urgent"}) {
		t.Fatalf("Tags = %v; want [cache urgent]", got.Tags)
	}
}

func TestEditReconcileManualTagSurvivesPriorityEdit(t *testing.T) {
	orig := mtt.Task{ID: "e1", Type: "epic", Title: "plain", Status: "tbd",
		Tags: []string{"urgent"}, Created: fixed(), Updated: fixed()}
	got, err := NewEditor(&editStore{get: orig}, later).Edit("e1", EditParams{Priority: prioptr(mtt.PriorityHigh)})
	if err != nil {
		t.Fatal(err)
	}
	// A priority-only edit touches no text -> tags untouched.
	if !reflect.DeepEqual(got.Tags, []string{"urgent"}) {
		t.Fatalf("Tags = %v; want [urgent]", got.Tags)
	}
}

func TestEditReconcileTextAndManualCollisionCorner(t *testing.T) {
	// A tag added BOTH via text (#x in title) and manually: removing #x from the
	// text drops it (no provenance) — the documented corner.
	orig := mtt.Task{ID: "e1", Type: "epic", Title: "do #x", Status: "tbd",
		Tags: []string{"x"}, Created: fixed(), Updated: fixed()}
	got, err := NewEditor(&editStore{get: orig}, later).Edit("e1", EditParams{Title: strptr("do it")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Tags != nil {
		t.Fatalf("Tags = %v; want nil (corner: text+manual x dropped with #x)", got.Tags)
	}
}

func TestTagCounts(t *testing.T) {
	tasks := []mtt.Task{
		{Tags: []string{"dx", "flow"}},
		{Tags: []string{"dx"}},
		{Tags: []string{"flow", "sec"}},
		{Tags: nil},
	}
	got := TagCounts(tasks)
	want := []TagCount{{"dx", 2}, {"flow", 2}, {"sec", 1}}
	if len(got) != len(want) {
		t.Fatalf("TagCounts len = %d, want %d (%+v)", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("TagCounts[%d] = %+v, want %+v", i, got[i], w)
		}
	}
	if len(TagCounts(nil)) != 0 {
		t.Error("TagCounts(nil) should be empty")
	}
}
