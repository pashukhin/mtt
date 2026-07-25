package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newRoadmapCmd builds `mtt roadmap [--json]`: the non-terminal tasks in a
// dependency-respecting, priority-weighted execution order, each annotated with
// ready / blocked_by. A pure derived read (core.Roadmap); no mutation.
func newRoadmapCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "roadmap",
		Short: "Show tasks in dependency+priority execution order (ready / blocked_by)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			cfg, _, err := yaml.Load(root)
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			entries := core.Roadmap(tasks, cfg)
			if jsonFlag(cmd) {
				exists := existsSet(tasks)
				views := make([]roadmapJSON, 0, len(entries))
				for _, e := range entries {
					views = append(views, toRoadmapJSON(e, exists))
				}
				return writeJSON(cmd.OutOrStdout(), views)
			}
			return writeRoadmap(cmd.OutOrStdout(), entries, tasks)
		},
	}
}

// roadmapJSON is the machine view of one roadmap entry. priority is the STORED
// value ("" when unset — honest; consumers apply their own default), so it is NOT
// omitempty. blocked_by (depends_on) and contains (children) are always
// (possibly empty) arrays, never null. blocked_by_missing is the dangling subset
// of blocked_by (absent from the task set) — omitted when none, non-null when
// present, so a consumer sees a broken dependency without breaking blocked_by.
type roadmapJSON struct {
	ID               string   `json:"id"`
	Title            string   `json:"title,omitempty"`
	Status           string   `json:"status"`
	Priority         string   `json:"priority"`
	Ready            bool     `json:"ready"`
	BlockedBy        []string `json:"blocked_by"`
	BlockedByMissing []string `json:"blocked_by_missing,omitempty"`
	Contains         []string `json:"contains"`
}

func toRoadmapJSON(e core.RoadmapEntry, exists map[mtt.TaskID]bool) roadmapJSON {
	return roadmapJSON{
		ID: string(e.Task.ID), Title: e.Task.Title, Status: string(e.Task.Status),
		Priority: string(e.Task.Priority), Ready: e.Ready,
		BlockedBy: idStrings(e.BlockedBy), BlockedByMissing: missingIDs(e.BlockedBy, exists),
		Contains: idStrings(e.Contains),
	}
}

// existsSet is the task-id existence set built from the full snapshot (DRY with
// show.go's dependsEntries derivation), used to mark dangling blockers.
func existsSet(tasks []mtt.Task) map[mtt.TaskID]bool {
	set := make(map[mtt.TaskID]bool, len(tasks))
	for _, t := range tasks {
		set[t.ID] = true
	}
	return set
}

// missingIDs returns the subset of ids absent from exists (nil when none, so the
// blocked_by_missing field omitempty-drops rather than emitting []).
func missingIDs(ids []mtt.TaskID, exists map[mtt.TaskID]bool) []string {
	var out []string
	for _, id := range ids {
		if !exists[id] {
			out = append(out, string(id))
		}
	}
	return out
}

// markMissing renders ids for the human blocked-by line, appending " (missing)"
// to any id absent from exists (consistent with dep list / show).
func markMissing(ids []mtt.TaskID, exists map[mtt.TaskID]bool) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		s := string(id)
		if !exists[id] {
			s += " (missing)"
		}
		out = append(out, s)
	}
	return out
}

// idStrings maps typed ids to a non-null string slice ([] when empty — the house
// JSON rule), shared by the roadmap JSON view's array fields.
func idStrings(ids []mtt.TaskID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, string(id))
	}
	return out
}

// writeRoadmap renders the entries as a numbered list: "N. <id>  [<priority>]
// (<status>)  <title>" (the [..] label omitted when unset, the title when empty),
// with "  ↳ blocked by: a, b (missing)" under a depends_on-blocked entry (a
// dangling blocker marked "(missing)", consistent with dep list / show) and
// "  ↳ contains: a, b" under a parent (its non-terminal children). Priority is
// the stored value, never faked as medium (the ORDERING treats unset as medium,
// the LABEL never fabricates one).
func writeRoadmap(w io.Writer, entries []core.RoadmapEntry, tasks []mtt.Task) error {
	exists := existsSet(tasks)
	var b strings.Builder
	for i, e := range entries {
		fmt.Fprintf(&b, "%d. %s", i+1, e.Task.ID)
		if e.Task.Priority != "" {
			fmt.Fprintf(&b, "  [%s]", e.Task.Priority)
		}
		fmt.Fprintf(&b, "  (%s)", e.Task.Status)
		if e.Task.Title != "" {
			fmt.Fprintf(&b, "  %s", e.Task.Title)
		}
		b.WriteString("\n")
		if len(e.BlockedBy) > 0 {
			fmt.Fprintf(&b, "  ↳ blocked by: %s\n", strings.Join(markMissing(e.BlockedBy, exists), ", "))
		}
		if len(e.Contains) > 0 {
			fmt.Fprintf(&b, "  ↳ contains: %s\n", strings.Join(idStrings(e.Contains), ", "))
		}
	}
	_, err := fmt.Fprint(w, b.String())
	return err
}
