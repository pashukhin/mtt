package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/exec"
	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newGateRunner builds the exec runner every command-pipeline consumer uses —
// extracted from runTransition so gate/post edges and lifecycle events render
// identically (▶/✓/✗ stderr progress, -v/--log-file, failing-tail echo). The
// returned closer flushes/closes the optional log file.
func newGateRunner(cmd *cobra.Command, root string, settings yaml.Settings) (*exec.Runner, func(), error) {
	verbose, _ := cmd.Flags().GetBool("verbose")
	logFile, _ := cmd.Flags().GetString("log-file")
	cmdOut, closeOut, err := gateOutputWriter(cmd, verbose, logFile)
	if err != nil {
		return nil, nil, err
	}
	// When the commands' own output is otherwise hidden, ask the runner to echo a
	// failing command's output tail; with -v/--log-file it is already visible.
	tail := 0
	if !verbose && logFile == "" {
		tail = gateTailLines
	}
	return exec.NewRunner(root, settings.CommandTimeout, cmd.ErrOrStderr(), cmdOut, tail), closeOut, nil
}

// newEventEmitter wires the core.EventEmitter a mutating command passes into
// its usecase: the shared gate runner + the audit store (--no-run skip records).
func newEventEmitter(cmd *cobra.Command, root string, cfg mtt.Config, settings yaml.Settings) (*core.EventEmitter, func(), error) {
	runner, closeOut, err := newGateRunner(cmd, root, settings)
	if err != nil {
		return nil, nil, err
	}
	return core.NewEventEmitter(cfg, runner, yaml.NewAuditStore(root), time.Now), closeOut, nil
}

// eventOptions resolves a mutating command's --no-run bypass + attribution into
// core.EventOptions (--who/--by > MTT_BY > config.local author; --why).
func eventOptions(cmd *cobra.Command, noRun bool, author string) (core.EventOptions, error) {
	_, by, why, err := resolveAttribution(cmd, author)
	if err != nil {
		return core.EventOptions{}, err
	}
	return core.EventOptions{NoRun: noRun, By: by, Why: why}, nil
}

// renderPostRecovery prints the exit-5 recovery block on stderr for a
// *core.PostActionError: the saved-marker line, then the remaining commands —
// OMITTED entirely when Remaining is empty (e.g. a failed audit append has no
// user-runnable recovery; never render an empty "finish by hand:" list). A
// non-post error renders nothing. Shared by moves (t28) and mutations (t66).
func renderPostRecovery(cmd *cobra.Command, err error, savedLine string) {
	var pe *core.PostActionError
	if !errors.As(err, &pe) {
		return
	}
	w := cmd.ErrOrStderr()
	_, _ = fmt.Fprintln(w, savedLine)
	if len(pe.Remaining) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w, "finish the finalization by hand:")
	for _, c := range pe.Remaining {
		_, _ = fmt.Fprintf(w, "  %s\n", c)
	}
}

// mutationSavedLine is the saved-marker for non-flow mutations (the move path
// keeps its own wording in runTransition).
const mutationSavedLine = "the change IS saved; do NOT re-run the mutation."

// addNoRunFlag registers the shared --no-run bypass flag on a mutating command.
func addNoRunFlag(cmd *cobra.Command, noRun *bool) {
	cmd.Flags().BoolVar(noRun, "no-run", false, "skip the configured lifecycle-event pipeline (forces --who and --why)")
}

// finishMutation is the shared tail of a single-entity mutating command: on a
// hard error it propagates; on success OR a *core.PostActionError (the mutation
// persisted, only the event finalization failed) it renders the primary output
// first (t28 order — the object/line on stdout), then the recovery block on
// stderr, and returns err (nil, or the post error → exit 5).
func finishMutation(cmd *cobra.Command, err error, render func() error) error {
	if err != nil && !errors.As(err, new(*core.PostActionError)) {
		return err
	}
	if werr := render(); werr != nil {
		return werr
	}
	renderPostRecovery(cmd, err, mutationSavedLine)
	return err
}
