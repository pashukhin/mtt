package core

import (
	"sort"
	"strings"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// lessByRecency reports whether a should sort before b: the chosen timestamp
// descending (freshest first), tie-broken by ID as an opaque string. Shared by
// Select and the Index sibling order so both agree (never parses ID structure).
func lessByRecency(a, b mtt.Task, key SortKey) bool {
	ta, tb := a.Created, b.Created
	if key == SortUpdated {
		ta, tb = a.Updated, b.Updated
	}
	if !ta.Equal(tb) {
		return ta.After(tb)
	}
	return strings.Compare(a.ID, b.ID) < 0
}

// Index is a derived, read-only view of the parent→children hierarchy over a set
// of tasks. It is built once from a task slice (a pure value: no store, no clock)
// and is not part of the pkg/mtt contract — the resolved graph is derived.
// Children are computed (the inverse of Parent), never stored.
type Index struct {
	byID     map[string]mtt.Task
	children map[string][]mtt.Task // keyed by parent ID; roots live under key ""
}

// NewIndex builds the hierarchy index. A task with an empty Parent, or a Parent
// that does not resolve to a present task (an orphan), is treated as a root.
// Sibling buckets are ordered by lessByRecency (Created desc, ID tiebreak) so
// tree order matches Select.
func NewIndex(tasks []mtt.Task) Index {
	x := Index{
		byID:     make(map[string]mtt.Task, len(tasks)),
		children: make(map[string][]mtt.Task),
	}
	for _, t := range tasks {
		x.byID[t.ID] = t
	}
	for _, t := range tasks {
		key := t.Parent
		if key != "" {
			if _, ok := x.byID[key]; !ok {
				key = "" // orphan → root
			}
		}
		x.children[key] = append(x.children[key], t)
	}
	for k := range x.children {
		bucket := x.children[k]
		sort.SliceStable(bucket, func(i, j int) bool {
			return lessByRecency(bucket[i], bucket[j], SortCreated)
		})
	}
	return x
}

// Get returns the task with id, or false when absent.
func (x Index) Get(id string) (mtt.Task, bool) {
	t, ok := x.byID[id]
	return t, ok
}

// Roots returns the top-level tasks (no parent, or a dangling parent), in sibling order.
func (x Index) Roots() []mtt.Task { return x.children[""] }

// Children returns the direct children of id in sibling order (nil when none).
func (x Index) Children(id string) []mtt.Task { return x.children[id] }

// Ancestors returns id's parent chain from the outermost root down to the
// immediate parent (a breadcrumb, excluding id itself). Cycle-safe: a repeated
// id or a missing parent stops the walk.
func (x Index) Ancestors(id string) []mtt.Task {
	seen := map[string]bool{id: true}
	var chain []mtt.Task
	cur, ok := x.byID[id]
	for ok && cur.Parent != "" && !seen[cur.Parent] {
		seen[cur.Parent] = true
		parent, found := x.byID[cur.Parent]
		if !found {
			break
		}
		chain = append(chain, parent)
		cur = parent
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}
