# t19 — migrate agent-process rules into the flow descriptions (flow-led process)

Status: design spec (decision record) for t19. Input: the 2026-07-11 docs-vs-config audit (chat) applying
the prime directive (AGENTS.md TL;DR #0) after t21/t5/c1 mechanized the git mechanics. Goal: **mtt leads
the process** — every process detail expressible in the flow lives in the flow; agent docs keep only
discipline, principles, and what the flow cannot yet express.

## Decisions

### D1 — Method steps move into status descriptions (config change)

The superpowers method (brainstorm → spec → plan → review) currently lives in AGENTS.md/CLAUDE.md prose;
the statuses where those steps happen already print guidance on entry (s008.95). New texts (task type):

- `speccing`: `brainstorm first (superpowers:brainstorming), then write the design spec — a decision
  record — to docs/superpowers/specs/<this-task-id>-<slug>.md (commit early and often), then `mtt submit``
- `planning`: `write the implementation plan (superpowers:writing-plans) to
  docs/superpowers/plans/<this-task-id>-<slug>.md, then `mtt submit``
- `impl_review` (task): `run an adversarial code review: the AGENTS.md Principles self-check + Go
  conventions, and DESIGN.md/CLAUDE.md updated if behavior changed; `mtt approve` when it passes,
  `mtt decline` to send back`
- `impl_review` (chore): same checklist PLUS the type-boundary police line, which MUST keep the
  guard-asserted `it must be a` substring and now also carries the recovery step (review MINOR-5): `run
  an adversarial code review: the AGENTS.md Principles self-check + Go conventions, and
  DESIGN.md/CLAUDE.md updated if behavior changed; if the diff contains design decisions not recorded
  elsewhere — decline: it must be a `task` (cancel this chore and recreate).
  `mtt approve` / `mtt decline``
- `approved` (both types): name the one manual command instead of describing it: `open/update the PR:
  gh pr create --title '<this-task-id>: <title>' (the branch was auto-pushed), ask the human to merge;
  after the squash-merge run `mtt deliver`; human-requested changes -> `mtt decline``

Unchanged: `implementing` (already carries TDD + make check), `deliver`/`cancel`/entry edges, all gates
and post actions. Guard test: keep the two existing spot-checks green (`it must be a`, `pull main`) and
ADD two for the new load-bearing strings — `superpowers:brainstorming` in task `speccing`,
`gh pr create` in both `approved` statuses.

### D2 — AGENTS.md shrinks to discipline + principles + the not-yet-expressible

- **Definition of Done section** → replaced by: the DoD *is* the flow — each status prints its
  instructions on entry and in `mtt show` (NOTE, review MAJOR-1: `mtt types` renders type + edge
  descriptions but NOT status descriptions — point readers at `mtt show`/entry guidance); what stays on
  the agent: test-before-code, the Principles self-check, docs-sync judgment (all three referenced by
  the `impl_review` descriptions, D1).
- **Working under mtt** — bullets that duplicate what the flow prints are cut to pointers: the two-type
  litmus (lives in the type descriptions), id-keyed artifacts + commit-early (speccing/planning
  descriptions), PR-title + delivery mechanics (approved/deliver descriptions). Bullets that stay:
  backlog navigation (roadmap / backlog tag — pre-flow, no home in the flow yet → t29), attribution
  setup before the first move (pre-flow → t28 for the error-hint), mid-flight resumption
  (`git switch task/<id>`), dangerous-ops summary, auto-commit/auto-push mechanics summary (one bullet,
  incl. exit-5 recovery), config-is-code (SEC2).
- **Sessions section** → rewritten: the unit of work is an mtt task on a `task/<id>` branch; the
  superpowers method steps live in the flow (D1); `sessions/*.md` is the narrative archive for process
  milestones, not a per-task requirement (the apparatus decision itself is t31, out of scope here).
- **Git section** (review MAJOR-2): the `Branches: feat/…, fix/…, chore/…` bullet is updated in t19 —
  mtt work runs on flow-created `task/<id>` branches; `feat/…`/`chore/…` remain only for non-task
  exceptions (bootstrap/infra).
- Out of scope for t19 (queued as **c2**, which lands AFTER t19 and rebases on it): the push-rule
  contradiction and the model-specific commit trailer. c2's third item (the stale approved-push line)
  is likely deleted by t19's dedup of the "Delivery is verified" bullet — c2 then just verifies it is
  gone, in AGENTS.md AND in DESIGN.md's dogfood note (+ ru mirror), which carries the same stale
  sentence (review MINOR-6; c2's description gets widened accordingly).

### D3 — Root CLAUDE.md sync

Reading order becomes `AGENTS.md → DESIGN.md → mtt roadmap` (TASKS.md is frozen history); the header
line "task plan — in TASKS.md" becomes "the live queue — `mtt roadmap`" (review MINOR-4); the
non-negotiables bullet "Per-task branch → PR → CI green → squash into main" gains "(the flow creates
and pushes the branch; see AGENTS.md Working under mtt)".

## Not expressible yet (stays in docs; queued separately)

Attribution/exit-5 error hints (t28), pre-flow knowledge home / `mtt guide` (t29), version policy after
sessions (t30), sessions/NEXT_SESSION apparatus (t31), exact ids in descriptions (t16 — placeholders in
descriptions would turn `<this-task-id>` into the real id).

## Acceptance

- `.mtt/config.yaml` carries the D1 texts; a move into each touched status — and `mtt show` on a task
  resting there — prints them (status descriptions do NOT appear in `mtt types`; that stays as-is).
- `TestRepoDogfoodConfig` green with the two new description spot-checks (and the two old ones).
- AGENTS.md: DoD replaced per D2, Working-under-mtt deduped, Sessions rewritten; no sentence in
  AGENTS.md/CLAUDE.md contradicts what the flow prints.
- Root CLAUDE.md per D3. `make check` green. (Docs-only + config + guard-test change — no production code.)
