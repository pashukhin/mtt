# Session 009 — Dogfood / self-host (design spec)

Date: 2026-07-09 · **Reconciled 2026-07-10** · Branch: `feat/s009-dogfood` · Version bump: `0.8.98-dev → 0.9.0-dev`

Authoritative design for s009. Prose in [DESIGN.md](../../../DESIGN.md) stays the source of truth; this spec is
the resolved decision record the plan and implementation follow. This slice makes **mtt track its own
development**: `mtt` this repository with a hand-authored config whose gates are **task-aware**, and migrate the
**forward** backlog (open sessions) onto mtt itself. After s009, `TASKS.md` is frozen and mtt is the live queue.

**This is not a normal CLI feature.** It is integration + config + committed data + docs, with **no production
logic change expected** in `pkg/`/`core`/`adapter`/`cli` (only the version string and new tests). If a real gap
surfaces during migration, it is TDD'd as a separate, in-scope enabler — but the working assumption is zero
logic change (the s008.5…s008.98 enablers already made self-host practical).

> **Reconciliation note (2026-07-10).** This spec was rewritten after the s009 flow discussion (see
> [flow-granularity note §9, C–G](../notes/2026-07-09-flow-granularity-for-dogfood.md) and the s009 brainstorm
> 2026-07-10). Two things changed vs the original 2026-07-09 draft: **(1) two-tier hierarchy** `phase`/`session`
> (the `step` type was dropped — decision C); **(2) the `session` flow is the full per-artifact review cycle**
> (15 statuses — the richer form of decision A). The brainstorm additionally resolved: the spec/plan
> artifact-presence gate (a `.mtt`-excluding git proxy), the attribution strategy (project-global
> `require:{who}`), and the `phase → done` self-referential gate (kept). It also surfaced two constraints that
> reshaped the recorded decisions E and F — recorded inline below.

## Goal

1. **A committed `.mtt/config.yaml`** for this repo: two custom types (`phase`/`session`) whose flow gates are
   **task-aware** — a session-branch is created on the entry edge `→ speccing` via the `{{.ID}}` placeholder,
   `make check` gates the implementation-review edges, a coarse proxy (uncommitted work outside `.mtt/`) gates
   the spec/plan review edges, and a `phase` can't close while it has open child sessions (a fail-closed gate
   that queries mtt's own task graph).
2. **Migrate the forward (open) backlog** onto mtt as committed `.mtt/tasks/*.yaml`: a Phase-4 phase with its
   open sessions (references / comments / actor-profiles / coding-demo + dangerous-ops attribution) and bare
   Phase-5…8 phase containers. Completed sessions are **not** backfilled (git + `sessions/*.md` are their record).
3. **Guard + prove it:** a Go test that the committed config always loads+validates (asserting the two types and
   the key gated/named edges of the full flow), and a `testscript` e2e that proves the branch/gate/current
   **mechanism** on a minimal scratch config with fake commands (the real gate needs a real repo, so the e2e
   proves the mechanism, not git/`make check`, per s006/s007/s008).

## Decisions (brainstormed)

### Q1 — How much backlog to migrate? **Forward-only (open work).**
Migrate only **open** work: the Phase-4 sessions still ahead (references → comments → actor-profiles →
coding-demo) plus a **new dangerous-ops attribution** session (the elevated TASKS think-item — the ideal first
self-hosted session), plus **bare** Phase-5…8 phase containers. **Completed** sessions (001–008.98) are **not**
migrated — their record is git history + `sessions/*.md`; archiving them in mtt is churn with no payoff. mtt's
value is a **live actionable queue**, not an archive. Design **think-items** (TASKS.md "Later (think)") and the
**parked** `advance`/roles work stay in the docs (they are design notes, not actionable sessions). s009 itself is
**not** created as a task (it is the migration act; its record is this spec + `sessions/009_dogfood.md`).

*Rejected:* full-history (every session as a task, completed ones `done`) — mechanical, low value; minimal-seed
(2–3 tasks) — under-delivers the "mtt is the backlog" outcome.

