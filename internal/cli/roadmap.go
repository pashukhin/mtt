package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
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
				views := make([]roadmapJSON, 0, len(entries))
				for _, e := range entries {
					views = append(views, toRoadmapJSON(e))
				}
				return writeJSON(cmd.OutOrStdout(), views)
			}
			return writeRoadmap(cmd.OutOrStdout(), entries)
		},
	}
}

// roadmapJSON is the machine view of one roadmap entry. priority is the STORED
// value ("" when unset — honest; consumers apply their own default), so it is NOT
// omitempty. blocked_by is always a (possibly empty) array, never null.
type roadmapJSON struct {
	ID        string   `json:"id"`
	Title     string   `json:"title,omitempty"`
	Status    string   `json:"status"`
	Priority  string   `json:"priority"`
	Ready     bool     `json:"ready"`
	BlockedBy []string `json:"blocked_by"`
}

func toRoadmapJSON(e core.RoadmapEntry) roadmapJSON {
	blocked := make([]string, 0, len(e.BlockedBy))
	for _, id := range e.BlockedBy {
		blocked = append(blocked, string(id))
	}
	return roadmapJSON{
		ID: string(e.Task.ID), Title: e.Task.Title, Status: string(e.Task.Status),
		Priority: string(e.Task.Priority), Ready: e.Ready, BlockedBy: blocked,
	}
}

// writeRoadmap renders the entries as a numbered list: "N. <id>  [<priority>]
// (<status>)  <title>" (the [..] label omitted when unset, the title when empty),
// with "  ↳ blocked by: a, b" under a blocked entry. Priority is the stored value,
// never faked as medium (the ORDERING treats unset as medium, the LABEL never
// fabricates one).
func writeRoadmap(w io.Writer, entries []core.RoadmapEntry) error {
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
			ids := make([]string, len(e.BlockedBy))
			for j, id := range e.BlockedBy {
				ids[j] = string(id)
			}
			fmt.Fprintf(&b, "  ↳ blocked by: %s\n", strings.Join(ids, ", "))
		}
	}
	_, err := fmt.Fprint(w, b.String())
	return err
}
