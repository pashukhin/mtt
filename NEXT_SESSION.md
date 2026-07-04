# NEXT_SESSION — primer

A living handoff doc. Update it at the end of each session (what's done / what's next).

## Where we are

- **Phase 0 (scaffold) + session 001 (init & inspect) + session 002 (create & view) + session 003 (list &
  edit) are DONE** (003 done on branch `feat/s003-list-edit`, `make check` green, PR not yet opened,
  e2e `list_edit.txt` passing). Session 001 shipped `mtt init [--template default|coding] [--force]
  [--name]` and `mtt types [<type>]`. Session 002 shipped `mtt add [title] [--type] [--no-parent]
  [--description]` and `mtt show <id>`. Session 003 shipped `mtt list` (`--status`/`--type`/`--sort
  created|updated`/`--json`), `mtt edit <id> [--title] [--description]`, and the global flags
  `--dir`/`MTT_DIR`, `--version`, `--json`. CI runs on Node 24 action majors.
- **In place now:** the pure `pkg/mtt` contract (`Config/Type/Flow/Status/Transition`, the `StatusKind`
  value object, structural `Config.Validate()`, `DefaultType`/`ChildrenIn`), the YAML adapter's **config
  layer** (`FindRoot`, `HasProject`, embedded `default`/`coding` templates, atomic `Init`, `Load` +
  `config.local.yaml` overlay, DTO↔domain mapping, prefix/one-default checks), the `Task` model + full
  `TaskStore` port (`Create`/`Get`/`List`/`Update`, `pkg/mtt`), `internal/core`'s `Adder`/`Select`/`Editor`
  (add is a mutation usecase; list-select is a pure filter/order function; edit is a mutation usecase, all
  clocked via an injected `now`), the YAML adapter's **flat per-prefix ID minting** (`O_EXCL`) + deterministic
  serialization, and the CLI's `projectRoot`/`baseDir` (DRY root resolution) + `taskJSON` (shared `--json`
  view for `show`/`list`/`edit`). **Not yet:** hierarchy (`--parent`/`tree`), dependencies, flow enforcement,
  comments; also deferred out of 003 — a durable edit-audit trail and the subject-identity (`By`) source
  (see "Open design slice" below).
- Work is organized in **compact sessions** (see [sessions/README.md](sessions/README.md)); next up is
  **session 004** (hierarchy: `mtt add --parent`, `mtt tree`).
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

## Next task — session 004 (hierarchy)

- No `sessions/004_*.md` file yet — **create it from `sessions/000_template.md`** as the first step
  (mirrors how 003 started: a design-spec + plan commit before implementation), named per the roadmap
  (`sessions/README.md`): `004 — hierarchy`. Branch `feat/s004-hierarchy`. Refine the plan (superpowers
  brainstorming/planning) before writing code; work **test-first**; the acceptance e2e + `make check` must
  pass before the PR.
