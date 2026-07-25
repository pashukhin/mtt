// Package fsutil provides a durable, atomic file write shared across adapters:
// temp-file + fsync + rename + parent-dir fsync, with an installer-style mode
// policy (preserve an existing non-empty target's mode, else a fallback). It is
// infrastructure only — no domain types, no storage semantics.
package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// AtomicWrite writes data to path via a temp file in the same directory + rename,
// with the installer's durability discipline: chmod (preserve an existing
// non-empty target's mode, else fallbackMode), fsync the file before close (the
// rename must never promote un-flushed bytes), and fsync the parent directory
// after the rename so the new directory entry itself survives a crash. A
// zero-byte existing target is treated as absent (its umask-filtered mode must
// not masquerade as a user-tightened one), so fallbackMode wins there.
// MkdirAll of the parent directory is the caller's responsibility.
func AtomicWrite(path string, data []byte, fallbackMode os.FileMode) error {
	mode := fallbackMode
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		mode = info.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".fsutil-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := f.Chmod(mode); err != nil {
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
	// The rename has landed; a SyncDir failure below surfaces as a write error
	// even though the content is in place. Fail-loud is deliberate: a store that
	// cannot make its writes durable should say so.
	return SyncDir(filepath.Dir(path))
}

// SyncDir fsyncs a directory so a just-renamed entry is durable. Best-effort
// where a directory handle cannot be synced: Windows (FlushFileBuffers on a
// directory fails), filesystems reporting unsupported, and mounts (some
// network/FUSE) that return EINVAL for directory fsync — git tolerates EINVAL
// the same way. The write itself is already flushed; only the entry's
// durability window stays platform-dependent there.
func SyncDir(dir string) error {
	// #nosec G304 -- dir is a caller-supplied local directory path (project root
	// subtree), never network/untrusted input. The yaml adapter's exclusion no
	// longer covers this code once extracted, so suppress inline (init.go idiom).
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open dir %s: %w", dir, err)
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil &&
		!errors.Is(err, errors.ErrUnsupported) && !errors.Is(err, syscall.EINVAL) {
		if runtime.GOOS == "windows" {
			return nil
		}
		return fmt.Errorf("sync dir %s: %w", dir, err)
	}
	return nil
}
