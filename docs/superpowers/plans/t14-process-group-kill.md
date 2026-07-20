# SEC1 process-group kill (t14) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A gate/post command that backgrounds a child must not outlive its timeout — kill the command's **whole process group** on a gate timeout, instead of only the top shell.

**Architecture:** Confined to the `internal/adapter/exec` adapter. A build-tagged seam `configureGroupKill(*exec.Cmd)` makes each command the leader of a new process group (Unix `Setpgid`) and overrides `Cmd.Cancel` to `SIGKILL` the whole group; Windows is a documented no-op. A cross-platform `Cmd.WaitDelay` in the shared `runOne` bounds `Wait` against an inherited-pipe hang. `pkg/mtt`, `core.Runner`, exit codes, config, and every public signature are untouched.

**Tech Stack:** Go 1.23+ (`go.mod` floor `go 1.23.1`), stdlib `os/exec`/`syscall`/`os`/`errors`/`time`. No new dependency. Table-driven unit tests; the SEC1 proof is a hermetic Unix-only test (temp dir, PID file, `kill(pid,0)` liveness).

## Global Constraints

- **Spec of record:** `docs/superpowers/specs/t14-process-group-kill.md`. Every decision (D1–D6) is binding.
- **TDD:** red → green → refactor. Failing test first, watch it fail, then implement. `make check` (gofmt + vet + golangci-lint v2 + `go test -race -cover` + build) green before every commit.
- **Layering unchanged:** the fix lives entirely in the `exec` adapter; no `pkg/mtt`/`core`/`cli`/config/exit-code change. `Compensate` inherits the fix for free (shares `runOne`).
- **Signal policy (D2):** single-phase `SIGKILL` to the negative pgid; **no** SIGTERM/grace/config. `errors.Is(err, syscall.ESRCH)` → `os.ErrProcessDone`.
- **`WaitDelay` (D3):** `2 * time.Second`, a package constant (not config). Always on, both platforms. Never elapses for a normally-terminating command; on the timeout path the group-kill closes the pipe first; it only bites a pipe-holding orphan.
- **Windows (D4):** `configureGroupKill` is a no-op; best-effort process-only kill, CI-unverified (no runner) — mirrors `installer/replace_windows.go`.
- **Build tags:** `procattr_unix.go` = `//go:build !windows`; `procattr_windows.go` = `//go:build windows`; the Unix-only test = `//go:build !windows`.
- **Return contract (D5):** on timeout `ctx.Err() == context.DeadlineExceeded` still holds → `runOne` returns `(-1, "command … timed out …")`; the "failing `Check{Exit:-1}` is last" `core.Runner` contract is preserved.
- **Docs bilingual** (EN + RU) where applicable: `DESIGN`. Grep **all** parallel occurrences before editing.

---

## File structure

**Create:**
- `internal/adapter/exec/procattr_unix.go` (`//go:build !windows`) — `configureGroupKill(*exec.Cmd)`: `Setpgid` + group `Cancel`.
- `internal/adapter/exec/procattr_windows.go` (`//go:build windows`) — `configureGroupKill(*exec.Cmd)`: no-op.
- `internal/adapter/exec/exec_pgid_test.go` (`//go:build !windows`) — the SEC1 orphan-kill test + `readPID` helper.

**Modify:**
- `internal/adapter/exec/exec.go` — in `runOne`: call `configureGroupKill(c)` + set `c.WaitDelay = waitDelay` after `exec.CommandContext`; add the `waitDelay` const.
- `internal/adapter/exec/CLAUDE.md` — document the group-kill + WaitDelay semantics + Windows caveat.
- `DESIGN.md` ↔ `DESIGN.ru.md` — the timeout-kills-the-group clause.
- `CHANGELOG.md` — `[Unreleased]` entry.

---

## Task 1: process-group kill on gate timeout (the fix, red→green)

**Files:**
- Create: `internal/adapter/exec/procattr_unix.go`, `internal/adapter/exec/procattr_windows.go`, `internal/adapter/exec/exec_pgid_test.go`
- Modify: `internal/adapter/exec/exec.go`

**Interfaces:**
- Produces:
  - `func configureGroupKill(c *exec.Cmd)` — build-tagged; Unix sets `SysProcAttr.Setpgid` + a group-kill `Cancel`, Windows is a no-op.
  - `const waitDelay = 2 * time.Second` (package-level, in `exec.go`).
- Consumes: `runOne` (existing) calls both after `exec.CommandContext`, before `c.Run()`.

