# MY_TT — Design

> Русская версия: [DESIGN.ru.md](DESIGN.ru.md). English is the source of truth; keep versions in sync.

Source of truth for the architecture. Changes only together with a behavior change.
High-level "why" — in [README.md](README.md).

**Vision:** mtt is an agent-friendly, lightweight "tasks + knowledge" pairing (like Jira+Confluence,
without the bulk). The concrete storage is **abstracted behind ports**: the default is YAML files, but
an adapter can plug in a user's own "tracker + knowledge base" pairing (Jira+Confluence,
GitHub Issues+Wiki, beads, …). The domain knows nothing about storage.

**The human UI is optional too.** Agents don't need it — their interface is the CLI. For humans
(view diagrams, CRUD/CQRS) there's a default utility `mtt-ui` (a small local web server); and if the
backend is a ready-made pairing (Jira+Confluence), the human uses its native UI and `mtt-ui` isn't
needed. Result: the project fits both solo devs (zero-footprint default) and corporate monsters (a thin
agent layer over the existing stack).

## Decisions

| Topic | Decision |
|---|---|
| Language / form | Go CLI `mtt` (`mtt add`, `mtt list`, …) |
| Architecture | Hexagonal: `domain+usecase` ← ports; storage is a swappable adapter |
| Ports | `TaskStore` + `KnowledgeStore` (2 independent); a "pairing" = a pair of adapters |
| Contract | Public `pkg/mtt` (domain types + ports) — for external Go adapters |
| Storage (default) | YAML adapter: **one file per task**, `.mtt/` directory |
| Source of truth | Files (in the YAML adapter); a DB/index would be derived, gitignored |
| Human UI | Optional: default `mtt-ui` (local web); with an external backend — its native UI |
| ID/slug | Minted by the **adapter** (YAML: stable flat per-prefix `e1`/`t17`/`s3`); the domain knows only the logical task |
| Flow | Executable transitions: `description` + `commands` (all → 0, else the transition is blocked) |
| Advance | `advance --to` (meta: walk to a target); modes `--stop`(default)/`--atomic`/`--force`; no config DSL |
| Roles | `start`/`done` semantics depend on the role — seam laid (`role` in history, `--role`, config `roles`); implementation deferred |
| Statuses | Category `kind` (initial/active/terminal) by flow **topology**; ≥1 of each per flow; multiple initials allowed; identity is per-flow `(type,name)`; names are config's, never code literals |
| Domain model | Pure `pkg/mtt` (no serialization tags, no `prefix`); adapters map via DTOs; DDD value objects (`StatusKind`); references by identity, back-refs computed |
| Providers | Types/flow may come from an external provider; domain requires a **mandatory minimum**, rest optional; wired later via `mtt connect` |
| Recategorization | A task's **type is immutable**; recategorize = close old + create new + link via `refs` |
| History | Append-only `history` of transitions in the task (audit + reconstruction); flow — via git |
| Capabilities | Features are optional per adapter (`Capabilities()` / `ErrUnsupported`); YAML is the reference |
| KB & refs | KB is an optional capability; `refs` (note/task/comment/url) — verifiable references, ≠ `depends_on` |
| Hosting | GitHub `github.com/pashukhin/mtt`, GitHub Actions |
| Branching | Per-task branch → PR → CI green → squash into `main` |
| Gate | `make check`: gofmt + vet + golangci-lint + `go test -race` |

## Positioning (honestly, vs beads)

**Framing.** mtt is not "another task tracker for agents" (a crowded slot) — it's a **deterministic
workflow-enforcement layer**: an *executable task state machine* whose transitions gate on your checks.
The enemy isn't Jira; it's `TODO.md`, the prompt line "run tests before done", and an agent that typed
"done" without running anything. The pitch leads with that; storage/backends and the KB are the adoption
ladder, not the headline.

