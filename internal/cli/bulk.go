package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// bulkItem is one task's outcome in a bulk mutation.
type bulkItem struct {
	id  mtt.TaskID
	err error
}

// bulkJSON is the per-item machine view of a bulk mutation.
type bulkJSON struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// idJSON is the dry-run machine view (just the affected ids — nothing happened, so
// no status/error field, keeping the json clean vs reusing bulkJSON).
type idJSON struct {
	ID string `json:"id"`
}

// runBulk applies apply(id) to each id best-effort, renders a report, and returns a
// generic aggregate error (exit 1) when any item failed. With --dry-run it previews
// without applying. verbPast is the success label ("tagged"/"untagged"/"removed").
func runBulk(cmd *cobra.Command, ids []mtt.TaskID, verbPast string, apply func(mtt.TaskID) error) error {
	if dry, _ := cmd.Flags().GetBool("dry-run"); dry {
		return previewBulk(cmd, ids)
	}
	items := make([]bulkItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, bulkItem{id: id, err: apply(id)})
	}
	return reportBulk(cmd, items, verbPast)
}

// reportBulk renders items (human summary + stderr per-item errors, or a --json
// per-item array) and returns a plain aggregate error (no %w) when any failed, so
// exitCode maps it to the generic 1 (never a per-item sentinel).
func reportBulk(cmd *cobra.Command, items []bulkItem, verbPast string) error {
	nFail := 0
	for _, it := range items {
		if it.err != nil {
			nFail++
		}
	}
	if jsonFlag(cmd) {
		rows := make([]bulkJSON, 0, len(items))
		for _, it := range items {
			row := bulkJSON{ID: string(it.id), Status: verbPast}
			if it.err != nil {
				row.Status, row.Error = "error", it.err.Error()
			}
			rows = append(rows, row)
		}
		if err := writeJSON(cmd.OutOrStdout(), rows); err != nil {
			return err
		}
	} else {
		ok := make([]string, 0, len(items))
		for _, it := range items {
			if it.err == nil {
				ok = append(ok, string(it.id))
			} else {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", it.id, it.err)
			}
		}
		if len(ok) > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %d task(s): %s\n", verbPast, len(ok), strings.Join(ok, ", "))
		}
	}
	if nFail > 0 {
		return fmt.Errorf("%d of %d task(s) failed", nFail, len(items))
	}
	return nil
}

// previewBulk prints the selector's affected ids without mutating (--dry-run):
// ids one per line on stdout + a stderr summary, or a --json id array. exit 0.
func previewBulk(cmd *cobra.Command, ids []mtt.TaskID) error {
	if jsonFlag(cmd) {
		rows := make([]idJSON, 0, len(ids))
		for _, id := range ids {
			rows = append(rows, idJSON{ID: string(id)})
		}
		return writeJSON(cmd.OutOrStdout(), rows)
	}
	if err := writeIDs(cmd.OutOrStdout(), ids); err != nil {
		return err
	}
	_, err := fmt.Fprintf(cmd.ErrOrStderr(), "(dry run) would affect %d task(s)\n", len(ids))
	return err
}
