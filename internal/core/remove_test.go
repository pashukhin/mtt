package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestRemoveUnreferenced(t *testing.T) {
	m := newMemStore(mtt.Task{ID: "t1", Type: "task", Status: "tbd"})
	if err := NewRemover(m).Remove("t1", false); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, ok := m.byID["t1"]; ok {
		t.Fatal("task not deleted from store")
	}
}

func TestRemoveNotFound(t *testing.T) {
	m := newMemStore()
	err := NewRemover(m).Remove("t99", false)
	if !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("err = %v; want ErrNotFound", err)
	}
}

func TestRemoveRejectedByDependent(t *testing.T) {
	// t2 depends on t1; removing t1 without --force is rejected and t1 survives.
	m := newMemStore(
		mtt.Task{ID: "t1", Type: "task", Status: "tbd"},
		mtt.Task{ID: "t2", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t1"}},
	)
	err := NewRemover(m).Remove("t1", false)
	if err == nil || !strings.Contains(err.Error(), "t2") {
		t.Fatalf("err = %v; want a referenced-by-t2 error", err)
	}
	if _, ok := m.byID["t1"]; !ok {
		t.Fatal("t1 must NOT be deleted on a rejected remove")
	}
}

func TestRemoveRejectedByChild(t *testing.T) {
	// s1's parent is t1; removing t1 without --force is rejected.
	m := newMemStore(
		mtt.Task{ID: "t1", Type: "task", Status: "tbd"},
		mtt.Task{ID: "s1", Type: "subtask", Status: "tbd", Parent: "t1"},
	)
	err := NewRemover(m).Remove("t1", false)
	if err == nil || !strings.Contains(err.Error(), "s1") {
		t.Fatalf("err = %v; want a referenced-by-s1 error", err)
	}
}

func TestRemoveForceDeletesReferenced(t *testing.T) {
	m := newMemStore(
		mtt.Task{ID: "t1", Type: "task", Status: "tbd"},
		mtt.Task{ID: "t2", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t1"}},
	)
	if err := NewRemover(m).Remove("t1", true); err != nil {
		t.Fatalf("force remove: %v", err)
	}
	if _, ok := m.byID["t1"]; ok {
		t.Fatal("t1 not deleted under --force")
	}
}

func TestRemoveReferencedDedup(t *testing.T) {
	// t2 is BOTH a child and a dependent of t1 → must appear once in the message.
	m := newMemStore(
		mtt.Task{ID: "t1", Type: "task", Status: "tbd"},
		mtt.Task{ID: "t2", Type: "task", Status: "tbd", Parent: "t1", DependsOn: []mtt.TaskID{"t1"}},
	)
	err := NewRemover(m).Remove("t1", false)
	if err == nil || strings.Count(err.Error(), "t2") != 1 {
		t.Fatalf("err = %v; want t2 mentioned exactly once", err)
	}
}
