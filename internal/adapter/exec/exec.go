// Package exec implements core.Runner: it runs a transition's commands as gates,
// in the project root, each with a per-command timeout, stopping at the first
// non-zero exit. Commands are trusted project config (like a Makefile), never
// network input.
package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Runner runs shell commands in dir, each bounded by timeout. It reports live
// pipeline progress to `progress` (always) and streams each command's own
// stdout/stderr to `cmdOut` (opt-in — the CLI passes io.Discard, stderr, or a
// file). Timing is display-only (not persisted).
type Runner struct {
	dir      string
	timeout  time.Duration
	progress io.Writer
	cmdOut   io.Writer
}

// NewRunner returns a Runner that executes commands with cwd=dir and the given
// per-command timeout. progress receives the ▶/✓/✗ pipeline lines; cmdOut
// receives the commands' own output. Nil writers default to io.Discard.
func NewRunner(dir string, timeout time.Duration, progress, cmdOut io.Writer) *Runner {
	if progress == nil {
		progress = io.Discard
	}
	if cmdOut == nil {
		cmdOut = io.Discard
	}
	return &Runner{dir: dir, timeout: timeout, progress: progress, cmdOut: cmdOut}
}

// runReport runs one command, reports ▶ then ✓|✗ with timing to progress, and
// returns its Check plus any operational error. Shared by Run and Compensate.
func (r *Runner) runReport(cmd mtt.Command) (mtt.Check, error) {
	_, _ = fmt.Fprintf(r.progress, "▶ %s\n", cmd.Run)
	start := time.Now()
	timeout := cmd.Timeout
	if timeout <= 0 {
		timeout = r.timeout // fall back to the global command_timeout
	}
	exit, err := r.runOne(cmd.Run, timeout)
	elapsed := time.Since(start).Round(time.Millisecond)
	mark := "✓"
	if exit != 0 || err != nil {
		mark = "✗"
	}
	_, _ = fmt.Fprintf(r.progress, "%s %s (exit %d, %s)\n", mark, cmd.Run, exit, elapsed)
	return mtt.Check{Cmd: cmd.Run, Exit: exit}, err
}

// Run executes commands in order, recording a Check per executed command and
// reporting live progress. It stops at the first non-zero exit (a Check, not an
// error). An operational failure (launch error or timeout) returns the checks so
// far — with the failing command's Check as the LAST element — plus a non-nil
// error (core's compensation relies on this ordering).
func (r *Runner) Run(commands []mtt.Command) ([]mtt.Check, error) {
	checks := make([]mtt.Check, 0, len(commands))
	for _, cmd := range commands {
		ck, err := r.runReport(cmd)
		checks = append(checks, ck)
		if err != nil {
			return checks, err
		}
		if ck.Exit != 0 {
			return checks, nil
		}
	}
	return checks, nil
}

// Compensate runs already-expanded compensators best-effort: in order, NEVER
// stopping and NEVER returning an error (an operational failure is recorded as
// Exit -1 by runOne). It prints a labeled compensation phase to progress. core
// passes the reversed, succeeded-only rollbacks.
func (r *Runner) Compensate(commands []mtt.Command) []mtt.Check {
	if len(commands) == 0 {
		return nil
	}
	_, _ = fmt.Fprintf(r.progress, "↩ compensating (%d command%s)\n", len(commands), plural(len(commands)))
	checks := make([]mtt.Check, 0, len(commands))
	for _, cmd := range commands {
		ck, _ := r.runReport(cmd) // best-effort: ignore the operational error, never stop
		checks = append(checks, ck)
	}
	return checks
}

// plural returns "s" unless n == 1.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// runOne runs a single command with the given timeout, streaming its output to
// cmdOut and returning its exit code. A clean non-zero exit yields (code, nil); a
// timeout or launch failure yields (-1, error).
func (r *Runner) runOne(cmd string, timeout time.Duration) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	name, args := shell(cmd)
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = r.dir
	c.Stdout = r.cmdOut
	c.Stderr = r.cmdOut
	err := c.Run()
	if err == nil {
		return 0, nil
	}
	if ctx.Err() == context.DeadlineExceeded {
		return -1, fmt.Errorf("command %q timed out after %s", cmd, timeout)
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