- [ ] **Step 1: Add the scaffolding — WaitDelay + a no-op `configureGroupKill` on BOTH platforms (this is the RED baseline)**

This makes the code compile and establishes the spec's red baseline (seam disabled, `WaitDelay` on) so Step 3's test is *genuinely* red.

Create `internal/adapter/exec/procattr_unix.go`:

```go
//go:build !windows

package exec

import "os/exec"

// configureGroupKill — no-op placeholder (Task 1 Step 1); the real Unix
// implementation lands in Step 4. Kept here so the build compiles and the
// SEC1 test (Step 3) is red against a disabled seam.
func configureGroupKill(_ *exec.Cmd) {}
```

Create `internal/adapter/exec/procattr_windows.go`:

```go
//go:build windows

package exec

import "os/exec"

// configureGroupKill is a no-op on Windows: there are no POSIX process groups,
// so the command keeps os/exec's default context-cancel (a process-only Kill).
// Best-effort, CI-unverified (no Windows runner) — mirrors the installer's
// replace_windows.go. WaitDelay (set by the caller) still bounds Wait here.
func configureGroupKill(_ *exec.Cmd) {}
```

In `internal/adapter/exec/exec.go`, add the constant just above `runOne` (near line 126, before the `// runOne …` doc comment):

```go
// waitDelay bounds how long Cmd.Wait blocks after the process exits or the
// context is cancelled before os/exec force-closes the I/O pipes and returns
// (Cmd.WaitDelay, Go 1.20+). It is the safety net for a child that inherited our
// stdout/stderr pipe and outlives the killed command (e.g. one that setsid'd out
// of the group): without it, Wait can hang forever. It never elapses for a
// normally-terminating command.
const waitDelay = 2 * time.Second
```

In `runOne`, wire both in — after `c.Stderr = out` and before `err := c.Run()`:

```go
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = r.dir
	c.Stdout = out
	c.Stderr = out
	configureGroupKill(c) // Unix: own process group + SIGKILL the group on timeout; Windows: no-op
	c.WaitDelay = waitDelay
	err := c.Run()
```

- [ ] **Step 2: Run the existing exec tests — they must stay green (regression guard, AC-2/AC-3)**

