# mtt — CLI Reference

> Русская версия: [CLI_REFERENCE.ru.md](CLI_REFERENCE.ru.md). English is the source of truth.

The complete **target** command surface of the `mtt` CLI, derived from [DESIGN.md](DESIGN.md). It serves
two purposes: a reference for humans and agents, and a way to sanity-check the design from the CLI angle
(man/usage) rather than from requirements.

**Status:** this is the target command surface. **Implemented today (through session 008.98, `0.8.98-dev`):**
`version`, `init`, `types`, `add`, `show`, `list`, `edit`, `tree`, `dep add/rm/list`, `ready`, `roadmap`,
`status` (plus the `mtt <status> <id>` verb sugar), `use`, `rm`, `tag add/rm` — with a **task-set selector**
(explicit ids | stdin `-` | `--filter`) + an **`--ids`** output on `list`/`ready` powering **bulk** `tag add/rm`
and `rm` (s008.9) — and cobra's built-in `completion`/`help`. Everything
else below is design surface, each tagged with the phase/session that introduces it (see the plan in
[DESIGN.md](DESIGN.md#implementation-order)). The `advance`/`start`/`done`/`cancel` meta-walk is **PARKED**
(single-edge `status` is the norm — see the note in "Flow" below).

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
| `--role <role>` | `MTT_ROLE` | The acting role (e.g. `implementer`, `reviewer`). Recorded into a task's transition `history`. A reserved seam — it does not change routing yet (see DESIGN → Roles). **Implemented (session 006)** — recorded, not enforced. |
| `--by <subject>` | `MTT_BY` | The acting subject ("who"), recorded into transition `history`. Distinct from `--role` ("what hat"). Falls back to `MTT_BY`, then the `config.local.yaml` `author` (the durable personal default). **Implemented (session 006)**. |
| `--who <subject>` | `MTT_BY` | Symmetric alias of `--by` (reads as a pair with `--why`). **Mutually exclusive** with `--by` (set only one). **Implemented (session 006.5)**. |
| `--why <text>` | — | A durable free-text reason for the transition, recorded into `history` and rendered by `mtt show`. **Implemented (session 006.5)**. |
| `-v, --verbose` | — | Stream a gate command's own output to stderr (only meaningful on a gated transition). **Implemented (session 006; root-persistent since 006.5)**. |
| `--log-file <path>` | — | Write a gate command's own output to a file. **Implemented (session 006; root-persistent since 006.5)**. |
| `-q, --quiet` | — | Suppress non-essential output (still prints errors and requested data). *(pending)* |
| `--no-color` | `NO_COLOR` | Disable ANSI color in human output. *(pending)* |
| `-h, --help` | — | Help for the command. |
| `--version` | — | Print the version and exit (same as `mtt version`). **Implemented (session 003)**. Unlike the other flags in this table, this is root-only (cobra's `root.Version`): `mtt --version` works, `mtt <subcommand> --version` does not. |

## Transition flags (shared by status-changing commands: `status`, `advance`, `start`, `done`)

| Flag | Meaning |
|---|---|
| `--no-run` | Do not execute the transition's `commands` (bypass gates/actions). Emergency/debug. **Forces `--who`+`--why` (t5)** — you may skip the gate, but you must sign for it (missing → exit 2). |
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

**`command_timeout`** (top-level, e.g. `command_timeout: 5m`) bounds each transition gate command (per
command). It is an execution/adapter setting (kept out of the pure domain), defaults to `5m` when absent,
and is overridable via `config.local.yaml`. A command may override it with its own **per-command timeout**
(see "Transition commands" below); the global is the fallback.

**Transition commands (structured — session 007; `rollback` — session 008).** A transition's `commands` is a
list where each entry is either a **bare string** (the command) or a **map** `{run, timeout, rollback}`:

```yaml
transitions:
  - from: tbd
    to: in_progress
    commands:
      - run: git checkout -b task/{{.ID}}
        rollback: git branch -D task/{{.ID}}  # undo THIS command if a later one fails
      - make test                             # a later gate; if it fails, the branch is removed
  - from: in_progress
    to: done
    commands:
      - make lint                      # bare string — uses the global command_timeout
      - {run: make test, timeout: 30s} # per-command timeout overrides the global
```

- **Placeholders** in `run` are expanded before the gate runs: `{{.ID}}`, `{{.Type}}`, `{{.From}}` (the
  status being left), `{{.To}}` (the target). Only these shape-safe fields are available — free text
  (title/description) is never interpolated; a stray `{{.Title}}` is an error. The expanded command is what
  runs and what the transition `history` records.
- **`timeout`** (a Go duration like `30s`, `2m`) bounds that command, overriding `command_timeout` for it.
- **`rollback`** (session 008) is a **compensator** for that command — itself a scalar or `{run, timeout}`
  (same placeholders). **Intra-pipeline compensation:** when a **later** command in the same pipeline fails,
  the already-succeeded commands' rollbacks run in **reverse order** (undo the branch a first command created,
  …). It is **best-effort** (all compensators run, continuing past a failed one) and **never changes the
  outcome** — the transition is still **blocked** (exit `3`), the task stays put, and **no history** is
  written (the task file is untouched). A failing command's own rollback is **not** run. The gate prints a
  live `↩ compensating (N)` phase and the block message appends `compensated N commands`. (Cross-edge /
  `--atomic` compensation across several transitions is not built yet.)
- `mtt types` prints a command as `$ <run>` (`(timeout <d>)` when set) and, on the next line, `↩ <rollback>`
  when the command declares a compensator.

**`author`** (top-level, typically in the gitignored `config.local.yaml`) is the durable default for the
history `by` field — "who is acting" — used when neither `--by` nor `MTT_BY` is set (precedence
`--by` > `MTT_BY` > `author`). Personal, so it belongs in the local overlay, not the committed config.

**`require`** (top-level, in the **committed** config, e.g. `require: {who: true, why: true}`) makes
`--who`/`--why` mandatory on a status change — validated **before** the gate runs and not bypassed by
`--no-run`; `config.local` may only **tighten** it (a committed requirement cannot be relaxed locally). A
violation aggregates all missing fields into one usage error (exit `2`). **Implemented (session 006.5)**.

**`require` per transition** (under a `transitions[]` entry, e.g. `require: {who: true, why: true}`) marks that
edge **critical**: it forces `--who`/`--why` for that move only, unioned with the global policy (tighten-only).
**Implemented (t5).**

**Dangerous ops force `--who`/`--why` (t5).** Independent of `require`, a **gate bypass** (`--no-run`) forces
**both** who+why on any edge, and a destructive **`mtt rm --force`** forces who+why as a pre-flight check
(exit `2` when missing, on both single and bulk). `rm --force` also appends an audit record to `.mtt/audit.log`
(JSONL `{at, who, why, action, id}`, committed with `.gitattributes` `merge=union`) **before** deleting.

**`post:` per transition (t21).** A transition may carry a `post:` command list (same shape as `commands:`)
that runs **after** the status is persisted — the finalization phase. This repo uses it to auto-commit `.mtt`
on every move (`git add .mtt && git commit -m "{{.ID}}: {{.From}} → {{.To}}" -- .mtt`); the `deliver`/`cancel`
edges commit on `main`, so they **narrow** that add-pathspec to the task file (+`.mtt/audit.log` when present)
— a dirty `.mtt/config.yaml` must not ride the main-landing commit past review (c3). And (c1) to **auto-push**:
`approve` also runs `git push -u origin task/{{.ID}}` (the task branch), then opens/updates the PR (`gh pr
create`, idempotent — needs `gh`+`jq`; body from `docs/superpowers/pr/{{.ID}}.md` when present, else generated;
t27); `deliver`/`cancel` run `git push origin main` (a delivered or cancelled terminal state must not live only
locally — c5). Failure semantics
differ from the gate: a `post:` failure **keeps** the move (status already written) and exits **5** (`commands:`
gate a failure → exit 3, status unchanged). `--no-run` skips **both** `commands:` and `post:`. Shown in
`mtt types` as a `⇢` line under the edge.

---

## Project & meta

### `mtt init` — initialize a project  *(phase 1, implemented in session 001)*
Creates `.mtt/` with a default `config.yaml` (types `task`/`chore`, flow `tbd → in_progress →
done` plus the terminal `cancelled`, no commands) and the `tasks/` (and later `knowledge/`) directories. A
personal, gitignored `.mtt/config.local.yaml` may override it (connection params, local prefs — see
Configuration).

- `--force` — overwrite an existing `config.yaml`.
- `--name <name>` — project name written into the config (default: directory name).
- `--template <name>` — starter config: `default` (flat `task`+`chore`, no commands), `coding`
  (feature/bugfix/refactor, each with a gated per-type Definition of Done), or `hierarchy`
  (epic/task/subtask, no commands). Default: `default`.
- `--json` — emit the created-config summary `{path, template, name, created}` (absolute `path`) instead of
  the human line. *(t45)*

Running any other command outside a project (no `.mtt/` found by discovery, or a `--dir` without one) errors
with a `run 'mtt init' to create one` hint (session 008.97/U4).

### `mtt version` — print the version  *(phase 0, implemented)*
Prints the build version. No arguments. `--json` emits `{"version": …}` — as does the root `--version` flag
(`mtt --version --json`), so the flag and the command agree. *(t45)*

### `mtt types` — show configured types and their flows  *(implemented in session 001; flow/command detail grew through s006–s008)*
Lists each task type: its `parent`, statuses (with their `kind`), and transitions (with `description` and
whether `commands` are attached).

- `[<type>]` — show only this type.
- `--json` — emit the flow graph: an array of `{name, prefix, parents, default?, description?, statuses:[{name,
  kind, default?, description?}], transitions:[{name?, from, to, description?, current?, require?, commands:[{run,
  timeout?, rollback?}], post:[…]}]}` (structural arrays are non-null; `mtt types <type> --json` → a one-element
  array; an unknown type → exit 1). *(t45)*

### `mtt caps` — show the current backend's capabilities  *(phase 3 — not yet implemented; design surface)*
Prints which capabilities the active adapter supports (history, dependencies, comment tree, search,
knowledge base). Lets an agent avoid relying on a feature the backend lacks.

### `mtt completion <shell>` — shell completion script  *(cobra built-in)*
Generates a completion script for `bash`/`zsh`/`fish`/`powershell`.

---

## Tasks (CRUD)

### `mtt add [title] [flags]` — create a task  *(phase 1, `add`/`show` shipped in session 002)*
Create a task. Provide a `title` (positional) and/or `--description`; at least one is required. The
adapter mints the ID — a flat, per-prefix ID such as `e1` or `t17` — and prints `created <id>` (or, with
`--json`, the created task object — session 008.97).

- `--type <name>` — task type from config (default: the type marked `default`).
- `--parent <id>` — place the task under an existing parent (session 004). Validated: the parent exists and
  its **type** is allowed by the child type's `parents`. Mutually exclusive with `--no-parent`. *(implemented)*
- `--no-parent` — create a parent-requiring type at top level (a conscious exception). *(implemented)*
- `--description <text>` — the task description (stdin via `--description -` planned).
- `--priority <high|medium|low>` — the task's scheduling priority (session 008.6). Default: **unset** (orders
  as `medium`, written nothing on disk). An unknown value is a usage error. **Implemented (session 008.6)**.
- `--depends-on <id>…` — set blocking dependencies at creation (repeatable, comma-separated). Each target
  must exist (else the add errors and nothing is created); validated in `core.Adder`. **Implemented (session
  008.5)**.
- `--ref <kind>:<target>` — attach an informational reference at creation (repeatable; `note:auth-design`/
  `task:t2`/`url:https://…`). Warn-not-block: a dangling target is stored with a warning (exit 0). **t1.**
- `--tag <tag>…` — add a tag (repeatable, session 008.7). `#hashtags` in the title/description are also
  extracted and merged into the same set. Values are normalized (Unicode-lowercased over letters/digits plus
  `. _ -`, any script; an optional leading `#` is allowed); an out-of-charset value is a usage error.
  **Implemented (session 008.7)**.

A non-root type given neither `--parent` nor `--no-parent` errors and tells you how to proceed. A missing
parent, or a parent whose type the child may not sit under, errors with guidance.

### `mtt show [<id>] [flags]` — show a task  *(phase 1, implemented; lineage in session 004; omitted id → current in 006.7; priority in 008.6)*
Shows a task: id, type, status, title, **priority** (a `priority:` line, shown only when set — session 008.6),
**tags** (a `tags:` line — the sorted set, shown only when non-empty — session 008.7),
the **flow guidance** for its current status (session 008.95 — the status's `description` as a `▸ …` line when
set, and the **onward moves** `next: <status> (<transition description>) · …` when the status is non-terminal),
the **lineage** breadcrumb, a **children** summary, timestamps, and
description. The lineage is a "you are here" path from the root **down to and including the task**
(`lineage:  e1 › t1 › s1`), shown only when the task has a parent; a root task shows none. The children line
lists direct children (`children: 2 (t1, t2)`), shown only when present. There is no separate `parent:` line
— the parent is the breadcrumb's second-to-last element. Dependencies, references and **backlinks**, the
comment tree, and the transition `history` (audit trail) print once those land in later phases.

- `<id>` — the task to show.
- `--json` — the task object plus **`status_description`** (omitempty), **`next`** (an array of
  `{name?, to, description?}` onward moves, omitempty — `name` is the edge verb for `mtt do`, session 008.98),
  and **`history`** (an array of
  `{at, by?, role?, why?, from, to, checks?:[{cmd, exit}]}` transition entries, omitempty — session 008.97/U3,
  so the JSON consumer sees the checks + attribution the human view renders). The structured, queryable form of
  the flow guidance + audit. The shared `list`/`edit` `--json` view is unaffected (these fields are `show`-only).
- `--no-history` — *(later)* omit the history/audit trail.
- `--no-comments` — *(later)* omit comments.

### `mtt list [flags]` — list tasks  *(phase 1, `--status`/`--type`/`--sort`/`--json` shipped in session 003)*
Prints tasks in a stable order. Filters combine with AND.

- `--status <status>…` — filter by status name. *(implemented)*
- `--kind <initial|active|terminal>…` — filter by status category (session 004). *(implemented)*
- `--type <type>…` — filter by task type. *(implemented)*
- `--parent <id>` — only direct children of this task (session 004). *(implemented)*
- `--priority <high|medium|low>…` — filter by priority (session 008.6, repeatable). Matches the **stored**
  label: an unset task matches only when no `--priority` is given. *(implemented)*
- `--tag <tag>…` — filter by tag (session 008.7, repeatable; on `list`, `ready`, and `tree`). **OR within** the
  dimension (a task matches if it carries **any** given tag), AND across the other filters. Values are
  normalized like `--tag` on `add`. *(implemented)*
- `--exclude-tag <tag>…` — **negative** filter (c8, repeatable, on `list`, `ready`, **and** `tree` — `tree`
  added in c10): reject any task carrying **any** of these tags. Composes with `--tag` as AND, so on overlap
  exclude wins. E.g. `mtt ready --exclude-tag backlog` de-noises the queue. *(implemented)*
- `--ready` — only tasks that are ready (no open blockers) — shorthand for `mtt ready`. *(implemented, session 005)*
- `--sort <created|updated|priority>` — ordering key; default `created`. `created`/`updated` are descending,
  tie-broken by ID; `priority` (session 008.6) orders high→low (unset in the medium band), tie-broken by
  recency. *(implemented)*
- `--ids` — print only task ids, one per line, for pipelines (session 008.9). Honours the filters and
  `--sort`; **mutually exclusive with `--json`**. E.g. `mtt list --tag x --ids | mtt tag rm x -`. *(implemented)*

### `mtt edit [<id>] [flags]` — edit non-flow fields  *(phase 1, implemented in session 003; omitted id → current in 006.7; priority in 008.6)*
Changes title, description, and/or priority. **Status is not editable here** — status changes go through `status` /
`advance` so the flow is enforced. Re-parenting/re-typing are not simple edits (they would re-mint the ID
in the YAML adapter — see Notes).

- `--title <text>` — new title.
- `--description <text>` — new description (`-` for stdin still later).
- `--priority <high|medium|low>` — new priority (session 008.6). `--priority ""` **clears** it back to unset.
  An unknown value is a usage error. *(implemented)*
- **Tags reconcile on a text edit** (session 008.7): editing `--title`/`--description` re-derives the
  `#hashtags` — a tag whose `#hashtag` left the text is dropped, a newly-typed one is added, and manual tags
  (from `mtt tag add`) survive. There is no `--tag` on `edit`; surgical tag changes go through `mtt tag add/rm`.

### `mtt rm [<id>...] [-] [--force]` — delete tasks (hard delete)  *(session 008.5; bulk + selector in 008.9)*
Permanently removes tasks (distinct from `cancel`, which is a terminal *status*, not removal). `rm` is for
backlog hygiene — purging a mistaken or obsolete task. A task has **no history** for a delete (the file is
gone); the git commit that drops `.mtt/tasks/<id>.yaml`, plus the `.mtt/audit.log` record written by
`--force` (see below), are the audit trail.

**`--force` forces attribution (t5).** Because it destroys a task despite inbound references, `mtt rm --force`
requires **both** `--who` and `--why` (missing → **exit 2**, nothing deleted — a pre-flight check on single
*and* bulk) and appends `{at, who, why, action: "rm --force", id}` to `.mtt/audit.log` **before** each delete.
A plain `rm` (no `--force`) is unchanged (reject-if-referenced; no attribution, no audit).

**Task-set selector (session 008.9).** `rm` takes a set from **one** of three mutually-exclusive sources:
explicit ids (`mtt rm t1 t2`), stdin `-` (ids one per line — `mtt list … --ids | mtt rm -`), or a `--filter`
(`--status/--type/--kind/--parent/--priority/--tag/--ready` — `mtt rm --status cancelled`). Giving more than
one source, or none, is a usage error; a source that matches nothing is a no-op (exit 0). A **single explicit
id** keeps the exact single-task behaviour (below); a multi-id / `-` / `--filter` delete is **bulk**:
best-effort per task, a `removed N task(s): …` summary on stdout, per-task failures on stderr, and **exit 1**
if any failed. `--dry-run` previews the affected ids (one per line) without deleting. Bulk `rm` is
**subgraph-aware** — the reject-if-referenced check ignores referents that are **themselves in the deletion
set**, so `mtt rm <epic> <child>…` removes a whole subtree in one call **without** `--force`.

- Requires an **explicit `<id>`** for the single form — `rm` does **not** resolve the current-task pointer (a
  destructive op takes an explicit target). If a deleted task was the current pointer, it is cleared.
- By default `rm` is **rejected** if the task is **referenced** — another task `depends_on` it, or it has
  children (`parent` points at it) — listing the referencing ids. This keeps a delete from silently stranding
  references (exit `1`).
- `--force` — delete anyway, leaving the references **dangling** (which the system tolerates: `ready` is
  conservative — a dangling blocker leaves the dependent not ready — and `tree` surfaces orphans as roots).
  **Caveat (id reuse):** the YAML adapter mints ids as `max+1` per prefix, so deleting the *highest-numbered*
  task frees its id — a later `add` may **reuse** it, silently re-pointing any dangling reference at the new
  (unrelated) task. Prefer `cancel` over `rm --force` for a referenced task, or remove the dangling edges
  first. A monotonic/never-reuse minting fix is tracked in TASKS.md → Later.
- A missing `<id>` exits `4` (not found). On success prints `removed <id>`; with `--json` the single form
  emits the **removed task object** (captured before deletion, mirroring `add --json`), while bulk `--json`
  stays the per-item outcome array. *(t45)*

### `mtt tree [<id>] [flags]` — show the hierarchy  *(session 004, implemented)*
Prints the task hierarchy (parent → child) as an ASCII tree (`├─`/`└─`/`│` connectors; each node is
`<id>  <type>  [<status>]  <title>`). Without `<id>` it renders the forest from all roots; with `<id>` it
roots the tree at that task. Children are **computed** (an inverse index in `core`, not stored); sibling
order is deterministic (`Created` desc, tie-broken by ID). An orphan (a task whose parent id is absent) is
surfaced as a root, never dropped.

- `--status <status>…` / `--kind <initial|active|terminal>…` / `--tag <tag>…` / `--exclude-tag <tag>…` — filter
  displayed nodes (`--tag` session 008.7, OR-within; `--exclude-tag` c10, negative). Filtering uses
  **keep-ancestors** semantics: a node shows if it
  matches or any descendant matches, and non-matching ancestors are kept as the path to a match (so a matching
  leaf is never lost under a non-matching parent).
- `--depth <n>` — limit visible levels, like `tree -L n` (`--depth 1` = roots only; `0`/unset = unlimited).
- `--json` — emit a **nested** tree (`{…task fields…, "children": [ … ]}`); the top level is always a JSON
  array (`[]` when empty, never `null`); leaf `children` are omitted.

### `mtt tag add|rm <id> <tag>... | <tag>... (- | --filter)` — manage tags  *(session 008.7; bulk in 008.9)*
Tags are cross-cutting labels. The **primary** way to tag is a `#hashtag` in the title/description (extracted
on `add`/`edit`); `mtt tag add/rm` is the secondary, pointed path. Both take **one or more** tags (variadic),
so a whole set changes in one write. Tags are stored as a normalized, deduplicated, **sorted** set and ride
`Task.Tags` (no new port — like `depends_on`).

**Bulk over a task set (session 008.9).** The argument layout is **context-sensitive**: with a selector
marker — a `-` (ids from stdin) or a `--filter` flag
(`--status/--type/--kind/--parent/--priority/--tag/--ready`) — the **positionals are the tags** and they are
applied to every selected task (`mtt tag add urgent --status tbd`, `mtt list --tag x --ids | mtt tag rm x -`).
Without a marker it is the **single** form `mtt tag add <id> <tag>…` (unchanged). Bulk is best-effort per task
(a `tagged/untagged N task(s): …` summary on stdout, per-task failures — e.g. the `rm` guard — on stderr, exit
1 if any failed); `--dry-run` previews the affected ids. On `tag`, the `--tag` **filter** flag selects tasks
carrying a tag — distinct from the positional tags being added/removed.

- `mtt tag add <id> <tag>...` — add tags (idempotent: re-adding an existing tag writes nothing). Prints
  `tagged <id>: <tags>`, or the task object with `--json`.
- `mtt tag rm <id> <tag>...` — remove tags. **Guarded:** a tag whose `#hashtag` is still in the title or
  description is **refused** (`cannot remove tag "x": #x is present in the title …`) — edit the text to remove
  it (the guard is faithful to "the text is authoritative", and has no bypass). The guard is checked for
  **all** targets before any change, so a multi-tag call is atomic; removing an absent tag is a no-op. Prints
  `untagged <id>: <tags>`, or the task object with `--json`.
- A missing `<id>` exits `4` (not found); the `rm` guard is a plain error (exit `1`).
- Tag values are normalized: Unicode-lowercased over letters/digits plus `. _ -` (any script — `#Бэкенд` →
  `бэкенд`), with an optional leading `#`. Whitespace or other characters are a usage error. (Comparison is by
  lowercased code points; there is **no** Unicode NFC folding.)

### `mtt tags [flags]` — tag vocabulary with counts  *(c9, implemented)*
Read-only: tallies tags across the counted tasks and prints `count  tag` rows, most-used first (count
descending, then tag ascending). Default scope is **open** tasks (`initial`+`active` kinds) — the bare
command shows the live vocabulary. A **status-scoping** flag sets the scope explicitly and drops the open
default: `--all` counts every task; an explicit `--status`/`--kind` scopes to those (so `mtt tags --status
done` reaches terminal tasks). The other `list` filters (`--type`/`--priority`/`--tag`/`--exclude-tag`/
`--parent`) narrow **within** the scope. `--json` emits a `[{tag, count}]` array. Replaces the
`mtt list … | jq … | uniq -c` workaround.

---

## Flow (status changes)

### `mtt status [<id>] <status> [flags]` — single transition  *(session 006, implemented; omitted id in 006.7)*
Moves the task across **one** edge to `<status>`, validating it against the type's `transitions` and
running that edge's `commands` (gate: all exit `0`, else the move is **blocked** — exit `3` — and the task
is left unchanged, no history). On success it appends a `history` entry (`from→to`, `at`, `by`/`role`/`why`
from `--who`/`--by`/`--role`/`--why`, `checks`) and prints `t1: tbd → in_progress` (plus a line per check),
or the task object with `--json`. A transition not in the flow exits `6`. If the project's `require` policy
is unmet, it exits `2` **before** running the gate (see Configuration → `require`).

