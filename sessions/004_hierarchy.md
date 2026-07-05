# 004 — Hierarchy

Status: planned   ·   Branch: `feat/s004-hierarchy`

## Target

Give tasks a place in the tree: create a child under a parent (`mtt add --parent`), render the
epic → task → subtask hierarchy (`mtt tree`), and show a task's lineage in `mtt show`. First use of the
`parents`-from-config hierarchy that the model has carried since phase 1.

## Scope

- **In:**
  - **`mtt add --parent <id>`** — place a task under an existing parent. Validates the parent exists and
    that the parent's **type** is allowed by the child type's `parents` (from config); the child ID stays
    **flat per-prefix** (`<prefix><N>`, with `Parent` set — identity decoupled from position). This
    completes the `add` placement rule begun in 002 (`--no-parent` was the escape hatch; `--parent` is the
    normal path for a parent-requiring type).
  - **`mtt tree [<id>]`** — render the hierarchy (epic → task → subtask); with `<id>`, root the tree there.
    **Children are computed** (an inverse index built in `core`, not stored). Deterministic sibling order
    (reuse the 003 provider-agnostic ordering).
  - **`mtt show` lineage** — the "you are here" parent-chain line deferred from 002 (needs the in-memory
    index/traversal introduced here).
  - `internal/core` in-memory **task index + hierarchy traversal** (the resolved graph is derived, not part
    of the contract): parent → children, ancestor chain, cycle-safe.
- **Out (explicitly deferred):**
  - `depends_on` / `ready` / cycle detection over dependencies → **005**.
  - `--depends-on` / `--ref` on `add`; `refs` resolution/backlinks → later phases.
  - `mtt reparent`/`move` (re-parenting) → later (enabled by flat IDs, but a distinct operation).
  - `tree` filters `--status`/`--kind`/`--depth` and a `list --parent`/`--kind` filter — pull in only if
    cheap; the session brainstorm decides (a small follow-up otherwise).

## Acceptance (must pass)

- **User scenario:** `mtt add --type epic "E"` → `mtt add --type task --parent e1 "T"` →
  `mtt add --type subtask --parent t1 "S"` → `mtt tree` renders `e1 → t1 → s1`; `mtt show t1` shows its
  lineage; a bad parent (missing, or a type the child may not sit under) errors with guidance.
- **e2e:** `testscript` `tree.txt` covering the above.
- `make check` green.

## Plan (refine at session start — test-first; brainstorm → writing-plans)

Design decisions resolved in brainstorm — authoritative spec:
[../docs/superpowers/specs/2026-07-04-session-004-hierarchy-design.md](../docs/superpowers/specs/2026-07-04-session-004-hierarchy-design.md).
Summary: parent-type validation lives in `core.Adder` (mutation) via a pure `Type.AcceptsParent`; the
computed children/ancestors graph is a derived `core.Index` (no store/clock); a single `core.Match`
predicate is shared by `list` and the `tree` walk (DRY); **full filter set** (`tree --status/--kind/--depth`,
`list --parent/--kind`) is **in scope**; `tree` filtering uses **keep-ancestors** semantics; `tree --json`
is a **nested** tree. No new port method.

- [x] Brainstorm the open questions — resolved (see the spec above).
- [ ] `pkg/mtt` / `internal/core`: parent-type validation + the in-memory index / traversal (children,
      ancestors), cycle-safe; keep back-refs **computed**.
- [ ] `internal/adapter/yaml`: child ID minting is already flat per-prefix — confirm `--parent` sets
      `Parent`; no new store method expected (uses `List`/`Get`/`Create`).
- [ ] `internal/cli`: `mtt add --parent`; `mtt tree`; `mtt show` lineage; `--json` where it fits.
- [ ] `testscript` `tree.txt`; docs (DESIGN/CLI_REFERENCE bilingual, CLAUDE.md, session doc, NEXT_SESSION).

## Done (fill during/after the session)

Shipped (all test-first, `make check` green, version bumped to `0.4.0-dev`):

- **`pkg/mtt`**: pure predicates `Type.AcceptsParent(parentType)` and `Type.StatusKind(status)`.
- **`internal/core`**: `Index` (derived hierarchy — `Roots`/`Children`/`Ancestors`/`Get`, computed children,
  orphans-as-roots, cycle-safe) built from `TaskStore.List`; `Match` (shared status/type/kind/parent
  predicate); `Select` refactored to `Select(tasks, ListFilter, cfg)` using `Match` + the shared
  `lessByRecency` comparator; `Adder` validates `--parent` (exists + `AcceptsParent`).
- **`internal/cli`**: `mtt add --parent <id>` (mutually exclusive with `--no-parent`); `mtt tree [<id>]`
  (ASCII render, keep-ancestors `--status`/`--kind`, `--depth`, nested `--json`); `mtt list --parent`/`--kind`;
  `mtt show` lineage breadcrumb (root-to-self path) + children summary line (the raw `parent:` line dropped
  as redundant — a post-review refinement); shared `taskLine`/`parseKinds` helpers.
- **Tests**: unit (fixed clock) for predicates, `Index`, `Match`, `Adder`, `renderTree`/`buildTreeJSON`;
  e2e `tree.txt` + extended `add_show.txt`/`list_edit.txt`.
- **Docs**: DESIGN.md/.ru (hierarchy section), CLI_REFERENCE.md/.ru (add/show/list/tree), CLAUDE.md
  (pkg/mtt, core, cli).

Decisions (full detail in the spec): parent-type validation is a `core.Adder` mutation using a pure domain
predicate; the children/ancestors graph is a derived `core.Index` (no store/clock, not in the contract);
one `core.Match` predicate serves `list` and `tree` (DRY); full filter set is in scope; `tree` filtering is
**keep-ancestors**; `tree --json` is a **nested** tree. No new `TaskStore` method (`Create` already persists
`Parent`).

Deferred (unchanged): `depends_on`/`ready`/dependency cycles → 005; `reparent`/`move`; `--depends-on`/`--ref`
on `add`; `tree --sort`.
