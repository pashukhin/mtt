//go:build !windows

package installer

import (
	"fmt"
	"os"
	"path/filepath"
)

// Replace writes newBinary to a same-dir temp file (so the rename is atomic on one
// filesystem), preserves the target's mode, then renames over the target. The
// running process keeps its open inode. A permission failure surfaces from the
// temp-create (attempt-and-surface, not a racy stat precheck).
func (r *replacer) Replace(path string, newBinary []byte) error {
	dir := filepath.Dir(path)
	mode := os.FileMode(0o755)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, ".mtt-update-*")
	if err != nil {
		return fmt.Errorf("cannot write %s (create temp): %w", dir, err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(newBinary); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename over %s: %w", path, err)
	}
	ok = true
	return nil
}
