# mtt

**An executable task state machine for coding agents — a fuse between an agent and the word "done".**

> 🇷🇺 Читать по-русски: [README.ru.md](README.ru.md)

> **Status:** working alpha (`0.8.98-dev`). Phases 1–3 are implemented — `init`, `add`/`show`/`list`/`edit`,
> hierarchy (`tree`), dependencies (`dep`/`ready`), and the **flow gate** (`mtt status <id> <new>` or the
> `mtt <status> <id>` sugar run a transition's commands and block on a red gate, writing history; structured
> commands + rollback included).
> The `advance`/`start`/`done` meta-walk is **parked** (single-edge `status` is the norm); the knowledge base,
> search, `mtt-ui`, and external adapters are later phases. Full plan in [DESIGN.md](DESIGN.md).

> **Pitch.** Coding agents write code well but respect a task's lifecycle poorly — "done" is often just a
> text label. mtt turns a task into an executable state machine: a status transition passes through gates
> — create the branch, run lint/test, check artifacts — and if a gate is red, `mtt done` doesn't pass.
> The Definition of Done is **per task type** (bugfix, refactor, feature each differ). It's not a commit
> hook or CI — it's a gate on the *task lifecycle*, in the agent's own vocabulary, over your storage:
> zero-footprint YAML for solo, or a thin enforcement layer over Jira/GitHub for a team.

## What it is

`mtt` is a small Go CLI (`mtt add`, `mtt start`, `mtt done`, …). Its defining idea: a **config-driven
per-type status flow whose transitions run gates** (shell commands that must pass) — so an agent can't mark
a task `done` without meeting its Definition of Done. Its second idea, the adoption ladder: **storage is
abstracted behind ports**, so the same CLI runs over local files or your existing tracker.

- **Solo / zero-footprint:** the default backend is local YAML files (one file per task, in `.mtt/`) —
  no daemon, no database, no ports; clean git merges and PR-reviewable diffs.
- **Corporate:** point mtt at an existing "tracker + knowledge base" pairing (Jira+Confluence,
  GitHub Issues+Wiki, …) via an adapter. Agents get the same clean CLI; humans keep their native UIs.

## Why

`beads` is great but heavy for this use: it conflicts with other tooling, and its flow is a flat status
enum. mtt's wedge is **lightness** and a **real per-type flow** — plus the ability to adapt to whatever
backend you already have. (An honest comparison lives in
[DESIGN.md](DESIGN.md#positioning-honestly-vs-beads).)

## Key ideas

- **Executable, per-type transitions (the killer feature).** A status transition carries shell commands
  that must all pass — `["make lint", "make test"]` gating `→ done` — or perform actions (create a branch).
  The Definition of Done differs per task type (bugfix/refactor/feature). Agents work in task terms
  (`mtt start`, `mtt done`) while the tool enforces the discipline.
- **Config-driven types & hierarchy.** Epic → task → subtask is just the default config; no type names or
  ID structure are hardcoded. IDs are readable and **flat** per-prefix (`e1`, `t17`, `s3`) — stable under
  re-parenting (hierarchy lives in a `parent` field, not the ID).
- **Priorities, tags, dependencies & a roadmap.** `--priority`, `#hashtags` / `--tag`, `depends_on` / `ready`,
  and `mtt roadmap` give an agent an ordered, filterable backlog; a `current` pointer tracks the task in flight,
  and batch/pipeline (`--ids`, stdin `-`) makes it Unix-composable.
- **Hexagonal, pluggable backends.** A public contract (`pkg/mtt`) with `TaskStore` and `KnowledgeStore`
  ports; the YAML adapter is the reference. Optional features (history, dependencies, comment trees,
  search) are per-adapter capabilities.
- **Verifiable references & append-only history.** Tasks/comments carry checkable `refs` to notes/tasks;
  every status transition is recorded for audit.
- **Optional human UI (`mtt-ui`).** A small local web server (task management, Gantt, KB browser) — only
  needed on the YAML default; with an external backend, humans use its native UI.

## For agents

Task and dependency management from the command line, comment trees, optional knowledge storage, simple
text search, and an optional external indexer hook.

## For humans (optional)

`mtt-ui` (a small local web server): minimal task management, a Gantt chart, a KB browser. Plus a CLI
(not TUI) text output for task info, KB search, and an ASCII Gantt.

## Install

### Prebuilt binary (recommended)

Download the binary for your platform from the
[latest release](https://github.com/pashukhin/mtt/releases/latest):

| Platform | Asset |
|---|---|
| Linux x86-64 | `mtt_<version>_linux_amd64` |
| Linux ARM64 | `mtt_<version>_linux_arm64` |
| macOS (Intel) | `mtt_<version>_darwin_amd64` |
| macOS (Apple Silicon) | `mtt_<version>_darwin_arm64` |
| Windows x86-64 | `mtt_<version>_windows_amd64.exe` |

Then, on Unix:

```sh
chmod +x mtt_<version>_<os>_<arch>
sudo mv mtt_<version>_<os>_<arch> /usr/local/bin/mtt
mtt version
```

Verify integrity against `SHA256SUMS` from the same release (`sha256sum -c` on your asset's line).

### With the Go toolchain

```sh
go install github.com/pashukhin/mtt/cmd/mtt@latest
```

### Build from source

```sh
git clone https://github.com/pashukhin/mtt && cd mtt
make build      # -> bin/mtt
```

## Quickstart

```sh
mtt init                              # create .mtt/ (default template: epic > task > subtask)
mtt add "Ship auth" --type epic       # -> e1
mtt add "Login endpoint" --parent e1  # -> t1
mtt status t1 in_progress             # move through the flow (a transition's commands run + gate here)
mtt done t1                           # -> the done terminal (default flow has no gate commands)
mtt list                              # all tasks
mtt tree                              # the epic > task hierarchy
mtt roadmap                           # dependency + priority execution order
```

See [CLI_REFERENCE.md](CLI_REFERENCE.md) for the full command surface and exit codes.

## Docs

- [DESIGN.md](DESIGN.md) — architecture and decisions (the source of truth)
- [CLI_REFERENCE.md](CLI_REFERENCE.md) — the full CLI command surface (target design)
- [AGENTS.md](AGENTS.md) — how to work in this repo (rules, gate, principles)
- [TASKS.md](TASKS.md) — the phased plan
