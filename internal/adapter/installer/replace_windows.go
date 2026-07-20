//go:build windows

package installer

import (
	"fmt"
	"os"
)

// Replace swaps a running Windows .exe: a running image can be renamed but not
// overwritten. Remove any stale ".old" (rename won't overwrite), move the running
// exe aside, write the new bytes; on failure, roll the old one back. The fresh
// ".old" is left for best-effort cleanup on a later run.
//
// NOTE: unverified in CI (no Windows runner) — see the spec's D6 recorded risk.
func (r *replacer) Replace(path string, newBinary []byte) error {
	old := path + ".old"
	_ = os.Remove(old)
	if err := os.Rename(path, old); err != nil {
		return fmt.Errorf("move running exe aside (%s): %w", path, err)
	}
	if err := os.WriteFile(path, newBinary, 0o755); err != nil {
		_ = os.Rename(old, path) // rollback
		return fmt.Errorf("write new binary %s: %w", path, err)
	}
	return nil
}
