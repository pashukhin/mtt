# t78 — Example-flow gallery (quick-start pick-a-shape) + a docs/no-code template

**Task:** t78 · **Type:** task · **Status:** speccing · **Tags:** flow, release
**Relates to:** t62 (shipped the init templates), t75 (the repo's own no-code terminal — kept out of scope here).

This is a **decision record**. It fixes the scope, the deliverables, and the design of the one new
artifact, before the plan and the code.

> **rev2 — response to adversarial spec review (rev1 DECLINE).** Changes: (MAJOR1) added the
> `CLI_REFERENCE.md`/`.ru` enumeration to the sync surface, plus the `templates.go`/`load_test.go` doc
> comments — and **deliberately excluded** `DESIGN.md`/`.ru` with a stated reason (D5). (MAJOR2) **reshaped
> `docs.yaml`** to a `review ⇄ revision` loop (two active states, a cycle) so it is a genuinely distinct
> shape — not `default` renamed, not a §7 duplicate (D3). (MINORs) the `minimal` row no longer implies a
> shipped gate (D2); `TestDocsIsNoCode` is now **structural** — asserts the template executes nothing —
> closing the git/gh-token false-negative (D4).

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
| **minimal** | solo, zero-config; one linear flow you add your own gate to | `mtt init` (the built-in `default` — gate-less; §2 shows adding a gate) |
| **coding** | a software project; per-type DoD (feature / bugfix / refactor) | `mtt init --template templates/coding.yaml` — see `demo/`, §4 |
| **docs / no-code** | an iterative **review ⇄ revision** cycle with no code gate — docs, RFCs, specs, articles | `mtt init --template templates/docs.yaml` **(new)** |
| **hierarchy** | epic → task → subtask nesting | `mtt init --template templates/hierarchy.yaml` |
| **git-flow** ⚠ **ADVANCED** | branch → agent review → PR → deliver (this repo's own shape) | `mtt init --template templates/git-flow.yaml` — needs `gh`+`jq`; review before use; see §8 |

Plus one pointer line: **non-code domains with gates/posts** (content-review's publish gate, approval's
validate gate) — inline examples in **§7**.

The rows are deliberately non-overlapping on their *teaching* axis: `minimal` = a linear generic flow;
`docs` = a **cyclic** no-code flow (the loop is the lesson); §7 = non-code flows that **carry a gate/post**.
None restates another's YAML.

Each "Try" also notes the `raw.githubusercontent.com/pashukhin/mtt/main/templates/<name>.yaml` URL form once
(as §7/README already do), not per row.

### D3 — New runnable `templates/docs.yaml` (hosted, verbatim-installed)

A `doc` type with a **review ⇄ revision loop**: `draft → review`, `review → published` (accepted) or
`review → revision` (send back), `revision → review` (re-review), plus a `cancelled` terminal from every
non-terminal state. **Nothing is executed** — no branch, no PR, no gate; a doc reaches a terminal on human
review, not a build.

**Why this is a distinct shape (resolves rev1 MAJOR2).** Every other shipped flow is **linear**: the
built-in `default` (`todo→in_progress→done`), `coding`, and §7's content-review (`draft→review→published`)
all reach their terminal in one forward pass. `docs.yaml` is the only shape with **two active states and a
cycle** (`review ⇄ revision`) — its lesson is exactly that: *an active status can loop, and a terminal need
not be one hop away*. It does not restate `default` (which is generic and one-active) nor §7's content-review
(which is linear **and** carries a `commands:` gate + a `post:` — a different lesson). The gate-less,
side-effect-free body is the "no-code" half; the loop is the "iterative" half.

```yaml
version: 1
project:
  name: my-project
command_timeout: 5m
types:
  - name: doc
    description: A document or no-code deliverable (RFC, spec, article). DoD — reviewed and published; iterate via revision. No branch, no PR, no build gate.
    prefix: d
    parents: []
    default: true
    statuses:
      - {name: draft,     kind: initial}
      - {name: review,    kind: active}
      - {name: revision,  kind: active}
      - {name: published, kind: terminal}
      - {name: cancelled, kind: terminal}
    # A docs / no-code flow gates on human review, not on a build: no branch is
    # created, no PR is opened, nothing runs. The review<->revision loop is the
    # point — a reviewer can send a doc back any number of times. Want an
    # automated non-code check (a link checker, a markdown linter)? Hang a
    # `commands:` list on `publish` (see FLOW_GUIDE §4).
    transitions:
      - {from: draft,    to: review,    name: submit,          description: "hand the draft to a reviewer", current: set}
      - {from: draft,    to: cancelled, description: "drop the draft"}
      - {from: review,   to: published, name: publish,         description: "accepted — publish", current: clear}
      - {from: review,   to: revision,  name: request_changes, description: "send back for edits"}
      - {from: review,   to: cancelled, description: "reject the doc"}
      - {from: revision, to: review,    name: resubmit,        description: "edits done — re-review"}
      - {from: revision, to: cancelled, description: "abandon the doc"}
```

Contract details this must honour (from `templates/CLAUDE.md` invariants), all re-checked for the reshaped
graph:

- **Verbatim install** → **literal** `project.name: my-project` (never `{{.Name}}`; an unquoted `{{.Name}}`
  in a verbatim file is invalid standalone YAML — this is why only `default.yaml` keeps the placeholder).
- **Structurally valid** per FLOW_GUIDE §11: one `initial` (`draft`, no incoming); two `active`
  (`review` in←{draft,revision} out→{published,revision,cancelled}; `revision` in←review out→{review,
  cancelled}); two `terminal` (`published`, `cancelled`, no outgoing). `kind` matches topology; single
  `default` type; edge names `submit`/`publish`/`request_changes`/`resubmit` are unique **per source
  status** and disjoint from status names; `prefix` letters-only. (The `review↔revision` cycle is legal:
  both endpoints are `active`, which requires in **and** out edges — the cycle supplies both.)
- **`current`** is `set` when entering the active cycle (`submit`) and `clear` when leaving it to a terminal
  (`publish`); the intra-cycle edges (`request_changes`, `resubmit`) and the `cancelled` edges carry no
  `current` directive (matching `coding.yaml`, which never touches `current` on its cancel edges).
- `command_timeout` is kept for template-family consistency even though there are no commands (a user who
  later adds a gate benefits); it is inert otherwise.

### D4 — CI honesty

- Add `"docs.yaml"` to the explicit list in `TestExamplesLoadAndValidate` (`templates/templates_test.go`),
  so the new example is loaded and validated exactly like the others.
- Add a **`TestDocsIsNoCode`** guard that is **structural, not textual** (rev1 MINOR: a git/gh-token grep
  has a false-negative — a `push`/`branch`/`pr`/`merge` reference or a non-git deploy gate like
  `make deploy`/`curl …/pipelines` carries none of those tokens). The test unmarshals `docs.yaml` (via
  `gopkg.in/yaml.v3`, already a dependency) into a minimal struct that captures only the executable
  surfaces, and asserts every one is empty:
  - top-level `events:` is empty;
  - no type carries `post_defaults:`;
  - every transition has empty `commands:` **and** empty `post:`.

  This proves the shipped no-code starter **executes nothing at all** — the honest form of "no branch, no
  PR, no gate," immune to prose wording and catching *any* side-effecting step, not just git/gh. It is a
  tripwire, not a promise the template can never gain a gate: it guards the *shipped* file; a user who
  uncomments a `commands:` gate in their own copy is unaffected (the test reads `templates/docs.yaml`, not
  the user's install).

### D5 — Docs sync surface (bilingual where required)

A repo-wide sweep for the example-set enumeration (`grep -rn 'coding.yaml\|hierarchy.yaml\|git-flow'`)
classified every hit as **set-enumeration** (must gain `docs`) vs **specific-template reference** (leave
alone — those name `hierarchy`/`coding` for a specific example or fixture, not "the set"). Every
set-enumeration and the "pick a flow" surfaces are updated together:

- `FLOW_GUIDE.md` + `FLOW_GUIDE.ru.md` — the new §2b gallery; and the §1 install line (FLOW_GUIDE.md:32 /
  .ru:31) and §12 "Neighbours" (:287 / :289) mention `docs` alongside `coding`/`hierarchy`/`git-flow`.
- `README.md` + `README.ru.md` — the Quickstart pointer to §2b (near README.md:129 / .ru:131; exact anchor
  pinned in the plan).
- `CLI_REFERENCE.md` + `CLI_REFERENCE.ru.md` — the `mtt init --template` prose enumerates the hosted set
  (CLI_REFERENCE.md:228 / .ru:229 "…(`coding`, `hierarchy`, and the `git-flow` flagship)…"); add `docs`.
  **(rev1 MAJOR1.)**
- `templates/README.md` — add the `docs.yaml` row; demote it to a terse file list that points to the §2b
  gallery as the canonical "which to pick" chooser (removes the two-index drift). (EN-only: `templates/`
  docs are not in CLAUDE.md's bilingual pair list.)
- `templates/CLAUDE.md` — add `docs.yaml` to the hosted-examples enumeration (lines 11–12, 19) and note its
  invariants (verbatim / literal name / the structural no-code guard).
- `templates/templates.go` (doc comment, line 4) and `internal/adapter/yaml/load_test.go` (comment, line
  13) both enumerate `(coding, hierarchy, git-flow)`. **Reword generically** ("the hosted examples") rather
  than appending `docs`, so these comments never drift again as the set grows. This is a comment-only touch;
  it does not violate the "no Go core change" non-goal (no logic, no exported surface). **(rev1 MINOR.)**
- `CHANGELOG.md` — one line under `[Unreleased]` (a new shipped template + a gallery is user-facing). Note:
  the impl-submit gate's "code changed → needs CHANGELOG" check inspects only `cmd internal pkg go.mod
  go.sum`; `templates/` is outside it, so this entry is **hygiene, not gate-forced** — we add it anyway.

**Deliberately NOT touched — `DESIGN.md:781` / `DESIGN.ru.md:788`** (rev1 MAJOR2 hunt turned this up too).
That sentence — "`coding`/`hierarchy` and the **git-flow flagship** … are hosted files" — lives inside the
**"Shipped (t62)"** narrative block: it is a *historical record of what t62 delivered*, not a live index of
the current set. Retrofitting `docs` (a t78 artifact) into a t62 Shipped block would be revisionist, and it
runs directly against **c23** (queued: "DESIGN unload — distill the Shipped blocks into KB notes; slim
DESIGN"). t78 adds no new DESIGN Shipped block either (a doc-only gallery + one sample does not earn one;
if anything it is c23's material). The other DESIGN/README hits (DESIGN.md:699/:763, README.md:56, the
`demo/` READMEs, the `internal/cli/testdata/scripts/*.txt` and `*_test.go` fixtures) are
**specific-template references**, not set-enumerations — left alone.

## 4. Test plan (red → green)

Order matters (TDD):

1. **`templates_test.go`** — extend `TestExamplesLoadAndValidate` with `"docs.yaml"` (RED: file absent →
   read error). Add `TestDocsIsNoCode` (RED: file absent). Create `templates/docs.yaml` → both GREEN.
2. Everything else is documentation; `make check` (gofmt/vet/golangci-lint/`go test -race`/build) must stay
   green. No new Go source under `cmd/internal/pkg`.

## 5. Files touched (summary)

New: `templates/docs.yaml`, `docs/superpowers/specs/t78-example-flow-gallery.md` (this file), later
`docs/superpowers/plans/t78-*.md`.
Edited: `templates/templates_test.go`, `templates/templates.go` (comment), `templates/README.md`,
`templates/CLAUDE.md`, `internal/adapter/yaml/load_test.go` (comment), `FLOW_GUIDE.md`, `FLOW_GUIDE.ru.md`,
`README.md`, `README.ru.md`, `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md`, `CHANGELOG.md`.
Deliberately **unchanged**: `DESIGN.md` / `DESIGN.ru.md` (see D5 — the enumeration there is a t62 Shipped
record, and is c23's territory).

## 6. Risks & mitigations

- **Two indexes drift** (templates/README vs gallery) → D1 demotes templates/README to a file list pointing
  at the canonical gallery.
- **Renumber cascade in FLOW_GUIDE** → D1 uses a lettered §2b (existing §5b precedent); zero renumbering.
- **docs.yaml over-promises vs t75** → D2/D3 label it a generic starter; the gallery text never claims it
  closes the repo's own milestone tasks.
- **docs.yaml not distinct** (rev1 MAJOR2) → D3 reshapes it to a `review ⇄ revision` cycle — the only
  shipped shape with a loop/two active states — with the distinctness argument stated inline.
- **No-code guard is theatre** (rev1 MINOR) → D4's `TestDocsIsNoCode` is structural (no `commands`/`post`/
  `post_defaults`/`events`), catching any executed step, not just git/gh tokens.
- **Bilingual / multi-file desync** → D5 lists the full EN+RU surface *including CLI_REFERENCE* and states
  which enumerations are deliberately excluded (DESIGN) and why; the plan pins exact anchors.

## 7. Out of scope

Moving §2/§7/§8 examples into the gallery; a `mtt templates`/`--list` CLI; embedding more templates in the
binary; any change to `init`'s default; solving t75.
