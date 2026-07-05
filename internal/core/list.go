package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// SortKey selects the list ordering. Ordering is always descending on the chosen
// timestamp (freshest first), tie-broken by ID for determinism.
type SortKey string

// The supported sort keys. An empty key defaults to SortCreated.
const (
	SortCreated SortKey = "created"
	SortUpdated SortKey = "updated"
)

// ListFilter holds the list predicates and ordering. Empty slices/zero Parent
// match everything; within a field the values are OR-ed, across fields AND-ed.
type ListFilter struct {
	Statuses []string
	Types    []mtt.TypeName
	Kinds    []mtt.StatusKind
	Parent   mtt.TaskID
	Sort     SortKey
}

// Match reports whether t satisfies f. Within a dimension the values are OR-ed;
// across dimensions AND-ed. cfg is consulted only for the Kinds dimension (to
// resolve t's status category via its type's flow); a task whose type or status
// is unknown to cfg fails a non-empty Kinds filter. Shared by Select and tree.
func Match(t mtt.Task, f ListFilter, cfg mtt.Config) bool {
	if !anyOrEmpty(f.Statuses, t.Status) || !anyOrEmpty(f.Types, t.Type) {
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
	typ, ok := cfg.TypeByName(t.Type)
	if !ok {
		return false
	}
	k, ok := typ.StatusKind(t.Status)
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
		return lessByRecency(out[i], out[j], f.Sort)
	})
	return out
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
