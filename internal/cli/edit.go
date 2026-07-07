package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newEditCmd builds `mtt edit <id>`: edit a task's non-flow fields.
func newEditCmd() *cobra.Command {
	var title, desc, priority string
	cmd := &cobra.Command{
		Use:   "edit [<id>]",
		Short: "Edit a task's title, description, and/or priority (the current task when the id is omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var p core.EditParams
			if cmd.Flags().Changed("title") {
				p.Title = &title
			}
			if cmd.Flags().Changed("description") {
				p.Description = &desc
			}
			if cmd.Flags().Changed("priority") {
				pr, err := parsePriority(priority)
				if err != nil {
					return err
				}
				p.Priority = &pr
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			editor := core.NewEditor(yaml.NewTaskStore(root), time.Now)
			id, err := resolveTaskID(root, argOrEmpty(args))
			if err != nil {
				return err
			}
			task, err := editor.Edit(id, p)
			if err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return taskNotFound(id)
				}
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", task.ID)
			return err
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&desc, "description", "", "new description")
	cmd.Flags().StringVar(&priority, "priority", "", "new priority: high|medium|low (empty string clears it)")
	return cmd
}
