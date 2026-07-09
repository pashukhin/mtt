# Changelog

All notable changes to mtt are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Pre-1.0 versions mirror the development
session (see [sessions/README.md](sessions/README.md)): a full session bumps the minor, a point-session
the patch.

## [Unreleased]

First public release line. Shipped so far:

### Added
- **Project & flow:** `mtt init` (`default` / `coding` templates), `mtt types`.
- **Tasks (CRUD):** `mtt add` (`--type` / `--parent` / `--priority` / `--tag` / `--depends-on`), `mtt show`,
  `mtt list` (status/type/kind/parent/priority/tag/ready filters, `--json`), `mtt edit`,
  `mtt rm` (reject-if-referenced + `--force`), `mtt tree` (hierarchy).
- **Dependencies:** `mtt dep add/rm/list` (`--tree` / `--cycles`), `mtt ready`, `list --ready`.
- **Flow gate (the killer feature):** `mtt status <id> <new>` runs a transition's executable `commands` and
  gates on exit codes (blocked → the task stays put); append-only `history`; verb sugar `mtt <status> <id>`;
  attribution `--why` / `--who`; project-global required-attribution; exit codes `2`/`3`/`4`/`6`.
- **Current task (working context):** `mtt use`, a personal `current` pointer in `config.local`, moved by a
  `Transition.Current` (`set` / `clear`) flow property.
- **Structured commands:** per-command `{run, timeout}` with placeholder expansion (`{{.ID}}` / `{{.Type}}` /
  `{{.From}}` / `{{.To}}`) and per-command rollback/compensation (reverse-order, best-effort).
- **Priorities + roadmap:** `--priority` (`high` / `medium` / `low`), `--sort priority`, `mtt roadmap [--json]`
  (dependency + priority execution order).
- **Tags:** `#hashtags` in title/description (the primary path) + `mtt tag add/rm`, `--tag` filters on
  `add` / `list` / `tree`.
- **Batch & pipeline:** a task-set selector (explicit IDs | `--filter` | stdin `-`), `--ids` output on
  `list` / `ready`, and bulk `tag add/rm` + `rm` (subgraph-aware).

### Packaging
- Cross-platform prebuilt binaries via `make release` + a tag-triggered GitHub release workflow;
  `SHA256SUMS` for integrity.

[Unreleased]: https://github.com/pashukhin/mtt/commits/main
