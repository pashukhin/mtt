package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

func depTask(id mtt.TaskID, blockers ...mtt.TaskID) mtt.Task {
	return mtt.Task{ID: id, Type: "task", Status: "tbd", DependsOn: blockers,
		Created: time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC)}
}

func TestRenderDepList(t *testing.T) {
	g := core.NewDepGraph([]mtt.Task{depTask("t1"), depTask("t2", "t1", "ghost"), depTask("t3", "t2")})
	out := renderDepList(g, "t2")
	if !strings.Contains(out, "t2 depends on:") {
		t.Fatalf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "t1  task  [tbd]") {
		t.Fatalf("missing resolved blocker:\n%s", out)
	}
	if !strings.Contains(out, "ghost  (missing)") {
		t.Fatalf("missing dangling flag:\n%s", out)
	}
	if !strings.Contains(out, "required by:") || !strings.Contains(out, "t3  task  [tbd]") {
		t.Fatalf("missing dependents:\n%s", out)
	}
}

func TestBuildDepListJSONEmpty(t *testing.T) {
	g := core.NewDepGraph([]mtt.Task{depTask("t1")})
	v := buildDepListJSON(g, "t1")
	if v.DependsOn == nil || v.RequiredBy == nil {
		t.Fatalf("slices must be non-nil (marshal to [] not null): %+v", v)
	}
}

func TestRenderDepTree(t *testing.T) {
	// t3 -> t2 -> t1  (transitive blockers of t3)
	g := core.NewDepGraph([]mtt.Task{depTask("t1"), depTask("t2", "t1"), depTask("t3", "t2")})
	out := renderDepTree(g, "t3")
	if !strings.Contains(out, "t3  task  [tbd]") ||
		!strings.Contains(out, "└─ t2  task  [tbd]") ||
		!strings.Contains(out, "t1  task  [tbd]") {
		t.Fatalf("transitive tree wrong:\n%s", out)
	}
}

func TestBuildDepTreeJSONNested(t *testing.T) {
	g := core.NewDepGraph([]mtt.Task{depTask("t1"), depTask("t2", "t1")})
	v := buildDepTreeJSON(g, "t2")
	if v.ID != "t2" || len(v.DependsOn) != 1 || v.DependsOn[0].ID != "t1" {
		t.Fatalf("nested tree json wrong: %+v", v)
	}
}

func TestBuildDepTreeJSONDiamond(t *testing.T) {
	// Diamond: t1 -> {t2, t3}, both -> t4. The revisited t4 must still appear
	// under the second branch — as a node WITHOUT children (the text renderer's
	// revisit policy) — not vanish from the JSON graph.
	g := core.NewDepGraph([]mtt.Task{
		depTask("t4"), depTask("t2", "t4"), depTask("t3", "t4"), depTask("t1", "t2", "t3"),
	})
	v := buildDepTreeJSON(g, "t1")
	if len(v.DependsOn) != 2 {
		t.Fatalf("want 2 branches under t1: %+v", v)
	}
	first, second := v.DependsOn[0], v.DependsOn[1]
	if first.ID != "t2" || len(first.DependsOn) != 1 || first.DependsOn[0].ID != "t4" {
		t.Fatalf("first branch must nest t4: %+v", first)
	}
	if second.ID != "t3" || len(second.DependsOn) != 1 || second.DependsOn[0].ID != "t4" {
		t.Fatalf("revisited t4 dropped from the second branch (diamond): %+v", second)
	}
	if len(second.DependsOn[0].DependsOn) != 0 {
		t.Fatalf("revisited node must carry no children: %+v", second.DependsOn[0])
	}
}

func TestBuildDepTreeJSONCycleRevisit(t *testing.T) {
	// Hand-broken cycle t1 <-> t2: the back-edge renders as a childless node
	// (mirroring the text renderer's line + skip), never an infinite recursion.
	g := core.NewDepGraph([]mtt.Task{depTask("t1", "t2"), depTask("t2", "t1")})
	v := buildDepTreeJSON(g, "t1")
	if len(v.DependsOn) != 1 || v.DependsOn[0].ID != "t2" {
		t.Fatalf("tree json wrong: %+v", v)
	}
	back := v.DependsOn[0].DependsOn
	if len(back) != 1 || back[0].ID != "t1" || len(back[0].DependsOn) != 0 {
		t.Fatalf("cycle back-edge must render childless: %+v", v.DependsOn[0])
	}
}
