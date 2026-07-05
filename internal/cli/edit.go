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
	var title, desc string
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a task's title and/or description",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("provide exactly one task id (example: mtt edit e1 --title \"…\")")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var p core.EditParams
			if cmd.Flags().Changed("title") {
				p.Title = &title
			}
			if cmd.Flags().Changed("description") {
				p.Description = &desc
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			editor := core.NewEditor(yaml.NewTaskStore(root), time.Now)
			id, err := mtt.NewTaskID(args[0])
			if err != nil {
				return err
			}
			task, err := editor.Edit(id, p)
			if err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return fmt.Errorf("task %q not found", args[0])
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
	return cmd
}
