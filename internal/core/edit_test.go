package core

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// editStore is a fake TaskStore: Get returns a seeded task, Update records it.
type editStore struct {
	get     mtt.Task
	getErr  error
	updated mtt.Task
}

func (s *editStore) Create(t mtt.Task) (mtt.Task, error) { return t, nil }
func (s *editStore) Get(mtt.TaskID) (mtt.Task, error)    { return s.get, s.getErr }
func (s *editStore) List() ([]mtt.Task, error)           { return nil, nil }
func (s *editStore) Update(t mtt.Task) (mtt.Task, error) { s.updated = t; return t, nil }
func (s *editStore) Delete(mtt.TaskID) error             { return nil }

func strptr(s string) *string { return &s }
func later() time.Time        { return time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC) }

func TestEditTitleOnly(t *testing.T) {
	orig := mtt.Task{ID: "e1", Type: "epic", Title: "old", Status: "tbd", Description: "d",
		Created: fixed(), Updated: fixed()}
	fs := &editStore{get: orig}
	got, err := NewEditor(fs, later, nil).Edit("e1", EditParams{Title: strptr("new")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "new" || got.Description != "d" {
		t.Fatalf("title/desc = %q/%q", got.Title, got.Description)
	}
	if !got.Updated.Equal(later().Truncate(time.Second)) {
		t.Fatalf("updated not bumped: %v", got.Updated)
	}
	if !got.Created.Equal(fixed()) {
		t.Fatalf("created changed: %v", got.Created)
	}
	if !fs.updated.Updated.Equal(got.Updated) {
		t.Fatal("store.Update did not receive the bumped task")
	}
}

func TestEditNothing(t *testing.T) {
	_, err := NewEditor(&editStore{}, later, nil).Edit("e1", EditParams{})
	if err == nil || !strings.Contains(err.Error(), "nothing to edit") {
		t.Fatalf("want 'nothing to edit', got %v", err)
	}
}

func TestEditEmptyingBothRejected(t *testing.T) {
	orig := mtt.Task{ID: "e1", Type: "epic", Title: "old", Status: "tbd", Created: fixed(), Updated: fixed()}
	_, err := NewEditor(&editStore{get: orig}, later, nil).Edit("e1", EditParams{Title: strptr("")})
	if err == nil || !strings.Contains(err.Error(), "title or a description") {
		t.Fatalf("want content invariant error, got %v", err)
	}
}

func TestEditRejectsNewlineInTitle(t *testing.T) {
	orig := mtt.Task{ID: "t1", Type: "task", Title: "old", Status: "tbd", Created: fixed(), Updated: fixed()}
	_, err := NewEditor(&editStore{get: orig}, later, nil).Edit("t1", EditParams{Title: strptr("a\nb")})
	if err == nil || !strings.Contains(err.Error(), "single line") {
		t.Fatalf("want single-line title error, got %v", err)
	}
	// a newline in the description is allowed (free-form)
	if _, err := NewEditor(&editStore{get: orig}, later, nil).Edit("t1", EditParams{Description: strptr("multi\nline")}); err != nil {
		t.Fatalf("newline in description should be allowed: %v", err)
	}
}

func TestEditNotFoundPropagates(t *testing.T) {
	fs := &editStore{getErr: mtt.ErrNotFound}
	_, err := NewEditor(fs, later, nil).Edit("ghost", EditParams{Title: strptr("x")})
	if !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func prioptr(p mtt.Priority) *mtt.Priority { return &p }

func TestEditPriorityOnly(t *testing.T) {
	// A priority-only edit is allowed (the guard permits it) and does not need
	// --title/--description.
	orig := mtt.Task{ID: "t1", Type: "task", Title: "a", Status: "tbd", Created: fixed(), Updated: fixed()}
	got, err := NewEditor(&editStore{get: orig}, later, nil).Edit("t1", EditParams{Priority: prioptr(mtt.PriorityHigh)})
	if err != nil {
		t.Fatalf("priority-only edit: %v", err)
	}
	if got.Priority != mtt.PriorityHigh {
		t.Fatalf("Priority = %q, want high", got.Priority)
	}
}

func TestEditPriorityNilUnchanged(t *testing.T) {
	orig := mtt.Task{ID: "t1", Type: "task", Title: "a", Status: "tbd", Priority: mtt.PriorityLow, Created: fixed(), Updated: fixed()}
	got, err := NewEditor(&editStore{get: orig}, later, nil).Edit("t1", EditParams{Title: strptr("b")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Priority != mtt.PriorityLow {
		t.Fatalf("Priority = %q, want low (unchanged when nil)", got.Priority)
	}
}

func TestEditPriorityClear(t *testing.T) {
	// edit --priority "" clears the priority back to unset (empty is Valid; the
	// pointer is non-nil so it applies).
	orig := mtt.Task{ID: "t1", Type: "task", Title: "a", Status: "tbd", Priority: mtt.PriorityHigh, Created: fixed(), Updated: fixed()}
	got, err := NewEditor(&editStore{get: orig}, later, nil).Edit("t1", EditParams{Priority: prioptr("")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Priority != "" {
		t.Fatalf("Priority = %q, want unset", got.Priority)
	}
}

func TestEditNothingIncludesPriorityGuard(t *testing.T) {
	orig := mtt.Task{ID: "t1", Type: "task", Title: "a", Status: "tbd", Created: fixed(), Updated: fixed()}
	if _, err := NewEditor(&editStore{get: orig}, later, nil).Edit("t1", EditParams{}); err == nil {
		t.Fatal("empty EditParams should error (nothing to edit)")
	}
}

func TestEditFiresUpdateEvent(t *testing.T) {
	cfg := eventCfg(taskHook(mtt.EventUpdate, "echo {{.ID}} {{.Event}}"))
	store := newMemStore(tbdTask("t1"))
	runner := &fakeRunner{}
	ed := NewEditor(store, testClock, NewEventEmitter(cfg, runner, &fakeAudit{}, testClock))
	title := "renamed"
	if _, err := ed.Edit("t1", EditParams{Title: &title}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(runner.gotCmds) != 1 || runner.gotCmds[0].Run != "echo t1 update" {
		t.Fatalf("pipeline = %+v, want [echo t1 update]", runner.gotCmds)
	}
}

func TestEditNoRunPreflight(t *testing.T) {
	store := newMemStore(tbdTask("t1"))
	ed := NewEditor(store, testClock, NewEventEmitter(eventCfg(mtt.Events{}), &fakeRunner{}, &fakeAudit{}, testClock))
	title := "renamed"
	_, err := ed.Edit("t1", EditParams{Title: &title, Events: EventOptions{NoRun: true}})
	if !errors.Is(err, ErrMissingAttribution) {
		t.Fatalf("want ErrMissingAttribution, got %v", err)
	}
	if got, _ := store.Get("t1"); got.Title == "renamed" {
		t.Fatal("preflight must run BEFORE persist")
	}
}
