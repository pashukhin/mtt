# Named transitions + edge-verb sugar (s008.98) — design

Status: **approved design** (brainstormed 2026-07-10), ready for writing-plans. Session **s008.98**, version
`0.8.97-dev → 0.8.98-dev`. Branch `feat/s008.98-named-transitions` (off `main`).

## Motivation

The verb sugar `mtt <status> [<id>]` (s006.5) moves a task by naming the **target status** (`mtt fix`,
`mtt done`) — status names are unique per flow and reachability is checked. But there is no verb that names
the **edge out of the current status**, letting an agent act "in terms of where the task is now"
(`mtt decline` for `review → fix`). Naming the edge is more semantic (an *action*, not a destination), keeps
agent context small, and reads naturally in a review/QA flow.

The fix is a small, additive domain change: an optional `name` on a transition, resolved before the target
status in the sugar, plus a symmetric explicit command `mtt do <edge>`. **The gate/execution core
(`core.Transitioner`) is not touched** — an edge name resolves to its target status and rides the existing
`runTransition(to)` path.

## The resolution triad

| resolve by | explicit command | verb sugar |
|---|---|---|
| **target status** | `mtt status [<id>] <status>` | `mtt <status> [<id>]` |
| **edge name** *(new)* | `mtt do [<id>] <edge>` *(new)* | `mtt <edge> [<id>]` *(new)* |

`mtt status` stays status-only; `mtt do` is edge-name-only; the **sugar** is the one "smart" form (tries edge
name first, then target status — safe because the two namespaces are disjoint, see Validation).

## Domain (`pkg/mtt`)

- **`Transition.Name string`** — an optional label for the edge, placed after `Description` in the struct
  ([config.go, the `Transition` type](../../../pkg/mtt/config.go)). Like `Description` it is an **open label**
  (not a value object, not a reject-empty identity — it is optional, empty = an unnamed edge). No serialization
  tags (adapter maps it).
- **`Type.FindTransitionByName(from StatusName, name string) (Transition, bool)`** — a new pure primitive
  mirroring `Type.FindTransition` / `Type.StatusByName` (s008.95): returns the single outgoing edge from
  `from` whose `Name == name`. Returns `false` when `name == ""` or no such edge exists. Single source of
  truth for edge-name lookup (used by the CLI; the domain stays name-agnostic — it does not know the string
  `"decline"`, only that a caller asked for some name).
