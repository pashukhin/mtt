package core

import (
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// roadmapCfg is a task-type flow with TWO distinct terminal statuses (done and
// cancelled), so the tests can pin the cancelled==terminal unblocking rule apart
// from done. The shared cfg()/flow()/matchCfg() helpers lack cancelled.
func roadmapCfg() mtt.Config {
	return mtt.Config{Types: []mtt.Type{
		{Name: "task", Default: true, Flow: mtt.Flow{
			Statuses: []mtt.Status{
				{Name: "tbd", Kind: mtt.KindInitial},
				{Name: "in_progress", Kind: mtt.KindActive},
				{Name: "done", Kind: mtt.KindTerminal},
				{Name: "cancelled", Kind: mtt.KindTerminal},
			},
			Transitions: []mtt.Transition{
				{From: "tbd", To: "in_progress"},
				{From: "in_progress", To: "done"},
				{From: "tbd", To: "cancelled"},
			},
		}},
	}}
}

func entryIDs(entries []RoadmapEntry) []mtt.TaskID {
	out := make([]mtt.TaskID, len(entries))
	for i, e := range entries {
		out[i] = e.Task.ID
	}
	return out
}

func sameIDs(a []mtt.TaskID, b ...mtt.TaskID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func findEntry(entries []RoadmapEntry, id mtt.TaskID) (RoadmapEntry, bool) {
	for _, e := range entries {
		if e.Task.ID == id {
			return e, true
		}
	}
	return RoadmapEntry{}, false
}

// Priority PROPAGATION: a high task behind a lower blocker pulls that blocker
// forward, ahead of independent lower-priority work. t1(high) depends_on t2(unset);
// t3(medium) is independent → t2 inherits high → order [t2, t1, t3] (NOT the greedy
// [t3, t2, t1] or [t2, t3, t1] where the medium independent task jumps ahead).
func TestRoadmapPriorityPropagation(t *testing.T) {
	base := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		{ID: "t1", Type: "task", Status: "tbd", Priority: mtt.PriorityHigh, DependsOn: []mtt.TaskID{"t2"}, Created: base},
		{ID: "t2", Type: "task", Status: "tbd", Created: base}, // unset, but blocks a high task
		{ID: "t3", Type: "task", Status: "tbd", Priority: mtt.PriorityMedium, Created: base},
	}
	got := entryIDs(Roadmap(tasks, roadmapCfg()))
	if !sameIDs(got, "t2", "t1", "t3") {
		t.Fatalf("order = %v, want [t2 t1 t3] (t2 inherits t1's high, pulled ahead of medium t3)", got)
	}
}

// Parent axis: a non-terminal child precedes its parent (a parent completes only
// once its children do), and the parent is annotated with its children (Contains).
// Readiness stays depends_on-only, so the parent with open children is still Ready.
func TestRoadmapParentAxis(t *testing.T) {
	base := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		{ID: "e1", Type: "task", Status: "tbd", Created: base},                              // parent
		{ID: "c1", Type: "task", Status: "tbd", Parent: "e1", Created: base.Add(time.Hour)}, // child (older-in-list)
		{ID: "c2", Type: "task", Status: "tbd", Parent: "e1", Created: base.Add(2 * time.Hour)},
	}
	entries := Roadmap(tasks, roadmapCfg())
	// e1 is last (both children precede it).
	if entries[len(entries)-1].Task.ID != "e1" {
		t.Fatalf("parent e1 must be last; got order %v", entryIDs(entries))
	}
	e1, _ := findEntry(entries, "e1")
	// Contains lists both children in sibling order (recency: newer c2 first).
	if len(e1.Contains) != 2 || e1.Contains[0] != "c2" || e1.Contains[1] != "c1" {
		t.Fatalf("e1.Contains = %v, want [c2 c1] (children, recency order)", e1.Contains)
	}
	// Readiness is depends_on-only: e1 has no depends_on blockers → Ready, BlockedBy empty.
	if !e1.Ready || len(e1.BlockedBy) != 0 {
		t.Fatalf("e1: ready=%v blockedBy=%v, want ready=true blockedBy=[] (parent axis is ordering-only)", e1.Ready, e1.BlockedBy)
	}
	// A leaf child has no Contains.
	c1, _ := findEntry(entries, "c1")
	if len(c1.Contains) != 0 {
		t.Fatalf("leaf c1.Contains = %v, want empty", c1.Contains)
	}
}

