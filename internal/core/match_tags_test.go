package core

import (
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func taskWithTags(id mtt.TaskID, tags ...string) mtt.Task {
	return mtt.Task{ID: id, Type: "epic", Status: "tbd", Tags: tags}
}

func TestMatchTagsORWithin(t *testing.T) {
	task := taskWithTags("e1", "auth", "backend")
	// OR within the dimension: matches if it carries any filter tag.
	if !Match(task, ListFilter{Tags: []string{"auth", "frontend"}}, cfg()) {
		t.Fatal("want match on shared tag auth")
	}
	if Match(task, ListFilter{Tags: []string{"frontend", "infra"}}, cfg()) {
		t.Fatal("want no match: task has none of the filter tags")
	}
}

func TestMatchTagsEmptyFilterMatchesAll(t *testing.T) {
	if !Match(taskWithTags("e1"), ListFilter{}, cfg()) {
		t.Fatal("empty tag filter must match a tagless task")
	}
}

func TestMatchTagsUntaggedFailsNonEmptyFilter(t *testing.T) {
	if Match(taskWithTags("e1"), ListFilter{Tags: []string{"auth"}}, cfg()) {
		t.Fatal("a tagless task must fail a non-empty --tag filter")
	}
}

func TestMatchTagsANDAcrossDimensions(t *testing.T) {
	task := taskWithTags("e1", "auth")
	// AND across dimensions: right tag but wrong status -> no match.
	if Match(task, ListFilter{Tags: []string{"auth"}, Statuses: []mtt.StatusName{"doing"}}, cfg()) {
		t.Fatal("tag matches but status does not -> overall no match")
	}
	if !Match(task, ListFilter{Tags: []string{"auth"}, Statuses: []mtt.StatusName{"tbd"}}, cfg()) {
		t.Fatal("tag and status both match -> match")
	}
}
