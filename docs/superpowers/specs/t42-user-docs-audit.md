# t42 — user-docs audit (release-gating docs sweep)

**Type:** task · **Tags:** docs, release · **Blocks:** t59 (pilot), t60 (public release)

Decision record. The audit reconciles the **human-facing** docs with the **actually-shipped**
surface before the pilot migration and public release. It is evidence-based (claim → proof), not
by-eye, and leaves behind a lightweight machine guard so the docs cannot silently drift again.

## Problem

The s009.x assessment and the 2026-07-22 audit flagged human docs that describe unshipped features
as present, internal contradictions, EN↔RU drift, and stale version strings. The original finding
line-numbers predate this session's t52/t46 deliveries (which already edited README/DESIGN/
CLI_REFERENCE/CHANGELOG in EN+RU for `mtt agent hooks`/`mtt agent docs` + init-default), so every
finding was **re-verified against the current `main`** (build `v0.10.0-51`) before being written down.

## Scope

**In scope (the ledger — full file-by-file pass):** `README.md`/`README.ru.md`,
`DESIGN.md`/`DESIGN.ru.md`, `CLI_REFERENCE.md`/`CLI_REFERENCE.ru.md`, `CHANGELOG.md`,
`FLOW_GUIDE.md`/`FLOW_GUIDE.ru.md`.

**Fact-check only, not rewritten:** `AGENTS.md`, the `CLAUDE.md` files (agent-facing docs with their
own English-only language rules) — corrected only where they intersect a finding (e.g. a phantom
command reference or a stale badge), never restructured to the bilingual rules.

**Out of scope:** feature work, new commands, doc restructuring beyond what a finding requires, the
DESIGN historical-narrative unload (that is t63).

**Audit axes:** (a) unshipped-as-present claims, (b) internal contradictions, (c) EN↔RU drift,
(d) stale versions/badges.

## Decisions (the brainstorm forks)

1. **Scope** — core four doc pairs + CHANGELOG, plus `FLOW_GUIDE`/.ru. AGENTS/CLAUDE are fact-checked
   on intersection only.
2. **Verification method** — evidence-based. Sources of truth, in order: the live **command registry**
   (`root.AddCommand(...)` + `mtt --help` + `mtt <cmd> --help`), **running the binary** (actually invoke
   documented commands/flags), and **shipped capability** read from code / DESIGN "Shipped(tX)" blocks.
   Every parallel occurrence of a changed fact is grepped (EN+RU + repeated "Shipped" blocks), not just
   the first — the docs restate facts in several places.
3. **Drift-guard** — ship a **lightweight Go guard test** in the `make check` gate (details below). This
   is what makes t42 a task (design-open), not a chore.
