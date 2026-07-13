package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is the ldflags-injected build version ("-X …/internal/cli.version=…").
// It defaults to "dev"; release and explicit `make` builds stamp a real value.
var version = "dev"

// resolveVersion returns the effective version: an ldflags-injected value wins,
// else the module version recorded in the build info (populated by
// `go install …@vX.Y.Z`), else "dev".
func resolveVersion() string {
	return resolve(version, readBuildVersion)
}

// resolve is the testable core of version resolution.
func resolve(ldflags string, buildVersion func() string) string {
	if ldflags != "" && ldflags != "dev" {
		return ldflags
	}
	if bv := buildVersion(); bv != "" && bv != "(devel)" {
		return bv
	}
	return "dev"
}

// readBuildVersion returns the main module's version from the build info. Plain
// `go build`/`go test` binaries yield "(devel)" or "" (which resolve treats as
// absent); "" is also the fallback when build info is unavailable at all.
func readBuildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return ""
}

// newVersionCmd prints the build version.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the mtt version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), resolveVersion())
			return err
		},
	}
}
