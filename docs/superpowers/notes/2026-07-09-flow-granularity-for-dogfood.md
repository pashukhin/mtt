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
- **`session`** (the gated unit): the §3 flow. A reasonable **moderate** cut is `tbd → in_progress → review →
  done` (fold `speccing`/`planning` into `in_progress` if the extra states feel heavy), each edge with a real
  `description`, a branch action on `→ in_progress` (`git checkout -b feat/{{.ID}}`), and `make check` gating
  the *heavy* edges only (`→ review` and/or `→ done`).
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

## 8. Recommendation for s009

Decide `session` granularity in the s009 brainstorm with §6's litmus. A sound default:

1. **`session`: `tbd → in_progress → review → done` (+`cancelled`)** — add `review` (a real "work done, awaiting
   the gate" state); keep `speccing`/`planning` optional (introduce only if living without them feels lossy).
2. **Author a real `description` on every edge** (they are now agent instructions — §2/§3).
3. **Gate `make check`** on `→ review` (and/or `→ done`); **branch** on `→ in_progress` (`feat/{{.ID}}`, with a
   `git branch -D` rollback for idempotency); **`step` gates `make check` on `→ done`**.
4. **Showcase a self-referential gate** on `phase → done` (§4) — it makes the dogfood config a demo of mtt
   gating on its own graph, and is the kind of example neighboring repos will copy.
5. Revisit after living with it — the point of dogfood is to *feel* the granularity, not to get it perfect up
   front. The **default template stays** `tbd → in_progress → done (+cancelled)`; richness is a self-host
   choice (a richer reusable template could ship later).
