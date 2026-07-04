# Session 002 — Create & view a task — Design Spec

Date: 2026-07-04 · Branch: `feat/s002-create-view` · Session: [../../../sessions/002_create_and_view.md](../../../sessions/002_create_and_view.md)

Language note: agent-facing docs are English only (see AGENTS.md). This spec is the authoritative
statement of the session-002 design. It builds on the session-001 spec
([2026-07-03-session-001-init-and-types-design.md](2026-07-03-session-001-init-and-types-design.md)) and
records **two model/architecture corrections** decided in the 002 brainstorm that supersede wording in
DESIGN.md/AGENTS.md (see §10 for the exact doc reconciliation):

1. **Flat, per-prefix task IDs** replace the hierarchical `e1_t3_s2` scheme (identity is decoupled from
   position, so re-parenting keeps IDs stable).
2. **`--no-parent`** — a conscious escape hatch that lets a parent-requiring type be created top-level; and
   **`Status.Default`** — the marker that resolves the entry status when a flow has more than one `initial`.

## 1. Goal

Deliver the second vertical slice: create a task and view it. This is where the full hexagon appears —
`cli → core → port ← adapter` — with the `Task` model, the `TaskStore` port, the `internal/core` add
usecase, and the YAML adapter minting IDs and persisting one file per task.

```
$ mtt add [title] [--type <name>] [--no-parent] [--description <text>]
$ mtt show <id>
```

## 2. Scope

- **In:**
  - `pkg/mtt`: the `Task` model + reserved value objects (`Ref`/`RefKind`, `Comment`, `HistoryEntry`/`Check`),
    the `TaskStore` port + `ErrNotFound`, the `Status.Default` field, and pure helpers
    (`Type.IsRoot`, `Type.InitialStatus`, `Config.TypeByName`, `RefKind.Valid`) + two new `Config.Validate`
    invariants.
  - `internal/core` (new package): the `add` usecase (`Adder`) with an injected clock; imports only `pkg/mtt`.
  - `internal/adapter/yaml`: a `TaskStore` implementation — **flat per-prefix ID minting**, deterministic
    serialization (DTO + `omitempty`), atomic write, `Get`, `.mtt/tasks/<id>.yaml` layout.
  - `internal/cli`: `mtt add`, `mtt show` (composition root wiring).
  - `testscript` e2e `add_show.txt`; golden test for a serialized task; doc updates (§10).
- **Out (deferred):**
  - `--parent` and real hierarchy → session 004 (child ID minting, the `show` "you are here" lineage line).
  - `list` / filters, `edit` → session 003.
  - Populating `depends_on` / `refs` / `comments` / `history` / `tags`; flow transitions & command gates;
    recategorization; re-parenting; boards/views; any non-YAML adapter — later phases.

## 3. User stories

Persona: a developer or agent working inside an mtt-initialized project.

- **US-1 — Create a top-level task of an explicit type.** `mtt add --type epic "Build auth"` persists a task
  and prints its minted root ID (`e1`); a second → `e2`. (Root type; no parent flag needed.)
- **US-2 — Create with the default type, safely.** `mtt add "x"` uses the configured default type. If that
  type's flow requires a parent (the default template's `task` does), `add` **refuses with guidance**
  ("use `--parent` [session 004] or `--no-parent`") — never a silently-invalid task.
- **US-3 — Deliberately create a parentless task (conscious exception).** `mtt add --no-parent "fix login"`
  mints a root ID for the default type (`t1`), parent empty. Explicit opt-in.
- **US-4 — View a task.** `mtt show t1` shows id, type, title (when present), status, created/updated,
  description.
- **US-5 — Clear failures.** Unknown `--type`, an add with neither title nor `--description`, a `show` of a
  missing ID, and running either command outside a project each fail with a clear message.

## 4. Domain model — `pkg/mtt` additions (pure; no serialization concerns)

The domain types carry **no yaml/json tags and no `prefix`** (adapter owns those). Field order is chosen for
readable diffs; the on-disk layout is the adapter's concern (§7).

