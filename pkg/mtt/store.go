package mtt

import "errors"

// TaskStore is the mandatory-minimum driven port for tasks: create (the adapter
// mints the ID), get by ID, list all, and update an existing task. Implementations
// map their own DTOs to and from these pure domain types.
type TaskStore interface {
	// Create persists a logical task (empty ID); the adapter mints the ID and
	// returns the stored task.
	Create(t Task) (Task, error)
	// Get loads a task by ID, returning ErrNotFound when it does not resolve.
	Get(id TaskID) (Task, error)
	// List returns all tasks. The order is unspecified — callers impose their
	// own deterministic order (an adapter is not required to sort).
	List() ([]Task, error)
	// Update overwrites an existing task identified by t.ID, returning the stored
	// task; it never mints and never creates. Missing task -> ErrNotFound.
	Update(t Task) (Task, error)
}

// ErrNotFound is returned by TaskStore.Get when the ID does not resolve.
var ErrNotFound = errors.New("mtt: task not found")
