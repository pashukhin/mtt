# Session 006 — Flow gate (the killer feature) — design

Date: 2026-07-05 · Branch: `feat/s006-flow-gate` · Status: approved (+ addendum below)

> **Addendum (post-manual-review):** gate execution reports **live pipeline progress** (always) and
> optionally streams **command output** behind flags; the subject-identity `by` also reads a durable
> `config.local.yaml` `author`. See "Addendum — live gate output & config.local author" at the end.

## Goal

Make a task's status transition **executable and gated**: `mtt status <id> <new>` moves the task across
**one** edge, validated against the type's `transitions`, running that edge's `commands` (all must exit `0`,
else the move is **blocked**), and appending a `history` entry (`from→to`, `at`, `by`, `role`, `checks`).
This is mtt's wedge — a deterministic workflow-enforcement layer: an agent can't declare a new status
without passing the gate.

Scope is deliberately **one edge**. The meta-walk (`advance`/`start`/`done`, multi-edge) is s007.

## Key finding: the domain model is already in place

`pkg/mtt` already carries `HistoryEntry` (`At/By/Role/From/To/Checks`), `Check` (`Cmd/Exit`), and
`Transition.Commands`; the YAML adapter already serializes them (`ymlHistoryEntry`/`ymlCheck`,
`fromDomainHistory`/`toDomainHistory`). **s006 changes no `pkg/mtt` type.** It adds a driven port, an
adapter, a core usecase, a CLI command, and an execution-timeout config field.

## Architecture (dependencies point inward: cli → core → port ← adapter)

### 1. `core` — the `Runner` port + flow sentinel errors

```go
// Runner executes a transition's commands and reports each result. Defined in
// core (only core uses it); implemented in internal/adapter/exec; faked in tests.
type Runner interface {
	Run(commands []string) ([]mtt.Check, error)
}

var ErrBlocked           = errors.New("mtt: transition blocked by a failed gate")
var ErrInvalidTransition = errors.New("mtt: transition not allowed by the flow")
```

- **Deviation from `docs/architecture/model.go`:** the port method is `Run(commands)` — **no `dir`
  parameter**. The exec adapter is constructed with `cwd = project root`, so `core` never handles a
  filesystem path (cleaner hexagon). `model.go` is updated to match.
- **Sentinels live in `core`**, not `pkg/mtt`: flow validation and gating are core policy. (`ErrNotFound`
  is in `pkg/mtt` because it is a port contract; a blocked/invalid transition is a usecase outcome.) The
  CLI matches them with `errors.Is` to pick an exit code.

### 2. `core.Transitioner` — the single-edge usecase

```go
type TransitionOptions struct {
	Role  string
	By    string
	NoRun bool
}

func NewTransitioner(store mtt.TaskStore, cfg mtt.Config, runner Runner, now func() time.Time) *Transitioner
func (tr *Transitioner) Transition(id mtt.TaskID, to mtt.StatusName, opts TransitionOptions) (mtt.Task, error)
```

Algorithm:

1. Load the task (`store.Get`; `ErrNotFound` → wrapped "task %q not found").
2. Resolve its type from `cfg` (`TypeByName`); an unknown type is an error (config drift).
3. **Single-edge lookup** (no `ResolvedFlow` — YAGNI for one edge; it earns its keep in s007's multi-edge
   walk): linear scan of `typ.Transitions` for `From == task.Status && To == to`. No such edge →
   `ErrInvalidTransition`, wrapped with a message listing the allowed targets from the current status.
4. Gate:
   - `opts.NoRun` → skip commands; `checks = nil`.
   - else `checks, err := runner.Run(edge.Commands)`. An operational `err` (command couldn't launch /
     timed out) or **any** `Check.Exit != 0` → `ErrBlocked`, wrapped with the failing command + its exit.
     **On a block the task is not changed and no history is written.**
   - an edge with no `commands` passes vacuously (`checks` empty).
