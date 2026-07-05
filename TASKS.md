# TASKS

Bootstrap tracker until self-hosting. Once tasks + hierarchy + dependencies exist (end of phase 4),
mtt's development moves onto mtt itself, and this file is frozen.

The identifiers (`e{N}_t{M}`) are illustrative bootstrap numbering only, predating the flat-ID decision
(mtt's actual scheme is flat per-prefix, e.g. `e1`/`t17` — see DESIGN.md); not re-derived here.
Order and architecture — in [DESIGN.md](DESIGN.md); rules — in [AGENTS.md](AGENTS.md).

Legend: `[ ]` todo · `[~]` in progress · `[x]` done.

**Cross-cutting — global flags** (root persistent flags; see [CLI_REFERENCE.md](CLI_REFERENCE.md) → "Global
flags"). Not a phase of their own — land early so new commands inherit them instead of retrofitting:
`[x]` `--dir`/`MTT_DIR` + the `--version` flag (verify `--help`) with `mtt list` (**shipped session 003**,
via `projectRoot`; also DRYs the repeated `Getwd → FindRoot`); `[x]` `--json` machine output, `show`/`list`/
`edit` (**shipped session 003**, via `taskJSON`); `--role`/`MTT_ROLE` in phase 3 with `history` (see e4_t4);
`-q/--quiet`, `--no-color` later.

---

## Status & session mapping (updated 2026-07-05)

Sessions are the operative slices (see [sessions/README.md](sessions/README.md)); the phase epics below are
the architectural axis. They map many-to-one, not one-to-one. Current position:

- **Phase 0 (e1)** — `[x]` scaffold.
- **Phase 1 (e2)** — `[~]` shipped across **s001** (init & types), **s002** (add & show), **s003** (list &
  edit + global flags). Not done: `mtt close` (a status change → belongs to phase 3/flow, not phase 1) and
  the **optional capability interfaces** (`HistoryStore`/`DependencyStore`/`CommentStore`/`SearchStore`,
  `Capabilities()`, `ErrUnsupported`) — deliberately NOT built yet; now designed in
  [docs/architecture/model.go](docs/architecture/model.go) and added per capability when first needed.
- **Phase 2 (e3)** — `[x]` hierarchy (index/traversal, `tree`, `show` lineage, `add --parent`,
  `list --parent/--kind`) shipped in **s004**; dependencies / `ready` / cycles shipped in **s005**
  (`core.DependencyEditor`/`Ready`/`DepGraph`; `mtt dep add/rm/list` + `--tree`/`--cycles`, `mtt ready`,
  `list --ready`).
- **Phase 3 (e4)** — `[~]` single-edge flow enforcement shipped in **s006**: the `Runner` port +
  `internal/adapter/exec` + fake, `core.Transitioner`, `mtt status <id> <new>` (gate on `commands`, exit
  codes 3/6, append `history`, `--role`/`--by`), config-driven `command_timeout`, `mtt show` history. Next:
  the `advance`/`start`/`done` meta-walk (e4_t5 / **s007**).

Two decisions from the domain-model snapshot ([docs/architecture/model.go](docs/architecture/model.go)):

1. **s005 adds no new port.** `depends_on` is a `Task` field round-tripped via `TaskStore.Update` (as
   `parent` was in s004); `DependencyStore` is only for external adapters that cannot embed. s005 = core
   `DependencyEditor` + `Ready` + cycle-check, no `pkg/mtt` port method.
2. **Typed-identity retrofit** (`TaskID`/`TypeName`/`StatusName`) — **shipped (chore 004.5)**: the
   `pkg/mtt`/`core`/`adapter`/`cli` surface uses the typed identities, so s005 is written against the typed
   contract. The YAML DTO keeps plain strings on disk (`string↔typed` at its boundary); the only behaviour
   change is fail-fast on a corrupt on-disk `id`/`type`/`status`.

---

## e1 — Phase 0: project scaffold  `[x]`

- [x] e1_t1 — git init, Go module `github.com/pashukhin/mtt`, `main` branch
- [x] e1_t2 — CLI skeleton: `cmd/mtt` + `internal/cli` (root + `version`) + a test
- [x] e1_t3 — gate: Makefile `make check`, `.golangci.yml` (v2), `.gitignore`
- [x] e1_t4 — CI: `.github/workflows/ci.yml` (the same gate)
- [x] e1_t5 — DESIGN.md, AGENTS.md, README.md
- [x] e1_t6 — guards: principles (SOLID/DRY/KISS/TDD), hierarchical CLAUDE.md, superpowers

## e2 — Phase 1: `pkg/mtt` contract, config, `mtt init`, YAML adapter, core, commands  `[~]`

Test-first, one subtask per branch+PR. **Start with planning** (see NEXT_SESSION.md); the breakdown
below is a guide — planning refines it. Invariants: types/hierarchy come from config (no literals in
code); the **adapter** mints the ID/slug; exactly one type is marked `default` (no literal `task`); each
flow has ≥1 status of each kind (initial/active/terminal), `kind` by topology; storage is behind a port;
`core` doesn't import `adapter/*`.

- [x] e2_t1 — plan phase 1 (superpowers), reconcile with the DESIGN.md invariants
- [~] e2_t2 — `pkg/mtt` **pure** contract (no serialization tags, no `prefix`): `Config`, `Type`
      (`name/description/parents/default/flow`), `Flow`, `Status` (`name/kind/description`; `kind` a
      `StatusKind` **value object**), `Transition` (`from/to/description/commands`); `Task` (with
      `history[]`+`refs[]`), `Comment` (`refs[]`), `Ref` {kind,id,label}; the history entry reserves `role`
      — the roles seam; the base `TaskStore` + optional capability interfaces (`HistoryStore`,
      `DependencyStore`, `CommentStore`, `SearchStore`), `Capabilities()`, `ErrUnsupported`; references by
      identity, back-refs computed + `pkg/mtt/CLAUDE.md`
- [x] e2_t3 — config: type (`name/description/parents/default/statuses(with kind)/transitions`; `prefix`
      is a YAML-adapter field, held in the adapter DTO), **structural name-agnostic** invariant validation
      (kind↔topology; ≥1 of each kind; no 2-status flow; multiple initials ok; per-flow status identity, no
      cross-flow transitions; at-most-one `default` at the domain / exactly-one at the YAML provider; prefix
      present+unique in the adapter); the default template (via DTO→domain mapping); config load merges an
      optional gitignored `.mtt/config.local.yaml` overlay (personal params override committed config)
- [x] e2_t4 — `mtt init [--template default|coding]`: write the starter `.mtt/config.yaml` (`coding` =
      feature/bugfix/refactor with a gated per-type DoD — a demo of the enforcement value)
- [~] e2_t5 — `internal/adapter/yaml`: implement `TaskStore` **and all capability interfaces** (the
      reference) — **ID minting** (`<prefix><N>`, **flat per-prefix** — not walking the parent chain —
      `max+1`, `O_EXCL`), deterministic serialization, atomic write (temp+rename), find the `.mtt/` root,
      load config + `.../yaml/CLAUDE.md`
- [x] e2_t6 — `internal/core`: the usecase layer (add/list/show/edit/close); parent-type validation;
      creates a logical task and asks `TaskStore` for the ID + `internal/core/CLAUDE.md`
- [x] e2_t7 — golden tests for task and config serialization (`-update` flag)
- [x] e2_t8 — `mtt add` (type from config, `--parent`, `--title`)
- [x] e2_t9 — `mtt list` (filters: status/type/parent; stable order) + `mtt show <id>`
- [~] e2_t10 — `mtt edit` / `mtt close` (change fields/status)
- [x] e2_t11 — first `testscript` e2e scenario: init → add → list → show

## e3 — Phase 2: hierarchy, dependencies, ready  `[~]`

(Dependencies — capability `DependencyStore`; if the adapter lacks it — `ErrUnsupported`.)

- [x] e3_t1 — `internal/core`: in-memory task index, hierarchy traversal
- [x] e3_t2 — `depends_on`: add/remove, existence validation (s005: `core.DependencyEditor`, `mtt dep add/rm`)
- [x] e3_t3 — dependency cycle detection (s005: `DepGraph.Reaches`/`Cycles`, cycle rejected on add, `dep list --cycles`)
- [x] e3_t4 — compute `ready` + the `mtt ready` command (s005: `core.Ready` conservative; `mtt ready`, `list --ready`)
- [x] e3_t5 — `mtt tree` (hierarchical output)
- [ ] e3_t6 — resolve `refs` of kind `task`/`comment` (existence verification) + backlinks in `show`

## e4 — Phase 3: flow enforcement + executable transitions (the killer feature)  `[~]`

(The type/flow model is introduced in phase 1; here — applying transitions and running commands.)
Single-edge `mtt status` shipped in **s006**; the meta-walk (`advance`/`start`/`done`) is **s007**.

- [x] e4_t1 — validate a status transition against the type's `transitions` (s006: `core.Transitioner`
      single-edge lookup; `ErrInvalidTransition` → exit 6, message lists allowed targets)
- [x] e4_t2 — the `Runner` port (in `core`) + `internal/adapter/exec` (run commands; per-command timeout,
      cwd=root, cross-platform shell seam); a fake for tests (s006)
- [x] e4_t3 — run a transition's `commands` in order, gating on exit codes (blocked on the first non-zero →
      exit 3, task unchanged); the `--no-run` flag (s006). Timeout is config-driven (`command_timeout`, 5m default)
