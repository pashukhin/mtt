package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Remover is the delete-a-task usecase. By default it refuses to delete a task
// referenced by others (a child via parent, or a dependent via depends_on) so a
// delete never silently strands references; --force overrides, leaving the
// references dangling (tolerated: Ready is conservative, Index orphans->roots).
// No clock — a delete records nothing.
type Remover struct {
	store mtt.TaskStore
}

// NewRemover wires the usecase.
func NewRemover(store mtt.TaskStore) *Remover { return &Remover{store: store} }

// RemoveResult is one task's outcome in a bulk delete.
type RemoveResult struct {
	ID  mtt.TaskID
	Err error // nil on success; wraps ErrNotFound / a load or referenced error
}

// Remove deletes a single id, returning the same error taxonomy as before. It is a
// thin wrapper over RemoveMany (set={id}, so every referent is external — identical
// reject semantics).
func (r *Remover) Remove(id mtt.TaskID, force bool) error {
	return r.RemoveMany([]mtt.TaskID{id}, force)[0].Err
}

// RemoveMany deletes each id best-effort. Existence is checked per id via store.Get
// (preserving the not-found / load-error wordings), while Index+DepGraph are built
// ONCE from a single List snapshot for the referenced-check. That check counts only
// referents OUTSIDE the id set, so deleting a subtree in one call needs no --force.
// force overrides an external referent.
func (r *Remover) RemoveMany(ids []mtt.TaskID, force bool) []RemoveResult {
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
		results = append(results, RemoveResult{ID: id, Err: r.removeOne(id, force, set, idx, g, snapErr)})
	}
	return results
}

// removeOne deletes one id, applying the subgraph-aware referenced-check.
func (r *Remover) removeOne(id mtt.TaskID, force bool, set map[mtt.TaskID]bool, idx Index, g DepGraph, snapErr error) error {
	if _, err := r.store.Get(id); err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return fmt.Errorf("task %q: %w", id, mtt.ErrNotFound)
		}
		return fmt.Errorf("load task %q: %w", id, err)
	}
	if !force {
		if snapErr != nil {
			return snapErr
		}
		if refs := externalReferencingIDs(idx, g, id, set); len(refs) > 0 {
			return fmt.Errorf("task %q is referenced by %s; use --force to delete anyway",
				id, strings.Join(refs, ", "))
		}
	}
	return r.store.Delete(id)
}

// externalReferencingIDs returns the ids referencing id — its children (via Index)
// and its dependents (via DepGraph), deduped, children first — EXCLUDING any id in
// the deletion set (they are being deleted too — the subgraph-ignore). Both sources
// are already ordered by lessByRecency, so the result is deterministic.
func externalReferencingIDs(idx Index, g DepGraph, id mtt.TaskID, set map[mtt.TaskID]bool) []string {
	seen := map[mtt.TaskID]bool{}
	var out []string
	add := func(refs []mtt.Task) {
		for _, t := range refs {
			if set[t.ID] || seen[t.ID] {
				continue
			}
			seen[t.ID] = true
			out = append(out, string(t.ID))
		}
	}
	add(idx.Children(id))
	add(g.Dependents(id))
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
