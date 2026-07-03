# pkg/mtt

The **public domain contract**: pure types + (later) ports. No CLI, no files, no YAML.

## Rules

- **No serialization concerns**: no yaml/json struct tags, no adapter fields (e.g. `prefix`). Adapters own DTOs and map to/from these types.
- **Value objects over primitives**: `StatusKind` is a closed vocabulary (initial/active/terminal) the code reasons about. Type/status **names** are open and user-defined — never literals in code.
- **References by identity**: transitions reference statuses by name; cross-aggregate links are names/IDs, never pointers. Back-references (e.g. `ChildrenIn`) are **computed**, not stored.
- **Provider-agnostic**: mandatory minimum (a Type needs a name + a flow with statuses(name+kind) and transitions(from/to)); the rest is optional.

## Invariants (Config.Validate — structural, name-agnostic)

kind↔topology (initial: 0 in/≥1 out; active: ≥1 in/≥1 out; terminal: ≥1 in/0 out); ≥1 of each kind per flow; unique type/status names; transition refs resolve; ≤1 default type; parents exist and are not self.

## Boundaries

Does NOT: read/write storage, mint IDs, or format output. Those live in adapters / CLI.
