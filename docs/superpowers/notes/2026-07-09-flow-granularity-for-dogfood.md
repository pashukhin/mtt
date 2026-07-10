# Flow design findings — input for s009 dogfood

Status: **design input for the s009 dogfood brainstorm** (findings + options, NOT decisions). Started while
shipping s008.95 **flow guidance on entry** (a status/transition `description` now prints as an inline agent
instruction on a move and in `mtt show`) and expanded with what that feature changes. s009 decides the final
shape; this note is what we know going in.

---

## 1. Core insights

1. **Model *progressing* intermediate work as statuses/transitions — not node-level actions.** The parked
   "node-level status actions" seam (run a pipeline *without* changing status) mostly targets things that are
   really *state changes* and deserve to be statuses. `spec_writing → spec_review` is a transition, not a verb
   hanging off `in_progress`. Modeling it as a status buys: (a) each edge can **gate** (spec exists → review
   approved → tests green); (b) **history becomes signal** ("spec written @T1, reviewed @T2") where a self-loop
   would be false-history noise; (c) mtt already supports arbitrary per-type flows from config, so it costs only
   config. The genuinely-different residual for node-actions is **repeatable, non-progressing** ops (commit WIP
   N times, run build on demand) — an agent just runs plain `git commit` / `make` for those. → node-actions
   stay **YAGNI**; invest in richer flows instead.

2. **A `description` is the agent's runbook step, now that guidance is surfaced (s008.95).** Finer statuses
   *and* shown descriptions make the flow config a **self-instructing runbook**: entering `speccing` prints
   "write the spec to docs/…; then `mtt planning <id>`"; `next:` lists the moves. The flow tells the agent what
   to do at each state — no external CONTRIBUTING doc needed. **This is the payoff for spreading to neighboring
   repos:** a new agent reads `mtt types` + gets guidance on entry and knows the process.

3. **Guidance changed the calculus toward richer flows.** Before s008.95 a fork or a long chain forced the
   agent to *know* the flow; now `next:` surfaces the options at every node, so a **branching or multi-step
   flow is cheap to consume**. Richer flows got more attractive the moment guidance shipped.

---

## 2. The grammar: what to hang on the flow, and where

Deciding a flow is deciding *where* each authored thing lives. The mapping that fell out:

| Thing | Lives on | Means | Answers |
|---|---|---|---|
| `status.description` | a **status** (node) | standing instructions while here + how to leave | "I'm in this state — what now?" |
| `transition.description` | an **edge** | the intent of *this* step | "why am I moving?" |
| `transition.commands` (gate) | an **edge** | the **exit criteria of the source state** (the DoD checkpoint to leave it) | "have I earned this move?" |
| `transition.commands` (action) | an **edge** | a side effect performed on the move (create a branch) | "set up the target state" |
| `transition.current` (set/clear) | an **edge** | ownership / working-context handoff | "is this now my active task?" |
| `transition.commands[].rollback` | a **command** | compensate a partial pipeline if a later command fails | "undo the setup if the gate fails" |

Rule of thumb: **a gate on `A → B` checks the work done *in A*** (its exit criteria); **B's `description` tells
you what to do *in B*** (entry instructions); **the edge's `description` is why you moved.** Actions (side
effects) run *after* checks on the same edge (s008 caveat), paired with a `rollback` if they can strand state.

---

## 3. Encode the *actual* process (map to the superpowers skills)

The dogfood flow should mirror the lifecycle we actually follow in this repo, so mtt gates what we really do.
Our observed session lifecycle ↔ a candidate `session` flow:

| our step (superpowers) | `session` status | deliverable / exit criterion |
|---|---|---|
| brainstorming | `speccing` | a spec in `docs/superpowers/specs/…` |
| writing-plans | `planning` | a plan in `docs/superpowers/plans/…` |
| TDD implementation | `in_progress` | code, green between commits |
| requesting/receiving review, finishing-branch | `review` | `make check` green; findings addressed |
| squash-merge | `done` | merged |

So the dogfood `session` flow ≈ **the superpowers process encoded as a gated state machine.** That is the most
honest possible self-host: mtt enforces the very method it was built with.

---

## 4. Task-aware gates that query mtt itself (a new capability we can showcase)

Gates aren't limited to git/make — a gate can shell out to **`mtt` and gate on the task graph**. The headline
example, on the `phase` type:

```yaml
# a phase can't close while it still has open (non-terminal) sessions
- {from: in_progress, to: done,
   description: "close the phase — all its sessions are done",
   commands: ["! mtt list --parent {{.ID}} --kind initial --kind active --ids | grep -q ."]}
```

