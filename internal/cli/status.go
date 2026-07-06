package cli

import (
	"errors"
	"fmt"
	"io"
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
	var (
		noRun   bool
		verbose bool
		logFile string
	)
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
			role, by := resolveRoleBy(cmd, settings.Author)

			cmdOut, closeOut, err := gateOutputWriter(cmd, verbose, logFile)
			if err != nil {
				return err
			}
			defer closeOut()
			runner := exec.NewRunner(root, settings.CommandTimeout, cmd.ErrOrStderr(), cmdOut)

			tr := core.NewTransitioner(yaml.NewTaskStore(root), cfg, runner, time.Now)
			task, err := tr.Transition(id, to, core.TransitionOptions{Role: role, By: by, NoRun: noRun})
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
			}
			last := task.History[len(task.History)-1]
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s → %s\n", id, last.From, last.To)
			return err
		},
	}
	cmd.Flags().BoolVar(&noRun, "no-run", false, "skip the edge's commands (bypass the gate)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "stream gate command output to stderr")
	cmd.Flags().StringVar(&logFile, "log-file", "", "write gate command output to a file")
	return cmd
}

// gateOutputWriter resolves where each gate command's own stdout/stderr goes:
// hidden by default, streamed to stderr with -v, and/or written to --log-file.
// The returned closer flushes the log file (a no-op otherwise).
func gateOutputWriter(cmd *cobra.Command, verbose bool, logFile string) (io.Writer, func(), error) {
	var writers []io.Writer
	if verbose {
		writers = append(writers, cmd.ErrOrStderr())
	}
	closeOut := func() {}
	if logFile != "" {
		f, err := os.Create(logFile)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file %q: %w", logFile, err)
		}
		closeOut = func() { _ = f.Close() }
		writers = append(writers, f)
	}
	switch len(writers) {
	case 0:
		return io.Discard, closeOut, nil
	case 1:
		return writers[0], closeOut, nil
	default:
		return io.MultiWriter(writers...), closeOut, nil
	}
}

// resolveRoleBy resolves --role/--by. role: flag, else MTT_ROLE. by: flag, else
// MTT_BY, else authorDefault (config.local author — the durable subject seam).
func resolveRoleBy(cmd *cobra.Command, authorDefault string) (role, by string) {
	role, _ = cmd.Flags().GetString("role")
	if role == "" {
		role = os.Getenv("MTT_ROLE")
	}
	by, _ = cmd.Flags().GetString("by")
	if by == "" {
		by = os.Getenv("MTT_BY")
	}
	if by == "" {
		by = authorDefault
	}
	return role, by
}
