# NEXT_SESSION — primer

A living handoff doc. Update it at the end of each session (what's done / what's next).

## Where we are

- **Phase 0 (scaffold) + session 001 (init & inspect) + session 002 (create & view) are DONE and merged to
  `main`.** Session 001 shipped `mtt init [--template default|coding] [--force] [--name]` and
  `mtt types [<type>]`. Session 002 shipped `mtt add [title] [--type] [--no-parent] [--description]` and
  `mtt show <id>`; `make check` green. CI runs on Node 24 action majors.
- **In place now:** the pure `pkg/mtt` contract (`Config/Type/Flow/Status/Transition`, the `StatusKind`
  value object, structural `Config.Validate()`, `DefaultType`/`ChildrenIn`), the YAML adapter's **config
  layer** (`FindRoot`, embedded `default`/`coding` templates, atomic `Init`, `Load` + `config.local.yaml`
  overlay, DTO↔domain mapping, prefix/one-default checks), **and, from 002:** the `Task` model + `TaskStore`
  port (`pkg/mtt`), `internal/core`'s `add` usecase (default/explicit type, `--no-parent`, entry-status
  resolution, injected clock), and the YAML adapter's **flat per-prefix ID minting** (`O_EXCL`) +
  deterministic serialization. **Not yet:** `list`/`edit`, hierarchy (`--parent`/`tree`), dependencies, flow
  enforcement, comments.
- Work is organized in **compact sessions** (see [sessions/README.md](sessions/README.md)); next up is
  **session 003** (list & edit tasks).
- Repo: <https://github.com/pashukhin/mtt> (public). Branch per session → PR → CI green → merge into `main`.
- Stack: Go 1.23, cobra, `gopkg.in/yaml.v3`, `go-internal/testscript` (e2e); storage — YAML file-per-task.

## The session starts with planning (mandatory)

Before any code — a planning phase (use the superpowers skills: brainstorming/planning). The plan must
account for the key invariants from DESIGN.md:

- **Types and hierarchy are domain (from config); ID/slug is the adapter's job.** The code has NO literals
  for type names or ID structure. Hierarchy comes from a type's `parents` field, stored (not encoded in the
  ID). The ID is minted by `TaskStore` and is **flat, per-prefix** (YAML: `<prefix><N>`, e.g. `e1`/`t17`/`s3`;
  `prefix` is a YAML-adapter field) — decoupled from position, so re-parenting never re-mints it.
- **Invariants (validated on config load, structural & name-agnostic):** `kind` (initial/active/terminal) is
  defined by flow **topology** and validated; **≥1 of each kind per flow** (minimal `initial → active →
  terminal`; a 2-status flow is invalid); **multiple initials allowed**; status identity is per-flow
  `(type, name)` with **no cross-flow transitions**; the default type is marked `default: true` (**no literal
  `task`**); ready/list work by category, never by a literal name.
- **Domain is pure & provider-agnostic (DDD):** `pkg/mtt` carries no serialization tags and no `prefix`
  (adapters map via DTOs); `kind` is a value object (`StatusKind`); references are by identity, back-refs are
  computed; the domain needs only a **mandatory minimum**, so types/flow can later come from an external
  provider (`mtt connect`). A task's type is **immutable** (recategorize = close + create + link via `refs`).
- **Capabilities:** features (history, dependencies, comment tree, search, **KB**) are optional per adapter
  (`Capabilities()` / `ErrUnsupported`); YAML is the reference (does everything), `core` writes to the
  minimum and "lights up" what's available. A task carries append-only `history` (always, in YAML).
- **References:** tasks/comments carry structured verifiable `refs` (`note`/`task`/`comment`/`url`) —
  informational, **≠ `depends_on`**. Verification is capability-aware (note only with a KB). Without a KB,
  knowledge lives in tasks/comments and the links between them.
- **Roles — a seam (not built now):** the semantics of `start`/`done` depend on the role (reviewer vs
  implementer). For now we only reserve `role` in `history` (non-deferrable) and `--role`/`MTT_ROLE`;
  routing and a config `roles` section come later. Roles are semantic routing, not RBAC; role names come from config.
- **Killer feature — executable transitions:** a transition carries `description` + `commands` (all → 0,
  else the transition is blocked). Execution is behind the `Runner` port (`core` defines it,
  `internal/adapter/exec` implements it, tests use a fake). `start`/`done` are the meta-command
  `advance --to` (walk to a target; modes `--stop`(default)/`--atomic`/`--force`; no config DSL). Phase 3.
