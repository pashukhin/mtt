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
   `mtt <status>` sugar / point at `mtt types`/`mtt show`); do **not** hardcode our flow. The Quickstart
   block is already correct (`mtt status t1 in_progress` → `mtt done t1`).
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
- **F1** — `README.md:7` & `README.ru.md:7`: status badge `0.8.98-dev`; build is `v0.10.0-51`.
  *Proof:* `mtt version`. The version is injected at build time (ldflags), so any pinned number in prose
  is stale by construction → replace with a version-agnostic status line.

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
- **F6** — Pitch (`README.md:23`, `:45`): `mtt start` used as if usable; it does not resolve on the
  shipped default template (see decision 5). The same doc's caveat (`:11`) already says the
  `advance`/`start`/`done` meta-walk is parked → contradiction.

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
- **F9** — `DESIGN.md:705` and `:721`: two YAML list items glued onto the previous line
  (`...to: cancelled}  - name: task` / `...  - name: subtask`) in the hierarchy-template example.
  `DESIGN.ru.md` (714/729) is correct → EN regressed against RU. Fix EN to match the correct RU shape.

## Drift-guard test (design)

A single lightweight Go test in the `make check` gate, following the `TestRepoDogfoodConfig` pattern
(discover the repo root, read repo-root doc files). Three focused, low-fragility assertions:

1. **Registry ⊆ CLI_REFERENCE** — every live top-level command (walk the built root command) has a
   ```### `mtt <name>` ``` section heading in `CLI_REFERENCE.md`. Catches a shipped-but-undocumented
   command.
2. **Command tokens ⊆ registry ∪ allowlist** — scan the in-scope prose docs (README/.ru, CLI_REFERENCE/.ru,
   CHANGELOG, FLOW_GUIDE/.ru) for backtick command tokens ```` `mtt <word>` ````; each `<word>` must be
   (a) a registry command, (b) a shipped-`default`-template status used as sugar (`done`/`in_progress`/
   `cancelled`), or (c) in a small **explicit allowlist** of documented-parked/future verbs
   (`advance`/`start`/`cancel`/`caps`/`comment`/`search`/`gantt`/`ui`). Catches phantoms like `mtt guide`.
   When a parked command ships it moves from the allowlist into the registry — the test then guides the
   author to update it (a forcing function, not a maintenance tax).
3. **Version badge** — `README.md`/`README.ru.md` status line carries no hardcoded `\d+\.\d+\.\d+`
   semver. Catches a re-introduced pinned badge.

The exact extraction regexes are pinned during TDD: write the test first, confirm it fails on the
current docs (F1/F6/F8 + any undocumented command), then fix the docs to green. The token-scan
allowlist is the one deliberate, explicit cap — documented in the test so it never reads as "covers
everything".

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
