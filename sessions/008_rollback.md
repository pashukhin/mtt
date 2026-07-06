# 008 — Rollback / compensation

Status: in progress   ·   Branch: `feat/s008-rollback`

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

<What was actually built; deviations from the plan; follow-ups spun out into later sessions.>
