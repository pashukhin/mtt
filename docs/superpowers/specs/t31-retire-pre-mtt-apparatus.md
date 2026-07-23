# t31 — Retire the pre-mtt session apparatus (spec)

Status: draft for adversarial spec review.
Decided in the 2026-07-23 brainstorm (investigation + 3 scoping decisions with the user).

## Problem

The s009 self-host migration froze TASKS.md but only half-retired the session apparatus. Evidence
(2026-07-23 investigation):

- **NEXT_SESSION.md is a live bypass channel.** Its header still says "a living handoff doc — update
  it at the end of each session"; a post-freeze commit (`15ca7e5`) actualized it — duplicating
  `mtt roadmap` state into markdown — and it is already stale again ("Next task: t1" while t1 + ~15
  more tasks are delivered; "v0.9.0 still UNTAGGED" while v0.10.0 is out). Its "Ready-to-paste
  kickoff prompt" teaches the retired lifecycle: `feat/…` branches, "tick TASKS.md", "fill
  sessions/*.md", per-session version bumps (all superseded by flow v2 + t30 semver).
- **Frozen TASKS.md is still cited as live.** DESIGN.md (8 spots) and CLI_REFERENCE.md (2 spots),
  plus RU mirrors, say "see TASKS.md → Later" for items that actually live in mtt (t10, t36, …).
- **Lifecycle bypass precedent.** t43 was "resolved … done directly" (`968fdf5`): work landed as a
  direct main commit, the task was `rm`'d — no flow move, no history, record erased.
- **The KB is empty** despite t47 (notes) + t51 (prime) shipping: knowledge keeps accruing in
  NEXT_SESSION carry-over blocks and `docs/superpowers/notes/*.md`, so `mtt prime` injects nothing.

Root cause: the retirement decision was deferred (this task, ex-"Think"), and meanwhile the leftover
docs actively teach the old process. Per AGENTS.md TL;DR #0 every such manual convention is a bug.

## Goals (this task)

1. **KB seed** — distill durable knowledge into 10–12 curated notes; make `mtt prime` useful.
2. **Mechanize** — clean-tree gate on all submit edges; CHANGELOG gate on impl submits (resolves
   t54); KB reminder on deliver.
3. **Purge** — delete NEXT_SESSION.md, TASKS.md, `sessions/`, delivered-task artifacts; rewire or
   drop every pointer to them.
4. **Rules** — codify mtt-first working rules in AGENTS.md + root CLAUDE.md.

## Non-goals

- DESIGN.md unload (Shipped-block history → KB) — split out as **t63** (depends on t31).
- The rest of the user-docs audit (**t42**): README feature claims, versions, CHANGELOG 0.9.0
  claims. Only the TASKS.md-pointer rewiring is pulled forward here.
- Product-code changes: t26 (auto-commit non-flow mutations), t52 (session-start hooks) stay queued.
  This task changes **config, docs, notes, and the guard test only** — no production Go code.
- Wholesale copying of specs/plans into the KB (the t53 trap: curation, not migration).

## D1 — KB seed (curated notes, no DESIGN duplication)

Create via `mtt note add <slug> --title … --priority … --tag …` (+ `--ref` where a task is the
natural anchor). Content is **distilled** from NEXT_SESSION carry-over blocks and
`docs/superpowers/notes/*`; nothing that already lives in DESIGN.md/AGENTS.md is copied — notes may
point at DESIGN sections instead.

| slug | priority | distilled from |
|---|---|---|
| `process-model` | high | s009 lesson: product-vs-process axis; tasks = product change; the 15-status flow = a task's maturation; sessions are not tracked items |
| `tag-conventions` | high | AGENTS.md interim block (it says itself it "migrates into mtt later"): backlog/think semantics, thematic vocabulary, hashtag caveat; AGENTS keeps a one-line pointer |
| `adversarial-reviews-pay` | high | the recurring cross-session lesson + 3–4 concrete catches (fail-open gate, YAML quoting trap, Write-over-existing-test, cobra Args ordering) |
| `working-under-flow-traps` | high | commit non-.mtt work before `submit` (now also gated); mid-flight backlog adds on local main → reconcile before deliver (reset + cherry-pick); gates inherit caller env (MTT_DIR leak); exit-5 recovery |
| `positioning-vs-beads` | high | 2026-07-09 positioning/agent-UX analysis: wedge = per-type flow + zero footprint + adaptivity; deps stay simple; accepted ID-collision tradeoff; niche window |
| `release-and-launch` | medium | t30 semver (git-describe stamp), batched release cadence (RELEASING.md pointer), distilled launch-plan essentials feeding t60 |
| `testscript-e2e-conventions` | medium | anchored asserts, no pipes (cp stdout → stdin), wall-clock-tie robustness, txtar gated configs, git-in-testscript needs born branch + identity, output-only needles |
| `go-cli-conventions` | medium | cobra validates Args before RunE; stdout via OutOrStdout; SilenceErrors buffer trap; `unused` linter (declare where first used); exit-code taxonomy via `%w`; bulk aggregate = plain `fmt.Errorf` |
| `architecture-heuristics` | medium | port-vs-field test (embeddable → field + Update; non-embeddable → capability port); VO idioms (closed vocab = VO, open transforming vocab = plain slice + pure funcs); domain-vs-policy (authored-on-edge vs runner default); derived graphs live in core; pure read needs no usecase |
| `flow-authoring-lessons` | medium | descriptions are load-bearing → guard-test them; single-quote gate scalars (YAML `!`/`"` traps); commands run pre-write; fail-closed gate shape (`out=$(…) && test -z "$out"`); isolate one violation per invariant fixture |
| `git-github-traps` | medium | squash subject comes from the commit on single-commit PRs (repo needs PR_TITLE); branch protection on main would break deliver/cancel push (t33); `git switch` from unborn HEAD exits 128 |
| `dogfood-history` | low | archaeology pointer: the bootstrap arc (s001→s009), what was retired in t31 and where it lives (git history); replaces sessions/README as the orientation breadcrumb |

Acceptance: `mtt prime` prints a non-empty digest led by the high notes; `mtt note list` shows the
set; no note restates a DESIGN/AGENTS section (pointers allowed).

## D2 — flow-config mechanization (.mtt/config.yaml; SEC2: config is code)

All edits keep the single-quoted-scalar rule. `TestRepoDogfoodConfig` (exact-string assertions in
`internal/adapter/yaml/dogfood_test.go`) is updated **first** (red), then the config (green) — TDD
on config.

**(a) Clean-tree gate on every submit edge** — 8 edges: task `speccing→spec_review`,
`spec_fix→spec_review`, `planning→plan_review`, `plan_fix→plan_review`, `implementing→impl_review`,
`impl_fix→impl_review`; chore `implementing→impl_review`, `impl_fix→impl_review`. Ordering is
cheap-first:

- spec/plan submits: `ls docs/superpowers/{specs|plans}/{{.ID}}-*.md` (existing), then the new gate.
- impl submits: the new gate **before** `make check` (fail fast; also `make check` should run on a
  fully committed tree).

Gate command (identical on all 8):

    'out=$(git status --porcelain) && test -z "$out" || { echo "working tree not clean - commit your work (code and docs travel with the branch; only .mtt is auto-committed)" >&2; false; }'

Fail-closed shape (the s009 lesson); `.mtt/config.local.yaml` is gitignored since c11 and never
trips it. Gates run pre-write, so the pending status change itself is not in the tree yet.

**(b) CHANGELOG gate on the 4 impl submits** (task + chore `implementing→impl_review`,
`impl_fix→impl_review`), after the clean-tree gate, before `make check`:

    'git diff --quiet main...HEAD -- cmd internal pkg go.mod go.sum || ! git diff --quiet main...HEAD -- CHANGELOG.md || { echo "code changed but CHANGELOG.md has no entry - add one under [Unreleased] (pure refactor? bypass: --no-run --who --why)" >&2; false; }'

Semantics: pass when no code changed vs the merge base, or when CHANGELOG.md changed too. Known
false positive: a pure refactor — the documented, audited bypass is `--no-run --who --why` (exits 2
without both). Resolves **t54** (cancel it after delivery with `--why "mechanized as a gate in t31"`).
Merge-base form (`main...HEAD`) keeps an advanced local main from polluting the diff.

**(c) Deliver reminder** — both types' `approved→done` deliver edges get their description extended:
`"…writes done there); before delivering, capture this task's durable lessons: mtt note add"`.
Description-only (no objective gate exists for "knowledge captured").

Self-test note: t31's own `submit` from implementing will run the new gates read live from the
working tree — the diff touches `internal/` (dogfood_test.go), so CHANGELOG.md gets a real
`[Unreleased]` entry (gates are user-visible behavior), satisfying (b) honestly.

## D3 — purge + pointer rewiring

**Deleted outright** (git history is the archive; no tombstone files):

- `NEXT_SESSION.md` (lessons distilled per D1 first; superpowers activation moves per D4)
- `TASKS.md`
- `sessions/` (entire directory, ~30 files incl. README and template)
- `docs/superpowers/specs/*`, `docs/superpowers/plans/*`, `docs/superpowers/pr/*` for tasks now
  terminal — i.e. every existing file except t31's own spec/plan/pr (all current id-keyed files are
  for delivered tasks; the 2026-07-* session-named files are all pre-flow history)
- `docs/superpowers/notes/*` — after D1 distillation (verify the debt/security triage items all
  exist as tasks before deleting that one; file anything missing as backlog first)

`docs/architecture/model.go` stays (architecture snapshot, not process apparatus).

**Pointer rewiring** (EN + RU mirrors in the same pass; per the parallel-occurrences lesson, sweep
with `git grep -n 'TASKS\.md\|NEXT_SESSION\|sessions/'` at the end — the lists below are the known
sites, not the definition of done):

- Live deferred items get their real mtt id: DESIGN.md:618/:627/:746 → t10; :756 → t36;
  CLI_REFERENCE.md:317 → t10; DESIGN.md:532/:556 → map to the matching open task at implementation
  time (read context, check `mtt list`; if none exists, point at `mtt roadmap` generically).
- Historical "see sessions/NNN and TASKS.md → …" citations: drop the pointer clause (plain-text
  history mentions may stay; **no markdown links to deleted files anywhere**).
- CLI_REFERENCE.md:769-770: delete the stale "(TASKS.md still mentions close…)" parenthetical.
- README.md:134 (+ru): the TASKS.md bullet becomes the dogfood line — the live backlog is this
  repo's own mtt (`mtt roadmap`).
- CHANGELOG.md is history — leave untouched.

## D4 — rules (AGENTS.md + root CLAUDE.md)

AGENTS.md changes:

- **"Working under mtt" gains the closure rules:** a task leaves the queue only over a flow edge —
  `deliver` (after the squash-merge) or `cancel --why`; `mtt rm` is not closure (it erases the
  record; it exists for mistakes/duplicates); "done directly" is forbidden — work whose design is
  already fixed becomes a `chore` and rides the chore flow.
- **Knowledge rule:** durable lessons/decisions go to `mtt note add` (the KB feeds `mtt prime`);
  markdown files are not a task-state or knowledge channel; the only "what's next" source is
  `mtt roadmap`. No parallel state docs.
- **"Sessions → tasks" section rewritten:** the unit of work is the mtt task; the sessions dir is
  retired (history in git); narrative archaeology = `dogfood-history` note.
- **"Documentation language":** drop TASKS.md/NEXT_SESSION.md from the agent-docs list.
- **Tag conventions block** shrinks to a pointer at the `tag-conventions` note.

Root CLAUDE.md changes: drop the TASKS.md mention ("frozen history" note no longer needed); the
"Skills / guards" section absorbs the 3-line superpowers activation instruction (marketplace add +
plugin install + verify) that lived in NEXT_SESSION.md; "Read at the start of a session" becomes
AGENTS.md → DESIGN.md → `mtt roadmap` + `mtt prime`.

## Acceptance

1. `make check` green; `TestRepoDogfoodConfig` asserts the new exact gate/description strings.
2. `mtt prime` prints the curated digest (highs first); `mtt note list` matches the D1 set.
3. The D3 files are gone; `git grep -l 'NEXT_SESSION\|TASKS\.md'` over the tree returns nothing
   beyond CHANGELOG.md (history), `.mtt/` records (task data is history too), and t31's own
   spec/plan artifacts — and even there only plain-text mentions: zero markdown links to deleted
   files anywhere; no `sessions/` path references outside the same exclusions.
4. A dirty tree blocks `mtt submit` (observed live on t31 itself); the CHANGELOG gate passes on
   t31's own submit via a real changelog entry.
5. AGENTS.md/CLAUDE.md carry the D4 rules; EN/RU mirrors consistent where touched.
6. Post-delivery (outside this PR): `cancel t53 --why "resolved by t31"`, `cancel t54 --why
   "mechanized as a gate in t31"`.

## Risks / notes

- **Config guard is the sole load-time validation** — exact-string asserts in the guard test are
  what protect the new gates from silent YAML mangling; update them with the config in one commit.
- **Clean-tree gate strictness:** untracked scratch files block submits by design; the error text
  names the fix. `-v`/`--log-file` unaffected.
- **CHANGELOG gate scope** is the Go surface (`cmd internal pkg go.mod go.sum`): docs/config-only
  chores pass untouched; Makefile/CI changes are deliberately out (rare, and the bypass is audited).
- **RU mirrors:** only sections this task touches are re-synced; the broader EN-RU drift stays t42.
- **`mtt roadmap` ordering oddity** (unset-priority t58 listed above high t31) is pre-existing
  behavior observed during grooming — explicitly out of scope; investigate separately if it
  reproduces.
