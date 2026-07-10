# 009 — Dogfood / self-host

Status: planned   ·   Branch: `feat/s009-dogfood`

## Target

Make mtt **track its own development**: `mtt` this repository with a hand-authored config whose flow gates are
**task-aware** (a task-branch on the entry edge `→ speccing` via `{{.ID}}`; an artifact proxy on the spec/plan
review edges; `make check` on the implementation-review edges), and migrate the **forward product backlog** onto
committed `.mtt/tasks/*.yaml`. After s009, `TASKS.md` freezes and mtt is the live queue. **Not a normal CLI
feature** — integration + config + data + docs, ~no production logic change.

**Model:** one axis — **product** (`task` = unit of product change), not **process** (session/phase = how we
work). A single `task` type carries the full 15-status maturation flow (design → plan → impl, each with review);
structure is **deps + tags + priority** (no hierarchy). Epics are product-valid but deferred.

## Scope

- **In:**
  - Hand-authored **`.mtt/config.yaml`** (committed): **one** type `task` (`t`, `default`, no parents). Project-
    global **`require: {who: true}`**. `command_timeout: 10m`.
    - **`task` flow (15 statuses):** `tbd` → three artifact stages (design/plan/impl), each
      `do → _review (adversarial subagent) → _human_review (human) → next`, with `decline → _fix → _review`.
      Entry `tbd → speccing` (`start`): `current:set` + idempotent branch `git switch -c task/{{.ID}} || git
      switch task/{{.ID}}` (no rollback — U1). Named edges `start/submit/approve/decline/cancel` (disjoint from
      status names). **`make check`** on every edge into `impl_review`; **proxy** `git status --porcelain | grep
      -qv '\.mtt/'` on the spec/plan submit edges (artifact uncommitted until `_human_review` approves);
      `current:clear` on `→ done` and `→ cancelled`. `cancel` fires from
      `{tbd,speccing,planning,in_progress,spec_fix,plan_fix,impl_fix}` (no forward-trap — a review cycle reaches
      `cancelled` via `decline → _fix → cancel`). Gate commands are **single-quoted** YAML scalars (double-quote
      breaks `\.mtt/`; a plain `!` scalar is silently dropped — verified vs yaml.v3).
  - **Forward product backlog migrated** to committed `.mtt/tasks/*.yaml` (via `./bin/mtt add …`), flat:
    - **active queue** (no tag): references **high** / comments / actor-profiles / coding-demo **low** /
      dangerous-ops attribution.
    - **backlog** (tag `backlog`, `--priority low`): former Phases 5–8 (KB+search, Gantt+query, mtt-ui, external
      adapters) + design think-items ("Later (think)"). Promotion = drop the `backlog` tag + start work (no
      re-parenting — no hierarchy).
    - All `tbd`; ordering via **priority + tags + deps**, **`mtt roadmap` hand-run & eyeballed** before commit
      (S3); `current` unset. (Backlog tasks surface in `ready`; `list --tag backlog` is the backlog view.)
  - **`TestRepoDogfoodConfig`** — Go test guarding the committed config (FindRoot repo `.mtt/`, Load+Validate;
    asserts 1 type `task` default + prefix, the 15 statuses/kinds, the entry-branch + `current:set`, the
    `make check` impl-review gates, the spec/plan proxy, the `→done` clear, the named-edge invariants,
    `require:{who}`; **exact** command strings). The **sole** guard — Validate is not called on Load (S6).
  - **e2e `dogfood.txt`** — a **minimal valid** scratch config (fake commands) proving the mechanism:
    `mtt types` before the first move (§9 precondition), branch + `current:set` on the entry edge, gate
    block/allow (non-zero / move) + `current:clear`, `[!exec:git] skip` (+ `git symbolic-ref` for the unborn
    branch — s007).
  - Docs sync + version `0.8.98-dev → 0.9.0-dev`.
- **Out (deferred):** **epics/hierarchy** (+ the §4 children-done gate — returns with epics); **re-parenting**
  (`edit --parent` — unneeded flat); migrating completed sessions; a new embedded template / `mtt init
  --template mtt`; **per-edge/role `require`** (E's full form — needs a core change, designed in the dangerous-ops
  task); bulk-transition; monotonic-id / lost-update / scale-stress; changing the s009 branch workflow itself.

## Decisions (brainstormed — see the spec)

Spec: [../docs/superpowers/specs/2026-07-09-session-009-dogfood-design.md](../docs/superpowers/specs/2026-07-09-session-009-dogfood-design.md)
(reconciled + re-modelled 2026-07-10). Flow basis: [flow-granularity note §9/§10](../docs/superpowers/notes/2026-07-09-flow-granularity-for-dogfood.md).
Plan: `docs/superpowers/plans/2026-07-10-session-009-dogfood.md` (written next).

1. **Q1 — forward product backlog**, split active (no tag) vs `backlog` (tag, low). Completed sessions stay in
   git/docs. mtt = a live queue. s009 is not a task.
2. **Q2 — single `task` type** (prefix `t`, default, no hierarchy). Structure via deps + tags + priority. Epics
   deferred (product-valid, "enough with deps + tags"). The shipped default template stays `epic/task/subtask`.
3. **Q3 — full 15-status flow + task-aware gates.** Branch on entry `→ speccing`; `make check` on impl-review
   edges; artifact proxy on spec/plan review edges. **Attribution:** project-global `require:{who}` (per-edge
   require not expressible without a core change — E's full form parked). Findings folded: fail-closed forms,
   YAML single-quoting, cancel-from-`_fix`, uncommitted-until-review proxy semantics.

## Acceptance (must pass)

- **User scenario:** `mtt types` shows the gated `task` flow; `mtt list`/`roadmap`/`ready` render the migrated
  active queue + backlog; `mtt list --tag backlog` filters the backlog.
- **Committed-config guard:** `TestRepoDogfoodConfig` green (genuine red→green), asserting the full flow above.
- **e2e:** `testscript` `dogfood.txt` — `mtt types` first, branch + `current:set` on entry, gate block on a
  failing command (non-zero, unchanged) / move + `current:clear` on a passing command, `[!exec:git] skip`.
- `make check` green.

## Plan (refine at session start — test-first)

- [ ] (written by writing-plans next)

## Done (fill during/after the session)

<pending>
