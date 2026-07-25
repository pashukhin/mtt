package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// repoRoot returns this repo's root, anchored via runtime.Caller (robust
// regardless of cwd — usable from the testscript Setup hook, which has no
// *testing.T). It walks up to the dir containing go.mod.
func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("repoRoot: go.mod not found")
		}
		dir = parent
	}
}

// hierarchyTemplate returns the repo's hosted hierarchy example path — the
// file-path replacement for the removed `hierarchy` built-in name (t62).
func hierarchyTemplate(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(), "templates", "hierarchy.yaml")
}
