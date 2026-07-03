# mtt

**A minimalist, file-native task tracker + knowledge base for coding agents and humans.**
Like Jira + Confluence, without the bulk.

> 🇷🇺 Читать по-русски: [README.ru.md](README.ru.md)

> **Status:** early — design phase. The architecture is settled (see [DESIGN.md](DESIGN.md));
> implementation starts at phase 1. Phase 0 (scaffold + quality gate + CI) is done.

## What it is

`mtt` is a small Go CLI (`mtt add`, `mtt list`, …) that gives coding agents a clean, task-centric
interface, and humans an optional light UI. Its defining idea: **storage is abstracted behind ports**.

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

- **Config-driven types & hierarchy.** Epic → task → subtask is just the default config; no type names or
  ID structure are hardcoded. IDs are readable and hierarchical (`e1_t3_s2`).
- **Executable transitions (the killer feature).** A status transition can carry a description and a
  sequence of shell commands that must all pass (e.g. `["make lint", "make test"]` gating `→ done`), or
  perform actions (create a branch). Agents work in task terms (`mtt start`, `mtt done`) while the flow
  hides the mechanics.
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

## Build

```bash
make build      # -> ./bin/mtt
make check      # the gate: fmt + vet + lint + test -race + build
```

## Docs

- [DESIGN.md](DESIGN.md) — architecture and decisions (the source of truth)
- [AGENTS.md](AGENTS.md) — how to work in this repo (rules, gate, principles)
- [TASKS.md](TASKS.md) — the phased plan
