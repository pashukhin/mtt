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

func TestRemoveManySubgraphIgnore(t *testing.T) {
	// e1 has child t1; deleting BOTH in one call ignores the in-set reference.
	m := newMemStore(
		mtt.Task{ID: "e1", Type: "epic", Status: "tbd"},
		mtt.Task{ID: "t1", Type: "task", Status: "tbd", Parent: "e1"},
	)
	res := NewRemover(m).RemoveMany([]mtt.TaskID{"e1", "t1"}, false)
	if len(res) != 2 || res[0].Err != nil || res[1].Err != nil {
		t.Fatalf("results = %+v; want both nil", res)
	}
	if len(m.byID) != 0 {
		t.Fatalf("store not empty: %v", m.byID)
	}
}

func TestRemoveManyExternalRejects(t *testing.T) {
	// deleting only e1 (child t1 NOT in the set) is rejected without --force.
	m := newMemStore(
		mtt.Task{ID: "e1", Type: "epic", Status: "tbd"},
		mtt.Task{ID: "t1", Type: "task", Status: "tbd", Parent: "e1"},
	)
	res := NewRemover(m).RemoveMany([]mtt.TaskID{"e1"}, false)
	if len(res) != 1 || res[0].Err == nil || !strings.Contains(res[0].Err.Error(), "t1") {
		t.Fatalf("res = %+v; want referenced-by-t1", res)
	}
	if _, ok := m.byID["e1"]; !ok {
		t.Fatal("e1 must survive a rejected delete")
	}
}

func TestRemoveManyForceOverrides(t *testing.T) {
	m := newMemStore(
		mtt.Task{ID: "e1", Type: "epic", Status: "tbd"},
		mtt.Task{ID: "t1", Type: "task", Status: "tbd", Parent: "e1"},
	)
	res := NewRemover(m).RemoveMany([]mtt.TaskID{"e1"}, true)
	if res[0].Err != nil {
		t.Fatalf("force err: %v", res[0].Err)
	}
	if _, ok := m.byID["e1"]; ok {
		t.Fatal("e1 not deleted under force")
	}
}

func TestRemoveManyBestEffort(t *testing.T) {
	// a missing id does not stop the rest; each has its own result.
	m := newMemStore(mtt.Task{ID: "t1", Type: "task", Status: "tbd"})
	res := NewRemover(m).RemoveMany([]mtt.TaskID{"t1", "t99"}, false)
	if len(res) != 2 || res[0].Err != nil {
		t.Fatalf("t1 should succeed: %+v", res)
	}
	if !errors.Is(res[1].Err, mtt.ErrNotFound) {
		t.Fatalf("t99 err = %v; want ErrNotFound", res[1].Err)
	}
	if _, ok := m.byID["t1"]; ok {
		t.Fatal("t1 not deleted")
	}
}

func TestRemoveManyDedup(t *testing.T) {
	m := newMemStore(mtt.Task{ID: "t1", Type: "task", Status: "tbd"})
	res := NewRemover(m).RemoveMany([]mtt.TaskID{"t1", "t1"}, false)
	if len(res) != 1 || res[0].Err != nil {
		t.Fatalf("res = %+v; want a single ok", res)
	}
}