The gate reports **live pipeline progress** to stderr (`▶ <cmd>` / `✓|✗ <cmd> (exit N, <elapsed>)`) as each
command runs; the commands' own output is hidden by default. On a **block**, the failing command's last ~10
output lines are echoed under its `✗` line and the error carries a `hint: re-run with -v or --log-file …`
(session 008.97) — so the agent sees *why* the gate failed without re-running it; use `-v`/`--log-file` for
the full output.

After a successful move, mtt prints the flow's **guidance** on stdout under the `from → to` line (session
008.95): the traversed edge's `description` and the destination status's `description` (each as a `▸ …` line,
shown only when set) and the **onward moves** (`next: <status> (<transition description>) · …`; omitted at a
terminal status). This surfaces the flow's authored text as **inline instructions for the agent** — what the
move is for, what to do now, and where it can go next. A **blocked** move prints no guidance (the task did not
enter the status). With `--json` the move output stays the task object; query the structured guidance via
`mtt show --json` (below).

- `--no-run` — skip the edge's `commands` (bypass the gate). Local to `mtt status` (the sugar cannot bypass
  the gate); does **not** bypass required-attribution. *(implemented)*
- `-v`, `--verbose` / `--log-file <path>` — gate-output control (root-persistent global flags). *(implemented)*
- `--force` — *(not yet — lands with the advance family, s007)*

