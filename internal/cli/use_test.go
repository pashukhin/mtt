package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCmd(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"--dir", dir}, args...))
	err := root.Execute()
	return out.String(), err
}

func TestUseSetShowClear(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := runCmd(t, dir, "add", "A", "--no-parent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// show with no current
	out, err := runCmd(t, dir, "use")
	if err != nil || !bytes.Contains([]byte(out), []byte("no current task")) {
		t.Fatalf("use (empty) = %q, %v", out, err)
	}
	// set
	out, err = runCmd(t, dir, "use", "t1")
	if err != nil || !bytes.Contains([]byte(out), []byte("current: t1")) {
		t.Fatalf("use t1 = %q, %v", out, err)
	}
	// show
	out, _ = runCmd(t, dir, "use")
	if !bytes.Contains([]byte(out), []byte("t1")) {
		t.Fatalf("use show = %q", out)
	}
	// clear
	out, _ = runCmd(t, dir, "use", "--clear")
	if !bytes.Contains([]byte(out), []byte("current cleared")) {
		t.Fatalf("use --clear = %q", out)
	}
	// set a missing task -> error, pointer not written
	if _, err := runCmd(t, dir, "use", "t99"); err == nil {
		t.Fatal("use t99 = nil err, want not-found")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, ".mtt", "config.local.yaml")); bytes.Contains(data, []byte("t99")) {
		t.Fatal("missing task was written to current")
	}
}

func TestUseClearRejectsID(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, dir, "add", "A", "--no-parent"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, dir, "use", "t1", "--clear"); err == nil {
		t.Fatal("use t1 --clear = nil, want a mutual-exclusion error")
	}
}

func TestUseJSON(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, dir, "add", "A", "--no-parent"); err != nil {
		t.Fatal(err)
	}
	// set --json emits the task object
	out, err := runCmd(t, dir, "use", "t1", "--json")
	if err != nil || !strings.Contains(out, `"id"`) || !strings.Contains(out, "t1") {
		t.Fatalf("use t1 --json = %q, %v", out, err)
	}
	// show --json emits the task object
	out, err = runCmd(t, dir, "use", "--json")
	if err != nil || !strings.Contains(out, `"id"`) || !strings.Contains(out, "t1") {
		t.Fatalf("use --json (show) = %q, %v", out, err)
	}
}
