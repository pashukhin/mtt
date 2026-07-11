# Flow v2 — mechanized delivery, chore type, honest gates (design spec)

Date: 2026-07-11 · Branch: `feat/s009-dogfood` (amends PR #23 pre-merge) · No version change (rides s009's `0.9.0-dev`)

Authoritative decision record for the flow-v2 rework, produced by the post-s009 adversarial review
(10 confirmed findings) and a brainstorm with the user. Supersedes the flow shape in
`2026-07-09-session-009-dogfood-design.md` §Q3 where they conflict; that spec's Q1/Q2 (forward-only
migration, single product-task model) stand. Prose home after shipping: DESIGN.md.

## Principles (fixed by the user — design invariants)

- **P1 — gates check form, never content.** Content evaluation is what agents are for; the flow's
  review statuses are the controlled injection points for agent/human judgment. Never pretend a shell
  command verifies meaning.
- **P2 — change the process to fit mechanization, not vice versa.** Project rules, conventions, and
  agent instructions are all mutable in service of moving mechanical burden into mtt. Discipline is a
  last resort reserved for what cannot be mechanized at all.
- **P3 — a new task type is justified iff its flow is guaranteed different.**
- **P4 — many small PRs are good** (easier review); 1 task = 1 branch = 1 PR is the desired shape.
- **P5 — delivery to `main` is part of the task lifecycle** and deserves statuses.

## Root cause being fixed

The YAML store lives in the git working tree, so every task-state write lands on whatever branch is
checked out, while task state is logically global. Two confirmed holes followed: a `cancel` write
strands on the abandoned task branch (task resurrects as `tbd` on main), and the entry edge branches
from current HEAD with no return to main (task branches stack; uncommitted artifacts leak across
tasks and satisfy the next task's proxy gate). A third cluster: the `git status --porcelain` proxy
gate blocks silently, its precondition ("artifact uncommitted until review") lives outside the flow,
and its `_fix → _review` instances can never block.

## Decisions

### D1 — Sequencing: fix inside PR #23 with existing mechanisms; new mechanism as a follow-up mtt task

Everything below is expressible with today's product (config + tests + docs) and rides PR #23. The one
missing product mechanism — **post-persist actions** (D10) — becomes one of the first self-hosted
tasks. Until it lands there are **exactly two** manual steps (D10); everything else is mechanized.

### D2 — Artifact gates: id-keyed filenames replace the porcelain proxy; commit-early replaces the uncommitted convention

New process convention (P2): a task's design/plan artifacts are named by its id —
`docs/superpowers/specs/<id>-<slug>.md` and `docs/superpowers/plans/<id>-<slug>.md` (existing files are
not renamed). The spec/plan submit gates become honest existence checks:

```yaml
commands: ['ls docs/superpowers/specs/{{.ID}}-*.md']
```

- Diagnosable: on failure `ls` prints a clear message to stderr — the s008.97 output tail shows it
  (the porcelain proxy was silent by construction: empty tail, dead `-v` hint).
- The "artifact stays uncommitted until human review" convention **dies**: commit early and often (the
  natural agent behavior). This is what makes D4's `git switch main` safe.
- The `_fix → _review` resubmit gates keep the same command and are now meaningful (artifact must
  exist) instead of provably toothless.
- When references land (t1/s010), the glob upgrades naturally to a ref-resolve gate.
- Note: edge `description`s cannot interpolate `{{.ID}}` today (backlog t16 — this is its concrete
  trigger); descriptions phrase the convention generically ("write the spec to
  docs/superpowers/specs/<this-task-id>-<slug>.md").

### D3 — Delivery tail: `approved → deliver → done`, on main, verified by the squash-commit trace

`done` must mean "in main". New status **`approved`** (= agent review passed, awaiting the human
merge) and edge verb **`deliver`**:

- **New process convention (P2, accepted): the PR title — and therefore the squash-commit subject —
  starts with the task id:** `t1: references — …`, `c3: fix stale AGENTS preamble`. The colon makes
  the match prefix-safe (`^t1:` cannot match `t10:`).
- `deliver` runs standing wherever; its commands mechanize the delivery form-check and put the state
  write on main:

```yaml
- from: approved
  to: done
  name: deliver
  current: clear
  description: "after the PR is squash-merged: pull main, then deliver (writes done on main)"
  commands:
    - 'git switch main'
    - 'git log -n 50 --format=%s | grep "^{{.ID}}: " || { echo "no squash commit \"{{.ID}}: …\" on local main — git pull first" >&2; false; }'
```

  The first command moves the working tree to main **before** the task-file write (commands run
  pre-write), so `status: done` lands on main. The second fails closed with a self-explanatory stderr
  line (shown by the s008.97 tail) when the merge hasn't landed on the *local* main yet — no network
  in gates, per the security posture.
- Human rejection at the PR stage is expressible: `approved --decline--> impl_fix`.

### D4 — Entry edge: re-enter first, else branch from main

```yaml
commands: ['git switch task/{{.ID}} || (git switch main && git switch -c task/{{.ID}})']
```

Re-entering an existing task branch stays first (idempotent retake); a **new** branch is always born
from main. A dirty tree that conflicts makes `git switch` fail closed. With D2's commit-early
convention the tree is normally clean at `start`.

### D5 — Cancel lands on main

Every cancel edge gets `commands: ['git switch main']` (pre-write), so the `cancelled` write lands on
main — not on the branch that is about to be abandoned. `current: clear` stays. The state commit on
main is one of the two interim manual steps (D10). Cancel sources (no forward-trap): every
non-terminal status except the `_review` pairs — including **`approved`** (PR closed without merge).

### D6 — Second type `chore` (P3-justified: guaranteed different flow)

**Physical meaning.** `task` = a change whose **design space is open** (decisions to make and record;
spec/plan are their material traces; review cycles inject content judgment where being wrong is most
expensive). `chore` = a change whose **design is already fixed elsewhere** (a review finding with a
fix sketch, a backlog item with a recorded decision, docs sync, a dependency bump, a mechanical
refactor); its "spec" is the pointer to that source. Forcing a spec artifact on a chore produces
fiction that a form gate would happily accept — gate theater. A chore's residual risk is execution
only: mechanized (`make check`) + one agent review + the human merge.

**Flow** (7 statuses; prefix `c`):

```
tbd --start--> implementing --submit--> impl_review --approve--> approved --deliver--> done
                    ^                        |decline               |decline
                    |                        v                      v
                    +-------submit------- impl_fix <----------------+          (+cancelled)
```

- `start`/`submit(make check)`/`deliver`/cancels — identical mechanics to `task` (the types share the
  delivery tail; they differ in the head, which is what P3 requires).
- The `impl_review ⇄ impl_fix` loop is the single content-check before the human (P1). Its
  `description` carries the **type-boundary police instruction**: "if this diff contains design
  decisions not recorded elsewhere — decline: it must be a `task`". The boundary cannot be
  form-checked; the review cycle is exactly the controlled place for that judgment.
- No mid-cycle human status: the only human decision left for a chore is "merge or not", which
  physically happens as the PR merge; `approved` is the waiting state, `deliver` verifies the act's
  trace, `approved → decline → impl_fix` expresses requested changes.
- The type-choice litmus lives in each type's `description` (surfaced by `mtt types` — the
  self-instructing runbook): "what+how already decided and recorded elsewhere → chore; design
  decisions needed → task". A misfiled task is recategorized the mtt way: cancel + recreate (type is
  immutable by design).