#### Verb sugar: `mtt <status> <id>`  *(session 006.5, implemented)*
A shorthand for a single-edge move: `mtt in_progress t1` ≡ `mtt status t1 in_progress` (note the **reversed**
argument order — `<status> <id>`). It is resolved by **fallback-routing**, not a registered command: with
exactly two arguments where the first is not a real subcommand, an existing task `<id>`, and `<status>` is a
status in that task's type flow, mtt routes to the `status` path (reusing all its validation, gates, exit
codes, and `--who`/`--why`). A real command always wins a name clash (e.g. there is no sugar that shadows
`list`). If `<status>` is a plausible status verb but `<id>` does not exist, it is a **not-found** error
(exit `4`, consistent with the explicit `mtt status` form); anything else that does not classify as a status
move is an `unknown command` (exit `1`). The sugar takes
no gate-control flags (`--no-run`/`-v`/`--log-file` remain on `mtt status`); it is forward-compatible — its
semantics can grow single-edge → `advance` later without a surface change.

### `mtt do [<id>] <edge>` — move along a named edge  *(session 008.98, implemented)*
Move a task across the **named edge** leaving its current status — the explicit form for working *in terms of
the current status* (a semantic verb like `decline`/`approve`), symmetric to `mtt status` for moves by target
status. A transition may carry an optional **`name:`** in the config; `mtt do <edge>` finds the edge with that
name out of the task's current status and moves to its target, running the same gate/attribution/`--json` as
`mtt status` (it rides the same path). The id is optional (resolves to the current task). Edge-name-**only** —
no status fallback; an unknown edge from the current status exits `6` (invalid transition) and **lists the
available named actions**. `--no-run` bypasses the gate (like `status`). `mtt types` lists each edge's name.

