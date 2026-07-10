# Session 009 — Dogfood / self-host (design spec)

Date: 2026-07-09 · **Reconciled 2026-07-10 (two-tier)** · **Re-modelled 2026-07-10 (single `task`)** ·
Branch: `feat/s009-dogfood` · Version bump: `0.8.98-dev → 0.9.0-dev`

Authoritative design for s009. Prose in [DESIGN.md](../../../DESIGN.md) stays the source of truth; this spec is
the resolved decision record the plan and implementation follow. This slice makes **mtt track its own
development**: `mtt` this repository with a hand-authored config whose flow gates are **task-aware**, and migrate
the **forward** product backlog onto mtt itself. After s009, `TASKS.md` is frozen and mtt is the live queue.

**This is not a normal CLI feature.** It is integration + config + committed data + docs, with **no production
logic change expected** in `pkg/`/`core`/`adapter`/`cli` (only the version string and new tests). If a real gap
surfaces during migration, it is TDD'd as a separate, in-scope enabler — but the working assumption is zero
logic change (the s008.5…s008.98 enablers already made self-host practical).

> **Model note — one axis, not two (decided 2026-07-10, superseding the two-tier reconciliation).** An earlier
> draft modelled a two-tier `phase`/`session` hierarchy whose `session` flow encoded our *working process*
> (brainstorm → plan → implement → review). That conflated two orthogonal axes: **process** (session/phase — how
> *we* work: ephemeral, executed, not queued) vs **product** (task/epic — how the *product* changes: the
> backlog). mtt should track the **product** axis. So s009 uses a **single `task` type** — the unit of product
> change — and drops the process hierarchy. **The rich 15-status flow stays**, re-read as *how a task matures*
> (design it → review → plan it → review → build it → review → done), not as our session mechanics: a task may
> take one or several work-sessions to walk, and the flow survives that. Structure comes from **dependencies +
> tags + priority**, not hierarchy. **Epic** (a group of related tasks) is a legitimate *product* concept but is
> **deferred** ("enough with deps + tags for now"); when epics land, the §4 "close the epic when its children
> are done" self-referential gate returns on the `epic` type (it left with the `phase` type).

## Goal

1. **A committed `.mtt/config.yaml`** for this repo: **one** custom type `task` whose flow is the full
   **15-status per-artifact review cycle**, with **task-aware gates** — a task-branch on the entry edge
   `→ speccing` via the `{{.ID}}` placeholder, an artifact-presence proxy on the spec/plan review edges, and
   `make check` on the implementation-review edges. Global `require: {who: true}`.
2. **Migrate the forward product backlog** onto mtt as committed `.mtt/tasks/*.yaml`: the concrete Phase-4 work
   (references / comments / actor-profiles / coding-demo + dangerous-ops attribution) as the **active queue**,
   plus the further-out chunks (former Phases 5–8) and the design think-items as **backlog** tasks (tag
   `backlog`, low priority). All `tbd`. Structure via **priority + tags + deps** (no hierarchy). Completed
   sessions are **not** migrated (git + `sessions/*.md` are their record).
3. **Guard + prove it:** a Go test that the committed config always loads+validates (asserting the one type and
   the key gated/named edges of the full flow), and a `testscript` e2e that proves the branch/gate/current
   **mechanism** on a minimal scratch config with fake commands (the real gate needs a real repo, so the e2e
   proves the mechanism, not git/`make check`, per s006/s007/s008).

## Decisions (brainstormed)

### Q1 — How much backlog to migrate? **Forward product backlog (open work), split active vs backlog.**
Migrate the **open** product backlog:
- **Active queue** — the concrete next work: `references`, `comments`, `actor-profiles`, `coding-template demo`,
  and **`dangerous-ops attribution`** (the elevated TASKS think-item — the ideal first self-hosted task). `tbd`,
  **no** `backlog` tag, priority-ordered (high→low).
- **Backlog** — the further-out chunks (former Phases 5–8: KB + search, Gantt + query, mtt-ui, external
  adapters) **and** the design think-items ("Later (think)": monotonic-minting, roles-on-edges, lost-update,
  node-actions, …). `tbd`, tag **`backlog`**, `--priority low`. "Plan it later" and "think about it" both live
  here — promotion = drop the `backlog` tag and start work (no re-parenting needed; there is no hierarchy).

