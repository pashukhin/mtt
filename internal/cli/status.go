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

// gateTailLines is how many trailing lines of a FAILED gate command the CLI asks
// the runner to echo (U2) when the command's own output is otherwise hidden.
const gateTailLines = 10

// newStatusCmd builds `mtt status <id> <new>`: one gated flow transition. Its
// only local flag is --no-run (the sugar path deliberately cannot bypass the
// gate); --verbose/--log-file are root-persistent and shared with the sugar.
func newStatusCmd() *cobra.Command {
	var noRun bool
	cmd := &cobra.Command{
		Use:   "status [<id>] <new-status>",
		Short: "Move a task across one flow edge (runs & gates the edge's commands)",
		Long: `Move a task across ONE flow edge, running (and gating on) that edge's commands.

The id is optional: when omitted it resolves to the current task ('mtt use <id>').
The shorthand 'mtt <new-status> [<id>]' does the same move (e.g. 'mtt done t1' or,
on the current task, 'mtt done'). A red gate blocks the move (exit 3) and leaves
the task untouched; re-run with -v or --log-file to see the failing command's output.`,
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
	// When the commands' own output is otherwise hidden, ask the runner to echo a
	// failing command's output tail (so a blocked gate shows why), and append a
	// hint to the block error; with -v/--log-file the output is already visible,
	// so neither fires (no duplication).
	hidden := !verbose && logFile == ""
	tail := 0
	if hidden {
		tail = gateTailLines
	}
	runner := exec.NewRunner(root, settings.CommandTimeout, cmd.ErrOrStderr(), cmdOut, tail)

	tr := core.NewTransitioner(yaml.NewTaskStore(root), cfg, runner, time.Now)
	task, txErr := tr.Transition(id, to, core.TransitionOptions{
		Role: role, By: by, Why: why, NoRun: noRun,
		RequireWho: settings.Require.Who, RequireWhy: settings.Require.Why,
	})
	// ErrPostAction (t21): the move IS persisted (only the post phase failed) — fall
	// through to render it, then surface the post error + exit 5. Any other error
	// means no move: return it (with the blocked-gate hint).
	postFailed := errors.Is(txErr, core.ErrPostAction)
	if txErr != nil && !postFailed {
		if hidden && errors.Is(txErr, core.ErrBlocked) {
			return fmt.Errorf("%w\n  hint: re-run with -v or --log-file to see the command's full output", txErr)
		}
		return txErr
	}
	if err := applyCurrent(root, cfg, task, id); err != nil {
		return fmt.Errorf("transition applied but updating the current pointer failed: %w", err)
	}
	// Render the move FIRST (it happened): --json emits the task object on stdout, text
	// prints the move line + guidance. NB: local `e` for the render writes — never txErr,
	// or a successful write would clobber ErrPostAction to nil and lose exit 5.
	if jsonFlag(cmd) {
		if e := writeJSON(cmd.OutOrStdout(), toTaskJSON(task)); e != nil {
			return e
		}
	} else {
		last := task.History[len(task.History)-1]
		out := cmd.OutOrStdout()
		if _, e := fmt.Fprintf(out, "%s: %s → %s\n", id, last.From, last.To); e != nil {
			return e
		}
		if g := moveGuidance(cfg, task.ID, task.Type, last.From, last.To); g != "" {
			if _, e := fmt.Fprint(out, g); e != nil {
				return e
			}
		}
	}
	// exit-5 (t28): AFTER the move is rendered (it IS persisted), print the recovery block
	// on stderr in BOTH modes — so the order reads move-render → recovery → Execute's
	// `error:` line, not the inverted "move applied" before the move line. The cause is
	// left to Execute; here we add the idempotence warning + the exact remaining commands.
	if postFailed {
		var pe *core.PostActionError
		if errors.As(txErr, &pe) {
			w := cmd.ErrOrStderr()
			_, _ = fmt.Fprintln(w, "move applied — the status change IS saved; do NOT re-run the move.")
			_, _ = fmt.Fprintln(w, "finish the finalization by hand:")
			for _, c := range pe.Remaining {
				_, _ = fmt.Fprintf(w, "  %s\n", c)
			}
		}
	}
	return txErr // nil on success, ErrPostAction → exit 5 in both modes
}

// applyCurrent moves the personal current-task pointer per the edge just
// traversed: the task's last history entry gives from->to, and the type's
// transition carries the set/clear rule. A no-op when the edge declares nothing.
func applyCurrent(root string, cfg mtt.Config, task mtt.Task, id mtt.TaskID) error {
	if len(task.History) == 0 {
		return nil // only reached after a transition (which appends history); defensive
	}
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
		// G304: logFile is the user's own --log-file argument to a local CLI;
		// writing the path they named is the intended behavior, not a sink.
		f, err := os.Create(logFile) //nolint:gosec
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
