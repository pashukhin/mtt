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
	ev    *EventEmitter
}

// NewTagEditor wires the usecase; ev fires the update event (nil = none).
func NewTagEditor(store mtt.TaskStore, now func() time.Time, ev *EventEmitter) *TagEditor {
	return &TagEditor{store: store, now: now, ev: ev}
}

// AddTags unions the (already-normalized) tags into the task's canonical set.
// Adding only already-present tags is an idempotent no-op (no write, no timestamp
// bump). On a real change it bumps Updated and persists. The second return is the
// tags actually added (canonical order; nil on a no-op) — so callers can report
// only the real effect rather than the requested set (c14).
func (e *TagEditor) AddTags(id mtt.TaskID, tags []string, opts EventOptions) (mtt.Task, []string, error) {
	if err := opts.Preflight(); err != nil {
		return mtt.Task{}, nil, err
	}
	t, err := e.load(id)
	if err != nil {
		return mtt.Task{}, nil, err
	}
	merged := canonicalTags(t.Tags, tags)
	added := subtractTags(merged, t.Tags)
	if len(added) == 0 {
		return t, nil, nil // no persist ⇒ no event, no skip record
	}
	t.Tags = merged
	t.Updated = e.now().UTC().Truncate(time.Second)
	up, err := e.store.Update(t)
	if err != nil {
		return mtt.Task{}, nil, err
	}
	return up, added, e.ev.TaskEvent(mtt.EventUpdate, up, "tag add", opts)
}

// RemoveTags removes the (already-normalized) tags from the task's set. GUARD: a
// tag whose #hashtag is still present in the title or description is refused — all
// targets are validated BEFORE any change, so a multi-tag call is atomic. Removing
// an absent (unguarded) tag is an idempotent no-op. On a real change it bumps
// Updated and persists. The second return is the tags actually removed (canonical
// order; nil on a no-op) — callers report only the real effect (c14).
func (e *TagEditor) RemoveTags(id mtt.TaskID, tags []string, opts EventOptions) (mtt.Task, []string, error) {
	if err := opts.Preflight(); err != nil {
		return mtt.Task{}, nil, err
	}
	t, err := e.load(id)
	if err != nil {
		return mtt.Task{}, nil, err
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
			return mtt.Task{}, nil, fmt.Errorf("cannot remove tag %q: #%s is present in the %s (edit the text to remove it)", tag, tag, field)
		}
	}
	remove := tagSet(tags)
	kept := make([]string, 0, len(t.Tags))
	var removed []string
	for _, tag := range t.Tags { // t.Tags is a canonical (sorted) set, so removed stays sorted
		if remove[tag] {
			removed = append(removed, tag)
		} else {
			kept = append(kept, tag)
		}
	}
	if len(removed) == 0 {
		return t, nil, nil // no persist ⇒ no event, no skip record
	}
	t.Tags = canonicalTags(kept)
	t.Updated = e.now().UTC().Truncate(time.Second)
	up, err := e.store.Update(t)
	if err != nil {
		return mtt.Task{}, nil, err
	}
	return up, removed, e.ev.TaskEvent(mtt.EventUpdate, up, "tag rm", opts)
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

// contains reports whether s contains v.
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
