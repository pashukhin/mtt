# 009 — Dogfood / self-host

Status: planned   ·   Branch: `feat/s009-dogfood`

## Target

Make mtt **track its own development**: `mtt` this repository with a hand-authored config whose gates are
**task-aware** (a session-branch on the entry edge `→ speccing` via `{{.ID}}`; `make check` on the
implementation-review edges; an artifact-presence proxy on the spec/plan review edges; a `phase` can't close
while it has open child sessions), and migrate the **forward** (open) backlog onto committed `.mtt/tasks/*.yaml`.
After s009, `TASKS.md` freezes and mtt is the live queue. **Not a normal CLI feature** — integration + config +
data + docs, ~no production logic change.

## Scope

- **In:**
  - Hand-authored **`.mtt/config.yaml`** (committed): **two** custom types (decision C — `step` dropped):
    `phase` (`p`, root, self-ref gate on `→ done`) / `session` (`s`, `default`, `parents:[phase]`, the full
    gated method). Project-global **`require: {who: true}`**. `command_timeout: 10m`.
    - **`session` (15 statuses, full per-artifact review cycle):** `tbd` → three artifact stages
      (design/plan/impl), each `do → _review (adversarial subagent) → _human_review (human) → next`, with
      `decline → _fix → _review`. Entry `tbd → speccing` (`start`): `current:set` + idempotent branch
      `git switch -c feat/{{.ID}} || git switch feat/{{.ID}}` (no rollback — U1). Named edges
      `start/submit/approve/decline/cancel` (disjoint from status names). **`make check`** on every edge into
      `impl_review`; **proxy** `git status --porcelain | grep -qv '\.mtt/'` on the spec/plan submit edges
      (artifact uncommitted until `_human_review` approves); `current:clear` on `→ done` and `→ cancelled`.
      `cancel` fires from `{tbd,speccing,planning,in_progress,spec_fix,plan_fix,impl_fix}` (no forward-trap — a
      review cycle reaches `cancelled` via `decline → _fix → cancel`). Gate commands are **single-quoted** YAML
      scalars (double-quote breaks `\.mtt/`; a plain `!` scalar is silently dropped — verified vs yaml.v3).
    - **`phase` (4 statuses):** `tbd → in_progress → done` (+`cancelled`); no branch/current; **fail-closed**
      `finish` gate on `→ done`: `out=$(mtt list --parent {{.ID}} --kind initial --kind active --ids) && test -z "$out"`
      (§4, read-only; the `! … | grep -q .` form is fail-open when `mtt` is missing/errors — avoided).
  - **Forward backlog migrated** to committed `.mtt/tasks/*.yaml` (via `./bin/mtt add …`): Phase-4 phase (`p1`) +
    sessions (references **high** / comments / actor-profiles / coding-demo **low** / **dangerous-ops
    attribution**); bare Phase-5…8 phases (`p2`–`p5`, `--priority low`). All `tbd`; ordering via **priorities**,
    **`mtt roadmap` hand-run & eyeballed** before commit (S3); `current` unset.
  - **`TestRepoDogfoodConfig`** — Go test guarding the committed config (FindRoot repo `.mtt/`, Load+Validate;
    asserts 2 types, `session` default, prefixes, the 15 statuses/kinds, the entry-branch + `current:set`, the
    `make check` impl-review gates, the spec/plan proxy, the `→done` clear, the named-edge invariants, the phase
    self-ref gate, `require:{who}`). The **sole** guard — Validate is not called on Load (S6).
  - **e2e `dogfood.txt`** — a **minimal valid** scratch config (fake commands) proving the mechanism:
    `mtt types` before the first move (§9 precondition), branch + `current:set` on the entry edge, gate
    block/allow (exit 3 / move) + `current:clear`, `[!exec:git] skip` (+ `git symbolic-ref` for the unborn
    branch — s007).
  - Docs sync + version `0.8.98-dev → 0.9.0-dev`.
- **Out (deferred):** migrating completed sessions / think-items / parked work (stay in docs); a new embedded
  template / `mtt init --template mtt`; **per-edge/role `require`** (E's full form — needs a core change, the
  parked roles work, designed in the dangerous-ops session); bulk-transition migration; monotonic-id /
  lost-update / scale-stress; changing the s009 branch workflow itself (bootstrap on manual `feat/s009-dogfood`).

## Decisions (brainstormed — see the spec)

Spec: [../docs/superpowers/specs/2026-07-09-session-009-dogfood-design.md](../docs/superpowers/specs/2026-07-09-session-009-dogfood-design.md)
(reconciled 2026-07-10). Flow basis: [flow-granularity note §9 C–G](../docs/superpowers/notes/2026-07-09-flow-granularity-for-dogfood.md).
Plan: `docs/superpowers/plans/2026-07-10-session-009-dogfood.md` (written next).

1. **Q1 — forward-only migration.** Only open work (Phase-4 sessions + bare Phase-5…8 phases). Completed
   sessions, think-items, and parked work stay in git/docs. mtt = a live queue, not an archive. s009 is not a
   task (it is the migration act).
2. **Q2 — two-tier `phase`/`session`** (decision C — `step` dropped; prefixes `p`/`s`, non-overlapping; fresh id
   namespace). `session` is the `default`; a root `phase` adds freely; `session.parents=[phase]`. The shipped
   default template stays `epic/task/subtask`.
3. **Q3 — full session flow + task-aware gates.** 15-status per-artifact review cycle (decision A/D). Branch on
   entry `→ speccing`; `make check` on impl-review edges; artifact proxy on spec/plan review edges; phase self-ref
   gate on `→ done`. **Attribution:** project-global `require:{who}` (per-edge require not expressible without a
   core change — decision E's full form parked). **Proxy caveat:** artifact stays uncommitted until
   `_human_review` approves (the `.mtt` churn defeats a bare `grep -q .` — finding F). Bootstrap caveats
   documented (mtt ids ≠ doc `sNNN`; no slug in branch — placeholder whitelist; s009 runs on the manual branch).

## Acceptance (must pass)

- **User scenario:** in the repo, `mtt types` shows the two gated types; `mtt list`/`tree`/`roadmap` render the
  migrated Phase-4 hierarchy + open sessions.
- **Committed-config guard:** `TestRepoDogfoodConfig` green (genuine red→green), asserting the full flow above.
- **e2e:** `testscript` `dogfood.txt` — `mtt types` first, branch + `current:set` on entry, gate block on a
  failing command (exit 3, unchanged) / move + `current:clear` on a passing command, `[!exec:git] skip`.
- `make check` green.

## Plan (refine at session start — test-first)

- [ ] (written by writing-plans next)

## Done (fill during/after the session)

<pending>
