package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newTagsCmd builds `mtt tags`: the tag vocabulary with per-tag task counts, a
// pure derived read (core.TagCounts over the filtered set; no mutation). Default
// scope is OPEN tasks (initial+active kinds); --all counts every task; the list
// filters narrow the counted set.
func newTagsCmd() *cobra.Command {
	var (
		statuses    []string
		types       []string
		kinds       []string
		priorities  []string
		tags        []string
		excludeTags []string
		parent      string
		all         bool
	)
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "List the tag vocabulary with per-tag task counts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			kindVals, err := parseKinds(kinds)
			if err != nil {
				return err
			}
			prioVals, err := toPriorities(priorities)
			if err != nil {
				return err
			}
			tagVals, err := toTags(tags)
			if err != nil {
				return err
			}
			excludeTagVals, err := toTags(excludeTags)
			if err != nil {
				return err
			}
			// Default scope: open tasks (initial+active). A status-scoping flag
			// (--all, --kind, or --status) sets the scope explicitly and suppresses
			// the open default — so `mtt tags --status done` reaches terminal tasks
			// instead of silently ANDing to empty. Non-status filters (--type/
			// --priority/--tag/--exclude-tag/--parent) narrow WITHIN the scope.
			if !all && len(kindVals) == 0 && len(statuses) == 0 {
				kindVals = []mtt.StatusKind{mtt.KindInitial, mtt.KindActive}
			}
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
			selected := core.Select(tasks, core.ListFilter{
				Statuses: toStatusNames(statuses), Types: toTypeNames(types), Kinds: kindVals,
				Priorities: prioVals, Tags: tagVals, ExcludeTags: excludeTagVals, Parent: mtt.TaskID(parent),
			}, cfg)
			counts := core.TagCounts(selected)
			if jsonFlag(cmd) {
				views := make([]tagCountJSON, 0, len(counts))
				for _, c := range counts {
					views = append(views, tagCountJSON{Tag: c.Tag, Count: c.Count})
				}
				return writeJSON(cmd.OutOrStdout(), views)
			}
			return writeTagCounts(cmd.OutOrStdout(), counts)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "count every task, not just open (initial+active) ones")
	cmd.Flags().StringArrayVar(&statuses, "status", nil, "filter by status (repeatable)")
	cmd.Flags().StringArrayVar(&types, "type", nil, "filter by type (repeatable)")
	cmd.Flags().StringArrayVar(&kinds, "kind", nil, "filter by status category: initial|active|terminal (repeatable)")
	cmd.Flags().StringArrayVar(&priorities, "priority", nil, "filter by priority: high|medium|low (repeatable)")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "filter by tag (repeatable, comma-separated)")
	cmd.Flags().StringSliceVar(&excludeTags, "exclude-tag", nil, "exclude tasks carrying this tag (repeatable, comma-separated)")
	cmd.Flags().StringVar(&parent, "parent", "", "only direct children of this task id")
	return cmd
}

// tagCountJSON is one `mtt tags --json` element.
type tagCountJSON struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// writeTagCounts renders the count/tag rows (most-used first).
func writeTagCounts(w io.Writer, counts []core.TagCount) error {
	for _, c := range counts {
		if _, err := fmt.Fprintf(w, "%d  %s\n", c.Count, c.Tag); err != nil {
			return err
		}
	}
	return nil
}
