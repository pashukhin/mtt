package cli

import (
	"strings"
	"testing"
)

func TestResolveTaskID(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, dir, "add", "A", "--no-parent"); err != nil {
		t.Fatal(err)
	}
	// explicit wins
	if id, err := resolveTaskID(dir, "t1"); err != nil || id != "t1" {
		t.Fatalf("explicit = (%q,%v)", id, err)
	}
	// no current -> error
	if _, err := resolveTaskID(dir, ""); err == nil || !strings.Contains(err.Error(), "no current task") {
		t.Fatalf("no-current err = %v", err)
	}
	// set current -> resolves
	if _, err := runCmd(t, dir, "use", "t1"); err != nil {
		t.Fatal(err)
	}
	if id, err := resolveTaskID(dir, ""); err != nil || id != "t1" {
		t.Fatalf("current = (%q,%v)", id, err)
	}
}

func TestOmittedIdVerbsUseCurrent(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, dir, "add", "A", "--no-parent"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, dir, "use", "t1"); err != nil {
		t.Fatal(err)
	}
	// show with no id -> current
	if out, err := runCmd(t, dir, "show"); err != nil || !strings.Contains(out, "t1") {
		t.Fatalf("show (no id) = %q, %v", out, err)
	}
	// edit with no id -> current
	if out, err := runCmd(t, dir, "edit", "--title", "renamed"); err != nil || !strings.Contains(out, "updated t1") {
		t.Fatalf("edit (no id) = %q, %v", out, err)
	}
	// status with 1 arg -> current
	if out, err := runCmd(t, dir, "status", "in_progress"); err != nil || !strings.Contains(out, "tbd → in_progress") {
		t.Fatalf("status in_progress (no id) = %q, %v", out, err)
	}
	// bare sugar `mtt done` on current (now in_progress) -> moves + clears
	if out, err := runCmd(t, dir, "done"); err != nil || !strings.Contains(out, "in_progress → done") {
		t.Fatalf("done (no id) = %q, %v", out, err)
	}
	// current cleared by the done edge -> bare verb now errors helpfully
	if _, err := runCmd(t, dir, "done"); err == nil {
		t.Fatal("done with no current = nil, want error")
	}
	// unknown first arg still unknown command
	if _, err := runCmd(t, dir, "bogus"); err == nil {
		t.Fatal("bogus = nil, want unknown command")
	}
}
