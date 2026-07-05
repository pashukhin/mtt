package core

import (
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func withDeps(id mtt.TaskID, blockers ...mtt.TaskID) mtt.Task {
	return mtt.Task{ID: id, Type: "task", Status: "tbd", DependsOn: blockers, Created: fixed()}
}

func TestDepGraphDependentsAndReaches(t *testing.T) {
	// t3 -> t2 -> t1  (t3 depends on t2, t2 depends on t1)
	g := NewDepGraph([]mtt.Task{withDeps("t1"), withDeps("t2", "t1"), withDeps("t3", "t2")})

	if got := ids(g.Dependents("t1")); len(got) != 1 || got[0] != "t2" {
		t.Fatalf("Dependents(t1) = %v; want [t2]", got)
	}
	if !g.Reaches("t3", "t1") {
		t.Fatalf("Reaches(t3,t1) = false; want true (transitive)")
	}
	if g.Reaches("t1", "t3") {
		t.Fatalf("Reaches(t1,t3) = true; want false (wrong direction)")
	}
	if got := g.DependsOn("t2"); len(got) != 1 || got[0] != "t1" {
		t.Fatalf("DependsOn(t2) = %v; want [t1]", got)
	}
}

func TestDepGraphCyclesHandBuilt(t *testing.T) {
	// a -> b -> a  (hand-broken data; the CLI cannot create this)
	g := NewDepGraph([]mtt.Task{withDeps("a", "b"), withDeps("b", "a")})
	cycles := g.Cycles()
	if len(cycles) == 0 {
		t.Fatalf("Cycles() = []; want at least one cycle")
	}
	// acyclic graph reports none
	clean := NewDepGraph([]mtt.Task{withDeps("t1"), withDeps("t2", "t1")})
	if got := clean.Cycles(); len(got) != 0 {
		t.Fatalf("Cycles(acyclic) = %v; want []", got)
	}
}

func TestDepGraphReachesCycleSafe(t *testing.T) {
	g := NewDepGraph([]mtt.Task{withDeps("a", "b"), withDeps("b", "a")})
	// must terminate even though a<->b cycle; b is reachable from a
	if !g.Reaches("a", "b") {
		t.Fatalf("Reaches(a,b) = false; want true")
	}
}
