# NEXT_SESSION — primer

A living handoff doc. Update it at the end of each session (what's done / what's next).

## Where we are

- **Phase 0 (scaffold) + sessions 001–005 are DONE** (version `0.5.0-dev`, `make check` green). **Session 005
  (dependencies)** shipped `mtt dep add/rm/list <id>` (`--tree`/`--cycles`), `mtt ready`, and `list --ready`
  over `core.DependencyEditor` (add/rm + cycle rejection, no new port — the edge rides `Task.DependsOn` +
  `TaskStore.Update`), a conservative `core.Ready` primitive (shared by `ready` + `list --ready`), and a
  derived `core.DepGraph` (over `depends_on`, kept separate from `Index`). Phase 2 (e3) is now complete;
  **next is session 006 — flow enforcement (the killer feature)**. Session 001
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

## Domain-model snapshot (read before s006)

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

## Next task — session 006 (flow gate — the killer feature)

- **Create `sessions/006_flow_gate.md` from `sessions/000_template.md`** as the first step (mirrors 003/004/005:
  a design-spec + plan commit before implementation), named per the roadmap (`sessions/README.md`):
  `006 — flow gate`. Branch `feat/s006-flow-gate`. Refine the plan (superpowers brainstorming/planning) before
  writing code; work **test-first**; the acceptance e2e + `make check` must pass before the PR.
- Scope per the roadmap (see also DESIGN.md → "Flow: executable transitions"): **`mtt status <id> <new>`** —
  a **single** transition validated against the type's `transitions`, running that edge's `commands` (all →
  0, else the move is **blocked**), and appending a `history` entry (`from→to`, `at`, `by`, `role` from
  `--role`/`MTT_ROLE`, `checks` results). Introduce the **`Runner` port** (defined in `core`) +
  `internal/adapter/exec` (run commands, per-command timeout, cwd = project root), with a **fake Runner** in
  tests. `--no-run` (bypass gates) and the `--role`/`MTT_ROLE` seam land here. `advance`/`start`/`done`
  (the meta-walk) is s007 — keep s006 to a single edge.
- Architecture stays **`cli → core → port ← adapter`**; `core` defines `Runner`, `adapter/exec` implements it,
  tests fake it — the first *driven* port beyond storage. `history` rides the `Task.History` field +
  `TaskStore.Update` (no `HistoryStore` port — same GAP #1 rule as `depends_on`). Weigh whether the resolved
  flow graph (`ResolvedFlow`, model.go Layer B) is worth building now or a single-edge lookup suffices (likely
  the latter for `status`; `ResolvedFlow` earns its keep in s007's multi-edge `advance`).
- **Reference (authoritative model):** DESIGN.md → "Flow: executable transitions (the killer feature)" and
  "Advancing through the flow"; model.go → `Runner`/`Advancer`/`ResolvedFlow` (T2), GAP #5 (`By` source).

### Open design slice to schedule (not session 006's scope, but don't lose it)
- **Durable, git-independent audit of edits** + **the subject-identity (`By`) source.** `edit` today only
  bumps `updated`; git is the de facto audit trail. A change-log or field versioning (additive) would make
  edit history queryable without git, and needs an identity source for "who" (likely
  `.mtt/config.local.yaml`, distinct from `--role`, which is "what hat"). See DESIGN.md → "Listing and
  editing" / TASKS.md → "Later (coarse)". Schedule a design pass before it's needed for real auditing.

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

> We're continuing mtt. Sessions 001–005 and chore **004.5 (typed-identity retrofit)** are merged to `main`,
> version `0.5.0-dev`, `make check` + CI green. Phase 2 is complete: s005 shipped dependencies —
> `mtt dep add/rm/list` (`--tree`/`--cycles`), `mtt ready`, `list --ready` over `core.DependencyEditor`
> (add/rm + cycle rejection, no new port), a conservative `core.Ready`, and a derived `core.DepGraph`.
> Read, in order: CLAUDE.md, AGENTS.md, DESIGN.md, NEXT_SESSION.md, sessions/README.md,
> `docs/architecture/model.go` (`Runner`, `Advancer`, `ResolvedFlow`, GAP #5), TASKS.md,
> sessions/005_dependencies.md (shipped `DependencyEditor`/`Ready`/`DepGraph` shape) and
> sessions/004_hierarchy.md, CLI_REFERENCE.md. Confirm the superpowers skills are active (else activate per
> NEXT_SESSION.md).
>
> Do **session 006 (flow gate — the killer feature)** on branch `feat/s006-flow-gate` off fresh `main`: first
> create `sessions/006_flow_gate.md` from `sessions/000_template.md`, then brainstorm → writing-plans, then
> implement strictly test-first until the acceptance e2e + `make check` are green; branch → PR → CI green →
> squash into `main`.
>
> Scope: **`mtt status <id> <new>`** — a single transition validated against the type's `transitions`, running
> that edge's `commands` (all → 0, else **blocked**), appending a `history` entry (`from→to`, `at`, `by`,
> `role` from `--role`/`MTT_ROLE`, `checks`). Introduce the **`Runner` port** (defined in `core`) +
> `internal/adapter/exec` (per-command timeout, cwd = project root) with a **fake** in tests; `--no-run`
> bypasses gates. `advance`/`start`/`done` (the meta-walk) is s007 — keep s006 to one edge. Architecture stays
> `cli → core → port ← adapter`; `core` defines `Runner`, `adapter/exec` implements it (the first driven port
> beyond storage). `history` rides `Task.History` + `TaskStore.Update` (no `HistoryStore` port — same GAP #1
> rule as `depends_on`). Everything typed — `mtt.TaskID`/`StatusName`; convert strings only at the cli/adapter
> boundary. Weigh whether `ResolvedFlow` (model.go Layer B) is worth building for a single edge (likely a
> single-edge lookup suffices; `ResolvedFlow` earns its keep in s007's multi-edge `advance`).
>
> Heed the "Carry-over lessons" below (CLI stdout via `fmt.Fprint(cmd.OutOrStdout(), …)`; anchored testscript
> asserts; `golangci unused` — declare a symbol with its first use; keep each `CLAUDE.md` current;
> provider-agnostic order, unit-test order + e2e asserts presence; zero-match `--json` = `[]` not `null`;
> derived graphs live in `core`, not the contract; one shared predicate for filters; no new port for an
> embedded field; conservative derived-read semantics; a fake for the new driven port). Don't lose the **open
> design slices** (durable edit-audit + subject-identity `By` — GAP #5, now due since `history` writes `by`;
> `cancelled`-blocker semantics from s005). Follow SOLID/DRY/KISS/TDD/DDD/clean-architecture and the AGENTS.md
> self-check.