// The full combined example (the reported case): an epic with three children, one
// of which (high) depends on another (low). Both axes + propagation → the low
// blocker is lifted (it holds a high task), and the epic sinks to the end with a
// Contains annotation.
func TestRoadmapCombinedEpicExample(t *testing.T) {
	base := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		{ID: "e1", Type: "task", Status: "tbd", Created: base},
		{ID: "t1", Type: "task", Status: "tbd", Priority: mtt.PriorityLow, Parent: "e1", Created: base.Add(time.Hour)},
		{ID: "t2", Type: "task", Status: "tbd", Priority: mtt.PriorityHigh, Parent: "e1", DependsOn: []mtt.TaskID{"t1"}, Created: base.Add(2 * time.Hour)},
		{ID: "t3", Type: "task", Status: "tbd", Priority: mtt.PriorityMedium, Parent: "e1", Created: base.Add(3 * time.Hour)},
	}
	entries := Roadmap(tasks, roadmapCfg())
	// t1(low) is lifted above t3(medium) because it blocks t2(high); t1 before t2
	// (hard dep); epic e1 last.
	if !sameIDs(entryIDs(entries), "t1", "t2", "t3", "e1") {
		t.Fatalf("order = %v, want [t1 t2 t3 e1]", entryIDs(entries))
	}
	e1, _ := findEntry(entries, "e1")
	if len(e1.Contains) != 3 {
		t.Fatalf("e1.Contains = %v, want 3 children", e1.Contains)
	}
}

// A task that is BOTH a child of and depends_on the same node forms a cross-axis
// cycle (child-before-parent vs depends-on-parent) — unreachable in normal use but
// expressible; Roadmap must terminate and return every node (best-effort).
func TestRoadmapCrossAxisCycleSafe(t *testing.T) {
	base := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		{ID: "e1", Type: "task", Status: "tbd", Created: base},
		{ID: "t1", Type: "task", Status: "tbd", Parent: "e1", DependsOn: []mtt.TaskID{"e1"}, Created: base}, // child of e1 AND depends on e1
	}
	entries := Roadmap(tasks, roadmapCfg())
	if len(entries) != 2 {
		t.Fatalf("cross-axis cycle returned %d entries, want all 2 (best-effort)", len(entries))
	}
}

// (i) A blocker is placed before its dependent even when the blocker is LOWER
// priority (dependency = hard constraint; priority = soft tiebreak). With
// propagation the low blocker also inherits the dependent's high priority.
func TestRoadmapHardConstraintBeatsPriority(t *testing.T) {
	base := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		{ID: "t1", Type: "task", Status: "tbd", Priority: mtt.PriorityLow, Created: base},
		{ID: "t2", Type: "task", Status: "tbd", Priority: mtt.PriorityHigh, DependsOn: []mtt.TaskID{"t1"}, Created: base},
	}
	got := entryIDs(Roadmap(tasks, roadmapCfg()))
	if !sameIDs(got, "t1", "t2") {
		t.Fatalf("order = %v, want [t1 t2] (low blocker before high dependent)", got)
	}
}

// (ii) Priority tie-break among INDEPENDENT available tasks (== Select order).
func TestRoadmapPriorityTiebreak(t *testing.T) {
	base := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		{ID: "t1", Type: "task", Status: "tbd", Priority: mtt.PriorityLow, Created: base},
		{ID: "t2", Type: "task", Status: "tbd", Priority: mtt.PriorityHigh, Created: base},
		{ID: "t3", Type: "task", Status: "tbd", Priority: mtt.PriorityMedium, Created: base},
	}
	got := entryIDs(Roadmap(tasks, roadmapCfg()))
	if !sameIDs(got, "t2", "t3", "t1") {
		t.Fatalf("order = %v, want [t2 t3 t1] (high, medium, low)", got)
	}
}

// (iii) Ready / BlockedBy annotations: a terminal blocker is satisfied; a dangling
// blocker leaves the task not-ready and is listed in BlockedBy.
func TestRoadmapAnnotations(t *testing.T) {
	base := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		{ID: "t1", Type: "task", Status: "done", Created: base},                                 // terminal blocker
		{ID: "t2", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t1"}, Created: base},   // satisfied → ready
		{ID: "t3", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"gone"}, Created: base}, // dangling → not ready
	}
	entries := Roadmap(tasks, roadmapCfg())
	if _, ok := findEntry(entries, "t1"); ok {
		t.Fatal("terminal t1 must be excluded")
	}
	e2, _ := findEntry(entries, "t2")
	if !e2.Ready || len(e2.BlockedBy) != 0 {
		t.Fatalf("t2: ready=%v blockedBy=%v, want ready=true blockedBy=[]", e2.Ready, e2.BlockedBy)
	}
	e3, _ := findEntry(entries, "t3")
	if e3.Ready || len(e3.BlockedBy) != 1 || e3.BlockedBy[0] != "gone" {
		t.Fatalf("t3: ready=%v blockedBy=%v, want ready=false blockedBy=[gone]", e3.Ready, e3.BlockedBy)
	}
}

