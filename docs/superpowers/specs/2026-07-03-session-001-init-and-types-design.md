# Session 001 — Init & inspect — Design Spec

Date: 2026-07-03 · Branch: `feat/s001-init-and-types` · Session: [../../../sessions/001_init_and_types.md](../../../sessions/001_init_and_types.md)

Language note: agent-facing docs are English only (see AGENTS.md). This spec is the authoritative
statement of the session-001 design **and** of a domain-model correction that supersedes some wording in
DESIGN.md/NEXT_SESSION.md/TASKS.md (see §10 for the exact doc reconciliation).

## 1. Goal

Deliver the first vertical slice: initialize a project and inspect its configured task types and flow.

```
$ mtt init [--template default|coding] [--force] [--name <name>]
$ mtt types [<type>]
```

This exercises the contract (`pkg/mtt`), the YAML adapter's config layer, and the CLI end-to-end.

## 2. Scope

- **In:**
  - `mtt init [--template default|coding] [--force] [--name <name>]` — write the starter `.mtt/config.yaml`.
  - `mtt types [<type>]` — print the configured types with their flow.
  - `pkg/mtt` contract: `Config`, `Project`, `Type`, `Flow`, `Status` (+ `kind`), `Transition`, plus
    `Config.Validate()` and pure helpers — enough to load, validate, and display.
  - YAML adapter: root discovery, embedded templates (`default`, `coding`), atomic write, load + optional
    gitignored `config.local.yaml` overlay merge.
  - Structural, name-agnostic config invariants (see §4).
- **Out (deferred):** tasks (`add`/`show`/`list`/`edit`) → session 002+; `refs`; capabilities/`mtt caps`;
  command **execution** (commands are inert data here — run in phase 3); recategorization; any non-YAML adapter.

## 3. Domain model (corrected)

This supersedes the "canonical anchors `tbd → in_progress → done`" and "default `task`" wording in the
current docs. The driving principle (already in DESIGN.md) is honored strictly: **the code contains no
literals for type/status names or ID structure.** Names live only in the config template (a text constant).

Key facts:

1. **Status kinds are `initial` / `active` / `terminal`**, defined by flow topology:
   - `initial` — no incoming transition (and ≥1 outgoing);
   - `active` — ≥1 incoming and ≥1 outgoing;
   - `terminal` — no outgoing transition (and ≥1 incoming).
2. **A flow is a per-type closed graph.** Status identity is the pair `(type, name)` — same-named statuses
   in different flows are **different** statuses with potentially different semantics. There are **no
   cross-flow transitions**: a transition's `from`/`to` always reference statuses of the same type.
3. The status graph across all types is therefore a **forest of disjoint per-type flows**.
4. **No named anchors.** Default statuses are whatever the template says (`tbd`/`in_progress`/`done` in the
   default template are just names); nothing in code checks for them.
5. **Multiple `initial` statuses are allowed** (e.g. `to_implement` / `to_describe`). We do not decide how
   many entry states a user wants — we enable it. (Consequence for `add`, deferred: with >1 initial, the
   entry status must be resolved — a per-type entry marker or `--status`; decided in session 002.)
6. **Reopen is modeled as a transition into a separate `active` status**, never back into `initial` and
   never out of a `terminal`. This keeps task history linear and honest. The default template therefore has
   **no `in_progress → tbd` edge**.
7. **The default type** (used by `add` without `--type`) is marked by a boolean `default: true` — not by the
   literal name `task`. Domain is tolerant (see `DefaultType` in §5); the *full* YAML provider must mark
   exactly one (an adapter-level check, §4).
8. **A task's type is immutable.** Recategorization = close the old task (move to a terminal) + create a new
   task of the target type + link them via `refs` (kind `task`, backlinks both ways). Deferred well beyond
   001; recorded here because it justifies keeping `refs` in the model from phase 1.
9. **`kind` is a value object, not a name.** Names (type/status names) are *open* and user-defined → never
   literals in code (name-agnostic). `kind` is a *closed* domain vocabulary of exactly three values that the
   code reasons about (ready/terminal logic) → a distinct `StatusKind` type with constants. This does **not**
   violate "no literals": it is the very abstraction that lets names stay out of the code. (DDD: model the
   domain vocabulary explicitly.)
