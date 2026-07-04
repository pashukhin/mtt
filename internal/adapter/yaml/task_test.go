package yaml

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

var _ mtt.TaskStore = (*Store)(nil)

func initDefault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	return root
}

func TestStoreCreateAndGet(t *testing.T) {
	root := initDefault(t)
	s := NewTaskStore(root)

	e1, err := s.Create(mtt.Task{Type: "epic", Title: "build auth", Status: "tbd", Created: fixedTime(), Updated: fixedTime()})
	if err != nil || e1.ID != "e1" {
		t.Fatalf("create epic = %q, %v; want e1", e1.ID, err)
	}
	if _, err := s.Create(mtt.Task{Type: "epic", Title: "second", Status: "tbd", Created: fixedTime(), Updated: fixedTime()}); err != nil {
		t.Fatal(err)
	}
	// independent per-prefix counter
	t1, err := s.Create(mtt.Task{Type: "task", Title: "orphan", Status: "tbd", Created: fixedTime(), Updated: fixedTime()})
	if err != nil || t1.ID != "t1" {
		t.Fatalf("create task = %q, %v; want t1", t1.ID, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".mtt", "tasks", "e1.yaml")); err != nil {
		t.Fatalf("e1 file missing: %v", err)
	}

	got, err := s.Get("e1")
	if err != nil {
		t.Fatalf("get e1: %v", err)
	}
	if got.Title != "build auth" || got.Type != "epic" || got.Status != "tbd" {
		t.Fatalf("round-trip lost fields: %+v", got)
	}

	if _, err := s.Get("nope"); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("get missing err = %v, want ErrNotFound", err)
	}
	if _, err := s.Create(mtt.Task{Type: "ghost", Status: "tbd", Created: fixedTime(), Updated: fixedTime()}); err == nil {
		t.Fatal("create with unknown type should error")
	}
}
