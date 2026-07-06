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
- `Ready` (pure read): actionable tasks — status non-terminal AND every `depends_on` terminal, resolved by
  category (`kindOf` → `Type.StatusKind`). **Conservative**: a dangling blocker or a config-drift status
  (unresolvable) leaves a task not-ready. No store/clock; ordered by the shared `lessByRecency`. One
  primitive behind both `mtt ready` and `list --ready` (`Select(Ready(...), filter, cfg)`). `kindOf` DRYs
  the type→`StatusKind` lookup shared with `matchesKind`.
- `DependencyEditor` (mutation): `AddDependency`/`RemoveDependency` edit `Task.DependsOn` and persist via
  `TaskStore.Update` (**no new port** — the edge rides the field, like `parent` in s004). Rejects a
  self-edge and, via `DepGraph.Reaches`, any edge that would create a **cycle**; a duplicate add is an
  idempotent no-op; removing an absent edge is likewise an idempotent no-op (the task must exist). Bumps
  `updated` from the injected clock on a real change.
- `Runner` (driven **port**, defined here — the first beyond storage): `Run(commands) ([]mtt.Check, error)`.
  Implemented by `internal/adapter/exec`, **faked** in tests. A non-zero exit is **data** (a `Check`), not a
  Go error; the error is only an operational failure. No `dir` param — the exec adapter holds `cwd=root`, so
  `core` stays free of filesystem paths.
- `Transitioner` (the flow-gate usecase, a mutation): `Transition(id, to, TransitionOptions{Role,By,NoRun})`
  applies a **single** edge — a **linear lookup** in `Type.Transitions` (no `ResolvedFlow` yet; it earns its
  keep in s007's multi-edge walk), gate via `Runner` (any non-zero check → `ErrBlocked`, task unchanged, no
  history), append a `HistoryEntry` (`from/to/at/by/role/checks`), persist via `TaskStore.Update` (**no new
  port** — history rides `Task.History`, GAP #1 rule). `ErrBlocked`/`ErrInvalidTransition` are core sentinels
  (flow is core policy); the CLI maps them to exit codes 3/6.
- `DepGraph` (pure derived graph over `depends_on`, parallel to `Index` over `parent`): built from a task
  slice (`NewDepGraph`) — no store/clock, not in the `pkg/mtt` contract. Exposes `Get`/`DependsOn` (stored
  order, dangling kept)/`Dependents` (**computed** reverse edges)/`Reaches` (cycle-check)/`Cycles`
  (defensive). Cycle-safe (visited-set); sibling order matches `Select`. Kept **separate** from `Index`
  (GAP #6 not extracted — `parent` is a single-parent tree, `depends_on` a multi-edge DAG).

## Identities

- `ListFilter`, `Index`, `AddParams`, and `Editor` use the named `pkg/mtt` identities (`TaskID`/`TypeName`/
  `StatusName`), not bare `string`. `anyOrEmpty[T comparable]` is generic so it serves both status and type
  filters. Conversion to/from `string` happens at the CLI (arg parsing) and the adapter (DTO), never here.
- `Ready`, `DependencyEditor`, and `DepGraph` use `mtt.TaskID` throughout; string conversion stays at the
  cli/adapter boundary.

## Boundaries

- No storage access, no ID minting, no output formatting, no YAML — those live in the adapter / CLI.
- The clock is injected (`now func() time.Time`) for deterministic tests.
- Policy lives here; the pure primitives it composes (`IsRoot`, `InitialStatus`, `TypeByName`, `DefaultType`)
  live in `pkg/mtt`.
