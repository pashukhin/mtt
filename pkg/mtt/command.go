package mtt

import "time"

// Command is one gate step of a transition: a shell command (Run) with an
// optional per-command timeout that overrides the adapter's global
// command_timeout (zero = fall back to the global), and an optional compensator
// (Rollback) run in reverse over the already-succeeded commands when a later
// command in the same pipeline fails (s008). Run/Rollback.Run hold raw templates
// (e.g. "git checkout -b task/{{.ID}}"); the domain does not interpret them —
// core expands the placeholders before the runner runs them.
type Command struct {
	Run      string
	Timeout  time.Duration
	Rollback *Command // optional compensator for THIS command; nil = none
}

// Valid reports whether the command is well-formed: a non-empty Run, a
// non-negative Timeout, and — if present — a well-formed LEAF compensator (a
// compensator is not itself compensated: its own Rollback must be nil).
func (c Command) Valid() bool {
	if c.Run == "" || c.Timeout < 0 {
		return false
	}
	if c.Rollback != nil {
		return c.Rollback.Run != "" && c.Rollback.Timeout >= 0 && c.Rollback.Rollback == nil
	}
	return true
}
