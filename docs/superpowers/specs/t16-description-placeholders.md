# Placeholders in shown descriptions/guidance (`t16`)

Status: spec (decision record). Type: task (`t16`). Branch: `task/t16`. Tags: `dx`, `flow`, `release`.

## Context / problem

Flow **`commands`** expand `{{.ID}}`/`{{.Type}}`/`{{.From}}`/`{{.To}}` (`core.expandCommands`), so a gate can
`git checkout -b task/{{.ID}}`. But the **descriptions** shown as inline guidance — the traversed-edge and
destination-status text after a move (`moveGuidance`), the current-status text and onward moves in `mtt show`
(`statusGuidance`/`formatNextMoves`) — are printed **verbatim**. So an author who wants guidance to name the
concrete task can only write a human placeholder like `task/<id>` or
`docs/superpowers/specs/<this-task-id>-<slug>.md` (this repo's own config does exactly that), which the agent
reads as literal `<id>`, not `t17`. `t16` makes shown descriptions expand the **same** four placeholders, so
guidance is actionable (`task/t17`, not `task/<id>`).

Constraints:

- **Reuse the command whitelist.** Only `{ID, Type, From, To}` — never free text (`Title`/`Description`) — so a
  stray `{{.Title}}` stays a template error (not a silent leak), mirroring `core.cmdContext`.
- **Guidance must NEVER break.** Unlike a gate command (a bad template aborts the transition — exit 1),
  guidance is informational: a malformed or unknown-field template must degrade to the **raw** text, never
  error out a `show`/move. This is the key behavioral difference from `expandCommands`.
- **Hexagon.** The expansion primitive lives in `core` (it owns placeholder policy); the CLI (`guidance.go`)
  calls it. Descriptions are terminal output (not a shell), so injection isn't a risk — the whitelist is for
  consistency + the `{{.Title}}`-is-an-error property.
- **TDD, KISS.**

## Decisions

### D1 — `core.ExpandText`: a best-effort, exported placeholder expander

Export a thin function reusing the existing `expandTemplate`:

```go
// ExpandText expands {{.ID}}/{{.Type}}/{{.From}}/{{.To}} in raw, for SHOWN guidance
// (descriptions), returning the raw text UNCHANGED on any parse/execute error —
// guidance is informational and must never break a command. (Gate commands use the
// strict expandCommands, which aborts on error; this is the lenient sibling.)
func ExpandText(raw, id, typ, from, to string) string {
    out, err := expandTemplate(raw, cmdContext{ID: id, Type: typ, From: from, To: to})
    if err != nil {
        return raw
    }
    return out
}
```

- **Best-effort (raw on error)** is the whole point — an author's typo in a description can't wedge `mtt show`.
- Reuses `cmdContext` (the four-field whitelist) + `expandTemplate` (DRY); no second template engine.
- **Rejected — a strict variant** (error to the CLI): guidance rendering would have to handle/swallow it
  everywhere; leniency-at-source is simpler and correct for informational text.

### D2 — Expand every shown description in `guidance.go`

Thread the task's `id`/`type` through the guidance helpers and expand each description with the **right**
From/To:

- **Edge description** (a traversed or onward transition): `From = edge.From`, `To = edge.To` — the edge has a
  real direction.
- **Status (node) description** (a destination after a move, or the current status in `show`): a status is a
  **node**, not an edge, so per the brainstorm decision `From = To = the status's own name` ("you are here at
  `<status>`"). Uniform (all four fields always populated), never empty.

Concretely:
- `moveGuidance(cfg, id, typeName, from, to)` (gains `id`): expand `edge.Description` with `(id, type, from,
  to)`; expand the destination `status.Description(to)` with `(id, type, to, to)` (node rule).
- `formatNextMoves(onward, id, typeName)` (gains `id, typeName`): expand each onward `edge.Description` with
  `(id, type, edge.From, edge.To)`.
- `statusGuidance(cfg, task)`: expand the current `status.Description` with `(task.ID, task.Type, task.Status,
  task.Status)` (node rule); the onward moves are expanded by `formatNextMoves` (which `show.go` calls with
  `task.ID`/`task.Type`).

Callers: `status.go:126` (`moveGuidance` — `task.ID` available), `show.go:44` (`statusGuidance`) + `:87`
(`formatNextMoves` — `task` available), and the `formatNextMoves` call inside `moveGuidance` (threads its `id`).
The signature change also breaks the existing `guidance_test.go` `formatNextMoves(...)` callers (old arity) —
they are updated in the same change (a compile break, so self-catching; the plan lists it).

**`mtt show --json` must be expanded too (the primary agent surface).** `show.go:44` computes
`statusGuidance` **once** and feeds both the human render and `toShowJSON`, so `status_description` in `--json`
is already expanded via `statusGuidance`. But the onward `next[].description` in JSON comes from
`json.go`'s `nextMoveJSON{Description: e.Description}` — the **raw** transition text, bypassing
`formatNextMoves`. Left alone, `--json` would ship `status_description` expanded next to `next[].description`
raw — inconsistent, and an agent reading `--json` (the main consumer) would still see `{{.ID}}`. So
**`toShowJSON` expands `next[].description`** via `core.ExpandText(e.Description, task.ID, task.Type, e.From,
e.To)` — `--json` is then fully expanded and consistent with the human output. (The move path's `--json` emits
only `toTaskJSON` — no description — so it needs no change; `mtt types`/`writeTypeBlock` renders the flow
**schema** with no task in scope, so it stays raw — there is no `.ID` to expand, consistent by nature.)

### D3 — Dogfood: use `{{.ID}}` in a few repo descriptions (demo)

Replace the human placeholders (`<id>` / `<this-task-id>`) with real `{{.ID}}` in a **few representative**
`.mtt/config.yaml` descriptions where a concrete id helps — e.g. the `speccing`/`planning` artifact paths
(`docs/superpowers/specs/{{.ID}}-<slug>.md`) and the branch hints (`task/{{.ID}}`). Small, targeted diff;
**`TestRepoDogfoodConfig` (`internal/adapter/yaml/dogfood_test.go`) must stay green** (it loads a temp copy).
This proves the feature end-to-end on our own flow and shows the value live. (Not every `<id>` — only the
ones where expansion is clearly useful; `<slug>` stays literal, there is no slug placeholder.)

## Scope

**In:** `core.ExpandText` (+ unit tests); expansion wired into `moveGuidance`/`formatNextMoves`/`statusGuidance`
(signatures gain `id`/`type`) **and `toShowJSON`** (so `show --json`'s `status_description` + `next[].description`
are expanded — the primary agent surface); the `guidance_test.go` caller updates for the new arity; a few repo
`.mtt/config.yaml` descriptions switched to `{{.ID}}`; e2e; docs sync.

**Out:**
- **Expanding `title`/free text as a placeholder source** — the whitelist stays `{ID,Type,From,To}` (safety).
- **A new placeholder vocabulary** (e.g. `{{.Status}}`, `{{.Parent}}`) — YAGNI; the four-field set is reused.
- **Strict/erroring expansion for guidance** — rejected (D1).
- **Converting every repo description** — only a representative few (D3).

## Acceptance criteria

1. **`ExpandText` (unit).** `ExpandText("task/{{.ID}}", "t17", "task", "tbd", "in_progress") == "task/t17"`;
   `{{.From}}`/`{{.To}}`/`{{.Type}}` expand; a **malformed** template (`"{{.ID"`) and an **unknown field**
   (`"{{.Title}}"`) both return the **raw** string unchanged (best-effort); empty raw → empty.
2. **Move guidance expands (e2e).** A config whose `tbd→active` edge description is `create branch
   task/{{.ID}}` and whose destination status description references `{{.ID}}` — after `mtt <status> <id>`, the
   printed guidance shows the **concrete** id (`task/t1`), not `{{.ID}}`/`<id>`.
3. **Show guidance expands, human + `--json` (e2e).** `mtt show t1` (human) renders the current-status
   description and the `next:` onward descriptions with placeholders expanded (node rule: `{{.From}}`/`{{.To}}`
   in a status description both render the current status; an onward edge description renders its own
   `from`/`to`). **`mtt show t1 --json`** emits **both** `status_description` **and** `next[].description` with
   placeholders expanded (the F1 consistency guard — no raw `{{.ID}}` survives in either JSON field).
4. **Best-effort in situ (e2e).** A description containing `{{.Title}}` (unknown field) renders **verbatim**
   (raw `{{.Title}}`), and the command still exits 0 — guidance never breaks the move/show.
5. **Dogfood config still valid.** `TestRepoDogfoodConfig` stays green after the D3 edits; during the
   `impl_review` verification, `mtt show <a tbd task>` on this repo renders a D3-changed, placeholder-bearing
   description with the concrete id (e.g. the `start` edge's `task/{{.ID}}` → `task/t16`, or a `speccing` task's
   `docs/superpowers/specs/{{.ID}}-…`) — a real expansion, not a raw==expanded no-op.
6. `make check` green. Docs synced (below).

## Testing approach

- **Unit (`internal/core`):** `ExpandText` table — expansion of each field, best-effort on malformed/unknown,
  empty. (Reuses the `expandTemplate` path already tested for commands.)
- **e2e (testscript, hermetic):** a `flow.yaml` with `{{.ID}}`/`{{.From}}`/`{{.To}}` in an edge description, a
  destination-status description, and one `{{.Title}}` (best-effort) — assert the move guidance and `mtt show`
  output. No network.
- **Dogfood:** `TestRepoDogfoodConfig` green (the D3 config edits load clean).

## Docs to sync (docs-sync judgment, `impl_review`)

Grep **all** parallel occurrences (EN + RU) before editing.

- **`DESIGN.md ↔ .ru.md`:** the flow/placeholder material — note that the **same** `{{.ID}}/{{.Type}}/{{.From}}/
  {{.To}}` placeholders now expand in **shown descriptions/guidance** too (best-effort: raw on error; node
  descriptions see `From=To=status`), not just `commands`. One parallel clause each.
- **`CLI_REFERENCE.md ↔ .ru.md`:** where transition `description`/guidance is documented, mention the
  placeholder expansion (in human **and** `--json` output; a bad template degrades to raw). **Note the node
  rule:** in a **status** (node) description `{{.From}}`/`{{.To}}` both render that status's own name (a status
  is not a transition), so authors shouldn't expect edge/direction semantics there. Grep for
  `description`/`guidance`.
- **`CHANGELOG.md`** `[Unreleased]` → **Changed** (or Added): shown flow descriptions/guidance now expand the
  `{{.ID}}`/`{{.Type}}`/`{{.From}}`/`{{.To}}` placeholders (best-effort), so guidance names the concrete task.
- **CLAUDE.md:** `internal/core` (`ExpandText` — the lenient sibling of `expandCommands`) and `internal/cli`
  (guidance helpers expand descriptions; signatures thread `id`/`type`). Keep thin.
- **`AGENTS.md`:** no rule change expected.

## Sequencing & tracking (process, not code)

`t16` is `speccing` on `task/t16`. This document is the `speccing` deliverable. Next: commit it, adversarial
subagent **spec review**, `spec_human_review` → `planning` → `plan_review` → `plan_human_review` → TDD
`implementing` → `impl_review` → `approved` (auto PR) → merge → `deliver`. The **last** feature in the
**v0.10.0** batch (with `t44`/`t14`/`t28`/`t50`) — after it delivers, cut v0.10.0.
