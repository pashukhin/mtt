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
}
