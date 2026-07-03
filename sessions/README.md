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
| 001 | init & inspect | `mtt init [--template]`, `mtt types` | init → `types` shows epic/task/subtask + flow |
| 002 | create & view | `mtt add`, `mtt show` | init → add → show the task |
| 003 | list & edit | `mtt list` (filters), `mtt edit` | add several → list/filter → edit → show |
| 004 | hierarchy | `mtt add --parent`, `mtt tree` | epic→task→subtask; tree renders |
| 005 | dependencies | `mtt dep add/rm/list`, `mtt ready` | dep blocks ready; cycle rejected |
| 006 | **flow gate (killer)** | `mtt status <id> <new>` runs & gates commands | failing gate blocks transition; history written |
| 007 | **advance** | `mtt start`/`done`/`cancel` + modes | `mtt done` walks tbd→…→done, blocks on red gate |
| 008 | references | `mtt ref add/rm/list`, backlinks | ref resolves; dangling flagged |
| 009 | comments | `mtt comment add/list` (tree) | nested comments render in `show` |
| 010 | coding template + dogfood | `mtt init --template coding`; self-host | migrate this backlog onto mtt |
| 011+ | KB/search, Gantt, `mtt-ui`, external adapters | later phases (see DESIGN.md) | |
