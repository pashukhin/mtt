package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// DependencyEditor mutates a task's blocking edges (depends_on) and persists via
// TaskStore.Update. The cycle rule is enforced here (core policy). No new port:
// the edge is a Task field, round-tripped by the adapter DTO (mirrors s004's
// parent).
type DependencyEditor struct {
	store mtt.TaskStore
	now   func() time.Time
	ev    *EventEmitter
}

// NewDependencyEditor wires the usecase. now is injected for deterministic
// tests; ev fires the update event (nil = none).
func NewDependencyEditor(store mtt.TaskStore, now func() time.Time, ev *EventEmitter) *DependencyEditor {
	return &DependencyEditor{store: store, now: now, ev: ev}
}

// AddDependency makes id depend on dependsOn. Both tasks must exist; a self-edge
// and any edge that would create a cycle are rejected; an already-present edge is
// an idempotent no-op (no write, no timestamp bump). On a real change it bumps
// Updated and persists.
func (d *DependencyEditor) AddDependency(id, dependsOn mtt.TaskID, opts EventOptions) (mtt.Task, error) {
	if err := opts.Preflight(); err != nil {
		return mtt.Task{}, err
	}
	if id == dependsOn {
		return mtt.Task{}, fmt.Errorf("a task cannot depend on itself")
	}
	t, err := d.load(id, "task")
	if err != nil {
		return mtt.Task{}, err
	}
	if _, err := d.load(dependsOn, "dependency"); err != nil {
		return mtt.Task{}, err
	}
	for _, dep := range t.DependsOn {
		if dep == dependsOn {
			return t, nil // idempotent: already present
		}
	}
	tasks, err := d.store.List()
	if err != nil {
		return mtt.Task{}, fmt.Errorf("list tasks: %w", err)
	}
	if NewDepGraph(tasks).Reaches(dependsOn, id) {
		return mtt.Task{}, fmt.Errorf("adding dependency %q → %q would create a cycle", id, dependsOn)
	}
	t.DependsOn = append(t.DependsOn, dependsOn)
	t.Updated = d.now().UTC().Truncate(time.Second)
	return d.persistAndFire(t, "dep add", opts)
}

// RemoveDependency drops the dependsOn edge from id. The task must exist;
// removing an edge that is already absent is an idempotent no-op (no write, no
// timestamp bump), symmetric with AddDependency's duplicate no-op. On a real
// removal it bumps Updated and persists.
func (d *DependencyEditor) RemoveDependency(id, dependsOn mtt.TaskID, opts EventOptions) (mtt.Task, error) {
	if err := opts.Preflight(); err != nil {
		return mtt.Task{}, err
	}
	t, err := d.load(id, "task")
	if err != nil {
		return mtt.Task{}, err
	}
	idx := -1
	for i, dep := range t.DependsOn {
		if dep == dependsOn {
			idx = i
			break
		}
	}
	if idx == -1 {
		return t, nil // idempotent: edge already absent
	}
	t.DependsOn = append(t.DependsOn[:idx], t.DependsOn[idx+1:]...)
	t.Updated = d.now().UTC().Truncate(time.Second)
	return d.persistAndFire(t, "dep rm", opts)
}

// persistAndFire updates the task and fires its update event; a fired event's
// finalization failure returns the PERSISTED task with the error (exit 5).
func (d *DependencyEditor) persistAndFire(t mtt.Task, action string, opts EventOptions) (mtt.Task, error) {
	up, err := d.store.Update(t)
	if err != nil {
		return mtt.Task{}, err
	}
	return up, d.ev.TaskEvent(mtt.EventUpdate, up, action, opts)
}

// load fetches a task, mapping ErrNotFound to a caller-labelled message (role is
// "task" or "dependency").
func (d *DependencyEditor) load(id mtt.TaskID, role string) (mtt.Task, error) {
	t, err := d.store.Get(id)
	if err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return mtt.Task{}, fmt.Errorf("%s %q: %w", role, id, mtt.ErrNotFound)
		}
		return mtt.Task{}, fmt.Errorf("load %s %q: %w", role, id, err)
	}
	return t, nil
}
