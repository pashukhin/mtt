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

## Roadmap (vertical slices — each ends with a runnable command + e2e)

Maps onto the phases in [../DESIGN.md](../DESIGN.md) / the backlog in [../TASKS.md](../TASKS.md), but
sliced so **every session delivers something usable**. Order/size may be refined as we go.

| # | Target | You can now… | e2e gist |
|---|---|---|---|
| 001 ✅ | init & inspect | `mtt init [--template]`, `mtt types` | init → `types` shows epic/task/subtask + flow |
| 002 ✅ | create & view | `mtt add`, `mtt show` | init → add → show the task |
| 003 ✅ | list & edit | `mtt list` (filters), `mtt edit` | add several → list/filter → edit → show |
| 004 ✅ | hierarchy | `mtt add --parent`, `mtt tree`, `show` lineage | epic→task→subtask; tree renders |
| 004.5 | typed-id retrofit (chore) | — (internal: `TaskID`/`TypeName`/`StatusName`) | `make check` green; no behaviour change |
| 005 | dependencies | `mtt dep add/rm/list`, `mtt ready` | dep blocks ready; cycle rejected |
| 006 | **flow gate (killer)** | `mtt status <id> <new>` runs & gates commands | failing gate blocks transition; history written |
| 007 | **advance** | `mtt start`/`done`/`cancel` + modes | `mtt done` walks tbd→…→done, blocks on red gate |
| 008 | references | `mtt ref add/rm/list`, backlinks | ref resolves; dangling flagged |
| 009 | comments | `mtt comment add/list` (tree) | nested comments render in `show` |
| 010 | coding template + dogfood | `mtt init --template coding`; self-host | migrate this backlog onto mtt |
| 011+ | KB/search, Gantt, `mtt-ui`, external adapters | later phases (see DESIGN.md) | |

**Decisions carried from the domain-model snapshot** ([../docs/architecture/model.go](../docs/architecture/model.go)):

- **004.5 (typed-id retrofit)** is a small, optional chore recommended *before* 005 so new code is written
  against the typed identity surface (`TaskID`/`TypeName`/`StatusName`). Mechanical; no behaviour change.
- **005 adds no new port.** `depends_on` rides on the `Task` field + `TaskStore.Update` (as `parent` did in
  004); `DependencyStore` is only for external adapters that cannot embed. 005 = core `DependencyEditor` +
  `Ready` + cycle-check.
- **Packaging** (`make install` → `go install ./cmd/mtt`) is a separate small chore, best landed near **006**
  — that is when status transitions make mtt a real tracker worth dogfooding. Full release tooling
  (goreleaser, tagged binaries) is later.

**Cross-cutting — global flags** (root persistent flags; see [../CLI_REFERENCE.md](../CLI_REFERENCE.md) →
"Global flags"). Not a session of their own — land early so new commands **inherit** them instead of
retrofitting each one. Ownership: `--dir`/`MTT_DIR` + the `--version` flag (verify `--help`) → **003** (also
DRYs the repeated `Getwd → FindRoot`); `--json` machine-readable output → **003** (`mtt list` is its first
real consumer); `--role`/`MTT_ROLE` → **006** (recorded with `history`; the reserved roles seam);
`-q/--quiet`, `--no-color` → later (polish).
