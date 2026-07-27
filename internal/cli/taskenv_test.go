package cli

import (
	"encoding/json"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// envStubStore is a minimal TaskStore for the env-builder test.
type envStubStore struct{ tasks []mtt.Task }

func (s envStubStore) Create(t mtt.Task) (mtt.Task, error) { return t, nil }
func (s envStubStore) Get(id mtt.TaskID) (mtt.Task, error) {
	for _, t := range s.tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return mtt.Task{}, mtt.ErrNotFound
}
func (s envStubStore) List() ([]mtt.Task, error)           { return s.tasks, nil }
func (s envStubStore) Update(t mtt.Task) (mtt.Task, error) { return t, nil }
func (s envStubStore) Delete(_ mtt.TaskID) error           { return nil }

func TestTaskEnvBuilder(t *testing.T) {
	parent := mtt.Task{ID: "t1", Type: "task", Title: "hello, world", Status: "in_progress", Priority: mtt.PriorityHigh, Tags: []string{"a", "b"}}
	child := mtt.Task{ID: "t2", Type: "task", Status: "done", Parent: "t1"}
	env := taskEnvBuilder(envStubStore{tasks: []mtt.Task{parent, child}})(parent)

	if env["MTT_TASK_ID"] != "t1" || env["MTT_TASK_STATUS"] != "in_progress" || env["MTT_TASK_TITLE"] != "hello, world" {
		t.Fatalf("scalars wrong: %+v", env)
	}
	if env["MTT_TASK_TYPE"] != "task" || env["MTT_TASK_PRIORITY"] != "high" || env["MTT_TASK_TAGS"] != "a,b" {
		t.Fatalf("scalars wrong: %+v", env)
	}
	var children []map[string]string
	if err := json.Unmarshal([]byte(env["MTT_TASK_CHILDREN_JSON"]), &children); err != nil {
		t.Fatalf("children json %q: %v", env["MTT_TASK_CHILDREN_JSON"], err)
	}
	if len(children) != 1 || children[0]["id"] != "t2" || children[0]["status"] != "done" {
		t.Fatalf("children: %+v", children)
	}
	if !json.Valid([]byte(env["MTT_TASK_JSON"])) {
		t.Fatalf("MTT_TASK_JSON not valid: %q", env["MTT_TASK_JSON"])
	}
}

// A task with NO children must still emit a valid empty array, not "null".
func TestTaskEnvBuilderNoChildren(t *testing.T) {
	solo := mtt.Task{ID: "t9", Type: "task", Status: "tbd"}
	env := taskEnvBuilder(envStubStore{tasks: []mtt.Task{solo}})(solo)
	if env["MTT_TASK_CHILDREN_JSON"] != "[]" {
		t.Fatalf("children json = %q, want []", env["MTT_TASK_CHILDREN_JSON"])
	}
}