Run: `go test ./internal/adapter/exec/ -run 'TestRun|TestCompensate|TestTailBuffer' -race -count=1`
Expected: PASS. (WaitDelay + a no-op seam don't perturb normal completion, the fast-timeout tests, or compensation.)

- [ ] **Step 3: Write the failing SEC1 test and watch it fail (RED)**

Create `internal/adapter/exec/exec_pgid_test.go`:

```go
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
```

Run: `go test ./internal/adapter/exec/ -run TestRunTimeoutKillsProcessGroup -race -count=1 -v`
Expected: **FAIL** — "background child NNN survived the gate timeout". (Run returns after ~2s via WaitDelay; the orphan is still sleeping, so `kill(pid,0)` returns nil, not ESRCH.)

- [ ] **Step 4: Implement the Unix group-kill and watch it pass (GREEN)**

Replace the body of `internal/adapter/exec/procattr_unix.go`:

```go
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
```

Run: `go test ./internal/adapter/exec/ -run TestRunTimeoutKillsProcessGroup -race -count=1 -v`
Expected: **PASS** — Run returns in ~200ms (timeout → immediate group SIGKILL), the orphan is dead, `kill(pid,0)==ESRCH`.

- [ ] **Step 5: Full exec package + build-tag sanity**

Run: `go test ./internal/adapter/exec/ -race -count=1`
Expected: PASS (all existing tests + the new one).

Run: `GOOS=windows go build ./...`
Expected: builds clean (the Windows `configureGroupKill` no-op compiles under its tag).

- [ ] **Step 6: `make check` + commit**

Run: `make check`
Expected: green (gofmt, vet, golangci-lint v2, `go test -race -cover`, build).

```bash
git add internal/adapter/exec/exec.go internal/adapter/exec/procattr_unix.go \
        internal/adapter/exec/procattr_windows.go internal/adapter/exec/exec_pgid_test.go
git commit -m "t14: kill the whole process group on gate timeout (SEC1)"
```

---

## Task 2: docs sync

**Files:**
- Modify: `internal/adapter/exec/CLAUDE.md`, `DESIGN.md`, `DESIGN.ru.md`, `CHANGELOG.md`

**Interfaces:** none (docs only). Do this on `impl_review`'s docs-sync judgment; grep all parallel occurrences first.

- [ ] **Step 1: Update `internal/adapter/exec/CLAUDE.md`**

Under "Responsibilities" (near the shell-seam bullet), add a bullet:

```markdown
- **Process-group kill on timeout (t14/SEC1).** Each command runs as the **leader of its own process group**
  (Unix `Setpgid`); on a timeout the runner `SIGKILL`s the **whole group** (`kill(-pgid)`), so a gate that
  backgrounds a child (`daemon &`, `nohup`) cannot outlive its deadline. `Cmd.WaitDelay` (2s) bounds `Wait`
  against a child that inherited the stdout/stderr pipe (and, as a bonus, closes the former infinite hang of a
  gate that exits 0 but leaves such a child). The group-kill is build-tagged (`procattr_unix.go`); Windows
  (`procattr_windows.go`) is a documented **best-effort no-op** (no POSIX groups; CI-unverified — no runner).
```

- [ ] **Step 2: Update `DESIGN.md` (EN)**

Grep first: `grep -n "per-command timeout" DESIGN.md` → the flow/`Runner` sentence on line 379 ("…the working
directory is the project root; **per-command timeout;** the escape hatch `--no-run`…"). NOTE: the phrase is
line-wrapped in the source ("there's a" ends line 378, "per-command timeout;" starts line 379), so anchor on
the unique fragment `per-command timeout;` (line 385's "per-command timeout is config-driven" has no semicolon
— no ambiguity). Literal find/replace:

Find: `per-command timeout;`
Replace: `per-command timeout (which SIGKILLs the command's **whole process group** on Unix — so a gate that backgrounds a daemon can't outlive its deadline; best-effort on Windows);`

- [ ] **Step 3: Update `DESIGN.ru.md` (RU mirror)**

The RU mirror uses `таймаут на команду`, not the English phrase. Grep: `grep -n "таймаут на команду"
DESIGN.ru.md` → line 384 ("…рабочая директория — корень проекта; таймаут на команду; escape-hatch
`--no-run`…"). Literal find/replace at that parallel occurrence:

Find: `таймаут на команду;`
Replace: `таймаут на команду (на Unix по таймауту SIGKILL'ится **вся группа процессов** команды — фоновый демон, порождённый гейтом, не переживёт дедлайн; на Windows — best-effort);`

- [ ] **Step 4: Update `CHANGELOG.md`**

Under `[Unreleased]`, add to a **Fixed** (create the subsection if absent) entry:

```markdown
- SEC1: a gate/post command's timeout now kills the command's whole process group, not just the top shell —
  a gate that backgrounds a daemon can no longer survive its deadline (Unix; best-effort on Windows).
```

- [ ] **Step 5: Verify `CLI_REFERENCE.md` needs no change**

Run: `grep -n "timeout" CLI_REFERENCE.md CLI_REFERENCE.ru.md`
Expected: the per-command timeout is a config/behavior detail with no user-facing surface change (no new flag),
so **no edit** — the group-kill is transparent. If a timeout description exists that would mislead, add one
parallel clause EN + RU; otherwise leave both untouched. Record the decision in the commit message.

- [ ] **Step 6: `make check` + commit**

Run: `make check`
Expected: green.

```bash
git add internal/adapter/exec/CLAUDE.md DESIGN.md DESIGN.ru.md CHANGELOG.md
git commit -m "t14: docs — process-group kill on gate timeout (DESIGN EN/RU, CHANGELOG, exec CLAUDE)"
```

---

## Acceptance criteria mapping (spec → tasks)

- **AC-1** (orphan killed, true red→green) → Task 1 Steps 3–4.
- **AC-2** (existing timeout tests green) → Task 1 Step 2 + Step 5.
- **AC-3** (no regressions) → Task 1 Step 2 + Step 5.
- **AC-4** (builds both platforms) → Task 1 Step 5 (`GOOS=windows go build`).
- **AC-5** (real live proof) → `impl_review` manual smoke (below).
- **AC-6** (`make check` green + docs) → Task 1 Step 6, Task 2.

## impl_review checklist (per the flow)

- Run the **AC-5 live proof**: a real gate that backgrounds a `sleep` and overruns its timeout; confirm via
  `ps`/`kill -0` the backgrounded PID is gone after mtt returns (the live analogue of AC-1).
- Principles self-check (SOLID/DRY/KISS/TDD), docs-sync judgment (DESIGN EN+RU, CHANGELOG, exec CLAUDE),
  `make check` green.
