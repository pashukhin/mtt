# pkg/mtt

The **public domain contract**: pure types + (later) ports. No CLI, no files, no YAML.

## Rules

- **No serialization concerns**: no yaml/json struct tags, no adapter fields (e.g. `prefix`). Adapters own DTOs and map to/from these types.
- **Value objects over primitives**: `StatusKind` is a closed vocabulary (initial/active/terminal) the code reasons about. Type/status **names** are open and user-defined — never literals in code.
- **Named identities** (`identity.go`): `TaskID`, `TypeName`, `StatusName` are named string types so the domain's opaque references can't be mixed at compile time (a `TypeName` can't be passed where a `TaskID` is wanted). They stay **opaque** (nothing parses an id's structure — that's adapter-specific) and marshal as plain strings. Smart constructors `NewTaskID`/`NewTypeName`/`NewStatusName` **reject empty, never transform** (byte-for-byte round-trip); "normalize" is a reserved extension point. Existence is checked **contextually** (`Config.Validate`/usecases), never in a constructor. `Ref.ID` stays a plain `string` (heterogeneous target — `TaskID`/`NoteSlug`/URL by `Kind`).
- **References by identity**: transitions reference statuses by name; cross-aggregate links are names/IDs, never pointers. Back-references (e.g. `ChildrenIn`) are **computed**, not stored.
- **Type queries** (pure predicates, name-agnostic): `IsRoot`, `InitialStatus`, `ChildrenIn`, plus `AcceptsParent(parentType)` (placement rule: `parentType ∈ Parents`; a root type accepts none) and `StatusKind(status)` (category of a status in the type's flow, `false` on miss — used by `--kind` filtering).
- **Provider-agnostic**: mandatory minimum (a Type needs a name + a flow with statuses(name+kind) and transitions(from/to)); the rest is optional.
- **Task model**: `Task` (id/type/title/status/parent + reserved `tags`/`depends_on`/`refs`/`comments`/`history`), value objects `RefKind` and (existing) `StatusKind`. `Status.Default` marks the entry status when a flow has >1 initial (mirrors `Type.Default`; resolved by `Type.InitialStatus`).
- **Command VO** (`command.go`, s007): `Transition.Commands` is `[]Command`, a value object `{Run string, Timeout time.Duration}`. `Run` holds a **raw template** (e.g. `git checkout -b task/{{.ID}}`) — the domain **does not interpret it** (template-agnostic; `core` expands placeholders before running). `Timeout` is an optional per-command override of the adapter global `command_timeout` (zero = fall back). `Valid()` (non-empty `Run`, non-negative `Timeout`, and — if present — a well-formed **leaf** `Rollback`) is checked in `Config.Validate`. On disk a command is a bare scalar **or** a `{run, timeout, rollback}` map (the adapter's `ymlCommand` maps both here). **`Rollback *Command`** (s008): an optional per-command **compensator** run in reverse over the already-succeeded commands when a later command in the same pipeline fails (intra-pipeline compensation). A compensator is a **leaf** — its own `Rollback` must be nil (enforced by `Valid()`).
- **Port**: `TaskStore` — `Create` (mints the ID in the adapter), `Get` (`ErrNotFound`), `List` (all tasks; order unspecified — callers order), `Update` (overwrite existing by ID; `ErrNotFound` if absent), `Delete` (remove by ID; `ErrNotFound` if absent — the *D* in CRUD, a store op not an embedded field, so it lives on the base port; an archive-only external adapter may return `ErrUnsupported`). Pure — no prefix/YAML leaks through it.

## Invariants (Config.Validate — structural, name-agnostic)

kind↔topology (initial: 0 in/≥1 out; active: ≥1 in/≥1 out; terminal: ≥1 in/0 out); ≥1 of each kind per flow; unique type/status names; transition refs resolve; ≤1 default type; parents exist and are not self; ≤1 `default` status per flow; a `default` status must be `initial`.

## Boundaries

Does NOT: read/write storage, mint IDs, or format output. Those live in adapters / CLI.