- **Verb sugar `mtt <edge> [<id>]`** (session 008.98) — the shorthand for `mtt do`, mirroring `mtt <status>`:
  `mtt decline t1` ≡ `mtt do t1 decline`. Resolved by the same fallback-routing; an edge name out of the task's
  current status is tried **before** a target-status name (safe — see the namespace rules below).
- **Namespace rules (enforced by `Config.Validate`, i.e. on `mtt add`/`mtt types`):** an edge `name` is unique
  per source status, is **disjoint from status names** in the type (so the sugar never shadows a status), and
  every `(from,to)` pair is unique per type.

### `mtt use [<id>] [--clear]` — the current task (working context)  *(session 006.7, implemented)*
git-`HEAD`-for-tasks: a personal **current task** pointer (in `config.local.yaml`, gitignored) so you stop
repeating the id.
- `mtt use <id>` — set the current task (the id must exist). Prints `current: <id>` (or the task with `--json`).
- `mtt use` — show the current task as one line (or `no current task`).
- `mtt use --clear` — clear the pointer (prints `current cleared`).

With `--json`, every form emits the current task object, or `null` when there is none — so `mtt use --clear
--json` and `mtt use --json` with no current both print `null`. *(t45)*

**Omitted-id resolution:** when you leave off the id on a **single-task direct verb** — `mtt status <status>`,
the sugar `mtt <status>` (e.g. `mtt done`), `mtt show`, `mtt edit` — mtt uses the current task. Order is
**explicit id > current**; a stale or unset current gives an actionable error. It is **never** applied to
`list`/`tree`/`dep`/`ready` (set/filter operations). So a full loop reads: `mtt use t1` → `mtt in_progress` →
… → `mtt done` (no id repeated).

