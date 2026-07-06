// Package cli assembles the mtt command-line interface.
package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/core"
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
