# NEXT_SESSION — primer

A living handoff doc. Update it at the end of each session (what's done / what's next).

## Where we are

- **Phase 0 (scaffold) + sessions 001–006 + 006.5 + 006.7 + 007 + 008 + 008.5 + 008.6 + 008.7 + 008.9 + 008.95 are DONE** (version `0.8.9-dev`, `make check` green).
  **Chore 008.95 (release prep)** shipped first-release groundwork (version-neutral — no runtime change, so
  `0.8.9-dev` stays): **`make release`** (5-platform GOOS/GOARCH cross-compile → `dist/` version-stamped raw
  binaries + `SHA256SUMS`; `VERSION` required; out of `make check`, non-hermetic like `smoke`),
  **`.github/workflows/release.yml`** (on `v*` tag → `make release VERSION=<tag>` → `gh release create`, built-in
  `gh`, **no third-party action**; checkout@v7 + setup-go@v6 mirroring `ci.yml`), README/.ru **Install** (prebuilt
  binary + `SHA256SUMS` + `go install` fallback + build-from-source) + **Quickstart** (runnable, verified) + a
  staleness pass (version `0.8.5`→`0.8.9-dev`; the stale hierarchical-ID claim `e1_t3_s2` → flat per-prefix),
  **`CHANGELOG.md`** (features under `[Unreleased]`), and **`RELEASING.md`** (runbook) + a DESIGN/.ru "Releasing"
  pointer. The `v0.9.0` **tag + publish is user-triggered after s009 dogfood** (this chore only builds the
  machinery). **Bundled feature — flow guidance on entry:** `mtt status`/sugar print the traversed edge's +
  destination status's `description` and the onward moves (`next: …`) on stdout after a move, and `mtt show`
  surfaces the current status's `description` + `next` (human + `--json` via `showJSON`), turning the flow's
  authored text into **inline instructions for the agent** (pure `Type.StatusByName`/`TransitionsFrom`; spec
  `docs/superpowers/specs/2026-07-09-flow-guidance-on-entry-design.md`). A **flow-granularity** artifact for
  s009 is at `docs/superpowers/notes/2026-07-09-flow-granularity-for-dogfood.md`. Next: **s009 dogfood**.
  **Session 008.9 (batch & pipeline)** shipped a reusable **task-set selector** (`internal/cli/selector.go`,
  `selectTaskIDs`) — 3 **mutually-exclusive** sources (explicit ids | stdin `-` | `--filter` over
  `core.Select`/`Ready`; >1 or 0 = usage error; present-but-empty = no-op exit 0; dedup; **never** resolves
  `current`) + the shared filter flags (`addSelectorFilterFlags`/`readSelectorFilter`/`filterActive`); an
  **`--ids`** output on `list`/`ready` (`⊕ --json`); a shared **`runBulk`** (`bulk.go`: best-effort per item,
  `reportBulk` stdout summary + stderr per-item / `--json` per-item array, `previewBulk` `--dry-run`, a plain
  aggregate `fmt.Errorf` → **generic exit 1**, NEVER `%w`-wrapping a per-item sentinel); **`core.Remover.RemoveMany`**
  (subgraph-aware — `externalReferencingIDs` excludes in-set referents, so `rm <epic> <child>` needs no
  `--force`; `Remove` is a thin wrapper keeping single-id exit-4). **`rm` is `ArbitraryArgs`** (single
  `runRmSingle` verbatim vs bulk `RemoveMany`); **`tag add/rm` is marker-driven** (`tagArgs`: a `-`/filter marker
  → positionals are tags + selector; else the single `applyTagSingle` path). The adversarial spec review caught a
  MAJOR (cobra validates `Args` **before** `RunE`, so the old `idAndTags`/`ExactArgs(1)` would reject the bulk
  forms) — fixed pre-code. e2e `batch.txt`. Next: **s009 dogfood**.
  **Session 008.7 (tags)** shipped a pure `pkg/mtt` **tag vocabulary** (`tag.go`: `NormalizeTag`/`ExtractTags`
  over a **Unicode** `\pL\pN._-` charset, Unicode `ToLower`, no NFC — verified against RE2) and `Task.Tags` as a
  normalized+deduped+**sorted set** (open vocabulary → plain `[]string`, not a VO), riding the field +
  `TaskStore.Update` (**no new port**, GAP #1). `#hashtags` in title/description are the **primary** authoring
  path: `core.Adder` unions explicit `--tag` with `ExtractTags(title/desc)` into `canonicalTags`, and
  `core.Editor` reconciles on a **write-time text-delta** (`reconcileTags` — drop a tag when its `#hashtag`
  leaves the text, add new ones, keep manual tags; **no provenance** → a text+manual collision drops with the
  text, documented). The secondary path is **`core.TagEditor`** (`AddTags`/`RemoveTags`, idempotent, rides
  `Task.Tags`+`Update`; `RemoveTags` is **guarded** — refuses a tag whose `#hashtag` is still in the text, all
  targets validated before any write → atomic; `load` wraps `ErrNotFound` → exit 4). Filtering is a slice-valued
  **`ListFilter.Tags`** OR-within `Match` dimension (`anyOrEmptyIntersect`, the slice analogue of scalar
  `anyOrEmpty`). CLI: `mtt tag add/rm <id> <tag>…` (variadic), `--tag` (repeatable) on `add`/`list`/`tree`, a
  `tags:` line in `show`, `taskJSON.tags` (`omitempty`); the shared `toTags` normalizes/validates at the
  boundary. The YAML adapter needed **no code change** (the DTO already round-trips `tags,omitempty`); a golden
  `task_tags.yaml` locks the field position (adapter copies verbatim — sorting is a `core` invariant). Next:
  **s008.9 batch & pipeline**.
  **Session 008.6 (priorities + roadmap)** shipped a closed **`Priority` VO** (`pkg/mtt`, `high|medium|low`;
  `Valid()`/`Rank()`; empty = unset, ranks `medium`; the `StatusKind`/`CurrentAction` idiom — `type + consts +
  Valid()`, cast in `toDomain`, validated at the CLI boundary, **no** smart constructor) riding **`Task.Priority`
  + `TaskStore.Update`** (no new port, GAP #1) with `omitempty` on disk (existing files byte-untouched); the
  yaml DTO round-trip (plain conversion, unknown tolerated → ranks medium, + a `task_priority.yaml` golden);
  core `SortPriority` (+ `lessByPriority` → `lessByRecency`), `ListFilter.Priorities` + `Match` (matches the
  *stored* label), `EditParams.Priority` (+ clear via `--priority ""`), and the shared **`terminalSatisfied`**
  predicate factored out of `Ready`; the pure **`core.Roadmap(tasks,cfg) []RoadmapEntry`** — a cycle-safe
  priority-guided Kahn over **two "comes-after" axes** (`depends_on` **and `parent`** — a child precedes its
  parent; both hard) with **priority propagation** (a blocker takes `effectivePriority` = min of own and
  everything it transitively unblocks, so a high task pulls its prerequisites forward), building its **own**
  non-terminal graph (not `DepGraph`) and reusing `core.Ready` (depends_on-only) for `ready` + `terminalSatisfied`
  for `blocked_by`; each entry also carries `Contains` (a parent's non-terminal children); and the CLI surface —
  `--priority` on `add`/`edit`/`list`, `--sort priority`, `priority` in `show` + `taskJSON`, and **`mtt roadmap
  [--json]`** (`roadmapJSON`: honest `priority` `""`, non-null `blocked_by`/`contains` `[]`; `↳ blocked by:` / `↳
  contains:` lines). The roadmap ordering was **reworked post-ship on user feedback** (rev2 in the spec): the
  first cut was greedy-by-own-priority (`roadmap == list --sort priority`) — dropped for propagation + the parent
  axis. Next: **s008.7 tags**.
  **Session 008.5 (dogfood enablers, chore)** shipped `mtt rm <id>` — hard-delete via a new base-port method
  **`TaskStore.Delete`** (the *D* in CRUD; YAML `os.Remove`) + the pure `core.Remover` usecase (reject-if-referenced —
  children via `Index`, dependents via `DepGraph`, deduped — with `--force` to override, leaving dangling refs the
  system already tolerates); `mtt rm` takes an **explicit id** (no current resolution — destructive) and clears a
  stale `current` pointer. **`--depends-on` on `mtt add`** sets `depends_on` at creation, validated + deduped in
  `core.Adder` (no cycle possible — unminted id). **Packaging:** `make install`/`build` stamp the version via
  conditional `-ldflags` (`make build VERSION=…`), a new `make smoke` installs into a throwaway `GOBIN` and runs
  `version`/`--help`, `version.go` now prints to **stdout**, default bumped `0.8.0-dev` → `0.8.5-dev`. **Not-found
  → exit `4` made UNIFORM:** every single-task-by-id path (`rm`/`show`/`edit`/`tree`/`use`/`status`/`dep`) wraps
  `mtt.ErrNotFound` via the `taskNotFound` helper (cli) / `%w` (core), so `exitCode` maps them all to 4. Next:
  **s008.6 priorities + roadmap** (spec already written + subagent-reviewed).
  **Session 008 (rollback/compensation)** shipped an additive per-command `pkg/mtt.Command.Rollback *Command`
  (a **leaf** compensator; `Valid()` rejects a nested rollback), recursive `ymlCommand.rollback` (scalar|map) +
  `toDomain` deep-copy, **eager** rollback expansion (`expandOne`/`expandTemplate` — a bad rollback template is
  exit 1 before any side effect), the `core.Runner` port method **`Compensate`** (best-effort, labeled
  `↩ compensating` phase in the exec adapter), and `core.Transitioner` compensation on a block — the
  succeeded-prefix rollbacks run **in reverse** (single `failIdx` source), the outcome stays `ErrBlocked`
  (exit 3), the task is **unchanged**, **no history**, and the block error carries a `compensated N …` summary;
  `mtt types` shows `↩ <rollback>`. Next: **s008.5 dogfood enablers** (`mtt rm`, `--depends-on`, `make install`).
  **Session 007 (structured commands)** shipped the `pkg/mtt.Command` value object (`{Run, Timeout}`;
  `Transition.Commands` is now `[]Command`), **placeholder expansion** on `run` (`.ID`/`.Type`/`.From`/`.To`
  via `text/template` in `core.Transitioner` — a self-enforcing shape-safe whitelist; `pkg/mtt` stays
  template-agnostic), a **per-command timeout** overriding the global `command_timeout` (resolved in the exec
  `Runner`, global as fallback), back-compat via `ymlCommand.UnmarshalYAML` (bare scalar **or** `{run, timeout}`
  map), `Runner.Run([]mtt.Command)` (Run expanded at the boundary; `Check.Cmd` records the expanded command),
  and `mtt types` showing per-command timeouts. Next: **s008 rollback/compensation**.
  **Session 006.7 (current task / working context)** shipped `mtt use [<id>] [--clear]` (a personal
  git-`HEAD`-for-tasks pointer in `config.local.yaml`), the additive `pkg/mtt.Transition.Current` (`set|clear`)
  rule + the `CurrentStore` capability port (`yaml.NewCurrent` writes `config.local` via a comment-preserving
  `yaml.Node`; the CLI applies set/clear after a move — `core.Transitioner` untouched), the pure
  `Type.FindTransition` primitive, and **omitted-id resolution** to the current task for
  `status`/`mtt <status>`/`show`/`edit` only (never list/tree/dep/ready).
  **Session 006.5 (attribution + verb sugar)** shipped `--why` (durable reason; `HistoryEntry.Why` + DTO +
  `show`), `--who` (mutually-exclusive alias of `--by`), the `mtt <status> <id>` **verb sugar** (fallback-routing
  in `root.RunE` — reuses `core.Transitioner`; unknown arg0 → exit 1; real command wins a clash), and
  **required-attribution** (`require:{who,why}` committed adapter `Settings.Require`, `config.local` tighten-only;
  validated in `core.Transitioner` before the gate, aggregated `ErrMissingAttribution` → **exit 2**; `--no-run`
  no-bypass). `-v`/`--log-file` moved to root-persistent. **Session 006
  (flow gate — the killer feature)** shipped `mtt status <id> <new>` — a **single** gated transition: the
  `core.Runner` port (`Run(commands)`, no `dir`) + `internal/adapter/exec` (per-command timeout, cwd=root,
  cross-platform shell seam) + a fake in tests; `core.Transitioner` (single-edge lookup, gate → `ErrBlocked`,
  append `history`, `Update` — no new port); config-driven `command_timeout` (adapter `Settings`, 5m default);
  `--role`/`--by` (+ env) recorded into history; **exit codes 3 (blocked) / 6 (invalid)** via `Execute() int`;
  and a `history:` section in `mtt show`. **Session 005 (dependencies)** shipped `mtt dep add/rm/list <id>`
  (`--tree`/`--cycles`), `mtt ready`, and `list --ready` over `core.DependencyEditor` (no new port — the edge
  rides `Task.DependsOn` + `TaskStore.Update`), a conservative `core.Ready`, and a derived `core.DepGraph`.
  Phase 2 (e3) is complete and Phase 3 (e4) is underway; **next is session 006.7 — current task (working
  context)** (`current` in `config.local`, set/cleared via a transition property; omitted id → current for
  single-task verbs; `mtt use <id>`), then s007 structured commands. **`advance`/`start`/`done` + roles
  are PARKED** (on-demand — single-edge `status` is the norm). Session 001
  shipped `mtt init [--template default|coding] [--force] [--name]` and `mtt types [<type>]`. Session 002
  shipped `mtt add [title] [--type] [--no-parent] [--description]` and `mtt show <id>`. Session 003 shipped
  `mtt list` (`--status`/`--type`/`--sort created|updated`/`--json`), `mtt edit <id> [--title]
  [--description]`, and the global flags `--dir`/`MTT_DIR`, `--version`, `--json`. **Session 004 (hierarchy)**
  shipped `mtt add --parent <id>` (validated placement, mutually exclusive with `--no-parent`), `mtt tree
  [<id>] [--status/--kind/--depth/--json]` (ASCII, keep-ancestors, nested JSON), `mtt list --parent/--kind`,
  and the `mtt show` lineage breadcrumb — over a derived `core.Index` (computed children/ancestors, cycle-safe)
  and a shared `core.Match` predicate. CI runs on Node 24 action majors.
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
  (see "Open design slice" below). Hierarchy landed in 004; dependencies/`ready`/cycle-detection landed in
  005; **flow enforcement (transition validation + executable `commands` gates via the `Runner` port)** is next.
- Work is organized in **compact sessions** (see [sessions/README.md](sessions/README.md)); next up is
  **session 006** (flow gate: `mtt status <id> <new>` runs & gates the transition's `commands`, writes
  `history`; the `Runner` port + `internal/adapter/exec` + a fake; `--role`/`MTT_ROLE` recorded).
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

## Domain-model snapshot (read before s007)

[docs/architecture/model.go](docs/architecture/model.go) — a code-form, tiered (T1/T2/T3) index of the whole
intended contract: domain types + ports + optional capabilities, core usecases with dependencies, the derived
resolved graph, and open gaps. Two decisions locked there that shape s005:

- **s005 adds no new port.** `depends_on` is a `Task` field round-tripped via `TaskStore.Update` (as `parent`
  was in s004); `DependencyStore` is only for external adapters that cannot embed. s005 = core
  `DependencyEditor` + `Ready` + cycle-check, no `pkg/mtt` port method.
- **Chore 004.5: typed-identity retrofit — DONE.** `TaskID`/`TypeName`/`StatusName` are now used across the
  shipped `pkg/mtt`/`core`/`adapter`/`cli`; the YAML DTO keeps plain strings on disk and maps `string↔typed`
  at its boundary (`toDomain` fails fast on a corrupt empty `id`/`type`/`status`). s005 is written against the
  typed contract. Constructors reject empty, no transform; `Ref.ID` stays `string`; `NoteSlug` deferred (KB).

## Next task — session 009 (dogfood)

> **s008.9 (batch & pipeline) is SHIPPED** — see "Where we are" and "Carry-over lessons (008.9)" below; the
> spec is `docs/superpowers/specs/2026-07-09-session-008.9-batch-design.md` and the plan is
> `docs/superpowers/plans/2026-07-09-session-008.9-batch.md`. **Next up is s009 dogfood** (e5_t2): `mtt init`
> **this repo**, author a config whose gates are **task-aware** (a branch on the `→ in_progress` edge via a
> `{{.ID}}` placeholder, `make check` on `→ done`), and **migrate the backlog** (TASKS.md / sessions/README
> roadmap) onto mtt itself — the selector + bulk `rm`/`tag` (s008.9) are what make a bulk migration practical.
> After dogfood, `TASKS.md` freezes and mtt tracks its own work. Then references (s010), comments (s011), actor
> profiles (s012). `advance`/`start`/`done` + modes + roles-on-edges + node-level status-actions + cross-edge
> compensation stay **PARKED**. No spec yet — start with brainstorming → writing-plans. **Consider first** (a
> dogfood design question): how much of the backlog to migrate vs keep in docs, the id/prefix scheme for
> self-hosted tasks, and whether the gates shell out to `make check` (slow) or a lighter task-level check.

### Carry-over lessons (008.9 — batch & pipeline)
- **cobra validates `Args` BEFORE `RunE`, on the flag-stripped positionals — a context-sensitive command needs
  a context-sensitive `PositionalArgs`, not a fixed arity.** The adversarial review's MAJOR: `idAndTags` (≥2)
  and `ExactArgs(1)` would reject the bulk forms (`tag add y --status tbd` → 1 positional; `rm t1 t2` → 2)
  *before* the marker classification ever runs. The validator closure receives `cmd` (flags already parsed), so
  it can call `filterActive(cmd)`/`hasDash(args)` — `tag` requires ≥1 with a marker else ≥2; `rm` moved to
  `ArbitraryArgs` (the no-source case becomes the selector's usage error in `RunE`).
- **A best-effort bulk aggregate MUST be a plain `fmt.Errorf` (no `%w` of a per-item error).** `exitCode` maps
  via `errors.Is`; per-item `TagEditor`/`RemoveMany` errors wrap `ErrNotFound`, so wrapping one into the
  aggregate would mis-map the whole bulk to exit 4 (or 3). `fmt.Errorf("%d of %d task(s) failed", …)` keeps the
  generic exit 1; unit-lock it (`errors.Is(err, ErrNotFound)` must be false). A **single** `rm <id>`/`tag <id>`
  still returns the wrapped per-item error verbatim, so its exit 4 survives.
- **Subgraph-aware bulk delete = `referencingIDs − set`, over a single static snapshot.** `RemoveMany` builds
  `Index`+`DepGraph` once from one `List`, and the referenced-check excludes referents that are themselves in
  the deletion set (`externalReferencingIDs`), so an epic+children delete needs no `--force`. The **static**
  snapshot (not re-read during iteration) is what makes it order-independent and avoids a delete-during-iteration
  hazard. A single id degenerates for free (set={id} ⇒ every referent external ⇒ today's reject). Keep a per-id
  `store.Get` for existence (the List snapshot is only the graph) so `Remove`'s `load task %q` wording survives.
- **The selector is a CLI concern, not core.** Two of three branches are pure I/O (stdin, arg parsing); the
  third reuses `core.Select`/`Ready`. Putting an `io.Reader`/cobra flags into `core` would break "core has no
  I/O". Source detection keys off a **marker** (a `-` among positionals, or a filter flag `Changed()`), not the
  result count — so "source present but 0 results" (no-op, exit 0) is cleanly distinct from "no source" (usage
  error), and two sources are a conflict caught before any mutation.
- **`--ids` first-used in its own unit test, not the wiring task.** `golangci unused` fails on a package-level
  func with no use; `writeIDs` is declared in `selector.go` (Task 1) but first wired into `list`/`ready`
  (Task 2), so a `TestWriteIDs` in the same package (Task 1) is what keeps the Task-1 `make check` green. The
  same "declare where first used" rule caught `ready.go` needing an added `"fmt"` import for the `--ids`⊕`--json`
  guard (unlike `list.go`, which already imports it) — a transient IDE "imported and not used" clears once the
  usage lands.
- **testscript has no shell pipes.** Model `a | b` as `exec a` → `cp stdout ids.txt` → `stdin ids.txt` →
  `exec b -` (the `stdin` directive resets after each command). A txtar `-- file --` unpacks into `$WORK`, but
  the script `cd proj`s into `$WORK/proj`, so reference it as `$WORK/file`. `stdout` regexp is whole-output —
  use `(?m)^id$` to assert an id is on its own line in `--ids` output.
- **Context-sensitive positional layout beats breaking a good single form.** `tag`'s marker rule keeps
  `tag add t1 backend` (the frequent explicit-single case) working *and* adds bulk without a `--tag`-flag
  migration; the only subtlety is that on `tag` the `--tag` *flag* is a task **filter** while positionals are
  the tags to add/remove — reads naturally (`tag rm urgent --tag backend`), documented.

### Carry-over lessons (008.7 — tags)
- **An open, *transforming* vocabulary is a plain `[]string` + pure funcs, NOT a VO and NOT an identity.**
  Closed vocabularies (`Priority`) are VOs; named identities (`TypeName`) "reject empty, never transform". Tags
  are neither — they are user-open **and** need normalization (Unicode lowercase). So `Task.Tags` stayed a
  plain `[]string` (matching the reserved model, dodging `[]Tag` churn through DTO/JSON), with the vocabulary
  rules as two pure `pkg/mtt` functions (`NormalizeTag`/`ExtractTags`, alongside the type-query predicates).
  The canonical form (normalize+dedup+**sort**) lives in one core helper (`canonicalTags`).
- **A "primary via free text, secondary via command" field reconciles at WRITE time via a text-delta — no
  provenance stored.** `#hashtags` in title/description are the main authoring path; `Adder` unions them on
  create, `Editor` reconciles on a text change by `removed = oldTextTags − newTextTags` (drop those, add new,
  keep manual). The one anomaly (a tag added *both* by text and by `tag add` drops when the `#hashtag` is edited
  out) is inherent to being provenance-free — **document it, don't build provenance**. Capture `oldTitle/oldDesc`
  **before** the in-place apply (the `Editor` overwrites fields, so reconcile-after-apply needs the snapshot).
- **A slice-valued filter dimension needs its own helper — `anyOrEmpty` is scalar.** `Match`'s existing
  `anyOrEmpty(filter, scalar)` can't test slice∩slice; add `anyOrEmptyIntersect(filter, have []T)` (empty
  filter ⇒ true; else non-empty intersection). OR-within, AND-across is preserved by short-circuiting it
  alongside the other dimensions.
- **A "refuse while the source still holds it" guard has no bypass and validates ALL targets before any
  write.** `tag rm` refuses a text-anchored tag (its `#hashtag` is in the text) — faithful to "the text is
  authoritative". For a variadic call, check every target first, then apply, so one guarded target blocks the
  whole call (atomic). The guard names the field (title vs description) by re-checking `ExtractTags(title)`.
- **An embedded-field mutation usecase wraps `ErrNotFound` with `%w` so the CLI keeps exit 4 for free.**
  `TagEditor.load` mirrors `DependencyEditor.load` (`fmt.Errorf("task %q: %w", id, mtt.ErrNotFound)`); the CLI
  then just `return err` (like `dep`), and `exitCode` maps it to 4. Using `%v` would silently degrade to 1 —
  add a "missing id → ErrNotFound" unit test to lock it.
- **Unicode boundary, ASCII trap.** Go's `\w` is ASCII-only, so a `#tag` glued to a non-ASCII word
  (`тег#backend`) escaped an ASCII-`\w` adjacency guard and got extracted. Use `[^\pL\pN_#]` for the boundary
  (RE2 supports `\pL`/`\pN` in classes) and the same classes in the token, so both boundary and charset are
  Unicode. **Run the actual regex through RE2 on a table of tricky inputs** (Cyrillic, `café#x`, `##h`, `#L42`,
  trailing punctuation) before trusting a claim about what it "rejects" — my spec first over-claimed `#include`
  was rejected (it isn't; it's an accepted false positive of scanning description).
- **The formatter split: `formatTask` is in `show.go`, `taskLine` in `format.go`.** (A recurring trap —
  grep before editing.) `show.go` already imports `strings`; the tags line goes there, not in `format.go`
  (which would need an unused-import-triggering `strings` add).
- **A DTO golden proves field *position* + `omitempty`, NOT sort order.** The adapter copies `Tags` verbatim
  (round-trip already covered elsewhere with an *unsorted* slice); the sorted-set invariant is a `core` test
  (`canonicalTags`), not implied by the golden. Feed the golden already-sorted tags to mirror what core emits.
- **Two adversarial subagent reviews (spec, then plan) each caught a real defect a self-review missed** — the
  spec's `#include` over-claim / exit-4 wiring gap, and the plan's `format.go`-vs-`show.go` file-target BLOCKER.
  Worth the tokens on a moderately involved slice; the transient IDE "declared and not used" diagnostics during
  multi-edit flag wiring are noise (they clear once all edits land — build/`make check` is the real gate).

### Carry-over lessons (008.6 — priorities + roadmap)
- **A new closed VO mirrors `StatusKind`/`CurrentAction` — `type + consts + Valid()`, cast in `toDomain`,
  validated at the boundary; no smart constructor.** `Priority` shipped exactly like `CurrentAction`: the DTO
  does a plain `mtt.Priority(yt.Priority)` cast (no error path, no `toDomain` churn), validity is a CLI-boundary
  check (`parsePriority` → `!Valid()` usage error), and a corrupt on-disk value is **tolerated** (ranks medium),
  mirroring lazy status validation. `Rank()` gives the sort order; empty/unknown both rank medium, so the
  ordering never needs to special-case unset.
- **An embeddable field rides `Task` + `Update` — no new port (GAP #1 again).** Priority joined
  `depends_on`/`tags` on the "can the reference adapter embed this?" → yes → `omitempty` field + `Update` path.
  `omitempty` on the off-disk default (unset) is what keeps existing task files & goldens byte-unchanged —
  prove it with a golden that has **no** priority (`task_min.yaml` untouched) *and* one that does
  (`task_priority.yaml`).
- **A pure derived read builds its OWN restricted graph rather than overloading the shared one.** `Roadmap`
  needed a **non-terminal-restricted** DAG (a terminal blocker imposes no ordering constraint), but
  `DepGraph.Dependents` are unfiltered — so it builds `indeg`/`dependents` from scratch over the node set, and
  does **not** extend `DepGraph` (GAP #6 stays unextracted). The reuse that *did* pay off: `core.Ready` for the
  `ready` flag (one source of truth) and a `terminalSatisfied` predicate **factored out of `isReady`** as the
  single home for "is this blocker satisfied?" (shared by `Ready` and `Roadmap`).
- **Align a new order with the existing one so they never diverge.** `roadmap`'s same-priority tiebreak reuses
  the **shared `lessByPriority`** (Rank asc → `lessByRecency`), identical to `Select`+`SortPriority`, so
  `roadmap` is provably a dependency-constrained `list --sort priority` — not a second, drifting comparator.
- **Priority-guided Kahn: emit one, re-sort the available set, repeat; append the stuck set best-effort.** Seed
  `avail` with indeg-0 nodes, `sort.SliceStable(avail, lessByPriority)` before each pop (a node freed mid-walk
  may outrank the rest), decrement dependents' indeg, push newly-zero ones. Any node never reaching indeg 0 is
  **in — or downstream of — a cycle**; append the remaining nodes sorted by `(Rank, recency)` so the function
  always terminates and returns **every** node. Determinism comes from the sort (total order over unique IDs),
  not input order — assert it with a 20×-stable test.
- **JSON honesty for a derived view: stored value, non-null arrays.** `roadmap --json` emits the **stored**
  `priority` (`""` when unset — **not** `omitempty`; the consumer applies its own default) while the *ordering*
  treats unset as medium — the label is never fabricated. `blocked_by` is always a non-null array (`[]` when
  empty, built with `make([]string, 0, …)`) — the house rule, like `dep list --json`. `taskJSON.priority` is
  `omitempty` (a task field, absent when unset) — a deliberate contrast with roadmap's always-present field.
- **e2e asserts ordering *relationships*, not absolute positions (the s003 wall-clock lesson).** With `time.Now`
  timestamps that tie at second resolution, exact numbering shifts; a `(?s)t1  \[low\].*t2  \[high\]`
  multiline regexp captures the hard-constraint (blocker before dependent) robustly, and `--sort priority` uses
  that a *unique* `high` (rank 0) is strictly first. The deterministic exact order is the **unit** test.
- **Run the primitive by hand before declaring it done — an intuitive spec can still encode the wrong model.**
  The first roadmap cut was correct *to the spec* (greedy Kahn by own priority, subagent-verified) — but running
  `mtt roadmap` on a real scenario made two things obvious: a *high* task hidden behind a blocker sinks below an
  independent *medium* one, and a parent/epic isn't ordered relative to its children. Fix (rev2, same PR): **two
  ordering axes** — `depends_on` **and** `parent` (a parent completes only once its children do → a child
  precedes its parent), both hard — plus **priority propagation** (`effectivePriority(n)` = min of own and
  everything n transitively unblocks, memoized + cycle-safe), so a high task pulls its prerequisites forward.
  Kept clean: `Ready`/`BlockedBy` stay **depends_on-only** (the parent axis is ordering + a new `Contains`
  annotation, not readiness — an epic with open children is `Ready` yet ordered last), and the two claims the
  first cut leaned on (`roadmap == list --sort priority`; "greedy, not a scheduler") were explicitly retracted.
  Lesson: for a derived-order primitive, a manual run on a lifelike shape is worth more than another test — the
  spec was internally consistent yet modelled the domain wrong.
- **Parent-child is a second dependency axis, distinct from a link.** Hierarchy (`parent`) is not just display:
  a container can't be *completed* until its children are, so it's a real "comes after" ordering edge — but it
  is NOT a `depends_on` (readiness/`blocked_by` stay about explicit blockers). Model both axes in the ordering
  graph, annotate them separately (`blocked by:` vs `contains:`), and don't conflate the vocabularies.

### Carry-over lessons (008.5 — dogfood enablers)
- **A store operation earns a base-port method (the D in CRUD), unlike an embedded field.** `depends_on`/
  `history`/`current` ride `Update` (or a capability port); *delete* cannot be embedded in the aggregate it
  removes, so `TaskStore.Delete` goes on the mandatory-minimum port (YAML `os.Remove`). Read GAP #1 the other
  way: "can the reference adapter embed this?" — no → base-port method.
- **A new interface method's blast radius is every implicit implementer — fold the fakes into the same
  commit.** Adding `Delete` to `TaskStore` broke three `internal/core` test fakes (`fakeStore`/`memStore`/
  `editStore`, and `memStore` is reused by `transition_test.go`); stubbing all three is part of commit #1 or
  the package won't compile. Grep for fakes before an interface change.
- **A uniform exit-code taxonomy is a wrap-with-`%w` pass, and it changes error *text*.** Making not-found →
  exit 4 uniform meant wrapping `mtt.ErrNotFound` in every single-task-by-id path (`taskNotFound` helper in
  cli; `fmt.Errorf("…: %w", …, mtt.ErrNotFound)` in core). The wrapped message (`… : mtt: task not found`)
  keeps the `not found` substring (so substring e2e survive) but breaks exact-wording asserts — two testdata
  scripts (`add_show.txt`, `tree.txt`) needed the new wording. Grep `not found` across tests/testdata first.
- **`rm` is agent-facing: no interactive confirm, no attribution.** A `y/N` prompt hangs an agent (no stdin);
  a deleted task has no `history` to sign, so `--who`/`--why` don't apply — the git commit is the audit.
  Safety comes from reject-if-referenced + `--force`, and an **explicit id** (no current resolution on a
  destructive verb). Reused `Index.Children` + `DepGraph.Dependents` for the referenced-check — no new graph.
- **Conditional-LDFLAGS keeps `make build` predictable.** `VERSION ?=` empty → the code default
  (`internal/cli.version`) ships; `make build VERSION=v0.8.5` stamps via `-X`. A `make smoke` (throwaway
  `GOBIN`, `trap` cleanup, one `set -e` backslash-joined recipe — `set -e` so a broken `--help` actually fails
  it) install-tests the binary; kept out of `check` (real `go install` is non-hermetic). Also fixed `version.go`
  to print to **stdout** (`cmd.Println` → `OutOrStderr` meant `$(mtt version)` was empty); gated by a
  separate-buffer `TestVersionCommand` so a revert isn't silently green.
- **`rm --force` + `max+1` minting = id resurrection (impl-review [major], documented not fixed).** Deleting
  the highest-numbered task frees its id; a later `add` reuses it and **silently re-points** a dangling
  `depends_on`/`parent` at the new, unrelated task. Rooted in the pre-existing `mint` scheme (no high-water
  mark), which `rm` first makes reachable — so it was **documented** as a `--force` caveat (DESIGN/.ru,
  CLI_REFERENCE/.ru) + a monotonic-minting **think-item** (TASKS → Later), not smuggled into the chore. Lesson:
  a delete op re-opens id-allocation questions a create-only history had hidden — decide reuse-vs-monotonic
  before dogfooding at volume.

### Carry-over lessons (008 — rollback / compensation)
- **Additive VO field with a self-referential pointer + a leaf invariant.** A per-command compensator that is
  itself a structured command is `Rollback *Command` (a pointer breaks the infinite-size struct). Keep it a
  **leaf** (`rollback.Rollback == nil`) and enforce that in `Valid()` — the recursion (`expandOne`,
  `ymlCommand.UnmarshalYAML`, `toDomain`) then terminates at one level, and the config-time check catches
  nonsense. Struct comparability survives (pointers are comparable), so existing `cmd == mtt.Command{…}` tests
  keep compiling.
- **Recursive DTO for free via `*ymlCommand`.** Making the `rollback` YAML field a `*ymlCommand` reuses the
  s007 scalar-or-`{run,timeout}` `UnmarshalYAML` for the compensator automatically; a recursive
  `ymlCommand.toDomain()` **deep-copies** it (a fresh `*mtt.Command`, never aliasing the DTO pointer — assert
  this with a mutate-the-DTO test).
- **Compensation is core policy; execution + the labeled phase are the adapter's.** `core.Transitioner` decides
  *what* to compensate (which succeeded, reversed); the exec `Runner.Compensate` runs it **best-effort** and
  owns the `↩ compensating` progress header (core stays free of I/O). One new port method beat a core-side
  `Run`-per-command loop precisely because the header needs the progress writer. Best-effort never changes the
  outcome — still `ErrBlocked` (exit 3), no new code.
- **Derive "which command failed" from a single source.** Compute an explicit `failIdx` (`firstFailure` index
  for a non-zero check; `len(checks)-1` for an operational error) and compensate `expanded[:failIdx]` in
  reverse — do **not** re-infer it as "the last check" at one site and "the first non-zero" at another (they
  coincide only because the exec runner stops at the first non-zero; a divergent Runner would misclassify). The
  failed command's own rollback is then structurally never run.
- **`blocked → no history` is the reconciliation for compensation audit.** A blocked transition did not happen
  (no `from→to`), so `Task.History` (a *transition* journal) gets nothing and the task file is untouched;
  compensation is a side-effect event surfaced only live (progress) + in the block summary. A durable
  side-effect audit stays the parked edit-audit slice.
- **Document the `Runner.Run` operational-failure contract** (the failing command's `Check` is the last
  element, `Exit -1`) since compensation math depends on it; make the test fake replicate it.
- **e2e proves the mechanism, not git.** Rollback is for **arbitrary** commands; the acceptance e2e uses generic
  POSIX `touch`/`rm`/`false` (no `[exec:git]` guard) and a multi-command reverse compensation — precise
  reverse-order / best-effort / succeeded-only / no-history are **unit** tests (the s006/s007 split). The full
  swapped config must be a valid flow (`mtt add` runs `Config.Validate`).
- **Validation runs on `add`/`types`, not `Load`/the gate path** — the pre-existing s006/s007 status quo. Don't
  claim a domain invariant is "enforced at Load"; a config-time invariant only bites where `Config.Validate` is
  actually called.

### Carry-over lessons (007 — structured commands)
- **Domain-vs-policy split for a per-edge property.** The s006 rule "execution policy (`command_timeout`) rides
  the adapter `Settings`, not `pkg/mtt`" does **not** bar a *per-command* timeout from the domain: the global is
  one runner knob (applies to every command, external trackers run none), but a per-command timeout is an
  **authored property of a flow edge**, inseparable from its `run` (which already lives in `Transition.Commands`).
  Test: "is this a runner default, or authored on this specific edge?" Default → adapter `Settings`; authored →
  the domain VO. The runner resolves per-command-else-global, keeping the global out of `core`.
- **Reuse the DTO's mandatory custom `UnmarshalYAML` to parse and keep `toDomain` error-free.** `ymlCommand`
  needed a custom unmarshal anyway (scalar OR map back-compat); folding `time.ParseDuration` into it means a bad
  duration surfaces at `Load` (like `parseCommandTimeout`) and `toDomain` stays a pure, error-free copy — no
  smart constructor, no `toDomain` signature churn (the s006.7 lesson holds). Decode the map branch into a
  string-`Timeout` alias, never back into `ymlCommand` (infinite recursion; yaml.v3 can't decode `30s` into
  `time.Duration`).
- **A `text/template` struct context is a self-enforcing whitelist.** Exposing only `cmdContext{ID,Type,From,To}`
  means `{{.Title}}` (or any free-text/typo) is a template error at `Execute` — the struct's shape *is* the
  injection policy; no shell-quoting, no field allow-list check. Keep templating in `core` (where the policy
  lives), not `pkg/mtt` (which stays template-agnostic, storing the raw template). An expansion error is a plain
  error (exit 1), distinct from a gate block (`ErrBlocked`, exit 3) — a malformed command is a config fault, not
  a failed gate.
- **Expand with the pre-move status.** `.From` must be the status being *left*, so capture `from := t.Status`
  **before** `t.Status = to` and build the context from it; the same `from` feeds the history entry. `--no-run`
  skips expansion together with the gate (nothing to expand if nothing runs).
- **Type-migration task in a green-between-commits plan.** Flipping `Transition.Commands []string → []Command`
  breaks every consumer at once (unavoidable — the field type drives them). Land it as one behavior-preserving
  refactor commit (DTO maps string→`{Run}`, runner/exec/core/`types`/tests updated), *then* add the new YAML
  form + expansion + timeout on top. Each commit stays `make check`-green.
- **`Check.Cmd` records the expanded command** (`git checkout -b task/t1`, not the template) — a truthful audit;
  it falls out for free because `core` passes expanded `Command`s to the runner.
- **e2e: assert an unborn branch via `git symbolic-ref --short HEAD`, not `git branch --list`.** On a fresh
  `git init` with no commits, a newly-created branch is unborn and invisible to `git branch --list` (empty
  stdout); `symbolic-ref` reports it and needs no user config. Guard the one git-shelling script with
  `[!exec:git] skip`. Keep the slow-gate sleep short (~1s): `exec.CommandContext` kills at the timeout but
  `Run()` blocks until the orphaned child closes the inherited output pipe (same as `TestRunTimeout`), so a long
  sleep only slows the test — the block still fires.

### (Completed) session 006.7 brief — current task / working context

- **Create `sessions/006.7_current_task.md` from `sessions/000_template.md`** (design-spec + plan before
  code). Branch `feat/s006.7-current-task`. Refine the plan (superpowers brainstorming/planning), work
  **test-first**; acceptance e2e + `make check` green before the PR. See TASKS.md → **e4_t8a** and DESIGN.md →
  "Working context: the current task".
- Scope (kills id-repetition — git-`HEAD`-for-tasks):
  1. A **`current`** record in `config.local.yaml` (personal, gitignored — the **value**); companion
     `mtt use <id>` (git-checkout-like set) + a way to show the current task.
  2. Set/clear driven by a **transition property** in the **committed** flow (the **rule**) — a new additive
     `pkg/mtt.Transition` field (e.g. `current: set|clear`; name-agnostic; a topology default
     set-on-→active / clear-on-→terminal is a brainstorm option). This is the s006.7 `pkg/mtt` change.
  3. An **omitted id** resolves to `current` **only for single-task direct verbs** (status / `mtt <status>` /
     show / edit / tag) — never for filter/list/stdin/bulk (resolution order: explicit id > filter/stdin >
     current). Composes with the s008.9 selector (its "no source" single-verb case = current).
- **Caveat:** a shared checkout with multiple agents = one `config.local` = one `current` → collision;
  per-agent current ties to the parked subagent-identity think-item (fine for solo / one-agent-per-checkout).
- Reuse the s006.5 shape: the sugar and `mtt status` both funnel through `runTransition`; an omitted id
  resolves before that. Everything typed; string conversion only at the cli/adapter boundary.
- **PARKED (do not build):** `advance`/`start`/`done`/`cancel`, modes, roles-on-edges, config verb→status.
- **After 006.7 → s007 structured commands** (placeholders + per-command timeout — the "work in task terms"
  enabler; `Transition.Commands []string` → a `Command` value object, a domain-shape change), s008 rollback,
  s008.5 dogfood-enablers chore, s008.7 tags (+`#hashtags`), s008.9 batch & pipeline, then dogfood (s009).

### Carry-over lessons (006.7 — current task / working context)
- **A capability port is justified when the data is non-embeddable — that's the GAP #1 test, read the other
  way.** `depends_on`/`history` embed in `Task` and ride `Update`, so YAGNI the port (s005). `current` is a
  personal, gitignored, single-value pointer — *not* task state — so even YAML cannot embed it; it needs a
  separate store. That non-embeddability is exactly what earns `CurrentStore` a port **now** (unlike the parked
  `DependencyStore`). Use "can the reference adapter embed this in the aggregate?" as the port-vs-field test.
- **Interpreting a declared flow effect is dispatch, not policy — apply it at the composition root.** The
  set/clear RULE is domain data (`Transition.Current`); *applying* it (`set`→set, `clear`→clear) makes no
  decision, so the CLI reads the edge (via `Type.FindTransition`) and calls the port after a successful move —
  `core.Transitioner` stays a pure gate. Contrast the gate itself (non-zero → blocked, no history), which *is*
  policy and rightly lives in core. Accepted seam: when the parked `advance` unparks and needs the same, extract
  a shared core apply-edge-effects step (revisit-at-the-second-caller, like DepGraph/Index).
- **Round-trip a human-edited config file with `yaml.Node`, not a struct decode.** `config.local.yaml` carries
  `author` + comments a user wrote. `NewCurrent` reads/mutates/writes only the top-level `current:` key through
  a `yaml.Node` (upsert/delete on the root mapping's `Content`), so comments and other keys survive. A struct
  decode+re-encode would silently drop them. Keep it independent of `Load` (which ignores the unknown key
  non-strictly) — one object owns the pointer (SRP).
- **New value object → mirror `StatusKind`, not a smart constructor.** `CurrentAction` ships as
  `type + consts + Valid()`, validated in `Config.Validate`/`validateFlow`, and cast in `toDomain` — no
  `NewCurrentAction`, no `toDomain` signature churn. The spec suggested a constructor; the codebase idiom
  (`StatusKind`) is cast-then-Validate. Follow the idiom; note the deviation.
- **Defer discovery scaffolding until a consumer exists.** `CapCurrent`/`Capabilities()` were dropped from scope:
  no `Capability` vocabulary exists in code yet, `mtt caps` (e4_t6) is unstarted, and CLI-applies (option ii)
  never type-asserts for the capability — so the const would be dead surface. Ship the port; fold `CapCurrent`
  into `mtt caps`. Guard the port with `var _ mtt.CurrentStore = (*Current)(nil)`.
- **Default type needs `--no-parent` in tests.** The `default` template's default type (`task`) has
  `parents: [epic]`, so `mtt add A` in a fresh default project fails placement — unit/e2e that just want a task
  pass `add A --no-parent` (or ship a single-root-type `-- local.yaml --` like the e2e does).
- **Omitted-id resolution is a shared pre-step, kept off set/bulk verbs.** `resolveTaskID(root, explicit)`
  (explicit id > current; stale/absent → actionable error) is called by `status`/sugar/`show`/`edit` **before**
  the usecase; `list`/`tree`/`dep`/`ready` never call it. The 1-arg sugar (`mtt done`) resolves current first,
  then classifies arg0 against *that* task's flow — mirroring the s006.5 2-arg sugar shape.

### Carry-over lessons (006.5 — attribution + verb sugar)
- **Verb sugar via `root.RunE` fallback, NOT command registration.** Setting `root.Args = cobra.ArbitraryArgs`
  + `root.RunE = runSugar` makes cobra call the root for any unknown first arg; real subcommands still dispatch
  first (so a real command always wins a name clash, for free). `runSugar` classifies via `trySugar` (load
  project, `Get(arg1)`, `Type.StatusKind(arg0)`); a miss returns `unknown command` (exit 1) — do **not**
  swallow it into a confusing transition error. `mtt` no-args → `cmd.Help()`. Reuse this shape for s006.7's
  omitted-id resolution (resolve the id **before** `runTransition`, keep the single shared path).
- **Attribution/policy enforcement lives in `core.Transitioner`, checked before the gate.** `require:{who,why}`
  is adapter `Settings` (like `command_timeout`), passed into `TransitionOptions` as bools; the check
  aggregates missing fields into one `ErrMissingAttribution` (exit 2) **before** `runner.Run`, so `--no-run`
  can't bypass it and both `status` + sugar get it (they share `runTransition` → `Transitioner`). Put the new
  exit code in `exitCode(err)` (mirrors 3/6).
- **`config.local` "tighten-only" = OR with the pre-overlay committed value.** yaml.v3 overlay would let a
  local `who:false` relax a committed `who:true`; capturing committed `require` **before** `decodeInto(local)`
  and OR-combining prevents relaxation while still allowing local to add. Reuse for any future
  committed-vs-local policy where local must not weaken the project rule.
- **Mutually-exclusive aliases: manual check beat `MarkFlagsMutuallyExclusive`.** With `--who`/`--by`
  root-persistent and used by two commands (`status` + sugar), a manual `Changed("who") && Changed("by")` →
  error in the shared `resolveAttribution` is path-uniform and dodges cobra's persistent-flag flag-group
  cross-command subtleties.
- **`formatTask` lives in `show.go`, not `format.go`** (`format.go` is only `taskLine`) — grep before editing.
- **Root-persistent flags only some commands honor is an accepted pattern** (`--json` since s003; `-v`/
  `--log-file` since s006.5) — moving a gate-output flag up to root is fine; remove the now-duplicate local
  flag or cobra panics on the redefinition.

### Open design slice to schedule (not session 006's scope, but don't lose it)
- **Durable, git-independent audit of edits** + **the subject-identity (`By`) source.** `edit` today only
  bumps `updated`; git is the de facto audit trail. A change-log or field versioning (additive) would make
  edit history queryable without git, and needs an identity source for "who" (likely
  `.mtt/config.local.yaml`, distinct from `--role`, which is "what hat"). See DESIGN.md → "Listing and
  editing" / TASKS.md → "Later (coarse)". Schedule a design pass before it's needed for real auditing.

### Carry-over lessons (006 — flow gate)
- **First driven port beyond storage = a fake in tests, a real adapter in `internal/adapter/*`.** `core.Runner`
  is defined in `core` (only it needs it), implemented by `internal/adapter/exec`, and faked in `core` tests —
  no process spawned in unit tests; the exec adapter is unit-tested directly (`true`/`false`/timeout). Reuse
  this shape for any future driven port.
- **Deviate from model.go when the layer is cleaner for it — and update model.go.** `Runner.Run(commands)`
  dropped the `dir` param (the exec adapter holds `cwd=root`), keeping `core` free of filesystem paths. The
  snapshot is a guide, not a contract; sync it when you deviate.
- **A non-zero exit is DATA, not a Go error.** The `Runner` returns `[]Check` (cmd+exit) and reserves the
  Go `error` for operational failures (launch/timeout). `core.Transitioner` decides blocked-vs-applied from the
  checks. Don't conflate "the gate said no" with "the runner broke".
- **Adapter-level settings ride the config layer, not `pkg/mtt`.** `command_timeout` is execution policy (an
  external tracker runs no commands), so it went into the YAML adapter's `Settings{Prefixes, CommandTimeout}`
  from `Load` (like `prefix`), never into the pure domain. Default in code (5m), overridable via config +
  `config.local`. Widening `Load`'s adapter-return is cheap: `_`-discard callers are untouched.
- **Exit-code taxonomy lives in `Execute() int`.** `Execute` maps core sentinels (`ErrBlocked`→3,
  `ErrInvalidTransition`→6, else 1); `main` and the **testscript harness** both do `os.Exit(Execute())` —
  changing that signature silently breaks the e2e harness if `TestMain` isn't updated in the same task (it was).
  testscript `!` asserts non-zero, not a specific code — unit-test the numbers (`exitCode`) separately.
- **e2e configs go in a txtar `-- gated.yaml --` file, `cp`'d over `.mtt/config.yaml` after `init`.** The
  default flow has no `commands`; to exercise a gate, ship a single-type root config with `commands: ["true"]`
  / `["false"]` on edges. `mtt add`'s title is **positional** (`mtt add 'A'`), not a `--title` flag.
- **Single-edge lookup beat `ResolvedFlow` for `status`** — a linear scan of `Type.Transitions` (YAGNI, the
  s005 "don't force the abstraction" lesson again). `ResolvedFlow` earns its keep in s007's multi-edge walk.
- **Sentinels for a new outcome live where the policy is.** `ErrBlocked`/`ErrInvalidTransition` are in `core`
  (flow is core policy), matched via `errors.Is` — mirror of how `ErrNotFound` sits in `pkg/mtt` (port contract).

### Carry-over lessons (005 — dependencies)
- **No new port for an embedded edge** (GAP #1 confirmed): `depends_on` rides `Task.DependsOn` + `TaskStore.Update`
  (like `parent` in 004); `core.DependencyEditor` owns the cycle-check. The `DependencyStore` capability stays
  unimplemented until an external adapter needs it — **YAGNI** the signature too (`NewDependencyEditor(store, now)`,
  no nil capability param). Apply the same to `history`/`comments` in later sessions.
- **Conservative derived-read semantics**: `core.Ready` requires *positive confirmation* — an unresolvable status
  (config drift, `kindOf` false) or a dangling blocker leaves a task **not** ready, mirroring how `Match` fails an
  unresolvable `--kind`. Prefer honest-not-ready over optimistic-ready for anything the data can't confirm.
- **Second derived graph kept separate** (GAP #6 not extracted): `DepGraph` (multi-edge DAG over `depends_on`) and
  `Index` (single-parent tree over `parent`) don't share a traversal primitive — a shared one would be forced.
  Revisit only if a third graph (`ResolvedFlow`, s006) naturally shares it. Both still reuse `lessByRecency` for order.
- **One primitive, two commands**: `mtt ready` and `list --ready` are both `Select(Ready(tasks, cfg), filter, cfg)` —
  readiness AND the list filters compose (both ANDs, order-independent). Extract the subset builder, don't duplicate.
- **e2e can't reach states a later session unlocks**: no status transition exists until s006, so `dep.txt`/`ready.txt`
  prove blocking + cycle rejection, while unblock-on-terminal and `--cycles`-on-a-real-cycle are **unit** fixtures
  (a `done`-blocker task / a hand-built cycle — the CLI's `add` rejects cycles, so it can't build one). Note the gap
  explicitly rather than faking the state.

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
  JSON consumers don't treat the same as an empty array. (004: nested `tree --json` — top-level slice is
  `make([]T,0,…)`; leaf `children` use `omitempty`.)
- **Derived graphs live in `core`, not the contract** (004): `core.Index` (parent→children/ancestors) is a
  pure value built from `TaskStore.List` — no store, no clock, **not** in `pkg/mtt`. Back-refs are computed,
  cycle-safe (visited-set), orphans surface as roots. Keep the sibling comparator shared with `Select`
  (`lessByRecency`) so tree and list order identically (DRY). Reuse this shape for `depends_on` in 005.
- **One predicate, many consumers** (004): `core.Match` (status/type/kind/parent) backs both `list` and the
  `tree` walk. When a filter needs config (e.g. `--kind` → `Type.StatusKind`), thread `cfg` into the pure
  function (`Select(tasks, ListFilter, cfg)`) — `cfg` is domain data, so the function stays pure.
- **Mutually-exclusive cobra flags**: `cmd.MarkFlagsMutuallyExclusive("parent","no-parent")`; the e2e error
  text to assert is `if any flags in the group`.
- **Tree filter semantics = keep-ancestors** (004 decision): a node shows if it matches or any descendant
  matches; non-matching ancestors stay as the path. Prove render/order by **unit test** (fixed clock); the
  e2e asserts presence.

## Ready-to-paste kickoff prompt (for a new session)

> Продолжаем mtt. Сессии 001–008 + 008.5 + 008.6 + 008.7 + 008.9 (+ 006.5/006.7/007) смёржены в `main`, версия
> `0.8.9-dev`, `make check` + CI зелёные. **s009 (dogfood) ещё НЕ заспечено.** Общайся по-русски.
>
> Прочитай сначала (в порядке): CLAUDE.md → AGENTS.md → DESIGN.md (секции «Implementation order» — фаза 4
> dogfood; «Flow: executable transitions» — гейты; «Positioning») → NEXT_SESSION.md (секции «Where we are»,
> «Next task — session 009», «Carry-over lessons» 008.9/008.7/008.6/008/007/006) → sessions/README.md (роадмап:
> 009 ← next) → TASKS.md (e5_t2) → sessions/008.9_batch.md (свежий образец сессии) → CLI_REFERENCE.md
> (init/status/`--depends-on`/tag/rm + селектор/`--ids`, таблица exit-кодов). Убедись, что superpowers-скиллы
> активны.
>
> Спеки НЕТ — начни с **brainstorming → writing-plans**, затем строго TDD/итеративно до зелёного `make check`.
> Если всплывут неоднозначности — уточни, не гадай. Ветка `feat/s009-dogfood` от свежего `main`; ветка → PR → CI
> green → мёрдж в `main`. Бампни версию `0.8.9-dev` → `0.9.0-dev` (s009 — полная сессия → минор).
>
> Scope (ориентир — уточни в брейнсторме): **self-host** — `mtt init` **этого репозитория** с конфигом, чьи
> гейты **task-aware** (ветка на ребре `→ in_progress` через плейсхолдер `{{.ID}}`; `make check` на `→ done`),
> и **миграция бэклога** (TASKS.md / sessions/README) на mtt (селектор + bulk `rm`/`tag` из s008.9 делают
> массовую миграцию практичной). После dogfood `TASKS.md` замораживается, mtt ведёт свою разработку. Реши в
> брейнсторме: сколько бэклога переносить vs оставить в доках; id/prefix-схему self-hosted задач; гейт шеллит
> `make check` (медленно) или лёгкую задачную проверку. Это НЕ обычная CLI-фича — это интеграция/конфиг + доки.
>
> Heed «Carry-over lessons», особенно (008.9): cobra валидирует `Args` ДО `RunE` (context-sensitive команда →
> context-sensitive `PositionalArgs`); bulk-агрегат — plain `fmt.Errorf` (НЕ `%w` per-item, иначе exit-код
> мис-мапится); non-zero exit — ДАННЫЕ; CLI-вывод через `fmt.Fprint(cmd.OutOrStdout(), …)`; zero-match `--json` =
> `[]` (`make([]T,0)`); anchored testscript-ассерты (`(?m)^id$` для построчного вывода; нет пайпов — `cp stdout`
> → `stdin`); e2e ассертит отношения, не абсолютные позиции (wall-clock ties); `golangci unused` (символ
> объявляй там, где ВПЕРВЫЕ используется); гейт-конфиг для e2e — txtar `-- gated.yaml --` cp'нутый поверх
> `.mtt/config.yaml`; **гоняй `make check` ПЕРЕД каждым коммитом БЕЗ пайпа** (не только `go test`); отдавай
> спеку и план на адверсариальное субагент-ревью; отражай фичи в `--help`. Docs-sync tick: DESIGN.md/.ru,
> CLI_REFERENCE.md/.ru (**+обнови статус-шапку в ТУ ЖЕ сессию**), model.go, TASKS e5_t2 ✅, sessions/README 009
> ✅, NEXT_SESSION (+carry-over 009), заполни sessions/009_*.md Done.
>
> После s009 → references (s010). **PARKED** — не делать: advance/start/done/cancel + режимы + роли-на-рёбрах;
> node-level status-actions; кросс-рёберная компенсация. Фанатично: SOLID/DRY/KISS/TDD/DDD/clean-arch + self-check
> из AGENTS.md.
