# Flow granularity — design artifact for s009 dogfood

Status: **input for the s009 dogfood brainstorm** (not a decision). Captured during s008.95 while shipping
**flow guidance on entry** (a status/transition `description` now prints as an inline agent instruction on a
move and in `mtt show`). This note records the granularity question so s009 can decide it deliberately.

## The two insights this comes from

1. **Model *progressing* intermediate work as statuses/transitions, not as node-level actions.** The parked
   "node-level status actions" seam (run a pipeline *without* changing status) targets things that are really
   *state changes* and deserve to be statuses. `spec_writing → spec_review` is a transition, not a verb hanging
   off `in_progress`. Modeling it as a status: (a) each edge can **gate** (spec exists → review approved →
   tests green); (b) **history becomes signal** ("spec written @T1, reviewed @T2"), where a self-loop would be
   false-history noise; (c) mtt already supports arbitrary per-type flows from config, so it costs only config.
   The genuinely-different residual for node-actions is **repeatable, non-progressing** ops (commit WIP N times,
   run build on demand) — modeling those as statuses forces self-loops; and an agent can just run plain
   `git commit`/`make` for them. → node-actions stay **YAGNI**; invest in richer flows instead.

2. **A description is the agent's runbook step, now that guidance is surfaced (s008.95).** With finer statuses
   *and* shown descriptions, the flow config becomes a **self-instructing runbook**: entering `spec_writing`
   prints "write the spec to docs/…; then `mtt spec_review <id>`"; `next:` lists the moves. `status.description`
   = standing instructions for the state; `transition.description` = the intent of the step just taken.

Together: **finer statuses carry the instructions; guidance-on-entry delivers them.** That is the concrete form
of "the flow organizes agentic development."

## The trade-off to calibrate in s009

More statuses = more **signal** (honest history, per-step gates, per-step instructions) **but** more **bloat**:
a transition per micro-step, more edges to author, and every status must satisfy the topology invariants
(≥1 initial/active/terminal; kind fixed by in/out degree). The sweet spot: **a status for each real state the
work rests in and can be gated/instructed at**, not for every action.

**Litmus:** is it a place the task *rests* (awaiting the next deliberate move) → status; or a thing you *do*
repeatedly while resting → not a status (plain shell, or a future node-action).

## A candidate `session` flow for s009 (illustrative — decide in the brainstorm)

The self-host `session` type (see the s009 dogfood spec) could be richer than `tbd → in_progress → done`:

```
tbd → speccing → planning → in_progress → review → done   (+ cancelled from any non-terminal)
```

| status        | kind     | description (instruction)                                   | edge gate (→ this) |
|---------------|----------|-------------------------------------------------------------|--------------------|
| `tbd`         | initial  | pick up: `mtt speccing <id>` to start the design            | —                  |
| `speccing`    | active   | brainstorm → write the spec to `docs/superpowers/specs/…`   | branch created (`feat/{{.ID}}`) |
| `planning`    | active   | write the implementation plan to `docs/superpowers/plans/…` | spec file exists   |
| `in_progress` | active   | implement test-first; commit green as you go                | plan file exists   |
| `review`      | active   | request review / self-review; address findings             | `make check` green |
| `done`        | terminal | merged                                                      | `make check` green + (review acked) |

This mirrors this project's *actual* session lifecycle (brainstorm → plan → TDD → review → merge), so each
transition's `description` is a real instruction and each gate is a real DoD checkpoint. `step` and `phase`
can stay simpler (a `step` is a TDD increment inside `in_progress`; a `phase` is a container).

### Cautions
- **Don't over-fragment.** `speccing`/`planning` as separate states is defensible (each has a distinct
  deliverable + gate); going finer (e.g. `spec_review` vs `spec_writing`) risks a transition per keystroke.
  Start moderate; add a state only when it has its own gate or instruction.
- **Gate cost.** Putting `make check` on several `→` edges means several full runs per session. Consider
  gating the heavy check only on `→ review` / `→ done`, and lighter/no gates on the earlier edges
  (file-exists checks, branch creation).
- **The default template is unchanged** by this — it stays `tbd → in_progress → done (+cancelled)` for
  general users. Richness is a *self-host* choice; a `coding`-style richer template could ship later.

## Recommendation for s009

Decide the `session` granularity in the s009 brainstorm using the litmus above. A reasonable default:
**add `review` between `in_progress` and `done`** (a real "work complete, awaiting the gate/review" state),
keep `speccing`/`planning` optional (fold into `in_progress` if the extra states feel heavy), author a real
`description` on every edge (they are now agent instructions), and gate `make check` on `→ review`/`→ done`
only. Revisit after living with it — the whole point of dogfood is to feel the granularity in practice.
