package core

import (
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func matchCfg() mtt.Config {
	return mtt.Config{Types: []mtt.Type{
		{Name: "task", Flow: mtt.Flow{Statuses: []mtt.Status{
			{Name: "tbd", Kind: mtt.KindInitial},
			{Name: "doing", Kind: mtt.KindActive},
			{Name: "done", Kind: mtt.KindTerminal},
		}}},
	}}
}

func TestMatchParent(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	child := mtt.Task{ID: "t1", Type: "task", Status: "tbd", Parent: "e1", Created: base}
	if !Match(child, ListFilter{Parent: "e1"}, mtt.Config{}) {
		t.Fatal("child of e1 should match Parent=e1")
	}
	if Match(child, ListFilter{Parent: "e2"}, mtt.Config{}) {
		t.Fatal("child of e1 must not match Parent=e2")
	}
}

func TestMatchKind(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	task := mtt.Task{ID: "t1", Type: "task", Status: "doing", Created: base}
	if !Match(task, ListFilter{Kinds: []mtt.StatusKind{mtt.KindActive}}, matchCfg()) {
		t.Fatal("doing is active — should match")
	}
	if Match(task, ListFilter{Kinds: []mtt.StatusKind{mtt.KindTerminal}}, matchCfg()) {
		t.Fatal("doing is not terminal — should not match")
	}
	// unknown type in cfg -> kind unresolved -> non-match
	ghost := mtt.Task{ID: "g1", Type: "ghost", Status: "doing", Created: base}
	if Match(ghost, ListFilter{Kinds: []mtt.StatusKind{mtt.KindActive}}, matchCfg()) {
		t.Fatal("unknown type must fail a kind filter")
	}
}

func TestMatchPriorityFilter(t *testing.T) {
	high := mtt.Task{ID: "t1", Type: "task", Status: "tbd", Priority: mtt.PriorityHigh}
	unset := mtt.Task{ID: "t2", Type: "task", Status: "tbd"}
	// A --priority high filter matches the stored value; an unset task does NOT
	// match a filter (filtering is on the authored label, not the ordering default).
	if !Match(high, ListFilter{Priorities: []mtt.Priority{mtt.PriorityHigh}}, mtt.Config{}) {
		t.Error("high task should match --priority high")
	}
	if Match(unset, ListFilter{Priorities: []mtt.Priority{mtt.PriorityHigh}}, mtt.Config{}) {
		t.Error("unset task should NOT match --priority high")
	}
	if Match(unset, ListFilter{Priorities: []mtt.Priority{mtt.PriorityMedium}}, mtt.Config{}) {
		t.Error("unset task should NOT match --priority medium (match the stored value, not the default)")
	}
	// No filter → everything matches, including unset.
	if !Match(unset, ListFilter{}, mtt.Config{}) {
		t.Error("unset task should match when no --priority filter is given")
	}
}

func TestMatchExcludeTags(t *testing.T) {
	task := mtt.Task{ID: "t1", Type: "task", Status: "tbd", Tags: []string{"backlog", "dx"}}
	// A task carrying an excluded tag is rejected.
	if Match(task, ListFilter{ExcludeTags: []string{"backlog"}}, mtt.Config{}) {
		t.Error("task with an excluded tag should NOT match")
	}
	// Excluding a tag the task lacks leaves it matching.
	if !Match(task, ListFilter{ExcludeTags: []string{"sec"}}, mtt.Config{}) {
		t.Error("task without the excluded tag should match")
	}
	// Empty exclude rejects nothing.
	if !Match(task, ListFilter{}, mtt.Config{}) {
		t.Error("no exclude filter should match")
	}
	// Include + exclude compose (AND); on overlap exclude wins.
	if Match(task, ListFilter{Tags: []string{"dx"}, ExcludeTags: []string{"backlog"}}, mtt.Config{}) {
		t.Error("exclude should win when a tag is in both include and exclude")
	}
}
