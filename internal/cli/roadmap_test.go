package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestWriteRoadmapHuman(t *testing.T) {
	entries := []core.RoadmapEntry{
		{Task: mtt.Task{ID: "t1", Type: "task", Status: "tbd", Priority: mtt.PriorityHigh, Title: "feature"}, Ready: true},
		{Task: mtt.Task{ID: "t2", Type: "task", Status: "tbd", Title: "docs"}, Ready: false, BlockedBy: []mtt.TaskID{"t1"}},
		{Task: mtt.Task{ID: "e1", Type: "epic", Status: "tbd", Title: "epic"}, Ready: true, Contains: []mtt.TaskID{"t1", "t2"}},
	}
	var b bytes.Buffer
	if err := writeRoadmap(&b, entries); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "1. t1  [high]  (tbd)  feature") {
		t.Fatalf("missing numbered priority row:\n%s", out)
	}
	if !strings.Contains(out, "2. t2  (tbd)  docs") {
		t.Fatalf("unset priority must omit the [..] label:\n%s", out)
	}
	if !strings.Contains(out, "↳ blocked by: t1") {
		t.Fatalf("missing blocked-by annotation:\n%s", out)
	}
	if !strings.Contains(out, "↳ contains: t1, t2") {
		t.Fatalf("missing contains annotation:\n%s", out)
	}
}

func TestRoadmapJSONShape(t *testing.T) {
	// A ready entry's blocked_by must serialize as [] (not null); priority is the
	// stored value, "" when unset (honest, not omitempty).
	views := []roadmapJSON{
		toRoadmapJSON(core.RoadmapEntry{Task: mtt.Task{ID: "t1", Status: "tbd", Priority: mtt.PriorityHigh}, Ready: true}),
		toRoadmapJSON(core.RoadmapEntry{Task: mtt.Task{ID: "t2", Status: "tbd"}, Ready: false, BlockedBy: []mtt.TaskID{"t1"}}),
		toRoadmapJSON(core.RoadmapEntry{Task: mtt.Task{ID: "e1", Status: "tbd"}, Ready: true, Contains: []mtt.TaskID{"t1", "t2"}}),
	}
	data, err := json.Marshal(views)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"blocked_by":[]`) {
		t.Fatalf("ready entry blocked_by must be [], got:\n%s", s)
	}
	if !strings.Contains(s, `"contains":[]`) {
		t.Fatalf("a leaf's contains must be [] (non-null), got:\n%s", s)
	}
	if !strings.Contains(s, `"contains":["t1","t2"]`) {
		t.Fatalf("a parent's contains must list children, got:\n%s", s)
	}
	if !strings.Contains(s, `"priority":""`) {
		t.Fatalf("unset priority must serialize as \"\" (honest), got:\n%s", s)
	}
	if !strings.Contains(s, `"priority":"high"`) {
		t.Fatalf("stored priority must serialize, got:\n%s", s)
	}
}