4. **`--json` claim** — the root help / CLI_REFERENCE / CHANGELOG say "all/every command supports
   `--json`". Empirically over-broad: `version --json`/`init --json` emit JSON, but the cobra scaffolding
   `completion`/`help` ignore it. Reword precisely ("every command that emits mtt output; the cobra
   completion/help scaffolding aside"), do not delete the claim.
5. **`mtt start` in README** — `start` is an edge **name in this repo's dogfood config only**; the shipped
   `default` template has statuses `tbd/in_progress/done/cancelled` and transitions with **no** `name:`, so
   `mtt start` does not resolve out of the box (whereas `mtt done`/`mtt in_progress`/`mtt cancelled` do, as
   status sugar). Replace pitch/"key ideas" uses of `mtt start` with template-neutral wording (the
   `mtt <status>` sugar / point at `mtt types`/`mtt show`); do **not** hardcode our flow. This applies to
   **every** `mtt start` occurrence, not just the README pitch — DESIGN.md/.ru carry it too (see F6). The
   README Quickstart block is already correct (`mtt status t1 in_progress` → `mtt done t1`).
6. **README vision sections** (`Key ideas` / `For agents` / `For humans`) — keep the product vision but mark
   each unshipped feature explicitly as **planned / later phase**, in sync with the top status caveat. This
   removes the caveat↔body contradiction while keeping the pitch's ambition honest.
7. **TDD form for a docs task** — the guard test is a genuine red→green vehicle (write it → it fails on the
   current docs → fix docs → green). Findings with no machine assertion (unshipped-as-present prose, EN↔RU
   drift, the broken YAML) ride a `claim → evidence` ledger in the plan, fixed and re-verified by grep /
   binary run. `make check` is green before every commit (CHANGELOG gate + bilingual sync).

## Findings ledger (re-verified against current main)

Each item is a confirmed defect with its proof. The planning step expands this into a file-by-file,
EN+RU-paired fix list; new findings surfaced during the full pass are appended there.

**Version / badge**
- **F1** — `README.md:7` & `README.ru.md:7`: the whole **Status** line is stale, not just the badge.
  (a) status badge `0.8.98-dev` vs build `v0.10.0-51` (*proof:* `mtt version`; the version is injected at
  build time via ldflags, so any pinned number in prose is stale by construction). (b) "Phases 1–3 are
  implemented" is itself out of date — the KB (`mtt note`/`mtt prime`, DESIGN phase 5), references
  (`mtt ref`/`mtt check`), and the agent-scaffold group (`mtt agent`) have all shipped since. **Fix owns the
  entire status line:** a version-agnostic, surface-accurate phrasing (drop the pinned number and the stale
  phase enumeration; point at `mtt version` / `mtt --help` for the live surface), not a badge-only swap.

**Unshipped-as-present (README)**
- **F2** — `README` Key ideas: "Optional features (history, dependencies, **comment trees, search**) are
  per-adapter capabilities." Comment trees (t2) and search (t6) are unshipped. *Proof:* registry has no
  `comment`/`search`; DESIGN KB row marks search t6, comments t2.
- **F3** — Key ideas: "Tasks/**comments** carry checkable refs." Refs shipped (t1); comments not (t2).
- **F4** — Key ideas + For humans: **`mtt-ui`** described as present; unshipped (t8) — and the top caveat
  already calls it a later phase (internal contradiction).
- **F5** — For agents: "**comment trees**, optional knowledge storage, **simple text search**, external
  **indexer hook**." Knowledge storage shipped (t47 `mtt note`); comment trees (t2), text search (t6),
  indexer hook (t9) unshipped.
- **F6** — `mtt start` used as if usable, though it does not resolve on the shipped default template (see
  decision 5). Occurrences: `README.md:23`, `:45`, **and `DESIGN.md:380` / `DESIGN.ru.md:385`**
  ("the agent works in task terms (`mtt start t17`, `mtt done t17`)"). *Proof (empirical):* temp-dir
  `mtt init` → `mtt start t1` → `unknown command "start"` (exit 1); `mtt in_progress t1` / `mtt done t1`
  route. README's own caveat (`:11`) already says the `advance`/`start`/`done` meta-walk is parked →
  contradiction. Both the README and DESIGN pairs (EN+RU) are fixed.

**`--json` over-broad**
- **F7** — `CLI_REFERENCE.md:902` / `.ru:897` ("every command supports JSON"), `CHANGELOG.md:209`
  ("every command honors `--json`"), and the root long-help ("All commands support --json"). *Proof:*
  `completion bash --json` and `help --json` ignore the flag. Reword per decision 4 (root help text is
  code, in `internal/cli/root.go` — fix there too, it is the source the docs mirror).

**CHANGELOG**
- **F8** — `CHANGELOG.md:173` (the `0.9.0` "First public release" entry) lists a `mtt guide` command that
  never existed. *Proof:* not in the registry, not in `mtt --help`, no `newGuideCmd`. Fix the erroneous
  claim in the historical entry (a factual correction is allowed; the `coding` built-in template it also
  mentions **was** true at 0.9.0 and stays — the t62 breaking removal is already noted under Unreleased).

**DESIGN broken YAML + EN↔RU drift**
- **F9** — `DESIGN.md` ~`:705` / ~`:719` (string-anchored; line numbers drift): two YAML list items glued
  onto the previous line (`...to: cancelled}  - name: task` / `...  - name: subtask`) in the
  hierarchy-template example. `DESIGN.ru.md` is correct (no glue) → EN regressed against RU. Fix EN to match
  the correct RU shape.
- **F10** (missed by the first pass; surfaced in spec review) — `CHANGELOG.md:85` (under **[Unreleased]**,
  the c14 error-polish entry) claims `mtt init --template <bogus>` lists valid names `(coding, default,
  hierarchy)`. *Proof:* the binary now prints `error: unknown template "bogus" (valid: default)` (t62 removed
  the built-in `coding`/`hierarchy` names). This is (a) unshipped-as-present drift and (b) an internal
  contradiction with `CHANGELOG.md:35`, also under [Unreleased], which states those built-ins were removed.
  Both are still-unreleased entries → fix the c14 line to `(default)`.

**Minor / nice-to-have (plan may fold in):** `mtt comment add`/`mtt comment list` (`CLI_REFERENCE.md:682`/
`:687`) rely on a section-level `## Comments *(phase 4; …)*` marker rather than an inline `*(phase 4)*` tag
per heading, unlike every other unshipped command. Honest at the section level; a one-line inline tag would
match the surrounding convention. The full file-by-file pass appends any further drift found in flight.

## Drift-guard test (design — revised after spec review)

A single lightweight Go test living in **`internal/cli`** (it needs the exported `NewRootCmd()` at
`internal/cli/root.go:16` to walk the live registry) and reusing the existing **`repoRoot()`** helper
(`internal/cli/repo_test.go:13` — anchors via `runtime.Caller` up to `go.mod`, more robust than
`dogfood_test.go`'s cwd-dependent discovery). The registry is walked from `NewRootCmd().Commands()`
**without** running `Execute`, so cobra's lazily-injected `help`/`completion` do not appear (confirmed in
review). Three focused assertions:

1. **Registry ⊆ CLI_REFERENCE.** For each command in `NewRootCmd().Commands()`, assert some
   `CLI_REFERENCE.md` heading of the form ```### `mtt …` ``` whose backtick content **starts with
   `mtt <name>` at a word boundary**. This must handle reality (verified): headings carry the **full usage
   line** (```### `mtt add [title] [flags]` — create a task```), and **group** commands (`dep`/`ref`/`note`/
   `agent`/`tag`) have **no bare heading** — only subcommand headings (```### `mtt dep add …` ```), which
   still satisfy the `mtt dep`-prefix test. The word boundary distinguishes `tag` (```mtt tag add|rm …```)
   from `tags` (```mtt tags …```). Catches a shipped-but-undocumented command.
2. **Command tokens ⊆ registry ∪ allowlist.** Scan **README/.ru + CLI_REFERENCE/.ru + CHANGELOG** for
   inline command tokens ```` `mtt <word>` ````; each `<word>` must be (a) a registry command, (b) a
   shipped-`default`-template status used as sugar (`done`/`in_progress`/`cancelled`), or (c) in a small
   **explicit allowlist** of documented non-registry verbs — parked/planned/edge-sugar:
   `advance`, `start`, `cancel`, `caps`, `comment`, `search`, `gantt`, `ui`, **`decline`** (edge-verb
   sugar), **`reparent`**, **`move`** (both documented as planned). Catches phantoms like `mtt guide`.
   **`FLOW_GUIDE`/.ru are excluded from this scan by design** — that guide *teaches authoring custom
   flows*, so it legitimately contains arbitrary illustrative verbs (`mtt doing`, and any future example),
   which are indistinguishable from a phantom by token shape; the exclusion is a **documented, deliberate
   cap**, stated in the test comment so it never reads as "covers everything".
3. **No hardcoded version badge.** `README.md`/`README.ru.md` carry no hardcoded `\d+\.\d+\.\d+` semver
   (the only current match is the F1 badge, so a whole-file scan is safe; the status blockquote is located
   by `> **Status:**` / `> **Статус:**`). Catches a re-introduced pinned badge.

**Honest framing (revised).** The allowlist is *not* zero-cost: it is a small, explicit, reviewed
enumeration of the known non-registry verbs the tool's docs mention. Adding a new parked/planned/edge-sugar
verb mention to README/CLI_REFERENCE/CHANGELOG requires a one-line allowlist update — a deliberate forcing
function that also blocks a phantom from slipping in. That is a real (small) maintenance point, not "no
maintenance tax". The exact extraction regexes are pinned during TDD: write the test first, confirm it goes
**red** on the current docs (F1 badge + F8 `mtt guide` + any undocumented command), fix the docs, confirm
**green** — including that the completed allowlist leaves the corrected docs (with their legitimate
`decline`/`reparent`/`move` mentions) passing.

## Non-goals / risks

- **Not** a rewrite of the docs' structure or a positioning rework beyond honesty fixes.
- **Risk: over-tight guard.** The token-scan could false-positive on legitimate future-command mentions;
  mitigated by the explicit allowlist (reviewed, small) and by scoping the scan to backtick command
  tokens only.
- **Risk: EN↔RU divergence during fixes.** Every prose edit is applied to both language mirrors in the
  same change; the plan pairs each fix EN+RU. `make check` (bilingual-sync + CHANGELOG gate) guards the
  commit.
- **Risk: line-number rot.** The ledger anchors on grep-able strings, not line numbers, for the plan.

## Acceptance

- Every ledger finding fixed in both language mirrors where applicable; a full file-by-file pass
  completed with any newly-found drift appended and fixed.
- The guard test is committed, part of `make check`, and green (was red on the pre-fix docs).
- `make check` green; CHANGELOG updated (Unreleased) documenting the audit + the guard.
