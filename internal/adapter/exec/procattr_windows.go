//go:build windows

package exec

import "os/exec"

// configureGroupKill is a no-op on Windows: there are no POSIX process groups,
// so the command keeps os/exec's default context-cancel (a process-only Kill).
// Best-effort, CI-unverified (no Windows runner) — mirrors the installer's
// replace_windows.go. WaitDelay (set by the caller) still bounds Wait here.
func configureGroupKill(_ *exec.Cmd) {}
