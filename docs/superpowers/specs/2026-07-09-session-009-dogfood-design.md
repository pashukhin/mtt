# Session 009 — Dogfood / self-host (design spec)

Date: 2026-07-09 · Branch: `feat/s009-dogfood` · Version bump: `0.8.9-dev → 0.9.0-dev`

Authoritative design for s009. Prose in [DESIGN.md](../../../DESIGN.md) stays the source of truth; this spec is
the resolved decision record the plan and implementation follow. This slice makes **mtt track its own
development**: `mtt` this repository with a hand-authored config whose gates are **task-aware**, and migrate the
**forward** backlog (open sessions) onto mtt itself. After s009, `TASKS.md` is frozen and mtt is the live queue.

**This is not a normal CLI feature.** It is integration + config + committed data + docs, with **no production
logic change expected** in `pkg/`/`core`/`adapter`/`cli` (only the version string and new tests). If a real gap
surfaces during migration, it is TDD'd as a separate, in-scope enabler — but the working assumption is zero
logic change (the s008.5/008.6/008.7/008.9 enablers already made self-host practical).

## Goal

1. **A committed `.mtt/config.yaml`** for this repo: three custom types (`phase`/`session`/`step`) whose flow
   gates are **task-aware** — a session-branch is created on `→ in_progress` via the `{{.ID}}` placeholder, and
   `make check` gates `→ done`.
2. **Migrate the forward (open) backlog** onto mtt as committed `.mtt/tasks/*.yaml`: a Phase-4 epic with its
   open sessions (references / comments / actor-profiles / coding-demo) and bare Phase-5…8 epic containers.
   Completed sessions are **not** backfilled (git + `sessions/*.md` are their record).
3. **Guard + prove it:** a Go test that the committed config always loads+validates, and a `testscript` e2e that
   proves the branch-on-`→in_progress` + `make check`-gate-on-`→done` **mechanism** (on a scratch config with
   fake commands — the real gate needs a real repo, so the e2e proves the mechanism, not git, per s006/s007).

## Decisions (brainstormed)

### Q1 — How much backlog to migrate? **Forward-only (open work).**
Migrate only **open** work: the Phase-4 sessions still ahead (references → comments → actor-profiles →
coding-demo) plus **bare** Phase-5…8 epic containers. **Completed** sessions (001–008.9) are **not** migrated —
their record is git history + `sessions/*.md`; archiving them in mtt is churn with no payoff. mtt's value is a
**live actionable queue**, not an archive. Design **think-items** (TASKS.md "Later (think)") and the **parked**
`advance`/roles work stay in the docs (they are design notes, not actionable sessions). s009 itself is **not**
created as a task (it is the migration act; its record is this spec + `sessions/009_dogfood.md`).

*Rejected:* full-history (every session as a task, completed ones `done`) — mechanical, low value; minimal-seed
(2–3 tasks) — under-delivers the "mtt is the backlog" outcome.

### Q2 — Task model & id/prefix scheme. **Custom `phase`/`session`/`step`.**
Three bespoke types matching mtt's own vocabulary, mapping the roadmap 1:1:

| type      | prefix | parents      | role                                                       | gated? |
|-----------|--------|--------------|------------------------------------------------------------|--------|
| `phase`   | `p`    | `[]` (root)  | a roadmap phase — a large body spanning multiple sessions  | no (container) |
| `session` | `s`    | `[phase]`, **`default: true`** | a compact, independently shippable slice (branch + e2e) | **yes** |
| `step`    | `t`    | `[session]`  | a granular test-first increment within a session           | **yes** |

Prefixes `p`/`s`/`t` are pairwise non-overlapping (none is a prefix of another), so the flat per-prefix mint
(`<prefix><N>`, `max+1`, `O_EXCL`) is unambiguous. IDs are freshly minted (`p1`, `s1`, `t1`…) — a **new
namespace**, disjoint from the illustrative `e5_t2` bootstrap ids in the (soon-frozen) `TASKS.md`.

