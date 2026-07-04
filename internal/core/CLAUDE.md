# internal/core

Usecase logic. Depends **only** on the `pkg/mtt` domain contract and its ports — **never** on `adapter/*`.

## Responsibilities

- `Adder` (the `add` usecase): resolve the type (`--type` or the config default), enforce placement
  (a non-root type needs `--no-parent` here since `--parent` is session 004), pick the entry status
  (`Type.InitialStatus` — default-marked initial, else first initial), stamp `created`/`updated` from an
  **injected clock**, and persist via `TaskStore.Create` (which mints the ID in the adapter).
- `Select` (pure read): filter tasks by status/type (AND across dimensions, OR within) and impose a
  deterministic order — `Created` desc by default (or `Updated`), tie-broken by ID as an **opaque string**
  (never parsing ID structure; provider-agnostic). No store injected — a pure function the CLI composes with
  `TaskStore.List` (a pure read needs no usecase; the only logic is the filter/sort). Reused later by
  `ready`/`tree`.
- `Editor` (the `edit` usecase, a mutation): load via `TaskStore.Get`, apply only the provided
  title/description (nil pointer = unchanged), enforce the title-or-description invariant, bump `updated`
  from the **injected clock**, persist via `TaskStore.Update`. id/type/status/parent are not editable here.

## Boundaries

- No storage access, no ID minting, no output formatting, no YAML — those live in the adapter / CLI.
- The clock is injected (`now func() time.Time`) for deterministic tests.
- Policy lives here; the pure primitives it composes (`IsRoot`, `InitialStatus`, `TypeByName`, `DefaultType`)
  live in `pkg/mtt`.
