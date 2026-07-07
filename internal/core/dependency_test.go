package core

import (
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// memStore is an in-memory TaskStore whose List returns its contents (the
// add_test fakeStore returns nil, which the cycle-check cannot use).
type memStore struct {
	byID    map[mtt.TaskID]mtt.Task
	updated mtt.Task
}

func newMemStore(tasks ...mtt.Task) *memStore {
	m := &memStore{byID: map[mtt.TaskID]mtt.Task{}}
	for _, t := range tasks {
		m.byID[t.ID] = t
	}
	return m
}

func (m *memStore) Create(t mtt.Task) (mtt.Task, error) { m.byID[t.ID] = t; return t, nil }
func (m *memStore) Get(id mtt.TaskID) (mtt.Task, error) {
	if t, ok := m.byID[id]; ok {
		return t, nil
	}
	return mtt.Task{}, mtt.ErrNotFound
}
func (m *memStore) List() ([]mtt.Task, error) {
	out := make([]mtt.Task, 0, len(m.byID))
	for _, t := range m.byID {
		out = append(out, t)
	}
	return out, nil
}
func (m *memStore) Update(t mtt.Task) (mtt.Task, error) {
	m.byID[t.ID] = t
	m.updated = t
	return t, nil
}
func (m *memStore) Delete(id mtt.TaskID) error {
	if _, ok := m.byID[id]; !ok {
		return mtt.ErrNotFound
	}
	delete(m.byID, id)
	return nil
}

func laterClock() time.Time { return time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC) }

func TestAddDependencyHappyPath(t *testing.T) {
	m := newMemStore(withDeps("t1"), withDeps("t2"))
	got, err := NewDependencyEditor(m, laterClock).AddDependency("t2", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.DependsOn) != 1 || got.DependsOn[0] != "t1" {
		t.Fatalf("DependsOn = %v; want [t1]", got.DependsOn)
	}
	if !got.Updated.Equal(laterClock().Truncate(time.Second)) {
		t.Fatalf("Updated not bumped: %v", got.Updated)
	}
	if len(m.updated.DependsOn) != 1 {
		t.Fatalf("not persisted via Update: %+v", m.updated)
	}
}

func TestAddDependencySelfRejected(t *testing.T) {
	m := newMemStore(withDeps("t1"))
	_, err := NewDependencyEditor(m, laterClock).AddDependency("t1", "t1")
	if err == nil || !strings.Contains(err.Error(), "itself") {
		t.Fatalf("err = %v; want a self-dependency error", err)
	}
}

func TestAddDependencyDuplicateNoop(t *testing.T) {
	m := newMemStore(withDeps("t1"), withDeps("t2", "t1"))
	before := m.byID["t2"].Updated
	got, err := NewDependencyEditor(m, laterClock).AddDependency("t2", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.DependsOn) != 1 {
		t.Fatalf("duplicate add changed DependsOn: %v", got.DependsOn)
	}
	if !got.Updated.Equal(before) {
		t.Fatalf("duplicate add bumped Updated: %v", got.Updated)
	}
}

func TestAddDependencyNotFound(t *testing.T) {
	m := newMemStore(withDeps("t1"))
	if _, err := NewDependencyEditor(m, laterClock).AddDependency("t1", "ghost"); err == nil ||
		!strings.Contains(err.Error(), "dependency") {
		t.Fatalf("err = %v; want dependency-not-found", err)
	}
	if _, err := NewDependencyEditor(m, laterClock).AddDependency("ghost", "t1"); err == nil ||
		!strings.Contains(err.Error(), "task") {
		t.Fatalf("err = %v; want task-not-found", err)
	}
}

func TestAddDependencyCycleRejected(t *testing.T) {
	// t2 depends on t1; adding t1 -> t2 would close a cycle.
	m := newMemStore(withDeps("t1"), withDeps("t2", "t1"))
	_, err := NewDependencyEditor(m, laterClock).AddDependency("t1", "t2")
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("err = %v; want a cycle rejection", err)
	}
}

func TestRemoveDependency(t *testing.T) {
	m := newMemStore(withDeps("t1"), withDeps("t2", "t1"))
	got, err := NewDependencyEditor(m, laterClock).RemoveDependency("t2", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.DependsOn) != 0 {
		t.Fatalf("DependsOn = %v; want empty", got.DependsOn)
	}
	if !got.Updated.Equal(laterClock().Truncate(time.Second)) {
		t.Fatalf("Updated not bumped on real removal: %v", got.Updated)
	}
}

func TestRemoveDependencyIdempotent(t *testing.T) {
	// t2 has no blockers; removing an absent edge is a no-op, not an error.
	m := newMemStore(withDeps("t1"), withDeps("t2"))
	before := m.byID["t2"].Updated
	got, err := NewDependencyEditor(m, laterClock).RemoveDependency("t2", "t1")
	if err != nil {
		t.Fatalf("idempotent rm errored: %v", err)
	}
	if len(got.DependsOn) != 0 {
		t.Fatalf("DependsOn = %v; want empty", got.DependsOn)
	}
	if !got.Updated.Equal(before) {
		t.Fatalf("no-op rm bumped Updated: %v", got.Updated)
	}
	// a missing task still errors
	if _, err := NewDependencyEditor(m, laterClock).RemoveDependency("ghost", "t1"); err == nil ||
		!strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v; want task-not-found", err)
	}
}
