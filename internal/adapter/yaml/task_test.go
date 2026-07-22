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
	root := initHierarchy(t)
	// A non-empty but domain-invalid task file (empty id): the List error must
	// name the offending file so it is a one-command fix at volume (A1).
	bad := filepath.Join(root, ".mtt", "tasks", "t99.yaml")
	if err := os.MkdirAll(filepath.Dir(bad), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("type: task\nstatus: tbd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewTaskStore(root).List()
	if err == nil {
		t.Fatal("List over a corrupt task file must error")
	}
	if !strings.Contains(err.Error(), "t99.yaml") {
		t.Fatalf("List error must name the offending file, got: %v", err)
	}
}

func TestReserveArtifactIsInvisibleAndNeverReminted(t *testing.T) {
	root := initHierarchy(t)
	s := NewTaskStore(root)
	if _, err := s.Create(mtt.Task{Type: "task", Title: "A", Status: "tbd", Created: fixedTime(), Updated: fixedTime()}); err != nil {
		t.Fatalf("create: %v", err)
	}
	// A zero-byte task file is the mint reserve window artifact (a crash between
	// the O_EXCL reserve and the content write, c18) — not corruption. Reads must
	// skip it instead of fail-stopping the whole store.
	artifact := filepath.Join(root, ".mtt", "tasks", "t7.yaml")
	if err := os.WriteFile(artifact, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	tasks, err := NewTaskStore(root).List()
	if err != nil {
		t.Fatalf("List must skip a zero-byte reserve artifact, got: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "t1" {
		t.Fatalf("List = %v, want just t1", tasks)
	}
	// Get on the artifact is "no such task", not a corrupt-file error.
	if _, err := s.Get("t7"); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("Get on a reserve artifact = %v; want ErrNotFound", err)
	}
	// The reserved id stays consumed: mint counts the filename, so the next
	// create must NOT reuse t7 (a reuse would re-point dangling references).
	next, err := s.Create(mtt.Task{Type: "task", Title: "B", Status: "tbd", Created: fixedTime(), Updated: fixedTime()})
	if err != nil {
		t.Fatalf("create after artifact: %v", err)
	}
	if next.ID != "t8" {
		t.Fatalf("next minted id = %q, want t8 (never reuse the reserved t7)", next.ID)
	}
}

func TestWritePermsUniform(t *testing.T) {
	root := initHierarchy(t)
	s := NewTaskStore(root)
	created, err := s.Create(mtt.Task{Type: "task", Title: "A", Status: "tbd", Created: fixedTime(), Updated: fixedTime()})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// One perm policy (0644, the git-checkout default) for every store write —
	// CreateTemp's 0600 must not leak through atomicWrite (c18: fresh writes vs
	// committed checkouts flip-flopped and were noisy cross-machine).
	for _, p := range []string{
		filepath.Join(root, ".mtt", "config.yaml"),
		filepath.Join(root, ".mtt", "tasks", string(created.ID)+".yaml"),
	} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o644 {
			t.Fatalf("%s perm = %o, want 644", p, perm)
		}
	}
}

func TestGetNamesCorruptFile(t *testing.T) {
	root := initHierarchy(t)
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

func initHierarchy(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := Init(root, "hierarchy", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	return root
}

func TestDeleteRemovesFile(t *testing.T) {
	root := initHierarchy(t)
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
	root := initHierarchy(t)
	if err := NewTaskStore(root).Delete("t99"); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("Delete absent = %v; want ErrNotFound", err)
	}
}

func TestStoreCreateAndGet(t *testing.T) {
	root := initHierarchy(t)
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
	root := initHierarchy(t)
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
	root := initHierarchy(t)
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
