package yaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// A task file's on-disk id is expanded into {{.ID}} inside gate/post shell
// commands, so the load path must refuse an id that does not match its filename
// (a split-brain / spoof) or that carries characters outside the adapter's id
// encoding (a shell-injection payload). These tests pin that fail-closed load.

const validTaskBody = "type: task\nstatus: tbd\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n"

// writeRawTask writes a task file verbatim (bypassing the store), simulating a
// hand-poisoned .mtt/tasks/*.yaml that rides a PR through the review blind spot.
func writeRawTask(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, ".mtt", "tasks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGetRejectsIDFilenameMismatch(t *testing.T) {
	root := initHierarchy(t)
	// t5.yaml claims to be t1 inside: a duplicate-id split-brain. Without the
	// cross-check it loads silently as t1, and mtt operates on the wrong task.
	writeRawTask(t, root, "t5.yaml", "id: t1\n"+validTaskBody)

	got, err := NewTaskStore(root).Get("t5")
	if err == nil {
		t.Fatalf("Get(t5) over a file whose id: is t1 must be refused, got task %+v", got)
	}
	if string(got.ID) == "t1" {
		t.Fatalf("the mismatched id must not leak into the domain, got %q", got.ID)
	}
}

func TestListRejectsIDFilenameMismatch(t *testing.T) {
	root := initHierarchy(t)
	writeRawTask(t, root, "t1.yaml", "id: t1\n"+validTaskBody) // well-formed
	writeRawTask(t, root, "t5.yaml", "id: t1\n"+validTaskBody) // stem t5, id t1

	if _, err := NewTaskStore(root).List(); err == nil {
		t.Fatal("List must fail closed on an id/filename mismatch, not load it silently")
	}
}

func TestListRejectsShellInjectionID(t *testing.T) {
	root := initHierarchy(t)
	// Filename stem == in-file id, so the mismatch guard passes and only the
	// charset guard stands between the payload and {{.ID}} in a sh -c command.
	payload := `t1; touch PWNED`
	writeRawTask(t, root, payload+".yaml", "id: '"+payload+"'\n"+validTaskBody)

	if _, err := NewTaskStore(root).List(); err == nil {
		t.Fatalf("List must refuse a shell-injection id %q, not admit it to the domain", payload)
	}
}

func TestGetRejectsShellInjectionID(t *testing.T) {
	root := initHierarchy(t)
	payload := `t1; touch PWNED`
	writeRawTask(t, root, payload+".yaml", "id: '"+payload+"'\n"+validTaskBody)

	got, err := NewTaskStore(root).Get(mtt.TaskID(payload))
	if err == nil {
		t.Fatalf("Get over a shell-injection id must be refused, got task %+v", got)
	}
	if strings.Contains(string(got.ID), ";") {
		t.Fatalf("a shell-metacharacter id must not leak into the domain, got %q", got.ID)
	}
}
