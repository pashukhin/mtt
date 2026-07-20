//go:build !windows

package exec

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// TestRunTimeoutKillsProcessGroup proves SEC1 (t14): a gate command that
// backgrounds a child must not outlive its timeout. The command records the
// background child's PID, then both it and the child sleep far past the (short)
// timeout. After Run returns the timeout error, the recorded PID must be DEAD.
//
// RED baseline (per the spec): with configureGroupKill a no-op (default
// process-only cancel) but WaitDelay ON, Run returns after ~WaitDelay while the
// orphan still sleeps -> kill(pid,0)==nil -> this fails. The Unix group-kill
// makes it pass. Do NOT remove WaitDelay to get red (Wait would block ~30s until
// the orphan self-exits -> false green); do NOT keep the group cancel without
// Setpgid (the child would sit in the test runner's own group -> kill(-pid)
// would SIGKILL the test process).
func TestRunTimeoutKillsProcessGroup(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	// Background a long sleep, record its PID, then sleep long in the foreground.
	script := "sleep 30 & echo $! > " + pidFile + "; sleep 30"
	_, err := NewRunner(dir, 200*time.Millisecond, io.Discard, io.Discard, 0).
		Run([]mtt.Command{{Run: script}})
	if err == nil {
		t.Fatal("want a timeout error, got nil")
	}
	pid := readPID(t, pidFile)
	// The orphan must be dead. Poll briefly (<= 1s, well under the orphan's 30s
	// lifetime, so a live orphan cannot masquerade as reaped) to absorb reaping.
	deadline := time.Now().Add(time.Second)
	for {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return // dead — success
		}
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL) // clean up the survivor
			t.Fatalf("background child %d survived the gate timeout (process group not killed)", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// readPID waits for the gate command to write the background child's PID, then
// parses it. The child writes it within a few ms of launch; retry until present.
func readPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		b, err := os.ReadFile(path)
		if err == nil && len(strings.TrimSpace(string(b))) > 0 {
			pid, perr := strconv.Atoi(strings.TrimSpace(string(b)))
			if perr != nil {
				t.Fatalf("bad pid %q: %v", b, perr)
			}
			return pid
		}
		if time.Now().After(deadline) {
			t.Fatalf("pid file never written: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
