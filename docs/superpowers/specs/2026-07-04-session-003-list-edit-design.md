# Session 003 — List & edit + global flags — Design Spec

Date: 2026-07-04 · Branch: `feat/s003-list-edit` · Session: [../../../sessions/003_list_and_edit.md](../../../sessions/003_list_and_edit.md)

Language note: agent-facing docs are English only (see AGENTS.md). This spec is the authoritative
statement of the session-003 design. It builds on the session-002 spec
([2026-07-04-session-002-create-and-view-design.md](2026-07-04-session-002-create-and-view-design.md))
and applies its correction that **a pure read needs no `core` usecase** (see §4).

## 1. Goal

Round out flat task CRUD before hierarchy: list tasks (with filters and a deterministic order) and edit a
task's non-flow fields. Session 003 also **owns the global-flags surface** — `--dir`/`MTT_DIR`, the
`--version` flag, and `--json` — wired as root persistent flags so later commands inherit them instead of
retrofitting each one.

```
$ mtt list [--status <s>…] [--type <t>…] [--sort created|updated]
$ mtt edit <id> [--title <text>] [--description <text>]
$ mtt [--dir <path>] [--json] … ; mtt --version
```

## 2. Scope

- **In:**
  - `pkg/mtt`: two new `TaskStore` methods — `List() ([]Task, error)` and `Update(Task) (Task, error)`.
  - `internal/core`: a pure `Select(tasks, ListFilter)` (filter **and** deterministic order) and an
    `Editor` usecase (mutation) with an injected clock.
  - `internal/adapter/yaml`: `Store.List` (enumerate `.mtt/tasks/*.yaml`) and `Store.Update` (atomic
    overwrite of an existing task), sharing a private `write` with `Create`.
  - `internal/cli`: `mtt list`, `mtt edit`; the global flags `--dir`/`MTT_DIR`, `--version`, `--json` on the
    root command; a `projectRoot(cmd)` helper (DRYs the repeated `Getwd → FindRoot`); a `taskJSON` output
    view. Refactor `add`/`show`/`types` to use `projectRoot`.
  - `testscript` e2e `list_edit.txt` (human + `--json`); unit tests; doc updates (§11).
- **Out (deferred):**
  - Hierarchy filters (`--parent`), `--kind`, `--ready` on `list` → sessions 004/005 (§13).
  - Status changes (a flow concern) → session 006; `--role`/`MTT_ROLE` → 006; `-q/--quiet`, `--no-color` →
    later.
  - **Durable, git-independent audit of edits** (a change-log or field versioning) and the **subject
    identity** (`By`) source → a dedicated later slice (§10). `history` stays transition-only (phase 3).
  - `--sort` direction (`--reverse`/asc) and `--sort id`; `--description -` (stdin) → later (§8, §9).
  - `--json` on `add`/other mutations → as those commands are next touched.

## 3. User stories

Persona: a developer or agent working inside an mtt-initialized project.

- **US-1 — List tasks in a stable order.** After adding several tasks, `mtt list` prints them one per line
  in a deterministic order (newest first), so the output is scriptable and golden/e2e-stable.
- **US-2 — Filter the list.** `mtt list --type bug` shows only bugs; `mtt list --status tbd` only `tbd`
  tasks; `mtt list --type bug --type task --status tbd` combines them (AND across dimensions, OR within one).
- **US-3 — Machine-readable output.** `mtt list --json` emits a JSON array of task objects; `mtt show e1
  --json` emits one object — so an agent drives mtt without parsing human text.
- **US-4 — Edit a task's title/description.** `mtt edit e1 --title "new"` updates the title, bumps
  `updated`, and `mtt show e1` reflects it. `mtt edit e1 --description "…"` likewise. Passing neither errors
  with guidance.
- **US-5 — Point mtt at a project explicitly.** `mtt --dir /path/to/proj list` (or `MTT_DIR=…`) operates on
  that project's `.mtt/` without `cd`-ing into it.
