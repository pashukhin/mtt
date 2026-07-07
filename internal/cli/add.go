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

// newAddCmd builds `mtt add [title]`: create a task.
func newAddCmd() *cobra.Command {
	var (
		typeName  string
		parent    string
		noParent  bool
		desc      string
		priority  string
		dependsOn []string
	)
	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Create a task",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return errors.New("too many arguments: wrap a multi-word title in quotes (example: mtt add \"fix login\")")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
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
			prio, err := parsePriority(priority)
			if err != nil {
				return err
			}
			depIDs := make([]mtt.TaskID, len(dependsOn))
			for i, d := range dependsOn {
				depIDs[i] = mtt.TaskID(d)
			}
			adder := core.NewAdder(yaml.NewTaskStore(root), cfg, time.Now)
			task, err := adder.Add(core.AddParams{Title: title, TypeName: mtt.TypeName(typeName), Parent: mtt.TaskID(parent), NoParent: noParent, Description: desc, Priority: prio, DependsOn: depIDs})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", task.ID)
			return err
		},
	}
	cmd.Flags().StringVar(&typeName, "type", "", "task type (default: the config's default type)")
	cmd.Flags().StringVar(&parent, "parent", "", "place under an existing parent task (by id)")
	cmd.Flags().BoolVar(&noParent, "no-parent", false, "create a parent-requiring type at top level (conscious exception)")
	cmd.Flags().StringVar(&desc, "description", "", "task description")
	cmd.Flags().StringVar(&priority, "priority", "", "task priority: high|medium|low (default: unset)")
	cmd.Flags().StringSliceVar(&dependsOn, "depends-on", nil, "ids this task depends on (repeatable, comma-separated)")
	cmd.MarkFlagsMutuallyExclusive("parent", "no-parent")
	return cmd
}