```go
// Task is a single unit of work. Field order == on-disk order (deterministic diff).
type Task struct {
    ID          string    // MANDATORY once stored; minted by the adapter (empty in a logical task)
    Type        string    // MANDATORY; a Type.Name from config (a task's type is immutable)
    Title       string    // optional if Description is set (at least one is required — enforced in core)
    Status      string    // MANDATORY; a status name of the type's flow
    Parent      string    // optional; parent task ID (always empty in session 002)
    Tags        []string  // optional; RESERVED (not populated in 002)
    DependsOn   []string  // optional; RESERVED
    Refs        []Ref     // optional; RESERVED
    Created     time.Time // MANDATORY
    Updated     time.Time // MANDATORY
    Description string    // optional if Title is set
    Comments    []Comment // optional; RESERVED
    History     []HistoryEntry // optional; RESERVED (append-only audit; written from phase 3)
}

// RefKind — closed vocabulary (a value object), like StatusKind.
type RefKind string
const (
    RefNote    RefKind = "note"
    RefTask    RefKind = "task"
    RefComment RefKind = "comment"
    RefURL     RefKind = "url"
)
func (k RefKind) Valid() bool

// Ref — a structured, verifiable reference (informational, != depends_on).
type Ref struct { Kind RefKind; ID string; Label string }

// Comment — a tree node via nested Replies; ID is sequential within the task.
type Comment struct {
    ID      int
    Author  string
    Created time.Time
    Body    string
    Refs    []Ref
    Replies []Comment
}

// HistoryEntry — one append-only transition record. Role is the non-deferrable roles seam.
type HistoryEntry struct {
    At     time.Time
    By     string
    Role   string
    From   string
    To     string
    Checks []Check
}
type Check struct { Cmd string; Exit int }
```

**Why define the reserved value objects now:** the project models the domain explicitly (DDD) and wants a
stable serialization boundary from the start (deterministic diffs are a headline value). The fields are
`omitempty` in the adapter DTO, so a task that only exercises 002's flow writes none of them, and adding
fields to `Comment`/`HistoryEntry` in their own sessions stays backward-compatible with existing files.
Exported types/fields don't trip the `unused` linter.

**`Status.Default` (new seam):**

```go
type Status struct {
    Name        string
    Kind        StatusKind
    Description string
    Default     bool // optional; marks THE entry status when a flow has >1 initial (mirrors Type.Default)
}
```

The YAML `ymlStatus` DTO gains a matching `default` field (`yaml:"default,omitempty"`) and `toDomain` maps
it. The embedded templates keep a single `initial` per flow and do not set `default`, so neither the rendered
config goldens nor `Load` behavior changes for existing templates.

**New pure helpers:**

```go
func (t Type) IsRoot() bool                       // len(Parents) == 0
func (t Type) InitialStatus() (Status, bool)      // the initial marked Default, else the first initial in
                                                  // config order; false if the flow has no initial
func (c Config) TypeByName(name string) (Type, bool)
```

`InitialStatus` mirrors `DefaultType`'s "marked, else first" tolerance. `IsRoot`/`InitialStatus`/`TypeByName`
are primitives; the *policy* that composes them (default-type resolution, placement rule) lives in `core` (§6),
so the 002-specific rules can evolve without touching the contract.

## 5. Config invariants — `Config.Validate()` additions

Two new **domain**, name-agnostic invariants, checked per flow (added to the existing `validateFlow`):

- **At most one `Default` status per flow.** (More than one entry marker is ambiguous.)
- **A `Default` status must be `kind: initial`.** (The entry state is where a task starts; by topology that is
  an `initial` status.)

`Validate` continues to return a joined error listing every violation. Not added now (deferred seam): a YAML
provider-level "exactly one `Default` when a flow has >1 initial" strictness — no template exercises multiple
initials yet, and `InitialStatus`'s "first initial" fallback is safe until one does.

## 6. Contract — the `TaskStore` port

