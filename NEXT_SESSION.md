# NEXT_SESSION — primer

A living handoff doc. Update it at the end of each session (what's done / what's next).

## Where we are

- **Phase 0 (scaffold) + session 001 (init & inspect) are DONE and merged to `main`.** Shipped
  `mtt init [--template default|coding] [--force] [--name]` and `mtt types [<type>]`; `make check` green
  (coverage pkg/mtt ~94% / yaml ~81% / cli ~92%). CI runs on Node 24 action majors.
- **In place now:** the pure `pkg/mtt` contract (`Config/Type/Flow/Status/Transition`, the `StatusKind`
  value object, structural `Config.Validate()`, `DefaultType`/`ChildrenIn`) and the YAML adapter's **config
  layer** (`FindRoot`, embedded `default`/`coding` templates, atomic `Init`, `Load` + `config.local.yaml`
  overlay, DTO↔domain mapping, prefix/one-default checks). **Not yet:** `internal/core`, the `TaskStore`
  port, the `Task` model, ID minting.
- Work is organized in **compact sessions** (see [sessions/README.md](sessions/README.md)); next up is
  **session 002** (create & view tasks).
- Repo: <https://github.com/pashukhin/mtt> (public). Branch per session → PR → CI green → merge into `main`.
- Stack: Go 1.23, cobra, `gopkg.in/yaml.v3`, `go-internal/testscript` (e2e); storage — YAML file-per-task.

## The session starts with planning (mandatory)

Before any code — a planning phase (use the superpowers skills: brainstorming/planning). The plan must
account for the key invariants from DESIGN.md:

- **Types and hierarchy are domain (from config); ID/slug is the adapter's job.** The code has NO literals
  for type names or ID structure. Hierarchy comes from a type's `parents` field. The ID is minted by
  `TaskStore` (YAML: `<prefix><N>` along the chain, `e1` → `e1_t3` → `e1_t3_s2`; `prefix` is a YAML-adapter field).
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

## Next task — session 002 (create & view)

- Start with **[sessions/002_create_and_view.md](sessions/002_create_and_view.md)**; branch
  `feat/s002-create-view`. Refine its plan (superpowers brainstorming/planning), then work **test-first**;
  the acceptance e2e + `make check` must pass before the PR.
- Session 002 introduces what 001 deferred: the **`TaskStore` port** (in `pkg/mtt`), **`internal/core`**
  (the `add` usecase — create its `CLAUDE.md`), the **`Task` model** (with `history`/`refs`/`comments`
  field stubs), and **ID minting** in the YAML adapter (`<prefix><N>` along the parent chain, `O_EXCL`,
  deterministic serialization, atomic write, `<id>.yaml`).
- Architecture now grows to the full **`cli → core → port ← adapter`**: `core` depends on the `TaskStore`
  interface (public `pkg/mtt`), the YAML adapter implements it, the CLI wires it. `core` must NOT import
  `adapter/*`. (Config stays config-as-data as in 001; **tasks** go through the port.)
- **Reference (authoritative model):** session 001's spec/plan under
  `docs/superpowers/{specs,plans}/2026-07-03-session-001-*` and DESIGN.md.

### Open questions to resolve in 002's brainstorm (flagged from 001 — do NOT skip)
1. **Entry status when a type has ≥1 `initial`.** The corrected model allows multiple initial statuses, so
   `add` must choose the task's starting status. Decide the rule: a per-type entry marker, "first initial",
   or `--status`.
2. **Root-vs-parent for the default `add`.** The default type `task` has `parents: [epic]`, so a bare
   `mtt add` (no `--parent`; `--parent` is deferred to 004) cannot mint an `<epic>_t<N>` id and would
   violate parent-type hierarchy. The 002 acceptance's `e1` even implies creating an *epic*. Reconcile:
   does bare `add` create a root type, seed an epic, or require a minimal `--type`/`--parent` now? (ID
   minting for a root type like `epic` `parents: []` is just `e1`; a child type needs its parent's id.)
3. **Parent-type validation** (a task's parent must be an allowed parent type) — decide how much lands in
   002 vs 004.

### Carry-over lessons from 001 (save review loops)
- **CLI output → stdout**: use `fmt.Fprint(cmd.OutOrStdout(), …)`, NOT `cmd.Print/Printf` (those route to
  stderr when no writer is set — breaks pipes and e2e `stdout` asserts). Errors surface via `Execute` (stderr).
- **golangci-lint `unused`** (standard set) fails on unused unexported package-level consts/funcs — declare
  a symbol in the task that first *uses* it, not ahead of time.
- **testscript e2e**: pin `go-internal@v1.14.1` (latest needs Go ≥1.25); use `testscript.Main` (not the
  deprecated `RunMain`); make sure an "uninitialized dir" case is a genuine sibling of the init'd dir (no
  `.mtt` ancestor) or the assertion is vacuous. Drive the real binary; assert stdout vs stderr correctly.
- **New package → its own thin `CLAUDE.md`** (per AGENTS): `internal/core` needs one in 002.

## Ready-to-paste kickoff prompt (for a new session)

> We're continuing mtt. Session 001 (init & types) is merged to main. Read CLAUDE.md, AGENTS.md, DESIGN.md,
> TASKS.md, NEXT_SESSION.md, sessions/README.md, and the session-001 spec/plan under docs/superpowers/.
> Make sure the superpowers skills are active (otherwise activate them per NEXT_SESSION.md). We work in
> compact sessions; do **session 002** (sessions/002_create_and_view.md): first brainstorm/refine the plan —
> resolving the "Open questions" in NEXT_SESSION (entry-status with ≥1 initial; root-vs-parent for the
> default `add`) — then implement strictly test-first on branch `feat/s002-create-view` until its
> acceptance e2e + `make check` are green. Build the full hexagon now: the `TaskStore` port in the public
> `pkg/mtt`, the `add` usecase in `internal/core` (which must NOT import adapter/*), the `Task` model, and
> ID minting in internal/adapter/yaml. Heed the "Carry-over lessons" in NEXT_SESSION. Follow
> SOLID/DRY/KISS/TDD/DDD/clean-architecture and the self-check from AGENTS.md.