- [x] e4_t4 — record the transition in the task's `history` (from→to, at, `by` from `--by`/`MTT_BY`, `role`
      from `--role`/`MTT_ROLE`, `checks`), append-only (s006; rides `Task.History` + `Update` — no `HistoryStore`
      port, GAP #1). `mtt show` renders the history section
- [~] e4_t5 — **s006** shipped `mtt status <id> <new>` (a single transition). The meta-walk `mtt advance
      <id> --to <status>` (progressing edges, stop at a fork, cycle guard, never into a different terminal);
      modes `--stop`(default)/`--atomic`/`--force`; `mtt start`/`mtt done` aliases → **s007**
- [ ] e4_t6 — `mtt types` (types/flow from config) + `mtt caps` (the current backend's capabilities)
- [ ] e4_t7 — `ready`/`list`/completeness — **by status category** (not by the literal `done`)

## e5 — Phase 4: comments (tree)  `[ ]`

- [ ] e5_t1 — `mtt comment add <id> [--reply <cid>]`
- [ ] e5_t2 — render the comment tree in `show`
- [ ] e5_t3 — **dogfooding**: move this tracker onto mtt itself

## Later (coarse)

- e6 — Phase 5: KB (`KnowledgeStore`) + text search; **versioned notes** (non-destructive; each save
  links to its predecessor — YAML implements, external backends use native versioning); resolve `refs`
  of kind `note`; `mtt check` (dangling references) + backlinks  _(KB is low priority; beads has an analog)_
- e7 — Phase 6: text/ASCII Gantt, richer list/query
- e8 — Phase 7: `mtt-ui` (optional, separate binary: web UI, Gantt SVG, KB browser)
- e9 — Phase 8: external indexer hook
- later — reconstruct the observed status graph from tasks' `history` (read-only aggregation);
  explicit flow versioning/migrations (the git history of config is enough for now)
- later — **actor profiles** (unify `by` + `role`): named profiles in config, each a `(by, role)` pair, one
  marked `default: true` (name-agnostic, mirrors the default-type/status marker) and applied when neither
  `--by`/`--role` nor `--profile` is given. Motivation: mtt is used mostly by **coding agents** that share the
  repo config with a human, so the agent's profile should be the ergonomic default (the human overrides with
  `--profile`/`--by`). `mtt profile add/list/rm` manages **only** the personal `.mtt/config.local.yaml`
  profiles and never touches shared project profiles (if a project defines any in `config.yaml`, they are
  read-only to the command). `--role` stays overridable per-invocation (one identity can switch hats).
  Supersedes the minimal s006 `author` seam (`author` = the default profile's `by`); forward-compatible.
  Extends the reserved `roles` section: role-aware semantics (a role tag on transitions,
  role-parameterization of `advance`/verb→target) build on profiles. Roles are semantic routing, not RBAC.
- later — **per-command timeout** overriding the global `command_timeout` (a fast command that overruns its
  own timeout signals a problem — fail fast, don't wait the global 5m). Attaches to the command (see the
  structured-command note below).
- later — **command placeholders**: template substitution in a transition's commands (e.g. auto-create a
  branch on `tbd → in_progress`: `git checkout -b task/{{.ID}}`). A small, safe vocabulary (`.ID`, `.Type`,
  `.From`, `.To`, …). **Caveat — shell injection:** substituted values are interpolated into `sh -c`, so
  either restrict to shape-safe fields (`id`/`type`/`status`) or shell-quote/escape arbitrary ones (`title`);
  never interpolate raw user text unquoted.
- later — rollback/compensation commands on transitions (`rollback`/`on_failure`), run in **reverse order**
  over the already-succeeded commands when a later command in the same pipeline fails (intra-pipeline
  compensation), and when an `--atomic`/multi-step `advance` aborts after side effects (undo a created branch,
  etc.). The executor's abort path is the hook.
- **structured commands (convergence of the three above):** per-command timeout + rollback + placeholders
  together argue for evolving `Transition.Commands []string` into a `[]Command` **value object**
  (`{run string; timeout?; rollback?}`) with placeholder expansion on `run`. Additive/back-compatible (a bare
  string maps to `{run: …}`), but it is a **domain-shape change in `pkg/mtt`** — plan it as one deliberate
  slice, not piecemeal.
- later — **`cancelled`-blocker semantics**: a `cancelled` (abandoned) `depends_on` currently unblocks its
  dependent (terminal by `kind`), which may be wrong — the dependent may need re-evaluation. Revisit with
  flow enforcement (s006), when terminal statuses become reachable. See DESIGN.md → "Dependencies".
- later — **re-parenting** (`mtt reparent`/`move`): change a task's `parent`; enabled by flat, position-free IDs.
- later — **tags**: a cross-cutting `[]string` label on tasks (reserved in the model now); filtering lands with `list`.
- later — **boards / views**: a query/view over tags/status/type (relates to `list` and `mtt-ui`); the backlog is such a view.
- later — **durable, git-independent audit of edits** (a change-log or field versioning for plain `edit`s,
  additive; `history` stays transition-only). (The subject-identity `By` source is now **resolved** — s006
  reads `--by` > `MTT_BY` > `config.local` `author`, to be subsumed by **actor profiles** above; only the
  edit-audit half remains open.)
- release — goreleaser, cross-platform binaries by tag
