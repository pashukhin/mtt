package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// kindOf resolves the status category of t via its type's flow in cfg. It reports
// false when the type or the status is unknown to cfg (config drift). Shared by
// Ready and matchesKind so the "resolve a task's kind" lookup lives in one place.
func kindOf(t mtt.Task, cfg mtt.Config) (mtt.StatusKind, bool) {
	typ, ok := cfg.TypeByName(t.Type)
	if !ok {
		return "", false
	}
	return typ.StatusKind(t.Status)
}

// Ready returns the actionable tasks in deterministic order (Created desc, ID
// tiebreak): a task is ready iff its own status resolves to a non-terminal kind
// AND every DependsOn resolves to a present task whose status is terminal.
// Conservative — any unresolvable status or dangling blocker leaves a task
// not-ready. Pure: no store, no clock.
func Ready(tasks []mtt.Task, cfg mtt.Config) []mtt.Task {
	byID := make(map[mtt.TaskID]mtt.Task, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
	}
	out := make([]mtt.Task, 0, len(tasks))
	for _, t := range tasks {
		if isReady(t, byID, cfg) {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return lessByRecency(out[i], out[j], SortCreated)
	})
	return out
}

// isReady reports whether t is actionable (see Ready), conservative on anything
// unresolvable.
func isReady(t mtt.Task, byID map[mtt.TaskID]mtt.Task, cfg mtt.Config) bool {
	k, ok := kindOf(t, cfg)
	if !ok || k == mtt.KindTerminal {
		return false
	}
	for _, blockerID := range t.DependsOn {
		blocker, ok := byID[blockerID]
		if !ok {
			return false // dangling blocker
		}
		bk, ok := kindOf(blocker, cfg)
		if !ok || bk != mtt.KindTerminal {
			return false
		}
	}
	return true
}
