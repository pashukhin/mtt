package core

import (
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func tsk(id mtt.TaskID, typ mtt.TypeName, status string, created time.Time) mtt.Task {
	return mtt.Task{ID: id, Type: typ, Status: status, Created: created, Updated: created}
}

func TestSelectFilters(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		tsk("e1", "epic", "tbd", base),
		tsk("t1", "task", "tbd", base),
		tsk("t2", "task", "done", base),
	}
	if got := Select(tasks, ListFilter{}, mtt.Config{}); len(got) != 3 {
		t.Fatalf("no filter len = %d, want 3", len(got))
	}
	if got := Select(tasks, ListFilter{Types: []mtt.TypeName{"task"}}, mtt.Config{}); len(got) != 2 {
		t.Fatalf("type=task len = %d, want 2", len(got))
	}
	got := Select(tasks, ListFilter{Types: []mtt.TypeName{"task"}, Statuses: []string{"done"}}, mtt.Config{})
	if len(got) != 1 || got[0].ID != "t2" {
		t.Fatalf("task AND done -> %+v", got)
	}
	if got := Select(tasks, ListFilter{Statuses: []string{"ghost"}}, mtt.Config{}); len(got) != 0 {
		t.Fatalf("no match len = %d, want 0", len(got))
	}
}

func TestSelectOrderCreatedDesc(t *testing.T) {
	older := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	got := Select([]mtt.Task{tsk("e1", "epic", "tbd", older), tsk("e2", "epic", "tbd", newer)}, ListFilter{}, mtt.Config{})
	if got[0].ID != "e2" || got[1].ID != "e1" {
		t.Fatalf("created desc = %s,%s; want e2,e1", got[0].ID, got[1].ID)
	}
}

func TestSelectTieBreakByIDString(t *testing.T) {
	same := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	got := Select([]mtt.Task{
		tsk("t2", "task", "tbd", same),
		tsk("t1", "task", "tbd", same),
		tsk("e1", "epic", "tbd", same),
	}, ListFilter{}, mtt.Config{})
	want := []mtt.TaskID{"e1", "t1", "t2"} // equal Created -> opaque ID string ascending
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("tie-break[%d] = %s, want %s", i, got[i].ID, id)
		}
	}
}

func TestSelectSortUpdated(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	a := mtt.Task{ID: "e1", Type: "epic", Status: "tbd", Created: base.Add(2 * time.Hour), Updated: base}
	b := mtt.Task{ID: "e2", Type: "epic", Status: "tbd", Created: base, Updated: base.Add(2 * time.Hour)}
	got := Select([]mtt.Task{a, b}, ListFilter{Sort: SortUpdated}, mtt.Config{})
	if got[0].ID != "e2" {
		t.Fatalf("sort=updated top = %s, want e2", got[0].ID)
	}
}

func TestSelectDoesNotMutateInput(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{tsk("e1", "epic", "tbd", base), tsk("e2", "epic", "tbd", base.Add(time.Hour))}
	_ = Select(tasks, ListFilter{}, mtt.Config{})
	if tasks[0].ID != "e1" || tasks[1].ID != "e2" {
		t.Fatalf("input reordered: %s,%s", tasks[0].ID, tasks[1].ID)
	}
}
