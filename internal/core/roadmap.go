package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// RoadmapEntry is one task in the computed execution order, annotated with whether
// it is actionable now and what still blocks it.
type RoadmapEntry struct {
	Task      mtt.Task
	Ready     bool         // membership in core.Ready (all blockers terminal)
	BlockedBy []mtt.TaskID // depends_on entries not terminal-satisfied (non-terminal or dangling)
}

// Roadmap returns the non-terminal tasks in a dependency-respecting,
// priority-weighted order (a priority-guided Kahn topological sort). A depends_on
// edge is a HARD constraint (a non-terminal blocker is placed before the task it
// blocks, even at lower priority); priority is a SOFT tiebreak among the tasks a
// topological step leaves available — exactly Select's SortPriority order, so
// roadmap is a dependency-constrained `list --sort priority`. Pure: no store, no
// clock. Reuses core.Ready (the ready flag) + the shared terminalSatisfied
// predicate + Priority.Rank. It builds its OWN non-terminal-restricted DAG (it
// does NOT reuse DepGraph, whose Dependents are unfiltered).
func Roadmap(tasks []mtt.Task, cfg mtt.Config) []RoadmapEntry {
	byID := make(map[mtt.TaskID]mtt.Task, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
	}

	// Nodes: tasks whose status kind is NOT confirmed-terminal. An unresolvable
	// status is included (conservative).
	isNode := make(map[mtt.TaskID]bool, len(tasks))
	nodes := make([]mtt.Task, 0, len(tasks))
	for _, t := range tasks {
		if k, ok := kindOf(t, cfg); ok && k == mtt.KindTerminal {
			continue
		}
		isNode[t.ID] = true
		nodes = append(nodes, t)
	}

	// Ready-flag membership — single source of truth for the Ready annotation.
	readySet := make(map[mtt.TaskID]bool)
	for _, t := range Ready(tasks, cfg) {
		readySet[t.ID] = true
	}

	// Node-restricted edges + BlockedBy. A blocker constrains ordering only if it
	// is itself a node (non-terminal & existing); a terminal or dangling blocker
	// imposes no constraint. BlockedBy lists every depends_on NOT terminal-satisfied.
	indeg := make(map[mtt.TaskID]int, len(nodes))
	dependents := make(map[mtt.TaskID][]mtt.TaskID)
	blockedBy := make(map[mtt.TaskID][]mtt.TaskID)
	for _, t := range nodes {
		for _, dep := range t.DependsOn {
			if !terminalSatisfied(dep, byID, cfg) {
				blockedBy[t.ID] = append(blockedBy[t.ID], dep)
			}
			if isNode[dep] {
				indeg[t.ID]++
				dependents[dep] = append(dependents[dep], t.ID)
			}
		}
	}

	// Priority-guided Kahn: repeatedly emit the available (indeg 0) node with min
	// Rank (high first), tie-broken by lessByRecency — i.e. lessByPriority.
	emitted := make(map[mtt.TaskID]bool, len(nodes))
	order := make([]mtt.Task, 0, len(nodes))
	avail := make([]mtt.Task, 0, len(nodes))
	for _, t := range nodes {
		if indeg[t.ID] == 0 {
			avail = append(avail, t)
		}
	}
	for len(avail) > 0 {
		sort.SliceStable(avail, func(i, j int) bool { return lessByPriority(avail[i], avail[j]) })
		pick := avail[0]
		avail = avail[1:]
		order = append(order, pick)
		emitted[pick.ID] = true
		for _, dep := range dependents[pick.ID] {
			indeg[dep]--
			if indeg[dep] == 0 {
				avail = append(avail, byID[dep])
			}
		}
	}

	// Cycle-safety: any node never reaching indeg 0 is in — or downstream of — a
	// depends_on cycle (a chain feeding a cycle also never drains). Unreachable via
	// the CLI (add rejects cycles) but defended against a hand-edited store: append
	// the remaining nodes sorted by (Rank, recency) so the function terminates and
	// returns EVERY node. Best-effort: it does NOT preserve the partial order among
	// the stuck set.
	if len(order) < len(nodes) {
		stuck := make([]mtt.Task, 0, len(nodes)-len(order))
		for _, t := range nodes {
			if !emitted[t.ID] {
				stuck = append(stuck, t)
			}
		}
		sort.SliceStable(stuck, func(i, j int) bool { return lessByPriority(stuck[i], stuck[j]) })
		order = append(order, stuck...)
	}

	out := make([]RoadmapEntry, 0, len(order))
	for _, t := range order {
		out = append(out, RoadmapEntry{Task: t, Ready: readySet[t.ID], BlockedBy: blockedBy[t.ID]})
	}
	return out
}
