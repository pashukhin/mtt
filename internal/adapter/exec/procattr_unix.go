//go:build !windows

package exec

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// configureGroupKill makes c the leader of a new process group (Setpgid) and
// overrides c's context-cancel to SIGKILL the ENTIRE group, so a gate command
// that backgrounds a child cannot outlive its timeout. -c.Process.Pid addresses
// the whole group (pgid == the leader's pid, because Setpgid put the child in a
// fresh group of its own). A group that has already exited yields ESRCH, mapped
// to os.ErrProcessDone so os/exec's watchCtx treats it as "nothing to cancel"
// rather than surfacing a spurious cancel error.
func configureGroupKill(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		if err := syscall.Kill(-c.Process.Pid, syscall.SIGKILL); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}
		return nil
	}
}
