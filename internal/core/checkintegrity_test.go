package core

import (
	"reflect"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestCheckIntegrity(t *testing.T) {
	tasks := []mtt.Task{
		{ID: "t1", Type: "task", Status: "tbd", Refs: []mtt.Ref{
			{Kind: mtt.RefNote, ID: "gone"},                 // dangling-ref (no such note)
			{Kind: mtt.RefURL, ID: "https://example.com/x"}, // unverified-ref (soft)
		}},
		{ID: "t2", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t99"}}, // dangling-dep
		{ID: "t3", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t4"}},  // cycle t3<->t4
		{ID: "t4", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t3"}},
		{ID: "t5", Type: "task", Status: "tbd"}, // clean
	}
	got := CheckIntegrity(tasks, nil, true)

	var kinds []IntegrityKind
	for _, f := range got {
		kinds = append(kinds, f.Kind)
	}
	// deterministic order: refs (CheckRefs order), then deps (task-slice order), then cycles
	want := []IntegrityKind{IntegrityDanglingRef, IntegrityUnverifiedRef, IntegrityDanglingDep, IntegrityCycle}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	// payloads
	if got[0].Ref == nil || got[0].Ref.Status != RefDangling {
		t.Fatalf("dangling-ref payload = %+v", got[0].Ref)
	}
	if got[1].Ref == nil || got[1].Ref.Status != RefUnverified {
		t.Fatalf("unverified-ref payload = %+v", got[1].Ref)
	}
	if got[2].Dep == nil || got[2].Dep.Task != "t2" || got[2].Dep.Missing != "t99" {
		t.Fatalf("dangling-dep payload = %+v", got[2].Dep)
	}
	if len(got[3].Cycle) == 0 {
		t.Fatalf("cycle payload empty")
	}
}

func TestCheckIntegrityDepOrder(t *testing.T) {
	// Multiple dangling depends_on must be reported in TASK-SLICE order, not by
	// recency: give the later slice entry the NEWER Created so a recency sort would
	// flip them — the findings must still read t1 then t2.
	tasks := []mtt.Task{
		{ID: "t1", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"gone1"},
			Created: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "t2", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"gone2"},
			Created: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	got := CheckIntegrity(tasks, nil, true)
	if len(got) != 2 {
		t.Fatalf("want 2 dangling-dep findings, got %d: %+v", len(got), got)
	}
	if got[0].Dep == nil || got[1].Dep == nil || got[0].Dep.Task != "t1" || got[1].Dep.Task != "t2" {
		t.Fatalf("dep order must follow the task slice (t1 then t2), got %+v", got)
	}
}

func TestCheckIntegrityClean(t *testing.T) {
	tasks := []mtt.Task{{ID: "t1", Type: "task", Status: "tbd"}}
	if got := CheckIntegrity(tasks, nil, true); len(got) != 0 {
		t.Fatalf("clean repo = %v, want empty", got)
	}
}
