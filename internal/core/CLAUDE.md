# internal/core

Usecase logic. Depends **only** on the `pkg/mtt` domain contract and its ports — **never** on `adapter/*`.

## Responsibilities

- `Adder` (the `add` usecase): resolve the type (`--type` or the config default), enforce placement
  (a non-root type needs `--no-parent` here since `--parent` is session 004), pick the entry status
  (`Type.InitialStatus` — default-marked initial, else first initial), stamp `created`/`updated` from an
  **injected clock**, and persist via `TaskStore.Create` (which mints the ID in the adapter).

## Boundaries

- No storage access, no ID minting, no output formatting, no YAML — those live in the adapter / CLI.
- The clock is injected (`now func() time.Time`) for deterministic tests.
- Policy lives here; the pure primitives it composes (`IsRoot`, `InitialStatus`, `TypeByName`, `DefaultType`)
  live in `pkg/mtt`.
