package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newRmCmd builds `mtt rm [<id>...] [-]`: hard-delete tasks (distinct from cancel).
// A single explicit id keeps today's surface (removed <id>, exit 4 on not-found);
// multiple ids / "-" / --filter select a set for a best-effort bulk delete.
func newRmCmd() *cobra.Command {
	var force, dryRun bool
	cmd := &cobra.Command{
		Use:   "rm [<id>...] [-]",
		Short: "Delete tasks (hard delete; use cancel for a terminal status)",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			if len(args) == 1 && args[0] != "-" && !filterActive(cmd) {
				return runRmSingle(cmd, root, args[0], force, dryRun)
			}
			ids, err := selectTaskIDs(cmd, args, true)
			if err != nil {
				return err
			}
			if dryRun {
				return previewBulk(cmd, ids)
			}
			results := core.NewRemover(yaml.NewTaskStore(root)).RemoveMany(ids, force)
			items := make([]bulkItem, 0, len(results))
			for _, r := range results {
				items = append(items, bulkItem{id: r.ID, err: r.Err})
				if r.Err == nil {
					_ = clearCurrentIfMatches(root, r.ID) // best-effort in bulk: the task is already reported removed
				}
			}
			return reportBulk(cmd, items, "removed")
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "delete even if referenced (leaves dangling refs)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the affected tasks without deleting")
	addSelectorFilterFlags(cmd)
	return cmd
}

// runRmSingle is the back-compat single-id path: exact `removed <id>` output and
// exit 4 on not-found (via core.Remover.Remove's ErrNotFound wrap).
func runRmSingle(cmd *cobra.Command, root, idArg string, force, dryRun bool) error {
	id, err := mtt.NewTaskID(idArg)
	if err != nil {
		return err
	}
	if dryRun {
		return previewBulk(cmd, []mtt.TaskID{id})
	}
	if err := core.NewRemover(yaml.NewTaskStore(root)).Remove(id, force); err != nil {
		return err
	}
	if err := clearCurrentIfMatches(root, id); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", id)
	return err
}

// clearCurrentIfMatches clears the current-task pointer when it names id (a deleted
// task must not leave a dangling current). A read error is ignored (mirrors the
// pre-s008.9 single-rm behaviour); a clear error is returned. Bulk rm calls this
// best-effort (`_ =`) since the task is already reported removed.
func clearCurrentIfMatches(root string, id mtt.TaskID) error {
	current := yaml.NewCurrent(root)
	if cur, ok, cerr := current.Current(); cerr == nil && ok && cur == id {
		return current.ClearCurrent()
	}
	return nil
}
