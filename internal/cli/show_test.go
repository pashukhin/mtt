package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestFormatTask(t *testing.T) {
	ts := time.Date(2026, 7, 4, 9, 20, 0, 0, time.UTC)
	got := formatTask(mtt.Task{ID: "t1", Type: "task", Title: "fix login", Status: "tbd",
		Parent: "e1", Created: ts, Updated: ts, Description: "do the thing"})
	want := "t1  task  [tbd]\n" +
		"  title:    fix login\n" +
		"  parent:   e1\n" +
		"  created:  2026-07-04T09:20:00Z\n" +
		"  updated:  2026-07-04T09:20:00Z\n" +
		"\n  do the thing\n"
	if got != want {
		t.Fatalf("formatTask mismatch:\n got: %q\nwant: %q", got, want)
	}
	// no title, no parent, no description => those lines are omitted
	bare := formatTask(mtt.Task{ID: "e1", Type: "epic", Status: "tbd", Created: ts, Updated: ts})
	if strings.Contains(bare, "title:") || strings.Contains(bare, "parent:") {
		t.Fatalf("bare task should omit title/parent lines: %q", bare)
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
