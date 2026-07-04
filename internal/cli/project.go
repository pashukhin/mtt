package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
)

// resolveDir returns the explicit project directory from --dir, else $MTT_DIR,
// else "" (meaning "discover from the cwd").
func resolveDir(cmd *cobra.Command) string {
	dir, _ := cmd.Flags().GetString("dir")
	if dir == "" {
		dir = os.Getenv("MTT_DIR")
	}
	return dir
}

// projectRoot resolves the project root for a command: --dir/MTT_DIR if set
// (which must itself contain .mtt/, no upward walk), else the nearest ancestor
// of the cwd containing .mtt/ (FindRoot).
func projectRoot(cmd *cobra.Command) (string, error) {
	if dir := resolveDir(cmd); dir != "" {
		if !yaml.HasProject(dir) {
			return "", fmt.Errorf("no .mtt/ in %q", dir)
		}
		return dir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return yaml.FindRoot(cwd)
}

// baseDir resolves the base directory for init: --dir/MTT_DIR if set, else the
// cwd. Unlike projectRoot it does not require an existing .mtt/ (init creates it).
func baseDir(cmd *cobra.Command) (string, error) {
	if dir := resolveDir(cmd); dir != "" {
		return dir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return cwd, nil
}

// jsonFlag reports whether the persistent --json flag was set.
func jsonFlag(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("json")
	return v
}
