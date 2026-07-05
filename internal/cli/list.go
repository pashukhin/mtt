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

// newListCmd builds `mtt list`: list tasks with filters and a stable order.
func newListCmd() *cobra.Command {
	var (
		statuses []string
		types    []string
		kinds    []string
		parent   string
		sortKey  string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch sortKey {
			case "", string(core.SortCreated), string(core.SortUpdated):
			default:
				return fmt.Errorf("invalid --sort %q: want created|updated", sortKey)
			}
			kindVals, err := parseKinds(kinds)
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
			typeNames := make([]mtt.TypeName, len(types))
			for i, s := range types {
				typeNames[i] = mtt.TypeName(s)
			}
			selected := core.Select(tasks, core.ListFilter{
				Statuses: statuses, Types: typeNames, Kinds: kindVals, Parent: mtt.TaskID(parent), Sort: core.SortKey(sortKey),
			}, cfg)
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
	cmd.Flags().StringVar(&sortKey, "sort", "", "sort order: created|updated (default created)")
	cmd.Flags().StringArrayVar(&kinds, "kind", nil, "filter by status category: initial|active|terminal (repeatable)")
	cmd.Flags().StringVar(&parent, "parent", "", "only direct children of this task id")
	return cmd
}

// writeList renders tasks one per line: "<id>  <type>  [<status>]  <title>"
// (the title is omitted when empty).
func writeList(w io.Writer, tasks []mtt.Task) error {
	var b strings.Builder
	for _, t := range tasks {
		b.WriteString(taskLine(t))
		b.WriteString("\n")
	}
	_, err := fmt.Fprint(w, b.String())
	return err
}
