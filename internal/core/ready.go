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
		if !terminalSatisfied(blockerID, byID, cfg) {
			return false
		}
	}
	return true
}

// terminalSatisfied reports whether blocker id resolves to a present task whose
// status kind is terminal — so it imposes no readiness/ordering constraint. A
// dangling (absent) blocker or an unresolvable-status one is NOT satisfied
// (conservative). The single home for "is this blocker satisfied" as a
// *decision*, shared by Ready and Roadmap (the CLI's `show` depends-line
// rendering re-derives the same predicate for display only — c12; keep the
// semantics aligned if this ever changes).
func terminalSatisfied(id mtt.TaskID, byID map[mtt.TaskID]mtt.Task, cfg mtt.Config) bool {
	blocker, ok := byID[id]
	if !ok {
		return false
	}
	bk, ok := kindOf(blocker, cfg)
	return ok && bk == mtt.KindTerminal
}
