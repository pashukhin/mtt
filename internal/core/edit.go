package core

import (
	"fmt"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Editor is the edit-a-task usecase (a mutation): it loads a task, applies the
// requested non-flow field changes, bumps Updated via the injected clock, and
// persists via TaskStore.Update.
type Editor struct {
	store mtt.TaskStore
	now   func() time.Time
}

// NewEditor wires the usecase. now is injected for deterministic timestamps.
func NewEditor(store mtt.TaskStore, now func() time.Time) *Editor {
	return &Editor{store: store, now: now}
}

// EditParams are the requested edits. A nil pointer means "leave unchanged"; a
// non-nil pointer (including to "") means "set to this value". Only title and
// description are editable: id/type are immutable, status moves through flow
// enforcement, and re-parenting is a separate operation.
type EditParams struct {
	Title       *string
	Description *string
}

// Edit applies p to task id, bumps Updated, persists, and returns the task.
func (e *Editor) Edit(id mtt.TaskID, p EditParams) (mtt.Task, error) {
	if p.Title == nil && p.Description == nil {
		return mtt.Task{}, fmt.Errorf("nothing to edit: provide --title and/or --description")
	}
	t, err := e.store.Get(id)
	if err != nil {
		return mtt.Task{}, err
	}
	if p.Title != nil {
		t.Title = *p.Title
	}
	if p.Description != nil {
		t.Description = *p.Description
	}
	if t.Title == "" && t.Description == "" {
		return mtt.Task{}, fmt.Errorf("a task needs a title or a description")
	}
	t.Updated = e.now().UTC().Truncate(time.Second)
	return e.store.Update(t)
}
