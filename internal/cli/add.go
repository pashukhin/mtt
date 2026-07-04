package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
)

// newAddCmd builds `mtt add [title]`: create a task.
func newAddCmd() *cobra.Command {
	var (
		typeName string
		noParent bool
		desc     string
	)
	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Create a task",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			root, err := yaml.FindRoot(cwd)
			if err != nil {
				return err
			}
			cfg, _, err := yaml.Load(root)
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}
			title := ""
			if len(args) == 1 {
				title = args[0]
			}
			adder := core.NewAdder(yaml.NewTaskStore(root), cfg, time.Now)
			task, err := adder.Add(core.AddParams{Title: title, TypeName: typeName, NoParent: noParent, Description: desc})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", task.ID)
			return err
		},
	}
	cmd.Flags().StringVar(&typeName, "type", "", "task type (default: the config's default type)")
	cmd.Flags().BoolVar(&noParent, "no-parent", false, "create a parent-requiring type at top level (conscious exception)")
	cmd.Flags().StringVar(&desc, "description", "", "task description")
	return cmd
}
