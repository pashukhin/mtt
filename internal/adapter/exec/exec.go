// Package exec implements core.Runner: it runs a transition's commands as gates,
// in the project root, each with a per-command timeout, stopping at the first
// non-zero exit. Commands are trusted project config (like a Makefile), never
// network input.
package exec

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Runner runs shell commands in dir, each bounded by timeout.
type Runner struct {
	dir     string
	timeout time.Duration
}

// NewRunner returns a Runner that executes commands with cwd=dir and the given
// per-command timeout.
func NewRunner(dir string, timeout time.Duration) *Runner {
	return &Runner{dir: dir, timeout: timeout}
}

// Run executes commands in order, recording a Check per executed command. It
// stops at the first non-zero exit (a Check, not an error). An operational
// failure (launch error or timeout) returns the checks so far plus a non-nil
// error.
func (r *Runner) Run(commands []string) ([]mtt.Check, error) {
	checks := make([]mtt.Check, 0, len(commands))
	for _, cmd := range commands {
		exit, err := r.runOne(cmd)
		checks = append(checks, mtt.Check{Cmd: cmd, Exit: exit})
		if err != nil {
			return checks, err
		}
		if exit != 0 {
			return checks, nil
		}
	}
	return checks, nil
}

// runOne runs a single command, returning its exit code. A clean non-zero exit
// yields (code, nil); a timeout or launch failure yields (-1, error).
func (r *Runner) runOne(cmd string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	name, args := shell(cmd)
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = r.dir
	err := c.Run()
	if err == nil {
		return 0, nil
	}
	if ctx.Err() == context.DeadlineExceeded {
		return -1, fmt.Errorf("command %q timed out after %s", cmd, r.timeout)
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), nil // clean non-zero exit: data, not an error
	}
	return -1, fmt.Errorf("command %q failed to run: %w", cmd, err)
}

// shell selects the platform shell that runs a command string: cmd /c on
// Windows, sh -c elsewhere. (CI is Linux; the Windows branch is documented, not
// CI-tested.)
func shell(cmd string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", cmd}
	}
	return "sh", []string{"-c", cmd}
}
