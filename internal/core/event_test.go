package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// eventCfg is a minimal config with one task type and the given event hooks.
func eventCfg(events mtt.Events) mtt.Config {
	cfg := flowCfg(nil, nil)
	cfg.Events = events
	return cfg
}

func taskHook(kind mtt.EventKind, runs ...string) mtt.Events {
	var ev mtt.Events
	hook := mtt.EventHook{Post: strCmds(runs)}
	switch kind {
	case mtt.EventCreate:
		ev.Task.Create = hook
	case mtt.EventUpdate:
		ev.Task.Update = hook
	case mtt.EventDelete:
		ev.Task.Delete = hook
	}
	return ev
}

func TestTaskEventRunsExpandedPipeline(t *testing.T) {
	runner := &fakeRunner{}
	e := NewEventEmitter(eventCfg(taskHook(mtt.EventUpdate, "echo {{.ID}} {{.Type}} {{.Event}}")), runner, &fakeAudit{}, testClock)
	err := e.TaskEvent(mtt.EventUpdate, tbdTask("t1"), "edit", EventOptions{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(runner.gotCmds) != 1 || runner.gotCmds[0].Run != "echo t1 task update" {
		t.Fatalf("pipeline = %+v, want [echo t1 task update]", runner.gotCmds)
	}
}

func TestTaskEventNoHookNoRun(t *testing.T) {
	runner := &fakeRunner{}
	e := NewEventEmitter(eventCfg(mtt.Events{}), runner, &fakeAudit{}, testClock)
	if err := e.TaskEvent(mtt.EventUpdate, tbdTask("t1"), "edit", EventOptions{}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if runner.called {
		t.Fatal("runner must not be called without a hook")
	}
}

func TestNilEmitterIsInert(t *testing.T) {
	var e *EventEmitter
	if err := e.TaskEvent(mtt.EventCreate, tbdTask("t1"), "add", EventOptions{}); err != nil {
		t.Fatalf("nil emitter TaskEvent = %v", err)
	}
	if err := e.NoteEvent(mtt.EventCreate, mtt.Note{Slug: "s"}, "note add", EventOptions{}); err != nil {
		t.Fatalf("nil emitter NoteEvent = %v", err)
	}
}

func TestTaskEventTypeDriftIsFinalizationFailure(t *testing.T) {
	// The c15-class payload: a poisoned on-disk type must NEVER reach the shell.
	runner := &fakeRunner{}
	e := NewEventEmitter(eventCfg(taskHook(mtt.EventUpdate, "echo {{.Type}}")), runner, &fakeAudit{}, testClock)
	task := tbdTask("t1")
	task.Type = mtt.TypeName(`x"; touch pwned; "`)
	err := e.TaskEvent(mtt.EventUpdate, task, "edit", EventOptions{})
	var pe *PostActionError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PostActionError, got %v", err)
	}
	if !strings.Contains(pe.Cause, "not in config") {
		t.Fatalf("cause = %q", pe.Cause)
	}
	if len(pe.Remaining) != 1 || pe.Remaining[0] != "echo {{.Type}}" {
		t.Fatalf("remaining = %v", pe.Remaining)
	}
	if runner.called {
		t.Fatal("SECURITY: a drifted/poisoned type must never reach the runner")
	}
}

func TestTaskEventFromToIsTemplateError(t *testing.T) {
	runner := &fakeRunner{}
	e := NewEventEmitter(eventCfg(taskHook(mtt.EventUpdate, "echo {{.From}}")), runner, &fakeAudit{}, testClock)
	err := e.TaskEvent(mtt.EventUpdate, tbdTask("t1"), "edit", EventOptions{})
	var pe *PostActionError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PostActionError (post-persist expansion), got %v", err)
	}
	if runner.called {
		t.Fatal("runner must not run on a template error")
	}
}

func TestTaskEventCommandFailure(t *testing.T) {
	runner := &fakeRunner{failSubstr: "boom"}
	e := NewEventEmitter(eventCfg(taskHook(mtt.EventDelete, "echo ok", "boom-two", "echo never")), runner, &fakeAudit{}, testClock)
	err := e.TaskEvent(mtt.EventDelete, tbdTask("t1"), "rm", EventOptions{})
	var pe *PostActionError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PostActionError, got %v", err)
	}
	want := []string{"boom-two", "echo never"}
	if len(pe.Remaining) != 2 || pe.Remaining[0] != want[0] || pe.Remaining[1] != want[1] {
		t.Fatalf("remaining = %v, want %v", pe.Remaining, want)
	}
}

