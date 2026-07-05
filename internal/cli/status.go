package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/exec"
	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newStatusCmd builds `mtt status <id> <new>`: one gated flow transition.
func newStatusCmd() *cobra.Command {
	var noRun bool
	cmd := &cobra.Command{
		Use:   "status <id> <new-status>",
		Short: "Move a task across one flow edge (runs & gates the edge's commands)",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("provide a task id and a target status (example: mtt status t1 in_progress)")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			cfg, settings, err := yaml.Load(root)
			if err != nil {
				return err
			}
			id, err := mtt.NewTaskID(args[0])
			if err != nil {
				return err
			}
			to, err := mtt.NewStatusName(args[1])
			if err != nil {
				return err
			}
			role, by := resolveRoleBy(cmd)
			runner := exec.NewRunner(root, settings.CommandTimeout)
			tr := core.NewTransitioner(yaml.NewTaskStore(root), cfg, runner, time.Now)
			task, err := tr.Transition(id, to, core.TransitionOptions{Role: role, By: by, NoRun: noRun})
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
			}
			last := task.History[len(task.History)-1]
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s → %s\n", id, last.From, last.To); err != nil {
				return err
			}
			for _, c := range last.Checks {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s (exit %d)\n", c.Cmd, c.Exit); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noRun, "no-run", false, "skip the edge's commands (bypass the gate)")
	return cmd
}

// resolveRoleBy resolves --role/--by, falling back to MTT_ROLE/MTT_BY (mirrors
// resolveDir).
func resolveRoleBy(cmd *cobra.Command) (role, by string) {
	role, _ = cmd.Flags().GetString("role")
	if role == "" {
		role = os.Getenv("MTT_ROLE")
	}
	by, _ = cmd.Flags().GetString("by")
	if by == "" {
		by = os.Getenv("MTT_BY")
	}
	return role, by
}
