package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/pkg/mtt"
)

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
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != version {
		t.Fatalf("version output = %q, want %q", got, version)
	}
}
