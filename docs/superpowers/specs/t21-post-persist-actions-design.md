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
- `Config.Validate` extends the transition check to `Post[].Valid()` (same rule as `Commands`), with a
  **distinct error string** — `invalid post command …` (not "invalid command …"), so an operator can tell which
  list is malformed (the parallel `Commands` loop keeps its wording).
- A post command's `Rollback` is **ignored at run time** (the post phase is not compensated — D4). Note the
  asymmetry: `expandCommands` still expands `Rollback.Run` for a post command (it recurses over the whole
  `Command`), so a malformed post-rollback *template* surfaces as `ErrPostAction` at expand time even though the
  rollback would never run. Acceptable (fail-fast on a bad template); documented so no one relies on a post
  rollback executing.

## 4. Core (`internal/core`)

- New sentinel `ErrPostAction` (in `runner.go`, beside `ErrBlocked`): "the move applied but a post-action
  failed."
- `Transitioner.Transition` gains a **post phase after `store.Update`**. New order:
  ```
  validate edge → attribution → [!NoRun: pre-gate(commands)] → persist(store.Update) → [!NoRun & len(Post)>0: POST run(edge.Post)]
  ```
  **`--no-run` skips BOTH phases (M1).** The post phase runs **only when `!opts.NoRun`** — same guard as the
  pre-gate. `--no-run` means "bypass the edge's commands", and `post:` are commands, so it skips them too.
  Rationale: otherwise `mtt status <id> done --no-run` would skip `deliver`'s pre-gate `git switch main` yet
  still run the post `git commit`, committing `done` on the **task branch** (the exact wrong-branch failure the
  two-phase model exists to prevent). With `--no-run` the persist still happens; finalization is the operator's
  job (the escape hatch's whole point). `from`/`to`/`t.ID`/`t.Type` are all in scope after `store.Update`, so
  the post `cmdContext{ID, Type, From, To}` is built from the same values as the pre-gate.
  Post phase (only when `!opts.NoRun && len(edge.Post) > 0`):
  - `expandCommands(edge.Post, cmdContext{ID, Type, From, To})` — the **same** expander and the **same**
    context as the pre-gate (`From` = pre-move status `from`, `To` = new status → correct for a
    `"{{.ID}}: {{.From}} → {{.To}}"` commit subject). An expand error → `(persisted, ErrPostAction)`.
  - `runner.Run(expanded)` — the **same** `Runner` port. An operational error or a non-zero check
    (`firstFailure`) → `(persisted, ErrPostAction)` wrapping the cause.
  - **No compensation, no rollback, no second `Update`.** Post checks are **not** written to history (persist
    already happened; the failure is surfaced in-band, not recorded — see §11 on the observability trade-off).
  - On success → `(persisted, nil)` (unchanged from today).
- **New return contract (M2), stated explicitly:** today every error path returns `mtt.Task{}` (`err ⇒ no
  move`, codified in the `Transition` godoc + `internal/core/CLAUDE.md`). `ErrPostAction` is the **single**
  exception: it returns the **persisted** task *with* a non-nil error (the move happened; only the finalization
  failed). This must be documented in the method godoc, `docs/architecture/model.go`'s `Transitioner` block,
  and `internal/core/CLAUDE.md`. The sole caller is `internal/cli/status.go` (§6).

## 5. Adapter (`internal/adapter/yaml`)

- `ymlTransition.Post []ymlCommand` (`yaml:"post,omitempty"`) — **reuses** `ymlCommand` (scalar-or-map,
  `timeout`, `rollback`; `UnmarshalYAML` already handles both). `toDomain` maps it into `Transition.Post`
  exactly as `Commands` is mapped (`c.toDomain()` per element). Decode path only; config is never marshaled.

## 6. CLI (`internal/cli`)

**`runTransition` restructure (B3 — this is a real control-flow change, not an add-on).** Today `runTransition`
returns on *any* non-nil error from `tr.Transition` (`status.go:100-105`), discarding `task`, and the
success-render block (`:106-122`, including the `--json` early-return at `:109-111` and the terminal
`return nil`) runs only on `err == nil`. The new shape:

```go
task, txErr := tr.Transition(...)          // NB: dedicated var — never reuse it for the Fprintf writes below
postFailed := errors.Is(txErr, core.ErrPostAction)
if txErr != nil && !postFailed {
    // existing failure handling (blocked-gate hint for ErrBlocked, etc.)
    if hidden && errors.Is(txErr, core.ErrBlocked) { return fmt.Errorf("%w\n  hint: …", txErr) }
    return txErr
}
// txErr == nil OR postFailed: the move IS persisted → render it.
if e := applyCurrent(root, cfg, task, id); e != nil { return fmt.Errorf("…current…: %w", e) }
if jsonFlag(cmd) {
    if e := writeJSON(cmd.OutOrStdout(), toTaskJSON(task)); e != nil { return e }
    return txErr // B2: nil on success, ErrPostAction on post-failure → exit 5 preserved even in --json
}
// text mode: print "<id>: from → to" + moveGuidance — use a LOCAL `e`, NOT txErr:
if _, e := fmt.Fprintf(out, "%s: %s → %s\n", id, last.From, last.To); e != nil { return e }
if g := moveGuidance(...); g != "" {
    if _, e := fmt.Fprint(out, g); e != nil { return e }
}
if postFailed {
    _, _ = fmt.Fprintf(cmd.ErrOrStderr(), "move applied, but a post-action failed: %v\n", txErr)
}
return txErr // nil on success, ErrPostAction on post-failure
```

**MAJOR-fix note:** the transition error is held in `txErr` and the text-print `Fprintf`s use a **local `e`** —
today's code reuses `err` for those writes (`status.go:114,118`), which would clobber `ErrPostAction` to `nil`
and defeat exit 5 in text mode. Do **not** reuse the transition error var for the render writes. The stderr
`Fprintf` return is explicitly discarded (`_, _ =`) to satisfy errcheck.

- **`--json` (B2):** the JSON branch emits the persisted task object as today, then **returns `txErr`** (not
  `nil`), so `ErrPostAction` still maps to **exit 5**. The failure is never swallowed. (The move object is
  honest — the status *did* change; the non-zero exit signals the post failure. Adding an error field to the
  JSON is optional and out of scope.)
- **`exitCode`** (`root.go`): add `case errors.Is(err, core.ErrPostAction): return 5` (5 is free; 3 stays
  pre-gate block = status unchanged).
- `mtt types` (`writeTypeBlock`): render a post phase under an edge as `⇢ <post.Run>` (+ `(timeout …)`),
  **after** the `$ <command>` lines and the `↩ <rollback>` line (order: `$` gate → `↩` rollback → `⇢` post) —
  mirrors the existing rollback render (`types.go:109`). Discoverability; cheap.

## 7. Repo config + docs (mechanizing our own pain)

- Add to the committed `.mtt/config.yaml`, on **every edge** (each changes the status and must commit it —
  `start`, the three `submit`, the `approve` edges, the `decline` edges, `deliver`, and the `cancel` edges; ~38
  edges across `task`+`chore`). **The command MUST be a single-quoted YAML scalar** (B1 — the repo rule,
  AGENTS.md; an unquoted scalar with the `: ` inside `{{.ID}}: ` is parsed as a mapping and hard-errors
  `yaml.Load` for the whole config), and use a **`-- .mtt` pathspec** so it commits only `.mtt` and never
  sweeps up unrelated staged files (nit — footgun):
  ```yaml
  post:
    - 'git add .mtt && git commit -m "{{.ID}}: {{.From}} → {{.To}}" -- .mtt'
  ```
  For `deliver`/`cancel` the pre-gate already ran `git switch main`, so the post commit lands on `main` —
  which **removes the two manual steps** (and fully replaces them; nothing else was manual there).
- **`TestRepoDogfoodConfig` (M3):** it pins transition **counts** and `Commands`, never `Post`, so adding
  `post:` neither reddens nor guards it. Given ~38 identical blocks (the DRY smell deferred to t24), add a
  **spot assertion** that every non-... edge carries the expected `post` (e.g. count edges with a non-empty
  `Post` == total, or assert the exact `post` on a representative edge) — so a dropped/drifted block is caught.
- Remove the "two manual steps remain" bullet from [AGENTS.md](../../../AGENTS.md) ("Working under mtt") and
  note post-persist auto-commit; sync `DESIGN.md`/`.ru`, `CLI_REFERENCE.md`/`.ru`, and the touched `CLAUDE.md`.

## 8. Error handling / exit codes

- **Pre-gate block** (unchanged): `ErrBlocked` → exit 3, status **not** changed, succeeded-prefix compensated.
- **Post failure**: `ErrPostAction` → exit **5**, status **kept** (move is valid), `.mtt` may be left
  uncommitted — the operator commits it by hand; mtt never rolls back a valid move for a post hiccup.
- **Commit "nothing to commit"**: near-impossible in practice — every persist appends a `HistoryEntry` and
  bumps `Updated` (`transition.go`), so the task file content **always** changes and `git add .mtt` always
  stages it. (Even a re-submit writes a new history row.) So the `git commit` won't spuriously fail; if it
  somehow did, exit 5 is loud and honest.
- **`--no-run`**: skips the post phase entirely (M1) — the persist still happens, but nothing is committed;
  finalization is the operator's job (consistent with "bypass the edge's commands").

## 9. Testing (TDD)

- **`core/transition_test.go`**: (a) an edge with `Post` runs the post commands **after** persist — a fake
  runner/probe sees the store already at `to` when post runs; (b) a failing post command → `ErrPostAction`
  **and** the store shows the **new** status (persisted, not rolled back); (c) an edge with no `Post` is
  byte-identical to today (no runner call for a post phase); (d) post placeholders expand (`{{.ID}}` → the
  real id reaches the runner); (e) an expand error in `Post` → `ErrPostAction` (not a plain error); (f)
  **`NoRun=true` skips the post phase** even when `Post` is set (M1 — no runner call, status still persisted).
- **`adapter/yaml`**: decode an edge with `post:` (both a scalar command and a `{run,timeout}` map) →
  `Transition.Post` populated.
- **`internal/cli` e2e** `post_actions.txt`: an edge whose `post:` is `[echo POSTRAN]` → the move prints and
  the post runs (assert both the move line and `POSTRAN`); an edge whose `post:` is a failing command (`false`)
  → **exit 5**, and `mtt show` confirms the status **did** change (move kept); the **same failing edge with
  `--json`** → **exit 5** too (B2 — the failure isn't swallowed in JSON mode); the failing edge with `--no-run --who a
  --why b` → exit 0 and no post output (M1). **Note the `--who`/`--why`:** since t5, `--no-run` forces who+why
  (`transition.go` ORs `NoRun` into `effWho`/`effWhy`), so a `--no-run` move without them exits 2, not 0 — the
  test must supply them to actually exercise the post-skip.
- **`TestRepoDogfoodConfig`** (M3): add the spot assertion (every edge carries the expected `post`).
- **Guard:** `make check` green.

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
  it). **Observability trade-off (accepted):** a post that fails (exit 5) and is fixed by hand leaves no trace
  in `mtt show` — asymmetric vs pre-gate checks, and for `deliver` the manual commit was previously the on-main
  audit. Acceptable because the git commit itself *is* the durable record on success, and a failure is loud
  (exit 5) at the moment. Recording post checks (a second `Update`, or an audit line) is a future option, not
  t21.
- **`push` as a post-action** — outward; stays manual (a `post` *could* run it, but the repo config won't).
- **Changing the pre-gate semantics** — `commands:` and s008 compensation are untouched.

## 12. Invariants (self-instructing)

1. Two phases: `commands:` gate the **entry** (fail → no persist, compensate); `post:` finalize **after** entry
   (fail → status kept, `ErrPostAction`, exit 5).
2. A post failure **never** rolls back a persisted move — the status is authoritative once written.
3. Post reuses the exact pre-gate machinery (`expandCommands`, `Runner`, `firstFailure`) and the same
   `{ID,Type,From,To}` context — one expander, one runner, two phases.
4. **`--no-run` skips BOTH phases** (gate and post) — the same `!opts.NoRun` guard wraps both; the persist
   still happens, finalization does not.
5. `ErrPostAction` is the **only** case where `Transition` returns a **valid task with a non-nil error** (move
   persisted, finalization failed) — every other error path returns an empty task.
