package mtt

// Priority is a task's scheduling priority — a closed, ordered domain vocabulary
// (a value object, like StatusKind), not a bare string or number. Empty means
// unset and orders as PriorityMedium (the neutral default); it is not written to
// disk (omitempty), so existing tasks are unaffected. It maps to a provider's
// native priority/labels later.
type Priority string

// The three priorities. Empty (unset) is valid and orders as PriorityMedium.
const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// Valid reports whether p is a known priority or empty (unset).
func (p Priority) Valid() bool {
	switch p {
	case "", PriorityHigh, PriorityMedium, PriorityLow:
		return true
	default:
		return false
	}
}

// Rank returns the sort rank (lower = higher priority): high=0, medium=1, low=2.
// Empty (unset) and any unknown value rank as medium — the neutral default, so a
// corrupt on-disk value is tolerated rather than rejected.
func (p Priority) Rank() int {
	switch p {
	case PriorityHigh:
		return 0
	case PriorityLow:
		return 2
	default: // medium, unset, and any unknown value
		return 1
	}
}
