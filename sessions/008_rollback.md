# 008 — Rollback / compensation

Status: done   ·   Branch: `feat/s008-rollback`

## Target

Give a **partially-applied transition the ability to undo its own side effects**: a command may declare a
**rollback** command, and when a *later* command in the same pipeline fails, the already-succeeded commands'
rollbacks run in **reverse order** (intra-pipeline compensation). Killer shape (git is one user example, not
ours): `git checkout -b task/{{.ID}}` with `rollback: git branch -D task/{{.ID}}` — a later failed gate leaves
the task blocked *and* removes the branch it created.

## Scope

- **In:**
  - `pkg/mtt`: additive `Command.Rollback *Command` (a leaf compensator: its own `Rollback` must be nil,
    enforced by `Valid()`); `Config.Validate` covers it (existing loop, no signature change).
  - `adapter/yaml`: `ymlCommand.Rollback *ymlCommand` — recursive `UnmarshalYAML` (scalar **or** `{run,
    timeout}`) + recursive `toDomain` (pure copy).
  - `core`: `expandCommands` expands `Rollback.Run` too, **eagerly** (before the gate; same `cmdContext`,
    pre-move `.From`); `Runner` port gains **`Compensate([]Command) []Check`** (best-effort); `Transitioner`
    computes the reversed succeeded-only rollback plan and runs it on a block — task **unchanged**, **no
    history**, exit **3** preserved, block message carries a `compensated N …` summary.
  - `adapter/exec`: best-effort `Compensate` (prints `↩ compensating (N)` + per-command progress, never stops,
    operational failure → `Exit -1`); `runReport` extracted and shared with `Run` (DRY).
  - `cli`: `mtt types` shows a command's `↩ <rollback>`; block summary surfaced.
  - Version `0.7.0-dev` → `0.8.0-dev`.
