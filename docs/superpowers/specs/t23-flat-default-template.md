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

Corollary (the design pivot): you cannot keep the epic→task→subtask hierarchy **and**
make the *mid-level* "unit of work" type both the default and frictionless. (Making
the **root** `epic` the default would keep the hierarchy *and* a frictionless
`mtt add` — but then the first thing every project creates is an epic, and the
friction merely shifts to "now add a task under it".) So we give the *default
template* a flat model and preserve the hierarchy as an explicit opt-in template.

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

## Implementation plumbing (load-bearing — easy to miss)

Adding `templates/hierarchy.yaml` requires editing **both** spots in
`internal/adapter/yaml/templates.go`:
- the `//go:embed templates/default.yaml templates/coding.yaml` directive (add
  `templates/hierarchy.yaml`) — a bare `//go:embed` of a missing file is a **compile
  error**, so this can't silently ship, but it must be done;
- the `templateFiles` map (`{"default":…, "coding":…}` → add `"hierarchy"`) — omitting
  this yields a runtime `unknown template "hierarchy"` from `renderTemplate` (and the
  new validity test + `TestRenderGolden` would fail).

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

## Test migration — the e2e scripts on the hierarchy fixture

All 28 cli testscripts run `mtt init`; 15 then `cp` a self-contained config over the
default (so they don't depend on it). An adversarial re-check corrected the naive
`grep epic|subtask` list (which had two false members). The precise picture:

**(a) 10 scripts go red and need a one-line init switch** `exec mtt init` →
`exec mtt init --template hierarchy` (they build epic→task→subtask via
`--type`/`--parent`; same e/t/s prefixes, so assertions hold unchanged):

```
add_depends_on, add_show, batch, dep, list_edit, ready, rm, roadmap, tags, tree
```

`add_show.txt`'s `! exec mtt add 'bare task'` / `stderr 'requires a parent'`
assertion stays **valid** under `--template hierarchy` (the hierarchy `task` still
requires a parent) — the init-line switch suffices.

**(b) `init.txt` needs a real rewrite** (not one line): it asserts the *default*
`mtt types` shows `epic`/`task`/`subtask` (incl. the single-type sub-block
`mtt types epic` → `prefix e`). Rewrite the default block to assert `task`/`chore`;
keep the coding-template block; add a `--template hierarchy` block asserting
epic/task/subtask so the new template has direct e2e coverage.

**(c) `add_json.txt` — a text/comment fix, not a failure.** It runs default
`mtt init` then `mtt add 'first task' --no-parent` with the comment *"The default
type requires a parent, so create the first task with --no-parent."* Under the flat
default, `task` is root, so `--no-parent` is silently accepted (no error) — the test
stays **green**, but the comment is now false and the `--no-parent` flags are
redundant. Fix: drop the now-pointless `--no-parent` and correct the comment (it is a
parallel occurrence of the changed fact — the exact class the parallel-occurrence
rule targets).

**Explicitly NOT migrated:**
- **`types_json.txt`** — the naive grep flagged it, but its only type-specific
  assertions are `mtt types task --json` → `"name": "task"` present + `!"name": "epic"`
  absent (both still hold: `task` exists, `epic` doesn't), and its `"commands"`/`"post"`
  key checks always emit (non-`omitempty`). **Stays green, no change.**
- `dogfood`, `command_timeout`, `structured_commands`, `rollback` — each `cp`s a
  self-contained config over `.mtt/config.yaml` right after `mtt init`, so they never
  depend on the default template.

Verify every per-script edit against a green `make check` — it is the arbiter.

## Docs sync (EN + RU — enumerated parallel occurrences)

Per the parallel-occurrence rule (which prior tasks c5/t27 missed), the specific
occurrences to update in lockstep. Line numbers are anchors, not contracts — re-grep
`epic|subtask|template|default config` in each file before editing.

**Code (no `make check` guard — will silently ship stale):**
- `internal/cli/init.go:51` — the `--template` flag help `"starter template:
  default|coding"` → `"default|coding|hierarchy"`.
- `internal/adapter/yaml/CLAUDE.md` — the `Init` template list (`default`/`coding` →
  add `hierarchy`); "Templates are the only home of default type/status names."

**README.md / README.ru.md — two occurrences, both in each language:**
- `:46` (+ru:46) — the feature bullet *"Config-driven types & hierarchy. Epic → task →
  subtask is **just the default config**…"* → false after t23; it is the `hierarchy`
  template now. Reword (e.g. "…is just the `hierarchy` template" or drop "default").
- `:111–117` (+ru) — the **Quickstart is BROKEN**, not merely mislabeled: it runs
  `mtt init  # default template: epic > task > subtask` then
  `mtt add "Ship auth" --type epic` → after t23 this is `unknown type "epic"`. Real
  rewrite in **both** languages: front the walkthrough with
  `mtt init --template hierarchy`, or re-author it flat (`mtt add "..."` at root, no
  `--type epic`). Also fix the `mtt tree # the epic > task hierarchy` comment.

**DESIGN.md / DESIGN.ru.md:**
- `:226-227` (+ru:228) — *"The epic → task → subtask hierarchy … follows from the
  **default config**"* → becomes false; now it follows from the **`hierarchy`
  template**. Reword.
- `:614` (+ru) — *"`mtt init` writes `.mtt/config.yaml` with example types
  `epic`/`task`/`subtask`"* → now `task`/`chore` by default. Reword.
- `:623-650` (+ru) — the example config YAML block (epic/task/subtask): reframe as
  *the `hierarchy` template*, or replace with the flat default. Decide in the plan.
- `:73` blockquote (+ru) and `:217`/`:674` (epic/task/subtask as *examples*): review
  — likely fine as capability examples (hierarchy still ships), touch only if they
  claim it is the default. The ID-scheme examples (`:186-188`, `:235-236`, `:298`,
  `:327`) are template-independent — leave.

**CLI_REFERENCE.md / CLI_REFERENCE.ru.md:**
- `:146` (+ru) — *"Creates `.mtt/` with a default `config.yaml` (types
  `epic`/`task`/`subtask` …)"* → `task`/`chore`. Reword.
- `:153` (+ru) — `--template <name>` — `default (epic/task/subtask, no commands) or
  coding` → `default (task+chore, flat) | coding | hierarchy (epic/task/subtask)`.
- `:303` (+ru) — *"Prints the epic → task → subtask tree"* (the `mtt tree` example):
  reword to a generic tree or note it as a hierarchy-template example.
- `:430` (+ru) — *"The default/`coding` templates `set` on → `in_progress`"* — **stays
  true** (flat `task`+`chore` keep `current: set/clear`). Confirm, no change.
- `:285` (+ru, `mtt rm <epic> <child>`) — a subtree-rm capability example; reword to
  generic only if convenient, not load-bearing.

## Non-goals / explicitly deferred

- Agent-doc scaffolding (AGENTS.md/CLAUDE.md/GEMINI.md) → **t46**.
- Coding-template demo polish → **t4**.
- Any engine/domain change (optional-parent semantics, new statuses, post-actions).
- Inline teaching comments inside templates — considered, dropped for now (keeps the
  starter minimal and goldens lean; revisit under t46's doc-scaffolding umbrella).

## Risks

- **Test churn (~11 scripts + a docs sweep).** Mitigated: 10 are one-line init
  switches; only `init.txt` is a real rewrite, and `add_json.txt` a comment/flag
  touch; `hierarchy` keeps the same type/prefix shape, so assertions are largely
  stable. `make check` is the gate.
- **A downstream consumer assuming the old default hierarchy.** Grep confirmed the
  only consumers are the testscripts (migrated) and docs (synced). The repo's own
  `.mtt/config.yaml` is hand-authored (dogfood), independent of templates —
  unaffected; `TestRepoDogfoodConfig` stays green.

## Downstream

- **t4** (coding demo) unblocks: `coding` confirmed coherent, no ergonomics bug.
- **t42** (docs audit) picks up the reconciled template docs as part of its sweep.
