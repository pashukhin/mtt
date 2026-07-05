# Session 004 — Hierarchy — Design Spec

Date: 2026-07-04 · Branch: `feat/s004-hierarchy` · Session: [../../../sessions/004_hierarchy.md](../../../sessions/004_hierarchy.md)

Language note: agent-facing docs are English only (see AGENTS.md). This spec is the authoritative
statement of the session-004 design. It builds on the session-003 spec
([2026-07-04-session-003-list-edit-design.md](2026-07-04-session-003-list-edit-design.md)) and reuses its
`core.Select` ordering and the "a pure read needs no `core` usecase" correction.

## 1. Goal

Give tasks a place in the tree. Create a child under a parent (`mtt add --parent`), render the
epic → task → subtask hierarchy (`mtt tree`), show a task's lineage in `mtt show`, and add the
`list --parent`/`--kind` companion filters. This is the first use of the `parents`-from-config hierarchy
the model has carried since phase 1; back-references (children) are **computed**, never stored.

```
$ mtt add --type task --parent e1 "T"
$ mtt tree [<id>] [--status <s>…] [--kind <k>…] [--depth <n>] [--json]
$ mtt list --parent <id> [--kind <k>…]
$ mtt show <id>            # now prints a lineage breadcrumb
```

## 2. Scope

- **In:**
  - `pkg/mtt` (domain, pure): `Type.AcceptsParent(name) bool` (placement predicate) and
    `Type.StatusKind(status) (StatusKind, bool)` (category lookup for `--kind`). **No port change.**
  - `internal/core`:
    - `Index` — a derived, in-memory hierarchy built from `[]mtt.Task` (no store, no clock): parent→children,
      ancestor chain, roots; cycle-safe. The "resolved graph is derived" (DESIGN.md), not part of the contract.
    - `Match(t, Filter, cfg) bool` — the one shared node predicate (status/type/kind/parent). `Select` is
      refactored to `Match` + sort; the `tree` walk reuses `Match`. A shared sibling comparator (`Created`
      desc, ID tiebreak) is extracted so `Select` and `Index` order identically (DRY).
    - `ListFilter` gains `Parent string` and `Kinds []mtt.StatusKind`.
  - `internal/core/Adder`: `AddParams.Parent`; validate the parent exists (`store.Get`) and that the child
    type accepts the parent's type (`AcceptsParent`). `--parent` is now the normal placement path.
  - `internal/cli`:
    - `add --parent <id>` (mutually exclusive with `--no-parent`).
    - `mtt tree [<id>] [--status…] [--kind…] [--depth <n>] [--json]` — ASCII render + nested `--json`.
    - `list --parent <id>` and `list --kind <…>` (list now loads config, needed for `--kind`).
    - `show` lineage breadcrumb (replaces the deferred-from-002 TODO).
  - `testscript` e2e `tree.txt`; unit tests; doc updates (§11).
- **Out (deferred):**
  - `depends_on` / `ready` / dependency-cycle detection → session 005.
  - `mtt reparent`/`move` (re-parenting) → later (flat IDs enable it, but it is a distinct mutation).
  - `--depends-on` / `--ref` on `add`; `refs` resolution/backlinks → later phases.
  - `tree --sort` (siblings are fixed to `Created` desc in 004).

## 3. Architecture & layer boundaries

Full hexagon `cli → core → port ← adapter` unchanged. `core` imports only `pkg/mtt` — never `adapter/*`.
The domain stays pure (no yaml/json tags). Split by mutation vs pure read (the 002/003 correction):

- **Mutation → `core` usecase.** `add --parent` stays in `core.Adder` (it already validates placement); it
  gains parent-existence + parent-type validation.
- **Pure read → no usecase.** `tree`, `show` lineage, and `list` read `TaskStore.List`/`Get` directly and
  compose pure `core` functions/structures (`Index`, `Match`, `Select`). No store is injected into `Index`
  or `Match`; the CLI passes already-loaded tasks and config.
- **No new port method.** `Create` already persists `Parent` (see the YAML DTO); `Adder` uses `Get` for
  validation; `tree`/`list`/`show` use `List`/`Get`. `List`/`Get`/`Create` suffice.

## 4. Domain: `pkg/mtt` (pure predicates)

```go
// AcceptsParent reports whether a task of this type may sit under a parent of
// type name (name ∈ Parents). A root type (empty Parents) accepts no parent.
func (t Type) AcceptsParent(name string) bool

// StatusKind returns the category of the named status within this type's flow,
// or false when the status is not in the flow (config drift — caller degrades).
func (t Type) StatusKind(status string) (StatusKind, bool)
```

Both are name-agnostic value-object logic (no literals). `AcceptsParent` is the domain half of the
"Hierarchy sanity" invariant; `StatusKind` reuses the existing per-flow `(type, name)` identity.

## 5. Core: `Index` (derived hierarchy) and `Match` (shared predicate)

### 5.1 `Index`

