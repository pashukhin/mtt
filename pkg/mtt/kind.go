// Package mtt is the public domain contract for mtt: pure domain types and ports,
// free of storage/serialization concerns. Adapters map their own DTOs to and from
// these types; nothing here knows about YAML, files, or the CLI.
package mtt

// StatusKind is the category of a flow status — a closed domain vocabulary the
// code reasons about (ready/terminal logic), unlike open, user-defined status
// names. It is a value object, not a name.
type StatusKind string

// The three status kinds. Every flow needs at least one status of each.
const (
	KindInitial  StatusKind = "initial"
	KindActive   StatusKind = "active"
	KindTerminal StatusKind = "terminal"
)

// Valid reports whether k is one of the three defined kinds.
func (k StatusKind) Valid() bool {
	switch k {
	case KindInitial, KindActive, KindTerminal:
		return true
	default:
		return false
	}
}
