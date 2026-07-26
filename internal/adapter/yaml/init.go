package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pashukhin/mtt/internal/fsutil"
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
	return writeConfig(root, content, force)
}

// InstallConfig validates external template bytes (fail-closed) then writes them
// verbatim. Used for --template <path|url>; the built-in path stays Init.
func InstallConfig(root string, data []byte, force bool) error {
	if err := ValidateTemplateBytes(data); err != nil {
		return err
	}
	return writeConfig(root, data, force)
}

// writeConfig writes content to <root>/.mtt/config.yaml atomically, dropping the
// .gitignore and refusing overwrite without force (shared by the builtin render
// path and the external verbatim path).
func writeConfig(root string, content []byte, force bool) error {
	dir := filepath.Join(root, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if err := writeGitignore(dir); err != nil {
		return err
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

// writeGitignore drops .mtt/.gitignore ignoring the personal config.local.yaml
// overlay, so it never becomes committable. Create-if-absent (O_EXCL): an
// existing file is the user's — never clobbered, even under Init force. It runs
// before the config existence check, so a re-init heals a missing .gitignore.
func writeGitignore(dir string) error {
	path := filepath.Join(dir, ".gitignore")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if _, err := f.Write([]byte("config.local.yaml\n")); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}

// filePerm is the store's write-perm policy for NEW files (c18): 0644, the
// git-checkout default, so fresh writes and checked-out files agree
// cross-machine. An existing file keeps its own mode (installer-style preserve).
const filePerm = 0o644

// atomicWrite writes data durably at filePerm for a NEW file (an existing
// target's mode is preserved). Delegates to the shared fsutil discipline (t52).
func atomicWrite(path string, data []byte) error {
	return fsutil.AtomicWrite(path, data, filePerm)
}

// atomicWriteMode is atomicWrite with an explicit fallback mode for a NEW file
// (the Current store uses 0600 for a fresh config.local.yaml — personal data up
// to backend credentials). A non-empty existing target's mode still wins.
func atomicWriteMode(path string, data []byte, fallback os.FileMode) error {
	return fsutil.AtomicWrite(path, data, fallback)
}
