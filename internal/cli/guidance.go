package cli

import (
	"fmt"
	"strings"

	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// moveGuidance returns the flow's inline instructions for a successful move (an
// empty string when there is nothing to say): the traversed edge's description
// ("what this move is for"), the destination status's description (standing
// instructions), and the onward moves ("next:"). Each line is omitted when its
// source is empty — a terminal status has no "next:". Best-effort: an unresolved
// type prints nothing. The caller writes the result (so the write error is handled).
func moveGuidance(cfg mtt.Config, id mtt.TaskID, typeName mtt.TypeName, from, to mtt.StatusName) string {
	typ, ok := cfg.TypeByName(typeName)
	if !ok {
		return ""
	}
	var b strings.Builder
	if edge, ok := typ.FindTransition(from, to); ok && edge.Description != "" {
		fmt.Fprintf(&b, "  ▸ %s\n", core.ExpandText(edge.Description, string(id), string(typeName), string(from), string(to)))
	}
	if st, ok := typ.StatusByName(to); ok && st.Description != "" {
		// node rule: a status description sees From=To=its own name (the destination).
		fmt.Fprintf(&b, "  ▸ %s\n", core.ExpandText(st.Description, string(id), string(typeName), string(to), string(to)))
	}
	if onward := typ.TransitionsFrom(to); len(onward) > 0 {
		fmt.Fprintf(&b, "  next: %s\n", formatNextMoves(onward, id, typeName))
	}
	return b.String()
}

// statusGuidance returns a task's current-status description and its onward
// transitions — the flow guidance shown by `mtt show` (human + --json). Empty /
// nil when the type or status can't be resolved (config drift), so the caller
// renders no guidance rather than erroring.
func statusGuidance(cfg mtt.Config, t mtt.Task) (statusDesc string, onward []mtt.Transition) {
	typ, ok := cfg.TypeByName(t.Type)
	if !ok {
		return "", nil
	}
	if st, ok := typ.StatusByName(t.Status); ok {
		// node rule: From=To=the current status name.
		statusDesc = core.ExpandText(st.Description, string(t.ID), string(t.Type), string(t.Status), string(t.Status))
	}
	return statusDesc, typ.TransitionsFrom(t.Status)
}

// formatNextMoves renders a status's onward transitions as a "next:" hint —
// each target status, with its transition description in parens when set, joined
// by " · ". Empty for no onward moves (a terminal status). Shared by the on-move
// guidance (status/sugar) and `mtt show`.
func formatNextMoves(onward []mtt.Transition, id mtt.TaskID, typeName mtt.TypeName) string {
	parts := make([]string, 0, len(onward))
	for _, e := range onward {
		s := string(e.To)
		if e.Name != "" {
			s = e.Name + " → " + string(e.To)
		}
		if e.Description != "" {
			s += " (" + core.ExpandText(e.Description, string(id), string(typeName), string(e.From), string(e.To)) + ")"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " · ")
}