**Completed** sessions (001–008.98) are **not** migrated — their record is git history + `sessions/*.md`. s009
itself is **not** created as a task (it is the migration act; its record is this spec + `sessions/009_dogfood.md`).

*Rejected:* full-history (every session as a task) — mechanical, low value; minimal-seed — under-delivers the
"mtt is the backlog" outcome.

### Q2 — Task model. **Single `task` type; structure via deps + tags + priority (no hierarchy).**

| type   | prefix | parents | default | role |
|--------|--------|---------|---------|------|
| `task` | `t`    | `[]`    | ✅       | the unit of product change; carries the full gated lifecycle |

- One type, freshly-minted flat ids (`t1`, `t2`…) — a new namespace, disjoint from the illustrative `e5_t2`
  bootstrap ids in the (soon-frozen) `TASKS.md`. `task` is `default: true` (`mtt add "…"` creates a task).
- **No hierarchy** (no `phase`/`epic`/`subtask`, no `parents`). Grouping/sequencing is **tags** (areas: `core`,
  `ux`, `kb`, `adapter`; state: `backlog`), **priority** (roadmap ordering), and **`depends_on`** (real
  prerequisites). *The shipped default template stays `epic/task/subtask`* — the single-type shape is a
  **self-host** choice, not a template change (no embedded-template edit).
- *Rejected:* two-tier `phase`/`session` — conflated process with product (model note above). *Rejected:*
  keeping `epic` now — deferred ("enough with deps + tags"); epic is product-valid and returns later (with its
  §4 children-done gate).

### Q3 — The `task` flow (15 statuses) + task-aware gates.

The committed `.mtt/config.yaml` wires **real** gates (the point of dogfood — mtt gates its own development). The
flow is the task's maturation, three artifact stages (**design → plan → implementation**), each
`do → <do>_review (adversarial subagent) → <do>_human_review (human sign-off) → next`; a `decline` bounces to
`<do>_fix → <do>_review` (the `_fix` bounce is the rework counter — history is signal).

Statuses (kind): `tbd`(initial) · `speccing`,`spec_review`,`spec_human_review`,`spec_fix`(active) ·
`planning`,`plan_review`,`plan_human_review`,`plan_fix`(active) ·
`in_progress`,`impl_review`,`impl_human_review`,`impl_fix`(active) · `done`,`cancelled`(terminal). (15)

**Edge pattern** (same for the three stages; the gate differs — proxy for spec/plan, `make check` for impl):

| edge | name | gate | current |
|---|---|---|---|
| `tbd → speccing` | `start` | — | **set** + `git switch -c task/{{.ID}} \|\| git switch task/{{.ID}}` |
| `<do> → <do>_review` | `submit` | spec/plan: `git status --porcelain \| grep -qv '\.mtt/'` · impl: `make check` | — |
| `<do>_review → <do>_human_review` | `approve` | — | — |
| `<do>_review → <do>_fix` | `decline` | — | — |
| `<do>_human_review → <next-do>` | `approve` | — | impl→done: **clear** |
| `<do>_human_review → <do>_fix` | `decline` | — | — |
| `<do>_fix → <do>_review` | `submit` | same as first submit (proxy / `make check`) | — |
| `{tbd,speccing,planning,in_progress,spec_fix,plan_fix,impl_fix} → cancelled` | `cancel` | — | **clear** |

where `<next-do>` is `spec_human_review→planning`, `plan_human_review→in_progress`, `impl_human_review→done`.
(26 edges total: 1 entry + 6×3 stage edges + 7 cancel.)

- **Edge names** `start / submit / approve / decline / cancel` are **disjoint from all status names**, **unique
  per source status**, and every `(from,to)` pair is **unique** — the three s008.98 invariants hold, so the
  edge-verb sugar (`mtt approve t5`, `mtt decline t5`, `mtt submit t5`) resolves unambiguously. `cancel` ≠
  `cancelled` (edge name vs status name).
