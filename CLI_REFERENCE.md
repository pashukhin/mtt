# mtt — CLI Reference

> Русская версия: [CLI_REFERENCE.ru.md](CLI_REFERENCE.ru.md). English is the source of truth.

The complete **target** command surface of the `mtt` CLI, derived from [DESIGN.md](DESIGN.md). It serves
two purposes: a reference for humans and agents, and a way to sanity-check the design from the CLI angle
(man/usage) rather than from requirements.

**Status:** this is the design surface. Only `mtt version` exists today (phase 0). Each command is tagged
with the phase that introduces it (see the plan in [DESIGN.md](DESIGN.md#implementation-order)).

**Notation:** `<required>`, `[optional]`, `…` repeatable. `<id>` is a task ID such as `t17` — flat,
per-prefix (in the YAML adapter). `<status>` is a status name from the type's flow (e.g. `tbd`,
`in_progress`, `done`, `cancelled`).

---

## Synopsis

```
mtt [global flags] <command> [arguments] [flags]
```

`mtt` is a stateless CLI: it reads `.mtt/` (via the configured adapter), applies a change, writes it back.
Run `mtt help [command]` or `mtt <command> -h` for built-in help.

---

## Global flags (unified semantics, available on every command)

| Flag | Env | Meaning |
|---|---|---|
| `--json` | — | Emit machine-readable JSON instead of human text. On a mutation, prints the resulting object; on a query, prints the result set. Off by default. Intended for agents. **Implemented (session 003)** on `show`/`list`/`edit`. |
| `--dir <path>` | `MTT_DIR` | Project root that holds `.mtt/`. Default: the nearest ancestor of the current directory that contains `.mtt/`. **Implemented (session 003)**: `--dir`/`MTT_DIR` is an explicit root (must itself contain `.mtt/`, no upward walk); omitted, falls back to ancestor discovery. |
| `--role <role>` | `MTT_ROLE` | The acting role (e.g. `implementer`, `reviewer`). Recorded into a task's transition `history`. A reserved seam — it does not change routing yet (see DESIGN → Roles). *(pending — lands with flow enforcement, phase 3)* |
| `-q, --quiet` | — | Suppress non-essential output (still prints errors and requested data). *(pending)* |
| `--no-color` | `NO_COLOR` | Disable ANSI color in human output. *(pending)* |
| `-h, --help` | — | Help for the command. |
| `--version` | — | Print the version and exit (same as `mtt version`). **Implemented (session 003)**. Unlike the other flags in this table, this is root-only (cobra's `root.Version`): `mtt --version` works, `mtt <subcommand> --version` does not. |

## Transition flags (shared by status-changing commands: `status`, `advance`, `start`, `done`)

| Flag | Meaning |
|---|---|
| `--no-run` | Do not execute the transition's `commands` (bypass gates/actions). Emergency/debug. |
| `--stop` | **(default, advance-family)** Advance until the first failed gate or ambiguous fork; report where and why it stopped. |
| `--atomic` | All-or-nothing **by status**: if any gate fails, don't change status and don't write transitions. Note: side effects of already-run commands are not rolled back (a rollback/compensation seam is planned — see DESIGN). |
| `--force` | Advance/transition unconditionally, ignoring gates (generalizes `--no-run` and also overrides a single-edge gate on `status`). |

`--stop`, `--atomic`, and `--force` are mutually exclusive.

## Configuration

mtt merges config layers, later overriding earlier: built-in defaults → optional global user config
(`$XDG_CONFIG_HOME/mtt/config.yaml`) → committed `.mtt/config.yaml` (shared **types & flow**) → gitignored
`.mtt/config.local.yaml` (personal connection params & local prefs) → env / CLI flags. Put credentials for
external backends in the local overlay or env vars, **never** in the committed config. See
[DESIGN.md](DESIGN.md) → Configuration.

---

## Project & meta

### `mtt init` — initialize a project  *(phase 1)*
Creates `.mtt/` with a default `config.yaml` (types `epic`/`task`/`subtask`, flow `tbd → in_progress →
done` plus the terminal `cancelled`, no commands) and the `tasks/` (and later `knowledge/`) directories. A
personal, gitignored `.mtt/config.local.yaml` may override it (connection params, local prefs — see
Configuration).

- `--force` — overwrite an existing `config.yaml`.
- `--name <name>` — project name written into the config (default: directory name).
- `--template <name>` — starter config: `default` (epic/task/subtask, no commands) or `coding`
  (feature/bugfix/refactor, each with a gated per-type Definition of Done). Default: `default`.

### `mtt version` — print the version  *(phase 0, implemented)*
Prints the build version. No arguments.

### `mtt types` — show configured types and their flows  *(phase 3)*
Lists each task type: its `parent`, statuses (with their `kind`), and transitions (with `description` and
whether `commands` are attached).

- `[<type>]` — show only this type.

### `mtt caps` — show the current backend's capabilities  *(phase 3)*
Prints which capabilities the active adapter supports (history, dependencies, comment tree, search,
knowledge base). Lets an agent avoid relying on a feature the backend lacks.

### `mtt completion <shell>` — shell completion script  *(cobra built-in)*
Generates a completion script for `bash`/`zsh`/`fish`/`powershell`.

---

## Tasks (CRUD)

### `mtt add [title] [flags]` — create a task  *(phase 1, `add`/`show` shipped in session 002)*
Create a task. Provide a `title` (positional) and/or `--description`; at least one is required. The
adapter mints the ID — a flat, per-prefix ID such as `e1` or `t17` — and prints `created <id>`.

- `--type <name>` — task type from config (default: the type marked `default`).
- `--parent <id>` — place the task under an existing parent (session 004). Validated: the parent exists and
  its **type** is allowed by the child type's `parents`. Mutually exclusive with `--no-parent`. *(implemented)*
- `--no-parent` — create a parent-requiring type at top level (a conscious exception). *(implemented)*
- `--description <text>` — the task description (stdin via `--description -` planned).
- `--depends-on <id>…` and `--ref <kind>:<target>…` (e.g. `note:auth-design`, `task:t2`) arrive in a later session.

A non-root type given neither `--parent` nor `--no-parent` errors and tells you how to proceed. A missing
parent, or a parent whose type the child may not sit under, errors with guidance.

### `mtt show <id> [flags]` — show a task  *(phase 1, implemented; lineage in session 004)*
Shows a task: id, type, status, title, the **lineage** breadcrumb (root-first parent chain, e.g.
`lineage:  e1 › t1`, omitted for a root task), timestamps, and description. Dependencies, references and
**backlinks**, the comment tree, and the transition `history` (audit trail) print once those land in
later phases.

- `<id>` — the task to show.
- `--no-history` — *(later)* omit the history/audit trail.
- `--no-comments` — *(later)* omit comments.

### `mtt list [flags]` — list tasks  *(phase 1, `--status`/`--type`/`--sort`/`--json` shipped in session 003)*
Prints tasks in a stable order. Filters combine with AND.

- `--status <status>…` — filter by status name. *(implemented)*
- `--kind <initial|active|terminal>…` — filter by status category (session 004). *(implemented)*
- `--type <type>…` — filter by task type. *(implemented)*
- `--parent <id>` — only direct children of this task (session 004). *(implemented)*
- `--ready` — only tasks that are ready (no open blockers) — shorthand for `mtt ready`. *(later)*
- `--sort <created|updated>` — ordering key; default `created`, both descending, tie-broken by ID.
  *(implemented)*

### `mtt edit <id> [flags]` — edit non-flow fields  *(phase 1, implemented in session 003)*
Changes title and/or description. **Status is not editable here** — status changes go through `status` /
`advance` so the flow is enforced. Re-parenting/re-typing are not simple edits (they would re-mint the ID
in the YAML adapter — see Notes).

- `--title <text>` — new title.
- `--description <text>` — new description (`-` for stdin still later).

### `mtt tree [<id>] [flags]` — show the hierarchy  *(session 004, implemented)*
Prints the epic → task → subtask tree as an ASCII tree (`├─`/`└─`/`│` connectors; each node is
`<id>  <type>  [<status>]  <title>`). Without `<id>` it renders the forest from all roots; with `<id>` it
roots the tree at that task. Children are **computed** (an inverse index in `core`, not stored); sibling
order is deterministic (`Created` desc, tie-broken by ID). An orphan (a task whose parent id is absent) is
surfaced as a root, never dropped.

- `--status <status>…` / `--kind <initial|active|terminal>…` — filter displayed nodes. Filtering uses
  **keep-ancestors** semantics: a node shows if it matches or any descendant matches, and non-matching
  ancestors are kept as the path to a match (so a matching leaf is never lost under a non-matching parent).
- `--depth <n>` — limit visible levels, like `tree -L n` (`--depth 1` = roots only; `0`/unset = unlimited).
- `--json` — emit a **nested** tree (`{…task fields…, "children": [ … ]}`); the top level is always a JSON
  array (`[]` when empty, never `null`); leaf `children` are omitted.

---

## Flow (status changes)

### `mtt status <id> <status> [flags]` — single transition  *(phase 3)*
Moves the task across **one** edge to `<status>`, validating it against the type's `transitions` and
running that edge's `commands` (gate). Fails if the transition isn't allowed or a gate returns non-zero.
Accepts the transition flags (`--no-run`, `--force`).

### `mtt advance <id> --to <status> [flags]` — walk to a target status  *(phase 3)*
Meta-command: walks the task through a chain of transitions to `--to <status>`, running edge gates along
the way. Follows only progressing edges, never enters a different terminal, stops at a real fork, guards
against cycles, and errors if the target is unreachable. Accepts all transition flags (default `--stop`).

- `--to <status>` — the target status (required).

### `mtt start <id> [flags]` — alias: advance to `in_progress`  *(phase 3)*
Equivalent to `mtt advance <id> --to in_progress`. Accepts the transition flags.

### `mtt done <id> [flags]` — alias: advance to `done`  *(phase 3)*
Equivalent to `mtt advance <id> --to done`. Runs the `→ done` gate (e.g. lint/test). By default warns if
the task is not `ready` (open dependencies).

### `mtt cancel <id> [reason] [flags]` — move to the `cancelled` terminal  *(phase 3)*
Transitions the task to `cancelled` (a terminal that unblocks its dependents). `[reason]` is recorded in
the history. Does not run the `done` gate.

### `mtt ready [flags]` — list actionable tasks  *(phase 2)*
Lists non-terminal tasks whose blockers are all in a terminal status (`done`/`cancelled`) — "what can be
picked up next". Accepts the `list` filters.

---

## Dependencies  *(phase 2; capability `DependencyStore`)*

### `mtt dep add <id> <depends-on-id>` — add a blocking dependency
Makes `<id>` depend on `<depends-on-id>`. Rejected if it would create a cycle.

### `mtt dep rm <id> <depends-on-id>` — remove a dependency

### `mtt dep list <id>` — list a task's dependencies and dependents
- `--tree` — show the transitive dependency tree.
- `--cycles` — report dependency cycles in the project.

---

## References  *(field: phase 1; commands: phase 2; `note` targets need a KB, phase 5)*

References are informational, verifiable links (`kind` ∈ `note`/`task`/`comment`/`url`) — not blocking
dependencies. A reference is identified by its natural key — the **pair `(kind, target)`** (no separate
reference ID). The target is part of the key, so an entity can hold many references of the same `kind` to
different targets (`note:auth-design` + `note:login-spec` are two distinct references); only an exact
`kind`+`target` duplicate is collapsed (its `--label` updated). `--label` is an annotation, not part of identity.

### `mtt ref add <id> <kind>:<target> [--label <text>]` — add a reference
Adds a reference from task `<id>` to `<kind>:<target>` (e.g. `note:auth-design`, `task:t2`). Idempotent:
re-adding the same key updates its `--label`. On success prints the stored reference; if the target can't
be resolved (a `note` with no KB, a missing task) it is still stored but flagged **unverified/dangling**
with a warning (not a hard error). With `--json`, echoes the reference object `{kind, id, label, status}`.

### `mtt ref rm <id> <kind>:<target>` — remove a reference
Removes the reference with that key from task `<id>`. Exits `4` if no such reference exists.

### `mtt ref list <id>` — list references and backlinks
Prints the task's outgoing references (each: `kind:target`, label, and resolution status
`ok`/`unverified`/`dangling`) and its incoming **backlinks** — the tasks/comments that reference this one.

### `mtt check [flags]` — verify references  *(phase 5)*
Sweeps the repository for dangling references (targets that don't exist / can't be resolved). Capability-
aware: `note` refs are only checkable with a knowledge base.

- `--fix` — interactively drop dangling references (optional).

---

## Comments  *(phase 4; capability `CommentStore`)*

### `mtt comment add <id> <body> [--reply <cid>]` — add a comment
Appends a comment to the task; `--reply <cid>` nests it under an existing comment (tree).

- `--ref <kind>:<target>…` — attach references to the comment.

### `mtt comment list <id>` — print the comment tree
(Also shown by `mtt show`.)

---

## Knowledge base  *(phase 5; capability `KnowledgeStore`)*

Absent a KB backend, these return `ErrUnsupported` and knowledge lives in tasks/comments instead.
**Notes are versioned** — writes never destroy prior content; `edit` saves a new version linked to the
previous (see DESIGN → Knowledge base). External backends use their native versioning.

### `mtt note add <slug> [flags]` — create a knowledge note
Creates a note at `<slug>` (its first version). Rejects an existing slug — use `edit` to add a version.
- `<slug>` — stable identifier / filename.
- `--title <text>` — human title.
- `--body <text>` — content (`-` for stdin).

### `mtt note edit <slug> [flags]` — save a new version
Saves a new version of the note's title/body, **linked to the previous version**; old versions are kept.

### `mtt note show <slug> [--version <n>]` — print a note (with backlinks)
Shows the current version, or version `<n>` with `--version`.

### `mtt note history <slug>` — list a note's versions
Lists versions (newest first) with author/time; each links to its predecessor.

### `mtt note list` — list notes

### `mtt search <query> [flags]` — text search  *(phase 5)*
Simple substring/token search over tasks and notes (no RAG).

- `--tasks` / `--notes` — restrict the scope.

---

## Views

### `mtt gantt [<id>] [flags]` — text/ASCII Gantt  *(phase 6)*
Renders a text/ASCII Gantt of the project (or the subtree at `<id>`).

- `--from` / `--to <date>` — time window.

---

## Separate binary: `mtt-ui`  *(phase 7)*

An **optional** driving adapter (a small local web server) over the same core — not part of the agent
binary. Not needed with an external backend that has its own UI.

```
mtt-ui [--addr <host:port>] [--dir <path>]
```
- `--addr <host:port>` — listen address (default `127.0.0.1:8080`).
- `--dir <path>` — project root (as `--dir`/`MTT_DIR` above).

---

## Exit codes (proposed)

Distinct codes let agents branch on the outcome without parsing text.

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Generic error |
| `2` | Usage error (bad flags/arguments) |
| `3` | Transition blocked — a gate command returned non-zero |
| `4` | Not found (task/note/target does not exist) |
| `5` | Unsupported — the active adapter lacks the required capability (`ErrUnsupported`) |
| `6` | Invalid transition — not allowed by the type's flow |

This richer taxonomy is still **proposed**: session 003 keeps a single generic failure code (`1`) for every
error path; codes `2`–`6` land alongside the behaviors they distinguish (usage validation, flow, capability
gates, …).

---

## Environment variables

| Var | Meaning |
|---|---|
| `MTT_DIR` | Project root containing `.mtt/` (same as `--dir`). |
| `MTT_ROLE` | Acting role recorded in history (same as `--role`). |
| `NO_COLOR` | Disable colored output. |

---

## Notes / observations (from the CLI-angle review)

These are things this reference surfaces that are worth keeping consistent with the design:

- **Clean split: `edit` vs flow commands.** `edit` only touches non-flow fields (title/description); all
  status movement goes through `status`/`advance`/`start`/`done`/`cancel` so the flow is always enforced.
- **`done` and `cancel` replace a generic `close`.** Closing a task = reaching a terminal: `done` (with
  its gate) or `cancel`. There is no separate `close` command. *(TASKS.md still mentions `close` in
  phase 1 — reconcile: fold it into `done`/`cancel`.)*
- **Re-parenting changes only `parent`; re-typing is still not `edit`.** IDs are flat and per-prefix (not
  parent-chain-encoded), so **re-parenting** (a planned `mtt reparent`/`move`) only changes the `parent`
  field — the ID stays stable, no re-mint, no broken inbound refs. **Re-typing** is bigger (the prefix is
  tied to the type): it stays out of scope for `edit` — see recategorization in DESIGN.md.
- **Capability-gated commands.** `dep*`, `comment*`, `note*`, `search`, and history rely on optional
  adapter capabilities; against a backend that lacks them they exit `5` (`ErrUnsupported`), not silently.
- **`--json` everywhere.** Every command supports JSON output so agents can drive mtt without parsing
  human text; mutations echo the resulting object.
- **`--role` is recorded, not enforced.** It writes into history now (the non-deferrable seam); role-based
  routing of `start`/`done` is deferred.
