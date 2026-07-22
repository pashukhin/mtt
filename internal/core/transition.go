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
// changed and no history is written. ErrPostAction (t21) is the SINGLE case where a
// non-nil error carries a VALID persisted task: the move happened (pre-gate passed,
// status written) but the post phase failed — callers keep the task and surface the
// error (exit 5). --no-run skips both the pre-gate and the post phase.
func (tr *Transitioner) Transition(id mtt.TaskID, to mtt.StatusName, opts TransitionOptions) (mtt.Task, error) {
	t, err := tr.store.Get(id)
	if err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return mtt.Task{}, fmt.Errorf("task %q: %w", id, mtt.ErrNotFound)
		}
		return mtt.Task{}, fmt.Errorf("load task %q: %w", id, err)
	}
	typ, ok := tr.cfg.TypeByName(t.Type)
	if !ok {
		return mtt.Task{}, fmt.Errorf("unknown type %q for task %q", t.Type, id)
	}
	edge, ok := findTransition(typ, t.Status, to)
	if !ok {
		if targets := allowedTargets(typ, t.Status); len(targets) > 0 {
			return mtt.Task{}, fmt.Errorf("%w: %s cannot move %s → %s (allowed from %s: %s)",
				ErrInvalidTransition, id, t.Status, to, t.Status, strings.Join(targets, ", "))
		}
		return mtt.Task{}, fmt.Errorf("%w: %s cannot move %s → %s (no moves out of %s — it is terminal)",
			ErrInvalidTransition, id, t.Status, to, t.Status)
	}
	effWho := opts.RequireWho || edge.Require.Who || opts.NoRun
	effWhy := opts.RequireWhy || edge.Require.Why || opts.NoRun
	if missing := missingAttributionFields(effWho, effWhy, opts.By, opts.Why); len(missing) > 0 {
		return mtt.Task{}, fmt.Errorf("%w: %s", ErrMissingAttribution, strings.Join(missing, ", "))
	}
	from := t.Status
	var checks []mtt.Check
	if !opts.NoRun {
		expanded, eerr := expandCommands(edge.Commands, cmdContext{
			ID:   string(t.ID),
			Type: string(t.Type),
			From: string(from),
			To:   string(to),
		})
		if eerr != nil {
			return mtt.Task{}, fmt.Errorf("expand commands for %s (%s->%s): %w", id, from, to, eerr)
		}
		checks, err = tr.runner.Run(expanded)
		if err != nil {
			// operational failure: the failing command is the last recorded check
			// (Runner CONTRACT); if none was recorded, len(checks)-1 == -1 → no comp.
			return tr.block(expanded, len(checks)-1, err.Error())
		}
		if i, c, failed := firstFailure(checks); failed {
			return tr.block(expanded, i, fmt.Sprintf("command %q exited %d", c.Cmd, c.Exit))
		}
	}
	ts := tr.now().UTC().Truncate(time.Second)
	t.Status = to
	t.History = append(t.History, mtt.HistoryEntry{
		At: ts, By: opts.By, Role: opts.Role, Why: opts.Why, From: from, To: to, Checks: checks,
	})
	t.Updated = ts
	updated, uerr := tr.store.Update(t)
	if uerr != nil {
		return mtt.Task{}, uerr
	}
	// POST phase (t21): after persist, gated by !NoRun. A post failure returns the
	// PERSISTED task with ErrPostAction — the move is kept (finalization only). This
	// is the single case where Transition returns a valid task with a non-nil error.
	if opts.NoRun || len(edge.Post) == 0 {
		return updated, nil
	}
	expanded, eerr := expandCommands(edge.Post, cmdContext{
		ID: string(t.ID), Type: string(t.Type), From: string(from), To: string(to),
	})
	if eerr != nil {
		return updated, &PostActionError{
			Remaining: runsOf(edge.Post), // raw templates — expansion is what failed
			Cause:     fmt.Sprintf("expand post for %s (%s->%s): %v", id, from, to, eerr),
		}
	}
	pchecks, rerr := tr.runner.Run(expanded)
	if rerr != nil {
		i := len(pchecks) - 1 // failing command is last (Runner CONTRACT); guard the empty case
		if i < 0 {
			i = 0
		}
		return updated, &PostActionError{Remaining: runsOf(expanded[i:]), Cause: rerr.Error()}
	}
	if i, c, failed := firstFailure(pchecks); failed {
		return updated, &PostActionError{
			Remaining: runsOf(expanded[i:]),
			Cause:     fmt.Sprintf("command %q exited %d", c.Cmd, c.Exit),
		}
	}
	return updated, nil
}

// runsOf extracts each command's .Run string — the copy-paste-ready form the CLI
// prints as a post-failure recovery hint.
func runsOf(cmds []mtt.Command) []string {
	out := make([]string, len(cmds))
	for i, c := range cmds {
		out[i] = c.Run
	}
	return out
}

// findTransition returns the edge from → to in typ's flow, if any. Delegates to
// the pure pkg/mtt primitive (single source of truth for edge lookup).
func findTransition(typ mtt.Type, from, to mtt.StatusName) (mtt.Transition, bool) {
	return typ.FindTransition(from, to)
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

// missingAttributionFields reports which required attribution fields (who/why) are
// absent, aggregated so the caller fixes them all at once. The single home for the
// who/why check, shared by the transition path and the rm --force pre-flight.
func missingAttributionFields(reqWho, reqWhy bool, by, why string) []string {
	var missing []string
	if reqWho && by == "" {
		missing = append(missing, "who")
	}
	if reqWhy && why == "" {
		missing = append(missing, "why")
	}
	return missing
}

// firstFailure returns the index and Check of the first non-zero exit (incl. an
// operational -1), and whether one was found.
func firstFailure(checks []mtt.Check) (int, mtt.Check, bool) {
	for i, c := range checks {
		if c.Exit != 0 {
			return i, c, true
		}
	}
	return 0, mtt.Check{}, false
}

// block runs best-effort compensation over the commands that succeeded before
// failIdx (their rollbacks, in reverse) and returns ErrBlocked with a summary.
// The task is left unchanged and no history is written (s006 invariant): block
// returns before any tr.store.Update.
func (tr *Transitioner) block(expanded []mtt.Command, failIdx int, cause string) (mtt.Task, error) {
	if rbs := rollbacksBefore(expanded, failIdx); len(rbs) > 0 {
		comp := tr.runner.Compensate(rbs)
		return mtt.Task{}, fmt.Errorf("%w: %s; %s", ErrBlocked, cause, compSummary(comp))
	}
	return mtt.Task{}, fmt.Errorf("%w: %s", ErrBlocked, cause)
}

// rollbacksBefore returns the rollbacks of expanded[:failIdx] in reverse order
// (compensation order) — never including the failing command itself. Safe for
// failIdx <= 0 (returns nil).
func rollbacksBefore(expanded []mtt.Command, failIdx int) []mtt.Command {
	var rbs []mtt.Command
	for i := failIdx - 1; i >= 0; i-- {
		if rb := expanded[i].Rollback; rb != nil {
			rbs = append(rbs, *rb)
		}
	}
	return rbs
}

// compSummary reports how many compensators ran and how many failed (Exit != 0).
func compSummary(checks []mtt.Check) string {
	noun := "commands"
	if len(checks) == 1 {
		noun = "command"
	}
	failed := 0
	for _, c := range checks {
		if c.Exit != 0 {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Sprintf("compensated %d %s (%d failed)", len(checks), noun, failed)
	}
	return fmt.Sprintf("compensated %d %s", len(checks), noun)
}
