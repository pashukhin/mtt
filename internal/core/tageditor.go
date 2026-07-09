package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// TagEditor mutates a task's tags and persists via TaskStore.Update. No new port —
// tags ride the Task.Tags field (GAP #1, like depends_on). now is injected for
// deterministic tests.
type TagEditor struct {
	store mtt.TaskStore
	now   func() time.Time
}

// NewTagEditor wires the usecase.
func NewTagEditor(store mtt.TaskStore, now func() time.Time) *TagEditor {
	return &TagEditor{store: store, now: now}
}

// AddTags unions the (already-normalized) tags into the task's canonical set.
// Adding only already-present tags is an idempotent no-op (no write, no timestamp
// bump). On a real change it bumps Updated and persists.
func (e *TagEditor) AddTags(id mtt.TaskID, tags []string) (mtt.Task, error) {
	t, err := e.load(id)
	if err != nil {
		return mtt.Task{}, err
	}
	merged := canonicalTags(t.Tags, tags)
	if sameTags(t.Tags, merged) {
		return t, nil
	}
	t.Tags = merged
	t.Updated = e.now().UTC().Truncate(time.Second)
	return e.store.Update(t)
}

// RemoveTags removes the (already-normalized) tags from the task's set. GUARD: a
// tag whose #hashtag is still present in the title or description is refused — all
// targets are validated BEFORE any change, so a multi-tag call is atomic. Removing
// an absent (unguarded) tag is an idempotent no-op. On a real change it bumps
// Updated and persists.
func (e *TagEditor) RemoveTags(id mtt.TaskID, tags []string) (mtt.Task, error) {
	t, err := e.load(id)
	if err != nil {
		return mtt.Task{}, err
	}
	titleTags := mtt.ExtractTags(t.Title)
	descTags := mtt.ExtractTags(t.Description)
	anchored := tagSet(titleTags, descTags)
	for _, tag := range tags {
		if anchored[tag] {
			field := "description"
			if contains(titleTags, tag) {
				field = "title"
			}
			return mtt.Task{}, fmt.Errorf("cannot remove tag %q: #%s is present in the %s (edit the text to remove it)", tag, tag, field)
		}
	}
	remove := tagSet(tags)
	kept := make([]string, 0, len(t.Tags))
	for _, tag := range t.Tags {
		if !remove[tag] {
			kept = append(kept, tag)
		}
	}
	next := canonicalTags(kept)
	if sameTags(t.Tags, next) {
		return t, nil
	}
	t.Tags = next
	t.Updated = e.now().UTC().Truncate(time.Second)
	return e.store.Update(t)
}

// load fetches a task, wrapping a missing id as a matchable ErrNotFound (the CLI
// maps it to exit 4 — the uniform not-found taxonomy). Never use %v here.
func (e *TagEditor) load(id mtt.TaskID) (mtt.Task, error) {
	t, err := e.store.Get(id)
	if err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return mtt.Task{}, fmt.Errorf("task %q: %w", id, mtt.ErrNotFound)
		}
		return mtt.Task{}, fmt.Errorf("load task %q: %w", id, err)
	}
	return t, nil
}

// sameTags reports whether two canonical (sorted) tag slices are equal.
func sameTags(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// contains reports whether s contains v.
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
