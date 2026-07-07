package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Remover is the delete-a-task usecase. By default it refuses to delete a task
// that other tasks reference (a child via parent, or a dependent via depends_on)
// so a delete never silently strands references; --force overrides, leaving the
// references dangling (which the system tolerates: Ready is conservative, Index
// surfaces orphans as roots). No clock — a delete records nothing.
type Remover struct {
	store mtt.TaskStore
}

// NewRemover wires the usecase.
func NewRemover(store mtt.TaskStore) *Remover { return &Remover{store: store} }

// Remove deletes id. A missing task yields an error wrapping mtt.ErrNotFound.
// Unless force, a task referenced by others is rejected with the referencing ids.
func (r *Remover) Remove(id mtt.TaskID, force bool) error {
	if _, err := r.store.Get(id); err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return fmt.Errorf("task %q: %w", id, mtt.ErrNotFound)
		}
		return fmt.Errorf("load task %q: %w", id, err)
	}
	if !force {
		tasks, err := r.store.List()
		if err != nil {
			return fmt.Errorf("list tasks: %w", err)
		}
		if refs := referencingIDs(tasks, id); len(refs) > 0 {
			return fmt.Errorf("task %q is referenced by %s; use --force to delete anyway",
				id, strings.Join(refs, ", "))
		}
	}
	return r.store.Delete(id)
}

// referencingIDs returns the ids that reference id — its children (parent == id)
// and its dependents (depends_on id) — deduped, children first (both sources are
// already ordered by lessByRecency, so the result is deterministic).
func referencingIDs(tasks []mtt.Task, id mtt.TaskID) []string {
	idx := NewIndex(tasks)
	g := NewDepGraph(tasks)
	seen := map[mtt.TaskID]bool{}
	var out []string
	add := func(refs []mtt.Task) {
		for _, t := range refs {
			if !seen[t.ID] {
				seen[t.ID] = true
				out = append(out, string(t.ID))
			}
		}
	}
	add(idx.Children(id))
	add(g.Dependents(id))
	return out
}
