package core

import (
	"sort"
	"strings"

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

// ListFilter holds the list predicates and ordering. Empty Statuses/Types match
// everything; within a field the values are OR-ed, across fields AND-ed.
type ListFilter struct {
	Statuses []string
	Types    []string
	Sort     SortKey
}

// Select returns the tasks matching f, in a deterministic order, without
// mutating the input. Primary order: the chosen timestamp (Created, or Updated
// when Sort==SortUpdated) descending. Tie-break: ID ascending as an opaque
// string compare — so equal timestamps never reorder between runs. Select never
// interprets ID structure, so it stays provider-agnostic.
func Select(tasks []mtt.Task, f ListFilter) []mtt.Task {
	out := make([]mtt.Task, 0, len(tasks))
	for _, t := range tasks {
		if anyOrEmpty(f.Statuses, t.Status) && anyOrEmpty(f.Types, t.Type) {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ti, tj := out[i].Created, out[j].Created
		if f.Sort == SortUpdated {
			ti, tj = out[i].Updated, out[j].Updated
		}
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return strings.Compare(out[i].ID, out[j].ID) < 0
	})
	return out
}

// anyOrEmpty reports whether values is empty (match everything) or contains v.
func anyOrEmpty(values []string, v string) bool {
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