- Immediate need (not speculation): the review fix-package itself is chore-class work.

### D7 — Consequence applied to `task`: the impl tail collapses (⚠ change vs the approved v1 flow)

The D6 logic ("a human act that physically happens as the merge must not be double-modeled as a
status") applies to `task`'s implementation stage equally: v1's `impl_human_review` sign-off and the
merge are the same human act. So `task` v2 ends
`impl_review --approve--> approved --deliver--> done`, with `approved --decline--> impl_fix`; the
mid-cycle human sign-offs that remain are the genuinely separate acts: `spec_human_review` and
`plan_human_review`. Status count stays 15 (`impl_human_review` out, `approved` in).

**Flagged for user review**: v1's three human sign-offs become two + the merge itself. The
alternative (keep `impl_human_review` *and* add `approved`, 16 statuses) double-models the merge and
was rejected in the D6 discussion for `chore`; consistency argues for the collapse.

### D8 — Timeout policy (review finding #9)

Global `command_timeout` returns to the 5m code default (drop the `command_timeout: 10m` line); the
one command needing headroom gets it explicitly:

```yaml
commands: [{run: 'make check', timeout: 10m}]
```

Millisecond git gates stop inheriting a 10-minute hang window (wedged-git case); `make check` keeps
its headroom. (First use of the s007 map form in the committed config — it is the feature's own demo.)

### D9 — Guard test and e2e brought up to the new flow (review findings #3, #6, #10)

- `TestRepoDogfoodConfig` asserts the v2 shape for **both** types (statuses/kinds, named edges, exact
  gate strings incl. the deliver check, `current` set/clear, `require.who`) and closes its own gaps:
  it validates the **committed** config only — copy `.mtt/config.yaml` into a temp dir and `Load`
  there (no gitignored `config.local.yaml` overlay can redden or mask it) — and asserts the **full
  cancel matrix + total transition count** (a flow edit cannot silently drop cancelability).
- `dogfood.txt` e2e: status assertions anchor on the header (`\[speccing\]`), assert **no history
  entry** after a blocked move (the spec's own promise), scratch config gains `require: {who: true}`
  (the real config's per-move policy is exercised), and the scratch flow mirrors the v2 tail
  (`approved`/`deliver` with a fake merge-trace check, e.g. grep over a file standing in for
  `git log`). Keep fake/cheap commands (s006–s008 e2e strategy).
- The dead `task.Default` assert is dropped or made meaningful (assert on the parsed type set).

### D10 — Follow-up product tasks (created in mtt right after PR #23 merges)

1. **`post-persist actions`** (type `task` — open design): transition commands that run **after** the
   task-file write (`after:`), reusing the `Command` VO (timeouts, rollback). Mechanizes the last
   manual steps: `git add .mtt/tasks/{{.ID}}.yaml && git commit …` on `deliver`/`cancel` (and, in
   general, per-move state auto-commit — retiring the S4 discipline class). Design questions: ordering
   guarantees, failure semantics (the transition already persisted), interaction with rollback.
2. **`team semantics of the YAML store`** (backlog, low): the solo-by-construction property (queue
   state on main lags branches; no claim visibility) — state-branch / auto-push / claim mechanics;
   adjacent to t10 (multi-agent cluster) but distinct: t10 is store-level concurrency, this is
   git-topology-level visibility.

**Interim manual steps (exactly two, both temporary until #1):** commit the state write on main after
`deliver`; commit the state write on main after `cancel`. Everything else that AGENTS.md's
"Working under mtt" section currently prescribes as discipline either becomes mechanized by this
design (branch-from-main, artifact presence, delivery verification) or dies with the uncommitted
convention. The section shrinks accordingly (finding #8's contradictions get fixed in the same pass:
the stale "backlog stays in TASKS.md" preamble, the yaml CLAUDE.md hand-edit/never-marshaled
invariants rewritten to bless the hand-authored committed config, guard-test mention added).

## Flow v2 — full shape summary

**task (prefix `t`, default, 15 statuses):**
`tbd → speccing → spec_review → spec_human_review → spec_fix → planning → plan_review →
plan_human_review → plan_fix → implementing → impl_review → impl_fix → approved → done | cancelled`

Edges: `start` (branch cmd, current:set) · `submit` ×6 (spec/plan: artifact glob; impl ×2:
`make check` 10m) · `approve` ×5 (spec_review→spec_human_review, spec_human_review→planning,
plan_review→plan_human_review, plan_human_review→implementing, impl_review→approved) · `decline` ×6
(each `_review`/`_human_review` → its `_fix`, approved→impl_fix) · `deliver` (approved→done,
switch-main + squash-trace check, current:clear) · `cancel` ×8 (tbd, speccing, planning,
implementing, spec_fix, plan_fix, impl_fix, approved — each: switch-main, current:clear).

**chore (prefix `c`, 7 statuses):** as in D6; `submit` gates `make check` (10m map form); shares
`deliver`/cancel mechanics.

`require: {who: true}` unchanged. PR-title convention: `<id>: <title>`.

## Review-findings coverage

| finding (2026-07-10 review) | closed by |
|---|---|
| #1 cancel strands the write | D5 (+ D10 interim step) |
| #2 entry from HEAD / no return to main | D4 + D2 (commit-early makes it safe) + D3 |
| #3 guard validates merged config | D9 |
| #4 proxy blocks undiagnosable / precondition misplaced | D2 (gate speaks; convention dies) |
| #5 `_fix` resubmit gates toothless | D2 (existence check is meaningful) |
| #6 weak e2e asserts | D9 |
| #8 doc self-contradictions | D10 docs pass |
| #9 inverted timeout policy | D8 |
| #10 guard misses cancel matrix | D9 |
| #7 migration completeness (orphaned backlog items) | out of scope here — separate small fix: `mtt add` the missing items (epic-return, advance/compensation, cancelled-blocker, edit-audit, boards/views, D10's two tasks) |

## Acceptance

- Committed `.mtt/config.yaml` v2 (two types, delivery tail, id-keyed artifact gates, per-command
  timeout) loads + validates; `mtt types` renders both flows.
- `TestRepoDogfoodConfig` green and overlay-proof per D9; full cancel-matrix + edge-count asserted.
- `dogfood.txt` v2 green: entry re-enter-or-from-main proven, blocked move leaves status
  (header-anchored) + history count unchanged, deliver verified via the fake merge-trace, `require`
  active.
- The 20 committed tasks unaffected (all `tbd`; no status renames touch them; `impl_human_review`
  never occurs in task files).
- Docs: DESIGN.md/.ru flow-v2 note (incl. "done = in main", the PR-title and artifact-name
  conventions, the two interim manual steps, F5 mid-flight-parking known limit); AGENTS.md
  "Working under mtt" rewritten smaller; finding-#8 contradictions fixed; spec/session files updated.
- `make check` green; PR #23 CI green.

## Out of scope

Post-persist actions and team-semantics (D10 — follow-up mtt tasks); references-based artifact gates
(t1/s010 upgrades D2's glob); per-edge `require`/roles (parked, unchanged); migrating the 20 task
files to any new shape (none needed); renaming existing date-keyed artifacts.
