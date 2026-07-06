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

// newStatusCmd builds `mtt status <id> <new>`: one gated flow transition. Its
// only local flag is --no-run (the sugar path deliberately cannot bypass the
// gate); --verbose/--log-file are root-persistent and shared with the sugar.
func newStatusCmd() *cobra.Command {
	var noRun bool
	cmd := &cobra.Command{
		Use:   "status <id> <new-status>",
		Short: "Move a task across one flow edge (runs & gates the edge's commands)",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 && len(args) != 2 {
				return errors.New("provide a target status (and optionally a task id): mtt status [<id>] <new-status>")
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
			explicit, toArg := "", args[0]
			if len(args) == 2 {
				explicit, toArg = args[0], args[1]
			}
			id, err := resolveTaskID(root, explicit)
			if err != nil {
				return err
			}
			to, err := mtt.NewStatusName(toArg)
			if err != nil {
				return err
			}
			return runTransition(cmd, root, cfg, settings, id, to, noRun)
		},
	}
	cmd.Flags().BoolVar(&noRun, "no-run", false, "skip the edge's commands (bypass the gate)")
	return cmd
}

// runTransition performs one gated flow edge, shared by `mtt status` and the
// `mtt <status> <id>` sugar. cfg/settings/root are pre-loaded by the caller.
func runTransition(cmd *cobra.Command, root string, cfg mtt.Config, settings yaml.Settings, id mtt.TaskID, to mtt.StatusName, noRun bool) error {
	role, by, why, err := resolveAttribution(cmd, settings.Author)
	if err != nil {
		return err
	}
	verbose, _ := cmd.Flags().GetBool("verbose")
	logFile, _ := cmd.Flags().GetString("log-file")
	cmdOut, closeOut, err := gateOutputWriter(cmd, verbose, logFile)
	if err != nil {
		return err
	}
	defer closeOut()
	runner := exec.NewRunner(root, settings.CommandTimeout, cmd.ErrOrStderr(), cmdOut)

	tr := core.NewTransitioner(yaml.NewTaskStore(root), cfg, runner, time.Now)
	task, err := tr.Transition(id, to, core.TransitionOptions{
		Role: role, By: by, Why: why, NoRun: noRun,
		RequireWho: settings.Require.Who, RequireWhy: settings.Require.Why,
	})
	if err != nil {
		return err
	}
	if err := applyCurrent(root, cfg, task, id); err != nil {
		return err
	}
	if jsonFlag(cmd) {
		return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
	}
	last := task.History[len(task.History)-1]
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s → %s\n", id, last.From, last.To)
	return err
}

// applyCurrent moves the personal current-task pointer per the edge just
// traversed: the task's last history entry gives from->to, and the type's
// transition carries the set/clear rule. A no-op when the edge declares nothing.
func applyCurrent(root string, cfg mtt.Config, task mtt.Task, id mtt.TaskID) error {
	typ, ok := cfg.TypeByName(task.Type)
	if !ok {
		return nil
	}
	last := task.History[len(task.History)-1]
	edge, ok := typ.FindTransition(last.From, last.To)
	if !ok {
		return nil
	}
	switch edge.Current {
	case mtt.CurrentSet:
		return yaml.NewCurrent(root).SetCurrent(id)
	case mtt.CurrentClear:
		return yaml.NewCurrent(root).ClearCurrent()
	}
	return nil
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

// resolveAttribution resolves the transition's role/by/why. role: --role, else
// MTT_ROLE. by: --who or --by (mutually exclusive aliases), else MTT_BY, else
// authorDefault (config.local author — the durable subject seam). why: --why.
func resolveAttribution(cmd *cobra.Command, authorDefault string) (role, by, why string, err error) {
	role, _ = cmd.Flags().GetString("role")
	if role == "" {
		role = os.Getenv("MTT_ROLE")
	}
	if cmd.Flags().Changed("who") && cmd.Flags().Changed("by") {
		return "", "", "", errors.New("--who and --by are aliases; set only one")
	}
	who, _ := cmd.Flags().GetString("who")
	byFlag, _ := cmd.Flags().GetString("by")
	by = who
	if by == "" {
		by = byFlag
	}
	if by == "" {
		by = os.Getenv("MTT_BY")
	}
	if by == "" {
		by = authorDefault
	}
	why, _ = cmd.Flags().GetString("why")
	return role, by, why, nil
}
