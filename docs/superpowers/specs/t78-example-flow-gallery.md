# t78 — Example-flow gallery (quick-start pick-a-shape) + a docs/no-code template

**Task:** t78 · **Type:** task · **Status:** speccing · **Tags:** flow, release
**Relates to:** t62 (shipped the init templates), t75 (the repo's own no-code terminal — kept out of scope here).

This is a **decision record**. It fixes the scope, the deliverables, and the design of the one new
artifact, before the plan and the code.

## 1. Problem

t78's card asks for "2-3 sanitized example flows for common cases … keep the zero-config init default
minimal … ship as documented examples." Since the card was written, **t62 already shipped the substance**:

- runnable init templates `default` (built-in: `task`+`chore`, flat 3-state), `coding` (`feature` =
  3-state with a test gate on `done`; `bugfix` = repro-first `! make test`; `refactor` = API-stable),
  `hierarchy` (epic/task/subtask), and the `git-flow` flagship;
- FLOW_GUIDE.md already carries inline example flows: §2 minimal (`todo→doing→done` + a gate), §7
  content-review and approval/sign-off, §8 the git-integration walkthrough;
- `demo/` runs and tests the `coding` flow end-to-end.

Cross-checking the card's three literal examples:

| Card example | Status today |
|---|---|
| a simple 3-state with a test gate on `done` | **exists** — FLOW_GUIDE §2 + `coding` `feature` |
| a bugfix repro-first flow | **exists** — `coding` `bugfix` + `demo/` |
| a docs / no-code flow | **missing** (and entangled with t75) |

So the real gaps are **not** "no examples." They are:

1. **Discoverability / shape.** The material is scattered and written as a *reference* (FLOW_GUIDE = "how the
   engine works"), plus a file index in `templates/README.md`. Nothing is a **quick-start "pick your shape"
   menu** a newcomer lands on and chooses from. t78's own framing is "for quick start."
2. **One missing shape.** A **docs / no-code** starter — reach a terminal *without* a code/branch/PR gate —
   is not shipped as a template.

## 2. Goal / non-goals

**Goal.** A single, pickable **Example-flow gallery** that indexes every shipped shape with a one-line
"use when" and a copy-paste "try" command, plus the one missing **docs/no-code** runnable template. Doc-heavy,
minimal code, low sync risk.

**Non-goals (YAGNI / boundaries):**

- **No duplication of YAML.** The gallery *indexes* §2/§7/§8 and the `templates/` files; it does not restate
  their configs (that would create the parallel-occurrence drift this repo has been bitten by).
- **Not t75.** The docs/no-code template is a **generic starter for a fresh project** — legitimate because a
  flow is per-project data and a fresh project may define a direct `review→published` terminal with no
  git. It does **not** solve *this* repo's milestone/no-code close problem (the dogfood flow can only reach
  `done` via `approved`, which auto-opens a PR); that remains t75. The gallery must not imply otherwise.
- **No Go core change.** `templates.go` still embeds only `default.yaml`; `BuiltinNames()` stays `{"default"}`.
  The docs template is a hosted, non-embedded example like `coding`/`hierarchy`/`git-flow`.
- **`init` default stays minimal** (`task`+`chore`) — unchanged.

## 3. Decisions

### D1 — The gallery lives as FLOW_GUIDE **§2b** (canonical), pointed to from README

- New section **`## 2b. Example flow gallery — pick a shape`**, inserted right after §2 (the minimal flow).
  A **lettered** subsection (precedent: the existing `## 5b. Lifecycle events …`) so **no section is
  renumbered** — every existing `§2/§5/§5b/§6/§8` cross-reference in FLOW_GUIDE (and the two `§`-refs in
  DESIGN) stays valid. Mirrored in FLOW_GUIDE.ru.md.
- README.md Quickstart gets **one pointer line** to the gallery (next to the existing hierarchy-template
  hint, README.md:129 / README.ru.md:131). No table in README — the menu has one canonical home.
- `templates/README.md` stops being a second "when to pick which" index: it stays a terse **file list** and
  points to the FLOW_GUIDE gallery as the canonical chooser (removing the two-index drift risk).

### D2 — The gallery is the full 5-shape menu + a pointer to §7

A table `Shape | Use when | Try`, ordered simplest→advanced, `git-flow` flagged **ADVANCED** with its
caveats (so it never reads as mandatory — the card's "sanitize or relabel" requirement):

| Shape | Use when | Try |
|---|---|---|
| **minimal** | one gate on `done`; solo; zero-config | `mtt init` (the built-in `default`) — see §2 |
| **coding** | a software project; per-type DoD (feature / bugfix / refactor) | `mtt init --template templates/coding.yaml` — see `demo/`, §4 |
| **docs / no-code** | docs, research, decisions — reach done **without** a code/branch/PR gate | `mtt init --template templates/docs.yaml` **(new)** |
| **hierarchy** | epic → task → subtask nesting | `mtt init --template templates/hierarchy.yaml` |
| **git-flow** ⚠ **ADVANCED** | branch → agent review → PR → deliver (this repo's own shape) | `mtt init --template templates/git-flow.yaml` — needs `gh`+`jq`; review before use; see §8 |

Plus one pointer line: **non-code domains** (content-review, approval/sign-off) — inline examples in **§7**.

Each "Try" also notes the `raw.githubusercontent.com/pashukhin/mtt/main/templates/<name>.yaml` URL form once
(as §7/README already do), not per row.

### D3 — New runnable `templates/docs.yaml` (hosted, verbatim-installed)

A `doc` type: `draft → review → published`, plus a `cancelled` terminal (house style, matching every other
template). Its teaching contrast with `git-flow`: **no branch, no PR, no executed gate** — a doc reaches a
terminal on human review, not on a build. The `publish` edge is **gate-less**; a YAML comment shows how to
add an optional non-code check.

```yaml
version: 1
project:
  name: my-project
command_timeout: 5m
types:
  - name: doc
    description: A document or no-code deliverable. DoD — drafted, reviewed, published. No branch, no PR.
    prefix: d
    parents: []
    default: true
    statuses:
      - {name: draft,     kind: initial}
      - {name: review,    kind: active}
      - {name: published, kind: terminal}
      - {name: cancelled, kind: terminal}
    # A docs / no-code flow gates on human review, not on a build: no branch is
    # created, no PR is opened, and `publish` runs nothing. Want an automated
    # non-code check? Hang a `commands:` list on `publish` (see FLOW_GUIDE §4),
    # e.g. a markdown linter or a link checker.
    transitions:
      - {from: draft,  to: review,     name: submit,  description: "hand the draft to a reviewer", current: set}
      - {from: draft,  to: cancelled,  description: "drop the draft"}
      - {from: review, to: published,  name: publish, description: "reviewed — publish the doc", current: clear}
      - {from: review, to: cancelled,  description: "reject the doc"}
```

Contract details this must honour (from `templates/CLAUDE.md` invariants):

- **Verbatim install** → **literal** `project.name: my-project` (never `{{.Name}}`; an unquoted `{{.Name}}`
  in a verbatim file is invalid standalone YAML — this is why only `default.yaml` keeps the placeholder).
- **Structurally valid** per FLOW_GUIDE §11: one `initial` (`draft`), one `active` (`review`), two
  `terminal` (`published`, `cancelled`); `kind` matches topology; single `default` type; edge names
  (`submit`, `publish`) unique per source status and disjoint from status names; `prefix` letters-only.
- `command_timeout` is kept for template-family consistency even though there are no commands (a user who
  later adds a gate benefits); it is inert otherwise.

### D4 — CI honesty

- Add `"docs.yaml"` to the explicit list in `TestExamplesLoadAndValidate` (`templates/templates_test.go`),
  so the new example is loaded and validated exactly like the others.
- Add a small **`TestDocsIsNoCode`** guard: assert `docs.yaml` contains **no `git` and no `gh` tokens**
  (case-insensitive, **word-boundary** regex — mirroring the existing `ghToken` trick in
  `TestGitFlowStatesItsAssumptions`, which already avoids the `through`/`high` bigram). This makes the
  gallery's "no branch/PR" promise **machine-true** and stops a future edit from sneaking VCS/PR automation
  into the no-code starter. The template's prose deliberately says "no branch, no PR" (words `branch`/`PR`),
  **not** "git"/"gh", so it passes — and the test intentionally forbids only the two tool tokens, not the
  descriptive words.

### D5 — Docs sync surface (bilingual where required)

Every place that enumerates the example set or teaches "pick a flow" is updated together:

- `FLOW_GUIDE.md` + `FLOW_GUIDE.ru.md` — the new §2b gallery; and the §1 install line (FLOW_GUIDE.md:32 /
  .ru:31) and §12 "Neighbours" (:285 / :287) mention `docs` alongside `coding`/`hierarchy`/`git-flow`.
- `README.md` + `README.ru.md` — the Quickstart pointer to §2b.
- `templates/README.md` — add the `docs.yaml` row; point to the gallery as canonical chooser. (EN-only: the
  `templates/` docs are not in CLAUDE.md's bilingual pair list.)
- `templates/CLAUDE.md` — add `docs.yaml` to the hosted-examples enumeration (lines 11–12, 19) and its
  invariants (verbatim / literal name / no-code guard test).
- `CHANGELOG.md` — one line under `[Unreleased]` (a new shipped template + a gallery is user-facing). Note:
  the impl-submit gate's "code changed → needs CHANGELOG" check inspects only `cmd internal pkg go.mod
  go.sum`; `templates/` is outside it, so this entry is **hygiene, not gate-forced** — we add it anyway.

## 4. Test plan (red → green)

Order matters (TDD):

1. **`templates_test.go`** — extend `TestExamplesLoadAndValidate` with `"docs.yaml"` (RED: file absent →
   read error). Add `TestDocsIsNoCode` (RED: file absent). Create `templates/docs.yaml` → both GREEN.
2. Everything else is documentation; `make check` (gofmt/vet/golangci-lint/`go test -race`/build) must stay
   green. No new Go source under `cmd/internal/pkg`.

## 5. Files touched (summary)

New: `templates/docs.yaml`, `docs/superpowers/specs/t78-example-flow-gallery.md` (this file), later
`docs/superpowers/plans/t78-*.md`.
Edited: `templates/templates_test.go`, `templates/README.md`, `templates/CLAUDE.md`, `FLOW_GUIDE.md`,
`FLOW_GUIDE.ru.md`, `README.md`, `README.ru.md`, `CHANGELOG.md`.

## 6. Risks & mitigations

- **Two indexes drift** (templates/README vs gallery) → D1 demotes templates/README to a file list pointing
  at the canonical gallery.
- **Renumber cascade in FLOW_GUIDE** → D1 uses a lettered §2b (existing §5b precedent); zero renumbering.
- **docs.yaml over-promises vs t75** → D2/D3 label it a generic starter; the gallery text never claims it
  closes the repo's own milestone tasks.
- **`TestDocsIsNoCode` false positives** (`prefix`/`project`/`approve` contain "pr"; "through"/"high"
  contain "gh") → forbid only word-boundary `git`/`gh`, not `pr`/`branch`; prose avoids `git`/`gh`.
- **Bilingual desync** → D5 lists the full EN+RU surface; the plan pins exact anchors.

## 7. Out of scope

Moving §2/§7/§8 examples into the gallery; a `mtt templates`/`--list` CLI; embedding more templates in the
binary; any change to `init`'s default; solving t75.