**Moving the pointer via the flow:** a committed transition can carry `current: set` (take-into-work) or
`current: clear` (release); mtt applies it after the move. The default/`coding` templates `set` on
`→ in_progress` and `clear` on `→ done` (leaving `→ cancelled` alone), so `mtt in_progress t1` makes `t1`
current and `mtt done` clears it. (Storing the pointer is a capability: the YAML adapter writes `config.local`;
an external adapter may map it to a native assignee.)

> **PARKED (on-demand) — the `advance` family is NOT implemented.** Most transitions are a single edge, so
> `mtt status` and the `mtt <status> <id>` sugar are the norm; the multi-edge walk (and the `--stop`/
> `--atomic`/`--force` modes) surfaces only when a flow actually branches. The four sections below are the
> **target semantics** for that point. Note `mtt done t1` works *today* as the single-edge verb sugar (move to
> a `done` status in one edge), not as this `advance` meta-walk. See DESIGN.md → "Advancing through the flow".

### `mtt advance <id> --to <status> [flags]` — walk to a target status  *(phase 3, PARKED)*
Meta-command: walks the task through a chain of transitions to `--to <status>`, running edge gates along
the way. Follows only progressing edges, never enters a different terminal, stops at a real fork, guards
against cycles, and errors if the target is unreachable. Accepts all transition flags (default `--stop`).

