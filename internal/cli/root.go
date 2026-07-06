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
var version = "0.7.0-dev"

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
		newListCmd(), newEditCmd(), newTreeCmd(), newDepCmd(), newReadyCmd(), newStatusCmd())
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
	if len(args) == 2 {
		if routed, err := trySugar(cmd, args[0], args[1]); routed {
			return err
		}
	}
	return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
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
		return false, nil
	}
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
	return true, runTransition(cmd, root, cfg, settings, id, to, false)
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
	default:
		return 1
	}
}
