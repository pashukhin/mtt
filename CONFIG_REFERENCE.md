# Config reference — `.mtt/config.yaml`

> Русская версия: [CONFIG_REFERENCE.ru.md](CONFIG_REFERENCE.ru.md). English is the source of truth; keep both in sync.

The authoritative, field-by-field schema of a project's flow config. This is a **reference** (look-ups); to
learn **how to author** a flow, read [FLOW_GUIDE.md](FLOW_GUIDE.md) — the section numbers below cross-link to it.

A flow config is **trusted code**: its gate/`post`/event commands run via `sh -c` with your privileges — only
run or commit a `.mtt/config.yaml` you trust, exactly as you would a `Makefile`. The live config is a two-file
overlay: the committed `.mtt/config.yaml` plus an optional gitignored `.mtt/config.local.yaml` (later wins at
top-level-field granularity). **`omitempty` in the DTO is inert** — the config is decode-only, so a field's
required/optional status is a *validation* property (below), never a serialization tag.

## Top-level keys

| Field | Type | Required? | Meaning / validity | See |
|---|---|---|---|---|
| `version` | int | required | schema version; currently `1`. | §11 |
| `project` | map | required | project block; holds `name`. | §1 |
| `project.name` | string | required | the project's display name. | §1 |
| `command_timeout` | duration | optional (default `5m`) | per-command budget; Go `time.ParseDuration` (`5m`, `10m0s`); a bad value fails at load. | §4 |
| `author` | string | optional | acting-subject default; normally set in the gitignored `config.local.yaml`, not the committed file (see Overlay). | §6 |
| `extract_hashtags` | bool | optional (default `false`) | opt-in extraction of `#hashtags` from a task's title/description into tags. | — |
| `require` | map | optional | required-attribution policy `{who, why}` (see Require); **tighten-only**. | §6 |
| `events` | map | optional | lifecycle-event pipelines fired on non-flow mutations (see Events). | §5b |
| `types` | list | required (≥1) | the entity types; each defines its own flow (see Type). | §1, §3 |

## `types[]` — a Type

| Field | Type | Required? | Meaning / validity | See |
|---|---|---|---|---|
| `name` | string | required | type name, unique across types. | §1 |
| `prefix` | string | required | id prefix (the YAML adapter mints `<prefix><N>`); **letters-only** `[a-zA-Z]+`, **unique** across types. | §3 |
| `parents` | list of string | required (may be `[]`) | parent type names; `[]` = a root type; `[epic]` = placeable only under an `epic`. | §3 |
| `default` | bool | optional | mark **exactly one** type `default: true` — used by `mtt add` without `--type`. | §3 |
| `description` | string | optional | human description, shown by `mtt types`. | — |
| `post_defaults` | list of command | optional | finalization commands prepended to every edge's `post:` for this type (opt out per-edge with `inherit_post: false`). | §5 |
| `statuses` | list | required | the states (see Status); needs ≥1 `initial`, ≥1 `active`, ≥1 `terminal`. | §1 |
| `transitions` | list | required | the directed edges (see Transition). | §1 |

## `statuses[]` — a Status

| Field | Type | Required? | Meaning / validity | See |
|---|---|---|---|---|
| `name` | string | required | status name, unique within the type. | §1 |
| `kind` | enum | required | one of `initial` \| `active` \| `terminal`; must **match topology** (`initial` = no incoming, `terminal` = no outgoing, `active` = both). | §3, §11 |
| `description` | string | optional | shown by `mtt show`/`mtt types` (self-documents the next step). | — |
| `default` | bool | optional | **at most one** status may be `default`, and it **must be `initial`** (the entry point when a type has >1 initial). | §3 |

## `transitions[]` — a Transition (edge)

| Field | Type | Required? | Meaning / validity | See |
|---|---|---|---|---|
| `from` | string | required | source status name (must resolve). | §1 |
| `to` | string | required | target status name (must resolve). | §1 |
| `name` | string | optional | edge verb for `mtt <name> <id>` / `mtt do`; **unique per source status** and **disjoint from status names**. | §3 |
| `description` | string | optional | shown at the edge by `mtt show`/`mtt types`. | — |
| `commands` | list of command | optional | **gates** — run in order; all must exit 0 or the move is blocked (exit 3). | §4 |
| `current` | enum | optional | `set` \| `clear` — moves the personal "current task" pointer (empty = no change). | §6 |
| `require` | map | optional | per-edge `{who, why}`; unioned with the global policy (tighten-only). | §6 |
| `post` | list of command | optional | **finalization** commands run *after* the move persists; a failure keeps the move (exit 5). | §5 |
| `inherit_post` | bool | optional | pointer semantics — absent ≠ `false`; only an explicit `inherit_post: false` opts the edge out of the type's `post_defaults`. | §5 |

## A command (`commands` / `post` / `post_defaults` items)

A command is a **scalar string** (a shell command, run via `sh -c`) **or** a map:

| Field | Type | Required? | Meaning / validity | See |
|---|---|---|---|---|
| `run` | string | required (map form) | the shell command. | §4 |
| `timeout` | duration | optional | per-command budget overriding `command_timeout`; Go `time.ParseDuration`. | §4 |
| `rollback` | command | optional | compensator (scalar or map) run in reverse if a **later** gate command fails; rejected in any `post`/event surface. | §5 |

## `events` — lifecycle pipelines (post-only)

`events:` hangs **post-only** command lists on non-flow mutations, per store × kind, fired per entity, only
when something persisted:

| Field | Type | Meaning | See |
|---|---|---|---|
| `task` / `note` | map | the entity store the hooks apply to. | §5b |
| `create` / `update` / `delete` | map | the mutation kind; holds `post`. | §5b |
| `post` | list of command | commands to run after that mutation persists. | §5b |

## `require` — required attribution

| Field | Type | Meaning | See |
|---|---|---|---|
| `who` | bool | require an acting subject before the gate runs. | §6 |
| `why` | bool | require a free-text reason. | §6 |

**Tighten-only:** `config.local` and per-edge `require` may *add* a requirement, never relax a committed one.
Attribution resolves in order: `--who`/`--by` > `MTT_BY` (env) > `author:` in `config.local.yaml`.

## `.mtt/config.local.yaml` — the gitignored overlay

| Field | Type | Meaning |
|---|---|---|
| `author` | string | your durable personal acting-subject default (source of truth for `by`). |
| `current` | string | the personal "current task" id pointer (managed by `current: set`/`clear`; not hand-edited). |

## Placeholders

Only a fixed whitelist expands; any other `{{…}}` (e.g. `{{.Title}}`) is a template error, never interpolated
(a safety boundary — free text rides the environment, not the command string).

- **Edge `commands`/`post`/`post_defaults`:** `{{.ID}}`, `{{.Type}}`, `{{.From}}`, `{{.To}}`.
- **`events.task.*`:** `{{.ID}}`, `{{.Type}}`, `{{.Event}}`.
- **`events.note.*`:** `{{.Slug}}`, `{{.Event}}`.

## Validating your config

`mtt add` and `mtt types` run structural validation and reject a broken flow before runtime (see
[FLOW_GUIDE.md §11](FLOW_GUIDE.md)). *Reference* integrity (dangling `depends_on`, cycles via `mtt check`) is
a separate concern.
