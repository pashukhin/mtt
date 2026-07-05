# 005 — Dependencies

Status: done   ·   Branch: `feat/s005-dependencies`

## Target

Give tasks **blocking** edges: make one task depend on another (`mtt dep add`), remove/inspect them
(`mtt dep rm`/`list`), reject dependency cycles on add, and surface the actionable set — `mtt ready`
(and `list --ready`): a task is ready ⇔ its status is not terminal AND every `depends_on` is terminal
(by status **category** `kind`, never a literal). Second derived graph in `core` (over `depends_on`),
mirroring the s004 hierarchy `Index`.

## Scope

- **In:**
  - **`mtt dep add <id> <depends-on-id>`** — add a blocking edge (`depends_on`). Validates both tasks
    exist; rejects a **self-edge** and any edge that would create a **cycle**; adding an already-present
    edge is an idempotent no-op. Bumps `updated` on a real change.
  - **`mtt dep rm <id> <depends-on-id>`** — remove a blocking edge; idempotent (removing an absent edge is
    a no-op, symmetric with `add`; the task must exist). Bumps `updated` on a real removal.
  - **`mtt dep list <id>`** — show a task's direct blockers (`depends on:`, dangling targets flagged
    `(missing)`) and its **computed** dependents (`required by:`); `--tree` renders the transitive
    dependency tree (cycle-safe); `--cycles` reports dependency cycles project-wide; `--json`.
  - **`mtt ready [filters]`** — list actionable tasks (non-terminal, all blockers terminal, **conservative**:
    anything unresolvable — a dangling blocker or a status not in the current flow — blocks). Accepts the
    `list` filters (`--status`/`--type`/`--kind`/`--parent`); `--json`.
  - **`mtt list --ready`** — shorthand companion; the same ready subset piped through `list`.
  - `internal/core`: `Ready` (pure read), `DependencyEditor` (mutation, owns the cycle-check), and a derived
    `DepGraph` (over `depends_on`) — parallel to s004's `Index` (over `parent`). **No new port**: the edge
    rides the `Task.DependsOn` field and round-trips via `TaskStore.Update` (as `parent` did in s004).
