package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
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
			store := yaml.NewTaskStore(root)
			task, err := store.Get(args[0])
			if err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return fmt.Errorf("task %q not found", args[0])
				}
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
			}
			tasks, err := store.List()
			if err != nil {
				return err
			}
			lineage := core.NewIndex(tasks).Ancestors(task.ID)
			_, err = fmt.Fprint(cmd.OutOrStdout(), formatTask(task, lineage))
			return err
		},
	}
}

// formatTask renders a task as a human-readable block. ancestors is the
// root-first parent chain (empty for a root task); it prints a "lineage" line.
func formatTask(t mtt.Task, ancestors []mtt.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s  [%s]\n", t.ID, t.Type, t.Status)
	if t.Title != "" {
		fmt.Fprintf(&b, "  title:    %s\n", t.Title)
	}
	if len(ancestors) > 0 {
		ids := make([]string, len(ancestors))
		for i, a := range ancestors {
			ids[i] = a.ID
		}
		fmt.Fprintf(&b, "  lineage:  %s\n", strings.Join(ids, " › "))
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
