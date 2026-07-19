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
	// Delete removes an existing task by ID. Missing task -> ErrNotFound. It is a
	// store operation (not an embedded field), so it lives on the base port; an
	// adapter that cannot hard-delete returns ErrUnsupported.
	Delete(id TaskID) error
}

// ErrNotFound is returned by TaskStore.Get and KnowledgeStore.GetNote when the
// ID/slug does not resolve. (The message text is task-worded; the CLI surfaces the
// right noun via taskNotFound/noteNotFound wrappers.)
var ErrNotFound = errors.New("mtt: task not found")