10. **Optional descriptions.** `Type`, `Status`, and `Transition` each carry an optional `Description`
    (human/agent orientation). Absent for minimal providers.
11. **Provider-agnostic domain.** Types and flows may come from an external tracker, not only our YAML store.
    So the domain requires a **mandatory minimum** and treats the rest as optional (see §5). Our YAML adapter
    is the *full provider* (supplies everything); an external provider supplies the minimum and is wired
    later via a dedicated command (`mtt connect`, deferred). Mandatory minimum per `Type`: `Name` + a `Flow`
    whose statuses have `Name`+`Kind` and whose transitions have `From`/`To`. Optional: `Description`,
    `Parents`, `Default`, `Commands` (the last is *our* local gate augmentation — external trackers don't
    supply it). `prefix` is not domain at all (adapter/ID-encoding only).

## 4. Config invariants (structural, name-agnostic)

Two homes, matching the provider-agnostic split (§3.11):

**Domain — `Config.Validate()`** (applies to any provider; **no name literals**):

- **Non-empty:** ≥ 1 type.
- **References resolve:** every `transition.from`/`to` names a status that exists in the same type's flow.
- **Unique names:** type names are unique; status names are unique within a type.
- **kind ↔ topology consistency** (declared kind must match computed degree):
  - `initial` ⇒ in-degree 0, out-degree ≥ 1;
  - `active` ⇒ in-degree ≥ 1, out-degree ≥ 1;
  - `terminal` ⇒ out-degree 0, in-degree ≥ 1.
  (This also rejects an isolated status with no edges.)
- **kind completeness per flow:** ≥ 1 `initial`, ≥ 1 `active`, ≥ 1 `terminal`. Consequence, accepted
  consciously: **a 2-status flow is invalid** (minimum is `initial → active → terminal`).
- **At most one default:** ≤ 1 `Type` has `default: true`. (`DefaultType` falls back to the first type when
  none is marked — tolerant of minimal providers.)
- **Hierarchy sanity:** every entry in a type's `parents` names an existing type; a type is not its own parent.

**Adapter — YAML-specific** (our full provider must be stricter):

- **Exactly one default type** in the committed config.
- **Prefix present, unique, non-empty** across types (needed for ID minting; `prefix` is adapter-only).

`Config.Validate()` returns a typed, aggregated error listing all violations (not just the first), each
naming the offending type/status. The YAML adapter surfaces its extra checks the same way.

## 5. Contract `pkg/mtt` (pure domain — no serialization concerns)

The domain types carry **no yaml/json tags and no `prefix`** — serialization and ID-encoding live in the
adapter (§6). This keeps the contract provider-agnostic (DDD/hexagonal). Field order is chosen for readable
diffs; the *file* layout is the adapter's concern.

```go
// StatusKind — closed domain vocabulary (a value object), not a name.
type StatusKind string
const (
    KindInitial  StatusKind = "initial"
    KindActive   StatusKind = "active"
    KindTerminal StatusKind = "terminal"
)
func (k StatusKind) Valid() bool // one of the three constants

type Config struct {
    Version int      // optional
    Project Project  // optional metadata
    Types   []Type   // MANDATORY: >= 1
}
type Project struct { Name string } // optional

type Type struct {
    Name        string   // MANDATORY
    Description string   // optional
    Parents     []string // optional; allowed parent types, empty = root level
    Default     bool     // optional; at most one type true (adapter: exactly one)
    Flow                 // MANDATORY (embedded)
}
type Flow struct {
    Statuses    []Status     // MANDATORY: >= 1, covering all three kinds
    Transitions []Transition // MANDATORY: defines the topology
}
type Status struct {
    Name        string     // MANDATORY
    Kind        StatusKind // MANDATORY
    Description string     // optional
}
type Transition struct {
    From        string   // MANDATORY
    To          string   // MANDATORY
    Description string   // optional
    Commands    []string // optional; OUR local gate augmentation (external providers omit it);
                         // inert data in session 001, executed in phase 3
}

func (c Config) Validate() error          // domain invariants of §4; pure, no I/O
func (c Config) DefaultType() (Type, bool) // marked default, else first type; false only if no types
func (t Type) ChildrenIn(c Config) []Type  // derived by inverting Parents (display/validation)
```

