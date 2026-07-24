# Changelog

All notable changes to mtt are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the version scheme is
[Semantic Versioning](https://semver.org); see [RELEASING.md](RELEASING.md) for the pre-1.0 bump rules.

## [Unreleased]

### Added
- **Lifecycle events (t66).** A new top-level `events:` config section hangs post-only command pipelines on
  create/update/delete of tasks and notes: they fire per entity, from the mutating usecases, only when
  something really persisted — never from a flow move. Task events expand `{{.ID}}`/`{{.Type}}`/`{{.Event}}`
  (the type is membership-checked against the config before expansion — a poisoned on-disk `type:` never
  reaches the shell), note events `{{.Slug}}`/`{{.Event}}`. A failing event keeps the mutation (exit 5 with
  the exact remaining commands; bulk aggregates stay the generic exit 1). Every mutating verb gains
  `--no-run` (skips the pipeline, forces `--who`/`--why`, writes an audit skip-record;
  `rm --force --no-run` writes one combined record). The dogfood config now auto-commits every task/note
  mutation (closing the manual-`.mtt`-commit gap), with narrowed pathspecs and `mtt: <id> <event>` subjects
  that can never satisfy the deliver squash-grep.
- **Per-type `post_defaults` (t66/t24).** A type-level command list prepended to every edge's `post:`;
  an edge opts out with `inherit_post: false`. Precedence: defaults first, edge specifics appended, opt-out
  only explicit. The repo's ~30 duplicated auto-commit post lines collapsed to one per type. `rollback:` is
  now rejected in every post surface (edge `post:`, `post_defaults:`, event `post:`).
- **Upgrade note:** a pre-t66 binary silently ignores `events:`/`post_defaults:` (non-strict decode) —
  update your installed binary after this lands, or moves and mutations stop auto-committing with no signal.
- **Dogfood flow gates (t31).** Every `submit` and both `impl_review → approved` edges now require a clean
  working tree (`.mtt` excluded — the move's own post-commit sweeps it); impl submits additionally require
  a CHANGELOG entry when code changed vs the merge base (audited bypass:
  `mtt do submit --no-run --who … --why …`); `deliver` reminds to capture lessons via `mtt note add`.
- **`mtt init` writes `.mtt/.gitignore`** ignoring the personal `config.local.yaml` overlay, so it can never
  be committed (with auto-committing flows like `git add .mtt` post actions it used to land in the shared
  repo). Create-if-absent: an existing `.mtt/.gitignore` is never overwritten, even with `--force`. Existing
  projects add it by hand: `echo config.local.yaml > .mtt/.gitignore`.
- **`depends_on` is visible in the agent contract (c12).** The task object carries `depends_on` (omitempty,
  like `priority`) on every task `--json` surface (the one exception: `dep list --tree --json` nodes, where
  `depends_on` is the nested child-node array), and `mtt show` prints a `depends:` line listing each blocker
  with its status — `✓` marks a terminal (satisfied) blocker, `(missing)` a dangling one. Previously the
  stored field was invisible outside `dep list`.

### Changed
- **`--no-run` caveat sweep (c19).** CLI_REFERENCE/FLOW_GUIDE (EN+RU) and the dogfood `deliver`/`cancel`
  descriptions now state that a bypass skips ALL edge commands — state-moving ones included — and DESIGN
  records why the signed bypass stays (the unsigned alternative, a local config edit, is invisible).
- **Error-message polish, round 2 (c14).** Five messages now read honestly:
  - `mtt init --template <bogus>` lists the valid names (`coding, default, hierarchy`) instead of just
    rejecting the input.
  - Moving a task whose status is **not in the flow** (a hand-edited / config-drift value) says so
    (`status X is not in the "task" flow — config drift?`) instead of the wrong "it is terminal".
  - The edge-verb sugar at the wrong status — e.g. `mtt decline t1` when `decline` is a real edge name but
    not valid out of the current status — now routes to the actionable exit-6 error listing the available
    actions (same as `mtt do`), instead of a misleading "unknown command" (exit 1).
  - `mtt tag add/rm` reports only the tags that **actually changed**: removing an absent tag or adding a
    present one reads as a no-op (`t1: no such tag ghost (no change)`) instead of a false `untagged/tagged`.
  - `mtt ref list` / `mtt note ref list` print `(none)` under an empty `refs:`/`backlinks:` header (mirroring
    `dep list`) instead of a bare header.

### Fixed
- **Text & tag hygiene (c13).** A `#fragment` inside a pasted `scheme://` URL is no longer minted as a tag —
  `mtt add 'see https://ex.com/page/#anchor'` used to create the tag `anchor` (and the `tag rm` guard then
  refused to remove it while the link stayed in the text). And `add`/`edit` now reject a **newline in the
  title** with a usage error: a title renders as one row in `list`/`tree`, so an embedded newline masqueraded
  as separate tasks for line-oriented consumers. Multi-line text belongs in the free-form `description`.
- **Storage durability (c18).** Every store write (tasks, notes, config, `config.local.yaml`) is now
  fsynced before the atomic rename and the parent directory is fsynced after it — a crash can no longer
  promote un-flushed bytes or lose the just-renamed entry; `.mtt/audit.log` appends fsync too ("no
  destruction without a trail" now survives power loss). Consistent perms: a **new** file lands `0644`
  (the git-checkout default) instead of the old 0600/0644 flip-flop across machines; an existing file
  keeps its own mode (never silently loosened), and a fresh `config.local.yaml` lands `0600` (it may hold
  credentials). A **zero-byte** task or note file — the artifact of a crash inside the id-reserve
  window — no longer bricks every `list`/`roadmap`/`ready`/`tree`: the store treats it as absent
  (`show` of it is "not found"), and its id stays consumed so dangling references are never silently
  re-pointed.
- **`dep list --tree --json` no longer drops diamond dependencies (c16).** A node reachable through two
  branches (t1→{t2,t3}, both→t4) was emitted only under the first branch — the second looked
  dependency-free in the headline JSON. A revisited node now renders without children (the text tree's
  revisit policy), in diamonds and hand-broken cycles alike.
- **Filter-flag parity across commands (c16).** The bulk selector (`rm`/`tag` `--filter`) gains
  `--exclude-tag` (extending the documented `--exclude-tag` de-noise idiom to `rm`), `ready` gains
  `--priority`, and `tree` gains `--type`/`--priority`/`--parent` — `list`/`ready`/`tree`/selector now share
  the same filter surface.

### Security
- **Security & error-wrapping linters in the quality gate (c17).** `make check` / CI now run `gosec` (security
  scanner) and `errorlint` (error-wrapping hygiene) under `golangci-lint` — apt for a tool that shell-executes
  gate/post commands and self-updates. Triaged to a clean gate: gosec's permission thresholds are pinned to the
  committed `.mtt` store's git-checkout perms (0755 dirs / 0644 files, per the c18 storage policy); the YAML
  store adapter — which has no network surface and opens only paths under the `.mtt` root, from mint-generated
  ids or re-validated note slugs — is exempted from G304 (file-inclusion-via-variable); and the two deliberate
  subprocess sites (the gate runner and self-update `go install`) plus the two user-named-file CLI sites
  (`--file` read, `--log-file` write) carry signed `//nolint:gosec`. No change to runtime behavior.
- **RCE fix (c15): the YAML store rejects a poisoned task file at load.** A hand-written `.mtt/tasks/*.yaml`
  whose `id:` field carried shell metacharacters was expanded into `{{.ID}}` inside gate/post `sh -c`
  commands — arbitrary code execution on the next `mtt` move (task files are machine-written, so a poisoned
  one rode a PR through the review blind spot). `Get`/`List` now fail closed: the in-file `id:` must equal the
  filename stem (also catching the duplicate-id split-brain) and match the adapter's id encoding
  `^[a-zA-Z]+[0-9]+$`; a non-letter type `prefix` is rejected at config-load so mint can never produce an id
  outside that shell-safe charset.

## [0.10.0] — 2026-07-22

### Added
- **`mtt self-update`** — update the installed binary to the latest published release: download the platform
  asset + `SHA256SUMS`, verify the SHA-256, and atomically replace the running binary; falls back to
  `go install …/cmd/mtt@<tag>` when no verifiable asset matches the platform. `--check-only` / `--force` /
  `--json`.

### Fixed
- **SEC1: gate timeout now kills the whole process group**, not just the top shell — a gate/post command that
  backgrounds a daemon (`daemon &`, `nohup`) can no longer survive its deadline (Unix `Setpgid` +
  `kill(-pgid)`; best-effort on Windows). Also closes a former infinite `Wait` hang when a gate exits 0 but
  leaves a child holding the inherited output pipe (`Cmd.WaitDelay`).

### Changed
- **Errors are now actionable.** Exit 2 (missing attribution) prints how to set who/why (`author:` in
  `.mtt/config.local.yaml` / `MTT_BY` / `--who` / `--why`); exit 5 (post-action failed) says the move is
  already saved (do not re-run it) and prints the exact remaining `post:` commands to finish by hand (text and
  `--json`); exit 4 (not found) points at `mtt roadmap`/`mtt list`; an invalid move out of a terminal status now
  reads cleanly instead of a dangling "allowed from …:".
- **`--tag`/`--exclude-tag` accept comma-separated values** (`--tag a,b,c`) like `--depends-on`, tool-wide
  (authoring + filters), still repeatable. `mtt add`'s too-many-arguments error now explains the comma/repeat
  form instead of only blaming the title.
- **Shown flow descriptions/guidance expand placeholders** — `{{.ID}}`/`{{.Type}}`/`{{.From}}`/`{{.To}}` now
  expand in on-move guidance and `mtt show` (human and `--json`), best-effort (a bad template shows raw), so
  guidance names the concrete task (`task/t17`, not `task/<id>`).

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

[Unreleased]: https://github.com/pashukhin/mtt/compare/v0.10.0...HEAD
[0.10.0]: https://github.com/pashukhin/mtt/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/pashukhin/mtt/releases/tag/v0.9.0
