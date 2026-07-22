package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newReadyCmd builds `mtt ready`: list actionable tasks (non-terminal, all
// blockers terminal). Accepts the list filters.
func newReadyCmd() *cobra.Command {
	var (
		statuses    []string
		types       []string
		kinds       []string
		tags        []string
		excludeTags []string
		parent      string
		idsOut      bool
	)
	cmd := &cobra.Command{
		Use:   "ready",
		Short: "List actionable tasks (no open blockers)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			kindVals, err := parseKinds(kinds)
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
			filter := core.ListFilter{
				Statuses: toStatusNames(statuses), Types: toTypeNames(types),
				Kinds: kindVals, Tags: tagVals, ExcludeTags: excludeTagVals, Parent: mtt.TaskID(parent),
			}
			selected := core.Select(core.Ready(tasks, cfg), filter, cfg)
			if idsOut {
				if jsonFlag(cmd) {
					return fmt.Errorf("--ids and --json are mutually exclusive")
				}
				return writeIDs(cmd.OutOrStdout(), idsOf(selected))
			}
			if jsonFlag(cmd) {
				views := make([]taskJSON, 0, len(selected))
				for _, t := range selected {
					views = append(views, toTaskJSON(t))
				}
				return writeJSON(cmd.OutOrStdout(), views)
			}
			return writeList(cmd.OutOrStdout(), selected)
		},
	}
	cmd.Flags().StringArrayVar(&statuses, "status", nil, "filter by status (repeatable)")
	cmd.Flags().StringArrayVar(&types, "type", nil, "filter by type (repeatable)")
	cmd.Flags().StringArrayVar(&kinds, "kind", nil, "filter by status category: initial|active|terminal (repeatable)")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "filter by tag (repeatable, comma-separated)")
	cmd.Flags().StringSliceVar(&excludeTags, "exclude-tag", nil, "exclude tasks carrying this tag (repeatable, comma-separated)")
	cmd.Flags().StringVar(&parent, "parent", "", "only direct children of this task id")
	cmd.Flags().BoolVar(&idsOut, "ids", false, "print only task ids, one per line (for pipelines)")
	return cmd
}

// toStatusNames / toTypeNames convert CLI string slices to typed identities.
func toStatusNames(ss []string) []mtt.StatusName {
	out := make([]mtt.StatusName, len(ss))
	for i, s := range ss {
		out[i] = mtt.StatusName(s)
	}
	return out
}

func toTypeNames(ss []string) []mtt.TypeName {
	out := make([]mtt.TypeName, len(ss))
	for i, s := range ss {
		out[i] = mtt.TypeName(s)
	}
	return out
}
