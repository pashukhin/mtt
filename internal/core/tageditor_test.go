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
	got, added, err := NewTagEditor(fs, later, nil).AddTags("t1", []string{"backend", "urgent"}, EventOptions{})
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
	got, added, err := NewTagEditor(fs, later, nil).AddTags("t1", []string{"auth"}, EventOptions{})
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
	got, removed, err := NewTagEditor(&editStore{get: orig}, later, nil).RemoveTags("t1", []string{"urgent"}, EventOptions{})
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
	_, _, err := NewTagEditor(fs, later, nil).RemoveTags("t1", []string{"auth"}, EventOptions{})
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
	_, _, err := NewTagEditor(&editStore{get: orig}, later, nil).RemoveTags("t1", []string{"auth"}, EventOptions{})
	if err == nil || !strings.Contains(err.Error(), "#auth is present in the description") {
		t.Fatalf("want description guard, got %v", err)
	}
}

func TestTagRemoveMultiAtomic(t *testing.T) {
	// One guarded target blocks the whole call — no partial write.
	orig := mtt.Task{ID: "t1", Type: "task", Title: "fix #auth", Status: "tbd",
		Tags: []string{"auth", "urgent"}, Created: fixed(), Updated: fixed()}
	fs := &editStore{get: orig}
	if _, _, err := NewTagEditor(fs, later, nil).RemoveTags("t1", []string{"urgent", "auth"}, EventOptions{}); err == nil {
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
	got, removed, err := NewTagEditor(fs, later, nil).RemoveTags("t1", []string{"ghost"}, EventOptions{})
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
	if _, _, err := NewTagEditor(fs, later, nil).AddTags("ghost", []string{"x"}, EventOptions{}); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("AddTags want ErrNotFound, got %v", err)
	}
	if _, _, err := NewTagEditor(fs, later, nil).RemoveTags("ghost", []string{"x"}, EventOptions{}); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("RemoveTags want ErrNotFound, got %v", err)
	}
}

func TestTagAddFiresUpdateEvent(t *testing.T) {
	cfg := eventCfg(taskHook(mtt.EventUpdate, "echo {{.ID}} {{.Event}}"))
	store := newMemStore(tbdTask("t1"))
	runner := &fakeRunner{}
	te := NewTagEditor(store, testClock, NewEventEmitter(cfg, runner, &fakeAudit{}, testClock))
	if _, _, err := te.AddTags("t1", []string{"x"}, EventOptions{}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(runner.gotCmds) != 1 || runner.gotCmds[0].Run != "echo t1 update" {
		t.Fatalf("pipeline = %+v", runner.gotCmds)
	}
}

func TestTagAddNoOpFiresNoEvent(t *testing.T) {
	cfg := eventCfg(taskHook(mtt.EventUpdate, "echo hi"))
	task := tbdTask("t1")
	task.Tags = []string{"x"}
	runner := &fakeRunner{}
	te := NewTagEditor(newMemStore(task), testClock, NewEventEmitter(cfg, runner, &fakeAudit{}, testClock))
	if _, _, err := te.AddTags("t1", []string{"x"}, EventOptions{}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if runner.called {
		t.Fatal("idempotent no-op must not fire an event")
	}
}

func TestTagAddNoOpNoRunWritesNoRecord(t *testing.T) {
	task := tbdTask("t1")
	task.Tags = []string{"x"}
	audit := &fakeAudit{}
	te := NewTagEditor(newMemStore(task), testClock, NewEventEmitter(eventCfg(mtt.Events{}), &fakeRunner{}, audit, testClock))
	if _, _, err := te.AddTags("t1", []string{"x"}, EventOptions{NoRun: true, By: "a", Why: "b"}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(audit.entries) != 0 {
		t.Fatalf("no persist ⇒ no skip record; got %+v", audit.entries)
	}
}