### Q2 — Task model & id/prefix scheme. **Two-tier `phase`/`session` (decision C — `step` dropped).**
Two bespoke types matching mtt's own vocabulary, mapping the roadmap 1:1:

| type      | prefix | parents        | default | role                                                              | gated? |
|-----------|--------|----------------|---------|------------------------------------------------------------------|--------|
| `phase`   | `p`    | `[]` (root)    | —       | a roadmap phase — a large body spanning multiple sessions         | self-ref gate on `→ done` |
| `session` | `s`    | `[phase]`      | ✅ `default: true` | a compact, independently shippable slice (branch + e2e) | **yes — the full gated method** |

Prefixes `p`/`s` are non-overlapping (neither is a prefix of the other), so the flat per-prefix mint
(`<prefix><N>`, `max+1`, `O_EXCL`) is unambiguous. IDs are freshly minted (`p1`, `s1`…) — a **new namespace**,
disjoint from the illustrative `e5_t2` bootstrap ids in the (soon-frozen) `TASKS.md`.

- **`default: true` = `session`** — the primary planning unit. `mtt add X --parent p1` creates a session; a root
  `phase` is added freely (no `--parent`). `session.parents = [phase]` (a session requires a phase parent).
- **No `step`/`subtask` type (decision C):** a rich `session` flow (per-artifact review) makes a separate
  increment tier redundant — the implementation phase **is** the work. If an implementation genuinely splits, a
  child is added *then* (a future config change), not authored up front. Consequence: the `tree` is two-level
  (phase → session). *The shipped default template stays `epic/task/subtask`* — the two-tier vocabulary is a
  **self-host** choice, not a template change (no embedded-template edit).
- *Rejected:* reuse `default` (epic/task/subtask) — the user chose the mtt-native vocabulary; the extra config
  authoring is one-time. *Rejected:* flat single-type — loses the phase/session hierarchy and roadmap's parent
  axis. *Rejected:* keeping `step` — the full session review cycle subsumes the increment tier (decision C).

### Q3 — Task-aware gates & the full session flow.

The committed `.mtt/config.yaml` wires **real** gates (the point of dogfood — mtt gates its own development).

#### `phase` flow (4 statuses)

