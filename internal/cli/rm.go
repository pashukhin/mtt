package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newRmCmd builds `mtt rm <id>`: hard-delete a task (distinct from cancel, a
// terminal status). Requires an explicit id — it does NOT resolve the current
// task (a destructive op takes an explicit target).
func newRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete a task (hard delete; use cancel for a terminal status)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			id, err := mtt.NewTaskID(args[0])
			if err != nil {
				return err
			}
			if err := core.NewRemover(yaml.NewTaskStore(root)).Remove(id, force); err != nil {
				return err
			}
			// Stale-current cleanup: if the deleted task was the current pointer,
			// clear it so no dangling pointer survives (CLI-level, like applyCurrent).
			current := yaml.NewCurrent(root)
			if cur, ok, cerr := current.Current(); cerr == nil && ok && cur == id {
				if err := current.ClearCurrent(); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", id)
			return err
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "delete even if the task is referenced (leaves dangling refs)")
	return cmd
}
