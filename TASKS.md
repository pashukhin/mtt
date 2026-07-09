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
  codes 3/6, append `history`, `--role`/`--by`), config-driven `command_timeout`, `mtt show` history. Then
  **s006.5** attribution+sugar (e4_t8), **s006.7** current task (e4_t8a), **s007** structured commands (e4_t9:
  the `Command` VO with placeholders + per-command timeout), **s008** rollback/compensation (e4_t10: per-command
  `Command.Rollback`, reverse-over-succeeded, best-effort, exit 3 preserved). Next: **s008.5** dogfood enablers.
  The `advance`/`start`/`done` meta-walk (e4_t5) stays **PARKED** (single-edge is the norm).

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
- [x] e4_t8 — **attribution + verb sugar (s006.5)** — shipped: `--why` (durable reason; `HistoryEntry.Why`
      field + YAML DTO + `mtt show` rendering), `--who` (symmetric alias of `--by`, **mutually exclusive**),
      the `mtt <status> <id>` verb sugar (via **fallback-routing** in `root.RunE` — not command registration;
      routes when arg0 is a status of arg1's type flow, else `unknown command` exit 1; a real command wins a
      clash; single-edge, forward-compatible to advance), and **required-attribution**: a project-global
      `require: {who, why}` in the committed config (adapter `Settings.Require`; `config.local` may only
      **tighten** — captured pre-overlay + OR-combined) validated in `core.Transitioner` **before the gate**
      (fail fast; `--no-run` does not bypass), aggregating all missing fields into one `ErrMissingAttribution`
      → exit **2**. `-v`/`--log-file` moved to root-persistent; `--no-run` stays local to `mtt status`.
- [x] e4_t8a — **current task / working context (s006.7)** — shipped: kills id-repetition (git-`HEAD`-for-tasks).
      A `current` value in `config.local.yaml` (personal, gitignored), moved by a **transition property** —
      the additive `pkg/mtt.Transition.Current` field (`set|clear`, `CurrentAction` value object; validated in
      `Config.Validate`; default/`coding` templates set on →`in_progress`, clear on →`done`, leaving
      →`cancelled` alone). The pointer is a **capability port** `mtt.CurrentStore` (justified now — `current`
      is **non-embeddable**, the GAP #1 case that earns a port even for YAML; `yaml.NewCurrent` writes the
      `current:` key of `config.local` via a comment-preserving `yaml.Node`); an external adapter maps it to a
      native assignee. The **CLI applies** set/clear after a move (reading the edge via the new pure primitive
      `Type.FindTransition`); `core.Transitioner` untouched. `mtt use [<id>] [--clear]` (set/show/clear). An
      **omitted id** resolves to `current` for `status`/`mtt <status>`/`show`/`edit` only (order: explicit id >
      current; stale/absent → actionable error); never for `list`/`tree`/`dep`/`ready`. `CapCurrent` +
      `Capabilities()` deferred to `mtt caps` (e4_t6). Follow-up think-items are in **Later (think)** below
      (separate `current`/`use` commands; description-on-move; current for all single-task ops; per-agent
      current; multi-assignee providers; the `advance` reuse seam).
- [x] e4_t9 — **structured commands (s007)** — shipped: `Transition.Commands` `[]string` → the `Command`
      value object (`{Run, Timeout}`, pure `pkg/mtt`) with **placeholder** expansion on `run`
      (`.ID`/`.Type`/`.From`/`.To`, a structural `text/template` whitelist — no free text, no shell-quoting) in
      `core.Transitioner` (pkg/mtt stays template-agnostic) + a **per-command timeout** overriding
      `command_timeout` resolved in the exec `Runner` (global as fallback). Back-compat: a bare YAML scalar ⇒
      `{run: …}` via a custom `ymlCommand.UnmarshalYAML` (scalar or `{run, timeout}` map). `Runner.Run` now
      takes `[]mtt.Command` (Run expanded at the boundary; `Check.Cmd` records the expanded command). An
      expansion error is exit 1 (not `ErrBlocked`). The enabler for "the agent works in task terms" (task-aware
      transitions, e.g. `git checkout -b task/{{.ID}}`). `rollback?` stays additive → s008 (e4_t10)