```go
// TaskStore is the mandatory-minimum driven port for tasks: create (adapter mints the ID) and get by ID.
type TaskStore interface {
    Create(t Task) (Task, error) // t has an empty ID; the adapter mints it, persists, and returns the stored task
    Get(id string) (Task, error) // returns ErrNotFound when no task has that ID
}

// ErrNotFound is returned by TaskStore.Get when the ID does not resolve.
var ErrNotFound = errors.New("mtt: task not found")
```

The port is pure: `Create` takes/returns the domain `Task`; ID minting (prefix/scan/`O_EXCL`) is entirely the
adapter's concern and never leaks through the port. `core` depends on this interface, never on `adapter/*`.

## 7. YAML adapter — `internal/adapter/yaml` (tasks)

New file(s) implement the port; the config layer from session 001 is unchanged.

```go
type Store struct { root string } // implements mtt.TaskStore
func NewTaskStore(root string) *Store
func (s *Store) Create(t mtt.Task) (mtt.Task, error)
func (s *Store) Get(id string) (mtt.Task, error)
```

- **Self-contained:** `Create` lazily calls the existing `Load(root)` to obtain the `type → prefix` map. The
  store owns its config I/O, so no prefix map is threaded through the constructor and `Get`/`show` stay
  independent of config. (Alternative considered — pass `prefixes` into the constructor — rejected for the
  nil-map smell on the read path.)

- **Flat, per-prefix ID minting (the corrected scheme):** for type `T` with prefix `p`, scan `.mtt/tasks/`
  for files matching `^p\d+\.yaml$`, take `N = max + 1` (starting at 1), and **reserve** `<p><N>.yaml` with
  `O_EXCL`; on `EEXIST` bump `N` and retry. IDs are **flat** — no parent chain — so:
  - a task's ID never encodes its position (`t17` regardless of which epic it sits under);
  - **re-parenting** (later) is a `Parent`-field change with a stable ID and no file rename;
  - the scan/regex is uniform for root and (future) child tasks — no special-casing in session 004.
  In session 002 `Parent` is always empty; the minting path is exactly the flat per-prefix path.

- **Serialization:** a `ymlTask` DTO carries the yaml tags and `omitempty` on every optional/reserved field
  (`parent`, `tags`, `depends_on`, `refs`, `description`, `comments`, `history`) — a fresh task writes only
  `id`, `type`, `title`, `status`, `created`, `updated`. Timestamps serialize as **RFC3339 UTC strings at
  second precision** (`2026-07-04T09:20:00Z`); the domain keeps `time.Time`, the adapter maps to/from the
  wire string. Field order in the DTO matches the `Task` field order (deterministic diff).

- **Write** is atomic (reuse `atomicWrite`: temp + rename); `MkdirAll .mtt/tasks/` on first create. `Get`
  reads `.mtt/tasks/<id>.yaml`, unmarshals the DTO, maps to the domain `Task`; a missing file → `mtt.ErrNotFound`.

## 8. Core — `internal/core` (the add usecase)

```go
type Adder struct {
    store mtt.TaskStore
    cfg   mtt.Config
    now   func() time.Time
}
func NewAdder(store mtt.TaskStore, cfg mtt.Config, now func() time.Time) *Adder

type AddParams struct {
    Title       string
    TypeName    string // "" = default type
    NoParent    bool
    Description string
}
func (a *Adder) Add(p AddParams) (mtt.Task, error)
```

Policy (the resolved open questions live here):

1. **Content required:** `Title != "" || Description != ""`, else an error ("provide a title or a description").
2. **Resolve type:** `TypeName` if given — `cfg.TypeByName`, unknown → error; else `cfg.DefaultType()`.
3. **Placement rule:** if the type is **not** root (`len(Parents) > 0`) **and** not `NoParent` → error:
   *"type %q requires a parent; use --parent (session 004) or --no-parent to create it top-level."* For a root
   type, `NoParent` is a harmless no-op. (`--parent` itself is deferred to 004.)
4. **Entry status:** `Type.InitialStatus()` (error if the flow somehow has no initial — should be unreachable
   after `Validate`).
