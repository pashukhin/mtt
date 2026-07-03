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
| ID/slug | Minted by the **adapter** (YAML: stable `e1_t3_s2`); the domain knows only the logical task |
| Flow | Executable transitions: `description` + `commands` (all → 0, else the transition is blocked) |
| Advance | `advance --to` (meta: walk to a target); modes `--stop`(default)/`--atomic`/`--force`; no config DSL |
| Roles | `start`/`done` semantics depend on the role — seam laid (`role` in history, `--role`, config `roles`); implementation deferred |
| Statuses | Category `kind` (initial/active/terminal); terminals `done` + `cancelled` |
| History | Append-only `history` of transitions in the task (audit + reconstruction); flow — via git |
| Capabilities | Features are optional per adapter (`Capabilities()` / `ErrUnsupported`); YAML is the reference |
| KB & refs | KB is an optional capability; `refs` (note/task/comment/url) — verifiable references, ≠ `depends_on` |
| Hosting | GitHub `github.com/pashukhin/mtt`, GitHub Actions |
| Branching | Per-task branch → PR → CI green → squash into `main` |
| Gate | `make check`: gofmt + vet + golangci-lint + `go test -race` |

## Positioning (honestly, vs beads)

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
  **mints the ID/slug** (`e1_t3_s2`).
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
    e1_t3.yaml           # task 3 of epic 1
    e1_t3_s2.yaml        # subtask 2 of task e1_t3
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

The **domain** knows the *logical* task: its **type**, **parent**, and **flow**. A type defines:

- `name` — the type name (e.g. `epic`, `task`, `subtask`);
- `parent` — the parent type's name (empty = root level) — **this defines the hierarchy**;
- `statuses` (each with a category `kind`: initial/active/terminal) / `transitions` — the flow (below).

The epic → task → subtask hierarchy is **not hardcoded** — it follows from the default config:
`epic` (root) ← `task` (parent: epic) ← `subtask` (parent: task).

**Naming (ID/slug) is the adapter's job, not the domain's.** `core` creates a logical task ("a task of
type X under parent Y"), and `TaskStore` mints the concrete ID: for YAML it's `e1_t3_s2`, for Jira
`PROJ-123`, for GitHub `#42`. So `prefix` is a **YAML-adapter** field (in its `config.yaml`), and ID
generation lives **in the adapter**, behind the port.

In the YAML adapter the ID is built by walking the parent chain: `<prefix><N>` at each level, joined with
`_` (`epic` #1 → `e1`; `task` #3 → `e1_t3`; `subtask` #2 → `e1_t3_s2`). The number `N` is sequential per
prefix within the parent (`max+1`), and the file is created atomically (`O_EXCL`). The ID is **stable**
and independent of text; the name lives in `title`. The file name = `<id>.yaml`.

### Model invariants (checked on config load)

- Any set of types has a **default `task`** (used by `mtt add` without `--type`).
- Every status has a **category** `kind`: `initial` / `active` / `terminal`. `ready`/completeness/`list`
  logic works **by category**, not by the literal `done`.
- Every flow contains the canonical anchors **`tbd` (initial) → `in_progress` (active) → `done`
  (terminal)** in that order (intermediate ones allowed). There may be several terminals (the default also
  has `cancelled`); exactly one `initial`.
- The code has **no** literals for type/status names or ID structure: types/hierarchy/categories come from
  config (domain), ID encoding from the adapter. Defaults live in the `mtt init` template, not in logic.

> **Known limitation of the YAML adapter (a conscious trade-off):** sequential IDs collide on concurrent
> creation across branches — `e2` on two branches gives a visible git add/add conflict. Acceptable for low
> concurrency; with more parallelism — a namespace prefix per branch/agent. Other adapters (e.g. Jira) have
> their own scheme without this issue.

## Task model

Fields serialize in a fixed order (struct field order) → a deterministic diff.

```yaml
id: e1_t3_s2
type: subtask
title: fix login redirect loop
status: in_progress
parent: e1_t3
depends_on:
  - e1_t2
refs:
  - {kind: note, id: auth-design, label: spec}
  - {kind: task, id: e1_t2}
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

## Flow: executable transitions (the killer feature) and `mtt init`

A type defines a **flow** — a status graph with transitions. On each transition you can hang:

- `description` — text about "what exactly we're doing" (understanding for agent/human);
- `commands` — a sequence of shell commands; **all must return 0**, otherwise the transition is
  **blocked** (the task stays in the source status).

This turns the flow from advice into an **executable gate + action**. Examples:

- `in_progress → done`: `["make lint", "make test"]` — don't let it into `done` until it's green.
- `tbd → in_progress`: review the spec + create a branch for the task.

The point: **the agent works in task terms** (`mtt start e1_t3`, `mtt done e1_t3`), while the transition
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

Thanks to the linear canonical `tbd → in_progress → done`, the default case is unambiguous; forks arise only
if the user adds branching.

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

`mtt init` writes `.mtt/config.yaml` with the types `epic`/`task`/`subtask` and the flow
`tbd → in_progress → done` (+ the terminal `cancelled`). **There are no commands by default** — the user
hangs them for their own project.

```yaml
version: 1
project:
  name: my_tt
types:
  - name: epic
    prefix: e                    # prefix is a YAML-adapter field (ID encoding), not domain
    parent: ""                   # root level
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, description: "all epic tasks closed"}
      - {from: in_progress, to: cancelled}
      - {from: in_progress, to: tbd}
  - name: task                   # DEFAULT type (add without --type)
    prefix: t
    parent: epic
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, description: "review the spec, create a branch"}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, description: "quality gate"}
      - {from: in_progress, to: cancelled}
      - {from: in_progress, to: tbd}
  - name: subtask
    prefix: s
    parent: task
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done}
      - {from: in_progress, to: cancelled}
      - {from: in_progress, to: tbd}
```

Hanging commands — by editing a transition (no code changes):

```yaml
      - from: in_progress
        to: done
        description: "quality gate"
        commands: ["make lint", "make test"]   # all → 0, else done is blocked
```

A `bug` type (`prefix: b`, `parent: epic`, a flow with `review`) is added the same way — config only.

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

Tasks and comments carry **`refs`** — a structured list of verifiable references:

```yaml
refs:
  - {kind: note, id: auth-design, label: "spec"}
  - {kind: task, id: e1_t2}
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
| 1 | `pkg/mtt` contract (domain types + `TaskStore` port); config+types (invariants: default `task`, `tbd→in_progress→done`), `mtt init`; the YAML adapter **mints IDs** `e1_t3_s2`; core usecases + `add/list/show/edit/close` | |
| 2 | Hierarchy (by `parent` from config); dependencies; `ready`; cycle detection | |
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
```
