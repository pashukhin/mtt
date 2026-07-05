# internal/core

Usecase logic. Depends **only** on the `pkg/mtt` domain contract and its ports — **never** on `adapter/*`.

## Responsibilities

- `Adder` (the `add` usecase): resolve the type (`--type` or the config default), enforce placement
  (`--parent <id>`: the parent must exist — `TaskStore.Get` — and its type must satisfy `AcceptsParent`;
  else a non-root type needs `--no-parent`), pick the entry status (`Type.InitialStatus` — default-marked
  initial, else first initial), stamp `created`/`updated` from an **injected clock**, and persist via
  `TaskStore.Create` (which mints the ID in the adapter).
- `Match` (pure predicate): reports whether a task satisfies a `ListFilter` — status/type/kind/parent (AND
  across dimensions, OR within). `cfg` is consulted only for the kind dimension (resolve the task's status
  category via its type's flow). Shared by `Select` **and** the CLI's `tree` walk (one predicate, two
  consumers — DRY).
- `Select` (pure read): `Match`-filter tasks, then impose a deterministic order — `Created` desc by default
  (or `Updated`), tie-broken by ID as an **opaque string** (never parsing ID structure; provider-agnostic).
  No store injected — a pure function (`Select(tasks, ListFilter, cfg)`) the CLI composes with
  `TaskStore.List`. The sibling comparator (`lessByRecency`) is shared with `Index`.
- `Index` (pure derived hierarchy): built from a task slice (`NewIndex`) — no store, no clock; **not** part
  of the `pkg/mtt` contract (the resolved graph is derived). Exposes `Roots`/`Children`/`Ancestors`/`Get`;
  children are **computed** (inverse of `parent`), never stored; orphans (dangling parent) surface as roots;
  `Ancestors` is cycle-safe (visited-set). Sibling order matches `Select`. Consumed by `tree` and `show`
  lineage (pure reads — no usecase).
- `Editor` (the `edit` usecase, a mutation): load via `TaskStore.Get`, apply only the provided
  title/description (nil pointer = unchanged), enforce the title-or-description invariant, bump `updated`
  from the **injected clock**, persist via `TaskStore.Update`. id/type/status/parent are not editable here.

## Boundaries

- No storage access, no ID minting, no output formatting, no YAML — those live in the adapter / CLI.
- The clock is injected (`now func() time.Time`) for deterministic tests.
- Policy lives here; the pure primitives it composes (`IsRoot`, `InitialStatus`, `TypeByName`, `DefaultType`)
  live in `pkg/mtt`.
