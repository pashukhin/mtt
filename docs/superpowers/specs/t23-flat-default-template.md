# t23 — `mtt init`: flat, frictionless default template (Part B)

- **Task:** t23 (narrowed from the original 3-thread task on 2026-07-17)
- **Split-off:** Part A (scaffold *how-to-use-mtt* agent docs) → **t46** (backlog)
- **Status:** spec (speccing)
- **Unblocks:** t4 (coding-template demo); **feeds:** t42 (user-docs audit)

## Problem

`mtt init` defaults to the `default` template, whose entry type is
```yaml
- name: task
  prefix: t
  parents: [epic]   # non-root
  default: true
```
Because `task` is the default type **and** declares a parent (`parents: [epic]` ⇒
`Type.IsRoot() == false`), the most natural first command after `mtt init` —
`mtt add "my first task"` — fails:

```
type "task" requires a parent; use --parent <id> (or --no-parent to create it at the top level)
```

A fresh project's first interaction is a wall. A smoke test hit exactly this during
the s009.x batch.

## Key principle — the engine is correct; the config is wrong

`requires a parent` is **not an engine bug**. It is a faithful property of a
*hierarchy* config: a non-root type must be placed under a parent. `core.Adder`,
`Type.IsRoot`, and `AcceptsParent` stay **untouched**. The domain does not support an
"optional parent" (a type is either root — `parents: []` — or requires one of its
declared parents, bypassable only with `--no-parent`). Therefore the fix is
**config-level only**: change *which* config `mtt init` ships by default.

Corollary (the design pivot): you cannot simultaneously keep the epic→task→subtask
hierarchy **and** make the default-typed `mtt add` frictionless. One must give. We
give the *default template* a flat model and preserve the hierarchy as an explicit
opt-in template.

## Scope

**In scope (Part B):** the default-config / starter-template ergonomics.
**Out of scope:** scaffolding agent docs (AGENTS.md/CLAUDE.md/GEMINI.md) — that is
**t46**. No engine/domain changes. No new flow features. We do **not** ship this
repo's specialized dogfood flow (speccing→…→approved→done, post-actions, git/gh) as a
template — that flow is tailored to our conditions and must not be imposed on fresh
projects.

## Decision — the template set after t23

| Template    | Types                              | Change                                   |
|-------------|------------------------------------|------------------------------------------|
| `default`   | `task` (root, default) + `chore` (root) | **new content** (flat)              |
| `hierarchy` | `epic` / `task` / `subtask`        | **relocation** of today's `default` (verbatim) |
| `coding`    | `feature` / `bugfix` / `refactor`  | **unchanged**                            |

`mtt init` still defaults to `--template default`; that default is now flat, so
`mtt add "x"` works out of the box.

### New `default` template (flat, generic)

Two root types sharing one minimal, generic flow. No `make`/`git`/spec assumptions;
no transition/status descriptions (an honest bare starter — projects add their own
guidance). `current: set/clear` are kept (useful, tool-agnostic).

```yaml
version: 1
project:
  name: {{.Name}}
command_timeout: 5m
types:
  - name: task
    description: A unit of work.
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, current: set}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, current: clear}
      - {from: in_progress, to: cancelled}
  - name: chore
    description: A small maintenance change.
    prefix: c
    parents: []
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, current: set}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, current: clear}
      - {from: in_progress, to: cancelled}
```

`task` + `chore` mirrors how this repo actually dogfoods mtt (two flat root types).

### New `hierarchy` template

Byte-for-byte today's `default.yaml` (with `{{.Name}}`): `epic`/`task`/`subtask`,
`task` default and parent-requiring. This **deliberately keeps** the "place it in the
hierarchy" discipline — for a hierarchy project, `mtt add` requiring `--type epic` or
`--parent` is a feature, not a bug, and the existing helpful error already points the
way (`--parent` / `--no-parent`). Its golden equals today's `default` golden.

### `coding` template

Untouched. It has **no** ergonomics bug (`feature` is root + default). Its
`make lint`/`make test` gate commands are illustrative for a coding project; any
demo-driven polish belongs to **t4**, not here.

## Testing strategy (TDD: red → green)

New tests (adapter/yaml + cli e2e):