- `--to <status>` — the target status (required).

### `mtt start <id> [flags]` — alias: advance to `in_progress`  *(phase 3, PARKED)*
Equivalent to `mtt advance <id> --to in_progress`. Accepts the transition flags.

### `mtt done <id> [flags]` — alias: advance to `done`  *(phase 3, PARKED — but `mtt done <id>` works today as verb sugar)*
Equivalent to `mtt advance <id> --to done`. Runs the `→ done` gate (e.g. lint/test). By default warns if
the task is not `ready` (open dependencies).

### `mtt cancel <id> [reason] [flags]` — move to the `cancelled` terminal  *(phase 3, PARKED — `mtt cancelled <id>` works today as verb sugar)*
Transitions the task to `cancelled` (a terminal that unblocks its dependents). `[reason]` is recorded in
the history. Does not run the `done` gate.

### `mtt ready [flags]` — list actionable tasks  *(session 005, implemented)*
Lists non-terminal tasks whose blockers are all in a terminal status (`done`/`cancelled`) — "what can be
picked up next". Accepts the `list` filters (`--status`/`--type`/`--kind`/`--parent`/`--tag`/`--exclude-tag`),
`--json`, and `--ids` (session 008.9; one id per line, mutually exclusive with `--json`). `--tag`/`--exclude-tag
<tag>…` (repeatable; `--exclude-tag` from c8, `--tag` on `ready` added in c10) include/de-noise the queue, e.g.
`mtt ready --tag urgent`, `mtt ready --exclude-tag backlog`.
Readiness is **conservative**: a dangling blocker or a status not in the current flow leaves a task not
ready (`mtt list --ready` is the same subset via `list`).

### `mtt roadmap [--json]` — execution-order view  *(session 008.6, implemented)*
Prints the **non-terminal** tasks in an **execution order**, each annotated with whether it is actionable now
(`ready`), what still blocks it (`blocked_by`), and — for a parent — what it contains (`contains`). This is the
"what do I do next, and what's it waiting on" view — what `ready` (unblocked *now*, flat) and `list --sort
priority` (no dependency order, own priority only) each miss. **Not** a time scheduler: no dates, no critical path.

Ordering runs over **two "comes-after" axes**, both **hard** constraints: `depends_on` (an explicit blocking
edge — a non-terminal blocker precedes the task it blocks) **and `parent`** (a parent completes only once its
children do, so a non-terminal child precedes its parent). Priority is the **soft** tie-breaker, and it
**propagates**: a blocker inherits the highest priority of everything it (transitively) unblocks, so a
high-priority task pulls its prerequisites forward, ahead of independent lower-priority work (a *low* task that
blocks a *high* one can outrank an independent *medium* task). Readiness stays `depends_on`-only — the parent
axis affects ordering and the `contains` annotation, not `ready`/`blocked_by` (a parent with open children can
be `ready` yet ordered last). Confirmed-terminal tasks (`done`/`cancelled`) are excluded. The order is
deterministic and cycle-safe across both axes (a hand-edited cycle cannot hang it — every node is still
returned, best-effort).

- Human: a numbered list — `1. t3  [high]  (tbd)  schema design`, the `[..]` priority label **omitted when
  unset** (as in `show`); a `  ↳ blocked by: t1, t2` line under a `depends_on`-blocked task, and a `  ↳
  contains: c1, c2` line under a parent (its non-terminal children).
- `--json`: `[{"id","title","status","priority","ready","blocked_by":[…],"contains":[…]}]`. `priority` is the
  **stored** value (`""` when unset — honest; consumers apply their own default); `blocked_by` and `contains`
  are always arrays (`[]` when empty, never `null`); an empty roadmap is `[]`.

---

## Dependencies  *(session 005, implemented; the `DependencyStore` capability is for external adapters only)*

`depends_on` is a **blocking** edge (distinct from hierarchy `parent` and informational `refs`). It rides
the `Task` field and round-trips via `TaskStore.Update` — the YAML reference needs **no dedicated port**.

### `mtt dep add <id> <depends-on-id>` — add a blocking dependency
Makes `<id>` depend on `<depends-on-id>`. Both tasks must exist. Rejected if it would create a **cycle** or
is a **self-edge**; re-adding an existing edge is an idempotent no-op. With `--json`, echoes the updated task.

### `mtt dep rm <id> <depends-on-id>` — remove a dependency
Removes the edge. Idempotent: removing an edge that is already absent is a no-op (the task must exist).

### `mtt dep list <id>` — list a task's dependencies and dependents
Prints the task's direct blockers (`depends on:`, dangling targets flagged `(missing)`) and its **computed**
dependents (`required by:`). With `--json`, emits `{id, depends_on, required_by}` (non-null arrays).
- `--tree` — show the transitive dependency tree (cycle-safe; nested `--json`).
- `--cycles` — report dependency cycles in the project (defensive — `dep add` rejects cycles, so this only
  fires on hand-edited data).

