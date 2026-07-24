package mtt

// EventKind is the closed vocabulary of lifecycle events a store mutation
// fires: an entity was created, updated, or deleted. A value object like
// StatusKind — code reasons about the kind, never a bare string. It is also
// the {{.Event}} placeholder value, so it stays shape-safe by construction.
type EventKind string

// The three lifecycle events.
const (
	EventCreate EventKind = "create"
	EventUpdate EventKind = "update"
	EventDelete EventKind = "delete"
)

// Valid reports whether k is one of the three lifecycle events.
func (k EventKind) Valid() bool {
	return k == EventCreate || k == EventUpdate || k == EventDelete
}

// EventHook is the pipeline configured for one lifecycle event of one store.
// Post-only by design (t66): an event finalizes a persisted mutation, it can
// never block one; a blocking commands: phase would be an additive extension.
type EventHook struct {
	Post []Command
}

// EventHooks groups the three per-event hooks of one store.
type EventHooks struct {
	Create EventHook
	Update EventHook
	Delete EventHook
}

// Hook returns the hook for kind (the zero EventHook for an unknown kind).
func (h EventHooks) Hook(kind EventKind) EventHook {
	switch kind {
	case EventCreate:
		return h.Create
	case EventUpdate:
		return h.Update
	case EventDelete:
		return h.Delete
	}
	return EventHook{}
}

// Events is the config's lifecycle-event section: per-store hooks for tasks
// and notes. Optional — the zero value configures no events.
type Events struct {
	Task EventHooks
	Note EventHooks
}