1. **Ergonomics regression (the headline red test).** A fresh `mtt init` (default
   template) followed by `mtt add "x"` **succeeds** and creates a root task `t1`
   with **no** `--parent`/`--no-parent`. Fails today (default→task requires a
   parent); passes after the flatten. (cli testscript.)
2. **Per-template validity.** For **each** of `default`, `coding`, `hierarchy`:
   render → write to a temp `.mtt/config.yaml` → `Load` → `Config.Validate` returns
   no error. No such guard exists today, so a structurally invalid shipped template
   would go unnoticed. (adapter/yaml test.)
3. **Goldens.** Regenerate `testdata/golden/default.yaml` (now flat) and add
   `testdata/golden/hierarchy.yaml`; extend `TestRenderGolden`'s template list to
   include `hierarchy` (via `-update`). `coding.yaml` golden unchanged.

## Test migration — the 12 e2e scripts on the hierarchy fixture

All 28 cli testscripts obtain their config via `mtt init` (none hand-write a config).
**12** rely on the default template's epic→task→subtask hierarchy as a fixture:

```
batch, list_edit, ready, tags, add_depends_on, rm,
tree, types_json, roadmap, add_show, init, dep
```

Migration:

- **Most (build epic/task/subtask via `--type`/`--parent`):** change the setup line
  `exec mtt init` → `exec mtt init --template hierarchy`. Behavior and assertions
  unchanged (same epic/task/subtask prefixes e/t/s).
- **`init.txt`:** asserts the *default* `mtt types` shows `epic`/`task`/`subtask` →
  update to assert `task`/`chore` for the (now flat) default; keep the coding-template
  block; add a `--template hierarchy` block asserting epic/task/subtask so the new
  template has direct coverage.
- **`add_show.txt`:** its `! exec mtt add 'bare task'` / `stderr 'requires a parent'`
  assertion stays **valid** under `--template hierarchy` — switch its init line and
  the assertions hold.
- **`types_json.txt`:** switch init to `--template hierarchy` (it inspects the
  hierarchical type graph) — or update the expected JSON to the flat default; prefer
  the former to keep the JSON assertions intact.

Verify the exact per-script edits against a green `make check`; the list above is the
plan's starting point, `make check` is the arbiter.

## Docs sync (EN + RU — grep **all** parallel occurrences)

Per the parallel-occurrences rule, grep every mention across EN+RU and update in
lockstep:

- `CLI_REFERENCE.md` / `CLI_REFERENCE.ru.md`: `--template <name>` now
  `default | coding | hierarchy`; describe the flat default (`task`+`chore`,
  frictionless `mtt add`) and `hierarchy` as the epic/task/subtask opt-in; review the
  `default/coding … set` note (~line 430).
- `DESIGN.md` / `DESIGN.ru.md`: wherever the default template / init hierarchy is
  described.
- `README.md` / `README.ru.md`: wherever epic/task/subtask is shown as the default.
- `internal/adapter/yaml/CLAUDE.md`: template list (`default`/`coding` → add
  `hierarchy`); "Templates are the only home of default type/status names."

## Non-goals / explicitly deferred

- Agent-doc scaffolding (AGENTS.md/CLAUDE.md/GEMINI.md) → **t46**.
- Coding-template demo polish → **t4**.
- Any engine/domain change (optional-parent semantics, new statuses, post-actions).
- Inline teaching comments inside templates — considered, dropped for now (keeps the
  starter minimal and goldens lean; revisit under t46's doc-scaffolding umbrella).

## Risks

- **Test churn (12 scripts).** Mitigated: mostly one-line init changes; `hierarchy`
  keeps the same type/prefix shape, so assertions are largely stable. `make check` is
  the gate.
- **A downstream consumer assuming the old default hierarchy.** Grep confirmed the
  only consumers are the testscripts (migrated) and docs (synced). The repo's own
  `.mtt/config.yaml` is hand-authored (dogfood), independent of templates —
  unaffected; `TestRepoDogfoodConfig` stays green.

## Downstream

- **t4** (coding demo) unblocks: `coding` confirmed coherent, no ergonomics bug.
- **t42** (docs audit) picks up the reconciled template docs as part of its sweep.
