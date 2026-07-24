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
	ev    *EventEmitter
}

// NewEditor wires the usecase. now is injected for deterministic timestamps;
// ev fires the update event (nil = none).
func NewEditor(store mtt.TaskStore, now func() time.Time, ev *EventEmitter) *Editor {
	return &Editor{store: store, now: now, ev: ev}
}

// EditParams are the requested edits. A nil pointer means "leave unchanged"; a
// non-nil pointer (including to "") means "set to this value". Only title and
// description are editable: id/type are immutable, status moves through flow
// enforcement, and re-parenting is a separate operation.
type EditParams struct {
	Title       *string
	Description *string
	Priority    *mtt.Priority
	Events      EventOptions // lifecycle-event bypass + attribution (t66)
}

// Edit applies p to task id, bumps Updated, persists, and returns the task. A
// *PostActionError return carries the PERSISTED task (the edit happened; only
// the lifecycle event's finalization failed — exit 5).
func (e *Editor) Edit(id mtt.TaskID, p EditParams) (mtt.Task, error) {
	if err := p.Events.Preflight(); err != nil {
		return mtt.Task{}, err
	}
	if p.Title == nil && p.Description == nil && p.Priority == nil {
		return mtt.Task{}, fmt.Errorf("nothing to edit: provide --title, --description, and/or --priority")
	}
	t, err := e.store.Get(id)
	if err != nil {
		return mtt.Task{}, err
	}
	oldTitle, oldDesc := t.Title, t.Description
	if p.Title != nil {
		if err := validateTitle(*p.Title); err != nil {
			return mtt.Task{}, err
		}
		t.Title = *p.Title
	}
	if p.Description != nil {
		t.Description = *p.Description
	}
	if p.Priority != nil {
		t.Priority = *p.Priority
	}
	if t.Title == "" && t.Description == "" {
		return mtt.Task{}, fmt.Errorf("a task needs a title or a description")
	}
	if p.Title != nil || p.Description != nil {
		t.Tags = reconcileTags(t.Tags, oldTitle, oldDesc, t.Title, t.Description)
	}
	t.Updated = e.now().UTC().Truncate(time.Second)
	updated, err := e.store.Update(t)
	if err != nil {
		return mtt.Task{}, err
	}
	if err := e.ev.TaskEvent(mtt.EventUpdate, updated, "edit", p.Events); err != nil {
		return updated, err // finalization failure: the edit IS persisted (exit 5)
	}
	return updated, nil
}
