package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestTaskJSONCarriesPriority(t *testing.T) {
	ts := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	j := toTaskJSON(mtt.Task{ID: "t1", Type: "task", Status: "tbd", Priority: mtt.PriorityLow, Created: ts, Updated: ts})
	if j.Priority != "low" {
		t.Fatalf("taskJSON.Priority = %q, want low", j.Priority)
	}
	// Unset priority is omitted from JSON (omitempty).
	data, err := json.Marshal(toTaskJSON(mtt.Task{ID: "t2", Type: "task", Status: "tbd", Created: ts, Updated: ts}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "priority") {
		t.Fatalf("unset priority must be omitted from JSON: %s", data)
	}
}

func TestToShowJSON(t *testing.T) {
	ts := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	task := mtt.Task{ID: "t1", Type: "task", Status: "in_progress", Created: ts, Updated: ts}
	onward := []mtt.Transition{{To: "done", Description: "quality gate"}, {To: "cancelled"}}

	sj := toShowJSON(task, "do the work", onward)
	if sj.ID != "t1" || sj.Status != "in_progress" {
		t.Fatalf("embedded taskJSON not promoted: %+v", sj)
	}
	if sj.StatusDescription != "do the work" {
		t.Fatalf("StatusDescription = %q, want %q", sj.StatusDescription, "do the work")
	}
	if len(sj.Next) != 2 || sj.Next[0].To != "done" || sj.Next[0].Description != "quality gate" || sj.Next[1].To != "cancelled" || sj.Next[1].Description != "" {
		t.Fatalf("Next = %+v", sj.Next)
	}

	// Empty guidance omits both fields; a populated one includes them.
	data, _ := json.Marshal(sj)
	if !strings.Contains(string(data), "status_description") || !strings.Contains(string(data), `"next"`) {
		t.Fatalf("populated guidance must include fields: %s", data)
	}
	empty, _ := json.Marshal(toShowJSON(task, "", nil))
	if strings.Contains(string(empty), "status_description") || strings.Contains(string(empty), "next") {
		t.Fatalf("empty guidance must omit both fields: %s", empty)
	}
}

func TestShowJSON(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runOut(t, "add", "--type", "epic", "build auth"); err != nil {
		t.Fatal(err)
	}
	out, _, err := runOut(t, "show", "--json", "e1")
	if err != nil {
		t.Fatalf("show --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	if got["id"] != "e1" || got["type"] != "epic" || got["title"] != "build auth" {
		t.Fatalf("json fields = %v", got)
	}
	if got["status"] != "tbd" {
		t.Fatalf("status = %v, want tbd", got["status"])
	}
	// show --json carries the flow guidance: the onward moves from the current
	// status (epic tbd -> in_progress, cancelled).
	next, ok := got["next"].([]any)
	if !ok || len(next) != 2 {
		t.Fatalf("next = %v, want 2 onward moves", got["next"])
	}
}