Notes:
- **References are by identity, not pointers** (DDD: reference across aggregates by identity). Within a flow,
  `Transition.From`/`To` are status *names*; across aggregates (types, and later tasks) references are
  names/IDs. This keeps the contract serializable and provider-agnostic (an external provider yields IDs, not
  a resolved graph — pointers would force full eager loads).
- **Back-references are computed, never stored** (e.g. `ChildrenIn`; later, backlinks) — an inverse index, so
  forward refs stay the single source of truth (no sync/cycle hazards).
- A **resolved object graph with back-references** (for traversal: ready / advance-walk / cycle detection) is
  a *derived, in-memory* structure built by `internal/core` when traversal is needed (phase 2+, e3_t1) — not
  part of the contract, not built in session 001 (validate/display need only flat structs + on-demand helpers).
- **Mandatory minimum vs optional** is documented per field above — this is what an external provider must
  supply vs may omit (§3.11). Validation (§4) enforces the mandatory structure.
- **`prefix` is not in the contract** — the adapter owns it (§6).
- `pkg/mtt/CLAUDE.md` documents: responsibility (pure domain contract), value objects (`StatusKind`),
  mandatory-minimum boundary, and the "no name literals / no serialization tags" rules.

## 6. YAML adapter `internal/adapter/yaml` (config-as-data)

The adapter exposes **package functions** for config bootstrap; it does not (yet) implement a domain port.
Rationale: config is what you bootstrap adapters *from* (DESIGN: "CLI assembles adapters from config"), a
chicken-and-egg that predates any port. `TaskStore` arrives in session 002 with tasks.

```go
func FindRoot(start string) (root string, err error)     // walk up to a dir containing .mtt/ (like git)
func Init(root, template, name string, force bool) error // render embedded template, atomic write
func Load(root string) (cfg mtt.Config, prefixes map[string]string, err error)
```

- **`FindRoot`** walks parents from `start` until it finds `.mtt/`; returns a typed "not initialized" error
  if none. `init` targets the current directory (creates `.mtt/` there).
- **`Init`** renders the selected embedded template through `text/template` (only substitution: `{{.Name}}`,
  default = base name of the target dir), then writes `.mtt/config.yaml` **atomically** (temp + rename).
  Refuses to overwrite an existing `.mtt/config.yaml` unless `force` is set.
- **DTOs + mapping (DDD split):** the adapter has its own serialization structs (`ymlConfig`, `ymlType`,
  `ymlStatus`, `ymlTransition`) carrying the yaml tags **and** `prefix`. It maps them to the pure `mtt`
  domain types. The domain never sees YAML; the adapter never leaks yaml/prefix into the domain.
- **`Load`** reads `.mtt/config.yaml`, deep-merges an optional gitignored `.mtt/config.local.yaml` overlay
  (later layer wins; per key/section), unmarshals into the DTOs, maps to `mtt.Config`, and runs the
  YAML-specific checks of §4 (prefix present/unique; exactly one default). It returns the domain
  `mtt.Config` plus a `type name → prefix` map. Domain invariants are the caller's `cfg.Validate()` call —
  the adapter only adds its provider-specific checks (keeps business rules out of the adapter).
- **Templates** live as files embedded via `go:embed` (`templates/default.yaml`, `templates/coding.yaml`):
  written verbatim so comments and the readable inline flow-style survive. Golden tests pin their rendered
  output. Content sketched in §8.
- Overlay merge for session 001 is deliberately minimal but real (a generic deep-merge of the parsed
  structure; exercised in tests by overriding `project.name`). Richer per-adapter connection schemas come
  with external adapters.
