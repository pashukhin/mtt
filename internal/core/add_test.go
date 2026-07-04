package core

import (
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

type fakeStore struct {
	got   mtt.Task
	retID string
}

func (f *fakeStore) Create(t mtt.Task) (mtt.Task, error) {
	f.got = t
	t.ID = f.retID
	return t, nil
}
func (f *fakeStore) Get(string) (mtt.Task, error) { return mtt.Task{}, mtt.ErrNotFound }

func flow() mtt.Flow {
	return mtt.Flow{
		Statuses: []mtt.Status{
			{Name: "tbd", Kind: mtt.KindInitial},
			{Name: "doing", Kind: mtt.KindActive},
			{Name: "done", Kind: mtt.KindTerminal},
		},
		Transitions: []mtt.Transition{{From: "tbd", To: "doing"}, {From: "doing", To: "done"}},
	}
}

func cfg() mtt.Config {
	return mtt.Config{Types: []mtt.Type{
		{Name: "epic", Flow: flow()},
		{Name: "task", Parents: []string{"epic"}, Default: true, Flow: flow()},
	}}
}

func fixed() time.Time { return time.Date(2026, 7, 4, 9, 20, 30, 500, time.UTC) }

func TestAddRootExplicitType(t *testing.T) {
	fs := &fakeStore{retID: "e1"}
	got, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "build auth", TypeName: "epic"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "e1" {
		t.Fatalf("id = %q, want e1", got.ID)
	}
	if fs.got.Type != "epic" || fs.got.Status != "tbd" || fs.got.Title != "build auth" {
		t.Fatalf("logical task wrong: %+v", fs.got)
	}
	if !fs.got.Created.Equal(fixed().Truncate(time.Second)) || !fs.got.Updated.Equal(fs.got.Created) {
		t.Fatalf("timestamps: created=%v updated=%v", fs.got.Created, fs.got.Updated)
	}
	if fs.got.Parent != "" {
		t.Fatalf("root task must have empty parent, got %q", fs.got.Parent)
	}
}

func TestAddDefaultTypeNeedsParent(t *testing.T) {
	_, err := NewAdder(&fakeStore{retID: "t1"}, cfg(), fixed).Add(AddParams{Title: "x"})
	if err == nil || !strings.Contains(err.Error(), "requires a parent") {
		t.Fatalf("want 'requires a parent', got %v", err)
	}
}

func TestAddNoParentException(t *testing.T) {
	fs := &fakeStore{retID: "t1"}
	got, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "orphan", NoParent: true})
	if err != nil || got.ID != "t1" || fs.got.Type != "task" {
		t.Fatalf("no-parent create failed: id=%q type=%q err=%v", got.ID, fs.got.Type, err)
	}
}

func TestAddUnknownType(t *testing.T) {
	_, err := NewAdder(&fakeStore{}, cfg(), fixed).Add(AddParams{Title: "x", TypeName: "ghost"})
	if err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("want 'unknown type', got %v", err)
	}
}

func TestAddNeedsTitleOrDescription(t *testing.T) {
	_, err := NewAdder(&fakeStore{}, cfg(), fixed).Add(AddParams{TypeName: "epic"})
	if err == nil || !strings.Contains(err.Error(), "title or a description") {
		t.Fatalf("want title-or-description error, got %v", err)
	}
	// description-only is allowed
	if _, err := NewAdder(&fakeStore{retID: "e1"}, cfg(), fixed).Add(AddParams{TypeName: "epic", Description: "figure it out"}); err != nil {
		t.Fatalf("description-only should be allowed: %v", err)
	}
}
