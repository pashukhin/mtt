package core

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// TransitionOptions carry the non-flow inputs to a transition: the roles seam
// (Role), the subject-identity seam (By) and the durable reason (Why), all
// recorded into history; NoRun to bypass the edge's command gate; and the
// project's required-attribution policy (RequireWho/RequireWhy) checked before
// the gate (fail fast; NoRun does not bypass it).
type TransitionOptions struct {
	Role       string
	By         string
	Why        string
	NoRun      bool
	RequireWho bool
	RequireWhy bool
}

// Transitioner applies a SINGLE flow edge: validate id's current status → to
// against the type's transitions, run that edge's commands as a gate (unless
// NoRun), append a history entry, and persist via TaskStore.Update. No new port —
// history rides the Task.History field (like depends_on in s005).
type Transitioner struct {
	store  mtt.TaskStore
	cfg    mtt.Config
	runner Runner
	now    func() time.Time
}

// NewTransitioner wires the usecase; now is injected for deterministic tests.
func NewTransitioner(store mtt.TaskStore, cfg mtt.Config, runner Runner, now func() time.Time) *Transitioner {
	return &Transitioner{store: store, cfg: cfg, runner: runner, now: now}
}

// Transition moves id across one edge to `to`. Errors: task not found; unknown
// type (config drift); ErrInvalidTransition (no such edge); ErrBlocked (a gate
// command exited non-zero or the runner failed). On a block the task is not
// changed and no history is written.
func (tr *Transitioner) Transition(id mtt.TaskID, to mtt.StatusName, opts TransitionOptions) (mtt.Task, error) {
	t, err := tr.store.Get(id)
	if err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return mtt.Task{}, fmt.Errorf("task %q not found", id)
		}
		return mtt.Task{}, fmt.Errorf("load task %q: %w", id, err)
	}
	typ, ok := tr.cfg.TypeByName(t.Type)
	if !ok {
		return mtt.Task{}, fmt.Errorf("unknown type %q for task %q", t.Type, id)
	}
	edge, ok := findTransition(typ, t.Status, to)
	if !ok {
		return mtt.Task{}, fmt.Errorf("%w: %s cannot move %s → %s (allowed from %s: %s)",
			ErrInvalidTransition, id, t.Status, to, t.Status, strings.Join(allowedTargets(typ, t.Status), ", "))
	}
	if missing := missingAttribution(opts); len(missing) > 0 {
		return mtt.Task{}, fmt.Errorf("%w: %s", ErrMissingAttribution, strings.Join(missing, ", "))
	}
	var checks []mtt.Check
	if !opts.NoRun {
		checks, err = tr.runner.Run(edge.Commands)
		if err != nil {
			return mtt.Task{}, fmt.Errorf("%w: %v", ErrBlocked, err)
		}
		if c, failed := firstFailure(checks); failed {
			return mtt.Task{}, fmt.Errorf("%w: command %q exited %d", ErrBlocked, c.Cmd, c.Exit)
		}
	}
	from := t.Status
	ts := tr.now().UTC().Truncate(time.Second)
	t.Status = to
	t.History = append(t.History, mtt.HistoryEntry{
		At: ts, By: opts.By, Role: opts.Role, Why: opts.Why, From: from, To: to, Checks: checks,
	})
	t.Updated = ts
	return tr.store.Update(t)
}

// findTransition returns the edge from → to in typ's flow, if any.
func findTransition(typ mtt.Type, from, to mtt.StatusName) (mtt.Transition, bool) {
	for _, e := range typ.Transitions {
		if e.From == from && e.To == to {
			return e, true
		}
	}
	return mtt.Transition{}, false
}

// allowedTargets lists the statuses reachable from `from` in one edge (for a
// helpful invalid-transition message).
func allowedTargets(typ mtt.Type, from mtt.StatusName) []string {
	var out []string
	for _, e := range typ.Transitions {
		if e.From == from {
			out = append(out, string(e.To))
		}
	}
	return out
}

// missingAttribution reports which required attribution fields (who/why) are
// absent, aggregated so the caller can fix them all in one shot.
func missingAttribution(opts TransitionOptions) []string {
	var missing []string
	if opts.RequireWho && opts.By == "" {
		missing = append(missing, "who")
	}
	if opts.RequireWhy && opts.Why == "" {
		missing = append(missing, "why")
	}
	return missing
}

// firstFailure returns the first non-zero check, if any.
func firstFailure(checks []mtt.Check) (mtt.Check, bool) {
	for _, c := range checks {
		if c.Exit != 0 {
			return c, true
		}
	}
	return mtt.Check{}, false
}
