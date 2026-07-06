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
- [~] e4_t5 — **s006** shipped `mtt status <id> <new>` (a single transition — the norm). **PARKED
      (on-demand):** the meta-walk `mtt advance <id> --to <status>` (progressing edges, stop at a fork, cycle
      guard, never into a different terminal), modes `--stop`/`--atomic`/`--force`, `mtt start`/`done`/`cancel`
      aliases, and **roles-on-edges**. Deferred until a flow actually branches (single-edge covers ~all cases;
      role-tagged edges are near-RBAC and premature — identity/role may later come from an external provider).
- [ ] e4_t6 — `mtt types` (types/flow from config) + `mtt caps` (the current backend's capabilities)
- [ ] e4_t7 — `ready`/`list`/completeness — **by status category** (not by the literal `done`)
- [ ] e4_t8 — **attribution + verb sugar (s006.5)** — a release-complete attribution slice (no roles/profiles
      needed): `--why` (a durable free-text reason recorded in `history`; new `HistoryEntry.Why` field + DTO +
      `show` rendering — "who + why moved the task"), `--who` (a symmetric alias of `--by`), the
      `mtt <status> <id>` verb sugar (via **fallback-routing** on an unknown first arg — not dynamic command
      registration; single-edge; forward-compatible to advance later; a status colliding with a real command
      name loses to the command), and **required-attribution**: a project-global `require: {who, why}` in the
      committed config (adapter-level `Settings`, like `command_timeout`; `config.local` may only **tighten**,
      not relax) — **validated before the gate runs** (fail fast) and **not** bypassed by `--no-run`/`--force`;
      on a violation, **aggregate all missing fields into one error** (agent fixes them in one shot) and exit
      **2 (usage)**. This is a natural release point (full release tooling — goreleaser/tags — stays later).
- [ ] e4_t9 — **structured commands (s007)**: evolve `Transition.Commands` `[]string` → a `Command`
      value object (`{run, timeout?}`) with **placeholder** expansion on `run` (`.ID`/`.Type`/`.From`/`.To`;
      shell-quote/restrict — injection caveat) + **per-command timeout** overriding `command_timeout`.
      Additive/back-compatible (bare string ⇒ `{run: …}`), but a **domain-shape change** in `pkg/mtt`. This
      is the enabler for "the agent works in task terms" (task-aware transitions, e.g. branch creation)
- [ ] e4_t10 — **rollback/compensation (s008)**: reverse-order compensating commands on a failed pipeline
      or (later) an `--atomic`/multi-step abort after side effects; the executor's abort path is the hook

## e5 — Phase 4: dogfood → references → comments → profiles (regrouped 2026-07-05)  `[ ]`

Reordered so mtt self-hosts as soon as flow orchestration is complete (after e4), ahead of references and
comments (which enrich a full self-host but don't enable it). See sessions/README.md → "Roadmap regrouped".

- [ ] e5_t1 — **dogfood enablers (chore, s008.5)**: `mtt rm <id>` (hard-delete, distinct from `cancel`),
      `--depends-on` on `add`, packaging (`make install` → `go install ./cmd/mtt` + a smoke test)
- [ ] e5_t1b — **tags (s008.7)** — needed to organize the self-hosted backlog: `mtt add --tag x`,
      `mtt tag add/rm <id> <tag>` (rides the reserved `Task.Tags` field + `Update`, no new port — like
      `depends_on`), and a `Tags` dimension in `ListFilter` for `mtt list/tree --tag` (reuses `Match`/`Select`
      — cheap). Plus **`#hashtag` extraction** from title/description on `add`/`edit` (less verbose than
      repeated `--tag`). **Brainstorm decisions:** (a) derived-on-read (tags = explicit ∪ parsed-from-text —
      single source, no staleness) vs extract-to-field (simpler, but stale on later edits); (b) which fields
      to scan — title reliably, description cautiously/opt-in (‌`#` is common in prose/code: `#!`, `#include`,
      `##` headings, URL anchors); (c) the token rule + case normalization. `boards/views` over tags stay Later.
- [ ] e5_t2 — **dogfooding (s009)**: `mtt init` this repo, a config whose gates are task-aware (branch on the
      `→ in_progress` edge via a placeholder, `make check` on `→ done`), migrate the backlog onto mtt
- [ ] e5_t3 — references (**s010**): `mtt ref add/rm/list`, backlinks; resolve `task`/`comment` refs (link a
      task ↔ its PR/spec)
- [ ] e5_t4 — comments (**s011**): `mtt comment add <id> [--reply <cid>]` (tree) + render in `show`
- [ ] e5_t5 — **actor profiles (s012)**: named `(by, role)` profiles in `config.local`, one `default: true`
      (= the coding agent), managed by `mtt profile add/list/rm` (local-only); subsumes the s006 `author` seam
- [ ] e5_t6 — `mtt init --template coding` demo (feature/bugfix/refactor with task-aware gated DoD) — fully
      powered once structured commands (e4_t9) land

## Later (coarse)

- e6 — Phase 5: KB (`KnowledgeStore`) + text search; **versioned notes** (non-destructive; each save
  links to its predecessor — YAML implements, external backends use native versioning); resolve `refs`
  of kind `note`; `mtt check` (dangling references) + backlinks  _(KB is low priority; beads has an analog)_
- e7 — Phase 6: text/ASCII Gantt, richer list/query
- e8 — Phase 7: `mtt-ui` (optional, separate binary: web UI, Gantt SVG, KB browser)
- e9 — Phase 8: external indexer hook
- later — reconstruct the observed status graph from tasks' `history` (read-only aggregation);
  explicit flow versioning/migrations (the git history of config is enough for now)
- later — **export the status flow as Graphviz** (`mtt types --dot` / `mtt flow --graphviz`): render a
  type's flow — statuses (by `kind`) + transitions, annotated with attached `commands`/roles — as DOT for
  visualization. Cheap read-only view; pairs well with the observed-graph reconstruction above.
- later (think) — **argument-resolution grammar**: generalize the s006.5 `mtt <status> <id>` fallback into a
  coherent scheme for resolving positional args (command / status / role / id / …). Is `mtt <role> …` a
  form? What's the precedence and disambiguation when arg0 (or arg1+) could be several kinds? Decide the
  grammar **before** adding more sugar forms, so the surface stays predictable.
- later (think) — **subagent identity under multi-agent access**: roles/RBAC are pointless unless we can
  distinguish subagents acting with **different** roles — that is what our RBAC ultimately hinges on. Figure
  out the identity mechanism (per-agent `config.local`? an env/handshake/token? a provider-supplied
  identity?) — this is the **real precondition** for the parked roles/profiles work (e5_t5) and for `By`
  attribution to mean more than a self-declared string. Decide it before reviving roles.
- **now scheduled (regrouped 2026-07-05):** attribution + verb sugar (`--why`/`--who` + `mtt <status> <id>`)
  → **e4_t8 / s006.5**; structured commands (placeholders + per-command timeout) → **e4_t9 / s007**;
  rollback/compensation → **e4_t10 / s008**; dogfood enablers (`mtt rm`, `--depends-on`) + packaging →
  **e5_t1 / s008.5**; **tags** (+`#hashtags`) → **e5_t1b / s008.7**; actor profiles → **e5_t5 / s012**. `advance`/`start`/`done`/`cancel` + modes +
  roles-on-edges are **parked** (on-demand — see e4_t5). Design detail: DESIGN.md → "Advancing through the
  flow" (parked), "Seam (deferred): structured commands", "Direction (deferred): actor profiles", rollback seam.
- later — **`cancelled`-blocker semantics**: a `cancelled` (abandoned) `depends_on` currently unblocks its
  dependent (terminal by `kind`), which may be wrong — the dependent may need re-evaluation. Revisit with
  flow enforcement (s006), when terminal statuses become reachable. See DESIGN.md → "Dependencies".
- later — **re-parenting** (`mtt reparent`/`move`): change a task's `parent`; enabled by flat, position-free IDs.
- **tags** — **scheduled s008.7** (e5_t1b), pulled forward for backlog management (was "later"): CRUD +
  `list/tree --tag` filter over the reserved `Task.Tags` field, plus `#hashtag` extraction from title/description.
- later — **boards / views**: a query/view over tags/status/type (relates to `list` and `mtt-ui`); the backlog is such a view.
- later — **durable, git-independent audit of edits** (a change-log or field versioning for plain `edit`s,
  additive; `history` stays transition-only). (The subject-identity `By` source is now **resolved** — s006
  reads `--by` > `MTT_BY` > `config.local` `author`, to be subsumed by **actor profiles** above; only the
  edit-audit half remains open.)
- release — goreleaser, cross-platform binaries by tag
