package core

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

type fakeStore struct {
	got   mtt.Task
	retID mtt.TaskID
	byID  map[mtt.TaskID]mtt.Task
}

func (f *fakeStore) Create(t mtt.Task) (mtt.Task, error) {
	f.got = t
	t.ID = f.retID
	return t, nil
}
func (f *fakeStore) Get(id mtt.TaskID) (mtt.Task, error) {
	if t, ok := f.byID[id]; ok {
		return t, nil
	}
	return mtt.Task{}, mtt.ErrNotFound
}
func (f *fakeStore) List() ([]mtt.Task, error) { return nil, nil }
func (f *fakeStore) Update(t mtt.Task) (mtt.Task, error) {
	f.got = t
	return t, nil
}
func (f *fakeStore) Delete(id mtt.TaskID) error {
	if _, ok := f.byID[id]; !ok {
		return mtt.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

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
		{Name: "task", Parents: []mtt.TypeName{"epic"}, Default: true, Flow: flow()},
		{Name: "subtask", Parents: []mtt.TypeName{"task"}, Flow: flow()},
	}}
}

func fixed() time.Time { return time.Date(2026, 7, 4, 9, 20, 30, 500, time.UTC) }

func TestAddStampsPriority(t *testing.T) {
	fs := &fakeStore{retID: "e1"}
	got, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "x", TypeName: "epic", Priority: mtt.PriorityHigh})
	if err != nil {
		t.Fatal(err)
	}
	if got.Priority != mtt.PriorityHigh {
		t.Fatalf("Priority = %q, want high", got.Priority)
	}
	// Default: unset (not medium).
	plain, err := NewAdder(&fakeStore{retID: "e2"}, cfg(), fixed).Add(AddParams{Title: "y", TypeName: "epic"})
	if err != nil {
		t.Fatal(err)
	}
	if plain.Priority != "" {
		t.Fatalf("default Priority = %q, want unset", plain.Priority)
	}
}

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

func TestAddUnderParentOK(t *testing.T) {
	fs := &fakeStore{retID: "t1", byID: map[mtt.TaskID]mtt.Task{"e1": {ID: "e1", Type: "epic"}}}
	got, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "child", TypeName: "task", Parent: "e1"})
	if err != nil {
		t.Fatalf("valid parent should succeed: %v", err)
	}
	if got.ID != "t1" || fs.got.Parent != "e1" {
		t.Fatalf("parent not set: id=%q parent=%q", got.ID, fs.got.Parent)
	}
}

func TestAddParentMissing(t *testing.T) {
	fs := &fakeStore{retID: "t1", byID: map[mtt.TaskID]mtt.Task{}}
	_, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "x", TypeName: "task", Parent: "e9"})
	if err == nil || !errors.Is(err, mtt.ErrNotFound) || !strings.Contains(err.Error(), `parent "e9"`) {
		t.Fatalf("want parent-not-found wrapping ErrNotFound, got %v", err)
	}
}

func TestAddParentWrongType(t *testing.T) {
	// subtask.parents = [task]; placing it under an epic must fail.
	fs := &fakeStore{retID: "s1", byID: map[mtt.TaskID]mtt.Task{"e1": {ID: "e1", Type: "epic"}}}
	_, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "x", TypeName: "subtask", Parent: "e1"})
	if err == nil || !strings.Contains(err.Error(), "cannot be placed under") {
		t.Fatalf("want placement error, got %v", err)
	}
}

func TestAddRootTypeRejectsParent(t *testing.T) {
	// epic is a root type; giving it a parent must fail.
	fs := &fakeStore{retID: "e2", byID: map[mtt.TaskID]mtt.Task{"e1": {ID: "e1", Type: "epic"}}}
	_, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "x", TypeName: "epic", Parent: "e1"})
	if err == nil || !strings.Contains(err.Error(), "cannot be placed under") {
		t.Fatalf("want placement error for root+parent, got %v", err)
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

func TestAddWithDependsOn(t *testing.T) {
	fs := &fakeStore{retID: "t2", byID: map[mtt.TaskID]mtt.Task{"t1": {ID: "t1", Type: "epic"}}}
	got, err := NewAdder(fs, cfg(), fixed).Add(AddParams{
		Title: "B", TypeName: "epic", DependsOn: []mtt.TaskID{"t1"},
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(got.DependsOn) != 1 || got.DependsOn[0] != "t1" {
		t.Fatalf("DependsOn = %v; want [t1]", got.DependsOn)
	}
	if len(fs.got.DependsOn) != 1 {
		t.Fatalf("DependsOn not persisted via Create: %+v", fs.got)
	}
}

func TestAddDependsOnMissingTarget(t *testing.T) {
	fs := &fakeStore{retID: "t2", byID: map[mtt.TaskID]mtt.Task{}}
	_, err := NewAdder(fs, cfg(), fixed).Add(AddParams{
		Title: "B", TypeName: "epic", DependsOn: []mtt.TaskID{"t99"},
	})
	if err == nil || !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("err = %v; want ErrNotFound", err)
	}
	if fs.got.Title != "" {
		t.Fatal("task must not be created when a depends-on target is missing")
	}
}

func TestAddDependsOnDedup(t *testing.T) {
	fs := &fakeStore{retID: "t2", byID: map[mtt.TaskID]mtt.Task{"t1": {ID: "t1", Type: "epic"}}}
	got, err := NewAdder(fs, cfg(), fixed).Add(AddParams{
		Title: "B", TypeName: "epic", DependsOn: []mtt.TaskID{"t1", "t1"},
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(got.DependsOn) != 1 {
		t.Fatalf("DependsOn = %v; want deduped [t1]", got.DependsOn)
	}
}

func TestAdderCanonicalizesRefs(t *testing.T) {
	fs := &fakeStore{retID: "e1"}
	got, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "x", TypeName: "epic", Refs: []mtt.Ref{
		{Kind: mtt.RefTask, ID: "t2"}, {Kind: mtt.RefNote, ID: "a"}, {Kind: mtt.RefTask, ID: "t2"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Refs) != 2 { // deduped by (kind,id)
		t.Fatalf("refs: %+v", got.Refs)
	}
	if got.Refs[0].Kind != mtt.RefNote { // sorted (note < task)
		t.Fatalf("sort: %+v", got.Refs)
	}
}
