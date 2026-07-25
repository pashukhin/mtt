package core

import "github.com/pashukhin/mtt/pkg/mtt"

// IntegrityKind is the closed vocabulary of integrity finding classes. The kind
// is self-describing of hardness: dangling-ref / dangling-dep / cycle gate
// (exit 7); unverified-ref is informational.
type IntegrityKind string

// The integrity finding classes; hardness is encoded in the kind itself (see the
// hard-kind exit rule in the CLI).
const (
	IntegrityDanglingRef   IntegrityKind = "dangling-ref"   // a refs entry to a missing target (HARD)
	IntegrityUnverifiedRef IntegrityKind = "unverified-ref" // a url / no-KB ref (SOFT, informational)
	IntegrityDanglingDep   IntegrityKind = "dangling-dep"   // a depends_on id absent from the task set (HARD)
	IntegrityCycle         IntegrityKind = "cycle"          // a depends_on cycle (HARD)
)

// DepFinding is a dangling blocking edge: Task depends on Missing, absent from
// the task set. References by identity (both mtt.TaskID), never the task value.
type DepFinding struct {
	Task    mtt.TaskID
	Missing mtt.TaskID
}

// IntegrityFinding is one problem found by the sweep; exactly one of Ref/Dep/Cycle
// is populated per Kind (a discriminated union).
type IntegrityFinding struct {
	Kind  IntegrityKind
	Ref   *CheckFinding
	Dep   *DepFinding
	Cycle []mtt.TaskID
}

// CheckIntegrity sweeps refs, dangling depends_on, and dependency cycles from one
// task/note snapshot in a deterministic order (refs in CheckRefs order, then deps
// by the task slice order, then cycles). Pure — no store, no clock.
func CheckIntegrity(tasks []mtt.Task, notes []mtt.Note, kbWired bool) []IntegrityFinding {
	var out []IntegrityFinding
	// 1. refs (reuse the existing sweep; map RefStatus -> kind)
	for _, rf := range CheckRefs(tasks, notes, kbWired) {
		rf := rf
		kind := IntegrityDanglingRef
		if rf.Status == RefUnverified {
			kind = IntegrityUnverifiedRef
		}
		out = append(out, IntegrityFinding{Kind: kind, Ref: &rf})
	}
	// 2. dangling depends_on (an id absent from the task set)
	exists := make(map[mtt.TaskID]bool, len(tasks))
	for _, t := range tasks {
		exists[t.ID] = true
	}
	for _, t := range tasks {
		for _, dep := range t.DependsOn {
			if !exists[dep] {
				out = append(out, IntegrityFinding{Kind: IntegrityDanglingDep, Dep: &DepFinding{Task: t.ID, Missing: dep}})
			}
		}
	}
	// 3. dependency cycles (DepGraph.Cycles is deterministic + cycle-safe)
	for _, cyc := range NewDepGraph(tasks).Cycles() {
		out = append(out, IntegrityFinding{Kind: IntegrityCycle, Cycle: cyc})
	}
	return out
}
