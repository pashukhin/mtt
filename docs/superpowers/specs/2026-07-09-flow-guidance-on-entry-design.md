# Flow guidance on entry (design spec)

Date: 2026-07-09 · Branch: `feat/s008.95-release-prep` (bundled into the same PR #20) · Version: `0.8.9-dev` (unchanged)

A small **flow-UX** feature landed alongside the release-prep chore (per the user): the flow's authored
`description`s become **inline instructions for the agent**, surfaced at the moment they're relevant. Two
capabilities that were dormant get turned on:

1. **On a successful transition** (`mtt status` / verb sugar), print — after the `id: from → to` ack — the
   move's guidance: the traversed edge's `description`, the destination status's `description`, and the
   **onward moves** (`next: …`). This is the "show the status/transition description on a move" think-item
   (TASKS.md → Later), reframed: it is not a nicety, it is the flow telling the agent *what to do now* and
   *where it can go next*.
2. **`mtt show`** surfaces the **current** status's `description` + onward moves (human output **and**
   `--json`), so an agent can query "where am I / what can I do" at any time — the robust, structured path.

Grounding facts (verified): a move prints only `id: from → to` ([status.go:91]); the transition `description`
is shown only by `mtt types`; the **status `description` field is surfaced nowhere today** (dormant). Both
`mtt status` and the `mtt <status> <id>` sugar funnel through one `runTransition`, so the on-move guidance
lands in a single place.

## Decisions (brainstormed)

- **Surface (on-move): stdout**, right after the `id: from → to` line, **text mode only** — the agent receives
  the instruction inline without needing `2>&1`. The `--json` move output is left as the task object
  (unchanged); structured guidance is a `show` concern (below). *Rejected:* stderr (matches the gate-progress
  convention and keeps stdout minimal, but the user chose inline-on-stdout so an agent driving mtt gets the
  instruction in its primary stream).
- **Scope: on-move (Part A) + `mtt show` (Part B).** Part B is the queryable/structured path (`--json`), so
  guidance isn't only a fleeting side effect of a move.
- **`descriptions` are the instruction vocabulary.** The traversed edge's `description` = "what this move is
  for / what to do now"; the destination status's `description` = standing instructions for that state; the
  onward transitions = the available next moves (each with its own `description` as a hint).
- **Blocked transitions print nothing** — the task didn't enter the status, so there's no guidance.
- **Granularity of flows** (richer statuses like `spec_writing → spec_review`) is **out of scope here** —
  captured as a separate design artifact for the s009 dogfood session (the default template is not changed).

## Architecture (resolved)

`cli → core → port ← adapter` — unchanged. Two **pure** helpers on `Type` (`pkg/mtt`), mirroring the existing
`StatusKind`/`FindTransition`:

- `Type.StatusByName(name StatusName) (Status, bool)` — the status (for its `Description`).
- `Type.TransitionsFrom(status StatusName) []Transition` — onward edges in definition order (empty for a
  terminal/unknown status → no `next:`).

CLI:

- **`internal/cli/guidance.go`** (new) — `formatNextMoves([]mtt.Transition) string` → `to (desc) · to (desc)`
  (the shared onward-moves renderer).
- **`status.go`** (`runTransition`) — after the `id: from → to` line, in **non-JSON** mode: print
  `  ▸ <edge.Description>` (if set), `  ▸ <status.Description>` (if set), and `  next: <formatNextMoves>` (if
  onward non-empty). Data is already in hand (`cfg`, `task`); reuses `TypeByName`/`FindTransition` +
  the two new helpers. The sugar inherits it (same path).
- **`show.go`** — load `cfg` (`yaml.Load`), compute the current status's `Description` + onward; the human
  path passes them into `formatTask` (a guidance block right under the header: `  ▸ <status desc>` +
  `  next:     …`); the `--json` path emits a new **`showJSON`** (anonymous-embeds `taskJSON`, adds
  `status_description` + `next[]`, both `omitempty`) — so `list`/`edit`/`tag`/`status --json` (which use
  `taskJSON`) are untouched.
- **`json.go`** — `showJSON` + `nextMoveJSON{to, description?}` + `toShowJSON(task, statusDesc, onward)`.

## Acceptance (must pass)

- **On-move (text):** `mtt status t1 in_progress` (default template) prints `t1: tbd → in_progress` then
  `  ▸ review the spec, create a branch` and `  next: done (quality gate) · cancelled`. A move to a terminal
  (`mtt done t1`) prints the edge description and **no** `next:` line. The verb sugar behaves identically.
- **On-move (json):** `mtt status … --json` output is unchanged (the task object; no guidance text on stdout).
- **`mtt show` (human):** shows `▸ <status desc>` (when the current status has one) and `next: …`; a terminal
  task shows no `next:`.
- **`mtt show --json`:** includes `status_description` (omitempty) + `next` (array of `{to, description?}`,
  omitempty); `list`/`edit --json` are byte-identical to before.
- **Core:** `Type.StatusByName` / `Type.TransitionsFrom` unit-tested (found/absent; onward order; terminal →
  empty).
- `make check` green.

## Out of scope / follow-ups

- Flow **granularity** redesign (richer statuses) — artifact for s009 dogfood.
- Guidance in the **`--json` move** output of `mtt status` (kept to `show --json` — one structured home).
- A `--quiet` flag to suppress the guidance (no `--quiet` exists yet; deferred with the polish flags).

## Docs sync (same PR)

`CLI_REFERENCE.md`/`.ru` (`status` + `show` output: the guidance lines + `show --json` fields);
`internal/cli/CLAUDE.md` (the new behavior + `guidance.go`); `docs/architecture/model.go` (the two new `Type`
methods, if the snapshot lists Type's API); the s008.95 spec/session/roadmap-row note the bundled feature.
