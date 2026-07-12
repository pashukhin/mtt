package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// SortKey selects the list ordering. Ordering is always descending on the chosen
// timestamp (freshest first), tie-broken by ID for determinism.
type SortKey string

// The supported sort keys. An empty key defaults to SortCreated. SortPriority
// orders by Priority.Rank ascending (high first), tie-broken by recency.
const (
	SortCreated  SortKey = "created"
	SortUpdated  SortKey = "updated"
	SortPriority SortKey = "priority"
)

// ListFilter holds the list predicates and ordering. Empty slices/zero Parent
// match everything; within a field the values are OR-ed, across fields AND-ed.
// ExcludeTags is a negative filter: a task carrying ANY of its tags is rejected
// (empty → rejects nothing); it composes with Tags as AND, so on overlap exclude
// wins (a tag in both Tags and ExcludeTags rejects the task).
type ListFilter struct {
	Statuses    []mtt.StatusName
	Types       []mtt.TypeName
	Kinds       []mtt.StatusKind
	Priorities  []mtt.Priority
	Tags        []string
	ExcludeTags []string
	Parent      mtt.TaskID
	Sort        SortKey
}

// Match reports whether t satisfies f. Within a dimension the values are OR-ed;
// across dimensions AND-ed. cfg is consulted only for the Kinds dimension (to
// resolve t's status category via its type's flow); a task whose type or status
// is unknown to cfg fails a non-empty Kinds filter. Shared by Select and tree.
func Match(t mtt.Task, f ListFilter, cfg mtt.Config) bool {
	if !anyOrEmpty(f.Statuses, t.Status) || !anyOrEmpty(f.Types, t.Type) || !anyOrEmpty(f.Priorities, t.Priority) {
		return false
	}
	if !anyOrEmptyIntersect(f.Tags, t.Tags) {
		return false
	}
	if intersects(f.ExcludeTags, t.Tags) {
		return false
	}
	if f.Parent != "" && t.Parent != f.Parent {
		return false
	}
	if len(f.Kinds) > 0 && !matchesKind(t, f.Kinds, cfg) {
		return false
	}
	return true
}

func matchesKind(t mtt.Task, kinds []mtt.StatusKind, cfg mtt.Config) bool {
	k, ok := kindOf(t, cfg)
	if !ok {
		return false
	}
	for _, want := range kinds {
		if want == k {
			return true
		}
	}
	return false
}

// Select returns the tasks matching f in a deterministic order, without mutating
// the input. Order: the chosen timestamp descending, tie-broken by ID as an
// opaque string, so equal timestamps never reorder between runs. Select never
// interprets ID structure, so it stays provider-agnostic.
func Select(tasks []mtt.Task, f ListFilter, cfg mtt.Config) []mtt.Task {
	out := make([]mtt.Task, 0, len(tasks))
	for _, t := range tasks {
		if Match(t, f, cfg) {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if f.Sort == SortPriority {
			return lessByPriority(out[i], out[j])
		}
		return lessByRecency(out[i], out[j], f.Sort)
	})
	return out
}

// lessByPriority orders by Priority.Rank ascending (high first; unset/unknown rank
// as medium), tie-broken by the shared recency comparator (Created desc, ID) — so
// list --sort priority and roadmap agree on order.
func lessByPriority(a, b mtt.Task) bool {
	ra, rb := a.Priority.Rank(), b.Priority.Rank()
	if ra != rb {
		return ra < rb
	}
	return lessByRecency(a, b, SortCreated)
}

// anyOrEmpty reports whether values is empty (match everything) or contains v.
func anyOrEmpty[T comparable](values []T, v T) bool {
	if len(values) == 0 {
		return true
	}
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}

// anyOrEmptyIntersect reports whether filter is empty (match everything) or shares
// at least one value with have — OR within a slice-valued dimension (e.g. tags).
func anyOrEmptyIntersect[T comparable](filter, have []T) bool {
	if len(filter) == 0 {
		return true
	}
	return intersects(filter, have)
}

// intersects reports whether a and b share at least one element (empty a → false).
func intersects[T comparable](a, b []T) bool {
	set := make(map[T]bool, len(b))
	for _, v := range b {
		set[v] = true
	}
	for _, v := range a {
		if set[v] {
			return true
		}
	}
	return false
}
