package core

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestTagAddUnionsAndBumps(t *testing.T) {
	orig := mtt.Task{ID: "t1", Type: "task", Title: "a", Status: "tbd",
		Tags: []string{"auth"}, Created: fixed(), Updated: fixed()}
	fs := &editStore{get: orig}
	got, added, err := NewTagEditor(fs, later).AddTags("t1", []string{"backend", "urgent"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Tags, []string{"auth", "backend", "urgent"}) {
		t.Fatalf("Tags = %v; want [auth backend urgent]", got.Tags)
	}
	if !reflect.DeepEqual(added, []string{"backend", "urgent"}) {
		t.Fatalf("added = %v; want [backend urgent] (only the actually-added)", added)
	}
	if fs.updated.ID == "" {
		t.Fatal("a real change must persist via Update")
	}
}

func TestTagAddIdempotentNoWrite(t *testing.T) {
	orig := mtt.Task{ID: "t1", Type: "task", Title: "a", Status: "tbd",
		Tags: []string{"auth", "urgent"}, Created: fixed(), Updated: fixed()}
	fs := &editStore{get: orig}
	got, added, err := NewTagEditor(fs, later).AddTags("t1", []string{"auth"})
	if err != nil {
		t.Fatal(err)
	}
	if fs.updated.ID != "" {
		t.Fatal("adding an existing tag must not write")
	}
	if len(added) != 0 {
		t.Fatalf("added = %v; want none on a no-op", added)
	}
	if !got.Updated.Equal(fixed()) {
		t.Fatal("Updated must not bump on a no-op")
	}
}

func TestTagRemoveManualTag(t *testing.T) {
	orig := mtt.Task{ID: "t1", Type: "task", Title: "a", Status: "tbd",
		Tags: []string{"auth", "urgent"}, Created: fixed(), Updated: fixed()}
	got, removed, err := NewTagEditor(&editStore{get: orig}, later).RemoveTags("t1", []string{"urgent"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Tags, []string{"auth"}) {
		t.Fatalf("Tags = %v; want [auth]", got.Tags)
	}
	if !reflect.DeepEqual(removed, []string{"urgent"}) {
		t.Fatalf("removed = %v; want [urgent]", removed)
	}
}

func TestTagRemoveGuardTitle(t *testing.T) {
	orig := mtt.Task{ID: "t1", Type: "task", Title: "fix #auth", Status: "tbd",
		Tags: []string{"auth"}, Created: fixed(), Updated: fixed()}
	fs := &editStore{get: orig}
	_, _, err := NewTagEditor(fs, later).RemoveTags("t1", []string{"auth"})
	if err == nil || !strings.Contains(err.Error(), "#auth is present in the title") {
		t.Fatalf("want title guard, got %v", err)
	}
	if fs.updated.ID != "" {
		t.Fatal("a guarded removal must not write")
	}
}

func TestTagRemoveGuardDescription(t *testing.T) {
	orig := mtt.Task{ID: "t1", Type: "task", Title: "a", Description: "see #auth", Status: "tbd",
		Tags: []string{"auth"}, Created: fixed(), Updated: fixed()}
	_, _, err := NewTagEditor(&editStore{get: orig}, later).RemoveTags("t1", []string{"auth"})
	if err == nil || !strings.Contains(err.Error(), "#auth is present in the description") {
		t.Fatalf("want description guard, got %v", err)
	}
}

func TestTagRemoveMultiAtomic(t *testing.T) {
	// One guarded target blocks the whole call — no partial write.
	orig := mtt.Task{ID: "t1", Type: "task", Title: "fix #auth", Status: "tbd",
		Tags: []string{"auth", "urgent"}, Created: fixed(), Updated: fixed()}
	fs := &editStore{get: orig}
	if _, _, err := NewTagEditor(fs, later).RemoveTags("t1", []string{"urgent", "auth"}); err == nil {
		t.Fatal("want guard error")
	}
	if fs.updated.ID != "" {
		t.Fatal("atomic: no write when any target is guarded")
	}
}

func TestTagRemoveAbsentIsNoOp(t *testing.T) {
	orig := mtt.Task{ID: "t1", Type: "task", Title: "a", Status: "tbd",
		Tags: []string{"auth"}, Created: fixed(), Updated: fixed()}
	fs := &editStore{get: orig}
	got, removed, err := NewTagEditor(fs, later).RemoveTags("t1", []string{"ghost"})
	if err != nil {
		t.Fatal(err)
	}
	if fs.updated.ID != "" {
		t.Fatal("removing an absent tag must not write")
	}
	if len(removed) != 0 {
		t.Fatalf("removed = %v; want none (absent tag is a no-op)", removed)
	}
	if !reflect.DeepEqual(got.Tags, []string{"auth"}) {
		t.Fatalf("Tags = %v; want [auth]", got.Tags)
	}
}

func TestTagEditorNotFoundWrapped(t *testing.T) {
	fs := &editStore{getErr: mtt.ErrNotFound}
	if _, _, err := NewTagEditor(fs, later).AddTags("ghost", []string{"x"}); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("AddTags want ErrNotFound, got %v", err)
	}
	if _, _, err := NewTagEditor(fs, later).RemoveTags("ghost", []string{"x"}); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("RemoveTags want ErrNotFound, got %v", err)
	}
}
