package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

// filePerm is the store's single write-perm policy (c18): every file atomicWrite
// lands gets 0644 — the git-checkout default, so fresh writes and checked-out
// files agree cross-machine (CreateTemp's 0600 must not leak through).
const filePerm = 0o644

// atomicWrite writes data to path via a temp file in the same directory + rename,
// with the installer's durability discipline (c18): chmod to the uniform perm,
// fsync the file before close (the rename must never promote un-flushed bytes —
// this is the source of truth), and fsync the parent directory after the rename
// so the new directory entry itself survives a crash.
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
	if err := f.Chmod(filePerm); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp: %w", err)
	}
	return syncDir(filepath.Dir(path))
}

// syncDir fsyncs a directory so a just-renamed entry is durable. Best-effort on
// platforms where a directory handle cannot be synced (Windows returns an error
// for it) — the write itself is already flushed, only the entry's durability
// window stays platform-dependent there.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open dir %s: %w", dir, err)
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil && !errors.Is(err, errors.ErrUnsupported) {
		if runtime.GOOS == "windows" {
			return nil
		}
		return fmt.Errorf("sync dir %s: %w", dir, err)
	}
	return nil
}