`tbd (initial) → in_progress (active) → done (terminal)` (+`cancelled` terminal). No branch, no `current`
(a phase isn't "taken into work" — sessions carry the working context).

| edge | name | gate | description (agent runbook) |
|---|---|---|---|
| `tbd → in_progress` | `start` | — | "phase work has begun" |
| `in_progress → done` | `finish` | `out=$(mtt list --parent {{.ID}} --kind initial --kind active --ids) && test -z "$out"` | "close the phase — all its sessions are terminal (gate: no open child sessions)" |
| `tbd → cancelled` | `cancel` | — | "abandon the phase" |
| `in_progress → cancelled` | `cancel` | — | "abandon the phase" |

The `finish` gate is the §4 headline: it shells out to **`mtt` itself** and gates on the task graph — `mtt list
--parent {{.ID}} --kind initial --kind active --ids` prints the open direct children into `$out`; `test -z "$out"`
passes only when it is empty, so the gate **blocks** the phase's close while a child session is open. **Fail-closed
(improves on the §4 sketch):** the `&&` short-circuits when `mtt` is absent or `mtt list` errors (a non-zero
command substitution — e.g. `mtt` not on `PATH`, or the audit's A1 corrupt-file case), so it **blocks** rather
than silently closing. The note's `! mtt list … | grep -q .` form is **fail-open** (no `pipefail`; empty stdout
for *any* reason → `grep` exits 1 → `!` → 0 → passes), so this spec deliberately does not use it. Read-only ⇒
SEC2-safe (below). Needs `mtt` on `PATH` (`make install`); checks **direct** children only (recursive = YAGNI).

#### `session` flow (15 statuses — full per-artifact review cycle, decision A/D)

Statuses (kind): `tbd`(initial) · `speccing`,`spec_review`,`spec_human_review`,`spec_fix`(active) ·
`planning`,`plan_review`,`plan_human_review`,`plan_fix`(active) ·
`in_progress`,`impl_review`,`impl_human_review`,`impl_fix`(active) · `done`,`cancelled`(terminal).

Each of the three artifact stages (**design → plan → implementation**) is
`do → <do>_review (adversarial subagent) → <do>_human_review (human sign-off) → next`; a `decline` bounces to
`<do>_fix → <do>_review` (the `_fix` bounce is the rework counter — history is signal, decision D).

**Edge pattern** (same for the three stages; the gate differs — proxy for spec/plan, `make check` for impl):

| edge | name | gate | current |
|---|---|---|---|
| `tbd → speccing` | `start` | — | **set** + `git switch -c feat/{{.ID}} \|\| git switch feat/{{.ID}}` |
| `<do> → <do>_review` | `submit` | spec/plan: `git status --porcelain \| grep -qv '\.mtt/'` · impl: `make check` | — |
| `<do>_review → <do>_human_review` | `approve` | — | — |
| `<do>_review → <do>_fix` | `decline` | — | — |
| `<do>_human_review → <next-do>` | `approve` | — | impl→done: **clear** |
| `<do>_human_review → <do>_fix` | `decline` | — | — |
| `<do>_fix → <do>_review` | `submit` | same as first submit (proxy / `make check`) | — |
| `{tbd,speccing,planning,in_progress,spec_fix,plan_fix,impl_fix} → cancelled` | `cancel` | — | **clear** |

where `<next-do>` is `spec_human_review→planning`, `plan_human_review→in_progress`, `impl_human_review→done`.

- **Abandon path (no forward-trap).** `submit` is one-way out of a `do` status, so without care a session that
  entered a review cycle could never reach `cancelled` (only loop or advance). `cancel` therefore fires from the
  three `do` statuses, `tbd`, **and the three `_fix` statuses**; the `_review`/`_human_review` statuses reach
  `cancelled` in one step (`decline → _fix → cancel`). So every in-flight status can be abandoned.
- **YAML authoring of gate commands (trap — verified against `yaml.v3`).** Author every gate command as a
  **single-quoted** YAML scalar (or a literal block scalar), **never double-quoted** and **never a plain scalar
  starting with `!`**. Double-quoting breaks the proxy — `\.mtt/` is an invalid escape → `Load` fails and the
  whole config is unloadable — and the shipped `coding.yaml` uses the double-quoted `["! make test"]`
  convention, so this is the *easy* mistake. A plain scalar beginning `! …` has its `!` parsed as a YAML tag and
  **silently dropped** (which is why the phase gate above avoids the `!`-form entirely). `TestRepoDogfoodConfig`
  guards this by asserting the **exact** command strings (including any leading `!` and the `\.mtt/`), never
  substrings.

- **Edge names** `start / submit / approve / decline / cancel` are **disjoint from all status names**, **unique
  per source status**, and every `(from,to)` pair is **unique** — the three s008.98 invariants hold, so the
  edge-verb sugar (`mtt approve s1`, `mtt decline s1`, `mtt submit s1`) resolves unambiguously. `cancel` ≠
  `cancelled` (edge name vs status name).
- **`make check` is the only heavy gate** — on **every edge into `impl_review`** (`in_progress → impl_review`
  and `impl_fix → impl_review`), so `impl_review` is only ever entered on green. It is **not** repeated on
  `→ done` (the code is already green at `impl_human_review`). Gate-cost decision (S5, accepted): ~1 full
  `make check` per impl submit/resubmit; `command_timeout: 10m` (headroom for a first-run lint + `-race`
  compile; the code default is 5m).
- **Artifact-presence proxy (spec/plan)** — `git status --porcelain | grep -qv '\.mtt/'` exits 0 iff there is
  an **uncommitted change outside `.mtt/`** (a **coarse** proxy: *some* uncommitted work exists outside `.mtt/`,
  keyed to the uncommitted-until-review convention below — it cannot distinguish a spec from a plan from any
  unrelated dirty file; decision F accepts this). It fails **closed** on a git error (a non-zero `git status`
  short-circuits the pipe). It excludes `.mtt/` because the accumulating `.mtt/tasks/*.yaml` churn (committed
  only with the session PR — S4) would
  otherwise make a bare `grep -q .` trivially pass (finding **F**, below). **Semantics (documented):** the
  artifact stays **uncommitted until its `_human_review` approves** — the review is over the working tree; the
  commit happens after sign-off / with the session PR. (impl is unaffected — it gates on `make check`, and code
  is committed per TDD cycle.)
- **Branch on entry** — `git switch -c feat/{{.ID}} || git switch feat/{{.ID}}` on `tbd → speccing`: idempotent
  (re-entering an existing session branch is correct for re-taking a session into work), **no `rollback:`**
  (finding **S2/U1**: the `git branch -D` compensator is broken and a rollback only fires on a *later* command's
  failure — here the branch command is the only command on the edge). The shell seam (`||`) is already relied on
  by `! make test` in the coding template.

**Branch naming = `feat/{{.ID}}`** (shares the `feat/` namespace with our session branches), not the DESIGN
canonical `task/{{.ID}}`. Accepted frictions (documented as a **bootstrap caveat**):
- mtt-minted session ids (`s1`, `s2`…) differ from the docs' historical `sNNN` numbering (`s010`…) — the mtt id
  is the going-forward identity; the `sNNN` doc labels are legacy.
- the branch carries **no slug** (`feat/s1`, not `feat/s1-references`) — placeholders are a **structural
  whitelist** (`.ID`/`.Type`/`.From`/`.To` only; free text like the title is never interpolated, by design —
  s007).
- s009 itself runs on the manually-created `feat/s009-dogfood` (the branch predates the config); the config
  governs **future** sessions. No migrated task is transition-driven through the gate during s009.

#### Attribution — project-global `require: {who: true}` (finding on decision E)

**Constraint (verified in code):** `require:{who,why}` in mtt is **project-global** (`Settings.Require` at the
committed-config root, `config.local` tighten-only, checked in `core.Transitioner` for *every* edge). There is
**no per-edge `require`** field on `pkg/mtt.Transition`. Decision E's "`require:{who,why}` on the human-review
edges" is therefore **not expressible without a core change** — which is out of scope for s009 ("no production
logic change"). Per-edge / role-based attribution is the **parked roles work** (E itself names this the first
concrete unpark trigger).