- **US-6 — Clear failures.** Editing a missing id, editing with no editable flag, an unknown `--type`/
  `--status` filter that matches nothing (empty result, not an error), a `--dir` without `.mtt/`, and
  running outside a project each behave predictably (§8, §9).

## 4. Layering decisions (explicit)

- **Full hexagon:** `cli → core → mtt.TaskStore ← yaml.Store`. `core` imports only `pkg/mtt`.
- **Enumeration is the adapter's job; order is `core`'s policy; filter is `core`'s pure logic.**
  - The port grows `List()` — pure enumeration. Its contract: **returns all tasks; order is unspecified.**
    (An adapter must not be forced to implement a particular sort; determinism is imposed above it.)
  - `core.Select` is a **pure function** (no store injected): it filters and imposes the deterministic
    order. This honors the 002 correction — the only logic in a `list` read is a pure filter/sort, so there
    is **no store-injected `Lister` usecase** (that would over-layer a read). The CLI composes
    `store.List()` → `core.Select(...)`. The pure function is trivially table-tested and reused by future
    `ready` (005) / `tree` (004).
- **Mutations go through `core`.** `edit` is a mutation, so it gets an `Editor` usecase (like `Adder`),
  not a direct port call.
- **`core` stays ID-agnostic and provider-agnostic.** The default order is by the **domain timestamp**
  (`Created`), never by ID structure — an external backend's IDs (`PROJ-123`, `#42`) carry no scheme `core`
  may assume. The tie-breaker treats the ID as an **opaque string** (a stable total order to break equal
  timestamps), not as `<prefix><N>` — it assigns the ID no semantics.

## 5. Contract — `TaskStore` grows (`pkg/mtt/store.go`)

```go
type TaskStore interface {
    Create(t Task) (Task, error)  // (002) mints the ID, persists, returns the stored task
    Get(id string) (Task, error)  // (002) ErrNotFound when the ID does not resolve
    List() ([]Task, error)        // (003) all tasks; ORDER UNSPECIFIED (core imposes a deterministic order)
    Update(t Task) (Task, error)  // (003) overwrite an EXISTING task by t.ID; ErrNotFound if absent
}
```

- `List` returns the pure domain tasks; the adapter maps each file's DTO → domain. An empty project yields
  an empty (non-nil) slice, no error.
- `Update` requires `t.ID` to name an existing task; it never mints and never creates (that is `Create`).
  A missing task → `ErrNotFound`. It returns the stored task (echoing what was written).

Growing the mandatory-minimum port (rather than a capability interface) is deliberate: enumerate and
update-by-id are basic CRUD every backend supports; they are not optional capabilities like history/search.

## 6. Core — filter/order + the edit usecase (`internal/core`)

### 6.1 `Select` — pure filter + deterministic order (`internal/core/list.go`)

```go
// ListFilter is the set of list predicates. Empty fields match everything.
// Within a field the values are OR-ed; across fields they are AND-ed.
type ListFilter struct {
    Statuses []string // match if the task's status is any of these
    Types    []string // match if the task's type is any of these
    Sort     SortKey  // ordering key; default SortCreated
}

type SortKey string
const (
    SortCreated SortKey = "created" // default; newest first
    SortUpdated SortKey = "updated" // most-recently-updated first
)

// Select returns the tasks matching f, in a deterministic order. It does not
// mutate its input. Order: the chosen timestamp DESC, tie-broken by ID string
// ASC (opaque, stable) so equal timestamps never reorder between runs.
func Select(tasks []mtt.Task, f ListFilter) []mtt.Task
```

- **Filtering:** a task passes when (no `Statuses` set **or** its status ∈ `Statuses`) **and** (no `Types`
  set **or** its type ∈ `Types`). Case-sensitive, exact match (names are config-defined).
- **Ordering:** primary key = `Created` (or `Updated` when `Sort==SortUpdated`), **descending** (freshest
  on top). Secondary key = `ID` ascending as an **opaque byte-string compare** (`strings.Compare`) — this
  only ever breaks ties between equal timestamps (e.g. several `add`s in the same clock-second under the
  fixed-clock e2e), keeping output deterministic without assuming any ID scheme.
