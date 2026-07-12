package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestExitCodeNotFound(t *testing.T) {
	err := fmt.Errorf("task %q: %w", "t9", mtt.ErrNotFound)
	if got := exitCode(err); got != 4 {
		t.Fatalf("exitCode(ErrNotFound) = %d; want 4", got)
	}
}

// TestRmMissingTaskExit4 pins the full rm path end-to-end: a missing id flows
// core.Remover → wrapped ErrNotFound → exitCode 4 (testscript can only assert
// non-zero, so this regression-locks the numeric code).
func TestRmMissingTaskExit4(t *testing.T) {
	dir := t.TempDir()
	if err := yaml.Init(dir, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--dir", dir, "rm", "t99"})
	err := root.Execute()
	if !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("rm missing err = %v; want ErrNotFound wrap", err)
	}
	if got := exitCode(err); got != 4 {
		t.Fatalf("exitCode = %d; want 4", got)
	}
}

// TestSugarMissingTaskExit4 pins the uniform-exit-4 gap fix: the `mtt <status>
// <id>` sugar on a missing task now wraps ErrNotFound (exit 4), instead of the
// misleading "unknown command" (exit 1), when arg0 is a plausible status verb.
func TestSugarMissingTaskExit4(t *testing.T) {
	dir, _ := initSugarProject(t)
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--dir", dir, "done", "t99"})
	err := root.Execute()
	if !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("sugar-missing err = %v; want ErrNotFound wrap", err)
	}
	// a non-status arg0 on a missing id stays an unknown command (exit 1).
	root2 := NewRootCmd()
	root2.SetOut(&out)
	root2.SetErr(&out)
	root2.SetArgs([]string{"--dir", dir, "bogus", "t99"})
	if err := root2.Execute(); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("bogus arg0 err = %v; want unknown command", err)
	}
}

// initSugarProject inits a default project in a temp dir and mints one task at
// the initial status, returning the project dir and the minted task id.
func initSugarProject(t *testing.T) (string, mtt.TaskID) {
	t.Helper()
	dir := t.TempDir()
	if err := yaml.Init(dir, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	task, err := yaml.NewTaskStore(dir).Create(mtt.Task{Type: "task", Title: "A", Status: "tbd", Created: now, Updated: now})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	return dir, task.ID
}

func TestSugarRoutesStatusMove(t *testing.T) {
	dir, id := initSugarProject(t)
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--dir", dir, "in_progress", string(id)})
	if err := root.Execute(); err != nil {
		t.Fatalf("sugar move: %v", err)
	}
	if !strings.Contains(out.String(), "tbd → in_progress") {
		t.Fatalf("sugar did not route the move; output:\n%s", out.String())
	}
}

func TestSugarUnknownFirstArg(t *testing.T) {
	dir, id := initSugarProject(t)
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--dir", dir, "bogus", string(id)})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("bogus first arg must be unknown command; got err=%v", err)
	}
}

func TestVersionCommand(t *testing.T) {
	root := NewRootCmd()
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}
	// version prints to STDOUT (not stderr), so an agent can capture it.
	if got := strings.TrimSpace(outBuf.String()); got != resolveVersion() {
		t.Fatalf("version stdout = %q, want %q", got, resolveVersion())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("version wrote to stderr: %q", errBuf.String())
	}
}

func TestRootShortNamesTheGate(t *testing.T) {
	// U5: the first line an agent reads should name the killer feature (the empty
	// niche), not the crowded "file-backed tracker" category.
	s := NewRootCmd().Short
	if !strings.Contains(s, "state machine") || !strings.Contains(strings.ToLower(s), "gate") {
		t.Fatalf("root Short must name the gate/state-machine feature: %q", s)
	}
}

func TestRootHelpMentionsSugar(t *testing.T) {
	// U4: the verb sugar (mtt <status> [<id>]) must be discoverable from --help,
	// since mtt's adoption theory is "the agent learns from the CLI itself".
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute --help: %v", err)
	}
	if !strings.Contains(out.String(), "<status>") {
		t.Fatalf("root help must mention the verb sugar: %s", out.String())
	}
}

func TestProjectRootNoDirHintsInit(t *testing.T) {
	// U4: an explicit --dir with no .mtt/ names `mtt init`. Root sets
	// SilenceErrors=true, so assert the RETURNED error (a stderr buffer stays empty).
	dir := t.TempDir()
	root := NewRootCmd()
	root.SetArgs([]string{"--dir", dir, "list"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "mtt init") {
		t.Fatalf("no-project error must name `mtt init`, got: %v", err)
	}
}