```go
// Index is a derived, read-only view of the parent→children hierarchy over a set
// of tasks. It is built once from a task slice (a pure value; no store, no clock)
// and is not part of the pkg/mtt contract (the resolved graph is derived).
type Index struct{ /* byID, children (keyed by parent ID; "" and dangling → roots) */ }

func NewIndex(tasks []mtt.Task) Index          // buckets sorted by the sibling comparator
func (x Index) Get(id string) (mtt.Task, bool)
func (x Index) Roots() []mtt.Task              // no Parent, or Parent not present (orphans surface here)
func (x Index) Children(id string) []mtt.Task  // direct children, sibling order
func (x Index) Ancestors(id string) []mtt.Task // root-first breadcrumb; cycle-safe (visited set)
```

- **Children are computed** — the inverse of `Parent`, built in `NewIndex`; never stored.
- **Sibling order** = the shared comparator: `Created` desc, tiebreak by ID as an **opaque string**
  (`core` never parses ID structure). Same comparator `Select` uses — extracted from its inline sort (DRY).
- **Cycle-safe.** `Ancestors` walks `Parent` upward with a visited-set; on a cycle or a missing parent it
  stops (no panic, no infinite loop). Re-parenting is deferred, so supported ops cannot create a cycle, but
  the guard is defensive against hand-edited data (per the scaffold's "cycle-safe").
- **Orphans.** A task whose `Parent` does not resolve to a present task is treated as a root (surfaced by
  `tree`, not silently dropped).

### 5.2 `Match` and `Select`

```go
type ListFilter struct {
    Statuses []string
    Types    []string
    Kinds    []mtt.StatusKind
    Parent   string          // "" = no parent filter; else exact Parent match
    Sort     SortKey
}

// Match reports whether t satisfies the filter: within a dimension values are
// OR-ed, across dimensions AND-ed. cfg is consulted only when Kinds is non-empty
// (to resolve the task's status category via its type's flow).
func Match(t mtt.Task, f ListFilter, cfg mtt.Config) bool
```

`Select(tasks, f, cfg)` = keep `Match`-passing tasks, then sort by the shared comparator (respecting
`f.Sort` for Created/Updated). The `tree` walk calls the **same** `Match` for node filtering — one predicate,
two consumers (DRY). A task whose type/status is unknown to `cfg` fails a `Kinds` filter (treated as
non-match), consistent with lazy validation.

> `Select`'s signature grows a `cfg` argument (needed for `--kind`). `cfg` is pure domain data (no ports),
> so `Select` stays a pure function. Session 004 owns this call-site change (only `list` calls it today).

## 6. `add --parent`

- CLI: `--parent <id>`, mutually exclusive with `--no-parent` (`MarkFlagsMutuallyExclusive`).
- `AddParams` gains `Parent string`. `Adder.Add` placement logic becomes:
  - both `--parent` and `--no-parent` → rejected by cobra before `core`.
  - `--parent p` set: `store.Get(p)` → `parent %q not found` if absent; then require
    `childType.AcceptsParent(parent.Type)`, else
    `type %q cannot be placed under type %q (allowed parents: [<list>])`. On success set `Task.Parent = p`.
  - neither flag, non-root type: unchanged — `type %q requires a parent; use --parent <id> (or --no-parent …)`.
  - root type + `--parent`: falls out naturally (`AcceptsParent` = false, empty allowed-parents list).
- The child ID stays **flat per-prefix** (the adapter mints it; identity is decoupled from position). Only
  `Parent` is set. Reserved fields are preserved (round-trip through the DTO already covers this).

## 7. `mtt tree`

```
$ mtt tree            # forest from all roots
$ mtt tree t1         # subtree rooted at t1
```

- Build `Index` from `store.List`; load `cfg` (for `--kind`).
- **Roots:** no `<id>` → `Index.Roots()`; with `<id>` → that single task (`ErrNotFound` → `task %q not found`).
- **Filter — keep-ancestors.** With `--status`/`--kind`, a node is rendered iff it matches (`Match`) **or**
  any descendant matches; non-matching ancestors are kept as the path to a match. No filter → whole tree.
- **`--depth <n>`** = number of visible levels below (and including) each displayed root, like `tree -L n`
  (`--depth 1` = roots only; unset/0 = unlimited). Filter decides the visible set; depth caps render depth.
- **Render (human):** ASCII connectors `├─ ` / `└─ ` with `│  ` / `   ` continuation; each node line is
  `<id>  <type>  [<status>]  <title>` (title omitted when empty) — same columns as `list`.
- **`--json` (nested):**
  ```go
  type treeNodeJSON struct {
      taskJSON                          // id/type/status/title/parent/…  (reused from 003)
      Children []treeNodeJSON `json:"children,omitempty"`
  }
  ```
  Top level is always a JSON array (`[]` when empty — never `null`); leaf `children` are omitted. The nested
  tree honors the same keep-ancestors filter and depth.

## 8. `list --parent` / `--kind`

- `--parent <id>` → `ListFilter.Parent` (direct children only; exact `Parent` match — no recursion).
- `--kind <initial|active|terminal>…` → `ListFilter.Kinds`; `list` now loads `cfg` and passes it to
  `Select`/`Match`. This also settles the deferred "`list` must load config" note from the 003 backlog.
- Human and `--json` output are the existing flat `list` renders (unchanged shape).

## 9. `mtt show` lineage & children  *(format revised post-review — see note)*

- Build `Index` from `store.List`; use `Index.Ancestors(id)` (root-first chain) and `Index.Children(id)`.
- **Lineage breadcrumb** — a "you are here" path from the root **down to and including the task itself**,
  e.g. `  lineage:  e1 › t1 › s1` (the last element is the task). Printed only when the task has ancestors
  (a root task shows no lineage line — the path would be just itself, already in the header).
- **Children line** — `  children:  N (id1, id2, …)` (direct children in sibling order), printed only when
  the task has children. Gives the downward count/ids without running `tree`.
- **The raw `parent:` line is removed** — it was redundant with the breadcrumb's second-to-last element.
- Replaces the `formatTask` TODO at [../../../internal/cli/show.go](../../../internal/cli/show.go).

> **Post-review revision:** the original spec rendered only ancestors (`lineage: e1 › t1`) and kept a
> separate `parent:` line. Review caught that `parent` duplicates the breadcrumb tail. Decision: the
> breadcrumb now includes the node itself (a complete root-to-self path), the redundant `parent:` line is
> dropped, and a `children:` summary line is added (hierarchy info that `show` previously lacked). Consistent
> fields (breadcrumb is always upward ids; children is always a count + ids) — no mixing of id and count in
> one token.

## 10. Errors & edge cases

| Case | Behavior |
|---|---|
| `--parent` + `--no-parent` | cobra mutual-exclusion error (before `core`). |
| `add --parent <missing>` | `parent %q not found`. |
| `add --parent` with a disallowed parent type | `type %q cannot be placed under type %q (allowed parents: […])`. |
| `add --parent` on a root type (e.g. epic) | same "cannot be placed under" error (empty allowed-parents). |
| `tree <missing>` | `task %q not found`. |
| orphan (parent id absent) | surfaced as a root in `tree`; not dropped. |
| cycle in stored data | `Ancestors`/tree walk stop via visited-set (no loop/panic). |
| `--kind` value not a known category | `invalid --kind %q: want initial\|active\|terminal`. |

## 11. Documentation

- **DESIGN.md** (+ **DESIGN.ru.md**): note hierarchy is now rendered (`tree`, `show` lineage) via a derived
  `core.Index`; children computed, back-refs never stored; `add --parent` is the normal placement path.
- **CLI_REFERENCE.md** (+ **.ru.md**): move `tree` / `add --parent` / `list --parent`/`--kind` / `show`
  lineage from "phase 2 / later" to implemented; document keep-ancestors, `--depth`, nested `--json`.
- **CLAUDE.md**: `internal/core` (add `Index`/`Match`, note the shared comparator); `internal/cli` (add
  `tree`, `list` now loads config). `pkg/mtt` (new predicates). Adapter CLAUDE.md unchanged (no store change).
- **sessions/004_hierarchy.md** "Done" section; **NEXT_SESSION.md** handoff.

## 12. Testing (strict TDD: red → green → refactor)

- **Unit (fixed clock):**
  - `pkg/mtt`: `AcceptsParent` (allowed/disallowed/root); `StatusKind` (hit/miss).
  - `core.Index`: `Roots`/`Children`/`Ancestors` (root-first), orphan-as-root, cycle-safety, sibling order.
  - `core.Match`: status/type/parent/kind dimensions, AND across / OR within, kind resolution + config-miss.
  - `core.Adder`: parent ok / missing / wrong-type / root+parent; reserved-field preservation.
  - `tree` render + keep-ancestors + depth (pure render function over `Index`, fixed clock).
- **e2e `tree.txt` (testscript):** `init` → `add --type epic "E"` → `add --type task --parent e1 "T"` →
  `add --type subtask --parent t1 "S"` → `tree` renders `e1`/`t1`/`s1` (anchored asserts, escaped `\[ \]`)
  → `show t1` shows lineage → bad parent (missing + wrong-type) errors → `tree --json`. Sibling **order** is
  not asserted on wall-clock timestamps (proven by the unit test); assert row/line **presence**.

## 13. Carry-over lessons applied

- CLI output via `fmt.Fprint(cmd.OutOrStdout(), …)`; errors via `RunE` (stderr `error: …`).
- testscript asserts anchored (`e1  epic  \[tbd\]`), `\[ \]` escaped; order determinism proven by unit
  tests with a fixed clock, not by e2e against real clocks.
- `golangci unused`: declare each new symbol in the task that first uses it.
- New behavior → keep the affected `CLAUDE.md` current; bilingual docs in sync (English source of truth).
- Preserve reserved task fields on mutation (round-trip through the DTO).
- Zero-match `--json` serializes `[]`, not `null` (top-level tree array; empty `children` omitted).
- Reuse existing CLI plumbing (`projectRoot`, `--dir`/`MTT_DIR`, `--json`, `taskJSON`) — don't duplicate.