**Resolution:** the committed config sets **`require: {who: true}`** globally. `who` is auto-satisfied by the
`config.local.yaml` `author` (each agent sets it, or uses `--by`/`MTT_BY`), so every move records a real `by:` —
a self-approval of a `_human_review` edge is **visible** in history as `by: <the agent>` (honest, not enforced,
faithful to E's intent within the global-only capability). `why` is left optional (`--why` where it matters — a
project-global `require:{why}` would force `--why` on every one of 15+ moves, too heavy). The dangerous-ops
attribution session (migrated below) is where per-edge/forced attribution gets designed properly.

**Setup / first-run papercut (documented, not a blocker):** because `require` is global, it applies to *every*
edge (incl. the mechanical `submit`/`decline`/`cancel`), and `--no-run` does **not** bypass it — so each agent
must set `author` in `config.local.yaml` (gitignored) or `MTT_BY`/`--by` **before the first move**, or moves
exit 2 on a fresh checkout. A one-time setup step; noted in the dogfood docs.

## Architecture (resolved)

`cli → core → port ← adapter` — **unchanged**. s009 adds **no** ports, usecases, or CLI commands. The config is
**hand-authored** in `.mtt/config.yaml` (not produced by `mtt init` — init emits the command-less `default`
template, and our types are bespoke). No new **embedded** template is added: `internal/adapter/yaml/templates/`
is for *other* projects' `mtt init`; this repo's config is repo-specific and lives only in `.mtt/`.

