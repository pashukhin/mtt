package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newShowCmd builds `mtt show <id>`: display a task.
func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a task",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("provide exactly one task id (example: mtt show e1)")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			task, err := yaml.NewTaskStore(root).Get(args[0])
			if err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return fmt.Errorf("task %q not found", args[0])
				}
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), formatTask(task))
			return err
		},
	}
}

// formatTask renders a task as a human-readable block. The parent line shows the
// raw parent ID; the computed lineage ("you are here") arrives in session 004.
func formatTask(t mtt.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s  [%s]\n", t.ID, t.Type, t.Status)
	if t.Title != "" {
		fmt.Fprintf(&b, "  title:    %s\n", t.Title)
	}
	if t.Parent != "" {
		fmt.Fprintf(&b, "  parent:   %s\n", t.Parent)
	}
	fmt.Fprintf(&b, "  created:  %s\n", t.Created.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "  updated:  %s\n", t.Updated.UTC().Format(time.RFC3339))
	if t.Description != "" {
		fmt.Fprintf(&b, "\n  %s\n", t.Description)
	}
	return b.String()
}
