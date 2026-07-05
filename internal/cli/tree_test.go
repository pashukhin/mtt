package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

func treeCfg() mtt.Config {
	return mtt.Config{Types: []mtt.Type{
		{Name: "epic", Flow: mtt.Flow{Statuses: []mtt.Status{
			{Name: "tbd", Kind: mtt.KindInitial}, {Name: "doing", Kind: mtt.KindActive}, {Name: "done", Kind: mtt.KindTerminal},
		}}},
		{Name: "task", Parents: []mtt.TypeName{"epic"}, Flow: mtt.Flow{Statuses: []mtt.Status{
			{Name: "tbd", Kind: mtt.KindInitial}, {Name: "doing", Kind: mtt.KindActive}, {Name: "done", Kind: mtt.KindTerminal},
		}}},
	}}
}

func treeTasks() []mtt.Task {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	return []mtt.Task{
		{ID: "e1", Type: "epic", Status: "doing", Title: "E", Created: base},
		{ID: "t1", Type: "task", Status: "done", Title: "T1", Parent: "e1", Created: base.Add(2 * time.Hour)},
		{ID: "t2", Type: "task", Status: "tbd", Title: "T2", Parent: "e1", Created: base.Add(time.Hour)},
	}
}

func TestRenderTreeFull(t *testing.T) {
	x := core.NewIndex(treeTasks())
	out := renderTree(x, x.Roots(), core.ListFilter{}, treeCfg(), 0)
	for _, want := range []string{"e1  epic  [doing]  E", "t1  task  [done]  T1", "t2  task  [tbd]  T2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	// sibling order is deterministic: t1 (newer) before t2 (older)
	if strings.Index(out, "t1") > strings.Index(out, "t2") {
		t.Fatalf("sibling order wrong (want t1 before t2):\n%s", out)
	}
}

func TestRenderTreeKeepAncestors(t *testing.T) {
	// filter status=done: only t1 matches, but e1 (its non-matching parent) is kept as the path.
	x := core.NewIndex(treeTasks())
	out := renderTree(x, x.Roots(), core.ListFilter{Statuses: []string{"done"}}, treeCfg(), 0)
	if !strings.Contains(out, "e1  epic") {
		t.Fatalf("keep-ancestors: e1 should remain as path to t1:\n%s", out)
	}
	if !strings.Contains(out, "t1  task  [done]") {
		t.Fatalf("keep-ancestors: matching t1 should show:\n%s", out)
	}
	if strings.Contains(out, "t2") {
		t.Fatalf("keep-ancestors: non-matching t2 with no matching descendant should be dropped:\n%s", out)
	}
}

func TestRenderTreeDepth(t *testing.T) {
	x := core.NewIndex(treeTasks())
	out := renderTree(x, x.Roots(), core.ListFilter{}, treeCfg(), 1) // roots only
	if !strings.Contains(out, "e1  epic") {
		t.Fatalf("depth 1 should show the root:\n%s", out)
	}
	if strings.Contains(out, "t1") || strings.Contains(out, "t2") {
		t.Fatalf("depth 1 must not show children:\n%s", out)
	}
}

func TestBuildTreeJSONNested(t *testing.T) {
	x := core.NewIndex(treeTasks())
	nodes := buildTreeJSON(x, x.Roots(), core.ListFilter{}, treeCfg(), 0)
	if len(nodes) != 1 || nodes[0].ID != "e1" {
		t.Fatalf("want one root e1, got %+v", nodes)
	}
	if len(nodes[0].Children) != 2 || nodes[0].Children[0].ID != "t1" {
		t.Fatalf("want e1 -> [t1 t2], got %+v", nodes[0].Children)
	}
	if len(nodes[0].Children[0].Children) != 0 {
		t.Fatalf("t1 is a leaf: children should be empty")
	}
}

func TestBuildTreeJSONEmptyIsSlice(t *testing.T) {
	x := core.NewIndex(nil)
	if nodes := buildTreeJSON(x, x.Roots(), core.ListFilter{}, treeCfg(), 0); nodes == nil {
		t.Fatal("empty tree must be a non-nil slice so it marshals to [] not null")
	}
}