- **`mtt init`** writes the default `.mtt/config.yaml` (types + flow, no commands). Defaults live in the
  init template, not in logic.
- Consequence for ordering: contract + types + adapter (with ID minting) — phase 1; flow enforcement with
  command execution — phase 3.
- **Positioning (see DESIGN.md → "Positioning vs beads"):** our wedge is per-type flow + zero-footprint +
  adaptivity (a thin agent layer over the existing backend). Dependencies stay simple, the KB is low
  priority; `mtt-ui` is a nice optional default, not the main argument. ID collisions are accepted
  consciously (don't complicate until real parallelism appears).

## What to read first (in order)

1. [CLAUDE.md](CLAUDE.md) — the entry point
2. [AGENTS.md](AGENTS.md) — rules, gate, principles, DoD
3. [DESIGN.md](DESIGN.md) — architecture and decisions
4. [TASKS.md](TASKS.md) — the plan; next up — section **e2 (Phase 1)**

## Activating guards (superpowers)

The plugin is declared in the personal `.claude/settings.local.json` (per-user, gitignored)
(`superpowers@superpowers-marketplace`). Plugins load **at session start**:

1. On opening the project, Claude Code may show a trust prompt for the marketplace
   `obra/superpowers-marketplace` — confirm it (once).
2. If the skills don't appear automatically, run once:
   ```
   /plugin marketplace add obra/superpowers-marketplace
   /plugin install superpowers@superpowers-marketplace
   ```
   (alternative — the official marketplace: `/plugin install superpowers@claude-plugins-official`)
3. Verify the TDD/brainstorming/debugging skills are available, and **use them**.

## Next task — session 003 (list & edit)

- Start with **[sessions/003_list_and_edit.md](sessions/003_list_and_edit.md)**; branch
  `feat/s003-list-edit`. Refine its plan (superpowers brainstorming/planning), then work **test-first**;
  the acceptance e2e + `make check` must pass before the PR.
- Session 003 rounds out flat task CRUD before hierarchy: **`mtt list`** (filters `--status`/`--type`,
  stable order) and **`mtt edit <id> [--title] [--description]`** (non-flow fields only — status changes
  go through flow enforcement in session 006). Both build on what 002 shipped: the `Task` model, the
  `TaskStore` port, `internal/core`, and flat per-prefix ID minting.
- **Global flags (cross-cutting — land now to avoid per-command retrofit; see CLI_REFERENCE.md → "Global
  flags"):** wire on the root command as **persistent flags** so later commands inherit them —
  `--dir`/`MTT_DIR` (project-root override; also DRYs the repeated `Getwd → FindRoot` in init/types/add/show),
  the `--version` flag (`--help` is already cobra-provided), and `--json` (machine-readable output — `mtt list`
  is its first real consumer). Defer `--role`/`MTT_ROLE` to 006 (with `history`) and `--quiet`/`--no-color` to
  later. If 003 gets heavy, `--json` may split into a small follow-up slice — the session brainstorm decides.
- Architecture stays the full **`cli → core → port ← adapter`**: `list`/`edit` usecases live in
  `internal/core` behind the `TaskStore` interface (public `pkg/mtt`); `core` must NOT import `adapter/*`.
- **Reference (authoritative model):** session 002's spec/plan under
  `docs/superpowers/{specs,plans}/2026-07-04-session-002-*`, and DESIGN.md/AGENTS.md (now updated with the
  flat-ID and `--no-parent`/`Status.Default` decisions taken in 002).

### Open questions to resolve in 003's brainstorm (do NOT skip)
1. **Enumerating tasks — the port grows.** `TaskStore` is only `Create`/`Get`. `list` needs to enumerate
   tasks: add `List()`/`All()` to the port (enumeration is the adapter's job), then filter/sort where? Apply
   the 002 correction ("a pure read needs no `core` usecase") — is `list` a direct port read, or does
   filter/sort count as logic that belongs in `core`? Draw the "query → adapter" vs "logic → core" line.
2. **Editing — a mutating store method.** `edit` needs a store `Update`/`Save` (atomic write; currently only
   `Create`). Decide which fields are editable (title/description) vs explicitly rejected with a clear error
   (id/type/status/parent — status is flow-only in 006, type is immutable, parent is re-parenting/later);
   bump `updated` via the injected clock.
3. **`--json` shape + global-flags scope.** The machine-readable output shape, and how many global flags land
   in 003 (all of `--dir`/`--version`/`--json`, or split `--json` into a small follow-up if 003 gets heavy).
4. **Deterministic `list` order** (by id? by created?) for stable golden/e2e output.

### Carry-over lessons (001 & 002 — save review loops)
- **CLI output → stdout**: use `fmt.Fprint(cmd.OutOrStdout(), …)`, NOT `cmd.Print/Printf` (those route to
  stderr when no writer is set — breaks pipes and e2e `stdout` asserts). Errors surface via `Execute` (stderr).
- **golangci-lint `unused`** (standard set) fails on unused unexported package-level consts/funcs — declare
  a symbol in the task that first *uses* it, not ahead of time.
- **testscript e2e**: pin `go-internal@v1.14.1` (latest needs Go ≥1.25); use `testscript.Main` (not the
  deprecated `RunMain`); make sure an "uninitialized dir" case is a genuine sibling of the init'd dir (no
  `.mtt` ancestor) or the assertion is vacuous. Drive the real binary; assert stdout vs stderr correctly.
- **testscript assertions must anchor, not substring-match** (a 002 review catch): assert e.g.
  `'t1  task  \[tbd\]'`, not a bare `'task'` — a loose substring can match vacuously against unrelated output.
- **A pure read needs no `core` usecase** (a 002 correction): `show` reads a task directly through the
  `TaskStore` port; only mutations (`add`) go through `core`. Don't over-layer a query — apply the same
  question to `list` in 003.
- **New package → its own thin `CLAUDE.md`** (per AGENTS): `internal/core` got one in 002; keep it current
  as `list`/`edit` land in 003.

## Ready-to-paste kickoff prompt (for a new session)

> We're continuing mtt. Sessions 001 (init & types) and 002 (create & view) are merged to `main`. Read, in
> order: CLAUDE.md, AGENTS.md, DESIGN.md, TASKS.md, NEXT_SESSION.md, sessions/README.md,
> sessions/003_list_and_edit.md, CLI_REFERENCE.md ("Global flags"), and the session-002 spec/plan under
> docs/superpowers/{specs,plans}/2026-07-04-session-002-*. Make sure the superpowers skills are active
> (otherwise activate them per NEXT_SESSION.md). We work in compact sessions; do **session 003 (list &
> edit)** on branch `feat/s003-list-edit`: first brainstorm/refine the plan (superpowers brainstorming →
> writing-plans), then implement strictly test-first until the acceptance e2e + `make check` are green;
> branch → PR → CI green → merge to `main`.
>
> Scope: `mtt list` (filters `--status`/`--type`, stable order; human output + `--json`) and
> `mtt edit <id> [--title] [--description]` (non-flow fields only — a status change is flow enforcement in
> 006; type is immutable; parent/re-parenting is later; bump `updated`). Session 003 also **owns the
> global-flags surface** (CLI_REFERENCE → "Global flags"): wire `--dir`/`MTT_DIR`, the `--version` flag, and
> `--json` as root **persistent flags** so later commands inherit them (`--dir` also DRYs the repeated
> `Getwd → FindRoot`); defer `--role`/`MTT_ROLE` to 006 and `--quiet`/`--no-color` to later.
>
> Architecture stays the full `cli → core → port ← adapter`; `core` must NOT import `adapter/*`. Apply the
> 002 correction — a pure read needs no `core` usecase (`show` reads directly via `TaskStore.Get`) — to
> `list`. Resolve the **Open questions** in NEXT_SESSION (chiefly the `TaskStore` port evolution —
> `List`/`Update` beside `Create`/`Get` — and whether `list` reads directly or filters/sorts in `core`; plus
> `edit`'s editable-vs-rejected fields, the `--json` shape, and `list` ordering). Heed the "Carry-over
> lessons" (CLI stdout via `fmt.Fprint(cmd.OutOrStdout(), …)`; anchor testscript assertions; `golangci
> unused`; new package → its own thin CLAUDE.md). Follow SOLID/DRY/KISS/TDD/DDD/clean-architecture and the
> self-check from AGENTS.md.
