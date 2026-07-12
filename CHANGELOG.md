# Changelog

All notable changes to mtt are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the version scheme is
[Semantic Versioning](https://semver.org); see [RELEASING.md](RELEASING.md) for the pre-1.0 bump rules.

## [Unreleased]

First public release line. Shipped so far:

### Changed
- **Versioning:** adopted SemVer (pre-1.0); the version is now derived from the git tag
  (ldflags / `git describe` → module build info → `"dev"`), replacing the hand-maintained session-mirrored
  `0.N.M-dev` literal. See [RELEASING.md](RELEASING.md).

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
- **Dogfood hardening (s008.97):** a blocked gate echoes the failing command's output tail (~10 lines) under
  its `✗` line and hints `-v`/`--log-file`; `mtt add --json` emits the created task; `mtt show --json` carries
  a `history` array (checks + attribution); discoverability — `status [<id>]` usage, the verb sugar in root
  help, and a `run 'mtt init'` hint outside a project; a tagline that names the gate/state-machine.
- **Named transitions + edge-verb sugar (s008.98):** an optional `name:` on a transition gives a semantic verb
  for the edge out of the current status — `mtt do [<id>] <edge>` (explicit) and `mtt <edge> [<id>]` (sugar),
  symmetric to `mtt status`/`mtt <status>` (e.g. `mtt decline t1` for `review → fix`). Edge names show in
  `mtt types`, the `next:` guidance, and `show --json` (`next[].name`). New flow validation: an edge name is
  unique per source status, disjoint from status names, and every `(from,to)` pair is unique per type.

### Changed
- The root tagline now names the executable-state-machine + gate feature (was "minimalist file-backed task
  tracker").

### Fixed
- **YAML `List`/`Get` name the offending file** on a corrupt or zero-byte task file (was a pathless
  `mtt: empty task id`); the not-found path is unchanged.
- **`Status.Default` is honored through the YAML adapter** (mapped in the DTO; previously the marker on an
  initial status was silently dropped and the first initial always won).

### Packaging
- Cross-platform prebuilt binaries via `make release` + a tag-triggered GitHub release workflow;
  `SHA256SUMS` for integrity.

[Unreleased]: https://github.com/pashukhin/mtt/commits/main
