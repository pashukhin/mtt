package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// RefEditor mutates a task's Refs (informational, non-blocking) and persists via
// TaskStore.Update. No new port — refs ride the Task field (GAP #1, like DependsOn).
type RefEditor struct {
	store mtt.TaskStore
	now   func() time.Time
}

// NewRefEditor wires the usecase.
func NewRefEditor(store mtt.TaskStore, now func() time.Time) *RefEditor {
	return &RefEditor{store: store, now: now}
}

// AddRef upserts r on task id by its natural key (setLabel = --label was given),
// bumps Updated, persists.
func (e *RefEditor) AddRef(id mtt.TaskID, r mtt.Ref, setLabel bool) (mtt.Task, error) {
	t, err := e.load(id)
	if err != nil {
		return mtt.Task{}, err
	}
	t.Refs = upsertRef(t.Refs, r, setLabel)
	t.Updated = e.now().UTC().Truncate(time.Second)
	return e.store.Update(t)
}

// RemoveRef drops the (kind,target) ref from task id; an absent key is an
// idempotent no-op (no write).
func (e *RefEditor) RemoveRef(id mtt.TaskID, kind mtt.RefKind, target string) (mtt.Task, error) {
	t, err := e.load(id)
	if err != nil {
		return mtt.Task{}, err
	}
	refs, found := removeRef(t.Refs, kind, target)
	if !found {
		return t, nil
	}
	t.Refs = refs
	t.Updated = e.now().UTC().Truncate(time.Second)
	return e.store.Update(t)
}

func (e *RefEditor) load(id mtt.TaskID) (mtt.Task, error) {
	t, err := e.store.Get(id)
	if err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return mtt.Task{}, fmt.Errorf("task %q: %w", id, mtt.ErrNotFound)
		}
		return mtt.Task{}, fmt.Errorf("load task %q: %w", id, err)
	}
	return t, nil
}

// NoteRefEditor is the note analogue over KnowledgeStore.
type NoteRefEditor struct {
	store mtt.KnowledgeStore
	now   func() time.Time
}

// NewNoteRefEditor wires the usecase.
func NewNoteRefEditor(store mtt.KnowledgeStore, now func() time.Time) *NoteRefEditor {
	return &NoteRefEditor{store: store, now: now}
}

// AddRef upserts r on note slug, bumps Updated, persists.
func (e *NoteRefEditor) AddRef(slug mtt.NoteSlug, r mtt.Ref, setLabel bool) (mtt.Note, error) {
	n, err := e.store.GetNote(slug)
	if err != nil {
		return mtt.Note{}, err // GetNote returns bare ErrNotFound; the CLI wraps to noteNotFound
	}
	n.Refs = upsertRef(n.Refs, r, setLabel)
	n.Updated = e.now().UTC()
	return e.store.UpdateNote(n)
}

// RemoveRef drops the (kind,target) ref from note slug; an absent key is an
// idempotent no-op.
func (e *NoteRefEditor) RemoveRef(slug mtt.NoteSlug, kind mtt.RefKind, target string) (mtt.Note, error) {
	n, err := e.store.GetNote(slug)
	if err != nil {
		return mtt.Note{}, err
	}
	refs, found := removeRef(n.Refs, kind, target)
	if !found {
		return n, nil
	}
	n.Refs = refs
	n.Updated = e.now().UTC()
	return e.store.UpdateNote(n)
}
