package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestFormatTaskRendersHistory(t *testing.T) {
	task := mtt.Task{
		ID: "t1", Type: "task", Title: "A", Status: "in_progress",
		Created: time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
		Updated: time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
		History: []mtt.HistoryEntry{{
			At: time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
			By: "grisha", Role: "impl", From: "tbd", To: "in_progress",
			Checks: []mtt.Check{{Cmd: "make lint", Exit: 0}},
		}},
	}
	out := formatTask(task, nil, nil)
	for _, want := range []string{"history:", "tbd → in_progress", "by grisha", "role impl", "make lint(0)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("formatTask output missing %q:\n%s", want, out)
		}
	}
}

func TestFormatTask(t *testing.T) {
	ts := time.Date(2026, 7, 4, 9, 20, 0, 0, time.UTC)
	// a nested task with ancestors and children: lineage is the full root-to-self
	// path (self last), a children line lists direct children, no raw parent line.
	anc := []mtt.Task{{ID: "e1"}}
	kids := []mtt.Task{{ID: "s1"}, {ID: "s2"}}
	got := formatTask(mtt.Task{ID: "t1", Type: "task", Title: "fix login", Status: "tbd",
		Parent: "e1", Created: ts, Updated: ts, Description: "do the thing"}, anc, kids)
	want := "t1  task  [tbd]\n" +
		"  title:    fix login\n" +
		"  lineage:  e1 › t1\n" +
		"  children: 2 (s1, s2)\n" +
		"  created:  2026-07-04T09:20:00Z\n" +
		"  updated:  2026-07-04T09:20:00Z\n" +
		"\n  do the thing\n"
	if got != want {
		t.Fatalf("formatTask mismatch:\n got: %q\nwant: %q", got, want)
	}
	if strings.Contains(got, "parent:") {
		t.Fatalf("the raw parent line must be dropped: %q", got)
	}
	// a root task with no ancestors and no children: no lineage/children/parent lines
	bare := formatTask(mtt.Task{ID: "e1", Type: "epic", Status: "tbd", Created: ts, Updated: ts}, nil, nil)
	if strings.Contains(bare, "lineage:") || strings.Contains(bare, "children:") || strings.Contains(bare, "parent:") {
		t.Fatalf("bare root task should omit lineage/children/parent lines: %q", bare)
	}
}

func TestShowCommand(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, _, err := runOut(t, "add", "--type", "epic", "fix login"); err != nil {
		t.Fatalf("add: %v", err)
	}
	out, _, err := runOut(t, "show", "e1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	for _, want := range []string{"e1", "epic", "tbd", "fix login"} {
		if !strings.Contains(out, want) {
			t.Fatalf("show output %q missing %q", out, want)
		}
	}
	if _, _, err := runOut(t, "show", "missing"); err == nil {
		t.Fatal("show of a missing id should error")
	}
}

func TestShowArgError(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	// `show` with no id must give a human-friendly error, not cobra's default.
	_, _, err := runOut(t, "show")
	if err == nil || !strings.Contains(err.Error(), "task id") {
		t.Fatalf("show with no id should mention 'task id', got: %v", err)
	}
}
