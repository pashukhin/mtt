# SEC1 — kill the process group on gate timeout (`t14`)

Status: spec (decision record). Type: task (`t14`). Branch: `task/t14`. Tags: `sec`, `release`.

## Context / problem

A transition's gate/post `commands` run through `core.Runner`, implemented by `internal/adapter/exec`
([exec.go](../../../internal/adapter/exec/exec.go)). `runOne` launches each command as
`exec.CommandContext(ctx, "sh", "-c", <cmd>)` with `ctx` bounded by the per-command (else global
`command_timeout`) deadline. On timeout, `CommandContext` sends **`SIGKILL` to the `sh` process only** —
its default `Cancel` is `Process.Kill()`, which targets a single PID.

That is the SEC1 hole: a gate command that spawns a **background child in the same process group** —
`some-daemon &`, `nohup …`, a build step that daemonizes — leaves that child **orphaned and alive past the
deadline**. The whole point of a timeout is that nothing survives it; today a daemon-spawning gate defeats it.

A **second, coupled defect** surfaces the moment we touch this path: `c.Stdout`/`c.Stderr` are **not**
`*os.File` (the runner passes an `io.MultiWriter`/buffer/`tailBuffer`), so `os/exec` creates an OS pipe and a
copier goroutine, and `Cmd.Wait` blocks until that goroutine finishes — which only happens when the pipe's
write end is closed in **every** process that inherited it, **including the orphaned child**. So even after
`sh` is killed, an orphan holding the inherited pipe would hang `Wait` — and thus hang `mtt`.

Constraints from AGENTS/DESIGN this design must satisfy:

- **Hexagon unchanged.** The fix is entirely inside the `exec` **adapter**; `pkg/mtt`, `core.Runner`, exit-code
  taxonomy, config, and every public signature are **untouched**. No business rule moves into the adapter.
- **Platform isolation via build tags** — the same pattern `t44` used for the self-replacer
  (`replace_unix.go` / `replace_windows.go`): POSIX process groups are Unix-only.
- **No network in tests; hermetic.** The proof (a gate spawns a background child that would outlive its
  parent → after the deadline the child is dead) runs in a temp dir with only shell builtins + `sleep`.
- **TDD, KISS, YAGNI.** One deterministic mechanism, no new config surface.

## User story

Primary user = the coding **agent** (or human) whose gate command starts a background process.

- **US1** — As a user, when my gate exceeds its timeout, mtt kills **everything the gate started**, not just
  the top shell — so a daemon-spawning gate cannot leak a process past its deadline. (no new flags; automatic)

## Decisions

### D1 — Start each gate command as the leader of a new process group (Unix)

Set `c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` before `c.Run()`. The command (`sh -c …`) becomes
the **leader of a fresh process group** whose `pgid == pid`, and every descendant it forks inherits that
group unless it deliberately leaves it (`setsid`). This gives us a single handle — the negative pgid — that
addresses the entire subtree in one signal.

- **Rejected — kill by walking `/proc` children:** racy (children fork between read and kill), Linux-specific
  in a different way, and reinvents what process groups exist for.

### D2 — On timeout, `SIGKILL` the whole group in one shot

Override `c.Cancel` (which `CommandContext` defaults to a process-only `Process.Kill()`) with a group kill:

```go
c.Cancel = func() error {
    err := syscall.Kill(-c.Process.Pid, syscall.SIGKILL) // -pid == "every process in group pid"
    if err == syscall.ESRCH {                            // already gone
        return os.ErrProcessDone                         // os/exec treats this as "nothing to cancel"
    }
    return err
}
```

`-c.Process.Pid` targets the group led by the child (valid because D1 made `pgid == pid`). One `SIGKILL` to
the negative pgid reaps `sh` **and** its descendants. **Single phase, no grace period** (maintainer decision,
brainstorm): a gate timeout is an emergency deadline, not a graceful drain — gates are checkers/build steps,
not long-lived services, so `SIGTERM → wait → SIGKILL` is cost with ~zero ROI here (same spirit as the
cancelled `t41`). This also **matches** today's semantics — `CommandContext` already sends `SIGKILL`; we only
widen the target from the process to its group.