5. Success: set `task.Status = to`; append `HistoryEntry{At: now, By: opts.By, Role: opts.Role,
   From: <old>, To: to, Checks: checks}`; bump `Updated = now`; persist via `store.Update`; return the task.

Signature mirrors the anticipated `NewAdvancer(store, cfg, runner, now)` so s007 slots in beside it.

### 3. `internal/adapter/exec` — the Runner implementation (first driven port beyond storage)

```go
func NewRunner(dir string, timeout time.Duration) *Runner
func (r *Runner) Run(commands []string) ([]mtt.Check, error)
```

- Each command runs via `exec.CommandContext(ctx, name, args...)` where `(name, args)` comes from a
  **platform seam** `shell(cmd)`:
  - Windows (`runtime.GOOS == "windows"`) → `cmd`, `["/c", cmd]`
  - otherwise → `sh`, `["-c", cmd]`
  Commands are trusted project config (like a Makefile / git hooks), never network input.
- `cwd = dir` (the project root, injected at construction). Each command gets its **own** `timeout` via a
  per-command `context.WithTimeout`.
- Runs in order; **stops at the first non-zero exit**; records a `mtt.Check{Cmd, Exit}` for every executed
  command. A non-zero exit is **data**, not a Go error. A launch failure or timeout returns the checks so
  far plus a non-nil `error` (the failing command's `Check.Exit` set to a non-zero sentinel).
- Tested directly (`true` → exit 0; `false` → stops at exit 1; a `sleep` beyond a tiny timeout → timeout
  error). The Windows branch is documented, not CI-tested (CI is Linux).
- In `core` tests, `Runner` is a **fake** (canned checks / injected error) — no real process spawned.

### 4. Execution timeout — config-driven (not hardcoded)

The per-command timeout is an **execution/adapter policy** (like `prefix`), not domain — an external
tracker adapter runs no commands. So it lives in the YAML-adapter config, and `pkg/mtt.Config` stays pure.

- New top-level key in `.mtt/config.yaml`: `command_timeout: 5m` (a `time.ParseDuration` string). Absent →
  built-in default **5m**. Overridable via the gitignored `config.local.yaml` overlay (existing mechanism).
- Surfaced through the existing config layer: `Load` already returns adapter-specific data (`prefixes`)
  alongside the pure `mtt.Config`; widen that into a small `Settings{Prefixes map[string]string;
  CommandTimeout time.Duration}` value. `toDomain()` does not touch the domain `Config`. Existing `Load`
  callers (`Store.Create`) read `Settings.Prefixes`.
- The `default` and `coding` templates gain an explicit `command_timeout: 5m` (golden tests updated).

### 5. CLI — `mtt status <id> <new>` + global role/by flags + exit codes

- `newStatusCmd()`: `Args` = exactly two (`<id> <new-status>`); flag `--no-run` (bypass gates).
- New **root persistent** flags (global seam so s007 `advance`/`start`/`done` inherit them, no retrofit):
  - `--role` (env `MTT_ROLE`) — the acting role, recorded in `history.role`.
  - `--by` (env `MTT_BY`) — the acting subject, recorded in `history.by`. A **minimal** subject-identity
    seam; the durable source (`.mtt/config.local.yaml` author field) + edit-audit stays deferred (GAP #5).
    A resolver mirrors `resolveDir`: flag, else env, else empty.
- Wiring: `projectRoot` → `Load` for `cfg` + `Settings.CommandTimeout` → `exec.NewRunner(root, timeout)` →
  `core.NewTransitioner(store, cfg, runner, time.Now)` → `Transition(id, to, opts)`.
- Output: human `t1: tbd → in_progress` (plus one line per executed check, e.g. `✓ make lint (exit 0)`);
  with `--json`, echo the resulting task via the shared `taskJSON` view (includes `history`).
- **Exit codes** (`CLI_REFERENCE` proposal, now realized where these states first appear):
  - `Execute()` changes from `error` to **`int`**; `main` calls `os.Exit(cli.Execute())`.
  - mapping: `errors.Is(err, core.ErrBlocked)` → **3**; `core.ErrInvalidTransition` → **6**; any other
    error → **1**; success → **0**. Centralizes exit-code policy in one place.

### 6. `mtt show` — render the history/audit section

Add a compact `history:` block to `formatTask` (the audit trail is now real and valuable to humans):
one line per entry — `at`, `from → to`, `by`/`role` when present, and a checks summary
(e.g. `checks: make lint(0) make test(0)`). Omitted when the task has no history. JSON already carries it.

## cancelled-blocker semantics (the s005 deferred question)

Now that `status` can move a task to a `terminal` (`cancelled`), a dependent whose blocker is `cancelled`
becomes **ready** under the current `kind`-based `Ready` (terminal-by-`kind` unblocks). **Decision: keep
this behavior for s006** — a correct fix (a succeeded-vs-abandoned distinction, hard/soft edges) needs new
domain modeling (an edge/status attribute) that is out of a flow-gate session's scope and would risk the
name-agnostic principle. s006 makes the state **reachable** and adds an e2e that demonstrates the current
semantics (the s005 lesson: prove it now that a real transition can produce a terminal). The deeper fix
stays a documented, deferred slice (DESIGN → Dependencies; TASKS → Later).

## Testing (test-first)

- **Unit — `core.Transitioner`** (fixed clock, fake Runner):
  - valid transition applies, records `history` (`from/to/at/by/role/checks`), bumps `updated`;
  - a gate returning non-zero → `ErrBlocked`, task unchanged, no history, no write;
  - an unknown edge → `ErrInvalidTransition`;
  - `--no-run` applies without invoking the runner (fake asserts it was not called);
  - `by`/`role` propagate into the entry; an edge with no commands passes with empty checks.
- **Unit — `exec.Runner`**: `true` → `[{true,0}]`; `["true","false","true"]` → stops after `false`
  (`[{true,0},{false,1}]`, `ErrBlocked`-worthy exit); a command exceeding a 1ms timeout → non-nil error.
- **Unit — config**: `command_timeout` parses; absent → 5m default; `config.local` override wins;
  templates round-trip (golden update).
- **e2e — `status.txt`**: `init` → overwrite `.mtt/config.yaml` with commands on edges (`tbd→in_progress:
  ["true"]`, `in_progress→done: ["false"]`) → `add` → `status t1 in_progress` (green gate, moves) →
  `status t1 done` (red gate → exit 3, stays `in_progress`) → `status t1 done --no-run` (bypass → `done`) →
  an invalid transition (exit 6) → assert `history` via `--json` / `show`.
- **e2e — cancelled unblock** (own script or an arm of `status.txt`): `dep add t2 t1` → `status t1
  cancelled` → `ready` now lists `t2` (documents the kept cancelled-unblocks semantics with a reachable
  state).
- `make check` green (fmt + vet + golangci-lint v2 + `go test -race -cover` + build).

## Docs to update

- **DESIGN.md / DESIGN.ru.md** — flow enforcement now shipped (`mtt status`, `Runner`, exit codes,
  `command_timeout`); reiterate the cancelled-blocker decision.
- **CLI_REFERENCE.md / .ru** — `mtt status` implemented; exit codes 3/6 implemented; `--role`/`--by`
  (`MTT_ROLE`/`MTT_BY`); `--no-run`; `command_timeout` under Configuration.
- **CLAUDE.md** ×3 — `internal/core` (`Runner`/`Transitioner`), `internal/cli` (`status`, role/by, exit
  codes), **new** `internal/adapter/exec`.
- **docs/architecture/model.go** — `Runner` signature (no `dir`), `Transitioner` shipped, GAP #5 (`By`
  partial via `--by`/`MTT_BY`; durable source still open), exit-code taxonomy realized.
- **TASKS.md** — tick e4_t1 (validate transition), e4_t2 (`Runner` + `exec` + fake), e4_t3 (run commands +
  `--no-run`), e4_t4 (`history`); note e4_t5 is partial (single-edge `status` done; the meta-walk is s007).
- **sessions/006_flow_gate.md** — filled Done; **sessions/README.md** — 006 marked in progress → done.
- **NEXT_SESSION.md** — handoff to s007 (advance/start/done, `ResolvedFlow`), carry-over lessons.

## Out of scope (explicitly deferred)

- `advance`/`start`/`done` and the multi-edge walk, modes `--stop`/`--atomic`/`--force`, `ResolvedFlow` → **s007**.
- Packaging (`make install` → `go install ./cmd/mtt` + a smoke test) → a **separate small chore-PR** after s006.
- The durable, git-independent edit-audit trail and the config.local subject-identity source (GAP #5 beyond
  the minimal `--by`/`MTT_BY`).
- A real fix for cancelled-blocker semantics (hard/soft edges) — documented, deferred.
- Per-transition timeout override, rollback/compensation commands, `mtt caps`.

## Addendum — live gate output & config.local `author` (post-manual-review)

Manual e2e surfaced two gaps: on a blocked transition you couldn't see which commands **passed** (the CLI
printed a post-hoc `✓` summary and swallowed the commands' own output), and `by` had no durable source.

### 1. Live pipeline progress (always) vs command output (opt-in)

Two separate concerns, previously conflated:

- **Pipeline progress** — which command is running and its outcome — is **always** shown, **live**, on
  **stderr**, so a blocked run shows the passed `✓` lines and the failing `✗` before it stops:
  ```
  ▶ make lint
  ✓ make lint (exit 0, 1.2s)
  ▶ make test
  ✗ make test (exit 1, 0.4s)
  ```
  Timing is per-command elapsed (real `time.Now()` in the exec adapter, display-only — **not** persisted to
  `history.checks`).
- **Command output** (each command's own stdout/stderr) is **opt-in** (hidden by default, as before):
  - `-v` / `--verbose` — stream it live to stderr;
  - `--log-file <path>` — write it to a file (create/truncate); with `-v`, tee to both (`io.MultiWriter`).

`exec.NewRunner(dir, timeout, progress, cmdOut io.Writer)`: `progress` is always the CLI's stderr; `cmdOut`
is `io.Discard` / stderr / a file / a multi-writer, chosen by the flags. The exec adapter prints the `▶`/`✓`/
`✗` progress lines (it alone knows real-time) and wires `cmd.Stdout=cmdOut; cmd.Stderr=cmdOut`. The port
method `Run(commands)` is unchanged (writers are baked in at construction, like `dir`/`timeout`). The CLI
drops its post-hoc `✓` summary and prints only the domain outcome (`t1: from → to`, or JSON) on **stdout**,
so `--json` stdout stays clean (progress on stderr).

### 2. `by` from `config.local.yaml` `author`

The durable subject-identity source (GAP #5, minimal): resolve `by` as `--by` > `MTT_BY` >
`config.local.yaml` `author:` > empty. `role` stays flag/env only (it is "what hat *now*", per-invocation,
not a personal constant). `author` lives **only** in the gitignored personal overlay — it is not added to the
committed templates. Surfaced via the adapter `Settings.Author` (from `Load`); `resolveRoleBy` takes the
author default. The remaining durable edit-audit trail stays deferred.

### Tests (addendum)

- `exec.Runner`: progress lines (`▶`/`✓`/`✗ … (exit N, …)`) go to the `progress` writer; command output goes
  to `cmdOut` only (assert a captured buffer sees it and the progress buffer does not, and vice-versa);
  durations are not asserted exactly (prefix match).
- CLI: `-v` routes command output to stderr; `--log-file` writes it to the file; `by` falls back to
  `config.local` `author` when `--by`/`MTT_BY` are unset (precedence).
- e2e `status.txt`: assert a `✗` progress marker on **stderr** for the blocked edge (passed `✓` visible);
  an added arm writes a `config.local.yaml` with `author:` and asserts `mtt show` history records that `by`.
