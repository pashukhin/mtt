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

> Caveat (for planning): commands with side effects (creating a branch) go **after** the checks; if one
> fails after a side effect, we don't commit the transition, but the side effect already happened — that's
> on the ordering of commands. A two-phase model (checks → actions) is introduced only if needed.

> **Seam (deferred): rollback / compensation.** A transition may optionally declare compensating commands
> (e.g. `rollback:` / `on_failure:`) run when an `--atomic` or multi-step `advance` aborts after side
> effects — to undo what a partially-applied transition did (delete the created branch, …). Not built now;
> the executor's abort path is the hook. Additive in config, so deferrable.

### Advancing through the flow: `advance` / `start` / `done`

`start`/`done` are not a single transition but a **meta-command**: the primitive `advance <task> --to
<status>` walks the task through a chain of transitions to a target status, running the edge gates along the
way. `start` = `--to in_progress`, `done` = `--to done` (built-in aliases). **We do not build a config
command DSL** — modes are flags.

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

- `depends_on: [id, …]`; on adding, we check for the absence of cycles.
- A task is **ready** ⇔ its status is not `terminal` AND all `depends_on` are in a `terminal` status (by
  category `kind`: `done` or `cancelled`, not by the literal). A cancelled blocker also unblocks.
- `mtt ready` — "what can be picked up for work".

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
| 3 | Flow enforcement: transition validation + running `commands` (the `Runner` port), gating on exit codes; `mtt start/done/status` | |
| 4 | Comments (tree) | |
| — | **⬆ agent-facing MVP — fully usable** | |
| 5 | `KnowledgeStore` port + YAML KB adapter; text search | |
| 6 | Text/ASCII Gantt in the CLI; richer list/query | |
| 7 | `mtt-ui` (optional, separate binary): web UI, Gantt (SVG), KB browser | |
| 8 | External adapters (subprocess protocol) + an indexer hook | |

Positioning priorities: phase 3 (flow) and **adaptivity** (external backends, phase 8) are **our wedge**.
`mtt-ui` (phase 7) is a nice optional default, not the main argument. Dependencies (phase 2) stay
**simple**; the knowledge base (phase 5) is low priority (beads already has an analog), done only if cheap.

Dogfooding: until phase 4 the plan is kept here; after that we move mtt's development onto mtt itself.

**Later (backlog):**

- later — **re-parenting** (`mtt reparent`/`move`): change a task's `parent`; enabled by flat, position-free IDs.
- later — **tags**: a cross-cutting `[]string` label on tasks (reserved in the model now); filtering lands with `list`.
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
