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

// newRmCmd builds `mtt rm [<id>...] [-]`: hard-delete tasks (distinct from cancel).
// A single explicit id keeps today's surface (removed <id>, exit 4 on not-found);
// multiple ids / "-" / --filter select a set for a best-effort bulk delete.
func newRmCmd() *cobra.Command {
	var force, dryRun, noRun bool
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
				return runRmSingle(cmd, root, args[0], force, dryRun, noRun)
			}
			ids, err := selectTaskIDs(cmd, args, true)
			if err != nil {
				return err
			}
			if dryRun {
				return previewBulk(cmd, ids)
			}
			cfg, settings, err := yaml.Load(root)
			if err != nil {
				return err
			}
			_, by, why, err := resolveAttribution(cmd, settings.Author)
			if err != nil {
				return err
			}
			bl, err := loadBacklinks(root)
			if err != nil {
				return err
			}
			ev, closeOut, err := newEventEmitter(cmd, root, cfg, settings)
			if err != nil {
				return err
			}
			defer closeOut()
			remover := core.NewRemover(yaml.NewTaskStore(root), yaml.NewAuditStore(root), time.Now, ev)
			results, err := remover.RemoveMany(ids, force, by, why, bl, noRun)
			if err != nil {
				return err // pre-flight ErrMissingAttribution → exit 2 (raw, not via reportBulk)
			}
			items := make([]bulkItem, 0, len(results))
			for _, r := range results {
				items = append(items, bulkItem{id: r.ID, err: r.Err})
				// A *PostActionError means the task IS deleted (only its event
				// finalization failed) — the pointer must not dangle (t66).
				if r.Err == nil || errors.As(r.Err, new(*core.PostActionError)) {
					_ = clearCurrentIfMatches(root, r.ID) // best-effort in bulk: the task is already reported removed
				}
			}
			return reportBulk(cmd, items, "removed")
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "delete even if referenced (leaves dangling refs)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the affected tasks without deleting")
	addNoRunFlag(cmd, &noRun)
	addSelectorFilterFlags(cmd)
	return cmd
}

// runRmSingle is the back-compat single-id path: exact `removed <id>` output and
// exit 4 on not-found (via core.Remover.Remove's ErrNotFound wrap).
func runRmSingle(cmd *cobra.Command, root, idArg string, force, dryRun, noRun bool) error {
	id, err := mtt.NewTaskID(idArg)
	if err != nil {
		return err
	}
	if dryRun {
		return previewBulk(cmd, []mtt.TaskID{id})
	}
	store := yaml.NewTaskStore(root)
	cfg, settings, err := yaml.Load(root)
	if err != nil {
		return err
	}
	_, by, why, err := resolveAttribution(cmd, settings.Author)
	if err != nil {
		return err
	}
	// Remove returns only an error, so capture the task before deleting when --json.
	var removed mtt.Task
	if jsonFlag(cmd) {
		removed, err = store.Get(id)
		if err != nil {
			if errors.Is(err, mtt.ErrNotFound) {
				return taskNotFound(id)
			}
			return err
		}
	}
	bl, err := loadBacklinks(root)
	if err != nil {
		return err
	}
	ev, closeOut, err := newEventEmitter(cmd, root, cfg, settings)
	if err != nil {
		return err
	}
	defer closeOut()
	remover := core.NewRemover(store, yaml.NewAuditStore(root), time.Now, ev)
	rmErr := remover.Remove(id, force, by, why, bl, noRun)
	if rmErr != nil && !errors.As(rmErr, new(*core.PostActionError)) {
		return rmErr
	}
	// A clear failure must not mask a pending finalization error (exit 5 > 1).
	if cerr := clearCurrentIfMatches(root, id); cerr != nil && rmErr == nil {
		return cerr
	}
	return finishMutation(cmd, rmErr, func() error {
		if jsonFlag(cmd) {
			return writeJSON(cmd.OutOrStdout(), toTaskJSON(removed))
		}
		_, werr := fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", id)
		return werr
	})
}

// loadBacklinks builds the cross-store backlink index (tasks + notes) so the delete
// guard refuses a task referenced by another task OR a note (D9). core.Remover stays
// store-agnostic — it consumes this computed value, never a KnowledgeStore port.
func loadBacklinks(root string) (core.Backlinks, error) {
	tasks, err := yaml.NewTaskStore(root).List()
	if err != nil {
		return nil, err
	}
	notes, err := yaml.NewKnowledgeStore(root).ListNotes()
	if err != nil {
		return nil, err
	}
	return core.NewBacklinks(tasks, notes), nil
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
