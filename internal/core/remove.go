package core

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Remover is the delete-a-task usecase. By default it refuses to delete a task
// referenced by others (a child via parent, or a dependent via depends_on) so a
// delete never silently strands references; --force overrides, leaving the
// references dangling (tolerated: Ready is conservative, Index orphans->roots).
// Under --force it FORCES who+why (pre-flight) and writes an audit record BEFORE
// deleting (no destruction without a preceding record). now is injected for
// deterministic audit timestamps.
type Remover struct {
	store mtt.TaskStore
	audit mtt.AuditStore
	now   func() time.Time
	ev    *EventEmitter
}

// NewRemover wires the usecase with the audit port and an injected clock; ev
// fires the delete event per id (nil = none).
func NewRemover(store mtt.TaskStore, audit mtt.AuditStore, now func() time.Time, ev *EventEmitter) *Remover {
	return &Remover{store: store, audit: audit, now: now, ev: ev}
}

// RemoveResult is one task's outcome in a bulk delete.
type RemoveResult struct {
	ID  mtt.TaskID
	Err error // nil on success; wraps ErrNotFound / a load or referenced error
}

// Remove deletes a single id. Thin wrapper over RemoveMany([id]); it forwards the
// pre-flight error and, absent that, the per-id result error. The empty-slice check
// guards the [0] index on the pre-flight path.
func (r *Remover) Remove(id mtt.TaskID, force bool, by, why string, bl Backlinks, noRun bool) error {
	res, err := r.RemoveMany([]mtt.TaskID{id}, force, by, why, bl, noRun)
	if err != nil {
		return err
	}
	return res[0].Err
}

// RemoveMany deletes each id best-effort. The error return is the PRE-FLIGHT
// precondition failure (missing attribution under --force), returned before any
// deletion with a nil results slice; the CLI forwards it raw (exit 2). Per-id
// outcomes ride []RemoveResult. Existence is checked per id via store.Get; Index+
// DepGraph are built ONCE from a single List snapshot for the referenced-check
// (counting only referents OUTSIDE the id set, so deleting a subtree needs no
// --force). Under --force each id is audited BEFORE it is deleted.
func (r *Remover) RemoveMany(ids []mtt.TaskID, force bool, by, why string, bl Backlinks, noRun bool) ([]RemoveResult, error) {
	if force {
		if missing := missingAttributionFields(true, true, by, why); len(missing) > 0 {
			return nil, fmt.Errorf("%w: %s", ErrMissingAttribution, strings.Join(missing, ", "))
		}
	}
	if noRun && !force { // the force branch above already demanded who+why
		if err := (EventOptions{NoRun: true, By: by, Why: why}).Preflight(); err != nil {
			return nil, err
		}
	}

	ordered := dedupIDSlice(ids)
	set := make(map[mtt.TaskID]bool, len(ordered))
	for _, id := range ordered {
		set[id] = true
	}

	var idx Index
	var g DepGraph
	var snapErr error
	if !force {
		tasks, err := r.store.List()
		if err != nil {
			snapErr = fmt.Errorf("list tasks: %w", err)
		} else {
			idx = NewIndex(tasks)
			g = NewDepGraph(tasks)
		}
	}

	results := make([]RemoveResult, 0, len(ordered))
	for _, id := range ordered {
		results = append(results, RemoveResult{ID: id, Err: r.removeOne(id, force, by, why, noRun, set, idx, g, bl, snapErr)})
	}
	return results, nil
}

// removeOne deletes one id. Under --force it appends the audit record FIRST; only on
// a successful append does it delete (a failed append leaves the task — and the
// current pointer — intact). Without --force the subgraph referenced-check runs and
// no audit is written.
func (r *Remover) removeOne(id mtt.TaskID, force bool, by, why string, noRun bool, set map[mtt.TaskID]bool, idx Index, g DepGraph, bl Backlinks, snapErr error) error {
	task, err := r.store.Get(id)
	if err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return fmt.Errorf("task %q: %w", id, mtt.ErrNotFound)
		}
		return fmt.Errorf("load task %q: %w", id, err)
	}
	if !force {
		if snapErr != nil {
			return snapErr
		}
		if refs := externalReferencingIDs(idx, g, bl, id, set); len(refs) > 0 {
			return fmt.Errorf("task %q is referenced by %s; use --force to delete anyway",
				id, strings.Join(refs, ", "))
		}
		if err := r.store.Delete(id); err != nil {
			return err
		}
		// Per-entity delete event, fired INSIDE the loop (mutation→pipeline
		// adjacency — a batch-then-fire would make later pipelines see an
		// already-swept tree). Under --no-run this writes the skip record.
		return r.ev.TaskEvent(mtt.EventDelete, task, "rm", EventOptions{NoRun: noRun, By: by, Why: why})
	}
	action := "rm --force"
	if noRun {
		action = "rm --force --no-run"
	}
	entry := mtt.AuditEntry{At: r.now().UTC().Truncate(time.Second), Who: by, Why: why, Action: action, TaskID: id}
	if err := r.audit.Append(entry); err != nil {
		return fmt.Errorf("audit append for %q: %w", id, err)
	}
	if err := r.store.Delete(id); err != nil {
		return err
	}
	if noRun {
		return nil // the force record above already signed the bypass (pin iii) — no second record, no pipeline
	}
	return r.ev.TaskEvent(mtt.EventDelete, task, "rm", EventOptions{})
}

// externalReferencingIDs returns the ids referencing id — its children (via Index),
// its dependents (via DepGraph), and its incoming refs (via the cross-store
// Backlinks), deduped, in that order — EXCLUDING any TASK id in the deletion set
// (they are being deleted too — the subgraph-ignore). A note carrier is never in the
// set, so an incoming note ref always blocks; it is labelled "note:<slug>" to keep
// the "referenced by" message unambiguous. Structural sources are ordered by
// lessByRecency and refs by NewBacklinks, so the result is deterministic.
func externalReferencingIDs(idx Index, g DepGraph, bl Backlinks, id mtt.TaskID, set map[mtt.TaskID]bool) []string {
	seen := map[string]bool{}
	var out []string
	addTask := func(refs []mtt.Task) {
		for _, t := range refs {
			if set[t.ID] || seen[string(t.ID)] {
				continue
			}
			seen[string(t.ID)] = true
			out = append(out, string(t.ID))
		}
	}
	addTask(idx.Children(id))
	addTask(g.Dependents(id))
	for _, ref := range bl.To(mtt.RefTask, string(id)) {
		if ref.Carrier == mtt.RefNote {
			key := "note:" + ref.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, key)
			continue
		}
		if set[mtt.TaskID(ref.ID)] || seen[ref.ID] {
			continue
		}
		seen[ref.ID] = true
		out = append(out, ref.ID)
	}
	return out
}

// dedupIDSlice removes duplicate ids, keeping first-occurrence order. (Mirrors
// cli.dedupIDs; kept separate across the cli/core package boundary — no shared
// exported home warrants adding one for a 6-line helper.)
func dedupIDSlice(ids []mtt.TaskID) []mtt.TaskID {
	seen := make(map[mtt.TaskID]bool, len(ids))
	out := make([]mtt.TaskID, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
