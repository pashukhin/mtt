package cli

import (
	"bytes"
	"testing"
)

func TestTransitionSetsAndClearsCurrent(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, dir, "add", "A", "--no-parent"); err != nil {
		t.Fatal(err)
	}
	// tbd->in_progress carries current: set (default template) -> t1 becomes current
	if _, err := runCmd(t, dir, "in_progress", "t1"); err != nil {
		t.Fatalf("in_progress t1: %v", err)
	}
	if out, _ := runCmd(t, dir, "use"); !bytes.Contains([]byte(out), []byte("t1")) {
		t.Fatalf("after in_progress, use = %q, want t1 current", out)
	}
	// in_progress->done carries current: clear -> pointer cleared
	if _, err := runCmd(t, dir, "done", "t1"); err != nil {
		t.Fatalf("done t1: %v", err)
	}
	if out, _ := runCmd(t, dir, "use"); !bytes.Contains([]byte(out), []byte("no current task")) {
		t.Fatalf("after done, use = %q, want cleared", out)
	}
}