The self-hosted `.mtt/` (config + tasks) is **committed**; `.mtt/config.local.yaml` is **already** gitignored
(present since early sessions — no `.gitignore` change needed). Existing tests are unaffected: the e2e
`testscript` suites run in `$WORK` temp dirs (`MTT_DIR`/`cd`), so they never `FindRoot` this repo's new `.mtt/`.

### Security / trust (SEC2 — folded in per the reconciliation)

**Gates may invoke read-only `mtt` commands only.** The `phase → done` self-referential gate uses `mtt list …`
(read-only, safe). A gate must **never** invoke an mtt **transition** — that could recurse (transition → gate →
transition …), bounded only by timeouts. The config-as-code trust model is otherwise unchanged (commands are
trusted project config, like a Makefile; placeholder injection defense is structural — only `.ID/.Type/.From/.To`
reach a command).

### Task-file commit discipline (S4)

`.mtt` mutations (every `add`/`status`/`do` writes `.mtt/tasks/*.yaml`) are **committed with the session PR**;
after the session's `approve → done` (or before the merge), run `git add .mtt && git commit` so the task file
lands in its final status. Between `add` and the squash-merge the task is invisible on `main` — accepted (solo).

### Migration content (forward-only)

Created with the built binary (`./bin/mtt add …`, scripted deterministically — phases first, then sessions), the
resulting `.mtt/tasks/*.yaml` committed. Everything is `tbd` (all forward work is unstarted); `current` is left
unset (a later `mtt use` sets it). Ordering in `mtt roadmap` comes from **priorities** (references-first and
demo-last); ties are recency-ordered (S3 — so bare phases sink via `--priority low`, and **`mtt roadmap` is
hand-run and eyeballed before committing** `.mtt/tasks/*.yaml`).

- **Phase 4** (`p1`, "dogfood → references → comments → profiles") → sessions:
  `references` (**high**), `comments` (medium), `actor profiles` (medium), `coding-template demo` (low), and
  **`dangerous-ops attribution`** (the elevated TASKS think-item — the ideal first self-hosted session; priority
  set so the roadmap reads sensibly, verified by the hand-run). Each carries a one-line description mirroring its
  `sessions/README.md` roadmap row + TASKS id.
- **Phase 5** (`p2`, "knowledge base + text search") — bare phase (`--priority low`).
- **Phase 6** (`p3`, "text/ASCII Gantt + richer query") — bare phase (`--priority low`).
- **Phase 7** (`p4`, "mtt-ui — optional web UI") — bare phase (`--priority low`).
- **Phase 8** (`p5`, "external adapters + indexer hook") — bare phase (`--priority low`).

No child sessions are pre-split under a session (a session's breakdown emerges in *that* session's brainstorm).

## Acceptance (must pass)

- **User scenario (real config, manual/CI):** in the repo, `mtt types` shows `phase`/`session` with the gates;
  `mtt list` / `mtt tree` / `mtt roadmap` render the migrated Phase-4 hierarchy and open sessions.
- **Committed-config guard (Go test, genuine red→green):** `TestRepoDogfoodConfig` — `FindRoot` locates this
  repo's `.mtt/`, `Load` + `Config.Validate()` are green, and it asserts:
  - exactly **two** types (`phase`, `session`); `session` is `default: true`; prefixes `p`/`s`.
  - the `session` flow has the **15 statuses** with the right kinds; the entry edge `tbd → speccing` carries the
    `git switch … feat/{{.ID}}` branch command **and** `current: set`; the edges into `impl_review`
    (`in_progress → impl_review`, `impl_fix → impl_review`) gate on `make check`; the spec/plan submit edges
    carry the `grep -qv '\.mtt/'` proxy; `impl_human_review → done` has `current: clear`; the named edges
    (`start`/`submit`/`approve`/`decline`/`cancel`) exist and satisfy the disjointness/uniqueness invariants.
    Gate assertions compare the **exact** command strings (the `make check`, the proxy including `\.mtt/`, the
    fail-closed phase gate) — **not** substrings — so a YAML-mangled (double-quote-broken) or inverted
    (`!`-dropped) gate is caught by the guard, not discovered at runtime.
  - the `phase` flow's `in_progress → done` (`finish`) carries the fail-closed self-ref gate
    (`out=$(mtt list … --ids) && test -z "$out"`).
  - the project sets `require: {who: true}`.
  A CI-forever guard against a broken committed config (`Config.Validate` runs on `add`/`types`, **not** on
  `Load` — this test is the **sole** guard, S6). Red before `.mtt/config.yaml` exists → green after.
