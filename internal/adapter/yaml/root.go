package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// dirName is the mtt data directory created inside a project root.
const dirName = ".mtt"

// configName is the filename of the mtt project config inside dirName.
const configName = "config.yaml"

// ErrNotInitialized is returned when no .mtt directory is found.
var ErrNotInitialized = errors.New("mtt: not initialized (no .mtt directory found)")

// FindRoot walks up from start until it finds a directory that contains .mtt/,
// returning that directory. It returns ErrNotInitialized when none is found.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve start dir: %w", err)
	}
	for {
		info, statErr := os.Stat(filepath.Join(dir, dirName))
		if statErr == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotInitialized
		}
		dir = parent
	}
}