---

## References  *(shipped t1)*

References are informational, verifiable links (`kind` ∈ `note`/`task`/`url`; **`comment` arrives with t2**)
— not blocking dependencies. They live on **tasks and notes**. A reference is identified by its natural key —
the **pair `(kind, target)`** (no separate reference ID). The target is part of the key, so an entity can
hold many references of the same `kind` to different targets (`note:auth-design` + `note:login-spec` are two
distinct references); only an exact `kind`+`target` duplicate is collapsed. `--label` is an annotation, not
part of identity. The target syntax splits on the **first** `:` (a URL keeps its own). Refs are stored
**sorted by `(kind, target)`** (clean diffs). A `comment:` target, an unknown kind, a malformed URL (needs
scheme + host), a non-kebab note slug, or an empty id is a usage error (**exit 1**).

### `mtt ref add <id> <kind>:<target> [--label <text>]` — add a reference to a task
Adds a reference from task `<id>` to `<kind>:<target>` (e.g. `note:auth-design`, `task:t2`,
`url:https://…`). **Idempotent upsert**: re-adding the same key overwrites its label only when `--label` is
given (omitted → unchanged). **Warn-not-block:** a dangling target (a missing `task`/`note`) is **still
stored**, with a stderr warning and **exit 0**; the carrier task must exist (else **exit 4**). A well-formed
`url` is stored as `unverified` (external, not resolved) and does not warn. `--json` echoes the reference
object `{kind, id, label, status}` (`status` ∈ `ok`/`dangling`/`unverified`).

### `mtt ref rm <id> <kind>:<target>` — remove a reference from a task
Removes the reference with that key. **Idempotent no-op** — removing an absent key still succeeds (**exit 0**),
like `dep rm`/`tag rm`. A missing carrier task exits `4`.

### `mtt ref list <id>` — list a task's references and backlinks
Prints the task's outgoing references (each: `kind:target`, resolution status `ok`/`dangling`/`unverified`,
and label) and its incoming **backlinks** — the tasks **and notes** that reference this task (computed, never
stored). `--json` emits `{refs, backlinks}` (both non-null arrays). Refs/backlinks also appear in `mtt show`.

### `mtt note ref add/rm/list <slug> …` — references on a note
The same three operations for a **note** carrier (`mtt note ref add auth-design task:t2`, …). Notes can
reference tasks/notes/urls; a note's backlinks show who references it.

### `mtt check [--json]` — verify references  *(shipped t1)*
Read-only repo-wide sweep for **dangling** references (a `task`/`note` target that does not exist). Prints
each finding (`carrier → kind:target [status]`) plus a summary, and **exits `7`** when any dangling reference
is found — usable as a CI / flow gate. Capability-aware: `note` refs are only checkable with a knowledge base
(YAML always has one); a `url` is external and reported `unverified`. **`unverified` is not a failure** (exit
0). `--json` emits a non-null array of `{carrier, ref, status}`. (An automated `--fix` is deferred — a
follow-up.)

---

## Comments  *(phase 4; capability `CommentStore`)*

### `mtt comment add <id> <body> [--reply <cid>]` — add a comment
Appends a comment to the task; `--reply <cid>` nests it under an existing comment (tree).

- `--ref <kind>:<target>…` — attach references to the comment.

### `mtt comment list <id>` — print the comment tree
(Also shown by `mtt show`.)

---

## Knowledge base  *(capability `KnowledgeStore`; base store shipped t47)*

Markdown notes, one file per note at `.mtt/knowledge/<slug>.md` (YAML frontmatter + markdown body). The
`<slug>` is the note's identity and file name — an explicit, kebab-ASCII string (`^[a-z0-9]+(-[a-z0-9]+)*$`);
an invalid slug is rejected. A missing note exits `4`. The base store is **single-version** (history is git);
note **versioning + full-text search are deferred** (t6), and **refs on notes are t1**.

### `mtt note add <slug> [flags]` — create a knowledge note
Creates a note at `<slug>`. Rejects an existing slug (no clobber). Body defaults to empty.
- `<slug>` — stable identifier / file name (kebab-ASCII).
- `--title <text>` — human title (optional).
- `--tag <t>` — add a tag (repeatable; explicit only — hashtags in the body are NOT extracted).
- `--priority high|medium|low` — importance (t51; reuses the task `Priority` VO). Drives `mtt prime`.
- `--body <text>` — content; **or** `--file <path>` (`--file -` reads stdin). `--body`/`--file` are exclusive.
- `--ref <kind>:<target>` — attach a reference at creation (repeatable; warn-not-block, t1). `mtt add` takes it too.
- `--json` — emit the created note object.

Refs/backlinks are shown by `mtt note show` and `mtt note ref list`.

### `mtt note list [--tag <t>]… [--priority <p>]… [--sort <k>] [--json]` — list notes
Lists notes (newest first), optionally filtered by tag (OR within tags) and/or `--priority` (repeatable; the
**stored** label — an unset note matches only when no `--priority` is given). `--sort created|updated|priority`
(`priority` = high→low, recency tiebreak). `--json` emits a (non-null) array.

### `mtt note show <slug> [--json]` — print a note
Prints the note's header (title/tags/**priority**/created/updated) and body; `--json` emits the note object
(`slug` always present, `tags` a non-null array, `priority` omitempty).

### `mtt note edit <slug> [flags]` — edit a note
Updates only the provided fields; `--tag` **replaces** the whole tag set. Bumps `updated`, keeps `created`.
- `--title <text>`, `--tag <t>` (repeatable), `--priority <p>` (`--priority ""` clears it), `--body <text>` /
  `--file <path>` (`-` = stdin), `--json`.

### `mtt note rm <slug> [--force --who <w> --why <r>]` — delete a note
Removes the note file. **Refuses by default** if any task or note references it (t1); `--force` overrides —
leaving the refs dangling — and, as a destructive override, **forces `--who`/`--why`** (missing → exit 2) and
writes an audit record. A note's own self-reference never blocks. A missing slug exits 4. `--json` echoes the
removed note.