- `internal/adapter/yaml/CLAUDE.md` documents: responsibility (config I/O + ID encoding), the "no business
  rules" boundary, atomic-write and root-discovery invariants, and that `prefix` is adapter-owned.

## 7. CLI `internal/cli` (thin; composition root)

The CLI is the composition root: it may import the adapter and `pkg/mtt`, wire them, and format output.

- `mtt init [--template default|coding] [--force] [--name <name>]` → resolve target dir → `yaml.Init(...)`
  → print a short confirmation (path written, template used).
- `mtt types [<type>]` → `yaml.FindRoot` → `yaml.Load` → `cfg.Validate()` (fail with a clear message) →
  block formatter. With `<type>`, filter to that one type (error if unknown).

**`mtt types` output — one block per type** (meets acceptance: statuses with kinds *and* transitions,
including per-transition descriptions/commands, are visible directly):

```
epic  (prefix e · root)
  A large body of work spanning multiple tasks.
  statuses:  tbd[initial]  in_progress[active]  done[terminal]  cancelled[terminal]
  flow:
    tbd          -> in_progress
    tbd          -> cancelled
    in_progress  -> done        # all epic tasks closed
    in_progress  -> cancelled

task  (prefix t · parent epic · default)
  ...

# coding template — command gates are shown under the transition:
feature  (prefix f · root · default)
  flow:
    in_progress  -> done        # quality gate
                                $ make lint
                                $ make test
```

Formatting details (alignment, ASCII `->`, `[kind]`, header markers) are fixed by the golden/testscript
outputs and may be tuned during implementation as long as acceptance holds. Prefix/parent/default/children
shown from the adapter view + derived helpers.

## 8. Templates (illustrative content — tune exact strings in the plan)

**`default`** — `epic` → `task` → `subtask` (config order = display order), canonical linear flow, no
reopen edge, `task` is default:

```yaml
version: 1
project:
  name: {{.Name}}
types:
  - name: epic            # (prefix e, root: parents [])
    description: A large body of work spanning multiple tasks.
    # ...same four statuses/kinds; transitions minus the reopen edge...
  - name: task            # DEFAULT (add without --type)
    description: A unit of work.
    prefix: t
    parents: [epic]
    default: true
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, description: "review the spec, create a branch"}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, description: "quality gate"}
      - {from: in_progress, to: cancelled}
  - name: subtask         # (prefix s, parents [task])
    description: A small step within a task.
    # ...same four statuses/kinds and transitions...
```

(Every type carries the same four statuses/kinds; only `task` is `default: true`. Type order in the file is
the display order in `mtt types`.)

**`coding`** — `feature` / `bugfix` / `refactor` (root-level, self-contained), same linear flow, differing by
`prefix` + command gates that encode a per-type Definition of Done. Commands are **inert data** in session
001 (shown by `mtt types`; executed only in phase 3). `feature` is the default.

- `feature` (prefix f): `in_progress → done` gate `["make lint", "make test"]`.
- `bugfix` (prefix b): `tbd → in_progress` gate "a failing test exists first" (illustrative
  `["! make test"]`); `in_progress → done` gate `["make lint", "make test"]`.
- `refactor` (prefix r): `in_progress → done` gate "no public-API change + green"
  (illustrative `["git diff --exit-code -- pkg/", "make lint", "make test"]`).

Command **execution semantics** (shell vs argv, timeouts, cwd) are a phase-3 decision; here the strings are
only data, so exact wording is not load-bearing.

## 9. Layering decisions (explicit)

- **config-as-data** — core does not receive a config port; the adapter exposes package functions and the
  CLI (composition root) wires them; domain invariants live on `Config.Validate()`.
- **Domain ≠ serialization (DDD)** — `pkg/mtt` is pure (no yaml tags, no `prefix`); the YAML adapter has its
  own DTOs and maps to/from the domain. `prefix` lives only in the adapter's DTO + a `type→prefix` map.
- **Provider-agnostic domain** — mandatory-minimum fields required, the rest optional (§3.11); enables an
  external type/flow provider later (`mtt connect`, deferred) without touching the contract.
