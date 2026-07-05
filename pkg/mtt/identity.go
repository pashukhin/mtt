package mtt

import "errors"

// TaskID is an adapter-minted task identity — opaque. core never parses its
// structure (that is adapter-specific). It marshals as a plain string.
type TaskID string

// TypeName is a configured type's name (e.g. epic/task/subtask).
type TypeName string

// StatusName is a status name. Full status identity is (TypeName, StatusName),
// scoped to one flow — a bare StatusName is not globally unique.
type StatusName string

// NewTaskID returns id from a raw string, rejecting empty. It is the boundary
// guard (e.g. deserialization); it does NOT parse structure and does NOT trim —
// preserving a byte-for-byte round-trip. Normalization is a future extension.
func NewTaskID(s string) (TaskID, error) {
	if s == "" {
		return "", errors.New("mtt: empty task id")
	}
	return TaskID(s), nil
}

// NewTypeName returns a type name from a raw string, rejecting empty.
func NewTypeName(s string) (TypeName, error) {
	if s == "" {
		return "", errors.New("mtt: empty type name")
	}
	return TypeName(s), nil
}

// NewStatusName returns a status name from a raw string, rejecting empty.
func NewStatusName(s string) (StatusName, error) {
	if s == "" {
		return "", errors.New("mtt: empty status name")
	}
	return StatusName(s), nil
}
