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