func TestNoRunSkipsPipelineAndWritesAuditRecord(t *testing.T) {
	for name, events := range map[string]mtt.Events{
		"with hook":    taskHook(mtt.EventUpdate, "echo hi"),
		"without hook": {},
	} {
		t.Run(name, func(t *testing.T) {
			runner := &fakeRunner{}
			audit := &fakeAudit{}
			e := NewEventEmitter(eventCfg(events), runner, audit, testClock)
			opts := EventOptions{NoRun: true, By: "me", Why: "because"}
			if err := e.TaskEvent(mtt.EventUpdate, tbdTask("t1"), "edit", opts); err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if runner.called {
				t.Fatal("--no-run must skip the pipeline")
			}
			if len(audit.entries) != 1 {
				t.Fatalf("audit entries = %d, want 1 (pin i: record whenever the flag is passed)", len(audit.entries))
			}
			got := audit.entries[0]
			if got.Who != "me" || got.Why != "because" || got.Action != "edit --no-run" || got.TaskID != "t1" {
				t.Fatalf("record = %+v", got)
			}
		})
	}
}

func TestNoRunAuditAppendFailure(t *testing.T) {
	audit := &fakeAudit{failOnID: "t1"}
	e := NewEventEmitter(eventCfg(mtt.Events{}), &fakeRunner{}, audit, testClock)
	err := e.TaskEvent(mtt.EventUpdate, tbdTask("t1"), "edit", EventOptions{NoRun: true, By: "a", Why: "b"})
	var pe *PostActionError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PostActionError, got %v", err)
	}
	if len(pe.Remaining) != 0 {
		t.Fatalf("remaining = %v, want empty (no user-runnable re-append)", pe.Remaining)
	}
	if !strings.Contains(pe.Cause, "the audit record was NOT written") {
		t.Fatalf("cause = %q", pe.Cause)
	}
}

func TestEventOptionsPreflight(t *testing.T) {
	err := (EventOptions{NoRun: true}).Preflight()
	if !errors.Is(err, ErrMissingAttribution) || !strings.Contains(err.Error(), "who") || !strings.Contains(err.Error(), "why") {
		t.Fatalf("bare NoRun preflight = %v", err)
	}
	if err := (EventOptions{NoRun: true, By: "a", Why: "b"}).Preflight(); err != nil {
		t.Fatalf("signed NoRun preflight = %v", err)
	}
	if err := (EventOptions{}).Preflight(); err != nil {
		t.Fatalf("no-bypass preflight = %v", err)
	}
}

func TestNoteEventContext(t *testing.T) {
	var ev mtt.Events
	ev.Note.Create = mtt.EventHook{Post: strCmds([]string{"echo {{.Slug}} {{.Event}}"})}
	runner := &fakeRunner{}
	e := NewEventEmitter(eventCfg(ev), runner, &fakeAudit{}, testClock)
	if err := e.NoteEvent(mtt.EventCreate, mtt.Note{Slug: "my-note"}, "note add", EventOptions{}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(runner.gotCmds) != 1 || runner.gotCmds[0].Run != "echo my-note create" {
		t.Fatalf("pipeline = %+v", runner.gotCmds)
	}

	ev.Note.Create = mtt.EventHook{Post: strCmds([]string{"echo {{.ID}}"})} // task-only field in a note hook
	e2 := NewEventEmitter(eventCfg(ev), &fakeRunner{}, &fakeAudit{}, testClock)
	err := e2.NoteEvent(mtt.EventCreate, mtt.Note{Slug: "my-note"}, "note add", EventOptions{})
	var pe *PostActionError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PostActionError on {{.ID}} in a note hook, got %v", err)
	}
}
