# Session 005 — Dependencies — Design Spec

Status: approved (brainstorm) · Branch: `feat/s005-dependencies` · Date: 2026-07-05

Authoritative design for session 005. The session Target/Scope/Acceptance live in
[../../../sessions/005_dependencies.md](../../../sessions/005_dependencies.md); the architecture rationale
lives in [../../../DESIGN.md](../../../DESIGN.md) → "Dependencies" and the domain-model snapshot
[../../architecture/model.go](../../architecture/model.go) (GAP #1, Ready, DependencyEditor, GAP #6).

## Goal

Blocking dependency edges between tasks (`depends_on`), plus the derived "what can I pick up" view:

- `mtt dep add/rm/list <id>` — manage and inspect `depends_on` (a **blocking** edge, distinct from the
  hierarchy `parent` and from the informational `refs`).
- cycle rejection on `dep add`.
- `mtt ready` (and `list --ready`) — actionable tasks: status **not terminal** AND every `depends_on`
  **terminal**, decided by status **category** (`kind`), never by a literal name.

## Locked decisions (from brainstorm)

1. **Full command surface** is in scope: core `dep add/rm/list` + `dep list --tree` + `dep list --cycles` +
   `mtt ready` + `list --ready`.
2. **Conservative `ready`**: readiness requires *positive confirmation*. A task is ready only if its own
   status resolves to a non-terminal `kind` AND every blocker resolves to a **present** task with a
   `terminal` `kind`. Anything unresolvable — a **dangling** `depends_on` (target absent) or a **config-drift**
   status (not in the current flow, so `Type.StatusKind` returns `false`) — is treated as **blocking**
   (mirrors how `core.Match` already fails an unresolvable `--kind`).
3. **`core.DependencyEditor(store, now)`** — YAGNI signature, **no** `DependencyStore` parameter (GAP #1:
   s005 adds no port; the edge rides `Task.DependsOn` + `TaskStore.Update`). The forward-looking port
   parameter shown in `model.go` is dropped for s005; `model.go` GAP #1 is updated to record this.
4. **GAP #6 not extracted**: `Index` (over `parent`, a single-parent tree walked upward) and the new
   `DepGraph` (over `depends_on`, a multi-edge DAG walked downward with a computed reverse index) are
   different enough that a shared visited-set primitive would be a forced abstraction. Revisit only if a
   third graph (flow, s006) naturally shares it.

## Deferred design question — `cancelled` blocker semantics

Per DESIGN.md a **terminal** blocker unblocks its dependent, and `cancelled` is a terminal `kind` — so a
task both of whose blockers are `done` **and** `cancelled` is formally **ready**. Semantically this is
ambiguous: a `cancelled` blocker was *abandoned*, not *completed*, so the dependent may in fact need
re-evaluation (or its own cancellation) rather than being silently unblocked.

**s005 keeps the current DESIGN behaviour** (terminal-by-`kind` unblocks, `cancelled` included) — changing
it is out of scope and would need product thought (a `terminal`-but-`succeeded` sub-distinction, or a
per-dependency "hard/soft" flag, or a warning on `ready` when a blocker is `cancelled`). This is recorded
as a **marker** in DESIGN.md → "Dependencies" and TASKS.md → "Later" so it is not lost. Revisit alongside
flow enforcement (s006), where terminal statuses first become reachable and the distinction bites.

## Architecture (stays `cli → core → port ← adapter`; `core` never imports `adapter/*`)

### `pkg/mtt` — no change (confirms GAP #1)

`Task.DependsOn []TaskID` already exists; the YAML DTO already round-trips `depends_on`; `Type.StatusKind`
already resolves a status category; `TaskStore.Update` already persists an updated task. **No new port,
type, or method.** The edge is a field, mutated by `core` and persisted via `Update` — exactly as `parent`
was in s004.

### `internal/core`

- **`kindOf(t mtt.Task, cfg mtt.Config) (mtt.StatusKind, bool)`** — a small private helper resolving a
  task's status category (`cfg.TypeByName` → `Type.StatusKind`). The existing `matchesKind` is refactored to
  use it, and `Ready` reuses it (DRY; the "resolve a task's kind" logic lives in one place).

- **`Ready(tasks []mtt.Task, cfg mtt.Config) []mtt.Task`** — pure read. Builds a `byID` map, then keeps a
  task iff `kindOf` resolves to a **non-terminal** kind AND every `DependsOn` id resolves to a present task
  whose `kindOf` is **terminal**. Ordered by the shared `lessByRecency` (Created desc, ID tiebreak), so the
  result is deterministic and provider-agnostic. No store, no clock.
  - **One primitive, two consumers**: both `mtt ready [filters]` and `list --ready [filters]` compute
    `Select(Ready(tasks, cfg), filter, cfg)` — the identical composition (readiness AND the list predicates,
    both ANDs, order-independent). No duplicated readiness logic.

- **`DependencyEditor`** (mutation usecase; struct + verb methods, like `Adder`/`Editor`):
  ```
  NewDependencyEditor(store mtt.TaskStore, now func() time.Time) *DependencyEditor
  (d) AddDependency(id, dependsOn mtt.TaskID) (mtt.Task, error)
  (d) RemoveDependency(id, dependsOn mtt.TaskID) (mtt.Task, error)
  ```
  - **AddDependency**: `Get(id)` and `Get(dependsOn)` → `ErrNotFound` mapped to clear messages
    (`task %q not found` / `dependency %q not found`). Reject **self** (`id == dependsOn`) as a trivial
    cycle. If the edge is **already present** → idempotent no-op (return the loaded task, no write, no
    `updated` bump). **Cycle-check**: build `DepGraph` from `store.List()`; if `dependsOn` can `Reaches(id)`
    over `depends_on`, reject (`… would create a cycle`). Otherwise append `dependsOn`, bump `updated` from
    the injected clock, `Update`.
  - **RemoveDependency**: `Get(id)`; if the edge is **absent** → idempotent no-op (return the loaded task,
    no write, no bump — symmetric with the duplicate-add no-op; post-review change); otherwise drop it, bump
    `updated`, `Update`. (The task itself must exist — a missing `id` still errors.)
  - The cycle rule lives **here** (core policy), never in an adapter.

- **`DepGraph`** (derived, pure — parallel to `Index`), built from a task slice via `NewDepGraph(tasks)`:
  ```
  Get(id) (mtt.Task, bool)
  DependsOn(id) []mtt.TaskID     // direct blockers, raw ids (dangling kept, flagged by the CLI)
  Dependents(id) []mtt.Task      // computed reverse edges (who depends on id), present tasks only
  Reaches(from, to mtt.TaskID) bool  // DFS over depends_on; powers the add cycle-check
  Tree(id) / Cycles()            // transitive tree (--tree) and project-wide cycle report (--cycles)
  ```
  Cycle-safe throughout (visited-set). Bucket order uses the shared `lessByRecency` (determinism, matches
  `Index`/`Select`). Not part of the `pkg/mtt` contract — the resolved graph is derived.

### `internal/adapter/yaml` — no change

`dep add/rm` go through `Get`/`Update`; `dep list`/`ready`/`--tree`/`--cycles` through `List`. `depends_on`
already round-trips (`fromDomainDeps`/`toDomainDeps`, `omitempty`). Add a round-trip unit test for
`DependsOn` if one is not already covered.

### `internal/cli`

- **`dep.go`** — `mtt dep` parent with `add`/`rm`/`list` subcommands (one file, like the pattern; each
  routes through `projectRoot` → adapter → core).
  - `dep add <id> <dep-id>` → `DependencyEditor.AddDependency`; prints `added <dep-id> to <id>` (or the JSON
    task).
  - `dep rm <id> <dep-id>` → `RemoveDependency`; prints `removed <dep-id> from <id>`.
  - `dep list <id>` → `NewDepGraph(List)`: human output
    ```
    <id> depends on:
      t1  task  [done]
      t9  (missing)
    required by:
      t5  task  [tbd]
    ```
    `--tree` — nested ASCII transitive tree (reuse the `renderTree` connectors idiom, cycle-safe);
    `--cycles` — one line per cycle, else `no cycles`; `--json` — structured, zero-match → `[]` (never
    `null`).
- **`ready.go`** — `mtt ready [list filters]`: `List` → `Select(Ready(...), filter, cfg)` → render via the
  shared `taskLine` / `taskJSON`.
- **`list.go`** — add `--ready` (bool): gate the tasks through the ready subset before `Select`.

### Folded minor semantics (follow the established `refs` pattern)

| Case | Behaviour |
|---|---|
| `dep add t1 t1` (self) | reject — trivial cycle |
| `dep add` unknown `<id>` / `<dep-id>` | not-found error |
| `dep add` duplicate edge | idempotent no-op (exit 0, no `updated` bump) |
| `dep rm` absent edge | idempotent no-op (post-review: symmetric with duplicate add) |
| `dep add`/`rm` real change | bump `updated` from the injected clock |
| exit codes | keep the current single generic failure (`1`); the `2`–`6` taxonomy lands with its behaviours later |

## Tests (test-first)

- **Unit (fixed clock)**:
  - `Ready` — no-dep task ready; a `tbd` blocker blocks; **a `done` blocker unblocks** (fixture, since the
    CLI can't reach terminal yet); a **dangling** blocker blocks; a **config-drift** status blocks; a
    terminal task itself is never ready.
  - `DependencyEditor` — add happy path (+`updated` bump); self rejected; duplicate no-op (no bump);
    unknown id/target not-found; **cycle rejected** (direct and transitive).
  - `DepGraph` — `DependsOn`/`Dependents` (computed reverse); `Reaches`; `Tree`; **`Cycles`** on a
    hand-built cyclic fixture; order via `lessByRecency`.
  - `kindOf` — resolve/miss.
  - adapter: `DependsOn` round-trip.
- **e2e `dep.txt`** (drives the real binary; anchored asserts): the acceptance scenario — blocking
  (`ready` excludes the blocked task), cycle rejection, `dep list` presence, `list --ready`. Order asserted
  by **presence**, not sequence (provider-agnostic order proven in unit). Zero-match `--json` → `[]`.

> **e2e limitation (explicit):** no status-transition command exists until s006, so the e2e cannot create a
> `terminal` task — it proves **blocking + cycle rejection**. Unblock-on-terminal and `--cycles` on a real
> cycle are unit-only (a `done`-blocker fixture / a hand-built cycle; `dep add` rejects cycles, so the CLI
> can't build one).

## Carry-over lessons applied (from NEXT_SESSION.md)

- CLI output via `fmt.Fprint(cmd.OutOrStdout(), …)`; errors via `RunE`/`Execute`.
- `golangci unused`: declare each new symbol in the change that first *uses* it.
- testscript asserts **anchor** (`'t2  task  \[tbd\]'`), never bare substrings; `--dir` sibling caveat.
- zero-match `--json` = `[]` (build with `make([]T, 0, …)`), not `null`.
- derived graphs live in `core`, not the contract (`DepGraph`, like `Index`).
- one shared predicate/primitive for filters (`Ready` for both `ready` and `list --ready`; `kindOf` shared
  with `matchesKind`).
- everything typed — `mtt.TaskID`/`TypeName`/`StatusName`; convert strings only at the cli/adapter boundary.
- new package behaviour → keep each `CLAUDE.md` current (core + cli).

## Docs to update (implementation step)

- `DESIGN.md`/`DESIGN.ru.md` — expand "Dependencies" (conservative ready; derived `DepGraph` in core; no new
  port) + the **`cancelled`-blocker deferred marker**.
- `CLI_REFERENCE.md`/`CLI_REFERENCE.ru.md` — mark `dep add/rm/list`, `ready`, `list --ready` implemented
  (session 005); reconcile phase tags.
- `internal/core/CLAUDE.md` — `Ready`, `DependencyEditor`, `DepGraph`, `kindOf`.
- `internal/cli/CLAUDE.md` — `dep`, `ready`, `list --ready`.
- `docs/architecture/model.go` — GAP #1 (DependencyEditor signature: no port param, shipped), Ready/
  DependencyEditor tier notes, GAP #6 (kept separate).
- `TASKS.md` — tick e3_t2/t3/t4; add the `cancelled`-blocker marker under "Later".
- `NEXT_SESSION.md` — end-of-session handoff (done/next).

## Out of scope (unchanged, don't lose)

- Status transitions / flow gates → s006. `refs` (informational links) + backlinks → s008.
- `--depends-on` on `add`; `DependencyStore` capability port; the durable edit-audit + subject-identity
  (`By`) open slice — all deferred.
