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

- [ ] Brainstorm the open questions: parent-type validation placement (core vs adapter); where the
      computed children index lives (pure `core` over `TaskStore.List`, mirroring 003's `Select`); `tree`
      rendering + deterministic sibling order; how much of `tree`/lineage filtering lands now.
- [ ] `pkg/mtt` / `internal/core`: parent-type validation + the in-memory index / traversal (children,
      ancestors), cycle-safe; keep back-refs **computed**.
- [ ] `internal/adapter/yaml`: child ID minting is already flat per-prefix — confirm `--parent` sets
      `Parent`; no new store method expected (uses `List`/`Get`/`Create`).
- [ ] `internal/cli`: `mtt add --parent`; `mtt tree`; `mtt show` lineage; `--json` where it fits.
- [ ] `testscript` `tree.txt`; docs (DESIGN/CLI_REFERENCE bilingual, CLAUDE.md, session doc, NEXT_SESSION).

## Done (fill during/after the session)

—