5. **Timestamps:** `now := a.now().UTC().Truncate(time.Second)`; build the logical `Task{Type, Title, Status,
   Description, Created: now, Updated: now}` with empty `ID`/`Parent`.
6. `return a.store.Create(task)`.

`core` imports only `pkg/mtt`. The injected `now` makes timestamps deterministic in tests. New
`internal/core/CLAUDE.md` documents the package (usecases; imports only the port + domain; owns add policy).

## 9. CLI — `internal/cli` (thin; composition root)

- **`mtt add [title] [--type <name>] [--no-parent] [--description <text>]`** — `MaximumNArgs(1)` (title
  optional). `os.Getwd` → `yaml.FindRoot` → `yaml.Load` (cfg, prefixes) → `cfg.Validate()` →
  `store := yaml.NewTaskStore(root)` → `adder := core.NewAdder(store, cfg, time.Now)` →
  `task, err := adder.Add(...)` → print `created <id>` to `cmd.OutOrStdout()`.
- **`mtt show <id>`** — `ExactArgs(1)`. `FindRoot` → `store := yaml.NewTaskStore(root)` → `store.Get(id)` →
  format a block to stdout; `errors.Is(err, mtt.ErrNotFound)` → a clear "task <id> not found" error.

`show` output block (exact strings pinned by the golden/testscript, tunable during implementation):

```
t1  task  [tbd]
  title:       fix login
  created:     2026-07-04T09:20:00Z
  updated:     2026-07-04T09:20:00Z

  <description, if any>
```

The hierarchy/lineage line ("epic 1 → task 2 (you are here) → N subtasks") is **deferred to session 004**
(it needs the in-memory index/traversal introduced in phase 2); in 002 no task has a parent, so `show` omits
any parent line. When `Title` is empty, `show` leads with the description (the title line is omitted).

## 10. Doc reconciliation (implementation task 0)

Behavior/model changed, so update the source-of-truth docs alongside the code:

- **DESIGN.md + DESIGN.ru.md:**
  - §"Types and hierarchy (domain) vs ID/slug (adapter)" and the Task-model example: replace the hierarchical
    `e1_t3_s2` scheme with **flat per-prefix IDs**; state that identity is decoupled from position and that
    re-parenting keeps IDs stable; keep the sequential-ID collision caveat (now per-prefix across branches).
  - §"Decisions"/`add` behavior: record **`--no-parent`** (conscious top-level exception for a
    parent-requiring type) and **`Status.Default`** (entry-status marker when a flow has >1 initial;
    resolution = marked default initial, else first initial).
  - Add to the backlog / "Later": **re-parenting** (`mtt reparent`/`move`; enabled by flat IDs),
    **tags** (a cross-cutting `[]string` label; lands with `list`/filters or the backlog), and
    **boards/views** (a query/view over tags/status/type; relates to `list` / `mtt-ui`).
- **AGENTS.md** §"Storage invariants": update the "IDs are stable (`e1_t3_s2`)" example to the flat scheme
  (stability now genuinely holds under re-parenting).
- **TASKS.md:** reflect flat IDs in e2 wording; note the new backlog items (reparent/tags/boards) under "Later".
- **CLI_REFERENCE.md + CLI_REFERENCE.ru.md:** document `mtt add` (with `--type`/`--no-parent`/`--description`)
  and `mtt show`.
- **CLAUDE.md files:** new `internal/core/CLAUDE.md`; update `pkg/mtt/CLAUDE.md` (Task model + `TaskStore`
  port + `Status.Default`), `internal/adapter/yaml/CLAUDE.md` (task store + flat ID minting + `.mtt/tasks/`),
  `internal/cli/CLAUDE.md` (`add`/`show`).
- **sessions/002_create_and_view.md:** fill "Done" at the end; update NEXT_SESSION.md for session 003.

## 11. Layering decisions (explicit)

- **Full hexagon:** `cli → core → mtt.TaskStore ← yaml.Store`. `core` imports only `pkg/mtt`; the CLI
  (composition root) wires the concrete adapter into the usecase.
