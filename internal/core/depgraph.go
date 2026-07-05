package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// DepGraph is a derived, read-only view of the depends_on blocking graph over a
// set of tasks (a pure value: no store, no clock). It is not part of the pkg/mtt
// contract — the resolved graph is derived. Forward edges are each task's stored
// DependsOn; Dependents (reverse edges) are computed. Cycle-safe throughout.
// Kept separate from Index: parent is a single-parent tree walked upward,
// depends_on is a multi-edge DAG walked downward (GAP #6 not extracted).
type DepGraph struct {
	byID       map[mtt.TaskID]mtt.Task
	dependents map[mtt.TaskID][]mtt.Task // keyed by blocker id: tasks that depend on it
}

// NewDepGraph builds the dependency view. Dependent buckets are ordered by
// lessByRecency (Created desc, ID tiebreak) so output matches list/tree order.
func NewDepGraph(tasks []mtt.Task) DepGraph {
	g := DepGraph{
		byID:       make(map[mtt.TaskID]mtt.Task, len(tasks)),
		dependents: make(map[mtt.TaskID][]mtt.Task),
	}
	for _, t := range tasks {
		g.byID[t.ID] = t
	}
	for _, t := range tasks {
		for _, blocker := range t.DependsOn {
			g.dependents[blocker] = append(g.dependents[blocker], t)
		}
	}
	for k := range g.dependents {
		bucket := g.dependents[k]
		sort.SliceStable(bucket, func(i, j int) bool {
			return lessByRecency(bucket[i], bucket[j], SortCreated)
		})
	}
	return g
}

// Get returns the task with id, or false when absent.
func (g DepGraph) Get(id mtt.TaskID) (mtt.Task, bool) {
	t, ok := g.byID[id]
	return t, ok
}

// DependsOn returns id's direct blocker ids in stored order (dangling ids kept —
// the caller resolves and flags them). Nil when id is absent or has no blockers.
func (g DepGraph) DependsOn(id mtt.TaskID) []mtt.TaskID {
	t, ok := g.byID[id]
	if !ok {
		return nil
	}
	return t.DependsOn
}

// Dependents returns the tasks that directly depend on id, in sibling order.
func (g DepGraph) Dependents(id mtt.TaskID) []mtt.Task { return g.dependents[id] }

// Reaches reports whether to is reachable from `from` by following depends_on
// edges (a path of one or more edges). Cycle-safe (visited-set). Powers the add
// cycle-check: adding id → dependsOn cycles iff Reaches(dependsOn, id).
func (g DepGraph) Reaches(from, to mtt.TaskID) bool {
	seen := map[mtt.TaskID]bool{}
	var dfs func(cur mtt.TaskID) bool
	dfs = func(cur mtt.TaskID) bool {
		t, ok := g.byID[cur]
		if !ok {
			return false
		}
		for _, dep := range t.DependsOn {
			if dep == to {
				return true
			}
			if !seen[dep] {
				seen[dep] = true
				if dfs(dep) {
					return true
				}
			}
		}
		return false
	}
	return dfs(from)
}

// Cycles returns the dependency cycles in the graph, each as the id chain of the
// nodes on the cycle (traversal order). Empty when the graph is acyclic. Entry
// order is deterministic (ids sorted as opaque strings). Defensive: the CLI's add
// rejects cycles, so this only fires on hand-edited data.
func (g DepGraph) Cycles() [][]mtt.TaskID {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[mtt.TaskID]int{}
	var stack []mtt.TaskID
	var cycles [][]mtt.TaskID
	var dfs func(cur mtt.TaskID)
	dfs = func(cur mtt.TaskID) {
		color[cur] = gray
		stack = append(stack, cur)
		for _, dep := range g.byID[cur].DependsOn {
			if _, ok := g.byID[dep]; !ok {
				continue // dangling — not a cycle
			}
			switch color[dep] {
			case white:
				dfs(dep)
			case gray:
				cycles = append(cycles, extractCycle(stack, dep))
			}
		}
		stack = stack[:len(stack)-1]
		color[cur] = black
	}
	entries := make([]mtt.TaskID, 0, len(g.byID))
	for id := range g.byID {
		entries = append(entries, id)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i] < entries[j] })
	for _, id := range entries {
		if color[id] == white {
			dfs(id)
		}
	}
	return cycles
}

// extractCycle slices the recursion stack from the back-edge target to the top.
func extractCycle(stack []mtt.TaskID, start mtt.TaskID) []mtt.TaskID {
	for i, id := range stack {
		if id == start {
			cyc := make([]mtt.TaskID, len(stack)-i)
			copy(cyc, stack[i:])
			return cyc
		}
	}
	return nil
}