- **Out (PARKED, do not build):** multi-step / `--atomic` / `advance` compensation across several edges;
  second-level compensation (a compensator's compensator); a durable side-effect audit of compensation;
  forcing `--who`/`--why` on rollback-risky ops; node-level status actions; roles-on-edges.

## Open questions (resolved in brainstorm — see the spec)

Spec: [../docs/superpowers/specs/2026-07-06-session-008-rollback-design.md](../docs/superpowers/specs/2026-07-06-session-008-rollback-design.md).

1. **Form** — per-command `Command.Rollback` (not per-transition): maps 1:1 to "reverse over the succeeded".
2. **Locus** — `core.Transitioner` orchestrates (knows which succeeded, computes the plan); the exec adapter
   executes it (`Compensate`). Non-zero exit is data; the Runner stays dumb.
3. **Rollback's own failure** — best-effort (run all compensators, continue past failures); the outcome stays
   `ErrBlocked`/exit 3 (never masks the gate failure); failures surfaced in progress + block summary.
4. **Audit** — **no** `history` on a blocked+compensated transition (task file untouched; `History` is a
   *transition* journal, compensation is a side-effect event). Live progress + block message only.
5. **Placeholders** — same `cmdContext{ID,Type,From,To}`, expanded **eagerly** up front (a bad rollback
   template → exit 1 before any side effect).
- **Refinements:** (B) the e2e proves compensation for **arbitrary** commands (file sentinels, no git guard);
  (C) the exec `Runner` gains a dedicated best-effort `Compensate` that owns the labeled `↩` phase.

## Acceptance (must pass)

- **User scenario:** `mtt init` → a gate config on `tbd → in_progress` with a first command that has a side
  effect + `rollback`, and a later command that fails → `mtt add A` (t1) → `mtt in_progress t1` **blocks**
  (exit 3, task stays `tbd`, no history) **and** runs the compensator so the side effect is undone; the live
  progress shows `↩ compensating (N)` and the error shows `compensated N commands`.
- **e2e:** `testscript` scenario `rollback.txt` — generic commands (`touch a-{{.ID}}` / `rm a-{{.ID}}`, a
  second sentinel, then `false`): after a blocked `in_progress`, both sentinels are gone, the task is `tbd`,
  no history, and the `↩ compensating` phase is shown. No `[exec:git]` guard.
- `make check` green.

## Plan (refine at session start — test-first; brainstorm → writing-plans)

Brainstorm done (five decisions + two refinements locked in the spec). Green between commits (s007's
behavior-preserving-slices lesson):

- [ ] `pkg/mtt`: `Command.Rollback` + `Valid()` leaf invariant + `Config.Validate` coverage.
- [ ] `adapter/yaml`: `ymlCommand.Rollback` recursive unmarshal + `toDomain`.
- [ ] `core`: `expandCommands` rollback expansion (`expandOne`/`expandTemplate` refactor).
- [ ] `core` + `exec`: `Runner.Compensate` (port + exec `runReport`/`Compensate` + fake).
- [ ] `core`: `Transitioner` compensation on block (`firstFailure`-index / `rollbacksBefore` / `compSummary`).
- [ ] `cli`: `mtt types` `↩` + block summary; e2e `rollback.txt`; unit tests per the spec.
- [ ] Docs: DESIGN.md/.ru, CLI_REFERENCE.md/.ru, CLAUDE.md ×5, model.go, TASKS.md, sessions/README.md,
      NEXT_SESSION.md; bump `0.7.0-dev` → `0.8.0-dev`.

## Done (fill during/after the session)

Shipped (all test-first, `make check` + acceptance e2e green), version `0.7.0-dev` → `0.8.0-dev`. Spec:
[../docs/superpowers/specs/2026-07-06-session-008-rollback-design.md](../docs/superpowers/specs/2026-07-06-session-008-rollback-design.md);
plan: [../docs/superpowers/plans/2026-07-06-session-008-rollback.md](../docs/superpowers/plans/2026-07-06-session-008-rollback.md).

- **`pkg/mtt`**: additive `Command.Rollback *Command` (`command.go`) — an optional per-command compensator;
  `Valid()` now also validates a **leaf** rollback (non-empty run, non-negative timeout, its own `Rollback ==
  nil`), so `Config.Validate` rejects a second-level `rollback.rollback` (on `add`/`types`).
- **`internal/adapter/yaml`**: `ymlCommand.Rollback *ymlCommand` — the `rollback` field is itself a scalar or
  `{run, timeout}`, decoded through the same recursive `UnmarshalYAML`; a new recursive `ymlCommand.toDomain()`
  **deep-copies** the rollback (a fresh `*mtt.Command`, not the DTO pointer).
- **`internal/core`**: `expandCommands` refactored to `expandOne`/`expandTemplate` — expands `Run` **and**
  `Rollback.Run` **eagerly** (same `cmdContext`, pre-move `.From`), so a malformed rollback template aborts as
  exit 1 before any side effect. The `Runner` port gained **`Compensate([]mtt.Command) []mtt.Check`** (with a
  documented `Run` CONTRACT: operational failure records the failing `Check` last). `Transitioner` computes the
  compensation plan from a **single failure index** (`firstFailure` for a non-zero check; `len(checks)-1` for an
  operational error) and runs the **succeeded-prefix** rollbacks **in reverse** (`rollbacksBefore`) via
  `Compensate`; the block error carries a `compSummary` (`compensated N …`). Outcome unchanged: `ErrBlocked`
  (exit 3), task untouched, **no history**.
- **`internal/adapter/exec`**: best-effort `Compensate` (prints `↩ compensating (N)` + per-command lines, never
  stops, operational failure → `Exit -1`); `runReport` extracted and shared with `Run` (DRY).
- **`internal/cli`**: `mtt types` renders `↩ <rollback>` (+ its own `(timeout <d>)`); no other wiring change —
  the compensation phase and block summary flow through the existing progress/error paths (stderr, exit 3).
- **Tests**: unit — `Command.Valid` (leaf), `Config.Validate` (nested rollback), `ymlCommand` rollback
  scalar/map + `toDomain` deep-copy, `expandCommands` rollback (+ malformed → exit-1, nil-stays-nil),
  `Transitioner` (reverse over succeeded, first-fail → none, operational-error path, best-effort compensator
  failure keeps `ErrBlocked` + no history), `exec.Compensate` (best-effort/empty/per-command-timeout),
  `mtt types` `↩`. e2e — `rollback.txt` (generic `touch`/`rm`/`false`, no git guard): a blocked `in_progress`
  removes both sentinels, shows `↩ compensating (2 commands)` + `compensated 2 commands`, task stays `tbd`.

**Deviations / notes:**
- The spec's "enforced at Load" claim was corrected during the subagent review — `Config.Validate` runs on
  `add`/`types`, not `yaml.Load` nor the gate path (the s006/s007 status quo); a stray second-level rollback on
  the gate path is harmlessly ignored at runtime (`expandOne` recurses, `rollbacksBefore` reads one level).
- Succeeded/failed derivation was hardened to a single `failIdx` source (subagent Issue 2), so the failed
  command's rollback is never run even if a future Runner does not stop at the first non-zero.
- `Runner.Compensate` (one new port method) was chosen over a core-side `Run`-per-command loop so the labeled
  `↩ compensating` phase lives with the progress writer (the exec adapter) while core still computes the plan.

**Next:** s008.5 dogfood enablers (`mtt rm`, `--depends-on` on `add`, packaging `make install`).