- **Abandon path (no forward-trap).** `submit` is one-way out of a `do` status, so without care a task that
  entered a review cycle could never reach `cancelled`. `cancel` therefore fires from the three `do` statuses,
  `tbd`, **and the three `_fix` statuses**; the `_review`/`_human_review` statuses reach `cancelled` in one step
  (`decline → _fix → cancel`).
- **`make check` is the only heavy gate** — on **every edge into `impl_review`** (`in_progress → impl_review`
  and `impl_fix → impl_review`), so `impl_review` is only ever entered on green; not repeated on `→ done`.
  Gate-cost (S5, accepted): ~1 full `make check` per impl submit/resubmit; `command_timeout: 10m` (headroom for
  a first-run lint + `-race` compile; the code default is 5m).
- **Artifact-presence proxy (spec/plan)** — `git status --porcelain | grep -qv '\.mtt/'` exits 0 iff there is
  an **uncommitted change outside `.mtt/`** (a **coarse** proxy: *some* uncommitted work exists outside `.mtt/`,
  keyed to the uncommitted-until-review convention below — it can't tell a spec from a plan from any dirty file;
  decision F accepts this). It fails **closed** on a git error. It excludes `.mtt/` because the accumulating
  `.mtt/tasks/*.yaml` churn (committed only with the PR — S4) would otherwise make a bare `grep -q .` trivially
  pass (finding **F**). **Semantics (documented):** the artifact stays **uncommitted until its `_human_review`
  approves** — the review is over the working tree; the commit happens after sign-off / with the PR. (impl is
  unaffected — it gates on `make check`, and code is committed per TDD cycle.)
- **Branch on entry** — one branch per task: `git switch -c task/{{.ID}} || git switch task/{{.ID}}` on
  `tbd → speccing` (DESIGN-canonical `task/` prefix). Idempotent (re-entering an existing branch is correct for
  re-taking a task into work), **no `rollback:`** (finding **S2/U1**: the `git branch -D` compensator is broken
  and a rollback only fires on a *later* command's failure — here the branch command is the only command on the
  edge). A task's spec + plan + code all live on its branch, over one or several work-sessions; one PR at `done`.

**YAML authoring of gate commands (trap — verified against `yaml.v3`).** Author every gate command as a
**single-quoted** YAML scalar (or a literal block scalar), **never double-quoted** and **never a plain scalar
starting with `!`**. Double-quoting breaks the proxy — `\.mtt/` is an invalid escape → `Load` fails and the
whole config is unloadable — and the shipped `coding.yaml` uses the double-quoted `["! make test"]` convention,
so this is the *easy* mistake. A plain scalar beginning `! …` has its `!` parsed as a YAML tag and **silently
dropped** (inverting the gate). `TestRepoDogfoodConfig` guards this by asserting the **exact** command strings
(including any leading `!` and the `\.mtt/`), never substrings.

**Bootstrap caveats (documented):** mtt-minted ids (`t1`…) differ from the docs' historical `sNNN` labels (the
mtt id is the going-forward identity); the branch carries **no slug** (`task/t1`, not `task/t1-references`) —
placeholders are a **structural whitelist** (`.ID`/`.Type`/`.From`/`.To`; free text is never interpolated,
s007); s009 itself runs on the manually-created `feat/s009-dogfood` (the config governs **future** tasks — no
migrated task is transition-driven through the gate during s009).

#### Attribution — project-global `require: {who: true}` (finding on decision E)

**Constraint (verified in code):** `require:{who,why}` in mtt is **project-global** (`Settings.Require` at the
committed-config root, `config.local` tighten-only, checked in `core.Transitioner` for *every* edge). There is
**no per-edge `require`** on `pkg/mtt.Transition`. Decision E's "`require` on the human-review edges" is
therefore **not expressible without a core change** — out of scope. Per-edge / role-based attribution is the
**parked roles work** (E names this its first concrete unpark trigger).

**Resolution:** the committed config sets **`require: {who: true}`** globally. `who` is auto-satisfied by the
`config.local.yaml` `author`, so every move records a real `by:` — a self-approval of a `_human_review` edge is
**visible** in history as `by: <the agent>` (honest, not enforced). `why` stays optional (`--why` where it
matters; a global `require:{why}` would force `--why` on every one of 26 edges — too heavy).

