# 009 — Dogfood / self-host

Status: planned   ·   Branch: `feat/s009-dogfood`

## Target

Make mtt **track its own development**: `mtt` this repository with a hand-authored config whose gates are
**task-aware** (a session-branch on `→ in_progress` via `{{.ID}}`, `make check` on `→ done`), and migrate the
**forward** (open) backlog onto committed `.mtt/tasks/*.yaml`. After s009, `TASKS.md` freezes and mtt is the
live queue. **Not a normal CLI feature** — integration + config + data + docs, ~no production logic change.

## Scope

- **In:**
  - Hand-authored **`.mtt/config.yaml`** (committed): custom types `phase` (`p`, root) / `session` (`s`,
    `default`, `parents:[phase]`) / `step` (`t`, `parents:[session]`). **Session gated:** `→in_progress`
    `git checkout -b feat/{{.ID}}` + `current:set`; `→done` `make check` + `current:clear`. **Step gated:**
    `→in_progress` `current:set` (no branch); `→done` `make check` + `current:clear`. **Phase:** `current
    set|clear` only, no commands. `command_timeout: 10m`.
  - **Forward backlog migrated** to committed `.mtt/tasks/*.yaml` (via `./bin/mtt add …`): Phase-4 epic +
    sessions (references **high** / comments / actor-profiles / coding-demo **low**); bare Phase-5…8 epics.
    All `tbd`; ordering via **priorities**; `current` unset.
  - **`TestRepoDogfoodConfig`** — Go test guarding the committed config (FindRoot repo `.mtt/`, Load+Validate,
    assert 3 types + session gates on `make check`/`feat/`).
  - **e2e `dogfood.txt`** — scratch config (fake commands) proving branch-on-`→in_progress` + gate-on-`→done`
    (block/allow + `current` clear) + `[!exec:git] skip`.
  - Docs sync + version `0.8.9-dev → 0.9.0-dev`.
- **Out (deferred):** migrating completed sessions / think-items / parked work (stay in docs); a new embedded
  template / `mtt init --template mtt`; bulk-transition migration; monotonic-id / scale-stress; changing the
  s009 branch workflow itself (bootstrap on manual `feat/s009-dogfood`).

## Decisions (brainstormed — see the spec)

Spec: [../docs/superpowers/specs/2026-07-09-session-009-dogfood-design.md](../docs/superpowers/specs/2026-07-09-session-009-dogfood-design.md).
Plan: `docs/superpowers/plans/2026-07-09-session-009-dogfood.md` (written next).

1. **Q1 — forward-only migration.** Only open work (Phase-4 sessions + bare Phase-5…8 epics). Completed
   sessions, think-items, and parked work stay in git/docs. mtt = a live queue, not an archive. s009 is not a
   task (it is the migration act).
2. **Q2 — custom `phase`/`session`/`step`** (prefixes `p`/`s`/`t`, non-overlapping; fresh id namespace). `session`
   is the `default` (primary planning unit); a root `phase` adds freely.
3. **Q3 — honest task-aware gates.** Session: branch on `→in_progress` (`feat/{{.ID}}`) + `make check` on
   `→done`. **Step also gates `make check`** on `→done` (every step green — AGENTS "make check before every
   commit"). Phase: current only. Bootstrap caveat documented (mtt ids ≠ doc `sNNN`; no slug in branch — placeholder
   whitelist; s009 runs on the manual branch, config governs future sessions).

## Acceptance (must pass)

- **User scenario:** in the repo, `mtt types` shows the three gated types; `mtt list`/`tree`/`roadmap` render the
  migrated Phase-4 hierarchy + open sessions.
- **Committed-config guard:** `TestRepoDogfoodConfig` green (genuine red→green).
- **e2e:** `testscript` `dogfood.txt` — branch on `→in_progress`, `→done` blocked on a failing gate (exit 3,
  unchanged) / moved on a passing gate (`current` cleared), `[!exec:git] skip`.
- `make check` green.

## Plan (refine at session start — test-first)

- [ ] (written by writing-plans next)

## Done (fill during/after the session)

<pending>
