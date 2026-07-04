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
func (s *editStore) Get(string) (mtt.Task, error)        { return s.get, s.getErr }
func (s *editStore) List() ([]mtt.Task, error)           { return nil, nil }
func (s *editStore) Update(t mtt.Task) (mtt.Task, error) { s.updated = t; return t, nil }

func strptr(s string) *string { return &s }
func later() time.Time        { return time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC) }

func TestEditTitleOnly(t *testing.T) {
	orig := mtt.Task{ID: "e1", Type: "epic", Title: "old", Status: "tbd", Description: "d",
		Created: fixed(), Updated: fixed()}
	fs := &editStore{get: orig}
	got, err := NewEditor(fs, later).Edit("e1", EditParams{Title: strptr("new")})
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
	_, err := NewEditor(&editStore{}, later).Edit("e1", EditParams{})
	if err == nil || !strings.Contains(err.Error(), "nothing to edit") {
		t.Fatalf("want 'nothing to edit', got %v", err)
	}
}

func TestEditEmptyingBothRejected(t *testing.T) {
	orig := mtt.Task{ID: "e1", Type: "epic", Title: "old", Status: "tbd", Created: fixed(), Updated: fixed()}
	_, err := NewEditor(&editStore{get: orig}, later).Edit("e1", EditParams{Title: strptr("")})
	if err == nil || !strings.Contains(err.Error(), "title or a description") {
		t.Fatalf("want content invariant error, got %v", err)
	}
}

func TestEditNotFoundPropagates(t *testing.T) {
	fs := &editStore{getErr: mtt.ErrNotFound}
	_, err := NewEditor(fs, later).Edit("ghost", EditParams{Title: strptr("x")})
	if !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
