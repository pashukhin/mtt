# pkg/mtt

The **public domain contract**: pure types + (later) ports. No CLI, no files, no YAML.

## Rules

- **No serialization concerns**: no yaml/json struct tags, no adapter fields (e.g. `prefix`). Adapters own DTOs and map to/from these types.
- **Value objects over primitives**: `StatusKind` is a closed vocabulary (initial/active/terminal) the code reasons about. Type/status **names** are open and user-defined — never literals in code.
- **References by identity**: transitions reference statuses by name; cross-aggregate links are names/IDs, never pointers. Back-references (e.g. `ChildrenIn`) are **computed**, not stored.
- **Type queries** (pure predicates, name-agnostic): `IsRoot`, `InitialStatus`, `ChildrenIn`, plus `AcceptsParent(parentType)` (placement rule: `parentType ∈ Parents`; a root type accepts none) and `StatusKind(status)` (category of a status in the type's flow, `false` on miss — used by `--kind` filtering).
- **Provider-agnostic**: mandatory minimum (a Type needs a name + a flow with statuses(name+kind) and transitions(from/to)); the rest is optional.
- **Task model**: `Task` (id/type/title/status/parent + reserved `tags`/`depends_on`/`refs`/`comments`/`history`), value objects `RefKind` and (existing) `StatusKind`. `Status.Default` marks the entry status when a flow has >1 initial (mirrors `Type.Default`; resolved by `Type.InitialStatus`).
- **Port**: `TaskStore` — `Create` (mints the ID in the adapter), `Get` (`ErrNotFound`), `List` (all tasks; order unspecified — callers order), `Update` (overwrite existing by ID; `ErrNotFound` if absent). Pure — no prefix/YAML leaks through it.

## Invariants (Config.Validate — structural, name-agnostic)

kind↔topology (initial: 0 in/≥1 out; active: ≥1 in/≥1 out; terminal: ≥1 in/0 out); ≥1 of each kind per flow; unique type/status names; transition refs resolve; ≤1 default type; parents exist and are not self; ≤1 `default` status per flow; a `default` status must be `initial`.

## Boundaries

Does NOT: read/write storage, mint IDs, or format output. Those live in adapters / CLI.
