package cli

import (
	"fmt"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// parsePriority parses a --priority value, rejecting an unknown one at the CLI
// boundary. "" is valid (unset) — Valid() is true for it, so !Valid() already
// implies a non-empty garbage value; no extra clause is needed.
func parsePriority(s string) (mtt.Priority, error) {
	p := mtt.Priority(s)
	if !p.Valid() {
		return "", fmt.Errorf("invalid --priority %q: want high|medium|low", s)
	}
	return p, nil
}

// toPriorities parses a repeatable --priority filter, validating each value.
func toPriorities(ss []string) ([]mtt.Priority, error) {
	out := make([]mtt.Priority, len(ss))
	for i, s := range ss {
		p, err := parsePriority(s)
		if err != nil {
			return nil, err
		}
		out[i] = p
	}
	return out, nil
}