- Scope per the roadmap: **`mtt add --parent <id>`** (place a task under a parent; validate the parent's
  type against the child type's `parents`, per the "Hierarchy sanity" invariant in DESIGN.md) and **`mtt
  tree [<id>] [--status …] [--kind …] [--depth <n>]`** (render the epic → task → subtask hierarchy;
  optionally rooted at `<id>`). `mtt list --parent <id>` (direct children only) is the natural companion —
  see CLI_REFERENCE.md → `mtt list`.
- Architecture stays the full **`cli → core → port ← adapter`**: hierarchy traversal is a pure read (in
  `internal/core`, alongside `Select`) built from `TaskStore.List` — apply the "pure read needs no usecase
  struct" lesson only where there's truly no mutation; `add --parent` still goes through `core.Adder` (a
  mutation) since it already validates placement.
- **Reference (authoritative model):** DESIGN.md → "Types and hierarchy" / "Model invariants" (hierarchy
  sanity, type immutability) and → "Listing (`list`) and editing (`edit`) — session 003" (the shipped
  `Select`/`Editor` this session builds on).

### Open design slice to schedule (not session 004's scope, but don't lose it)
- **Durable, git-independent audit of edits** + **the subject-identity (`By`) source.** `edit` today only
  bumps `updated`; git is the de facto audit trail. A change-log or field versioning (additive) would make
  edit history queryable without git, and needs an identity source for "who" (likely
  `.mtt/config.local.yaml`, distinct from `--role`, which is "what hat"). See DESIGN.md → "Listing and
  editing" / TASKS.md → "Later (coarse)". Schedule a design pass before it's needed for real auditing.

### Carry-over lessons (001, 002 & 003 — save review loops)
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
  `TaskStore` port; only mutations (`add`) go through `core`. 003 applied the same split to `list`:
  `core.Select` is a pure function (no store injected, no usecase struct) the CLI composes with
  `TaskStore.List`, while `edit` (a mutation) got a real usecase (`core.Editor`).
- **New package → its own thin `CLAUDE.md`** (per AGENTS): `internal/core` got one in 002, kept current in
  003 (`Select`/`Editor`); keep doing this for hierarchy in 004.
- **List order is provider-agnostic** (003): the default order is a **domain timestamp** (`Created` desc, or
  `Updated` desc with `--sort updated`), never ID structure — an external adapter's IDs may not sort
  meaningfully. Ties break on the ID as an **opaque string** compare. Order determinism is **unit-tested**
  (`core.Select`); the e2e (`list_edit.txt`) deliberately asserts row **presence**, not sequence, since it
  drives the real binary against wall-clock timestamps that can tie at second resolution.
- **Zero-match `--json` must be `[]`, not `null`**: build the slice with `make([]T, 0, …)` before appending,
  so `encoding/json` marshals an empty result set as `[]` — a `nil` slice marshals to `null`, which most
  JSON consumers don't treat the same as an empty array.

## Ready-to-paste kickoff prompt (for a new session)

> We're continuing mtt. Sessions 001 (init & types), 002 (create & view), and 003 (list & edit) are done
> (003 done on branch `feat/s003-list-edit`, `make check` green, PR not yet opened). Read, in order: CLAUDE.md, AGENTS.md, DESIGN.md, TASKS.md,
> NEXT_SESSION.md, sessions/README.md, sessions/003_list_and_edit.md (for the shipped `Select`/`Editor`
> shape), and CLI_REFERENCE.md. Make sure the superpowers skills are active (otherwise activate them per
> NEXT_SESSION.md). We work in compact sessions; do **session 004 (hierarchy)** on branch
> `feat/s004-hierarchy`: first create `sessions/004_hierarchy.md` from `sessions/000_template.md` and
> brainstorm/refine the plan (superpowers brainstorming → writing-plans), then implement strictly
> test-first until the acceptance e2e + `make check` are green; branch → PR → CI green → merge to `main`.
>
> Scope: `mtt add --parent <id>` (hierarchy placement, validated against the type's `parents`) and `mtt tree
> [<id>] [--status …] [--kind …] [--depth <n>]` (render epic → task → subtask; optionally rooted at `<id>`);
> `mtt list --parent <id>` is the natural companion. Architecture stays `cli → core → port ← adapter`;
> `core` must NOT import `adapter/*`; hierarchy traversal is a pure read (like 003's `core.Select`) built
> from `TaskStore.List`, while `add --parent` stays a `core.Adder` mutation.
>
> Heed the "Carry-over lessons" in NEXT_SESSION.md (CLI stdout via `fmt.Fprint(cmd.OutOrStdout(), …)`;
> anchor testscript assertions; `golangci unused`; new package → its own thin CLAUDE.md; list order is a
> provider-agnostic domain timestamp, unit-test the order, e2e asserts presence; zero-match `--json` must
> serialize `[]` not `null`). Note the **open design slice** parked in NEXT_SESSION.md (durable edit-audit +
> subject-identity `By` source) — not in scope for 004, just don't lose it. Follow
> SOLID/DRY/KISS/TDD/DDD/clean-architecture and the self-check from AGENTS.md.