- **`ESRCH → os.ErrProcessDone`:** if the group already exited between the deadline firing and the signal,
  `Kill` returns `ESRCH`; mapping it to `os.ErrProcessDone` keeps `Wait` from surfacing a spurious cancel
  error (documented `os/exec` special case). Any other `Kill` error propagates.

### D3 — Bound `Wait` with `c.WaitDelay` (the pipe-inheritance safety net, cross-platform)

Set `c.WaitDelay = 2 * time.Second` in the shared `runOne` (a package constant, both platforms). Go 1.20+
`WaitDelay` caps the interval between "process exited / context cancelled" and "force-close the I/O pipes and
return from `Wait`". Because D2 kills the **whole group**, the inherited pipe normally closes immediately and
`WaitDelay` never fires; it is the **safety net** for the one case D1/D2 can't reach — a child that
**double-`setsid`s out of the group** — so mtt returns instead of hanging forever (best-effort: that escapee
is by construction outside any group we own; we bound our own wait, we don't chase it).

- Cross-platform because the pipe-inheritance hang is not Unix-specific; on Windows (D4) it likewise prevents
  a wedged `Wait`.
- **2 s** is a fixed constant, not config (YAGNI): it only ever delays the *already-timed-out* error path, and
  only in the rare escapee case.

### D4 — Windows: documented best-effort no-op

New `procattr_windows.go` (`//go:build windows`): `configureGroupKill` is a **no-op** — Windows has no POSIX
process groups, so the command keeps the default `CommandContext` process-only kill. This mirrors `t44`'s
deliberately best-effort, **CI-unverified** Windows path (`replace_windows.go`; no Windows runner). A job-object
implementation (kill the whole tree) is explicitly **out** (maintainer decision, brainstorm): notably more
complex, unverifiable in CI, and against the `t44`-established spirit. Recorded risk. `WaitDelay` (D3) still
applies on Windows, so a wedged inherited pipe cannot hang `Wait` there either.

### D5 — File layout & the seam

- `procattr_unix.go` (`//go:build !windows`) — `configureGroupKill(c *exec.Cmd)`: sets `Setpgid` + the group
  `Cancel` override (D1/D2).
- `procattr_windows.go` (`//go:build windows`) — `configureGroupKill(c *exec.Cmd)`: no-op (D4).
- `exec.go` `runOne` — after `exec.CommandContext(...)` and before `c.Run()`: call `configureGroupKill(c)` and
  set `c.WaitDelay = waitDelay`. **Everything else in `runOne`/`Run`/`Compensate` is unchanged**, so the fix
  covers **both** `Run` (gates) and `Compensate` (rollbacks) for free — they share `runOne`.
- **Return semantics unchanged:** on timeout `ctx.Err() == context.DeadlineExceeded` still holds, so `runOne`
  still returns `(-1, "command … timed out")`; the "failing check is the last element" `core.Runner` contract
  is untouched. (When only `WaitDelay` fires without the context deadline — not our timeout case — `Run`
  returns `exec.ErrWaitDelay`; still an operational `-1`, consistent with the contract.)

### D6 — Dependencies

None new — `syscall` and `os` are stdlib. `c.WaitDelay`/`c.Cancel` are `os/exec` fields available since Go
1.20 (module floor is `go 1.23.1` — comfortably satisfied; no `go.mod` change).

## Scope

**In:** `procattr_unix.go` + `procattr_windows.go` (the build-tagged `configureGroupKill` seam); the two-line
wiring in `runOne` (`configureGroupKill(c)` + `c.WaitDelay`); a hermetic Unix-only test proving an orphaned
child is killed on timeout; the `exec` package `CLAUDE.md` update; docs sync (below).

**Out:**
- **Windows real verification** — implemented as a no-op, the process-only kill stays best-effort (isolated,
  unverified — no runner).
- **Killing a child that `setsid`s out of its group** — unreachable by process-group signalling by
  construction; `WaitDelay` bounds our wait, it doesn't hunt the escapee.
- **`SIGTERM`/grace-period/configurable signal** — rejected (D2); a gate timeout is a hard deadline.
- **Any config/flag/exit-code/contract change** — the fix is adapter-internal.

## Acceptance criteria

1. **Orphan is killed on timeout (unit, Unix, hermetic).** A gate command spawns a background child that
   records its PID and would outlive the parent (e.g. `sh -c 'sleep 30 & echo $! > $DIR/pid; sleep 30'`) with a
   short timeout. After `Run` returns the timeout error, the recorded PID is **dead** — asserted via
   `syscall.Kill(pid, 0) == syscall.ESRCH` (poll briefly to absorb reaping). This test **fails without D1**
   (no `Setpgid` → the orphan survives) — i.e. it is a true red→green for the fix.
2. **Existing timeout behavior preserved (unit).** The current timeout tests (`TestRunTimeout`,
   `TestRunPerCommandTimeoutOverridesGlobal`, `TestRunFallsBackToGlobalTimeout`,
   `TestRunOperationalFailureRecordsFailingCheckLast`) stay green: a timed-out command still yields the
   operational error with the failing `Check{Exit:-1}` **last**.
3. **No regressions in the happy/failed/compensate paths (unit).** All existing `exec` tests
   (pass-through, stop-at-first-nonzero, progress/stream separation, tail echo, compensation) stay green — the
   `WaitDelay`/`Setpgid` additions don't perturb normal completion.
4. **Builds on both platforms.** `go vet` / build succeed for the Unix files; the Windows file compiles under
   `GOOS=windows go build ./...` (build-tag sanity, no runner needed).
5. **Real proof (manual, `impl_review`).** Run a real gate that backgrounds a `sleep` and overruns its
   timeout; confirm via `ps`/`kill -0` that the backgrounded PID is gone after mtt returns — the live analogue
   of AC-1 (per the brief: "the child really died", proven on impl_review).
6. `make check` green. Docs synced (below).

## Testing approach

- **Unit (`internal/adapter/exec`, Unix-only `//go:build !windows`, hermetic):** the AC-1 orphan-kill test
  (temp dir, PID file, `kill(pid,0)` liveness check with a short poll). Reuse the existing table style; no
  network, only `sh`/`sleep`. AC-2/AC-3 are the existing tests kept green.
- **Windows:** `configureGroupKill` compiles under the build tag; not executed (no runner) — recorded, mirrors
  `t44`.
- **Manual smoke (`impl_review`):** AC-5, a live daemon-spawning gate.

## Docs to sync (docs-sync judgment, `impl_review`)

Grep **all** parallel occurrences (EN + RU) before editing — the "parallel occurrences" trap.

- **`internal/adapter/exec/CLAUDE.md`:** a sentence under Responsibilities/Boundaries — each gate command runs
  in its **own process group**; a timeout `SIGKILL`s the whole group (so a backgrounded child cannot outlive
  the deadline); `WaitDelay` bounds `Wait` against an inherited-pipe hang; Windows is best-effort process-only
  (unverified).
- **`DESIGN.md ↔ .ru.md`:** the flow/`Runner` material already says "there's a per-command timeout"; add a
  short clause that the timeout kills the command's **whole process group** (Unix; Windows best-effort), so a
  daemon-spawning gate can't survive it. One parallel clause each (EN + RU) — grep for `timeout`/`Runner`.
- **`CHANGELOG.md`** `[Unreleased]` → **Fixed** (or Security): gate timeout now kills the whole process group,
  not just the top shell (SEC1). (Feeds the `v0.10.0` cut.)
- **`CLI_REFERENCE.md ↔ .ru.md`:** touch only if the timeout is described there; no new surface — likely no
  change. Verify during sync.
- **`AGENTS.md`:** no new rule expected (no convention change).

## Sequencing & tracking (process, not code)

`t14` is `speccing` on `task/t14`. This document is the `speccing` deliverable. Next: commit it, run an
adversarial subagent **spec review**, address findings, then `spec_human_review` (maintainer sign-off) →
`planning` (writing-plans) → `plan_review` → `plan_human_review` → TDD `implementing` (AC-1 red→green) →
`impl_review` (including the AC-5 live proof) → `approved` (auto PR) → merge → `deliver`. Part of the
`v0.10.0` batch (with `t44`, `t28`); unblocks `t42` together with `t28`.