- [x] e4_t10 — **rollback/compensation (s008)** — shipped: additive per-command `Command.Rollback *Command`
      (a leaf compensator); on an **intra-pipeline** failure `core.Transitioner` runs the succeeded commands'
      rollbacks in **reverse** via the exec `Runner.Compensate` (best-effort) — outcome stays `ErrBlocked`
      (exit 3), task unchanged, **no history**; rollbacks expanded eagerly (bad rollback template → exit 1
      before side effects); `mtt types` shows `↩ <rollback>`. **Later:** `--atomic`/multi-step abort across
      several edges (still parked); second-level compensation rejected by `Valid()`

## e5 — Phase 4: dogfood → references → comments → profiles (regrouped 2026-07-05)  `[ ]`

Reordered so mtt self-hosts as soon as flow orchestration is complete (after e4), ahead of references and
comments (which enrich a full self-host but don't enable it). See sessions/README.md → "Roadmap regrouped".

- [x] e5_t1 — **dogfood enablers (chore, s008.5)** — shipped: `mtt rm <id>` (hard-delete via base-port
      `TaskStore.Delete`; `core.Remover` reject-if-referenced + `--force`; not-found → **uniform exit 4**;
      explicit id only; clears a stale `current` pointer), `--depends-on` on `add` (validated + deduped in
      `core.Adder`), packaging (`make install` ldflags version stamping + `make smoke`). Version `0.8.0-dev` →
      `0.8.5-dev`. Spec/plan: `docs/superpowers/{specs,plans}/2026-07-07-session-008.5-dogfood-enablers*`
- [x] e5_t1a — **priorities + roadmap (s008.6)** — ✅ **shipped (s008.6)**: a closed `Priority` VO
      (`high|medium|low`; empty=medium in order, off-disk; rides `Task` + `Update`, no port) with `--priority` on
      `add`/`edit`/`list`, `--sort priority`, `priority` in `show`/`taskJSON`; and **`mtt roadmap [--json]`** — a
      pure-core `Roadmap(tasks,cfg) []RoadmapEntry` (priority-guided Kahn: dependency hard, priority soft;
      cycle-safe; `ready`/`blocked_by` annotations, own non-terminal DAG, shared `terminalSatisfied`). The
      agent-queryable execution order that motivates dogfood. Version `0.8.5-dev` → `0.8.6-dev`. Spec/plan:
      `docs/superpowers/{specs,plans}/2026-07-07-session-008.6-priorities-roadmap*`
- [x] e5_t1b — **tags (s008.7)** — ✅ **shipped**: pure `pkg/mtt` tag vocabulary (`NormalizeTag`/`ExtractTags`,
      **Unicode** `\pL\pN._-` charset, no NFC) + `Task.Tags` a normalized+sorted **set** (rides the field +
      `Update`, **no new port** — GAP #1). `core.Adder` unions explicit `--tag` with `#hashtags` from title/
      description; `core.Editor` reconciles on a **text-delta** (drop tags whose `#hashtag` left the text, keep
      manual — the no-provenance corner is documented); `core.TagEditor` (`tag add/rm`, idempotent, **rm
      guard** on a text-anchored tag, atomic multi-tag, wraps `ErrNotFound` → exit 4); `ListFilter.Tags` +
      `Match` OR-within (`anyOrEmptyIntersect`); cli `mtt tag add/rm` (variadic), `--tag` on `add`/`list`/`tree`,
      tags in `show`/`--json`. **`#hashtags` in title/description are the primary path; `tag add/rm` secondary.**
      Version `0.8.6-dev` → `0.8.7-dev`. Spec/plan:
      `docs/superpowers/{specs,plans}/2026-07-09-session-008.7-tags*`. `boards/views` over tags stay Later.
- [x] e5_t1c — **batch & pipeline (s008.9)** — ✅ **shipped**: makes mtt Unix-composable. A reusable **task-set
      selector** (`internal/cli/selector.go`, `selectTaskIDs`) shared by every set-operating command — explicit
      IDs | a `--filter` (the `list` filters `--status/--type/--kind/--parent/--priority/--tag/--ready` over
      `Select`/`Match`) | **stdin `-`** (IDs one per line), **mutually exclusive** (>1 or 0 = usage error;
      present-but-empty = no-op exit 0), never resolves `current`. **`--ids`** output on `list`/`ready` (one ID
      per line, `⊕ --json`). Applied to `tag add/rm` and `rm` (no gates). Bulk mutations share `runBulk`
      (`bulk.go`): best-effort per item, report (stdout summary + stderr per-item / `--json` per-item array),
      **generic exit 1** on any failure (the aggregate never `%w`-wraps a per-item sentinel), and `--dry-run`
      preview. `tag` argument layout is **context-sensitive** (a `-`/filter marker → positionals are tags, tasks
      from the selector; else the single `tag add <id> <tag>…`). Bulk `rm` is **subgraph-aware**
      (`core.Remover.RemoveMany`: referents inside the deletion set are ignored, so `rm <epic> <child>` needs no
      `--force`; `Remove` is a thin wrapper — single `rm <id>` keeps exit-4). Spec/plan:
      `docs/superpowers/{specs,plans}/2026-07-09-session-008.9-batch*`. Version `0.8.7-dev` → `0.8.9-dev`. Bulk
      `status`/verbs/`edit`/`dep` stay **later** (gates + partial-success + atomicity).
- [ ] e5_t1d — **dogfood hardening (chore, s008.97)** — added 2026-07-10 from the deep-analysis notes
      (`docs/superpowers/notes/2026-07-09-positioning-and-agent-ux-analysis.md` U*,
      `…-s009-readiness-and-architecture-audit.md` A*): a blocked gate surfaces the failing command's output
      tail (or hints `-v`/`--log-file`) [U2]; yaml `List`/`Get` errors name the offending file, corrupt/
      zero-byte task files locked by tests [A1]; map `Status.Default` in the YAML DTO (today silently dropped)
      [A2]; `add --json` emits the created task [U3]; help discoverability (`status [<id>]`, sugar in root
      help, `mtt init` hint) + a tagline naming the gate feature [U4/U5]; `rm -rf bin/.mtt`; split the
      per-command-timeout e2e out of the git-gated script + a `-v` streaming test.
- [ ] e5_t2 — **dogfooding (s009)**: `mtt init` this repo, a config whose gates are task-aware, migrate the
      forward backlog onto mtt. **Starts by reconciling the spec with recorded decision A** (full session flow
      `tbd → speccing → planning → in_progress → review → done`; branch + `current: set` on `→ speccing` via
      the idempotent `git switch -c … || git switch …`; `make check` on `→ review`/`→ done`) — see the audit
      note items S1–S5.
- [ ] e5_t2a — **release positioning (chore, s009.5)**: README/DESIGN "why not harness hooks?" + refresh the
      2026 competitive scan + a copy-pasteable AGENTS.md adoption snippet (R0–R3); ship `pkg/mtt.ErrUnsupported`
      (the `Delete` godoc already references it — A7); decide the stale-`current` exit code (A5);
      config-review-as-code + Windows-honesty doc lines. Then the user-triggered **`v0.9.0`** tag.
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
- later (think) — **dangerous ops must mandate `--who`/`--why`**: flow-bypassing / risk actions — `--no-run`
  over a transition whose edge has a **non-empty** command list (skipping a real gate), and later `--force` /
  atomic aborts after side effects — should **force** attribution regardless of the project `require` setting
  (you may skip the gate, but you must sign for it). Independent of the global `require` knob; ties into the
  s007+ structured-commands / rollback risk surface. Surfaced in the s006.5 brainstorm. See DESIGN.md →
  required-attribution deferred note. **Elevated (2026-07-09): first candidate once s009 self-host lands** —
  dogfood puts agents under real gates, making `--no-run` the legal bypass of a red `make check`; the signature
  requirement is exactly what this item was written for. Small slice: force `--who`/`--why` when `--no-run`
  skips a transition whose edge has a **non-empty** command list, regardless of the project `require` setting.
- later (think) — **subagent identity under multi-agent access**: roles/RBAC are pointless unless we can
  distinguish subagents acting with **different** roles — that is what our RBAC ultimately hinges on. Figure
  out the identity mechanism (per-agent `config.local`? an env/handshake/token? a provider-supplied
  identity?) — this is the **real precondition** for the parked roles/profiles work (e5_t5) and for `By`
  attribution to mean more than a self-declared string. Decide it before reviving roles.
- later (think) — **node-level status actions** (custom verbs): generalize executable pipelines from **edges**
  (transitions) to **nodes** — a status may carry **named, rollback-able action pipelines** that run **without
  changing status** (intermediate `commit`, `build`, `check`), each invoked as a custom verb `mtt <action>
  <id>`. The shared primitive (a named rollback-able pipeline) hangs on an edge *or* a node; completes the
  "all shell in the flow / pure task terms" story. **Blocked on** structured commands + rollback (reuses the
  `Command` VO + compensation) and the argument-resolution grammar (custom verbs collide with real commands /
  the status-sugar — `mtt check` is reserved for ref-checking); a non-transition action's audit ties to
  edit-audit. **Open: release-needed?** Lean *no* (an agent commits WIP via plain `git` while `in_progress` —
  completeness polish, not the minimum); revisit once structured commands land. See DESIGN.md → Flow seam.
- later (think) — **`current` (working context) follow-ups (s006.7)**: (a) **separate `mtt current` (show) +
  `mtt use <id>` (set) + clear** — semantically cleaner than one overloaded `use` command; an ergonomics pass.
  (b) **resolve `current` for *all* single-task ops, including reads** (`tree <id>`, `dep list`; today only
  `show`/`edit`/`status`/sugar resolve it) — generalize deliberately, not verb-by-verb. (c) **per-agent
  current** — a shared checkout with multiple agents has one `config.local` = one `current` → collision;
  blocked on the subagent-identity item above (fine for solo / one-agent-per-checkout meanwhile). (d)
  **multi-assignee providers** — what "my current" means when an external backend allows several assignees
  (team work); YAML is single-valued, decide with the first external adapter. See DESIGN.md → "Working context:
  the current task".
- later (think) — **`current` vs roles: is "take into work" role-contextual?** (surfaced s006.7). Yes —
  `current` is a lightweight single-value form of **assignee** ("the task *I* am actively working"), and "I" is
  an actor/role: an *implementer* taking `tbd→in_progress` claims *their* current, whereas a *reviewer* moving
  `→review` claims *theirs* (different semantics on the same task). **What follows from roles being PARKED
  (e4_t5 / e5_t5):** what shipped — one global `config.local` pointer, role-agnostic — is a **degenerate
  single-role projection** of a per-actor concept (the correct default for solo / one-agent-per-checkout).
  When roles unpark, `current` likely generalizes to a **per-actor / per-role** pointer (keyed by identity),
  and `Transition.Current` may become **role-conditioned** (which role's current does an edge move? — composes
  with the parked roles-on-edges). This is the same need as [per-agent current] (subagent-identity item) and
  [multi-assignee providers] (a provider's assignee *is* role-scoped ownership). **Do NOT build role-conditioning
  now:** unconditional set/clear is the right single-role default and the design is forward-compatible (the
  field is name-agnostic; role-conditioning + a `SetCurrent(id, actor)` port param are additive). **Decide when
  roles unpark** — this is a precondition to consider *with* subagent-identity, not before. See DESIGN.md →
  "Roles — a seam" and "Working context: the current task".
- later (think) — **scale / algorithmic-complexity stress test** (surfaced s006.7 review). Does execution cost
  grow badly with **thousands of tasks** and **large, complex status graphs** (tens–hundreds of statuses, dense
  edges, deep chains, high fan-out)? Audit the complexity and set soft budgets before dogfooding at volume.
  **Suspected hotspots to check first:** (1) the dominant cost is likely **I/O, not algorithm** — the YAML
  `TaskStore.List()` reads + parses every `.mtt/tasks/*.yaml` on *every* command (list/tree/ready/dep) with no
  cache/index → O(N) disk + YAML per invocation. (2) **linear scans inside per-task loops** — `StatusKind` /
  `FindTransition` / `TypeByName` are linear and are called inside loops (`Ready`, `Match --kind`,
  `Config.Validate`), so a complex flow × many tasks creeps to O(N·S) / O(N·E). (3) **graph rebuilds** —
  `DependencyEditor` rebuilds `DepGraph` (O(V+E)) per `dep add` cycle-check; batch ops (s008.9) and the future
  `advance`/`ResolvedFlow` walk (s007) multiply this. **Likely fix if bad = a resolved index** (build map's
  `status→kind`, `(from,to)→edge`, adjacency once per command and reuse — O(1) lookups), **not a graph library**
  — the graphs are small/sparse and a dep (gonum/graph) buys little against KISS + "no heavy deps without
  reason"; revisit a lib only if measurement forces it. **How to run it (esp. graphs — the hard part):** a
  **task generator** minting N tasks (100 / 1k / 10k) into a temp `.mtt/`, plus a **flow generator**
  parameterized by #statuses / edge-density / depth / fan-out that emits *valid-but-stressful* topologies
  (respecting the initial/active/terminal invariants) and adversarial ones (long chains, dense DAGs, near-cycles);
  then `testing.B` benchmarks + `-benchmem` + pprof (CPU/alloc) over `list`/`tree`/`ready`/`dep`/`status` and the
  validate/`FindTransition`/(future) walk paths — measure the **scaling exponent** (cost vs N and vs graph size),
  flag any super-linear growth. The benchmarks then double as **regression guards** for s007 / s008.9. **Timing:**
  a cheap first pass (List I/O scaling + the linear-scan-in-loop audit) can happen anytime; the full graph
  stress is most valuable **after** s007 (`ResolvedFlow`/advance — the real graph-algorithm surface) and s008.9
  (batch — where per-op rebuilds compound), around s009 dogfood (real volume).
  **Measured (2026-07-09, the cheap first pass, `0.8.9-dev`, generated flat labs):** suspicion (1) — I/O
  dominance — is **retired**: `list`/`ready`/`tree`/`dep`/`show` scale linearly and stay cheap (≈30 ms at
  N=1000, ≈120 ms at N=5000); `status` (the gate path) is O(1) (≈3 ms at any N). The one super-linear hotspot
  is **`roadmap`**: 8 ms / 55 ms / **1.02 s** at N=100/1000/5000 (~quadratic) — not the graph build but the
  documented Kahn "re-sort the available set before each pop" (O(N²·log N) when most nodes are independent).
  Harmless at dogfood volume (10² tasks ⇒ sub-ms); the fix when needed is a heap (`container/heap` keyed by
  `(effective rank, recency)`) replacing the per-pop `sort.SliceStable`. The graph-topology stress half (dense
  flows, deep chains, fan-out) remains open, still best after `advance`/`ResolvedFlow` unparks. Details:
  `docs/superpowers/notes/2026-07-09-s009-readiness-and-architecture-audit.md`.
- ~~later (think) — **show the status/transition `description` on a successful move**~~ — **SHIPPED (s008.95,
  "flow guidance on entry")**: `mtt status`/sugar print the traversed edge's + destination status's
  `description` and the onward moves (`next:`) on stdout after a move; `mtt show` surfaces the current status's
  guidance. Kept for the record; see
  `docs/superpowers/specs/2026-07-09-flow-guidance-on-entry-design.md`.
- later (think) — **`advance` reuse seam for `current` set/clear**: s006.7 applies `Transition.Current` in the
  CLI (single-edge `status`/sugar share `runTransition`). When the parked multi-edge `advance` (e4_t5) unparks,
  extract a shared **core** "apply edge effects" step so `Transitioner` and `Advancer` both move the pointer
  (avoids the DRY split option ii accepts now). Revisit-at-the-second-caller. See DESIGN.md → "Working context".
- **now scheduled (regrouped 2026-07-05):** attribution + verb sugar (`--why`/`--who` + `mtt <status> <id>`)
  → **e4_t8 / s006.5**; current task / working context → **e4_t8a / s006.7**; structured commands
  (placeholders + per-command timeout) → **e4_t9 / s007**; rollback/compensation → **e4_t10 / s008**; dogfood enablers (`mtt rm`, `--depends-on`) + packaging →
  **e5_t1 / s008.5**; **tags** (+`#hashtags`) → **e5_t1b / s008.7**; **batch & pipeline** (task-set selector +
  `--ids` + stdin) → **e5_t1c / s008.9**; actor profiles → **e5_t5 / s012**. `advance`/`start`/`done`/`cancel` + modes +
  roles-on-edges are **parked** (on-demand — see e4_t5). Design detail: DESIGN.md → "Advancing through the
  flow" (parked), "Seam (deferred): structured commands", "Direction (deferred): actor profiles", rollback seam.
- later — **`cancelled`-blocker semantics**: a `cancelled` (abandoned) `depends_on` currently unblocks its
  dependent (terminal by `kind`), which may be wrong — the dependent may need re-evaluation. Revisit with
  flow enforcement (s006), when terminal statuses become reachable. See DESIGN.md → "Dependencies".
- later (think) — **monotonic / never-reuse ID minting** (surfaced s008.5 impl review). The YAML `mint` uses
  `max+1` per prefix with no high-water mark, so deleting the **highest-numbered** task frees its id; a later
  `add` reuses it and silently re-points any dangling `depends_on`/`parent` (left by `mtt rm --force`) at the
  new, unrelated task — a data-integrity hazard. Latent (needs `--force` + delete-the-max + a later create) and
  rooted in the mint scheme, not `rm`. **Fix options:** a persistent per-prefix counter (a small committed
  seq file, or a `next:` in config) that only increments; or record deletions (a tombstone) so `max+1` skips
  freed ids. Interacts with the known cross-branch collision limitation (both are about id allocation) and with
  re-parenting — worth a single minting brainstorm. Documented meanwhile as a `--force` caveat (DESIGN.md,
  CLI_REFERENCE.md). See DESIGN.md → "Deletion (`mtt rm`)".
- later (think) — **lost-update / write-concurrency contract** (surfaced by the 2026-07-09 architecture audit).
  Every mutation is Get → mutate → whole-file `Update` (last writer wins; the stat-then-write TOCTOU is
  documented-accepted): two agents transitioning/tagging the **same task** silently lose one `history` entry —
  or the status move itself — with no error. Fine for solo / one-agent-per-checkout (the s009 mode); **the
  trigger is the second writing agent on one store.** Fix direction: an optimistic-concurrency token on
  `Update` (compare-and-swap on `Updated` or a content hash) + the reserved `ErrConflict`; or per-file lock
  files. **Decide together with monotonic minting (above) and the cross-branch add/add collision** — all three
  are one "concurrent store semantics" brainstorm, not three separate fixes. See
  `docs/superpowers/notes/2026-07-09-s009-readiness-and-architecture-audit.md` (A3).
- pre-`v0.9.0` — **ship `pkg/mtt.ErrUnsupported`**: the `TaskStore.Delete` godoc already instructs an adapter
  that cannot hard-delete to return it, but the sentinel does not exist (a dangling contract reference — GAP
  #3's "reserve when first thrown" is overtaken by the godoc). Additive: the sentinel + a deliberate
  `exitCode` mapping decision + a CLI_REFERENCE note. See the audit note (A7).
- later — **re-parenting** (`mtt reparent`/`move`): change a task's `parent`; enabled by flat, position-free IDs.
- **tags** — **shipped s008.7** (e5_t1b): CRUD (`tag add/rm`, `--tag` on `add`) + `list/tree --tag` filter over
  `Task.Tags`, plus `#hashtag` extraction (the **primary** authoring path) from title/description with
  write-time reconciliation and a `tag rm` guard on text-anchored tags.
- later — **boards / views**: a query/view over tags/status/type (relates to `list` and `mtt-ui`); the backlog is such a view.
- later — **durable, git-independent audit of edits** (a change-log or field versioning for plain `edit`s,
  additive; `history` stays transition-only). (The subject-identity `By` source is now **resolved** — s006
  reads `--by` > `MTT_BY` > `config.local` `author`, to be subsumed by **actor profiles** above; only the
  edit-audit half remains open.)
- release — goreleaser, cross-platform binaries by tag