`{{.ID}}` expands to the phase id; `mtt list --parent … --kind initial --kind active --ids` prints the open
direct children; `grep -q .` is 0 iff any exist; `!` flips it, so the gate **blocks** the phase's close while a
child session is open. This makes hierarchy a *real* constraint (roadmap's parent axis is only ordering today),
and it demonstrates mtt gating on **its own state**, not just the filesystem.

**Caveats:** it needs `mtt` on `PATH` (installed — `make install`); it checks **direct** children only (a
recursive check needs a helper — YAGNI now); the `!`/pipe pattern relies on the shell seam (already used by the
`coding` template's `["! make test"]`). It reads the store mid-transition (before the phase's own write) —
consistent because a transition is sequential and single-process.

---

## 5. Candidate flows for all three self-host types

- **`phase`** (container): `tbd → in_progress → done` (+`cancelled`). No git/make gate; optionally the
  self-referential "no open sessions" gate on `→ done` (§4). Descriptions are light ("this phase groups …").
- **`session`** (the gated unit): the **full** §3 flow — **`tbd → speccing → planning → in_progress → review →
  done`** (+`cancelled`) — **decided (A)**, `speccing`/`planning` are *separate* statuses (not folded into
  `in_progress`). Branch + `current: set` on `→ speccing` (the spec is committed on the branch); a real
  `description` on every edge (they are agent instructions); `make check` gates the **heavy** edges only
  (`→ review`, `→ done`) — the early edges carry instructions but no heavy gate (an artifact-exists gate is
  awkward, §6).
- **`step`** (a TDD increment inside a session): `tbd → in_progress → done` (+`cancelled`), gating `make check`
  on `→ done` (per the earlier decision — every step is green). No branch (a step works in the session branch).

---

## 6. Cautions & honest limits (learned/anticipated)

- **Don't over-fragment.** Add a status only when it is a place the work *rests* **and** has its own gate or
  instruction. Litmus: *rests here awaiting the next deliberate move* → status; *done repeatedly while resting*
  → not a status (plain shell / future node-action).
- **Gate cost compounds.** `make check` on several `→` edges = several full runs per session. Gate the heavy
  check only where it's meaningful (`→ review` / `→ done`); use cheap or no gates on earlier edges.
- **Early "artifact exists" gates are awkward.** Our doc filenames are `date-slug`, not `{{.ID}}`-keyed, and
  placeholders are a **structural whitelist** (`.ID/.Type/.From/.To` only — no title/slug, by design). So a
  task-specific "the spec file exists" gate on `→ planning` is hard to express; lean on the
  description-as-instruction there and gate the *objective* checks (`make check`, the §4 mtt query) where they
  key cleanly off `{{.ID}}`.
- **Gate commands should be idempotent / re-runnable.** A blocked move is retried after a fix, so a re-run must
  be safe. `git checkout -b feat/{{.ID}}` fails if the branch exists — pair it with a `rollback`
  (`git branch -D`), or the retry hits "branch exists". Prefer re-runnable actions or explicit compensation.
- **Side-effect ordering.** On an edge that both checks and acts, checks run first (s008); an action that
  strands state on a *later* failure needs a `rollback`. Cross-edge (multi-status) compensation is still parked.

---

## 7. Two "next"s — don't conflate them

- **`next:` (flow, intra-task)** — the onward *moves* from the current status (s008.95 guidance). "Where can
  *this task* go?"
- **`mtt roadmap` / `ready` (queue, inter-task)** — which *task* to pick up next across the backlog. "What
  should I work on?"

They are orthogonal and complementary; the artifact/spec should keep the vocabularies separate. Likewise,
**parking/blocking is not a status**: a blocked task stays `tbd` with a `depends_on` (and `ready` hides it);
priority sequences it. `cancelled` means *abandoned*, not *paused*.

---

## 8. Decisions carried into s009

Two calls are **made**; the rest is settled in the s009 brainstorm.

- **(A) `session` uses the full flow — `tbd → speccing → planning → in_progress → review → done` (+`cancelled`).**
  `speccing`/`planning` are **separate statuses**, not folded into `in_progress`. **Rationale:** history then
  shows *what* was done at each stage **and how many times a session looped back** — a `planning → speccing →
  planning` bounce records spec rework; a coarse `in_progress` hides it. Granularity buys **iteration
  visibility** (sharpens insight #1: history is signal — not just *what/when* but *how many times*).
- **(B) Self-referential gates (§4): used sparingly** — only where there is genuinely no simpler way, **until a
  bank of proven techniques is built up**. The `phase → done` "no open sessions" gate is a candidate but is
  added only if clearly worth the config complexity; prefer plain git/make gates elsewhere.

Still to settle in the brainstorm: exact gate placement + rollbacks (branch + `current: set` on `→ speccing`
with a `git branch -D` rollback for idempotency; `make check` on `→ review`/`→ done`; `step` gates `make check`
on `→ done`), the set/clear edges, and phase/step descriptions. The **default template stays**
`tbd → in_progress → done (+cancelled)` — richness is a self-host choice (a richer reusable template could ship
later). Revisit after living with it: the point of dogfood is to *feel* the granularity, not to nail it up front.

## 9. Decided in the s009 flow discussion (2026-07-10, post-s008.98)

s008.98 (named transitions + edge-verb sugar) shipped **after** §1–§8, which changes the calculus again: a
review **fork** is now a first-class, cheap move (`approve`/`decline` as **named** edges — no `advance` needed),
so the flow can branch honestly. These calls were **made** in discussion (refine in the brainstorm):

- **(C) Two-tier hierarchy — `phase → session` (drop `step`/subtask).** A rich `session` flow (per-artifact
  review, below) makes a separate *increment* tier redundant — the implementation phase **is** the work; add a
  child only when an implementation genuinely splits. Responsibility split: **`phase`** (container) groups +
  sequences — coarse `tbd → in_progress → done`, optional self-ref "all child sessions terminal" gate (§4/B);
  **`session`** (the unit) carries the **whole gated method** + the DoD. Naming: the self-host config may use
  `phase`/`session` (truer to how we work) — the shipped default template stays `epic/task/subtask`.
- **(D) `session` = a two-stage per-artifact review cycle** (the richer form of decision A). Each artifact
  stage — **design**, **plan**, **implementation** — is `do → <stage>_review (adversarial agent) →
  <stage>_human_review (human) → next`; a **`declined`** verdict bounces to `<stage>_fix → <stage>_review`.
  This mirrors how we actually work (adversarial spec review, then plan review, both with human sign-off) and
  maxes out "history is signal" — each `_fix` bounce **counts the rework**. The review forks use **named edges**
  (`approve`/`decline`), which by the s008.98 disjointness rule must **not equal a status name** (so
  `approve`/`decline`/`rework`/`submit`, never `review`/`done`).
- **(E) Human review is advisory + `require:{who,why}` — for now.** mtt has **no role enforcement**
  (roles-on-edges parked), so nothing *stops* the agent self-approving a `_human_review` edge. `require:{who,why}`
  on those edges forces attribution, so a self-approval is **visible** in history (`by:` the agent, not a human)
  — honest, not enforced. **New think-item / unpark trigger:** *human review in the self-host flow is the first
  concrete case that justifies roles-on-edges* — revisit roles when advisory proves insufficient.
- **(F) Artifact-presence gates: a cheap proxy where checkable, else instruction.** Placeholders can't key a
  gate to a date-slug doc (§6), so for now "an artifact was produced" = a `{{.ID}}`-free proxy like
  `git status --porcelain | grep -q .` (uncommitted changes exist) on `<stage> → <stage>_review`. Where presence
  **can't** be cheaply/objectively checked ("is the spec *good*?"), it's **instruction-only** + the
  approve/decline judgment edge. A stronger `{{.ID}}`-keyed doc-path convention is possible later (ties to the
  "placeholders in descriptions" backlog item — TASKS.md Later).
- **(G) CI in-flow, CD out-of-flow.** **CI = `make check`** on the **implementation-review** edges — repo-global,
  needs no slug, keys cleanly. **CD is not per-task** — a release is a milestone event (tag → `release.yml`,
  user-triggered `v0.9.0`, s009.5), so it lives **outside** the session flow (a `phase`/milestone concern or a
  separate `release` type later), never on a session edge.

Still open for the brainstorm: exact edge names + gate placement across the three review stages; whether
`phase` gets the self-ref "children done" gate on v1 (B: sparingly); the set/clear edges; and whether v1 folds
the two review substages into one to cut status count, splitting later (start-simpler-and-enrich vs author the
full shape up front).

## 10. Resolved in the s009 brainstorm (2026-07-10)

The §9 open items were settled (see the reconciled spec
[specs/2026-07-09-session-009-dogfood-design.md](../specs/2026-07-09-session-009-dogfood-design.md) Q3):

- **Full shape, not folded.** v1 authors the **full 15-status** `session` flow — each artifact stage keeps a
  separate `_review` (adversarial subagent) and `_human_review` (human) status. Rationale: max history-as-signal
  (agent-review and human-review are distinct events, each `_fix` bounce counts). (Chosen over the leaner
  12/9-status folds.)
- **Edge names:** `start` (entry, → speccing) / `submit` (do → review, and fix → review) / `approve` /
  `decline` / `cancel`. All disjoint from status names, unique per source, `(from,to)` unique — the s008.98
  invariants hold, so edge-verb sugar (`mtt approve s1`) is unambiguous.
- **Gate placement:** `make check` on **every edge into `impl_review`** (not repeated on `→ done`); an
  artifact-presence **proxy** `git status --porcelain | grep -qv '\.mtt/'` on the spec/plan submit edges
  (instruction-only was the alternative); `phase → done` **takes** the self-ref gate (decision B's "clearly
  worth it" case — the §4 headline).
- **set/clear:** `current: set` + the idempotent branch on the entry edge `tbd → speccing`; `current: clear`
  on `→ done` and `→ cancelled`; `phase` carries neither (a phase isn't taken into work).
- **Hardened by an adversarial spec review (2026-07-10)** — three corrections folded into the reconciled spec:
  **(1)** the §4 phase gate's `! … | grep -q .` form is **fail-open** (no `pipefail`; a missing/erroring `mtt`
  → empty stdout → passes) — replaced with the **fail-closed** `out=$(mtt list … --ids) && test -z "$out"`;
  **(2)** gate commands must be **single-quoted / block** YAML scalars — double-quoting breaks `\.mtt/`
  (`Load` fails), and a plain `! …` scalar has its `!` dropped as a YAML tag (silent inversion), so
  `TestRepoDogfoodConfig` asserts **exact** command strings; **(3)** the full-shape flow otherwise **traps** a
  submitted session (no path to `cancelled` from a review cycle) — `cancel` also fires from the three `_fix`
  statuses (review/human_review reach it via `decline → _fix → cancel`).
- **Two findings that reshaped E and F:**
  - **(E → global `require:{who}`)** mtt's `require` is **project-global only** (`Settings.Require`), not
    per-edge — "require on the human-review edges" needs a core change (out of scope). v1 uses global
    `require:{who}` (auto-satisfied by `config.local author`, so `by:` is always recorded and self-approval is
    visible); per-edge/role `require` is the parked roles work E names as its trigger.
  - **(F → `.mtt`-excluding proxy)** a bare `git status --porcelain | grep -q .` is **defeated** because the
    accumulating `.mtt/tasks/*.yaml` churn (committed only with the PR — S4) always dirties the tree; excluding
    `.mtt/` makes the proxy meaningful, with the documented semantics that the artifact stays **uncommitted
    until `_human_review` approves**.

## 11. Re-modelled to a single `task` type (2026-07-10, supersedes the two-tier §9–§10 structure)

A deeper brainstorm found the two-tier `phase`/`session` shape modelled the **wrong axis**. `session`/`phase`
describe **how we work** (process — ephemeral, executed, not queued); `task`/`epic` describe **how the product
changes** (the backlog). mtt should track the **product** axis. Decisions:

- **One type `task`** (the unit of product change); **no hierarchy** — structure is **deps + tags + priority**.
  `epic` (a group of related tasks) is product-valid but **deferred** ("enough with deps + tags for now");
  when it returns, the §4 "close the epic when its children are done" self-ref gate rides the `epic` type.
- **The full 15-status flow stays — re-read as a task's *maturation*** (design → review → plan → review → build
  → review → done), not our session mechanics. A task may take one or several work-sessions to walk it; the flow
  survives that (no 1:1 task↔session). This is why "keep the 15 statuses" and "single type" are consistent, not
  contradictory.
- **Consequences vs §10:** drop the `phase` type, its self-ref gate, `parents`/hierarchy, re-parenting, and the
  "orphan backlog session" idea. **Keep** the whole gated flow (`make check` on impl-review; proxy on spec/plan;
  branch **per task** `task/{{.ID}}` on entry; `require:{who}`; cancel-from-`_fix`; YAML single-quoting).
  Backlog = `tbd` tasks tagged `backlog` (promotion = drop the tag + start; no re-parenting needed). The §4
  self-ref-gate showcase leaves with `phase` (returns with `epic`); the killer feature is still dogfooded via
  `make check` + proxy + branch. Migration becomes **flat** tasks + tags/priority/deps.

The reconciled spec ([specs/2026-07-09-session-009-dogfood-design.md](../specs/2026-07-09-session-009-dogfood-design.md))
is authoritative; this note's §1–§10 are the reasoning trail that led here.
