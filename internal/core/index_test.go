package core

import (
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func node(id, parent string, created time.Time) mtt.Task {
	return mtt.Task{ID: id, Type: "task", Status: "tbd", Parent: parent, Created: created, Updated: created}
}

func TestIndexRootsChildrenAncestors(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		node("e1", "", base),
		node("t1", "e1", base.Add(2*time.Hour)), // newer
		node("t2", "e1", base.Add(time.Hour)),   // older
		node("s1", "t1", base),
		node("x1", "ghost", base), // orphan: parent absent
	}
	x := NewIndex(tasks)

	roots := x.Roots()
	if len(roots) != 2 {
		t.Fatalf("roots = %d, want 2 (e1 + orphan x1)", len(roots))
	}

	kids := x.Children("e1")
	if len(kids) != 2 || kids[0].ID != "t1" || kids[1].ID != "t2" {
		t.Fatalf("children(e1) = %v; want [t1 t2] (Created desc)", ids(kids))
	}

	anc := x.Ancestors("s1")
	if len(anc) != 2 || anc[0].ID != "e1" || anc[1].ID != "t1" {
		t.Fatalf("ancestors(s1) = %v; want [e1 t1] (root-first)", ids(anc))
	}
	if got := x.Ancestors("e1"); len(got) != 0 {
		t.Fatalf("ancestors(root) = %v; want empty", ids(got))
	}
}

func TestIndexAncestorsCycleSafe(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	// a <-> b mutually parent each other (hand-broken data).
	x := NewIndex([]mtt.Task{node("a", "b", base), node("b", "a", base)})
	if got := x.Ancestors("a"); len(got) > 2 {
		t.Fatalf("cycle walk did not terminate: %v", ids(got))
	}
}

func ids(ts []mtt.Task) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}