- **Mechanism e2e (`testscript` `dogfood.txt`):** a **scratch** config (txtar `-- gated.yaml --` `cp`'d over
  `.mtt/config.yaml`) — a **minimal valid flow** (`initial → active → terminal`, not all 15 statuses) mirroring
  the real *mechanism* with **fake** commands — proves: `mtt types` validates (run **before** the first move —
  §9 precondition); the entry edge runs `git switch -c feat/{{.ID}}` → the branch exists (`git symbolic-ref
  --short HEAD`, guarded `[!exec:git] skip`, `git symbolic-ref` for the unborn branch — s007 lesson) and sets
  `current`; a `→ <review>` edge with a **failing** gate command **blocks** (non-zero — task unchanged, no
  history; the exact exit 3 is unit-tested, `testscript` asserts only non-zero); with a **passing** gate command
  **moves** and **clears** `current` on the terminal edge. Proves the mechanism,
  not the real `make check` / mtt-self-ref gate (a temp dir has no Makefile — the s006/s007/s008 e2e strategy).
- `make check` green.

## Out of scope (explicitly deferred)

- Migrating **completed** sessions / **think-items** / **parked** work into mtt (stay in docs + git).
- A new **embedded template** or a `mtt init --template mtt` (the config is repo-specific, hand-authored).
- **Per-edge / role-based `require`** (decision E's full form) — needs a core change; the parked roles work,
  designed in the migrated dangerous-ops session.
- **Bulk transition** migration (moving many tasks through gates) — the migrated set is created `tbd` via `add`.
- Changing the **branch workflow** wholesale to `feat/{{.ID}}` for s009 itself (bootstrap runs on the manual
  `feat/s009-dogfood`; the config governs future sessions).
- Any **monotonic-id** / lost-update / scale-stress work (TASKS "Later (think)") — surfaced, not built.

## Docs sync (same session)

`DESIGN.md`/`.ru` (a "Dogfooding / self-host" note under "Implementation order", incl. the bootstrap caveat +
the SEC2 read-only-gate rule + the S4 commit-discipline line); `CLI_REFERENCE.md`/`.ru` (a brief self-host
mention if warranted — likely minimal); `docs/architecture/model.go` (a note only if a decision touches the
contract — none expected); `TASKS.md` **frozen** (a banner + `e5_t2 ✅`); `sessions/README.md` (009 ✅, 010 ←
next); `NEXT_SESSION.md` ("Where we are" + "Next task = s010 references" + "Carry-over lessons (009)");
`sessions/009_dogfood.md` (Done filled); the flow-granularity note §9 open items marked decided; version
`0.8.98-dev → 0.9.0-dev` ([internal/cli/root.go](../../../internal/cli/root.go)); any package `CLAUDE.md` only
if a package changes (expected: none).

## Definition of Done

- `.mtt/config.yaml` (two custom types + task-aware gates + `require:{who}`) and `.mtt/tasks/*.yaml` (forward
  backlog) committed.
- `TestRepoDogfoodConfig` green; `dogfood.txt` e2e green; `make check` green.
- Docs synced (above); version bumped.
- Branch `feat/s009-dogfood` → PR → CI green → squash into `main`.
