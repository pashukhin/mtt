# 006 — Flow gate (the killer feature)

Status: in progress   ·   Branch: `feat/s006-flow-gate`

## Target

Make a status transition **executable and gated**: `mtt status <id> <new>` moves the task across **one**
edge, validated against the type's `transitions`, running that edge's `commands` (all → `0`, else the move
is **blocked**), and appending a `history` entry (`from→to`, `at`, `by`, `role`, `checks`). The first
driven port beyond storage — `Runner` (defined in `core`, implemented in `internal/adapter/exec`, faked in
tests). This is mtt's wedge: an agent can't declare a new status without passing the gate.

## Scope

- **In:**
  - **`mtt status <id> <new>`** — a single transition validated against `Type.Transitions`, running that
    edge's `commands` (gate: all exit `0`, else blocked, task unchanged, no history), appending `history`.
    Flags: `--no-run` (bypass gates). Human output + `--json` (echoes the task with history).
  - **`core.Runner`** port + **`core.ErrBlocked`/`ErrInvalidTransition`** sentinels; **`core.Transitioner`**
    usecase (single-edge lookup — no `ResolvedFlow`; gate via `Runner`; append `history`; persist via
    `TaskStore.Update` — **no new store port**, history rides the `Task.History` field like `depends_on`).
  - **`internal/adapter/exec`** — `Runner` over `os/exec`: per-command timeout, `cwd = project root`,
    cross-platform shell seam (`sh -c` / `cmd /c`), stop at first non-zero. Fake in `core` tests.
  - **`command_timeout`** config key (YAML-adapter config; default 5m; `config.local`-overridable) — the
    per-command timeout is execution/adapter policy, `pkg/mtt` stays pure.
  - **Global seam flags** `--role`/`MTT_ROLE` and `--by`/`MTT_BY` (root persistent) recorded into `history`.
  - **Exit codes** `3` (gate blocked) and `6` (invalid transition); `Execute()` → `int`.
  - `mtt show` renders a compact `history:` audit section.
- **Out (explicitly deferred):**
  - `advance`/`start`/`done`, the multi-edge walk, modes `--stop`/`--atomic`/`--force`, `ResolvedFlow` → **s007**.
  - Packaging (`make install`) → a separate small **chore-PR** after s006.
  - Durable edit-audit + the config.local subject-identity source (GAP #5 beyond minimal `--by`/`MTT_BY`).
  - A real fix for cancelled-blocker semantics (hard/soft edges) — kept as-is, documented, deferred.

## Acceptance (must pass)

- **User scenario:** `mtt init` → put `commands` on edges in `.mtt/config.yaml` (`tbd→in_progress: ["true"]`,
  `in_progress→done: ["false"]`) → `mtt add A` (t1) → `mtt status t1 in_progress` (green gate → moves) →
  `mtt status t1 done` (red gate → **blocked**, exit `3`, stays `in_progress`) → `mtt status t1 done
  --no-run` (bypass → `done`) → an unallowed transition exits `6`; `history` is visible via `mtt show` /
  `--json`. Cancelling a blocker (`mtt status t1 cancelled`) makes its dependent `ready`.
- **e2e:** `testscript` `status.txt` covering the above (green/red gate, `--no-run`, invalid edge, history)
  + the cancelled-unblock arm.
- `make check` green.

## Plan (refine at session start — test-first; brainstorm → writing-plans)

Design decisions resolved in brainstorm — authoritative spec:
[../docs/superpowers/specs/2026-07-05-session-006-flow-gate-design.md](../docs/superpowers/specs/2026-07-05-session-006-flow-gate-design.md).
Summary: model already carries `HistoryEntry`/`Check` (no `pkg/mtt` change); `Runner` port in `core`
(`Run(commands)` — no `dir`, exec adapter holds `cwd`); `Transitioner` does a single-edge lookup (no
`ResolvedFlow` yet); timeout is config-driven (`command_timeout`, adapter-level); exit codes 3/6 realized;
`--role`/`--by` global seams; cancelled unblocks (kept + documented).

- [x] Brainstorm the open questions — resolved (see the spec above).
- [ ] `pkg/mtt`/`core`: `Runner` port + `ErrBlocked`/`ErrInvalidTransition`; `Transitioner` (validate edge,
      gate via Runner, append history, `Update`); fake Runner in tests.
- [ ] `internal/adapter/exec`: `Runner` over `os/exec` (per-command timeout, cwd=root, shell seam); unit test.
- [ ] `internal/adapter/yaml`: `command_timeout` in the config DTO + `Settings{Prefixes, CommandTimeout}`
      from `Load` (default 5m); templates + golden update.
- [ ] `internal/cli`: `mtt status <id> <new>` (`--no-run`); root `--role`/`--by` (+ env); `Execute()` → `int`
      exit-code mapping (3/6); `mtt show` history section.
- [ ] `testscript` `status.txt` (+ cancelled-unblock); docs (DESIGN/.ru, CLI_REFERENCE/.ru, CLAUDE.md ×3,
      model.go, TASKS.md, sessions/README.md, NEXT_SESSION.md); bump `0.5.0-dev` → `0.6.0-dev`.

## Done (fill during/after the session)

<filled during/after the session>
