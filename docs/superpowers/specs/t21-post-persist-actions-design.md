# t21 — post-persist flow actions (design spec)

- **Task:** `t21` (core/flow, high) — *post-persist flow actions: auto-commit `.mtt` after a move.*
- **Date:** 2026-07-11
- **Status:** spec (speccing → spec_review)
- **Author:** Grigorii Pashukhin

## 1. Motivation

A transition persists the new status to `.mtt/tasks/<id>.yaml` (`core.Transitioner` → `store.Update`), but it
does **not** commit that change. So every mid-flow move (`start`/`submit`/`approve`/`decline`) is followed by a
manual `git add .mtt && git commit` — ~12 times in the s009 t5 session — and `deliver`/`cancel` carry two
explicit "manual steps" (AGENTS.md) to commit the terminal state on `main`.

The gate commands (`commands:`) run **before** the persist, so no gate can commit the *new* state — a commit
placed there would capture the *old* status. t21 closes this with a **post-persist phase**: commands that run
**after** the status is written, so a move commits its own `.mtt` change.

This is the first concrete case of the AGENTS.md prime directive (TL;DR #0): *mechanize the process into mtt.*

## 2. Decisions (from brainstorming)

| # | Decision |
|---|----------|
| D1 | **General mechanism**, not a bespoke auto-commit: arbitrary commands after persist (auto-commit is one use). |
| D2 | **Per-edge `post:`** in config, symmetric to `commands:` (reuses the `Command` VO). |
| D3 | **Two phases:** pre `commands:` (existing — blocks *entry*, persist doesn't happen on failure, succeeded-prefix compensation) **+** post `post:` (new — runs after entry, **finalization**). |
| D4 | **Post failure → surface, no rollback.** The move already happened (pre-gates passed, status persisted); a post failure returns a distinct sentinel `ErrPostAction`, the status is **kept**, exit code **5**. |
| D5 | **Architecture A:** `core` owns the post phase (reuses `expandCommands` + `Runner`); on post failure `Transition` returns `(persistedTask, ErrPostAction)` — task is valid; the CLI prints the successful move + surfaces the post error + maps exit 5. |
| D6 | **Deferred → t24:** a global/default `post` vs per-edge (precedence/merge/opt-out) is out of scope; per-edge only. |

**Why two phases (the load-bearing insight).** A pure "persist-then-run-everything, roll back on failure"
model breaks on `git switch`: `deliver`/`start`/`cancel` must switch branch **before** the persist (else the
status is written on the wrong branch — e.g. `deliver` would write `done` on `task/<id>` instead of `main`).
So context/gate commands must precede persist; only finalization (`git commit`) can follow it. Rollback for
the **pre** phase already exists (s008 compensation); the **post** phase is deliberately non-transactional.

## 3. Domain model (`pkg/mtt`)

- `Transition.Post []Command` — new field, **reusing** the existing `Command` value object (`{Run, Timeout,
  Rollback}`). Empty = no post phase. Placed after `Require` in the struct.
- `Config.Validate` extends the transition check to `Post[].Valid()` (same rule as `Commands`). A post
  command's `Rollback` is **ignored at run time** (the post phase is not compensated — D4); `Valid()` still
  accepts the shape (a leaf rollback) for symmetry — documented, not specially enforced.

## 4. Core (`internal/core`)

- New sentinel `ErrPostAction` (in `runner.go`, beside `ErrBlocked`): "the move applied but a post-action
  failed."
- `Transitioner.Transition` gains a **post phase after `store.Update`**. New order:
  ```
  validate edge → attribution → pre-gate(commands) → persist(store.Update) → [POST: run(edge.Post)]
  ```
  Post phase (only when `len(edge.Post) > 0`):
  - `expandCommands(edge.Post, cmdContext{ID, Type, From, To})` — the **same** expander and the **same**
    context as the pre-gate (`From` = pre-move status, `To` = new status → correct for a
    `"{{.ID}}: {{.From}} → {{.To}}"` commit subject). An expand error → `(persisted, ErrPostAction)`.
  - `runner.Run(expanded)` — the **same** `Runner` port. An operational error or a non-zero check
    (`firstFailure`) → `(persisted, ErrPostAction)` wrapping the cause.
  - **No compensation, no rollback, no second `Update`.** Post checks are **not** written to history (persist
    already happened; the failure is surfaced in-band, not recorded).
  - On success → `(persisted, nil)` (unchanged from today).
- `persisted` is the task returned by `store.Update` — always the real, persisted state, so the CLI can render
  the move even when `ErrPostAction` is returned.

## 5. Adapter (`internal/adapter/yaml`)

- `ymlTransition.Post []ymlCommand` (`yaml:"post,omitempty"`) — **reuses** `ymlCommand` (scalar-or-map,
  `timeout`, `rollback`; `UnmarshalYAML` already handles both). `toDomain` maps it into `Transition.Post`
  exactly as `Commands` is mapped (`c.toDomain()` per element). Decode path only; config is never marshaled.

## 6. CLI (`internal/cli`)

- `runTransition`: after `tr.Transition(...)` returns `(task, err)`:
  - `err == nil` → the existing success path (apply current pointer, print `<id>: from → to` + guidance).
  - `errors.Is(err, ErrPostAction)` → the **move happened**: run the same success rendering (`applyCurrent`,
    print the move + guidance), **then** write `move applied, but a post-action failed: <cause>` to stderr and
    **return the error** (so the process exits non-zero). `task` is the persisted state.
  - any other error → the existing failure handling (blocked-gate hint for `ErrBlocked`, etc.).
- `exitCode` (`root.go`): add `case errors.Is(err, core.ErrPostAction): return 5` (5 is free; 3 stays
  pre-gate block = status unchanged).
- `mtt types` (`writeTypeBlock`): render a post phase under an edge (e.g. `⇢ <post.Run>` + `(timeout …)`),
  mirroring how `commands` and the `↩ rollback` line render — discoverability, optional but cheap.

## 7. Repo config + docs (mechanizing our own pain)

- Add to the committed `.mtt/config.yaml`, on **every edge** (each one changes the status and must commit it —
  `start`, the three `submit`, the `approve` edges, the `decline` edges, `deliver`, and the `cancel` edges):
  ```yaml
  post:
    - git add .mtt && git commit -m "{{.ID}}: {{.From}} → {{.To}}"
  ```
  For `deliver`/`cancel` the pre-gate already ran `git switch main`, so the post commit lands on `main` —
  which **removes the two manual steps**.
- Update `TestRepoDogfoodConfig` if it asserts anything the new `post:` blocks touch (it pins transition
  **counts**, not `post` — so a `post` add doesn't redden it; add a spot assertion only if we want the field
  guarded, mirroring the t5 `require` note).
- Remove the "two manual steps remain" bullet from [AGENTS.md](../../../AGENTS.md) ("Working under mtt") and
  note post-persist auto-commit; sync `DESIGN.md`/`.ru`, `CLI_REFERENCE.md`/`.ru`, and the touched `CLAUDE.md`.

## 8. Error handling / exit codes

- **Pre-gate block** (unchanged): `ErrBlocked` → exit 3, status **not** changed, succeeded-prefix compensated.
- **Post failure**: `ErrPostAction` → exit **5**, status **kept** (move is valid), `.mtt` may be left
  uncommitted — the operator commits it by hand; mtt never rolls back a valid move for a post hiccup.
- **Commit "nothing to commit"** caveat: `git commit` exits non-zero when there is nothing staged. The repo
  `post` uses `git add .mtt && git commit …`; if a move somehow wrote no file change, the commit fails → exit
  5. Acceptable (loud), and in practice every move rewrites the task file. (A `git diff --cached --quiet ||
  git commit` guard is a config choice, not a mechanism concern.)

## 9. Testing (TDD)

- **`core/transition_test.go`**: (a) an edge with `Post` runs the post commands **after** persist — a fake
  runner/probe sees the store already at `to` when post runs; (b) a failing post command → `ErrPostAction`
  **and** the store shows the **new** status (persisted, not rolled back); (c) an edge with no `Post` is
  byte-identical to today (no runner call for a post phase); (d) post placeholders expand (`{{.ID}}` → the
  real id reaches the runner); (e) an expand error in `Post` → `ErrPostAction` (not a plain error).
- **`adapter/yaml`**: decode an edge with `post:` (both a scalar command and a `{run,timeout}` map) →
  `Transition.Post` populated.
- **`internal/cli` e2e** `post_actions.txt`: an edge whose `post:` is `[echo POSTRAN]` → the move prints and
  the post runs (assert both the move line and `POSTRAN`); an edge whose `post:` is a failing command (`false`)
  → **exit 5**, and `mtt show` confirms the status **did** change (move kept).
- **Guard:** `make check` green; update `TestRepoDogfoodConfig` if needed.

## 10. Affected files

`pkg/mtt/config.go` (`Transition.Post` + `Config.Validate`) · `internal/core/{transition.go,runner.go}`
(post phase + `ErrPostAction`) · `internal/adapter/yaml/dto.go` (`ymlTransition.Post` + map) ·
`internal/cli/{status.go,root.go}` (post-error rendering + exit 5; optional `types.go` render) ·
`.mtt/config.yaml` (post on edges) · tests + `internal/cli/testdata/scripts/post_actions.txt` ·
`docs/architecture/model.go` (`Transition.Post`, `ErrPostAction`) · docs sync (AGENTS/DESIGN/CLI_REFERENCE/
CLAUDE ×2-3).

## 11. Out of scope (explicit)

- **Global/default `post`** and its precedence vs per-edge — deferred to **t24**.
- **Post-phase rollback/compensation** — the post phase is non-transactional by design (D4).
- **Recording post checks in history** — not written (persist precedes post; a second `Update` is not worth
  it).
- **`push` as a post-action** — outward; stays manual (a `post` *could* run it, but the repo config won't).
- **Changing the pre-gate semantics** — `commands:` and s008 compensation are untouched.

## 12. Invariants (self-instructing)

1. Two phases: `commands:` gate the **entry** (fail → no persist, compensate); `post:` finalize **after** entry
   (fail → status kept, `ErrPostAction`, exit 5).
2. A post failure **never** rolls back a persisted move — the status is authoritative once written.
3. Post reuses the exact pre-gate machinery (`expandCommands`, `Runner`, `firstFailure`) and the same
   `{ID,Type,From,To}` context — one expander, one runner, two phases.