- **`internal/core` deferred to session 002** — with config-as-data, session 001's only domain logic is
  `Config.Validate()` + pure helpers (in `pkg/mtt`), and orchestration lives in the composition root. An
  empty pass-through `core` would violate KISS and risk a `core → adapter` import (forbidden). `core`
  appears in 002 with task usecases. (This deviates from the session-001 plan bullet that listed a
  `internal/core` config usecase — deviation is intentional and recorded here.)

## 10. Doc reconciliation (implementation task 0)

Before/with implementation, sync the source-of-truth docs to the corrected model:

- **AGENTS.md** §Principles: add **DDD** ("fanatical adherence") to the SOLID/DRY/KISS/TDD/clean-arch list —
  model the domain explicitly (value objects like `StatusKind`), keep it free of infrastructure/serialization
  concerns, mandatory-minimum vs optional fields for provider-agnosticism.
- **DESIGN.md** + **DESIGN.ru.md** §"Model invariants": replace named anchors + "default `task`" with the
  structural invariants of §4; add flow-scoped status identity, no cross-flow transitions, immutable-type +
  recategorization principle, and the provider-agnostic domain (mandatory minimum vs optional; external
  type/flow provider via a future `mtt connect`); update the default-config example (drop `in_progress →
  tbd`; add `default`/`description`/`parents`).
- **NEXT_SESSION.md** and **TASKS.md** (e2_t2, e2_t3): drop the anchor/`task`-literal wording; reflect
  `default:true`, `parents[]`, structural invariants.
- **sessions/001_init_and_types.md**: update scope/acceptance to structural invariants and name-agnostic
  phrasing (types come from the template, not asserted as fixed anchors).

## 11. Testing strategy (test-first)

- **`pkg/mtt`** — table-driven tests for `Validate()`: one red case per **domain** invariant in §4 (empty
  types, unresolved transition, duplicate names, kind/topology mismatch, missing kind, **two** defaults, bad
  parent, self parent, 2-status flow), plus green cases; `StatusKind.Valid`; `DefaultType` (marked / fallback
  to first / no types); `ChildrenIn`.
- **`internal/adapter/yaml`** — `FindRoot` (found/not-found/nested); `Init` in a temp dir (creates file;
  refuses without `--force`; overwrites with it; `{{.Name}}` substitution/default); `Load` + DTO→domain
  mapping + overlay merge; adapter-specific checks (**exactly one** default; prefix present/unique);
  **golden** tests for rendered `default.yaml` and `coding.yaml` (`-update` to regenerate).
- **`internal/cli`** — `testscript` `init.txt`: `init` creates `.mtt/config.yaml`; `types` prints the
  expected blocks; second `init` without `--force` errors; `init --force` overwrites; `init --template
  coding` shows feature/bugfix/refactor with gates. No network; temp dirs.

## 12. Acceptance (must pass)

- In an empty dir, `mtt init` creates `.mtt/config.yaml`; `mtt types` prints the configured types with
  statuses (kinds) and transitions. `mtt init --template coding` yields feature/bugfix/refactor with a gated
  per-type DoD, visible via `mtt types`.
- `testscript init.txt` passes (init → assert file + `types` output; `--force` overwrite; re-init errors).
- Golden test for the generated default config is deterministic.
- `make check` green.

## 13. Deferred seams (recorded, not built)

- Entry-status resolution for `add` when a type has >1 initial (session 002).
- `refs` field + resolution; recategorization (close+create+link); which terminal a recategorized task
  enters (`cancelled` vs a dedicated `superseded`) — later phases.
- `internal/core`, `TaskStore` port, ID minting — session 002.
- Resolved object graph + back-references (identities → linked, immutable in-memory index) built by
  `internal/core` for traversal (ready / advance-walk / cycles) — phase 2+ (e3_t1). Contract stays by-identity.
- Command execution (`Runner` port, `internal/adapter/exec`) — phase 3.
- External type/flow provider (external tracker) + a `mtt connect` command to wire it — later phase; the
  provider-agnostic domain (§3.11) is the seam laid now.
