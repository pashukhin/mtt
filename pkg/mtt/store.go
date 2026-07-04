package mtt

import "errors"

// TaskStore is the mandatory-minimum driven port for tasks: create (the adapter
// mints the ID) and get by ID. Implementations map their own DTOs to and from
// these pure domain types.
type TaskStore interface {
	// Create persists a logical task (empty ID); the adapter mints the ID and
	// returns the stored task.
	Create(t Task) (Task, error)
	// Get loads a task by ID, returning ErrNotFound when it does not resolve.
	Get(id string) (Task, error)
}

// ErrNotFound is returned by TaskStore.Get when the ID does not resolve.
var ErrNotFound = errors.New("mtt: task not found")