- **Validation:** an unknown `Sort` value is rejected at the CLI boundary (§9), so `Select` only ever sees
  `SortCreated`/`SortUpdated`; it defaults an empty `Sort` to `SortCreated`.
- `Select` works on a copy (`append([]mtt.Task(nil), tasks...)` then `sort.SliceStable`) — never reorders
  the caller's slice.

### 6.2 `Editor` — the edit usecase (`internal/core/edit.go`)

```go
type Editor struct {
    store mtt.TaskStore
    now   func() time.Time
}
func NewEditor(store mtt.TaskStore, now func() time.Time) *Editor

// EditParams are the requested edits. A nil pointer means "leave unchanged";
// a non-nil pointer (including to "") means "set to this value".
type EditParams struct {
    Title       *string
    Description *string
}
func (e *Editor) Edit(id string, p EditParams) (mtt.Task, error)
```

Policy:

1. **Something to edit:** if both `Title` and `Description` are nil → error
   *"nothing to edit: provide --title and/or --description"*.
2. **Load:** `t, err := store.Get(id)`; propagate `ErrNotFound`.
3. **Apply** only the provided fields: if `p.Title != nil` set `t.Title = *p.Title`; likewise Description.
4. **Preserve the content invariant** (from 002): after applying, if `t.Title == "" && t.Description == ""`
   → error *"a task needs a title or a description"* (an edit may not empty both).
5. **Stamp:** `t.Updated = e.now().UTC().Truncate(time.Second)`; `Created` is untouched.
6. `return store.Update(t)`.

`Editor` does **not** touch `id`/`type`/`status`/`parent`: the params expose no such fields. This is not
merely "a flag we didn't add" — those are **out of `edit`'s remit by design**: `status` moves through flow
enforcement (`status`/`advance`, 006); a task's `type` is **immutable** (recategorize = close + create +
link, per DESIGN); and **re-parenting is a distinct operation** (`mtt reparent`/`move`, later) about the
task's place in the hierarchy, not a field edit — it stays entirely outside `edit`.

Injecting `now` keeps the `updated` bump deterministic under test (mirrors `Adder`).

## 7. YAML adapter — `List` / `Update` (`internal/adapter/yaml/task.go`)

```go
func (s *Store) List() ([]mtt.Task, error)
func (s *Store) Update(t mtt.Task) (mtt.Task, error)
```