- **Out (explicitly deferred):**
  - Status transitions (`mtt status`/`done`/`cancel`) → **s006**. Consequence: e2e cannot *produce* a
    terminal task, so it proves **blocking + cycle rejection**; unblock-on-terminal and `--cycles` on a real
    cycle are proven by **unit fixtures** (a `done`-blocker graph / a hand-built cycle — the CLI's `add`
    rejects cycles, so one can't be created through it).
  - `--depends-on` on `add` (create-with-dep) → later; s005 uses the explicit `dep add`.
  - `refs` (`ref add/rm/list`, backlinks) — the **informational**, non-blocking link → s008.
  - The `DependencyStore` capability port (external adapters that cannot embed the field) → when a real
    external adapter needs it.

## Acceptance (must pass)

- **User scenario:** `mtt init` → `mtt add --type epic E` (e1) → `mtt add --type task --parent e1 A` (t1) →
  `mtt add --type task --parent e1 B` (t2) → `mtt dep add t2 t1` → `mtt ready` lists **e1, t1** and **not
  t2** (t2 is blocked by the `tbd` t1) → `mtt dep add t1 t2` is **rejected** as a cycle → `mtt dep list t2`
  shows `depends on: t1`; `mtt list --ready` excludes t2.
- **e2e:** `testscript` `dep.txt` covering the above (blocking + cycle rejection + `dep list` + `--ready`).
- `make check` green.

## Plan (refine at session start — test-first; brainstorm → writing-plans)

Design decisions resolved in brainstorm — authoritative spec:
[../docs/superpowers/specs/2026-07-05-session-005-dependencies-design.md](../docs/superpowers/specs/2026-07-05-session-005-dependencies-design.md).
Summary: **no new port** (GAP #1 accepted); `core.DependencyEditor(store, now)` (**YAGNI** — no
`DependencyStore` param yet) owns the cycle-check; a derived `core.DepGraph` mirrors `Index` (kept
**separate** — GAP #6 not extracted; `parent` is a single-parent tree, `depends_on` a multi-edge DAG);
`core.Ready` is one pure primitive shared by `mtt ready` and `list --ready`; ready is **conservative**
(unresolvable = blocks); a `cancelled` blocker unblocks (per DESIGN) — flagged as a **deferred** semantic
question (see spec).

- [x] Brainstorm the open questions — resolved (see the spec above).
- [ ] `internal/core`: `Ready` (pure) + a shared `kindOf` helper (DRY with `matchesKind`); `DependencyEditor`
      (add/rm + cycle-check); `DepGraph` (dependents/tree/cycles/`Reaches`), cycle-safe; back-refs computed.
- [ ] `internal/adapter/yaml`: confirm `depends_on` round-trips (no store change); add a round-trip test if missing.
- [ ] `internal/cli`: `mtt dep add/rm/list`; `mtt ready`; `mtt list --ready`; `--json` where it fits.
- [ ] `testscript` `dep.txt`; docs (DESIGN/.ru, CLI_REFERENCE/.ru, CLAUDE.md ×2, model.go GAP #1/#6/Ready,
      the deferred `cancelled`-blocker marker in DESIGN/TASKS).

## Done (fill during/after the session)

Shipped (all test-first, `make check` + CI green):

- **`internal/core`**: `Ready` (pure, conservative — unresolvable status/dangling blocker → not ready) + a
  shared `kindOf` helper (DRYs the type→`StatusKind` lookup with `matchesKind`); `DependencyEditor`
  (`AddDependency`/`RemoveDependency` via `TaskStore.Update` — self + cycle rejected, add and rm both
  idempotent (duplicate/absent-edge are no-ops)); `DepGraph` (derived over `depends_on`:
  `Get`/`DependsOn`/`Dependents`(computed)/`Reaches`/`Cycles`), cycle-safe, kept **separate** from `Index`.
- **`internal/adapter/yaml`**: no store change — added a focused `depends_on` DTO round-trip test (GAP #1
  confirmed: the edge rides the `Task` field).
- **`internal/cli`**: `mtt dep add/rm <id> <dep-id>` (via `core.DependencyEditor`); `mtt dep list <id>`
  (`depends on:` + computed `required by:`, dangling → `(missing)`; `--tree` transitive cycle-safe;
  `--cycles` project-wide defensive; non-null `--json`); `mtt ready` and `list --ready` sharing one
  primitive `Select(Ready(tasks, cfg), filter, cfg)`; shared `toStatusNames`/`toTypeNames` converters.
- **Tests**: unit (fixed clock) for `Ready` (incl. `done`-blocker unblocks via fixture), `DepGraph`
  (dependents/Reaches/Cycles hand-built), `DependencyEditor` (self/dup/notfound/cycle/rm), `renderDepList`/
  `renderDepTree`/JSON builders; e2e `dep.txt` (add/list/rm/self/cycle/tree/cycles) + `ready.txt`
  (blocking + `list --ready` + non-null json).
- **Docs**: DESIGN.md/.ru (Dependencies rewrite + `cancelled`-blocker deferred marker), CLI_REFERENCE.md/.ru
  (dep/ready/list --ready implemented), core + cli `CLAUDE.md`, `model.go` (GAP #1/#6 resolved,
  `NewDependencyEditor` signature, Ready/DependencyEditor shipped), TASKS.md (e3_t2–t4 ticked + marker).

Decisions (full detail in the spec): **no new port** (GAP #1 — edge on `Task.DependsOn` + `Update`);
**conservative ready** (unresolvable = blocks); `NewDependencyEditor(store, now)` — **YAGNI**, no
`DependencyStore` param; `DepGraph` kept **separate** from `Index` (GAP #6 not extracted); one `Ready`
primitive for `ready` + `list --ready`.

Deferred (don't lose): **`cancelled`-blocker semantics** (a cancelled blocker unblocks by `kind` — revisit
in s006 when terminals become reachable); the `DependencyStore` capability port (external adapters);
`--depends-on` on `add`; `refs`/backlinks → s008; the durable edit-audit + subject-identity (`By`) slice.