[beads](https://github.com/gastownhall/beads) (Steve Yegge, ~25k★) is powerful but heavy: **Dolt**
storage (a 44–48 MB binary, cgo), optional server processes with ports, git hooks, a dual git+Dolt
history, a huge ecosystem. Its strengths — **dependencies** (typed + gates + a ready cache),
**collision-free hash IDs**, and **cell-level merge** of a single task — we **don't chase**.

Our wedge is where beads is weak or heavy:

- **Real per-type flow with executable transitions** — `commands` hang on a transition (gates/actions,
  all must return 0). beads has only a flat enum. **This is our killer feature.**
- **Zero-footprint** — files only, no daemon/ports/hooks/cgo; a tiny binary.
- **Adaptivity** — a thin agent layer over *your* backend (YAML or Jira+Confluence): a clean CLI for
  agents, native UIs for humans.
- **Optional human UI** (`mtt-ui`) — a small local web; beads has none in core.
- Readable sequential IDs and an explicit config-driven hierarchy.

Explicitly **not competing on**: distribution/federation, dependency richness, ecosystem. Our
dependencies stay **simple and "sufficient"**. The knowledge base is **low priority** (beads already
has `remember`/`prime`), done only if cheap.

> **Honest caveat:** two complaints from the original motivation are outdated — current beads **has** an
> epic/task/subtask hierarchy and an **optional** sequential-ID mode. The live reasons for a separate
> project are precisely **lightness** and **per-type flow**, not "beads lacks this".

### Competitive landscape (2026 scan)

A scan of ~20 tools (beads, Task Master, Backlog.md, git-bug, dstask, Taskwarrior, Fossil, tracker MCP
servers, unified ticketing APIs, osmove/backlog, AgentWrapper AO) confirms and sharpens the above. Others
**orchestrate** agents (GitHub Agentic Workflows, Operator, AgentWrapper AO), **integrate** them with a
tracker (Jira+Copilot, Linear automations, tracker MCP servers), or **automate** PR/issue flow — mtt does
something narrower: it **gates the task's status transition** with per-type local checks, so an agent can't
declare `done` without passing them.

- **Un-copied core (our two bets).** (1) A config-driven **per-type flow with executable command gates** —
  no tool does it: beads/Task Master have flat status enums; Backlog.md's `onStatusChange` is a
  non-blocking callback; Taskwarrior has vetoing hooks but global, not per-type/per-transition; Fossil's
  TH1 can't shell out. (2) A thin agent CLI over a **swappable existing tracker+KB as the store of
  record** — unoccupied: beads/git-bug *sync* to Jira/GitHub while keeping their own store (the inverse);
  tracker MCP servers are single-backend; unified ticketing APIs (Merge.dev, unified.to) are cloud B2B
  middleware with no CLI/agents/local backend/gates.
- **Not a differentiator (be honest).** The tasks+knowledge *bundle* is old (Fossil ships tickets+wiki+
  forum for a decade; Backlog.md bundles tasks+docs). Typed deps, ready/blocked, hierarchical IDs,
  history, and agent memory are matched by beads. A plain "file-native task CLI for agents" is a crowded,
  consolidating slot (git-bug, Backlog.md, Task Master, beads) — not a defensible position on its own.
- **Adjacent threats to watch.** `osmove/backlog` and AgentWrapper "AO" combine a swappable tracker with a
  lifecycle/gate layer for agents, but are heavier orchestrators with one-way write-back, a fixed kanban
  (not config-driven per-type gates), and no KB pairing. `Backlog.md` is the fastest-growing ideological
  neighbor. `beads` (~25k★) has the richest tracker sync; if it flips adapters to primary storage it
  contests bet 1.
- **Takeaway.** The niche is real but narrow and closing. Position as *a uniform, gated control plane over
  any tracker*, not another storage format; double down on the two bets and zero-footprint; keep the KB a
  supporting feature, not the headline.

### Why not just pre-commit hooks or CI?

The nearest objection — mtt sits a layer **above** them:

- **pre-commit** fires on a *commit*, not a task lifecycle; it isn't per-task-type; an agent bypasses it
  with `--no-verify`; and it has no notion of "this task can't be `done`".
- **CI required checks** fire after the fact, on a *PR*, remotely; they don't gate the agent's *local*
  claim of done, they need a PR/host, and they don't encode a per-type Definition of Done.
- **mtt** gates the **task-lifecycle transition** with a **per-type DoD**, in the agent's own vocabulary
  (`mtt done`), locally, over any backend. It doesn't replace CI — it can *invoke* the same checks as gates
  and add task-level ones (acceptance criteria met, docs updated, branch created, PR linked to the issue).

## Architecture: domain, ports, adapters

Hexagonal (ports & adapters). Inside — domain and usecases; outside — adapters of two kinds: **driving**
(inbound: CLI, optional `mtt-ui`) call `core`; **driven** (outbound: storage) are called from `core`
through ports.

- **`pkg/mtt`** — the public contract: domain types (`Task`, `Type`, `Flow`, `Status`, `Comment`,
  `Note`, `Config`), the base ports **`TaskStore`** and **`KnowledgeStore`**, plus optional capability
  interfaces (see below). Public so external Go adapters can implement it.
- **`internal/core`** — usecase logic (add/list/ready/flow-transition/search): works **only through
  ports**, unaware of the concrete storage.
- **`internal/adapter/yaml`** — the default *driven* adapter: implements both ports over `.mtt/`;
  **mints the ID/slug** (flat, per-prefix — e.g. `e1`/`t17`).
- **`internal/adapter/exec`** — implements the **`Runner`** port (running transition commands); replaced
  by a fake in tests. `Runner` is defined in `core` (only it needs it, not third parties).
- **`cmd/mtt` + `internal/cli`** — the thin CLI (*driving*): parse → usecase → format; on startup it
  assembles adapters from config and injects them into core.
- **`cmd/mtt-ui`** — an *optional driving adapter*: a small local web server (view/CRUD, Gantt) over the
  same `core`. Not needed by agents, and not needed with an external backend that has its own UI.

The two ports are independent: **a "pairing" = a configured pair of adapters**, and they can be mixed
(e.g. tasks from Jira, KB from local YAML). The default is YAML for both.

**External backends** plug in two ways: (1) an in-process Go adapter implementing the `pkg/mtt` ports
(that's why the contract is public); (2) — later — a subprocess adapter over a documented wire protocol
(JSON stdin/stdout) that doesn't import our Go. For now we design the **seam** and ship the YAML adapter;
the external-adapter runtime comes when needed.

> **Open question (external adapters):** which flow is authoritative — our config or the backend's native
> workflow (e.g. Jira) — and how our `commands` relate to its transitions. Moot for the YAML default;
> decided when designing a concrete external adapter.

### Adapter capabilities

Not every backend can do everything: an external tracker may not provide transition history, a comment
tree, dependencies, or search. So capabilities are **optional at the adapter level**:

- **Mandatory minimum** — the base `TaskStore` (CRUD + list/get + ID minting): every adapter implements it.
- **Optional capabilities** — separate interfaces atop the base: `HistoryStore`, `DependencyStore`,
  `CommentStore` (tree), `SearchStore` (and `KnowledgeStore` itself). An adapter implements what it can;
  `core` probes via type assertion (idiomatic Go).
- **Discoverability** — a `Capabilities()` method on the backend: the CLI/agent knows what's available
  (`mtt caps`). A missing capability yields a typed `ErrUnsupported` with a clear message, not a silent failure.
- **The YAML adapter is the reference**: it implements **all** capabilities (writes history, etc.).
  External adapters are partial; `core` is written to the minimum and "lights up" what's available.

> **Layer invariant:** `core` never imports `adapter/*`; adapters contain no business rules (only
> CRUD/queries behind a port). The rules (flow, ready, cycles) live in `core`; **ID/slug minting is in
> the adapter** (ID encoding is backend-specific).

## Why the default is files (the YAML adapter)

beads' main pain is the daemon/locks/binary DB: conflicts with plugins and painful merge conflicts in
agents' branches. File-per-task fixes this:

- no daemon and no background state — `mtt` is a stateless CLI (read → change → write);
- clean git merges: two agents on different branches add different tasks with no conflict;
- tasks are reviewable in a PR as a normal diff.

At tracker scale (hundreds–thousands of tasks) we can load everything into memory on each call; search,
the Gantt chart, and dependencies are computed in memory. SQLite isn't needed for that.

## Data layout (the YAML adapter)

```
.mtt/
  config.yaml            # project, task types, and flow (shared, committed)
  config.local.yaml      # personal overlay: connection params, local prefs (gitignored)
  tasks/
    e1.yaml              # epic 1
    t17.yaml             # task 17 (parent: e1)
    s3.yaml              # subtask 3 (parent: t17)
  knowledge/
    <slug>.md            # KB notes (markdown + YAML frontmatter)   [phase 5]
```

`.mtt/` is **committed** (it's project data). This is the YAML adapter's layout; edits go **only** through
a port (`TaskStore`/`KnowledgeStore`) — don't hand-edit files, or determinism and validation are lost.

## Configuration (layering & local overrides)

Config is resolved by merging layers (later overrides earlier), keeping shared project settings and
personal per-user settings apart:

1. built-in defaults (the `mtt init` template);
2. an optional global user config (`$XDG_CONFIG_HOME/mtt/config.yaml`) — personal cross-project defaults;
3. **`.mtt/config.yaml`** — the shared, committed project config (**types & flow live here**);
4. **`.mtt/config.local.yaml`** — a **gitignored** personal overlay for this project;
5. environment variables / CLI flags — highest.

The local overlay is for **per-user connection parameters to external backends** (e.g. different Jira
users/credentials on the same project) and local preferences. **Credentials never go in the committed
`config.yaml`** — only in the gitignored local overlay or env vars. Overriding shared **types/flow** in the
local overlay is discouraged (it desyncs the team). Deep-merge, per key/section. Seam laid now (loader +
gitignore); richer per-adapter connection schemas come with the external adapters.

## Types and hierarchy (domain) vs ID/slug (adapter)

The **domain** knows the *logical* task: its **type**, **parents**, and **flow**. A type defines:

- `name` — the type name (e.g. `epic`, `task`, `subtask`);
- `description` — optional human/agent orientation;
- `parents` — allowed parent type names (empty = root level) — **this defines the hierarchy** (a type may
  sit under several parent types); the inverse (children) is **computed**, not stored;
- `default` — marks the type used by `add` without `--type` (at most one; the full YAML provider marks
  exactly one); there is no literal `task` in code;
- `statuses` (each a **value object** with a category `kind`: initial/active/terminal) / `transitions` —
  the flow (below).

The epic → task → subtask hierarchy is **not hardcoded** — it follows from the default config:
`epic` (root) ← `task` (`parents: [epic]`) ← `subtask` (`parents: [task]`).

**Naming (ID/slug) is the adapter's job, not the domain's.** `core` creates a logical task ("a task of
type X under parent Y"), and `TaskStore` mints the concrete ID: for YAML it's flat, e.g. `t17`, for Jira
`PROJ-123`, for GitHub `#42`. So `prefix` is a **YAML-adapter** field (in its `config.yaml`), and ID
generation lives **in the adapter**, behind the port.

In the YAML adapter the ID is **flat and per-prefix**: `<prefix><N>`, where `N` is sequential per prefix
(`max+1`, `O_EXCL`). The ID does **not** encode the parent chain (`epic` #1 → `e1`; `task` #17 → `t17`;
`subtask` #3 → `s3`), so identity is decoupled from position: **re-parenting** a task changes only its
`parent` field — the ID stays stable and the file is not renamed. The ID is **stable** and independent of
text; the name lives in `title`; hierarchy lives in `parent` and is **computed** for display (e.g. `mtt show`
renders the lineage). The file name = `<id>.yaml`.

### Model invariants (checked on config load)

Purely **structural and name-agnostic** — the code has **no** literals for type/status names or ID
structure. Types/hierarchy/categories come from config (domain), ID encoding from the adapter; defaults live
in the `mtt init` template, not in logic.

- **`kind` is defined by flow topology** (and validated against the declared value): `initial` = no incoming
  (≥1 outgoing); `active` = ≥1 incoming and ≥1 outgoing; `terminal` = no outgoing (≥1 incoming). `kind` is a
  **value object** (`StatusKind`), not a name — it is the abstraction that lets names stay out of the code.
- **≥1 of each kind per flow** (`initial`/`active`/`terminal`). Minimal flow: `initial → active → terminal`
  (a 2-status flow is invalid). **Multiple `initial` statuses are allowed** (the user decides how many entry
  states they want). `ready`/completeness/`list` logic works **by category**, never by a literal name.
- **A flow is a per-type closed graph.** Status identity is `(type, name)` — same-named statuses in
  different flows are **different**; there are **no cross-flow transitions**. The whole status space is a
  **forest of disjoint per-type flows**.
- **Reopen** is a transition into a *separate `active` status*, never back into an `initial` and never out of
  a `terminal` (terminals are final) — this keeps task history linear and honest.
- **Default type** is marked by `default: true` — at most one at the domain level (`DefaultType` falls back
  to the first type); the full YAML provider must mark **exactly one**. No literal `task`.
- **Entry status.** A task starts at its type's **initial** status. When a flow has more than one `initial`,
  the entry is the one marked `default: true` on the status (mirrors the default **type** marker), else the
  first `initial` in config order (`Type.InitialStatus`). Validation: at most one `default` status per flow,
  and a `default` status must be `initial`.
- **`add` placement / `--no-parent`.** A type whose `parents` is non-empty requires a parent (`--parent`);
  as a **conscious exception**, `--no-parent` creates it at top level (a flat root ID). Root types need
  neither. (This keeps the user from ever being blocked from creating a top-level task.)
- **Hierarchy sanity:** each entry in a type's `parents` names an existing type; a type is not its own parent.
- **A task's type is immutable.** Changing type = **recategorization**: close the old task (→ a terminal),
  create a new one of the target type, and link them via `refs` (kind `task`, backlinks both ways).

> **Known limitation of the YAML adapter (a conscious trade-off):** sequential IDs collide on concurrent
> creation across branches — `e2` on two branches gives a visible git add/add conflict. Acceptable for low
> concurrency; with more parallelism — a namespace prefix per branch/agent. Other adapters (e.g. Jira) have
> their own scheme without this issue.

### Domain model vs serialization (DDD)

- **Pure domain.** `pkg/mtt` types carry **no** yaml/json tags and **no** adapter fields (`prefix` is
  adapter-only). Each adapter has its own DTOs and **maps** them to/from the domain; the contract never
  depends on a storage format.
- **References by identity.** Within a flow, transitions reference statuses by **name**; across aggregates
  (types, and later tasks) references are **names/IDs**, never pointers. **Back-references are computed** (an
  inverse index: children, backlinks), never stored — forward refs are the single source of truth.
- **Resolved graph is derived.** A linked, immutable in-memory object graph (for traversal: `ready`,
  `advance`, cycle detection) is built by `internal/core` when needed — it is **not** part of the contract.
- **Provider-agnostic.** The domain requires a **mandatory minimum** (a `Type` needs a name + a flow whose
  statuses have name+kind and transitions have from/to) and treats the rest as optional (`description`,
  `parents`, `default`, and `commands` — the last is *our* local gate augmentation, absent from external
  trackers). So types/flow can come from an **external provider** (wired later via `mtt connect`); the YAML
  adapter is the **full provider** (supplies everything).

## Task model

Fields serialize in a fixed order (struct field order) → a deterministic diff.

```yaml
id: s3
type: subtask
title: fix login redirect loop
status: in_progress
parent: t17
depends_on:
  - t2
refs:
  - {kind: note, id: auth-design, label: spec}
  - {kind: task, id: t2}
created: 2026-07-03T09:20:00Z
updated: 2026-07-03T10:00:00Z
description: |
  Multi-line description.
comments:
  - id: 1
    author: agent
    created: 2026-07-03T09:25:00Z
    body: first comment
    replies:
      - id: 2
        author: human
        created: 2026-07-03T09:40:00Z
        body: reply (tree via nested replies)
history:
  - {at: 2026-07-03T09:25:00Z, by: agent, role: implementer, from: tbd, to: in_progress}
  - {at: 2026-07-03T10:00:00Z, by: agent, role: implementer, from: in_progress, to: done,
     checks: [{cmd: "make lint", exit: 0}, {cmd: "make test", exit: 0}]}
```

- `parent` — empty for an epic.
- `depends_on` — a list of IDs (including cross-epic); a **blocking** edge (affects `ready`).
- `refs` — verifiable references to `note`/`task`/`comment`/`url` (see "Knowledge base and references");
  **informational**, non-blocking. Comments can carry `refs` too.
- `comments` — a tree via nested `replies`; a comment's `id` is sequential within the task.
- `history` — an **append-only** audit of transitions (`from→to`, `at`, `by`, `checks` results); it can't
  be reconstructed after the fact, so we write it from the start — the basis for audit and graph reconstruction.

## Listing (`list`) and editing (`edit`) — session 003

- **`list` default order is provider-agnostic.** The primary key is a **domain timestamp** — `Created` desc
  (freshest first), or `Updated` desc with `--sort updated` — never ID structure (an external adapter's IDs
  may not sort meaningfully). Ties are broken by comparing the ID as an **opaque string**, so equal
  timestamps still produce a stable, deterministic order across runs.
- **`edit` touches only title/description.** Status moves through the flow (`status`/`advance`/…) so gates
  stay enforced; re-parenting (changing `parent`) and re-typing are **separate operations**, not `edit` —
  already called out above and in the backlog.

## Hierarchy: placement and rendering — session 004

- **`add --parent <id>` is the normal placement path.** A child records only its `parent` (a forward ref by
  ID); the adapter still mints a **flat per-prefix** ID, so identity is decoupled from position. Placement is
  validated as a mutation in `core.Adder`: the parent must exist and its **type** must be allowed by the child
  type's `parents` (the pure `Type.AcceptsParent` predicate). `--no-parent` stays the escape hatch.
- **Children/ancestors are computed, never stored.** `core.Index` is a derived, in-memory view built from
  `TaskStore.List` (a pure value — no store, no clock; the "resolved graph is derived", not part of the
  `pkg/mtt` contract): parent→children, ancestor chain, roots; cycle-safe; orphans (dangling parent) surface
  as roots. Sibling order reuses the provider-agnostic `Select` ordering (`Created` desc, ID tiebreak).
- **`tree` and `show` lineage are pure reads** (no usecase): `mtt tree` renders the forest (keep-ancestors
  filtering, `--depth`, nested `--json`); `mtt show` prints the "you are here" lineage breadcrumb. A single
  `core.Match` predicate (status/type/kind/parent) is shared by `list` and the `tree` walk (DRY).

## Flow: executable transitions (the killer feature) and `mtt init`

A type defines a **flow** — a status graph with transitions. On each transition you can hang:

- `description` — text about "what exactly we're doing" (understanding for agent/human);
- `commands` — a sequence of shell commands; **all must return 0**, otherwise the transition is
  **blocked** (the task stays in the source status).

This turns the flow from advice into an **executable gate + action**. Examples:

- `in_progress → done`: `["make lint", "make test"]` — don't let it into `done` until it's green.
- `tbd → in_progress`: review the spec + create a branch for the task.

The point: **the agent works in task terms** (`mtt start t17`, `mtt done t17`), while the transition
hides the status-flow mechanics (checks, branch, …) — less distraction on details.

Execution is behind the **`Runner`** port: `core` orchestrates the transition, calls `Runner`, and gates on
exit codes. `Runner` is defined in `core`, implemented in `internal/adapter/exec`, and faked in tests.
Commands run in order, aborting on the first non-zero; the working directory is the project root; there's a
per-command timeout; the escape hatch `--no-run` forces the transition without commands (for emergencies).
Commands come from config (trusted, like a Makefile/git hooks), not from the network.

> **Shipped (s006, `mtt status <id> <new>`):** a **single** gated edge. `Runner.Run(commands)` takes no
> `dir` — the `exec` adapter is constructed with `cwd = project root` (core stays free of filesystem paths).
> A non-zero exit is **data** (a `Check`), not a Go error; a blocked gate leaves the task unchanged and
> writes no `history`. The per-command timeout is config-driven (`command_timeout`, default `5m`, an
> adapter-level setting). Core sentinels `ErrBlocked`/`ErrInvalidTransition` map to exit codes `3`/`6`. A
> single-edge lookup in `Type.Transitions` suffices (no `ResolvedFlow` yet — it earns its keep in s007's
> multi-edge `advance`). The `advance`/`start`/`done` meta-walk and modes are s007. The gate prints **live
> pipeline progress** to stderr (`▶`/`✓`/`✗` + per-command timing); the commands' own output is hidden by
> default, streamed with `-v`, and/or written with `--log-file` — **except that a blocked gate echoes the
> failing command's last ~10 output lines under its `✗` line and hints `-v`/`--log-file`** (s008.97/U2), so
> the agent sees *why* it failed without re-running the whole gate. `by` resolves `--by` > `MTT_BY` >
> `config.local.yaml` `author` (the durable personal default; `role` stays flag/env only).

> **Shipped (s006.5, attribution + verb sugar):** `--why` records a durable free-text reason on the
> transition (`HistoryEntry.Why` — the only `pkg/mtt` change — rendered by `mtt show`); `--who` is a
> symmetric alias of `--by` (mutually exclusive; same `history.by`). **Verb sugar** `mtt <status> <id>` is a
> single-edge move via CLI **fallback-routing** (`root.RunE`: exactly-2-args where arg0 is not a registered
> command, arg1 is an existing task, and arg0 is a status in that task's type flow → route to the `status`
> path reusing `core.Transitioner`; a real command wins a name clash; anything else is `unknown command`,
> exit 1) — forward-compatible to `advance` later without a surface change. **Required-attribution:** a
> project-global `require: {who, why}` in the committed config (adapter `Settings.Require`, like
> `command_timeout`; `config.local` may only **tighten** it — captured before the local overlay and
> OR-combined) is validated in `core.Transitioner` **before** the gate (fail fast; `--no-run` does not bypass
> it), aggregating all missing fields into one `ErrMissingAttribution` → exit `2`. `-v`/`--log-file` became
> root-persistent; `--no-run` stays local to `mtt status` (the sugar cannot bypass the gate).

> **Shipped (s008.98, named transitions + edge-verb sugar):** a transition may carry an optional
> **`Name`** (`pkg/mtt.Transition.Name` — the only domain change, a plain open label like `Description`),
> giving a semantic verb for the **edge out of the current status** (`decline` for `review → fix`). This
> completes a **resolution triad**: move by **target status** (explicit `mtt status [<id>] <status>`, sugar
> `mtt <status> [<id>]`) and move by **edge name** (explicit `mtt do [<id>] <edge>`, sugar `mtt <edge> [<id>]`).
> The pure `Type.FindTransitionByName(from, name)` resolves an edge name to its target status, and the CLI rides
> the **existing** `runTransition(to)` — `core.Transitioner` and the gate path are **untouched**. Three new
> structural invariants (`Config.Validate`) keep this safe: an edge `Name` is **unique per source status** and
> **disjoint from status names** in the type (so the sugar's edge-first precedence never shadows a status), and
> every `(from,to)` pair is **unique per type**. **Trust boundary (unchanged):** the move path does not
> re-validate (validation runs on `add`/`types`), so route-by-`to` is correct exactly where these invariants
> hold — the same boundary the shipped `applyCurrent`/`FindTransition` already rely on. `mtt types`, the
> `next:` guidance, and `show --json`'s `next[].name` surface the verb.

> **Shipped (t5): dangerous ops mandate `--who`/`--why`.** Flow-bypassing / risk actions force attribution
> regardless of the project `require` setting (you may skip the gate, but you must sign for it). The
> **effective** required-attribution is the union `global ∨ per-edge ∨ --no-run` — tighten-only:
> - a **`--no-run`** bypass forces **both** who+why on any edge (checked in `core.Transitioner` before the
>   gate; covers `mtt status`/`mtt do`);
> - a transition can be marked **critical** with a per-edge `require: {who, why}` in the config
>   (`mtt.Transition.Require`, decoded by the YAML adapter, unioned in core);
> - a destructive **`rm --force`** forces who+why as a **pre-flight** precondition (`core.Remover`,
>   `ErrMissingAttribution` → exit 2 on single *and* bulk) and, per id, writes an audit record to
>   `.mtt/audit.log` **before** deleting — no destruction without a trail. The log is a new driven port
>   (`mtt.AuditStore`), JSONL (`{at, who, why, action, id}`), committed with `.gitattributes` `merge=union`.
>
> Roles / authorization ("who *may* do X") stay parked — t5 is attribution only. `init --force` is out
> (bootstrap, no `.mtt/` history yet).

> Caveat (for planning): commands with side effects (creating a branch) go **after** the checks; if one
> fails after a side effect, we don't commit the transition, but the side effect already happened — that's
> on the ordering of commands. A two-phase model (checks → actions) is introduced only if needed.

> **Shipped (t21): post-persist actions (the second phase).** A transition may carry a per-edge `post:` command
> list (same `Command` shape as `commands:`) that runs **after** the status is persisted — the finalization
> phase. The repo uses it to auto-commit `.mtt` on every move (`git add .mtt && git commit … -- .mtt`), so a
> move commits its own state (removing the former manual `git add .mtt` after each move and the two interim
> commits after `deliver`/`cancel`; their pre-gate `git switch main` runs first, so the post commit lands on
> main). **Two phases, different failure semantics:** `commands:` gate the **entry** (fail → no persist, the
> s008 compensation runs); `post:` finalize **after** entry (fail → the move is **kept**, `core.ErrPostAction`,
> CLI exit **5** — mtt never rolls back a persisted move for a post hiccup). `--no-run` skips **both** phases.
> Why not a global default `post`? Precedence/merge/opt-out questions we deferred (t24) — per-edge only for now.
> The `git switch` in `deliver`/`start`/`cancel` is exactly why a naive "persist → run everything → roll back"
> single phase can't work: context switches must precede persist, commits must follow it. **(c1) auto-push**
> extends this: `approve` post also `git push -u origin task/<id>` (the PR branch), `deliver` post also
> `git push origin main` — so the only manual git step left is `gh pr create` (title/body are a judgement call).

> **Shipped (s008): rollback / compensation (intra-pipeline).** A gate command may declare a `rollback:`
> compensator (a scalar or `{run, timeout}`, itself placeholder-expanded); when a **later** command in the
> same pipeline fails, the already-succeeded commands' rollbacks run in **reverse order** — undoing what the
> partial gate did (e.g. `git checkout -b task/{{.ID}}` with `rollback: git branch -D task/{{.ID}}`). The
> `Command` VO gained an additive `Rollback *Command` (a **leaf** — its own rollback must be nil). Decisions:
> **(1)** the compensator is **per-command** (`Command.Rollback`), not per-transition — it maps 1:1 to
> "reverse over the succeeded"; **(2)** `core.Transitioner` computes the plan (which succeeded — from a single
> failure index — reversed) and the exec **`Runner.Compensate`** executes it; **(3)** compensation is
> **best-effort** (run all, continue past a failed compensator) and **never masks the gate failure** — the
> outcome stays `ErrBlocked` (exit 3); **(4)** **no `history`** on a blocked+compensated transition (the task
> file is untouched — a `HistoryEntry` is a transition record, compensation is a side-effect event); **(5)**
> rollbacks are expanded **eagerly** with the forward commands (a malformed rollback template is exit 1 before
> any side effect). The failing command's own rollback is never run. The gate prints a live
> `↩ compensating (N)` phase and the block error carries a `compensated N …` summary. See sessions/008 and
> TASKS.md → e4_t10.
>
> **Still parked:** compensation across **several** edges (an `--atomic` / multi-step `advance` abort after
> side effects) — s008 is **intra-pipeline** only (one edge's pipeline); and second-level compensation
> (a compensator's compensator, rejected by `Valid()`).

> **Shipped (s007): structured commands.** `Transition.Commands` evolved from `[]string` into a `Command`
> value object `{run, timeout?}` (a pure `pkg/mtt` VO; `rollback?` is additive, deferred to s008). Two
> capabilities land: (1) **placeholder expansion** of `run` — `.ID`/`.Type`/`.From`/`.To` (e.g.
> `git checkout -b task/{{.ID}}` on `tbd → in_progress`); (2) a **per-command timeout** overriding the global
> `command_timeout` (fail fast when a fast command overruns). **Back-compat:** a bare YAML scalar ⇒ `{run: …}`
> (a custom `UnmarshalYAML` on the DTO accepts a scalar **or** a `{run, timeout}` map); both collapse to one
> `Command` at the adapter boundary — nothing above it branches on the form. Four decisions (brainstormed):
> **(1)** the per-command timeout lives in the **domain VO** (an authored property of a flow edge, inseparable
> from `run`), while the global `command_timeout` stays adapter **execution policy** (`Settings`) — the runner
> resolves per-command-else-global; **(2)** expansion happens in **`core.Transitioner`** (it holds task+edge),
> so `pkg/mtt` stays **template-agnostic** (stores the raw template, core expands before `runner.Run`) and the
> exec adapter stays dumb; **(3)** injection defense is a **structural whitelist** — `text/template` over a
> struct exposing only the four shape-safe fields, so free text (`title`) is never interpolated and a stray
> `{{.Title}}` is a template error (no shell-quoting needed; if a free-text field is ever exposed it MUST be
> quoted — a documented seam); **(4)** `Runner.Run` takes `[]Command` (Run already expanded; `Check.Cmd`
> records the expanded command for a truthful audit). An expansion error aborts the transition as a plain
> error (exit 1), distinct from a gate block (`ErrBlocked`, exit 3). See sessions/007 and TASKS.md → e4_t9.

> **Seam (deferred, think): node-level status actions.** Today executable pipelines hang only on **edges**
> (transitions — they change status and gate). But "commit intermediate work / build / run checks **while
> staying** in a status" is a node operation with no home (a self-loop transition is a hack — false history,
> broken topology). Generalize: a status may carry **named, rollback-able action pipelines**, each invoked as
> a **custom verb** `mtt <action> <id>` on the task's current status. The shared primitive — *a named
> rollback-able command pipeline* — then hangs on either an **edge** (transition) or a **node** (status), both
> served by `Runner` + rollback. This completes the "all shell orchestration lives in the flow / agent works
> purely in task terms" story. **Blocked on** structured commands + rollback (reuses the `Command` VO +
> compensation) and on the argument-resolution grammar (custom verbs collide with real commands and the
> status-sugar — e.g. `mtt check` is reserved for ref-checking); a non-transition action's audit ties to the
> edit-audit slice. **Open question: is it release-needed?** Lean *no* — for a release an agent commits WIP
> via plain `git` while `in_progress`; this is the completeness polish, not the minimum. Revisit once
> structured commands land (it is blocked on them regardless). See TASKS.md → Later.

> **Working context: the current task (shipped s006.7).** git's current-branch, for tasks — kills
> id-repetition. The **value** lives in `config.local.yaml` (`current: t17`, personal/gitignored); the
> **rule** for setting/clearing it is a **transition property** in the committed flow — the additive
> `Transition.Current` field (`set`|`clear`, a `CurrentAction` value object; name-agnostic; the default
> templates set on take-into-work / clear on →`done`, but leave →`cancelled` alone). An **omitted id**
> resolves to the current task **only for single-task direct verbs** (`status` / `mtt <status>` / `show` /
> `edit`) — never for `list`/`tree`/`dep`/`ready`/filter/stdin/bulk (resolution order: explicit id > current;
> the s008.9 filter/stdin tier slots between them later). Companion `mtt use [<id>] [--clear]` sets/shows/clears
> it without a transition. **Two design decisions (brainstormed):** (1) the pointer is a **capability port**
> (`mtt.CurrentStore`), not a CLI helper — "take into work" is a capability a real provider may own (an
> assignee), so YAML backs it via `config.local` while an external adapter maps it to its native feature or
> returns `ErrUnsupported`. Unlike the parked `DependencyStore`, a port is justified **now**: `current` is
> **non-embeddable** (personal, single-value, not task state), so even YAML needs a separate store — the GAP #1
> case that earns the port. (2) The **CLI applies** set/clear after a successful transition (reading the edge's
> `Current` via the shared `Type.FindTransition` primitive); `core.Transitioner` is untouched — interpreting a
> declared edge effect is mechanical dispatch, not policy, so it lives at the composition root. **Caveat:** a
> shared checkout with multiple agents has one `config.local` = one `current` → collision; per-agent current
> ties to the subagent-identity question (fine for solo / one-agent-per-checkout). **Backlog (think):** separate
> `mtt current`/`use` commands for ergonomic clarity; show the status/transition `description` on a move (an
> in-flow reminder for the agent); resolve current for **all** single-task ops incl. reads (`tree <id>`,
> `dep list`); multi-assignee providers ("my current" when several are assigned); the `advance` reuse seam
> (extract a shared core apply-edge-effects step when the parked multi-edge walk unparks). `CapCurrent` +
> `Capabilities()` land with `mtt caps` (e4_t6). See TASKS.md → e4_t8a.

### Advancing through the flow: `advance` / `start` / `done`

> **PARKED (2026-07-05, on-demand).** `advance` and the verbs `start`/`done`/`cancel`, the modes
> (`--stop`/`--atomic`/`--force`), and **roles-on-edges** are **deferred** — most status transitions are
> exactly **one edge**, so single-edge `mtt status` is the norm and a multi-edge walk solves a problem we
> don't have yet; role-tagged edges are near-RBAC and premature (identity/role may later come from an
> external provider). This subsection is the **design intent** for when a flow actually branches; nothing
> here is built until then. The ergonomic win is delivered earlier and cheaply by the `mtt <status> <id>`
> **verb sugar** (s006.5) — a single-edge move via fallback-routing on an unknown first arg (not dynamic
> command registration), forward-compatible: its semantics can grow single-edge → `advance` later without a
> surface change. Attribution is `--who`/`--by` (who) + `--why` (a durable free-text reason recorded in
> `history`). Design below is preserved as the target semantics.

`start`/`done` are not a single transition but a **meta-command**: the primitive `advance <task> --to
<status>` walks the task through a chain of transitions to a target status, running the edge gates along the
way. `start` = `--to <active>`, `done` = `--to <terminal>` (built-in aliases, resolved by category, not by
literal names). **We do not build a config command DSL** — modes are flags.

Traversal semantics (predictability over cleverness):

- follow only edges that **progress toward the target**; **never enter a different terminal** (no
  auto-`cancel`); cycle guard; unreachable target → error;
- at a real fork (≥2 progressing edges) — **stop, don't guess**;
- each traversed edge runs its `commands` and appends a `history` entry;
- a non-`ready` task (open `depends_on`) is not pushed into a terminal by default — we warn.

Thanks to the linear default flow (`initial → active → terminal`), the default case is unambiguous; forks
arise only if the user adds branching. (Status names come from config, never from code.)

Modes (flags):

- `--stop` (**default**) — advance until the first failed gate or fork, and report where/why;
- `--atomic` — all-or-nothing **by status** (a failed gate → don't move and don't write transitions);
  **caveat:** side effects of already-run commands are not rolled back;
- `--force` — advance unconditionally, ignoring gates (emergency; generalizes `--no-run`).

### Roles — a seam (implementation deferred)

The semantics of `start`/`done` (and transitions in general) depend on the caller's **role**: for a reviewer
agent, `done` means something different than for an implementer (e.g. the implementer's `done` moves to
`review`, the reviewer's `done` moves from `review` to a terminal). Fully, this is roles in config +
`--role`/`MTT_ROLE` + an optional role tag on transitions; the `advance` resolver is then parameterized by role.

**We don't build it now**, but we lay the seams:

- **The only non-deferrable seam is writing `role` into `history`** (next to `by`): it can't be
  reconstructed after the fact (like history itself). Reserve the field in the model right away.
- `advance` and verb→target — the resolver is **parameterized by role** (the signature takes a role); today
  it's role-agnostic (one implicit role).
- The CLI reserves `--role=` / `MTT_ROLE`; config may grow a `roles` section and a role tag on transitions —
  additively, with a forward-compatible loader.

Guardrails (to keep it from ballooning): roles are **semantic routing** (what a verb means for a role),
**not** RBAC/enforcement (agents are cooperative — we route, we don't police). Role names come from config,
never hardcoded (like types/statuses).

> **Ties to the current task (think, s006.7).** "Take into work" (`Transition.Current: set`) is a
> **role-contextual** action — `current` is a lightweight single-value form of assignee, and "mine" is
> per-actor/per-role. What shipped (one global `config.local` pointer) is a *degenerate single-role projection*;
> when roles unpark, `current` likely becomes per-actor and `Transition.Current` may become role-conditioned.
> Decide it **with** subagent-identity, not before. See TASKS.md → Later ("`current` vs roles").

> **Direction (deferred): actor profiles.** The `roles` section is expected to grow into named **profiles**
> that pair an identity with a role — `(by, role)`, e.g. `(coding-agent, implementer)` / `(Alice, reviewer)`
> — with exactly one marked `default: true`, applied when neither `--profile` nor `--by`/`--role` is given.
> Since mtt is used mostly by **coding agents** sharing the repo config with a human, the agent's profile is
> the ergonomic default and the human overrides. `mtt profile add/list/rm` manages **only** the personal
> `config.local.yaml` profiles (shared project profiles, if any, are read-only to the command). This
> **subsumes the s006 `author` seam** (`author` = the default profile's `by`) and is forward-compatible. See
> TASKS.md → Later.
>
> **Hard precondition (the real question): subagent identity.** Roles/RBAC are pointless unless we can
> distinguish subagents acting with **different** roles under multi-agent access — that is what RBAC hinges
> on. Until there is an identity mechanism (per-agent `config.local`, an env/handshake/token, or a
> provider-supplied identity), `by`/`role` are self-declared strings and profiles are just ergonomic
> defaults, not enforcement. Decide subagent identity **before** reviving roles. Meanwhile attribution is
> release-complete without any of this: `--who`/`--why` (free text) + a project-global `require: {who, why}`
> config (an execution/adapter setting, like `command_timeout`; `config.local` may only tighten) validated
> before the gate, aggregating all missing fields into one usage error (exit 2). Roles/profiles stay parked.

`mtt init` writes `.mtt/config.yaml` with example types `epic`/`task`/`subtask` and a linear flow
(`initial → active → terminal`, plus a second terminal for cancellation). Those names are the **template's**,
not the code's. **There are no commands by default** — the user hangs them for their own project.

```yaml
version: 1
project:
  name: my_tt
types:
  - name: epic
    description: A large body of work spanning multiple tasks.
    prefix: e                    # prefix is a YAML-adapter field (ID encoding), not domain
    parents: []                  # root level
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, description: "all epic tasks closed"}
      - {from: in_progress, to: cancelled}  - name: task                   # DEFAULT type (add without --type)
    description: A unit of work under an epic.
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
      - {from: in_progress, to: cancelled}  - name: subtask
    description: A small step within a task.
    prefix: s
    parents: [task]
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done}
      - {from: in_progress, to: cancelled}```

Hanging commands — by editing a transition (no code changes):

```yaml
      - from: in_progress
        to: done
        description: "quality gate"
        commands: ["make lint", "make test"]   # all → 0, else done is blocked
```

A `bug` type (`prefix: b`, `parents: [epic]`, a flow with a `review` active status) is added the same way —
config only.

`mtt init --template coding` ships example coding types — `feature`/`bugfix`/`refactor` — each with its own
gated Definition of Done (branch + lint/test; `bugfix` also requires a failing test first; `refactor`
requires no public-API diff), as a ready-made demo of the enforcement value.

## Dependencies

- `depends_on: [id, …]` is a **blocking** edge (distinct from the hierarchy `parent` and the informational
  `refs`). It rides the `Task` field and round-trips via `TaskStore.Update` — **no dedicated port** for the
  YAML reference (a `DependencyStore` capability exists only for external adapters that cannot embed the
  field). Adding an edge is rejected if it would create a **cycle** (a `core` rule), and a self-edge is
  rejected; the cycle-check builds a derived `core.DepGraph` over `depends_on` (parallel to the s004 `Index`
  over `parent`, kept separate — the two graphs have different shapes).
- A task is **ready** ⇔ its status is not `terminal` AND all `depends_on` are in a `terminal` status (by
  category `kind`: `done` or `cancelled`, not by the literal). Readiness is **conservative**: an
  unresolvable blocker (a dangling `depends_on`, or a status not in the current flow) leaves the task
  **not** ready — `ready` requires positive confirmation.
- `mtt ready` — "what can be picked up for work"; `mtt list --ready` is the shorthand companion (one shared
  `core.Ready` primitive backs both). `mtt dep list <id>` shows a task's direct blockers and its computed
  dependents, with `--tree` (transitive) and `--cycles` (project-wide, defensive).
- **Creation-time dependencies (`add --depends-on`, s008.5).** `depends_on` can be set at creation, not just
  via `dep add`: the targets are validated (each must exist) and **deduped** in `core.Adder` before `Create`.
  No cycle check is needed — the new task's ID is unminted, so it cannot be a target (a self- or back-edge is
  inexpressible). Typed end-to-end; string→`TaskID` conversion stays at the CLI boundary.

> **Deletion (`mtt rm`, s008.5).** Hard-delete is a **store operation**, so it earns a base-port method
> `TaskStore.Delete(id)` (the *D* in CRUD; not an embedded field, so the "field rides `Update`" rule does not
> cover it — the YAML reference does `os.Remove`, an archive-only external adapter returns `ErrUnsupported`).
> It is distinct from `cancel` (a terminal *status*): `rm` is for **backlog hygiene** (purge a mistaken task),
> not the normal lifecycle, and leaves **no `history`** — the git commit dropping the file is the audit.
> `core.Remover` is **conservative**: by default it **rejects** deleting a task that others reference (a child
> via `parent`, or a dependent via `depends_on`, found by reusing `Index`+`DepGraph`), so a delete never
> silently strands references; `--force` overrides, leaving them **dangling** (already tolerated: `ready` is
> conservative, `Index` surfaces orphans as roots). It is **agent-facing** — no interactive confirmation (that
> would hang an agent), and `--who`/`--why` are not mandated (nowhere to record them without a history). A
> missing id exits `4`, the first consumer of the now-**uniform** not-found taxonomy (every single-task-by-id
> path wraps `mtt.ErrNotFound`).
>
> **Caveat surfaced by `rm --force` — id reuse (pre-existing mint limitation).** The YAML adapter mints ids as
> `max+1` per prefix with no high-water mark, so deleting the **highest-numbered** task frees its id for a
> later `add` to **reuse** — which silently re-points any dangling reference (a `depends_on`/`parent` left by
> `--force`) at the new, unrelated task. `rm` is what first makes this reachable. It is latent (needs `--force`
> + deleting the max id + a subsequent create) and rooted in the mint scheme, not the delete; the proper fix
> is monotonic / never-reuse minting (see TASKS.md → Later). Meanwhile prefer `cancel` for a referenced task,
> or clear the dangling edges first.

> **Deferred design question — `cancelled` blocker semantics.** A `terminal` blocker unblocks its dependent,
> and `cancelled` is a terminal `kind`, so a task whose blockers are `done` **and** `cancelled` is formally
> ready — yet a *cancelled* (abandoned, not completed) blocker arguably means the dependent needs
> re-evaluation, not silent unblocking. **Revisited in s006** (terminals are now reachable via `mtt status`):
> the decision is to **keep** terminal-by-`kind` (cancelled unblocks) — a proper fix (a
> succeeded-vs-abandoned distinction, a hard/soft edge) needs new domain modelling that a flow-gate session
> should not smuggle in, and would risk the name-agnostic principle. s006 adds an e2e (`cancel_unblock`)
> demonstrating the current semantics with a reachable state; the deeper fix stays deferred. See TASKS.md → Later.

## Priorities and roadmap — session 008.6

- **Priority is the third ordering axis** (alongside `depends_on` and hierarchy). It is a closed, ordered
  **value object** `Priority` (`high|medium|low`) — the `StatusKind` idiom, **not** a bare string/int and
  **not** config-defined levels (YAGNI — configurable levels are a revisit-at-second-caller). Empty = **unset**,
  ordered as `medium` (the neutral default) and **not materialized on disk** (`omitempty`, so existing task
  files are byte-untouched; back-compat). It rides `Task.Priority` + `TaskStore.Update` — **no new port** (the
  GAP #1 rule, like `depends_on`) — and maps to a provider's native priority/labels later. Validity is enforced
  at the **CLI input boundary** (`--priority` rejects an unknown value); a corrupt on-disk value is **tolerated**
  (ranks as `medium`), mirroring how `status` is validated lazily against the flow. Authored via `--priority` on
  `add`/`edit` (`edit --priority ""` clears it), filtered via `list --priority` (matches the *stored* label — an
  unset task matches only when no filter is given), sorted via `list --sort priority`, and surfaced in `show` +
  `--json` (`taskJSON.priority`). The **default `list`/`ready` order is unchanged** (recency); priority is opt-in.
- **`mtt roadmap [--json]` is a derived execution-order view** — a **pure `core` read** (`Roadmap(tasks, cfg)
  []RoadmapEntry`, no store/clock, **not** in the `pkg/mtt` contract), like `Ready`/`Select`. It returns the
  **non-terminal** tasks in an execution order over **two "comes-after" axes** — `depends_on` (an explicit
  blocking edge) **and `parent`** (a parent completes only once its children do, so a non-terminal child
  precedes its non-terminal parent). **Both axes are hard constraints.** Each entry is annotated `ready` (via
  `core.Ready` — one source of truth) and `blocked_by` (the `depends_on` entries not terminal-satisfied), and a
  parent additionally lists its non-terminal children as `contains`. **Readiness stays `depends_on`-only** — the
  parent axis affects *ordering* and the `contains` annotation, not readiness (so a parent with open children
  can be `ready: true` yet ordered last; a container/epic type may itself carry a flow + artifacts, so it is
  both a task and a container and is never special-cased or filtered).
- **Priority propagates (the soft tiebreak).** Rather than sorting by a task's *own* priority, a blocker takes
  an **effective** rank = `min(own, min over everything it transitively unblocks across both axes)` — so a
  high-priority task **pulls its prerequisites forward**, ahead of independent lower-priority work ("to finish
  the important thing, do its blockers first"). The priority-guided Kahn tiebreak is `(effective rank,
  recency)`; deterministic and **cycle-safe** across both axes (memoized effective-rank DFS; a stuck node — in
  or downstream of a cycle, including a cross-axis one — is appended best-effort so the function always
  terminates and returns every node). It builds its **own** non-terminal-restricted graph (not a reuse of
  `DepGraph`, whose `Dependents` are unfiltered — GAP #6 stays unextracted) and reuses the shared
  `terminalSatisfied` predicate factored out of `Ready`. It is **not** a *time* scheduler (no dates / critical
  path) and, deliberately, **not** `list --sort priority` (which sorts by own priority; roadmap propagates).
  This is what an agent asks instead of re-deriving "what do I do next, and what's it waiting on" from raw
  tasks; it motivates dogfooding (s009).

## Flow versioning and task history

- **The flow definition is versioned by git** (in the YAML adapter): `git log .mtt/config.yaml` is the
  whole evolution of the graph. Explicit `flow-version`/migrations of existing tasks on a status rename are
  deferrable. The domain treats `status` as **data** and validates it against the current flow **lazily**
  (it flags a mismatch rather than crashing), so config drift doesn't break old tasks.
- **Transition history** is written into the task itself (`history`, append-only) — this gives audit and,
  later, **reconstruction of the observed graph** (a read-only aggregation of all tasks' histories). We
  write it in the task, not in a central log, to preserve clean per-file merges. History is an **optional
  capability** (`HistoryStore`): the YAML adapter writes it, an external backend may not support it (then
  `ErrUnsupported`), and `core` degrades gracefully.

## Knowledge base and references (`refs`)

**The KB is an optional capability** (`KnowledgeStore`, like Confluence atop Jira). Without it, knowledge
lives right in tasks and comments; the "knowledge base" is them and the links between them.

**Notes are versioned.** A `KnowledgeStore` never destroys note content: saving a note (`note edit`)
creates a **new version linked to its predecessor**, so history is preserved — same principle as task
transition `history`. This is a KB capability: the YAML reference adapter implements it (git also tracks
the files); external backends (Confluence, …) rely on their **native** versioning — that's their concern.
The domain seam is that a `Note` carries a version identity + a predecessor link. Deferred to phase 5 with the KB.

Tasks and comments carry **`refs`** — a structured list of verifiable references:

```yaml
refs:
  - {kind: note, id: auth-design, label: "spec"}
  - {kind: task, id: t2}
  - {kind: url,  id: "https://…"}
```

- `kind` ∈ `note` | `task` | `comment` | `url`. This is **not** `depends_on` (that's a blocking edge);
  `refs` is an informational, integrity-checked link.
- **Verification is capability-aware:** `task`/`comment` resolve via `TaskStore` (always); `note` only with
  `KnowledgeStore` (otherwise "cannot verify: no KB"); `url` is external, not resolved (optional HEAD check later).
- **Semantics:** on write — warn about a dangling reference (not a hard block); `mtt check` — a repo-wide
  sweep for dangling references; `mtt show` — the references and **backlinks** ("what references this"); on
  deleting a target — warn about incoming references.
- **Phases:** the `refs` field — in the model already at phase 1; resolving `task`/`comment` — phase 2;
  `note` + `mtt check`/backlinks — phase 5 (with the KB).

## Search (phase 5)

Built-in text search (substring/tokens) over tasks and the KB. No RAG. An "external indexer" — an optional
hook (external command) via config.

## Human UI — optional (`mtt-ui`, phase 7)

`mtt-ui` is a **separate optional utility** (a driving adapter), not part of the agent binary: `net/http` +
`embed.FS`, a minimal web UI, a Gantt via SVG, over the same `core`/ports. Needed on the YAML default; with
an external backend (Jira+Confluence) the human uses its native UI. Agents don't need the UI. The CLI still
has a text/ASCII Gantt. The latest phase.

## Implementation order

| Phase | Content | Status |
|---|---|---|
| 0 | Scaffold: repo, module, AGENTS/DESIGN, CLI skeleton, gate, CI | ✅ done |
| 1 | `pkg/mtt` **pure** contract (domain types + `TaskStore` port); config+types (**structural** invariants: `kind` by topology, ≥1 of each, no name literals), `mtt init`; the YAML adapter **mints IDs** flat per-prefix (`e1`/`t17`/`s3`); core usecases + `add/list/show/edit/close` | 🔄 s001–003 (`close` → phase 3; optional capability interfaces deferred, see [architecture snapshot](docs/architecture/model.go)) |
| 2 | Hierarchy (by `parents` from config); dependencies; `ready`; cycle detection | 🔄 hierarchy done (s004); dependencies/`ready`/cycles → s005 |
| 3 | Flow enforcement: transition validation + running `commands` (the `Runner` port), gating on exit codes; `mtt status`; **attribution + verb sugar**; **structured commands** (placeholders + per-command timeout) + **rollback** | 🔄 single-edge `mtt status` shipped (s006); attribution+sugar (`--why`/`--who`, `mtt <status> <id>`, required-attribution → exit 2) shipped (s006.5); structured commands (the "work in task terms" enabler) → s007; rollback → s008. **`advance`/`start`/`done`/`cancel` + modes + roles-on-edges are PARKED** (on-demand — single-edge `status` is the norm) |
| 4 | **Dogfood** (pulled ahead — s009, after flow orchestration is complete), then references (s010), comments (s011), actor profiles (s012) | |
| — | **⬆ agent-facing MVP — fully usable** | |
| 5 | `KnowledgeStore` port + YAML KB adapter; text search | |
| 6 | Text/ASCII Gantt in the CLI; richer list/query | |
| 7 | `mtt-ui` (optional, separate binary): web UI, Gantt (SVG), KB browser | |
| 8 | External adapters (subprocess protocol) + an indexer hook | |

Positioning priorities: phase 3 (flow) and **adaptivity** (external backends, phase 8) are **our wedge**.
`mtt-ui` (phase 7) is a nice optional default, not the main argument. Dependencies (phase 2) stay
**simple**; the knowledge base (phase 5) is low priority (beads already has an analog), done only if cheap.

Dogfooding: **pulled ahead** (s010, once flow orchestration — advance + structured commands — is complete),
before references/comments, since those enrich a full self-host but don't enable it. "The agent works in task
terms, with shell orchestration living in flow transitions" hinges on **command placeholders** (s008): a
transition can't create a per-task branch without them. Until then the plan is kept in this repo's docs;
after dogfood we move mtt's development onto mtt itself. See sessions/README.md → "Roadmap regrouped".

> **Shipped (s009, revised by the flow-v2 spec): dogfooding / self-host.** This repo tracks its own
> development in a committed `.mtt/` (config + tasks). **Model — one axis:** we track the **product** (a
> task = a unit of product change), not the **process** (session/phase = how *we* work — that stays in
> `sessions/*.md` + git); structure is **deps + tags + priority**, not hierarchy (**epics** are
> product-valid but deferred; the §4 self-ref gate returns with them). **TWO types.** `task` (design
> OPEN): `tbd → speccing → spec_review → spec_human_review ⇄ spec_fix → planning → plan_review →
> plan_human_review ⇄ plan_fix → implementing → impl_review ⇄ impl_fix → approved → done` (+`cancelled`);
> `chore` (design ALREADY FIXED elsewhere — a review finding, a recorded decision, docs sync): the impl
> stage + delivery tail only. Gates check **form, never content** (content = the review statuses):
> id-keyed artifact presence (`ls docs/superpowers/specs/<id>-*.md`) on spec/plan submits, `make check`
> (per-command 10m timeout) on impl submits, and a **verified delivery** — `deliver` moves the tree to
> main and greps the squash subject (`<id>: …` — the PR title, propagated by the repo's
> `squash_merge_commit_title=PR_TITLE` setting) from local `git log`, so **`done` truthfully means "in
> main"**. Branch context is mechanized: `start` re-enters or creates `task/{{.ID}}` from main (guarded:
> the task file must exist on the tree); `cancel` and `deliver` write their terminal state ON main;
> `approved → decline` returns to the task branch. Artifacts are **committed early** (id-keyed names
> `docs/superpowers/specs|plans/<id>-<slug>.md` — the v1 uncommitted-until-review convention is dead).
> **Conventions that remain:** the PR title starts with `<id>: `; opening the PR (`gh pr create`) is the
> one manual git step. (Since t21 every move auto-commits `.mtt` via a per-edge `post:` action, and since
> c1 `approve` auto-pushes the task branch and `deliver` auto-pushes main — the former manual
> state-commits and the push-before-merge convention are gone.)
> **Attribution:** project-global `require: {who}` (per-edge/role `require` needs a core change — parked
> roles work; the migrated `dangerous-ops` task is its first trigger). **Trust (SEC2):** gates invoke
> **read-only** `mtt` only — never an mtt transition (recursion). **Known limits (recorded):** the
> human's impl-stage act leaves no mtt history entry — the audit is git's merge trace (spec/plan human
> sign-offs keep theirs); a mid-flight blocked task has no parking status (it rests where it is);
> cross-branch stale reads are the documented lost-update class (multi-agent cluster). **Backlog** =
> `tbd` tasks tagged `backlog` (promotion = drop the tag). **Authoring caveat:** gate commands are
> **single-quoted** YAML scalars. The committed config is guarded by `TestRepoDogfoodConfig` (the
> **sole** load-time validation — `Config.Validate` runs on `add`/`types`, not `Load`; the guard loads a
> temp copy, bypassing the `config.local` overlay). **Bootstrap caveats:** mtt ids (`t1`…) ≠ the docs'
> historical `sNNN` labels; the branch carries no slug (placeholder whitelist); s009 itself ran on the
> manual `feat/s009-dogfood` (the config governs *future* tasks). Specs:
> `docs/superpowers/specs/2026-07-09-session-009-dogfood-design.md` (model, migration) +
> `docs/superpowers/specs/2026-07-11-flow-v2-mechanized-delivery-design.md` (the flow, authoritative).

**Later (backlog):**

- later — **re-parenting** (`mtt reparent`/`move`): change a task's `parent`; enabled by flat, position-free IDs.
- **tags — shipped s008.7** (backlog management): a cross-cutting label set on `Task.Tags` (rides the field +
  `Update`, **no new port** — GAP #1, like `depends_on`). Authored two ways: **`#hashtags` in title/description
  are the primary path** (extracted + merged into the canonical set on `add`/`edit`, the text left intact), and
  explicit `--tag` / `mtt tag add/rm` are secondary/pointed. Reconciliation is **write-time** — a text-delta on
  `edit` drops a tag when its `#hashtag` leaves the text and keeps manual tags (no provenance stored, so a
  text+manual collision drops with the text); `tag rm` is **guarded** (refuses a tag whose `#hashtag` is still
  in the text — edit the text instead). `list/tree --tag` filters (a slice-valued OR-within `ListFilter`
  dimension over `Match`). The tag vocabulary is a pure `pkg/mtt` pair (`NormalizeTag`/`ExtractTags`) over a
  **Unicode** charset (`\pL\pN._-`, Unicode `ToLower`, no NFC folding); tags are a normalized+sorted **set**
  (open vocabulary → plain `[]string`, not a VO). Spec:
  `docs/superpowers/specs/2026-07-09-session-008.7-tags-design.md`.
- **batch & pipeline — shipped s008.9** (mtt as a Unix-composable CLI): a reusable **task-set selector**
  shared by every set-operating command — explicit IDs | a `--filter` (the `list` predicates
  `--status/--type/--kind/--parent/--priority/--tag/--ready` over `Select`/`Match`) | **stdin `-`** (IDs one per
  line), **mutually exclusive** (>1 or 0 active source = usage error; a present-but-empty source = no-op, exit
  0) — plus an **`--ids`** output on `list`/`ready` (mutually exclusive with `--json`), so pipelines compose:
  `mtt list --tag x --ids | mtt tag rm x -`. Applied first to `tag add/rm` and `rm` (no gates). The selector is
  a **CLI concern** (stdin/flag I/O; the `--filter` branch reuses `core.Select`/`Ready`) — no core surface;
  it **never** resolves the `current` pointer. Bulk mutations are **best-effort per item** with a report
  (successes on stdout, per-item failures on stderr / a `--json` per-item array) and a **generic exit 1** on any
  failure (git-style; the aggregate never wraps a per-item sentinel, so it can't mis-map to exit 3/4). A
  **`--dry-run`** previews the affected ids without mutating. Argument layout is **context-sensitive** for `tag`:
  a selector marker (a `-` or a filter flag) makes the positionals TAGS and the tasks come from the selector
  (`mtt tag add urgent --status tbd`); without a marker the single `mtt tag add <id> <tag>…` form is unchanged.
  **Bulk `rm` is subgraph-aware** (`core.Remover.RemoveMany`): a referenced-check counts only referents outside
  the deletion set, so `mtt rm <epic> <child>` deletes the subtree in one call without `--force` (a single
  `rm <id>` keeps its exit-4 not-found and single-store reject). Bulk `status`/verbs/`edit`/`dep` stay **later**
  (gates + partial-success + atomicity are trickier). Spec:
  `docs/superpowers/specs/2026-07-09-session-008.9-batch-design.md`.
- later — **boards / views**: a query/view over tags/status/type (relates to `list` and `mtt-ui`); the backlog is such a view.
- later — **durable, git-independent audit of edits**: `edit` today only bumps `updated`, with git as the
  de facto history; a change-log or field versioning (additive, non-breaking) would make edit history
  queryable without git. Pairs with the **subject-identity (`By`) source** — who is "acting" for
  attribution, likely a `.mtt/config.local.yaml` field, distinct from `--role` (`--role` is *what hat* they
  wear, `By` is *who*). Both deferred; the `history` field stays **transition-only** (phase 3) — it does not
  grow to cover plain `edit`s.
- later — **`list` filter-value validation**: today `list --status X`/`--type Y` filters against tasks
  only, so an unknown value yields an empty result — indistinguishable from "valid status, no tasks yet".
  Validate the filter values against the config (a status must exist in *some* flow, a type must be a
  configured type) and error on an unknown value, so "no tasks in this status" and "this status can't exist
  in the flow" are distinct. Needs `list` to load config (it currently reads only the store).
- later — **`mtt edit <id> --editor`**: open the task in `$EDITOR` (edit the fields interactively / via the
  rendered file) instead of passing `--title`/`--description` on the command line — a human-friendly
  alternative to the flag-driven edit.
- later — **`show` multi-line description indent**: `mtt show` prints the description via `"\n  %s\n"`, so a
  multi-line description gets the 2-space indent on the **first line only**; continuation lines render
  flush-left. Data is correct (round-trips as a YAML block scalar) — this is a display nit in `formatTask`
  (from session 002). Fix: indent every line of the description. Cheap, cosmetic, non-blocking.

## Code layout

```
cmd/mtt/main.go            # entry point of the agent CLI (driving)
cmd/mtt-ui/main.go         # OPTIONAL web UI (driving adapter) over core — a separate binary
pkg/mtt/                   # PUBLIC contract: domain types + ports (TaskStore, KnowledgeStore)
internal/
  cli/                     # cobra commands (thin) + wiring adapters from config
  core/                    # usecase logic: hierarchy, dependencies, cycles, flow — only through ports
  adapter/
    yaml/                  # the default driven adapter: ports over .mtt/ files
docs/
  architecture/model.go    # code-form domain-model snapshot (target contract; not compiled into the binary)
```

> **Domain-model snapshot.** [docs/architecture/model.go](docs/architecture/model.go) is a code-form,
> tiered (T1/T2/T3) index of the intended contract surface — domain types + ports + optional capabilities,
> core usecases with their dependencies, the derived resolved graph, and the open gaps. It is a design
> reference (compiles, lint-clean, browsable via `go doc`), NOT imported by the binary; this prose stays
> authoritative and the snapshot is its structural map. Two layers: **A** (contract/persisted aggregates) —
> identity by typed value (`TaskID`/`TypeName`/`StatusName`), the serialization/provider boundary; **B**
> (core-derived resolved graph) — pointer links for traversal, never serialized.

## Releasing

Packaging + release process (cross-platform binaries via `make release`, a tag-triggered GitHub release
workflow, `SHA256SUMS`) lives in [RELEASING.md](RELEASING.md). Pre-1.0 versions mirror the session.
