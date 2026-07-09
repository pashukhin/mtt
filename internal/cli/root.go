// Package cli assembles the mtt command-line interface.
package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// version is the build version, overridable at build time via -ldflags.
var version = "0.8.7-dev"

// NewRootCmd builds the root mtt command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "mtt",
		Short:         "mtt — minimalist file-backed task tracker for agents and humans",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE:          runSugar,
	}
	root.PersistentFlags().String("dir", "", "project root containing .mtt/ (overrides discovery; env MTT_DIR)")
	root.PersistentFlags().Bool("json", false, "emit machine-readable JSON output")
	root.PersistentFlags().String("role", "", "acting role, recorded in history (env MTT_ROLE)")
	root.PersistentFlags().String("by", "", "acting subject, recorded in history (env MTT_BY)")
	root.PersistentFlags().String("who", "", "acting subject, alias of --by, recorded in history")
	root.PersistentFlags().String("why", "", "durable free-text reason recorded in history")
	root.PersistentFlags().BoolP("verbose", "v", false, "stream gate command output to stderr")
	root.PersistentFlags().String("log-file", "", "write gate command output to a file")
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd(), newShowCmd(),
		newListCmd(), newEditCmd(), newTreeCmd(), newDepCmd(), newReadyCmd(), newStatusCmd(),
		newUseCmd(), newRmCmd(), newRoadmapCmd(), newTagCmd())
	return root
}

// Execute runs the root command and returns a process exit code (0 success; 3
// gate blocked; 6 invalid transition; 1 any other error).
func Execute() int {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(root.ErrOrStderr(), "error:", err)
		return exitCode(err)
	}
	return 0
}

// runSugar is the root fallback: `mtt <status> <id>` routes to a single-edge
// transition when arg0 is a status of arg1's type flow; otherwise it is an
// unknown command. Real subcommands are dispatched by cobra before this runs, so
// a registered command always wins a name clash.
func runSugar(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	if len(args) == 1 {
		if routed, err := trySugarCurrent(cmd, args[0]); routed {
			return err
		}
		return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
	}
	if len(args) == 2 {
		if routed, err := trySugar(cmd, args[0], args[1]); routed {
			return err
		}
	}
	return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
}

// trySugarCurrent classifies `mtt <status>` (1 arg) as a status move on the
// current task. It routes when a current task is set and arg0 is a status in its
// type flow; when no current is set but arg0 is a plausible status name, it claims
// the command with an actionable error; otherwise it declines (-> unknown command).
func trySugarCurrent(cmd *cobra.Command, statusArg string) (bool, error) {
	root, err := projectRoot(cmd)
	if err != nil {
		return false, nil
	}
	cfg, settings, err := yaml.Load(root)
	if err != nil {
		return false, nil
	}
	id, ok, err := yaml.NewCurrent(root).Current()
	if err != nil {
		return false, nil
	}
	if !ok {
		if statusInAnyFlow(cfg, statusArg) {
			return true, errors.New("no current task set; run `mtt use <id>` or give an id")
		}
		return false, nil
	}
	task, err := yaml.NewTaskStore(root).Get(id)
	if err != nil {
		return true, staleCurrentErr(id) // current points at a task that no longer exists
	}
	return classifyStatusMove(cmd, root, cfg, settings, task, statusArg)
}

// trySugar classifies `<arg0> <arg1>` as a status move. It routes only when the
// project loads, arg1 is an existing task, and arg0 is a status in that task's
// type flow. Any classification miss returns routed=false (→ unknown command);
// once routed, the transition's own error (invalid edge, blocked, attribution)
// is returned verbatim.
func trySugar(cmd *cobra.Command, statusArg, idArg string) (bool, error) {
	root, err := projectRoot(cmd)
	if err != nil {
		return false, nil
	}
	cfg, settings, err := yaml.Load(root)
	if err != nil {
		return false, nil
	}
	id, err := mtt.NewTaskID(idArg)
	if err != nil {
		return false, nil
	}
	task, err := yaml.NewTaskStore(root).Get(id)
	if err != nil {
		// arg1 is not an existing task. If arg0 is a plausible status verb the user
		// meant a status move on a missing task → not-found (exit 4), not an unknown
		// command; otherwise decline (→ unknown command). Mirrors trySugarCurrent.
		if errors.Is(err, mtt.ErrNotFound) && statusInAnyFlow(cfg, statusArg) {
			return true, taskNotFound(id)
		}
		return false, nil
	}
	return classifyStatusMove(cmd, root, cfg, settings, task, statusArg)
}

// classifyStatusMove is the shared tail of the 1-arg and 2-arg sugar once the
// target task is in hand: it routes to the transition path iff statusArg is a
// status in the task's type flow (else routed=false → unknown command). The two
// callers differ only in how they obtain the task and in their Get-failure policy
// (2-arg declines; 1-arg reports a stale current) — that stays in the callers.
func classifyStatusMove(cmd *cobra.Command, root string, cfg mtt.Config, settings yaml.Settings, task mtt.Task, statusArg string) (bool, error) {
	typ, ok := cfg.TypeByName(task.Type)
	if !ok {
		return false, nil
	}
	to, err := mtt.NewStatusName(statusArg)
	if err != nil {
		return false, nil
	}
	if _, ok := typ.StatusKind(to); !ok {
		return false, nil
	}
	return true, runTransition(cmd, root, cfg, settings, task.ID, to, false)
}

// exitCode maps an error to the CLI's exit-code taxonomy.
func exitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, core.ErrBlocked):
		return 3
	case errors.Is(err, core.ErrInvalidTransition):
		return 6
	case errors.Is(err, core.ErrMissingAttribution):
		return 2
	case errors.Is(err, mtt.ErrNotFound):
		return 4
	default:
		return 1
	}
}