- **Validation** (`validateFlow`, [validate.go:45](../../../pkg/mtt/validate.go#L45)) — three new checks,
  each name-agnostic and structural:
  1. **Edge-name uniqueness per source status.** For each `from`, the non-empty `Name`s of its outgoing edges
     must be distinct (`type %q status %q: duplicate transition name %q`). (Names may repeat across *different*
     source statuses — `decline` from both `review` and `qa` is fine and intended.)
  2. **Namespace disjointness.** A non-empty edge `Name` must not equal any **status name** in the same type's
     flow (`type %q transition %q->%q: name %q collides with a status name`). This makes the sugar's
     "edge-first" precedence safe — a name is never both an edge and a status, so there is no shadowing.
  3. **`(from,to)` uniqueness per type.** No two transitions may share the same `(from,to)` pair
     (`type %q: duplicate transition %q->%q`). This surfaces an existing latent assumption — `FindTransition`
     already returns the *first* match, so a second parallel edge is dead code today — and it is what lets an
     edge name resolve to its `to` and reuse the existing gate path without touching `core.Transitioner`.
     **Non-breaking (verified 2026-07-10):** every shipped `default`/`coding` template type already has unique
     `(from,to)` pairs, so the new invariant rejects no existing config.

## Adapter (`yaml`)

- **`ymlTransition.Name string \`yaml:"name,omitempty"\`** ([dto.go:55](../../../internal/adapter/yaml/dto.go#L55))
  + map it in `ymlConfig.toDomain` ([dto.go:130](../../../internal/adapter/yaml/dto.go#L130)), exactly like
  `Description` / `Current`. No other adapter change; config is read-only (no marshal).

## CLI (`internal/cli`)

All three edge-aware entry points funnel through the **existing** `runTransition(cmd, root, cfg, settings, id,
to, noRun)` ([status.go](../../../internal/cli/status.go)) — resolution turns an edge name into a target
status `to`, then the gate path is unchanged.

- **Sugar** (`classifyStatusMove`, [root.go:142](../../../internal/cli/root.go#L142)) — the shared tail of the
  1-arg and 2-arg sugar. New first step: try `typ.FindTransitionByName(task.Status, arg0)`; if found, route to
  `runTransition(..., edge.To, false)`. Otherwise fall through to today's target-status classification
  (`NewStatusName` → `StatusKind` → route). Precedence is edge-first, but disjointness (Validation #2) makes it
  unobservable — an arg is at most one of the two.
- **"No current / plausible verb" branches.** `trySugarCurrent` (no current set) and `trySugar` (2-arg, missing
  task) currently claim the command with an actionable error when `statusInAnyFlow(cfg, arg0)`
  ([root.go](../../../internal/cli/root.go)). Extend the predicate so a **known edge name anywhere in the
  config** also claims it (`statusInAnyFlow(...) || edgeNameInAnyFlow(...)`) — so `mtt decline` with no current
  set says "no current task; run `mtt use <id>`", not "unknown command".
- **New command `mtt do [<id>] <edge>`** (`do.go`, `newDoCmd`) — mirrors `newStatusCmd`
  ([status.go:21](../../../internal/cli/status.go#L21)): `Args` 1-or-2, id resolved via `resolveTaskID`
  (explicit id > current), a local `--no-run` (the sugar cannot bypass the gate; the explicit form can, like
  `status`). It `Get`s the task to read its current status, resolves `typ.FindTransitionByName(task.Status,
  edge)`, and on a hit calls `runTransition(..., edge.To, noRun)`. On a miss it returns
  `fmt.Errorf("%w: no action %q from status %q (available: %s)", core.ErrInvalidTransition, edge, task.Status,
  <named edges from task.Status>)` → **exit 6**, symmetric with an invalid `mtt status` move and doubling as
  discoverability-in-an-error. `do` is **edge-name-only** (no status fallback) — one resolution mode per
  explicit command. A registered `do` command wins the sugar on a name clash (documented; a status literally
  named `do` would be shadowed — acceptable).

## Discoverability (the ergonomic payoff — in scope)

- **`mtt types`** ([types.go](../../../internal/cli/types.go)) renders a named edge with its verb, e.g.
  `[decline] review → fix` (exact format finalized against the current renderer during implementation).
- **Flow guidance `next:`** (s008.95, `formatNextMoves` in [guidance.go](../../../internal/cli/guidance.go))
  shows the verb when an onward edge is named: `next: decline → fix (send back) · approve → done`, so an agent
  reading `mtt show` / a successful-move footer sees exactly what it can type.
- **JSON:** `nextMoveJSON` ([json.go](../../../internal/cli/json.go)) gains `name string \`json:"name,omitempty"\``
  so `show --json`'s `next[]` carries the verb for machine consumers (the agent's primary channel).

## Out of scope (YAGNI)

- **Multigraph edges** (two `(from,to)` edges with different gates) — forbidden by Validation #3.
- **Alias arrays** (multiple verbs per edge) — a single `name`.
- **Edge names in `mtt status`** — `status` stays status-only; `do` is its edge-name symmetric.
- **Recording the verb in `history`** — the canonical record is `from→to`; the verb is only an input alias.

## Testing

- **Unit (`pkg/mtt`):** `FindTransitionByName` (hit / miss / empty-name / per-source scoping); `validateFlow`
  rejects duplicate edge-name-per-source, edge-name==status-name, and duplicate `(from,to)`; a valid named
  flow passes.
- **Unit (`yaml`):** `toDomain` maps `ymlTransition.Name` (a small config fixture, like the `Status.Default`
  test).
- **Unit (`cli`):** `nextMoveJSON` carries `name` (omitempty); `mtt do` unknown-edge error wraps
  `ErrInvalidTransition` (exit 6) and lists available actions.
- **e2e (`testscript`):** a `coding`-style scratch config with a named edge (`review → fix` named `decline`,
  gated) — `mtt do decline` and the sugar `mtt decline` both move + run the gate; a blocked gate still exits 3;
  `mtt do <bad>` exits non-zero and lists the actions; `mtt types` shows the verb. A validation e2e: an
  edge-name==status-name (or duplicate `(from,to)`) config is rejected by `mtt add`/`mtt types`.

## Acceptance

- A config edge `{from: review, to: fix, name: decline, commands: [...]}` makes **`mtt decline`** (current
  task) and **`mtt do decline t1`** move the task `review → fix` and run the gate; a red gate blocks (exit 3).
- `mtt do bogus t1` and `mtt <bogus>` fail helpfully (the explicit form lists the available actions, exit 6).
- `mtt config` with an edge name equal to a status name, or two `review → fix` edges, is rejected by
  `Config.Validate` (surfaced on `mtt add` / `mtt types`).
- `mtt show --json` `next[]` entries carry `name`; `mtt types` and the `next:` guidance show the verb.
- `make check` green; `core.Transitioner` and the gate path are byte-unchanged.

## Docs sync (at implementation time)

`CLI_REFERENCE.md`/`.ru` (the `do` command + edge-name sugar + the `name` on transitions + `next[].name` in
`show --json`), `DESIGN.md`/`.ru` (the resolution triad; the three new flow invariants), the affected
`CLAUDE.md` files (`pkg/mtt`, `internal/cli`, `internal/adapter/yaml`), `TASKS.md` (a new item under e5),
`sessions/README.md` (008.98 row) + `sessions/008.98_named_transitions.md`, `NEXT_SESSION.md`
(+ carry-over lessons), `CHANGELOG.md` (Unreleased).