// (iii-b) A cancelled (terminal) blocker is satisfied → dependent ready (pins the
// s005/s006 cancelled==terminal rule, distinct from done).
func TestRoadmapCancelledBlockerSatisfied(t *testing.T) {
	base := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		{ID: "t1", Type: "task", Status: "cancelled", Created: base},
		{ID: "t2", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t1"}, Created: base},
	}
	entries := Roadmap(tasks, roadmapCfg())
	if _, ok := findEntry(entries, "t1"); ok {
		t.Fatal("cancelled t1 is terminal → excluded")
	}
	e2, _ := findEntry(entries, "t2")
	if !e2.Ready || len(e2.BlockedBy) != 0 {
		t.Fatalf("t2 blocked by a cancelled task must be ready; got ready=%v blockedBy=%v", e2.Ready, e2.BlockedBy)
	}
}

// A node with an unresolvable OWN status (config drift) is INCLUDED (conservative)
// but Ready=false — the conservative signal wins (spec §3b). As a non-terminal
// blocker it still imposes an ordering edge and lands in its dependent's BlockedBy.
func TestRoadmapUnresolvableOwnStatusIncluded(t *testing.T) {
	base := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		{ID: "t1", Type: "task", Status: "bogus", Created: base}, // status not in flow → drift
		{ID: "t2", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t1"}, Created: base},
	}
	entries := Roadmap(tasks, roadmapCfg())
	e1, ok := findEntry(entries, "t1")
	if !ok {
		t.Fatal("drift-status t1 must be INCLUDED (not confirmed-terminal)")
	}
	if e1.Ready {
		t.Fatal("drift-status t1 must be Ready=false (conservative)")
	}
	// t1 is a non-terminal node → hard-constrains t2 (emitted first) and appears in
	// t2's BlockedBy (not terminal-satisfied).
	if !sameIDs(entryIDs(entries), "t1", "t2") {
		t.Fatalf("order = %v, want [t1 t2] (drift blocker precedes dependent)", entryIDs(entries))
	}
	e2, _ := findEntry(entries, "t2")
	if e2.Ready || len(e2.BlockedBy) != 1 || e2.BlockedBy[0] != "t1" {
		t.Fatalf("t2: ready=%v blockedBy=%v, want ready=false blockedBy=[t1]", e2.Ready, e2.BlockedBy)
	}
}

// (iv) Terminal tasks excluded; (vii) empty & all-terminal sets → empty result.
func TestRoadmapExcludesTerminalAndEmpty(t *testing.T) {
	if got := Roadmap(nil, roadmapCfg()); len(got) != 0 {
		t.Fatalf("empty tasks → %v, want empty", got)
	}
	base := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	allTerminal := []mtt.Task{
		{ID: "t1", Type: "task", Status: "done", Created: base},
		{ID: "t2", Type: "task", Status: "cancelled", Created: base},
	}
	if got := Roadmap(allTerminal, roadmapCfg()); len(got) != 0 {
		t.Fatalf("all-terminal → %v, want empty", got)
	}
}

// (v) Cycle-safe on a hand-built depends_on cycle + a chain-into-cycle node: the
// function terminates and returns EVERY node.
func TestRoadmapCycleSafe(t *testing.T) {
	base := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		{ID: "t1", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t2"}, Created: base}, // cycle t1<->t2
		{ID: "t2", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t1"}, Created: base},
		{ID: "t3", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t1"}, Created: base}, // downstream of the cycle
		{ID: "t4", Type: "task", Status: "tbd", Priority: mtt.PriorityHigh, Created: base},    // independent
	}
	entries := Roadmap(tasks, roadmapCfg())
	if len(entries) != 4 {
		t.Fatalf("cycle roadmap returned %d entries, want all 4", len(entries))
	}
	// t4 is independent (indeg 0) → emitted before the stuck set.
	if entries[0].Task.ID != "t4" {
		t.Fatalf("first entry = %s, want t4 (the only unblocked node)", entries[0].Task.ID)
	}
	// t3 (chain into cycle) is present, in the stuck fallback.
	if _, ok := findEntry(entries, "t3"); !ok {
		t.Fatal("chain-into-cycle node t3 must still appear")
	}
}

// (vi) Deterministic order across runs (map iteration must not leak).
func TestRoadmapDeterministic(t *testing.T) {
	base := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		{ID: "t1", Type: "task", Status: "tbd", Priority: mtt.PriorityMedium, Created: base},
		{ID: "t2", Type: "task", Status: "tbd", Priority: mtt.PriorityHigh, Created: base},
		{ID: "t3", Type: "task", Status: "tbd", Priority: mtt.PriorityHigh, DependsOn: []mtt.TaskID{"t2"}, Created: base},
	}
	first := entryIDs(Roadmap(tasks, roadmapCfg()))
	for i := 0; i < 20; i++ {
		if got := entryIDs(Roadmap(tasks, roadmapCfg())); !sameIDs(got, first...) {
			t.Fatalf("run %d order = %v, want stable %v", i, got, first)
		}
	}
}
