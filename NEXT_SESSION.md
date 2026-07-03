# NEXT_SESSION — primer

A living handoff doc. Update it at the end of each session (what's done / what's next).

## Where we are

- **Phase 0 (scaffold) is complete**, the `make check` gate is green, commit(s) on `main` (local).
- **No GitHub push yet** (waiting for an explicit "go" from the user).
- Stack: Go 1.23, cobra; storage — YAML file-per-task (see DESIGN.md).

## The session starts with planning (mandatory)

Before any code — a planning phase (use the superpowers skills: brainstorming/planning). The plan must
account for the key invariants from DESIGN.md:

- **Types and hierarchy are domain (from config); ID/slug is the adapter's job.** The code has NO literals
  for type names or ID structure. Hierarchy comes from a type's `parent` field. The ID is minted by
  `TaskStore` (YAML: `<prefix><N>` along the chain, `e1` → `e1_t3` → `e1_t3_s2`; `prefix` is a YAML-adapter field).
- **Invariants (validated on config load):** the type set has a default `task`; each status has a category
  `kind` (initial/active/terminal), with ≥1 `terminal` (the default has `done`+`cancelled`), and
  ready/list work by category; every flow has the anchors `tbd → in_progress → done` in that order.
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

## Next task — Phase 1 (after planning)

- Branch: `feat/phase-1-core` (implementation — after the plan).
- Task guide: **e2** in [TASKS.md](TASKS.md) (planning refines the breakdown/order).
- **Test-first** (TDD: red → green → refactor). `make check` green before every commit.
- Architecture — **hexagonal**: `cli → core → port ← adapter`, the contract (domain types + ports) in the
  public `pkg/mtt`. `core` doesn't import `adapter/*`.
- Order in substance: the `pkg/mtt` contract (types + `TaskStore` port) → config+types + `mtt init` (the ID
  depends on types) → `internal/adapter/yaml` → `internal/core` (ID minting, usecases) → the
  `add/list/show` commands.
- Create a `CLAUDE.md` for each new package (`pkg/mtt`, `internal/core`, `internal/adapter/yaml`, …).

## Ready-to-paste kickoff prompt (for a new session)

> We're continuing mtt. Read CLAUDE.md, AGENTS.md, DESIGN.md, TASKS.md and NEXT_SESSION.md.
> Make sure the superpowers skills are active (otherwise activate them per NEXT_SESSION.md).
> Start by PLANNING phase 1. Hexagonal architecture: the contract (domain types + ports
> `TaskStore`/`KnowledgeStore`) in the public `pkg/mtt`, logic in `internal/core`, YAML — the default
> adapter in `internal/adapter/yaml`; types/hierarchy/ID come from config (no literals in code),
> `mtt init` writes the epic/task/subtask defaults with flow. Then implement on the branch
> `feat/phase-1-core`, strictly test-first.
> Follow the principles (SOLID/DRY/KISS/TDD/clean architecture) and the self-check from AGENTS.md.
