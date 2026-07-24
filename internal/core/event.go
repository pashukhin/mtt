package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// EventOptions carry one mutation's bypass + attribution into its lifecycle
// event: NoRun skips the event pipeline (t5 discipline: forces By+Why — the
// usecase calls Preflight BEFORE persisting), By/Why sign the skip record.
type EventOptions struct {
	NoRun bool
	By    string
	Why   string
}

// Preflight validates the bypass attribution BEFORE anything persists: a
// --no-run bypass without who+why is ErrMissingAttribution (exit 2) and the
// mutation must not happen. Every mutating usecase calls it first.
func (o EventOptions) Preflight() error {
	if !o.NoRun {
		return nil
	}
	if missing := missingAttributionFields(true, true, o.By, o.Why); len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrMissingAttribution, strings.Join(missing, ", "))
	}
	return nil
}

// EventEmitter runs the config-authored lifecycle-event pipelines (post-only,
// t66) after a successful mutation. Fired by the mutating USECASES — never the
// store (a flow move must not double-fire) and never Transitioner (an edge has
// its own post). A nil emitter is inert, so tests and event-less wiring pass nil.
type EventEmitter struct {
	cfg    mtt.Config
	runner Runner
	audit  mtt.AuditStore
	now    func() time.Time
}

// NewEventEmitter wires the emitter; audit receives the --no-run skip records.
func NewEventEmitter(cfg mtt.Config, runner Runner, audit mtt.AuditStore, now func() time.Time) *EventEmitter {
	return &EventEmitter{cfg: cfg, runner: runner, audit: audit, now: now}
}

// TaskEvent fires the task hook for kind after t was persisted (or deleted).
// The mutation is already durable, so every failure here — template, command,
// type drift, audit append — is a FINALIZATION failure: a *PostActionError the
// caller returns beside the persisted result (CLI: exit 5, "the change is
// already saved"). action is the invoked command's name ("add", "tag rm", …)
// for the skip record. Under opts.NoRun the pipeline is skipped and the audit
// record is written instead — whether or not a hook is configured (one rule).
func (e *EventEmitter) TaskEvent(kind mtt.EventKind, t mtt.Task, action string, opts EventOptions) error {
	if e == nil {
		return nil
	}
	if opts.NoRun {
		return e.skipRecord(action, string(t.ID), opts)
	}
	hook := e.cfg.Events.Task.Hook(kind)
	if len(hook.Post) == 0 {
		return nil
	}
	typ, ok := e.cfg.TypeByName(t.Type)
	if !ok {
		// The c15-class guard (spec §4): the on-disk type: is validated only
		// as non-empty — never expand it. Membership in the trusted config
		// vocabulary is the shape-safety test; the config's own name is what
		// {{.Type}} renders.
		return &PostActionError{
			Remaining: runsOf(hook.Post),
			Cause:     fmt.Sprintf("task type %q not in config — event pipeline not run", t.Type),
		}
	}
	return e.run(hook.Post, taskEventContext{ID: string(t.ID), Type: string(typ.Name), Event: string(kind)})
}

// NoteEvent is TaskEvent's note-store sibling ({{.Slug}}/{{.Event}} context;
// the slug is structurally validated at every adapter boundary, so it is
// shape-safe by construction).
func (e *EventEmitter) NoteEvent(kind mtt.EventKind, n mtt.Note, action string, opts EventOptions) error {
	if e == nil {
		return nil
	}
	if opts.NoRun {
		return e.skipRecord(action, string(n.Slug), opts)
	}
	hook := e.cfg.Events.Note.Hook(kind)
	if len(hook.Post) == 0 {
		return nil
	}
	return e.run(hook.Post, noteEventContext{Slug: string(n.Slug), Event: string(kind)})
}

// run expands and executes one event pipeline. Expansion happens AFTER the
// persist (for create the ID does not exist earlier — spec §4), so a template
// error is a finalization failure too, never a lost mutation.
func (e *EventEmitter) run(post []mtt.Command, data any) error {
	expanded, err := expandCommands(post, data)
	if err != nil {
		return &PostActionError{Remaining: runsOf(post), Cause: fmt.Sprintf("expand event post: %v", err)}
	}
	checks, rerr := e.runner.Run(expanded)
	if rerr != nil {
		i := len(checks) - 1 // failing command is last (Runner CONTRACT)
		if i < 0 {
			i = 0
		}
		return &PostActionError{Remaining: runsOf(expanded[i:]), Cause: rerr.Error()}
	}
	if i, c, failed := firstFailure(checks); failed {
		return &PostActionError{
			Remaining: runsOf(expanded[i:]),
			Cause:     fmt.Sprintf("command %q exited %d", c.Cmd, c.Exit),
		}
	}
	return nil
}

// skipRecord signs a --no-run bypass into the audit log (t5: no bypass without
// a trail), written at the moment the skipped pipeline would have fired. A
// failed append is a finalization failure with NO recovery commands (there is
// no user-runnable re-append) — the CLI renders the message only.
func (e *EventEmitter) skipRecord(action, id string, opts EventOptions) error {
	entry := mtt.AuditEntry{
		At: e.now().UTC().Truncate(time.Second), Who: opts.By, Why: opts.Why,
		Action: action + " --no-run", TaskID: mtt.TaskID(id),
	}
	if err := e.audit.Append(entry); err != nil {
		return &PostActionError{
			Cause: fmt.Sprintf("audit append for %q: %v — the change is already saved; the audit record was NOT written", id, err),
		}
	}
	return nil
}
