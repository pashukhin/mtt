package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrAlreadyInitialized is returned by Init when config exists and force is false.
var ErrAlreadyInitialized = errors.New("mtt: already initialized (.mtt/config.yaml exists; use --force)")

// Init writes .mtt/config.yaml under root from the named template, substituting
// the project name. It refuses to overwrite an existing config unless force is set.
// The write is atomic (temp file + rename).
func Init(root, tmplName, projectName string, force bool) error {
	content, err := renderTemplate(tmplName, projectName)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	dst := filepath.Join(dir, configName)
	if !force {
		if _, statErr := os.Stat(dst); statErr == nil {
			return ErrAlreadyInitialized
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("stat %s: %w", dst, statErr)
		}
	}
	return atomicWrite(dst, content)
}

// atomicWrite writes data to path via a temp file in the same directory + rename.
func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}