- **`default: true` = `session`** — the primary planning unit. `mtt add X --parent p1` creates a session (like
  the default template's `task` under `epic`; a root `phase` is added freely, no `--parent`).
- *Rejected:* reuse `default` (epic/task/subtask) — the user chose the mtt-native vocabulary; the extra config
  authoring is one-time. *Rejected:* flat single-type — loses the phase/session hierarchy and roadmap's
  parent axis.

### Q3 — Task-aware gates. **Honest: branch on `→in_progress` + `make check` on `→done`.**
The committed `.mtt/config.yaml` wires **real** gates (the point of dogfood — mtt gates its own development):

- **`session`** — `tbd → in_progress`: `git checkout -b feat/{{.ID}}` (`current: set`, description "create the
  session branch"); `in_progress → done`: `make check` (`current: clear`, description "make check green (the
  gate)"); the two `→ cancelled` edges carry no commands.
- **`step`** — `tbd → in_progress`: `current: set`, **no branch** (a step works inside the session branch);
  `in_progress → done`: `make check` (`current: clear`). **Every step is green** — faithful to AGENTS.md
  ("`make check` green before every commit"); step-done re-running the same gate as session-done is redundant
  but harmless (honest re-verification).
- **`phase`** — `current: set|clear` only, **no commands** (a container completes when its sessions do; the
  `→ done` description is "all sessions in the phase closed").
- `command_timeout: 10m` (headroom for a first-run lint + `-race` compile; the code default is 5m).

**Branch naming = `feat/{{.ID}}`** (shares the `feat/` namespace with our session branches), not the DESIGN
canonical `task/{{.ID}}`. Accepted frictions (documented as a **bootstrap caveat**):
- mtt-minted session ids (`s1`, `s2`…) differ from the docs' historical `sNNN` numbering (`s010`…) — the mtt id
  is the going-forward identity; the `sNNN` doc labels are legacy.
- the branch carries **no slug** (`feat/s1`, not `feat/s1-references`) — placeholders are a **structural
  whitelist** (`.ID`/`.Type`/`.From`/`.To` only; free text like the title is never interpolated, by design —
  s007). A slug would need exposing a quoted free-text field: out of scope.
- s009 itself runs on the manually-created `feat/s009-dogfood` (the branch predates the config); the config
  governs **future** sessions. No migrated task is transition-driven through the gate during s009.

*Rejected:* `make check` on `→done` but no branch on `→in_progress` (a "not on main" guard instead) — the user
chose the honest branch gate; lightweight `go build` gate — weaker demonstration than the real DoD.

## Architecture (resolved)

`cli → core → port ← adapter` — **unchanged**. s009 adds **no** ports, usecases, or CLI commands. The config is
**hand-authored** in `.mtt/config.yaml` (not produced by `mtt init` — init emits the command-less `default`
template, and our types are bespoke). No new **embedded** template is added: `internal/adapter/yaml/templates/`
is for *other* projects' `mtt init`; this repo's config is repo-specific and lives only in `.mtt/`.

The self-hosted `.mtt/` (config + tasks) is **committed**; `.mtt/config.local.yaml` is **already** gitignored
(present since early sessions — no `.gitignore` change needed). Existing tests are unaffected: the e2e
`testscript` suites run in `$WORK` temp dirs (`MTT_DIR`/`cd`), so they never `FindRoot` this repo's new `.mtt/`.

### Migration content (forward-only)

Created with the built binary (`./bin/mtt add …`, scripted deterministically — phases first, then sessions), the
resulting `.mtt/tasks/*.yaml` committed. Everything is `tbd` (all forward work is unstarted); `current` is left
unset (a later `mtt use` sets it). Ordering in `mtt roadmap` comes from **priorities** (not artificial hard
deps — "comments" does not *depend on* "references"; they are merely sequenced).

- **Phase 4** (`p1`, "dogfood → references → comments → profiles") → sessions:
  `references` (**high**), `comments` (medium), `actor profiles` (medium), `coding-template demo` (low). Each
  carries a one-line description mirroring its `sessions/README.md` roadmap row + TASKS `e5_*` id.
- **Phase 5** (`p2`, "knowledge base + text search") — bare epic (description only, no sessions yet).
- **Phase 6** (`p3`, "text/ASCII Gantt + richer query") — bare epic.
- **Phase 7** (`p4`, "mtt-ui — optional web UI") — bare epic.
- **Phase 8** (`p5`, "external adapters + indexer hook") — bare epic.

No **steps** are created during migration (a session's step breakdown emerges in *that* session's brainstorm);
the `step` type exists in the config, ready for use. (Optional: a couple of illustrative steps under a session
to exercise the 3-level `tree` — decided during implementation, not required.)

## Acceptance (must pass)

- **User scenario (real config, manual/CI):** in the repo, `mtt types` shows `phase`/`session`/`step` with the
  gates; `mtt list` / `mtt tree` / `mtt roadmap` render the migrated Phase-4 hierarchy and open sessions.
- **Committed-config guard (Go test, genuine red→green):** `TestRepoDogfoodConfig` — `FindRoot` locates this
  repo's `.mtt/`, `Load` + `Config.Validate()` are green, and it asserts the three types exist and that the
  `session` flow gates on `make check` (`→done`) and `git checkout -b feat/` (`→in_progress`). A CI-forever
  guard against a broken committed config. (Red before `.mtt/config.yaml` exists → green after.)
- **Mechanism e2e (`testscript` `dogfood.txt`):** a **scratch** config (txtar `-- gated.yaml --` `cp`'d over
  `.mtt/config.yaml`) mirroring the real shape with **fake** commands proves: `→in_progress` runs
  `git checkout -b feat/{{.ID}}` → the branch exists (`git symbolic-ref --short HEAD`, guarded `[!exec:git]
  skip`); `→done` with a **failing** gate command **blocks** (exit 3, task unchanged, no history); with a
  **passing** gate command **moves** to `done` and **clears** `current`. Proves the mechanism, not the real
  `make check` (a temp dir has no Makefile — the s006/s007/s008 e2e strategy).
- `make check` green.

## Out of scope (explicitly deferred)

- Migrating **completed** sessions / **think-items** / **parked** work into mtt (stay in docs + git).
- A new **embedded template** or a `mtt init --template mtt` (the config is repo-specific, hand-authored).
- **Bulk transition** migration (moving many tasks through gates) — the migrated set is created `tbd` via
  `add`; gated bulk stays later (s008.9 out-of-scope).
- Changing the **branch workflow** wholesale to `feat/{{.ID}}` for s009 itself (bootstrap runs on the manual
  `feat/s009-dogfood`; the config governs future sessions).
- Any **monotonic-id** / scale-stress work (TASKS "Later (think)") — surfaced, not built.

## Docs sync (same session)

`DESIGN.md`/`.ru` (a "Dogfooding / self-host" note under "Implementation order", incl. the bootstrap caveat);
`CLI_REFERENCE.md`/`.ru` (a brief self-host mention if warranted — likely minimal); `docs/architecture/model.go`
(a note if a decision touches the contract — likely none); `TASKS.md` **frozen** (a banner + `e5_t2 ✅`);
`sessions/README.md` (009 ✅, 010 ← next); `NEXT_SESSION.md` ("Where we are" + "Next task = s010 references" +
"Carry-over lessons (009)"); `sessions/009_dogfood.md` (Done filled); version `0.8.9-dev → 0.9.0-dev`
([internal/cli/root.go](../../../internal/cli/root.go)); any package `CLAUDE.md` only if a package changes
(expected: none).

## Definition of Done

- `.mtt/config.yaml` (custom types + task-aware gates) and `.mtt/tasks/*.yaml` (forward backlog) committed.
- `TestRepoDogfoodConfig` green; `dogfood.txt` e2e green; `make check` green.
- Docs synced (above); version bumped.
- Branch `feat/s009-dogfood` → PR → CI green → squash into `main`.
