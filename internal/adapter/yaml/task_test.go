package yaml

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

var _ mtt.TaskStore = (*Store)(nil)

func TestListNamesCorruptFile(t *testing.T) {
	root := initDefault(t)
	// A zero-byte task file (the mint-window / crash artifact from A1): Unmarshal
	// yields a zero DTO, toDomain fails on the empty id. The List error must name
	// the offending file so it is a one-command fix at volume.
	bad := filepath.Join(root, ".mtt", "tasks", "t99.yaml")
	if err := os.MkdirAll(filepath.Dir(bad), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewTaskStore(root).List()
	if err == nil {
		t.Fatal("List over a zero-byte task file must error")
	}
	if !strings.Contains(err.Error(), "t99.yaml") {
		t.Fatalf("List error must name the offending file, got: %v", err)
	}
}

func TestGetNamesCorruptFile(t *testing.T) {
	root := initDefault(t)
	// A non-empty but domain-invalid file (empty id) — corrupt, not absent. The
	// wrapped error must NOT be ErrNotFound (that would mis-map to exit 4).
	bad := filepath.Join(root, ".mtt", "tasks", "t98.yaml")
	if err := os.MkdirAll(filepath.Dir(bad), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("type: task\nstatus: tbd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewTaskStore(root).Get("t98")
	if err == nil {
		t.Fatal("Get over a corrupt task file must error")
	}
	if errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("a corrupt file is not ErrNotFound (that would mis-map to exit 4): %v", err)
	}
	if !strings.Contains(err.Error(), "t98.yaml") {
		t.Fatalf("Get error must name the offending file, got: %v", err)
	}
}

func initDefault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	return root
}

func TestDeleteRemovesFile(t *testing.T) {
	root := initDefault(t)
	s := NewTaskStore(root)
	created, err := s.Create(mtt.Task{Type: "epic", Title: "E", Status: "tbd", Created: fixedTime(), Updated: fixedTime()})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Delete(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(created.ID); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("Get after Delete = %v; want ErrNotFound", err)
	}
}

func TestDeleteAbsentIsNotFound(t *testing.T) {
	root := initDefault(t)
	if err := NewTaskStore(root).Delete("t99"); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("Delete absent = %v; want ErrNotFound", err)
	}
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

func TestStoreList(t *testing.T) {
	root := initDefault(t)
	s := NewTaskStore(root)

	if tasks, err := s.List(); err != nil || len(tasks) != 0 {
		t.Fatalf("empty list = %v, %v; want 0 tasks", tasks, err)
	}
	if _, err := s.Create(mtt.Task{Type: "epic", Title: "a", Status: "tbd", Created: fixedTime(), Updated: fixedTime()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(mtt.Task{Type: "epic", Title: "b", Status: "tbd", Created: fixedTime(), Updated: fixedTime()}); err != nil {
		t.Fatal(err)
	}
	tasks, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("list len = %d, want 2", len(tasks))
	}
	titles := map[string]bool{}
	for _, tk := range tasks {
		titles[tk.Title] = true
	}
	if !titles["a"] || !titles["b"] {
		t.Fatalf("round-trip titles = %v, want a and b", titles)
	}
}

func TestStoreUpdate(t *testing.T) {
	root := initDefault(t)
	s := NewTaskStore(root)

	created, err := s.Create(mtt.Task{Type: "epic", Title: "old", Status: "tbd", Created: fixedTime(), Updated: fixedTime()})
	if err != nil {
		t.Fatal(err)
	}
	created.Title = "new"
	if _, err := s.Update(created); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "new" {
		t.Fatalf("title after update = %q, want new", got.Title)
	}
	if _, err := s.Update(mtt.Task{ID: "e999", Type: "epic", Status: "tbd", Created: fixedTime(), Updated: fixedTime()}); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("update missing = %v, want ErrNotFound", err)
	}
}
