package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newUseCmd builds `mtt use`: set (`use <id>`), show (`use`), or clear
// (`use --clear`) the current-task pointer — git-checkout-for-tasks.
func newUseCmd() *cobra.Command {
	var clear bool
	cmd := &cobra.Command{
		Use:   "use [<id>]",
		Short: "Set, show, or clear the current task (the working-context pointer)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			current := yaml.NewCurrent(root)
			switch {
			case clear:
				if len(args) > 0 {
					return errors.New("--clear takes no id")
				}
				if err := current.ClearCurrent(); err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "current cleared")
				return err
			case len(args) == 1:
				id, err := mtt.NewTaskID(args[0])
				if err != nil {
					return err
				}
				task, err := yaml.NewTaskStore(root).Get(id)
				if err != nil {
					if errors.Is(err, mtt.ErrNotFound) {
						return fmt.Errorf("task %q not found", args[0])
					}
					return err
				}
				if err := current.SetCurrent(id); err != nil {
					return err
				}
				if jsonFlag(cmd) {
					return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "current: %s\n", id)
				return err
			default:
				id, ok, err := current.Current()
				if err != nil {
					return err
				}
				if !ok {
					_, err = fmt.Fprintln(cmd.OutOrStdout(), "no current task")
					return err
				}
				task, err := yaml.NewTaskStore(root).Get(id)
				if err != nil {
					if errors.Is(err, mtt.ErrNotFound) {
						return fmt.Errorf("current task %q no longer exists; run `mtt use <id>` or `mtt use --clear`", id)
					}
					return err
				}
				if jsonFlag(cmd) {
					return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), taskLine(task))
				return err
			}
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "clear the current task")
	return cmd
}
