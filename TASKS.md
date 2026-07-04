# TASKS

Bootstrap tracker until self-hosting. Once tasks + hierarchy + dependencies exist (end of phase 4),
mtt's development moves onto mtt itself, and this file is frozen.

The identifiers (`e{N}_t{M}`) are illustrative bootstrap numbering only, predating the flat-ID decision
(mtt's actual scheme is flat per-prefix, e.g. `e1`/`t17` — see DESIGN.md); not re-derived here.
Order and architecture — in [DESIGN.md](DESIGN.md); rules — in [AGENTS.md](AGENTS.md).

Legend: `[ ]` todo · `[~]` in progress · `[x]` done.

---

## e1 — Phase 0: project scaffold  `[x]`

- [x] e1_t1 — git init, Go module `github.com/pashukhin/mtt`, `main` branch
- [x] e1_t2 — CLI skeleton: `cmd/mtt` + `internal/cli` (root + `version`) + a test
- [x] e1_t3 — gate: Makefile `make check`, `.golangci.yml` (v2), `.gitignore`
- [x] e1_t4 — CI: `.github/workflows/ci.yml` (the same gate)
- [x] e1_t5 — DESIGN.md, AGENTS.md, README.md
- [x] e1_t6 — guards: principles (SOLID/DRY/KISS/TDD), hierarchical CLAUDE.md, superpowers

## e2 — Phase 1: `pkg/mtt` contract, config, `mtt init`, YAML adapter, core, commands  `[ ]`

Test-first, one subtask per branch+PR. **Start with planning** (see NEXT_SESSION.md); the breakdown
below is a guide — planning refines it. Invariants: types/hierarchy come from config (no literals in
code); the **adapter** mints the ID/slug; exactly one type is marked `default` (no literal `task`); each
flow has ≥1 status of each kind (initial/active/terminal), `kind` by topology; storage is behind a port;
`core` doesn't import `adapter/*`.

- [ ] e2_t1 — plan phase 1 (superpowers), reconcile with the DESIGN.md invariants
- [ ] e2_t2 — `pkg/mtt` **pure** contract (no serialization tags, no `prefix`): `Config`, `Type`
      (`name/description/parents/default/flow`), `Flow`, `Status` (`name/kind/description`; `kind` a
      `StatusKind` **value object**), `Transition` (`from/to/description/commands`); `Task` (with
      `history[]`+`refs[]`), `Comment` (`refs[]`), `Ref` {kind,id,label}; the history entry reserves `role`
      — the roles seam; the base `TaskStore` + optional capability interfaces (`HistoryStore`,
      `DependencyStore`, `CommentStore`, `SearchStore`), `Capabilities()`, `ErrUnsupported`; references by
      identity, back-refs computed + `pkg/mtt/CLAUDE.md`
- [ ] e2_t3 — config: type (`name/description/parents/default/statuses(with kind)/transitions`; `prefix`
      is a YAML-adapter field, held in the adapter DTO), **structural name-agnostic** invariant validation
      (kind↔topology; ≥1 of each kind; no 2-status flow; multiple initials ok; per-flow status identity, no
      cross-flow transitions; at-most-one `default` at the domain / exactly-one at the YAML provider; prefix
      present+unique in the adapter); the default template (via DTO→domain mapping); config load merges an
      optional gitignored `.mtt/config.local.yaml` overlay (personal params override committed config)
- [ ] e2_t4 — `mtt init [--template default|coding]`: write the starter `.mtt/config.yaml` (`coding` =
      feature/bugfix/refactor with a gated per-type DoD — a demo of the enforcement value)
- [ ] e2_t5 — `internal/adapter/yaml`: implement `TaskStore` **and all capability interfaces** (the
      reference) — **ID minting** (`<prefix><N>`, **flat per-prefix** — not walking the parent chain —
      `max+1`, `O_EXCL`), deterministic serialization, atomic write (temp+rename), find the `.mtt/` root,
      load config + `.../yaml/CLAUDE.md`
- [ ] e2_t6 — `internal/core`: the usecase layer (add/list/show/edit/close); parent-type validation;
      creates a logical task and asks `TaskStore` for the ID + `internal/core/CLAUDE.md`
- [ ] e2_t7 — golden tests for task and config serialization (`-update` flag)
- [ ] e2_t8 — `mtt add` (type from config, `--parent`, `--title`)
- [ ] e2_t9 — `mtt list` (filters: status/type/parent; stable order) + `mtt show <id>`
- [ ] e2_t10 — `mtt edit` / `mtt close` (change fields/status)
- [ ] e2_t11 — first `testscript` e2e scenario: init → add → list → show

## e3 — Phase 2: hierarchy, dependencies, ready  `[ ]`

(Dependencies — capability `DependencyStore`; if the adapter lacks it — `ErrUnsupported`.)

- [ ] e3_t1 — `internal/core`: in-memory task index, hierarchy traversal
- [ ] e3_t2 — `depends_on`: add/remove, existence validation
- [ ] e3_t3 — dependency cycle detection
- [ ] e3_t4 — compute `ready` + the `mtt ready` command
- [ ] e3_t5 — `mtt tree` (hierarchical output)
- [ ] e3_t6 — resolve `refs` of kind `task`/`comment` (existence verification) + backlinks in `show`

## e4 — Phase 3: flow enforcement + executable transitions (the killer feature)  `[ ]`

(The type/flow model is introduced in phase 1; here — applying transitions and running commands.)

- [ ] e4_t1 — validate a status transition against the type's `transitions` (+ show `description`)
- [ ] e4_t2 — the `Runner` port (in `core`) + `internal/adapter/exec` (run commands; timeout,
      cwd=root); a fake for tests
- [ ] e4_t3 — run a transition's `commands` in order, gating on exit codes (the transition is blocked on
      the first non-zero); the `--no-run` flag
- [ ] e4_t4 — record the transition in the task's `history` (from→to, at, by, `role` from
      `--role`/`MTT_ROLE`, `checks` results), append-only (capability `HistoryStore`; if absent — graceful degradation)
- [ ] e4_t5 — `mtt advance <id> --to <status>` — the meta-walk to a target (progressing edges, stop at a
      fork, cycle guard, never into a different terminal); modes `--stop`(default)/`--atomic`/`--force`;
      `mtt start`/`mtt done` — aliases; `mtt status <id> <new>` — a single transition
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
- later — role-aware command semantics: a `roles` section in config, a role tag on transitions,
  role-parameterization of `advance`/verb→target (the seam is already laid: `role` in history + `--role`;
  roles are semantic routing, not RBAC)
- later — rollback/compensation commands on transitions (`rollback`/`on_failure`) run when an `--atomic`
  or multi-step `advance` aborts after side effects (undo a created branch, etc.)
- later — **re-parenting** (`mtt reparent`/`move`): change a task's `parent`; enabled by flat, position-free IDs.
- later — **tags**: a cross-cutting `[]string` label on tasks (reserved in the model now); filtering lands with `list`.
- later — **boards / views**: a query/view over tags/status/type (relates to `list` and `mtt-ui`); the backlog is such a view.
- release — goreleaser, cross-platform binaries by tag