- **`List`** reads `.mtt/tasks/`, selects `*.yaml` entries, unmarshals each `ymlTask`, maps `toDomain`, and
  returns the slice. A missing `tasks/` dir → empty slice, no error (a freshly-init'd project). A malformed
  file → a wrapped parse error naming the file. Order is whatever `os.ReadDir` yields (lexical) — **not**
  contractually meaningful; `core.Select` imposes the real order.
- **`Update`** resolves the path `.mtt/tasks/<id>.yaml`; if it does not exist → `mtt.ErrNotFound` (never
  creates). Otherwise it serializes `fromDomainTask(t)` and `atomicWrite`s (temp + rename) — same
  determinism as `Create`.
- **DRY:** extract the shared marshal-and-write tail of `Create` into a private
  `func (s *Store) write(t mtt.Task) (mtt.Task, error)`; `Create` = `mint` → set ID → `write`; `Update` =
  existence check → `write`. Serialization stays in exactly one place.

The adapter still needs no config for `List`/`Update` (unlike `Create`, which needs the type→prefix map to
mint). `Get`/`List`/`Update` remain config-independent reads/writes by ID.

## 8. CLI — commands (`internal/cli`)

### 8.1 `mtt list [--status …] [--type …] [--sort …]`

`NoArgs`. `projectRoot(cmd)` (§8.4) → `store := yaml.NewTaskStore(root)` → `tasks, err := store.List()` →
`core.Select(tasks, filter)` → render. `--status`/`--type` are `StringArray` (repeatable, OR within).
`--sort` is a string validated to `created`/`updated` (default `created`); an unknown value → a usage error
*"invalid --sort %q: want created|updated"*. Filtering an empty project or a no-match filter prints nothing
(exit 0) — an empty result is not an error.

Human output — one line per task, `<id>  <type>  [<status>]  <title>` (title omitted when empty):

```
t1  task  [tbd]  fix login
e2  epic  [tbd]  build billing
e1  epic  [tbd]  build auth
```

(Illustrative only — the exact sequence is `Created` desc with an opaque-ID tie-break.) `--json` → a JSON
**array** of task objects (§8.3); an empty result → `[]`.

**Order determinism is a unit concern, not an e2e one.** The e2e drives the real binary with wall-clock
`time.Now` (002 deferred a fixed-clock e2e hook), so whether two `add`s land in the same clock-second — and
thus whether newest-first or the ID tie-break decides their relative position — is not controlled. The
**deterministic order is proven by `core.Select` unit tests under a fixed clock** (§12); the e2e asserts
each expected row is **present** (anchored to its own content) and that filtered-out rows are **absent**,
never a specific inter-row sequence.

### 8.2 `mtt edit <id> [--title …] [--description …]`

`ExactArgs(1)`. Build `EditParams` from `cmd.Flags().Changed("title")`/`Changed("description")` so an
explicitly-passed empty string is distinguished from an unset flag. `projectRoot` → `store` →
`editor := core.NewEditor(store, time.Now)` → `editor.Edit(id, params)`. `ErrNotFound` → *"task %q not
found"*. On success print `updated <id>` (human) or the task object (`--json`).

### 8.3 `--json` output view (`internal/cli/json.go`)

JSON is a presentation concern → a CLI-layer view with json tags (the domain stays pure, so it carries no
json tags — mirror the yaml DTO rationale):

```go
type taskJSON struct {
    ID          string   `json:"id"`
    Type        string   `json:"type"`
    Title       string   `json:"title,omitempty"`
    Status      string   `json:"status"`
    Parent      string   `json:"parent,omitempty"`
    Created     string   `json:"created"`  // RFC3339 UTC
    Updated     string   `json:"updated"`
    Description string   `json:"description,omitempty"`
}
func toTaskJSON(t mtt.Task) taskJSON
```

`writeJSON(w, v)` marshals with two-space indent and a trailing newline (stable, diff-friendly). `list` uses
`[]taskJSON` (built from the selected slice; a nil result marshals as `[]`, not `null`); `show` uses one
`taskJSON`. Reserved collections (`tags`/`depends_on`/`refs`/`comments`/`history`) are **omitted** from the
003 JSON view — they are unpopulated until later phases; adding them later is additive.

### 8.4 Global flags (root persistent) + `projectRoot` helper

On `NewRootCmd`:

- `root.PersistentFlags().String("dir", "", "project root containing .mtt/ (overrides discovery; env MTT_DIR)")`
- `root.PersistentFlags().Bool("json", false, "emit machine-readable JSON")`
- `root.Version = version` — cobra then provides `--version` automatically. `mtt version` (the subcommand)
  stays.

```go
// projectRoot resolves the project root for a command: --dir if set, else
// $MTT_DIR, else the nearest ancestor of the cwd containing .mtt/ (FindRoot).
// With an explicit --dir/MTT_DIR the directory must itself contain .mtt/ (no
// upward walk); otherwise a clear error.
func projectRoot(cmd *cobra.Command) (string, error)
```

- Resolution: `dir, _ := cmd.Flags().GetString("dir")`; if empty, `dir = os.Getenv("MTT_DIR")`. If `dir !=
  ""` → require `<dir>/.mtt` to be a directory (else *"no .mtt/ in %q"*) and return `dir`. Else
  `os.Getwd()` → `yaml.FindRoot(cwd)` (unchanged walk-up behavior).
- **DRY refactor:** `add`, `show`, `types` replace their `os.Getwd() → yaml.FindRoot(cwd)` blocks with
  `projectRoot(cmd)`; `list`/`edit` use it from the start. `init` reads the same `--dir`/`MTT_DIR` for its
  **base** directory (where it creates `.mtt/`), defaulting to the cwd — it does **not** call `FindRoot`
  (it creates rather than discovers), and the project name still defaults to `filepath.Base(base)`.
- Reading `--json`: a small `jsonFlag(cmd) bool` helper (`cmd.Flags().GetBool("json")`) used by `list`/`show`.

## 9. Error handling & exit codes

- Reuse the 002 approach: commands return errors via `RunE`; `Execute` prints `error: <msg>` to stderr and
  returns non-zero (generic exit 1). The richer exit-code taxonomy from CLI_REFERENCE (2 usage / 4 not-found
  / …) is **not** implemented in 003 — it lands with flow (006+); 003 keeps the single generic failure code,
  consistent with 001/002. (Recorded as a known follow-up so the reference and behavior stay honestly in
  sync.)
- Specific messages: unknown `--sort` value; `edit` with no editable flag; `edit`/`show` of a missing id;
  `--dir` without `.mtt/`; running outside a project (existing `FindRoot` message).

## 10. Deferred seams (recorded, not built)

- **Durable, git-independent audit of edits.** `edit` records only the `Updated` bump in 003; there is **no**
  in-file edit log. Rationale: (a) `HistoryEntry` is transition-shaped (`From/To/Checks`) — an edit has no
  transition, so forcing it in pollutes the model that phase-3 flow will populate; (b) git is **not** a valid
  justification for dropping the record — mtt must not require git — so an eventual audit must be self-
  contained in the store. This is a **real future slice** with its own spec, choosing between a **lightweight
  change-log** (`{at, fields:[…]}` — what/when, no old values, git-independent but not reconstructable) and
  **field versioning** (old→new per edit — reconstructable, ≈ note versioning of phase 5). Either is **purely
  additive** later (a new `omitempty` field; existing files unaffected) — deferring creates no lock-in.
- **Subject identity (`By`) source.** Recording *who* edited/transitioned needs the **subject**, which is
  distinct from `--role` (the role — implementer/reviewer). `HistoryEntry` already reserves both `By` and
  `Role`. The **source** of `By` (likely the gitignored `.mtt/config.local.yaml` — e.g. a `user:`/`author:`
  key — perhaps with an env or `git config user.*` fallback) is decided in that same audit slice, alongside
  the roles work (006). Not resolved in 003.
- **`list` breadth:** `--parent` (004), `--kind`, `--ready` (005), and `mtt ready` reuse `ListFilter`/
  `Select` — the filter type is designed to grow.
- **`--sort` breadth:** direction (`--reverse`/asc) and an adapter-defined `--sort id` (natural per-prefix,
  meaningful only for the YAML adapter) — later; 003 ships `created`/`updated`, both descending.

## 11. Doc reconciliation (implementation task 0 / final)

- **CLI_REFERENCE.md + CLI_REFERENCE.ru.md:** mark `mtt list`, `mtt edit`, and the global flags
  `--dir`/`--json`/`--version` as implemented (phase-1 slice); note the `--sort` flag; keep `--status`/
  `--type` (implemented) vs `--kind`/`--parent`/`--ready` (later) honest; keep the exit-code taxonomy as
  "proposed" (not yet enforced — §9).
- **DESIGN.md + DESIGN.ru.md:** in the relevant sections, record that `list` order defaults to `Created`
  desc (provider-agnostic; not ID-based) with an opaque-ID tie-break; that `edit` touches only
  title/description and re-parenting is a separate operation; and add the **edit-audit + subject-identity**
  item to the "Later (backlog)" list.
- **TASKS.md:** tick the global-flags cross-cutting note for `--dir`/`--version`/`--json` landing in 003;
  add the edit-audit/subject-identity backlog item under "Later".
- **CLAUDE.md files:** `pkg/mtt` (port grows `List`/`Update`), `internal/core` (`Select` pure read +
  `Editor` mutation), `internal/adapter/yaml` (`List`/`Update` + shared `write`), `internal/cli`
  (`list`/`edit`, global flags, `projectRoot`, `taskJSON`).
- **sessions/003_list_and_edit.md:** fill "Done"; update **NEXT_SESSION.md** for session 004.

## 12. Testing strategy (test-first)

- **`internal/core` (`Select`)** — table-driven, pure: no filter (all, ordered); `--status` OR; `--type` OR;
  status AND type; no-match → empty; order `Created` desc; **tie-break** (equal `Created`, differing IDs →
  stable ID-string order); `SortUpdated` order; input slice not mutated (assert original order preserved).
- **`internal/core` (`Editor`)** — fake `TaskStore` (records `Update`, seeded `Get`) + fixed clock: edit
  title only / description only / both (other fields untouched, `Updated` bumped, `Created` intact); nil+nil
  → "nothing to edit"; emptying both → invariant error; `Get` `ErrNotFound` propagates; `Update` receives
  the merged task.
- **`internal/adapter/yaml`** — `List`: empty project → empty slice; after N `Create`s → N tasks with
  round-tripped fields; malformed file → wrapped error. `Update`: create → update title → `Get` reflects it
  and the file is rewritten atomically; `Update` of a missing id → `ErrNotFound`; `Create` then `Update`
  share serialization (no drift). Assert `Store` still satisfies `mtt.TaskStore` (compile-time
  `var _ mtt.TaskStore = (*Store)(nil)`).
- **`internal/cli`** — `testscript` `list_edit.txt` (**presence/absence asserts, never inter-row order** —
  see §8.1): `init` → add a few tasks (epics + `--no-parent` tasks) → `list` shows each row (anchored
  per-line like `stdout 'e1  epic  \[tbd\]'`) → `list --type epic` shows the epics and **omits** the tasks
  (`! stdout 't1 '`) → `list --status tbd` narrows likewise → `list --sort updated` is **accepted** (exit 0,
  rows present) → `edit e1 --title "renamed"` then `show e1` reflects it (`stdout 'renamed'`) → `list --json`
  / `show e1 --json` emit valid JSON (anchored on `"id": "e1"`) → `edit e1` (no flag) errors → `edit missing
  …` errors → `--dir <path> list` works from a sibling dir with no `.mtt/` ancestor → `MTT_DIR` env
  equivalently. No network; temp dirs; timestamps and inter-row order not asserted in e2e.

## 13. Acceptance (must pass)

- `mtt init` → add several tasks → `mtt list` prints them in a stable, newest-first order; `mtt list --type
  <t>` and `--status <s>` filter (AND across, OR within); `mtt list --json` is a valid JSON array.
- `mtt edit <id> --title "…"` bumps `updated` and `mtt show <id>` reflects the new title; `mtt edit <id>`
  with no editable flag errors with guidance.
- `mtt --dir <path> list` (and `MTT_DIR`) operate on that project; `mtt --version` prints the version.
- `testscript list_edit.txt` passes; `make check` green.

## 14. File plan

**Create:** `internal/core/list.go`, `internal/core/edit.go` (+ tests); `internal/cli/list.go`,
`internal/cli/edit.go`, `internal/cli/json.go` (+ tests); `internal/cli/testdata/scripts/list_edit.txt`.

**Modify:** `pkg/mtt/store.go` (port +`List`/`Update`); `internal/adapter/yaml/task.go` (`List`/`Update` +
shared `write`) (+ tests); `internal/cli/root.go` (persistent flags, `Version`, register `list`/`edit`);
`internal/cli/add.go`, `show.go`, `types.go` (use `projectRoot`; `show` honors `--json`); `internal/cli/
init.go` (base dir from `--dir`/`MTT_DIR`); the CLAUDE.md files; project docs (§11).