### `mtt search <query> [flags]` — text search  *(deferred, t6)*
Simple substring/token search over tasks and notes (no RAG).

- `--tasks` / `--notes` — restrict the scope.

### `mtt prime [--min-priority <p>] [--limit N] [--json]` — curated KB digest  *(shipped t51)*
Prints a compact, **pointer-only** digest of the **important** KB notes, for injection at session start (the
beads-`prime` analogue). Curation is **opt-in**: only notes with an **explicit** priority at or above
`--min-priority` (default **`high`**) appear — an **unset note is never** in the digest; note **bodies are
never emitted** (the digest lists slug / priority / tags / title; `mtt note show <slug>` for detail). Ordered
by priority band, then **backlink-count** (more-referenced first), then recency; capped by `--limit`
(default 20). The footer reads `(<N> of <M> …)` where `M` = eligible before the cap. Empty ⇒ a single
actionable line (exit 0). `--json` emits a non-null array of `{slug, title, tags, priority, backlinks}`.
`mtt prime` prints **no tasks** — compose with `mtt roadmap` at the hook if you want both.

**Wire it into session start** (Claude Code `settings.json` — mtt ships only the command; the hook is config):

```json
{ "hooks": { "SessionStart": [ { "hooks": [ { "type": "command", "command": "mtt prime" } ] } ] } }
```

---

## Self-update  *(shipped t44)*

### `mtt self-update [--check-only] [--force] [--json]` — update the installed binary

Replaces the running `mtt` with the latest published GitHub release. **Primary path:** download the platform
asset `mtt_<tag>_<os>_<arch>[.exe]` **and** `SHA256SUMS`, verify the asset's SHA-256 **before** an atomic
in-place swap (Unix: same-dir temp + `rename`; Windows: rename-then-swap). **Fallback:** when the platform has
no verifiable asset (missing asset **or** missing `SHA256SUMS`) and a Go toolchain is present, `go install
github.com/pashukhin/mtt/cmd/mtt@<tag>` (which installs into `GOBIN`/`$GOPATH/bin` — the command reports that
path when it differs from the running binary).

- `--check-only` — report `current` / `latest` / availability; write nothing. **Exit 0** for every resolvable
  state (only a network/API failure exits non-zero) — availability is in the output, not the exit code.
- `--force` — update from a **dev build** (a non-SemVer version, e.g. `dev` or a bare commit SHA, is otherwise
  refused because it can't be ordered) or re-install when already on the latest.
- `--json` — `{current, latest, update_available, updated, via, asset, path, reason, error}`; `via ∈
  {"", "asset", "go-install", "none"}` (`none` = a newer release exists but no install method on this platform).

Already latest ⇒ a no-op (`already up to date`, exit 0). A failure or a refusal exits `1` with an actionable
message (no dedicated taxonomy code — unlike `mtt check`). Consumes exactly what `make release` publishes (see
[RELEASING.md](RELEASING.md)).

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
| `2` | Usage error — here: missing required attribution (`ErrMissingAttribution`) |
| `3` | Transition blocked — a gate command returned non-zero |
| `4` | Not found (task/note/target does not exist) |
| `5` | Post-action failed after a persisted move (`ErrPostAction`, t21) — the move is kept |
| `6` | Invalid transition — not allowed by the type's flow |
| `7` | Integrity: dangling references found by `mtt check` (`ErrDanglingRefs`, t1) |

Codes `3` (gate blocked) and `6` (invalid transition) are **implemented (session 006)**, `2` (missing
required attribution) is **implemented (session 006.5)**, `4` (not found) is **implemented (session 008.5)** —
applied **uniformly** to every single-task-by-id path (`rm`/`show`/`edit`/`tree`/`use`/`status`/`dep`), which
all wrap `mtt.ErrNotFound` — `5` (post-action, t21), and `7` (dangling refs, t1). `Execute()` maps
`core.ErrBlocked`→3, `core.ErrInvalidTransition`→6, `core.ErrMissingAttribution`→2, `mtt.ErrNotFound`→4,
`core.ErrPostAction`→5, and `core.ErrDanglingRefs`→7. An unsupported capability (`ErrUnsupported`) has no
distinct code yet (it lands with capability gates); other error paths keep the generic `1`.

**Bulk mutations (session 008.9)** are best-effort: a partial or total failure exits `1` (generic, git-style)
with a per-item report — the aggregate deliberately does **not** map onto `3`/`4`/`6` (a heterogeneous set has
no single code). A **single** `rm <id>` / `tag add/rm <id>` still exits `4` on not-found. An empty selector
(a source that matched nothing) is a successful no-op (exit `0`); giving two sources, or none, is a usage
error (exit `1`).

---

## Environment variables

| Var | Meaning |
|---|---|
| `MTT_DIR` | Project root containing `.mtt/` (same as `--dir`). |
| `MTT_ROLE` | Acting role recorded in history (same as `--role`). |
| `MTT_BY` | Acting subject recorded in history (same as `--by`). |
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
  adapter capabilities; against a backend that lacks them they surface `ErrUnsupported` (a distinct exit code
  lands with capability gates), not silently.
- **`--json` everywhere.** Every command supports JSON output so agents can drive mtt without parsing
  human text; mutations echo the resulting object — including `add --json`, which emits the created task
  (session 008.97/U3), so the fresh id is `mtt add … --json | jq -r .id` rather than parsed from prose.
  Since **t45** this holds for every mtt command — `types`/`version`/`init` emit JSON, `rm <id> --json` emits
  the removed task object (captured before deletion; bulk `--json` is the per-item outcome array), and
  `use --json` emits the current task or `null`. Only cobra's generated `completion`/`help` are exempt
  (framework built-ins). (The plain `created <id>`/`removed <id>` lines remain the default when `--json` is
  *not* set.)
- **`--role` is recorded, not enforced.** It writes into history now (the non-deferrable seam); role-based
  routing of `start`/`done` is deferred.
