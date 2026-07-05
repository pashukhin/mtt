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