**Setup / first-run papercut (documented, not a blocker):** because `require` is global, it applies to *every*
edge (incl. mechanical `submit`/`decline`/`cancel`), and `--no-run` does **not** bypass it — so each agent must
set `author` in `config.local.yaml` (gitignored) or `MTT_BY`/`--by` **before the first move**, or moves exit 2
on a fresh checkout. A one-time setup step; noted in the dogfood docs.

## Architecture (resolved)

`cli → core → port ← adapter` — **unchanged**. s009 adds **no** ports, usecases, or CLI commands. The config is
**hand-authored** in `.mtt/config.yaml` (not produced by `mtt init` — init emits the command-less `default`
template; our type is bespoke). No new **embedded** template is added. The self-hosted `.mtt/` (config + tasks)
is **committed**; `.mtt/config.local.yaml` is **already** gitignored. Existing tests are unaffected: the e2e
`testscript` suites run in `$WORK` temp dirs, so they never `FindRoot` this repo's new `.mtt/`.

`make build` (→ `./bin/mtt`) drives the migration `add` calls; `make install` (mtt on `PATH`) is **recommended
for daily use** but no longer **required by any gate** (the flow gates are `make check` + git — no mtt-self-ref
gate now that the `phase` type is gone).

### Security / trust (SEC2)

**Gates may invoke read-only `mtt` commands only.** (No self-ref gate ships in this single-type config, but the
rule stands for any future gate — a gate must never invoke an mtt **transition**: that could recurse.) The
config-as-code trust model is otherwise unchanged (commands are trusted project config, like a Makefile;
placeholder injection defense is structural — only `.ID/.Type/.From/.To` reach a command).

### Task-file commit discipline (S4)

`.mtt` mutations (every `add`/`status`/`do` writes `.mtt/tasks/*.yaml`) are **committed with the task's PR**;
after `approve → done` (or before the merge), run `git add .mtt && git commit` so the task file lands in its
final status. Between `add` and the squash-merge the task is invisible on `main` — accepted (solo).

### Migration content (forward-only, flat)

Created with the built binary (`./bin/mtt add …`, scripted deterministically), the resulting `.mtt/tasks/*.yaml`
committed. Everything is `tbd`; `current` unset. Ordering in `mtt roadmap` comes from **priority** (+ real
`depends_on` where they exist); ties are recency-ordered (S3), so **`mtt roadmap` is hand-run and eyeballed
before committing** `.mtt/tasks/*.yaml`.

- **Active queue** (no `backlog` tag): `references` (**high**), `comments` (medium), `actor profiles` (medium),
  `coding-template demo` (low), `dangerous-ops attribution` (priority set so the roadmap reads sensibly — verified
  by the hand-run). Each carries a one-line description mirroring its `sessions/README.md` / TASKS row + area
  tags.
- **Backlog** (tag `backlog`, `--priority low`): the former Phases 5–8 as coarse tasks (KB + search, Gantt +
  query, mtt-ui, external adapters) and the design think-items ("Later (think)"). The exact enumeration is
  finalized during implementation by reading `TASKS.md`, so the freeze empties both the active plan and the
  design backlog into mtt.

Note: backlog tasks (`tbd`, no deps) will surface in `mtt ready`; there is no "exclude tag" filter, so
`mtt list --tag backlog` is the backlog view and `ready` shows everything workable (accepted for v1; a
"hide-tag-from-ready" filter is itself a backlog item).

## Acceptance (must pass)

- **User scenario (real config):** in the repo, `mtt types` shows `task` with the gated flow; `mtt list` /
  `mtt roadmap` / `mtt ready` render the migrated active queue + backlog; `mtt list --tag backlog` filters the
  backlog.
