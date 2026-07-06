package mtt

import "time"

// Command is one gate step of a transition: a shell command (Run) with an
// optional per-command timeout that overrides the adapter's global
// command_timeout (zero = fall back to the global). Run holds a raw template
// (e.g. "git checkout -b task/{{.ID}}"); the domain does not interpret it —
// core expands the placeholders before the runner runs it.
type Command struct {
	Run     string
	Timeout time.Duration
}

// Valid reports whether the command is well-formed: a non-empty Run and a
// non-negative Timeout. (Mirrors the StatusKind/CurrentAction Valid() idiom.)
func (c Command) Valid() bool { return c.Run != "" && c.Timeout >= 0 }
