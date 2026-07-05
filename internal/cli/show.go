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
			id, err := mtt.NewTaskID(args[0])
			if err != nil {
				return err
			}
			task, err := store.Get(id)
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
			idx := core.NewIndex(tasks)
			_, err = fmt.Fprint(cmd.OutOrStdout(), formatTask(task, idx.Ancestors(task.ID), idx.Children(task.ID)))
			return err
		},
	}
}

// formatTask renders a task as a human-readable block. ancestors is the
// root-first parent chain (empty for a root task) and children the direct
// children (both computed by core.Index). The lineage line is the "you are here"
// path from the root down to and including the task itself (shown only when the
// task has ancestors); the children line lists direct children (shown only when
// present). The raw parent is not printed — it is the breadcrumb's tail.
func formatTask(t mtt.Task, ancestors, children []mtt.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s  [%s]\n", t.ID, t.Type, t.Status)
	if t.Title != "" {
		fmt.Fprintf(&b, "  title:    %s\n", t.Title)
	}
	if len(ancestors) > 0 {
		ids := make([]string, 0, len(ancestors)+1)
		for _, a := range ancestors {
			ids = append(ids, string(a.ID))
		}
		ids = append(ids, string(t.ID)) // the path ends at the task itself ("you are here")
		fmt.Fprintf(&b, "  lineage:  %s\n", strings.Join(ids, " › "))
	}
	if len(children) > 0 {
		ids := make([]string, len(children))
		for i, c := range children {
			ids[i] = string(c.ID)
		}
		fmt.Fprintf(&b, "  children: %d (%s)\n", len(children), strings.Join(ids, ", "))
	}
	fmt.Fprintf(&b, "  created:  %s\n", t.Created.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "  updated:  %s\n", t.Updated.UTC().Format(time.RFC3339))
	if t.Description != "" {
		fmt.Fprintf(&b, "\n  %s\n", t.Description)
	}
	return b.String()
}
