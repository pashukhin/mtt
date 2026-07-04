// Package cli assembles the mtt command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is the build version, overridable at build time via -ldflags.
var version = "0.0.0-dev"

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
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd(), newShowCmd(), newListCmd(), newEditCmd())
	return root
}

// Execute runs the root command, reporting any error to stderr.
func Execute() error {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(root.ErrOrStderr(), "error:", err)
		return err
	}
	return nil
}
