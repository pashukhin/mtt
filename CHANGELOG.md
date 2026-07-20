# Changelog

All notable changes to mtt are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the version scheme is
[Semantic Versioning](https://semver.org); see [RELEASING.md](RELEASING.md) for the pre-1.0 bump rules.

## [Unreleased]

### Added
- **`mtt self-update`** — update the installed binary to the latest published release: download the platform
  asset + `SHA256SUMS`, verify the SHA-256, and atomically replace the running binary; falls back to
  `go install …/cmd/mtt@<tag>` when no verifiable asset matches the platform. `--check-only` / `--force` /
  `--json`.

## [0.9.0] — 2026-07-20

First public release. `mtt` is an agent-friendly "tasks + knowledge" pairing (a Go CLI) built around an
executable, gated status flow. Everything below ships in 0.9.0.

### Added
- **Project & flow:** `mtt init` (`default` / `coding` templates — the `default` is flat (`task` + `chore`),
  hierarchy is opt-in), `mtt types` (type + edge map), `mtt guide` (pre-flow orientation: queue navigation,
  first-move setup, mid-flight resumption).
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
- **Tags:** `#hashtags` in title/description (the primary path) + `mtt tag add/rm`; `--tag` / `--exclude-tag`
  filters on `add` / `list` / `ready` / `tree`, plus `mtt tags` (the tag vocabulary with counts).
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
- **Knowledge base (notes):** a `KnowledgeStore` port with a YAML/markdown adapter (`.mtt/knowledge/<slug>.md`)
  and `mtt note add/list/show/edit/rm` — notes carry tags and priorities, and `note list` takes `--priority` /
  `--sort` filters.
- **References:** `mtt ref add/rm/list` and `mtt note ref …` attach verifiable `note:` / `task:` / `url:`
  references (also `--ref` at creation); backlinks are computed, `mtt check` sweeps for dangling refs
  (exit 7), and delete guards refuse to remove a still-referenced task/note without `--force`.
- **KB prime:** `mtt prime` — a curated, bounded session-start digest of the knowledge base
  (`--min-priority`, `--limit`), ranked by note priority and backlinks, for injection at session start.
- **JSON everywhere:** every command honors `--json` (rounded out across `types` / `version` / `init` /
  `rm` / `use`), so agents can drive mtt entirely from structured output.

### Changed
- The root tagline now names the executable-state-machine + gate feature (was "minimalist file-backed task
  tracker").
- **Versioning:** adopted SemVer (pre-1.0); the version is now derived from the git tag
  (ldflags / `git describe` → module build info → `"dev"`), replacing the hand-maintained session-mirrored
  `0.N.M-dev` literal. See [RELEASING.md](RELEASING.md).

### Fixed
- **YAML `List`/`Get` name the offending file** on a corrupt or zero-byte task file (was a pathless
  `mtt: empty task id`); the not-found path is unchanged.
- **`Status.Default` is honored through the YAML adapter** (mapped in the DTO; previously the marker on an
  initial status was silently dropped and the first initial always won).

### Packaging
- Cross-platform prebuilt binaries via `make release` + a tag-triggered GitHub release workflow;
  `SHA256SUMS` for integrity.

[Unreleased]: https://github.com/pashukhin/mtt/compare/v0.9.0...HEAD
[0.9.0]: https://github.com/pashukhin/mtt/releases/tag/v0.9.0