- **Committed-config guard (Go test, genuine red→green):** `TestRepoDogfoodConfig` — `FindRoot` locates this
  repo's `.mtt/`, `Load` + `Config.Validate()` are green, and it asserts:
  - exactly **one** type (`task`), `default: true`, prefix `t`.
  - the flow has the **15 statuses** with the right kinds; the entry edge `tbd → speccing` carries the
    `git switch … task/{{.ID}}` branch command **and** `current: set`; the edges into `impl_review`
    (`in_progress → impl_review`, `impl_fix → impl_review`) gate on `make check`; the spec/plan submit edges
    carry the `grep -qv '\.mtt/'` proxy; `impl_human_review → done` has `current: clear`; the named edges
    (`start`/`submit`/`approve`/`decline`/`cancel`) exist and satisfy the disjointness/uniqueness invariants.
    Gate assertions compare the **exact** command strings (the `make check`, the proxy including `\.mtt/`) — not
    substrings — so a YAML-mangled (double-quote-broken) gate is caught by the guard, not at runtime.
  - the project sets `require: {who: true}`.
  A CI-forever guard against a broken committed config (`Config.Validate` runs on `add`/`types`, **not** on
  `Load` — this test is the **sole** guard, S6). Red before `.mtt/config.yaml` exists → green after.
- **Mechanism e2e (`testscript` `dogfood.txt`):** a **scratch** config (txtar `-- gated.yaml --` `cp`'d over
  `.mtt/config.yaml`) — a **minimal valid flow** (`initial → active → terminal`, not all 15 statuses) mirroring
  the *mechanism* with **fake** commands — proves: `mtt types` validates (run **before** the first move — §9
  precondition); the entry edge runs `git switch -c task/{{.ID}}` → the branch exists (`git symbolic-ref --short
  HEAD`, guarded `[!exec:git] skip`, `git symbolic-ref` for the unborn branch — s007 lesson) and sets `current`;
  a `→ <review>` edge with a **failing** gate command **blocks** (non-zero — task unchanged, no history; the
  exact exit 3 is unit-tested, `testscript` asserts only non-zero); with a **passing** gate command **moves** and
  **clears** `current` on the terminal edge. Proves the mechanism, not the real `make check` (a temp dir has no
  Makefile — the s006/s007/s008 e2e strategy).
- `make check` green.

## Out of scope (explicitly deferred)

- **Epics / hierarchy** (the `epic` type + its §4 children-done gate) — product-valid, deferred ("enough with
  deps + tags"); returns in a later session (a migrated backlog item).
- **Re-parenting** (`mtt edit --parent`) — not needed under the flat single-type model; a backlog item if epics
  return.
- Migrating **completed** sessions into mtt (stay in docs + git).
- A new **embedded template** or `mtt init --template mtt`.
- **Per-edge / role-based `require`** (decision E's full form) — needs a core change; the parked roles work,
  designed in the migrated dangerous-ops task.
- **Bulk transition** migration; **monotonic-id** / lost-update / scale-stress (surfaced, not built); changing
  the s009 branch workflow itself (bootstrap on the manual `feat/s009-dogfood`).

## Docs sync (same session)

`DESIGN.md`/`.ru` (a "Dogfooding / self-host" note incl. the bootstrap caveat + SEC2 read-only-gate rule + the
S4 commit-discipline line + the process-vs-product model note); `CLI_REFERENCE.md`/`.ru` (a brief self-host
mention if warranted — likely minimal); `docs/architecture/model.go` (a note only if a decision touches the
contract — none expected); **`TASKS.md` frozen** (a banner + `e5_t2 ✅`; the task plan is superseded by mtt, the
design backlog migrated as `backlog` tasks); `sessions/README.md` (009 ✅, 010 ← next); `NEXT_SESSION.md`
("Where we are" + "Next task = s010 references" + "Carry-over lessons (009)"); `sessions/009_dogfood.md` (Done
filled); the flow-granularity note §10 (model pivot recorded); version `0.8.98-dev → 0.9.0-dev`
([internal/cli/root.go](../../../internal/cli/root.go)); any package `CLAUDE.md` only if a package changes
(expected: none).

## Definition of Done

- `.mtt/config.yaml` (single `task` type + task-aware gated 15-status flow + `require:{who}`) and
  `.mtt/tasks/*.yaml` (forward backlog: active queue + `backlog`-tagged) committed.
- `TestRepoDogfoodConfig` green; `dogfood.txt` e2e green; `make check` green.
- Docs synced (above); version bumped.
- Branch `feat/s009-dogfood` → PR → CI green → squash into `main`.
