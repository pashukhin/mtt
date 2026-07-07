package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// RoadmapEntry is one task in the computed execution order, annotated with whether
// it is actionable now, what still blocks it, and (for a parent) what it contains.
type RoadmapEntry struct {
	Task      mtt.Task
	Ready     bool         // membership in core.Ready (all depends_on blockers terminal)
	BlockedBy []mtt.TaskID // depends_on entries not terminal-satisfied (non-terminal or dangling)
	Contains  []mtt.TaskID // non-terminal children (this parent is ordered after them)
}

// Roadmap returns the non-terminal tasks in an order that respects TWO "comes
// after" axes — depends_on (an explicit blocking edge) and parent (a parent is
// completed only once its children are, so its children precede it) — and is
// weighted by a PROPAGATED priority: a blocker inherits the highest priority of
// everything it (transitively) unblocks, so a high-priority task pulls its
// prerequisites forward, ahead of lower-priority independent work. Pure: no store,
// no clock; not in the pkg/mtt contract. Reuses core.Ready (the ready flag,
// depends_on-only) + the shared terminalSatisfied predicate. Both axes are HARD
// ordering constraints; priority is the SOFT tiebreak (effective rank, then
// recency). Ready/BlockedBy stay about depends_on only — the parent axis affects
// ordering and the Contains annotation, not readiness. Cycle-safe: a node in — or
// downstream of — a cycle (in either axis, or across them) is appended best-effort
// so the function always terminates and returns every node.
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

	// Ready-flag membership (depends_on only) — single source of truth for Ready.
	readySet := make(map[mtt.TaskID]bool)
	for _, t := range Ready(tasks, cfg) {
		readySet[t.ID] = true
	}

	// Combined ordering graph. A node's predecessors ("must come before") are its
	// non-terminal depends_on blockers AND its non-terminal children. indeg counts
	// them; blocks[n] lists the nodes n precedes (its successors across BOTH axes) —
	// used for Kahn AND priority propagation. blockedBy (depends_on only) and
	// contains (children only) are the annotations.
	indeg := make(map[mtt.TaskID]int, len(nodes))
	blocks := make(map[mtt.TaskID][]mtt.TaskID)
	blockedBy := make(map[mtt.TaskID][]mtt.TaskID)
	contains := make(map[mtt.TaskID][]mtt.TaskID)
	for _, t := range nodes {
		for _, dep := range t.DependsOn {
			if !terminalSatisfied(dep, byID, cfg) {
				blockedBy[t.ID] = append(blockedBy[t.ID], dep)
			}
			if isNode[dep] {
				indeg[t.ID]++
				blocks[dep] = append(blocks[dep], t.ID)
			}
		}
		if t.Parent != "" && isNode[t.Parent] {
			indeg[t.Parent]++
			blocks[t.ID] = append(blocks[t.ID], t.Parent)
			contains[t.Parent] = append(contains[t.Parent], t.ID)
		}
	}

	eff := effectivePriority(nodes, byID, blocks)
	lessByEff := func(a, b mtt.Task) bool {
		if eff[a.ID] != eff[b.ID] {
			return eff[a.ID] < eff[b.ID]
		}
		return lessByRecency(a, b, SortCreated)
	}

	// Priority-guided Kahn over the combined graph, ordered by effective priority:
	// repeatedly emit the available (indeg 0) node with min effective rank, tie-broken
	// by recency; decrement the indeg of everything it blocks.
	emitted := make(map[mtt.TaskID]bool, len(nodes))
	order := make([]mtt.Task, 0, len(nodes))
	avail := make([]mtt.Task, 0, len(nodes))
	for _, t := range nodes {
		if indeg[t.ID] == 0 {
			avail = append(avail, t)
		}
	}
	for len(avail) > 0 {
		sort.SliceStable(avail, func(i, j int) bool { return lessByEff(avail[i], avail[j]) })
		pick := avail[0]
		avail = avail[1:]
		order = append(order, pick)
		emitted[pick.ID] = true
		for _, s := range blocks[pick.ID] {
			indeg[s]--
			if indeg[s] == 0 {
				avail = append(avail, byID[s])
			}
		}
	}

	// Cycle-safety: append any node never reaching indeg 0 (in — or downstream of —
	// a cycle in either axis, or across them), sorted by (effective priority,
	// recency). Best-effort: it does not preserve the partial order among the stuck
	// set. Unreachable via normal use; defends a hand-edited store.
	if len(order) < len(nodes) {
		stuck := make([]mtt.Task, 0, len(nodes)-len(order))
		for _, t := range nodes {
			if !emitted[t.ID] {
				stuck = append(stuck, t)
			}
		}
		sort.SliceStable(stuck, func(i, j int) bool { return lessByEff(stuck[i], stuck[j]) })
		order = append(order, stuck...)
	}

	// Order each contains list in sibling order (recency, like tree/Index children).
	for p := range contains {
		kids := contains[p]
		sort.SliceStable(kids, func(i, j int) bool {
			return lessByRecency(byID[kids[i]], byID[kids[j]], SortCreated)
		})
	}

	out := make([]RoadmapEntry, 0, len(order))
	for _, t := range order {
		out = append(out, RoadmapEntry{
			Task: t, Ready: readySet[t.ID], BlockedBy: blockedBy[t.ID], Contains: contains[t.ID],
		})
	}
	return out
}

// effectivePriority computes each node's propagated priority rank: the min of its
// own Priority.Rank and the effective rank of every node it blocks (priority flows
// UP the ordering graph, so a blocker is as urgent as the most urgent thing it
// holds). Memoized; cycle-safe — a node re-entered on the current path contributes
// only its own rank (no recursion), so a hand-edited cycle terminates. Nodes are
// seeded in sorted-id order for a deterministic memo under cycles.
func effectivePriority(nodes []mtt.Task, byID map[mtt.TaskID]mtt.Task, blocks map[mtt.TaskID][]mtt.TaskID) map[mtt.TaskID]int {
	eff := make(map[mtt.TaskID]int, len(nodes))
	onStack := make(map[mtt.TaskID]bool, len(nodes))
	var rank func(n mtt.TaskID) int
	rank = func(n mtt.TaskID) int {
		if r, ok := eff[n]; ok {
			return r
		}
		best := byID[n].Priority.Rank()
		onStack[n] = true
		for _, s := range blocks[n] {
			sr := byID[s].Priority.Rank()
			if !onStack[s] {
				sr = rank(s)
			}
			if sr < best {
				best = sr
			}
		}
		onStack[n] = false
		eff[n] = best
		return best
	}
	ids := make([]mtt.TaskID, len(nodes))
	for i, t := range nodes {
		ids[i] = t.ID
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		rank(id)
	}
	return eff
}
