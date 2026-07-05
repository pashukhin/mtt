package core

import (
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// dep builds a task of the default "task" type with a status and blockers.
func dep(id mtt.TaskID, status mtt.StatusName, blockers ...mtt.TaskID) mtt.Task {
	return mtt.Task{ID: id, Type: "task", Status: status, DependsOn: blockers, Created: fixed()}
}

func TestReadyConservative(t *testing.T) {
	tasks := []mtt.Task{
		dep("t1", "tbd"),          // no blockers, non-terminal → ready
		dep("t2", "tbd", "t1"),    // blocker t1 is tbd (non-terminal) → not ready
		dep("t3", "done"),         // terminal itself → not ready
		dep("t4", "tbd", "ghost"), // dangling blocker → not ready
		dep("t5", "weird"),        // status not in flow (drift) → not ready
	}
	got := ids(Ready(tasks, cfg()))
	if len(got) != 1 || got[0] != "t1" {
		t.Fatalf("Ready = %v; want [t1]", got)
	}
}

func TestReadyBlockerDoneUnblocks(t *testing.T) {
	tasks := []mtt.Task{
		dep("t1", "done"),      // terminal blocker
		dep("t2", "tbd", "t1"), // blocker terminal → ready
	}
	got := ids(Ready(tasks, cfg()))
	if len(got) != 1 || got[0] != "t2" {
		t.Fatalf("Ready = %v; want [t2] (t1 is terminal, excluded; t2 unblocked)", got)
	}
}

func TestKindOf(t *testing.T) {
	if k, ok := kindOf(dep("x", "done"), cfg()); !ok || k != mtt.KindTerminal {
		t.Fatalf("kindOf(done) = %v,%v; want terminal,true", k, ok)
	}
	if _, ok := kindOf(dep("x", "nope"), cfg()); ok {
		t.Fatalf("kindOf(unknown status) ok=true; want false")
	}
}
