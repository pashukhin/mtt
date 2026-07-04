package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// tasksDirName is the subdirectory of .mtt that holds one file per task.
const tasksDirName = "tasks"

// mint reserves and returns the next flat, per-prefix task ID under .mtt/tasks/.
// It scans <prefix><N>.yaml files, takes max(N)+1 (from 1), and creates the
// reserved file with O_EXCL so a concurrent mint cannot pick the same ID. IDs
// are flat (no parent chain), so identity is stable under re-parenting.
func mint(root, prefix string) (string, error) {
	dir := filepath.Join(root, dirName, tasksDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	re := regexp.MustCompile("^" + regexp.QuoteMeta(prefix) + `(\d+)\.yaml$`)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", dir, err)
	}
	maxN := 0
	for _, e := range entries {
		m := re.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if n, _ := strconv.Atoi(m[1]); n > maxN {
			maxN = n
		}
	}
	for n := maxN + 1; ; n++ {
		id := fmt.Sprintf("%s%d", prefix, n)
		path := filepath.Join(dir, id+".yaml")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = f.Close()
			return id, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("reserve %s: %w", path, err)
		}
	}
}
