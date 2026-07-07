# Sessions

Work happens in **compact sessions**. Each session is small and ends with something **practical and
immediately verifiable** — a user-runnable command backed by an e2e test (`testscript`) and a user
scenario. No session leaves the tree half-usable.

## How it works

- One file per session: `NNN_<slug>.md` (e.g. `001_init_and_types.md`). Copy `000_template.md`.
- Write the **Target** (goal, scope, acceptance) up front; during the session fill in **Done** — what
  was actually built, deviations, follow-ups.
- Start each session by refining the plan (superpowers: brainstorming/planning), then work **test-first**.
- Definition of Done: the session's acceptance e2e passes + `make check` green + docs updated.
  Branch `feat/sNNN-<slug>` → PR → squash into `main`.
- **Version mirrors the session** (pre-1.0 mnemonic, not strict semver): `sN` → `0.N.0-dev`, a
  point-session `sN.M` → `0.N.M-dev` (a full session bumps the minor, a point-session the patch). E.g.
  s006 → `0.6.0-dev`, s006.5 → `0.6.5-dev`, s006.7 → `0.6.7-dev`, s007 → `0.7.0-dev`.

## Roadmap (vertical slices — each ends with a runnable command + e2e)

Maps onto the phases in [../DESIGN.md](../DESIGN.md) / the backlog in [../TASKS.md](../TASKS.md), but
sliced so **every session delivers something usable**. Order/size may be refined as we go.

| # | Target | You can now… | e2e gist |
|---|---|---|---|
| 001 ✅ | init & inspect | `mtt init [--template]`, `mtt types` | init → `types` shows epic/task/subtask + flow |
| 002 ✅ | create & view | `mtt add`, `mtt show` | init → add → show the task |
| 003 ✅ | list & edit | `mtt list` (filters), `mtt edit` | add several → list/filter → edit → show |
| 004 ✅ | hierarchy | `mtt add --parent`, `mtt tree`, `show` lineage | epic→task→subtask; tree renders |
| 004.5 ✅ | typed-id retrofit (chore) | — (internal: `TaskID`/`TypeName`/`StatusName`) | `make check` green; no behaviour change |
| 005 ✅ | dependencies | `mtt dep add/rm/list` (`--tree`/`--cycles`), `mtt ready`, `list --ready` | dep blocks ready; cycle rejected |
| 006 ✅ | **flow gate (killer)** | `mtt status <id> <new>` runs & gates commands | failing gate blocks transition; history written |
| 006.5 ✅ | attribution + verb sugar | `--why` (+ history field), `--who` (alias of `--by`), `mtt <status> <id>`, required-attribution (exit 2) | `mtt done t1`; `show` prints who/why; `require:{who,why}` blocks pre-gate |
| 006.7 ✅ | **current task** (working context) | `current` in `config.local` (capability port `CurrentStore`), set/clear via `Transition.Current`; omitted id → current for `status`/sugar/`show`/`edit`; `mtt use [<id>] [--clear]` | `mtt done` / `mtt show` act on the current task; edge sets/clears the pointer |
| 007 ✅ | **structured commands** | placeholders + per-command timeout in transition commands (`Command` VO) | `in_progress t1` creates a `task/t1` branch; a slow gate fails fast on its per-command timeout |
| 008 ✅ | **rollback** | per-command `rollback:` — reverse-order compensation on a failed pipeline (best-effort; blocked stays exit 3, no history) | a late gate failure undoes prior side effects; `↩ compensating` |
| 008.5 ✅ | dogfood enablers (chore) | `mtt rm` (reject-if-referenced + `--force`, uniform exit-4), `--depends-on` on `add`, packaging (`make install` ldflags + `make smoke`) | delete a task; `add --depends-on`; `go install ./cmd/mtt` |
| 008.6 | **priorities + roadmap** 📋 spec'd ⬅ **next** | `Priority` VO (`--priority` on add/edit/list, `--sort priority`); `mtt roadmap [--json]` — dependency+priority execution order | `roadmap --json` gives the agent-ordered plan with `ready`/`blocked_by` |
| 008.7 | **tags** | `mtt add --tag`, `tag add/rm`, `list/tree --tag`; `#hashtags` in title/description | tag a task; filter by tag; `add "fix #auth"` tags it |
| 008.9 | **batch & pipeline** | task-set selector (IDs ∪ `--filter` ∪ stdin `-`) + `--ids` output; bulk `tag add/rm`, `rm` | `list --tag x --ids \| tag rm x -`; `tag add y --status tbd` |
| 009 | **dogfood** | self-host: `mtt init` this repo, task-aware gates, migrate the backlog | mtt tracks its own tasks; `done` gated on `make check` |
| 010 | references | `mtt ref add/rm/list`, backlinks | ref resolves; task↔PR/spec link |
| 011 | comments | `mtt comment add/list` (tree) | nested comments render in `show` |
| 012 | actor profiles | `mtt profile …`; default profile = the coding agent | `by`/`role` from the default profile |
| 013+ | Graphviz flow export, coding template demo, KB/search, Gantt, `mtt-ui`, external adapters | later phases (see DESIGN.md) | |
| parked | **advance** / `start` / `done` / `cancel` + modes (`--stop`/`--atomic`/`--force`) + roles-on-edges | on-demand — surfaces only when a flow actually branches (single-edge `status` is the norm) | |