- **Domain ≠ serialization (DDD):** `pkg/mtt.Task` is pure (`time.Time`, no tags); the adapter's `ymlTask`
  DTO owns tags, `omitempty`, and the RFC3339 wire format.
- **Identity is the adapter's, decoupled from position:** flat per-prefix IDs; hierarchy lives in the `Parent`
  field (domain data) and is computed for display, never encoded in the ID.
- **Policy in core, primitives in the contract:** `IsRoot`/`InitialStatus`/`TypeByName` are pure helpers; the
  add policy (default resolution, `--no-parent` placement, entry-status choice, timestamps) lives in `core`.
- **Injected clock:** `core` takes `now func() time.Time` for deterministic timestamps under test.

## 12. Testing strategy (test-first)

- **`pkg/mtt`** — table-driven: `Type.IsRoot`; `Type.InitialStatus` (marked-default initial / first-initial
  fallback / no-initial); `Config.TypeByName` (found / not found); `RefKind.Valid`; new `Validate` red cases
  (two `Default` statuses in a flow; `Default` on a non-`initial` status) + green (one default initial; no
  default).
- **`internal/core`** — table-driven with a **fake `TaskStore`** (records the created task, returns it with a
  stub ID) and a **fixed clock**: default vs explicit type; unknown type → error; non-root type without
  `--no-parent` → error; non-root type **with** `--no-parent` → ok (parent empty); root type → ok;
  neither title nor description → error; entry-status assignment (default-marked and first-initial);
  `Created == Updated == clock`; the logical task passed to `Create` (type/title/status/description, empty
  parent); the minted ID from the store propagated to the caller.
- **`internal/adapter/yaml`** — `Create` mints `e1` then `e2` (same prefix), independent per prefix (a `task`
  → `t1`), `O_EXCL` collision (pre-create `e1.yaml` → next is `e2`); file lands at `.mtt/tasks/<id>.yaml`;
  **golden** for a fixed-timestamp task (deterministic serialization; `-update` to regenerate); a full-task
  round-trip populating `parent`/`tags`/`depends_on`/`refs`/`comments`/`history` (marshal → unmarshal → equal)
  to lock the reserved serialization; `Get` round-trip (create → get equal); unknown ID → `mtt.ErrNotFound`;
  a malformed file → a wrapped parse error.
- **`internal/cli`** — `testscript` `add_show.txt`: `init` → `add --type epic "fix login"` (stdout has `e1`) →
  a second epic (`e2`) → `add --no-parent "orphan"` (default `task` → `t1`) → `show e1` shows title/type/status
  → bare `add "x"` (default `task`, no `--no-parent`) errors with guidance → `add --type nope "y"` errors →
  `show missing` errors → `add`/`show` outside a project say "not initialized". Timestamps are not asserted in
  e2e (the injected clock is a unit/golden concern). No network; temp dirs.

## 13. Acceptance (must pass)

- `mtt init` → `mtt add --type epic "fix login"` prints `e1` → `mtt show e1` shows its title, type, and status
  (the type's initial status). A bare `mtt add "x"` with the default template errors with guidance;
  `mtt add --no-parent "x"` creates `t1`.
- `testscript add_show.txt` passes.
- The golden test for a serialized task file is deterministic.
- `make check` green.

## 14. Deferred seams (recorded, not built)

- `--parent` + child ID minting (flat: still `<prefix><N>`, with `Parent` set) and the `show` "you are here"
  lineage line — session 004.
- Re-parenting (`mtt reparent`/`move`) — enabled by flat IDs; later.
- `tags` population + filtering; boards/views — with `list`/filters (003) or later.
- `depends_on` / `refs` / `comments` / `history` population; flow transitions & command gates (`Runner`) —
  phases 2–3.
- YAML provider-level "exactly one `Default` when a flow has >1 initial" strictness — when a multi-initial
  template first ships.
- A fixed-clock env hook for e2e (to assert timestamps deterministically) — only if a future session needs it.