**Decisions carried from the domain-model snapshot** ([../docs/architecture/model.go](../docs/architecture/model.go)):

- **004.5 (typed-id retrofit)** ✅ shipped: the `pkg/mtt`/`core`/`adapter`/`cli` surface now uses the typed
  identities (`TaskID`/`TypeName`/`StatusName`), so 005 is written against the typed contract. Mechanical; the
  only behaviour change is fail-fast on a corrupt on-disk `id`/`type`/`status`.
- **005 adds no new port.** `depends_on` rides on the `Task` field + `TaskStore.Update` (as `parent` did in
  004); `DependencyStore` is only for external adapters that cannot embed. 005 = core `DependencyEditor` +
  `Ready` + cycle-check.
- **Packaging** (`make install` → `go install ./cmd/mtt`) folds into the **009.5** dogfood-enablers chore
  (alongside `mtt rm` and `--depends-on` on `add`). Full release tooling (goreleaser, tagged binaries) is later.

**Roadmap regrouped (2026-07-05, after s006).** Two moves:

1. Dogfooding "the agent works in task terms, with all shell orchestration living in flow transitions" has a
   hard prerequisite: **command placeholders** (a transition can't create a per-task branch —
   `git checkout -b task/{{.ID}}` — without them), plus per-command timeout and rollback. So **structured
   commands** + **rollback** were pulled up from the backlog, and **dogfood** moved *earlier* — ahead of
   references/comments, which enrich a full self-host but don't enable it.
2. **`advance` was parked** (and with it `start`/`done`/`cancel`, the modes, and roles-on-edges). Rationale:
   most status transitions are exactly **one edge**, so single-edge `mtt status` is the norm and a multi-edge
   walk solves a problem we don't have yet; role-tagged edges are near-RBAC and premature (identity/role may
   later come from an external provider). advance surfaces **on-demand**, when a flow actually branches. That
   makes **structured commands** the real next rock, preceded by a tiny **006.5** attribution+sugar slice:
   `--why` (a durable free-text reason recorded in history — "who + why moved the task"), `--who` (a symmetric
   alias of `--by`), and the `mtt <status> <id>` verb sugar (via fallback-routing on an unknown first arg, not
   dynamic command registration; single-edge, forward-compatible to advance later without a surface change).

The `coding` template demo (branch + gated DoD) only becomes fully powered once structured commands land, so
it sits in the later-phases bucket. **actor profiles** (default profile = the coding agent) pair with
instructing the agent to work in task terms.

**Cross-cutting — global flags** (root persistent flags; see [../CLI_REFERENCE.md](../CLI_REFERENCE.md) →
"Global flags"). Not a session of their own — land early so new commands **inherit** them instead of
retrofitting each one. Ownership: `--dir`/`MTT_DIR` + the `--version` flag (verify `--help`) → **003** (also
DRYs the repeated `Getwd → FindRoot`); `--json` machine-readable output → **003** (`mtt list` is its first
real consumer); `--role`/`MTT_ROLE` → **006** (recorded with `history`; the reserved roles seam);
`-q/--quiet`, `--no-color` → later (polish).
