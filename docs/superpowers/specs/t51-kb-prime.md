# KB prime — curated session-start KB digest (`t51`)

Status: spec (decision record). Type: task (`t51`). Branch: `task/t51`.

## Context / problem

mtt is a "tasks + knowledge" pairing. After t47 (notes) + t1 (refs/backlinks) the KB has real structure,
but **nothing surfaces it at the start of a session** — an agent begins cold, unaware of the durable
knowledge already captured. beads ships `prime`/`remember` for exactly this; a maintainer's expert reviewer
flagged the gap: agents need an **auto-injected KB digest** at session start (wired via a Claude Code
`sessionStart` hook).

The naive version — dump the whole KB — is **wrong**: it is noisy (burns context budget) and **unsafe** (a
note may hold large or sensitive content that would leak into every session). So the feature is really
**curation + ranking + a hard budget**, opt-in by construction. This is newly cheap and meaningful because
the building blocks just landed: **notes** exist (t47), **refs/backlinks** give a relevance signal (t1), and
tasks already have a **`Priority` value object** (s008.6) to reuse.

`t51` adds a small **note-importance axis** and a pure-read **`mtt prime`** command that emits a bounded,
pointer-style markdown digest of the important notes. The `sessionStart` hook itself is **config, not code** —
mtt provides the command; the hook is a documented `settings.json` snippet.

## User stories

Primary user = the coding **agent**; secondary = the human **maintainer**.

- **US1** — As an agent, at session start I receive a compact list of the **important** KB notes (slug, title,
  tags, priority), so I know what durable knowledge exists and can `mtt note show <slug>` the relevant ones.
  `mtt prime`
- **US2** — As a maintainer, I mark a note as important so it (and only it) appears in `prime`.
  `mtt note edit auth-design --priority high`
- **US3** — As an agent/maintainer, I widen or bound the digest. `mtt prime --min-priority medium --limit 30`
- **US4** — As a tool integrator, I wire the digest into session start. A documented `sessionStart` hook runs
  `mtt prime` and injects its stdout (`settings.json`).
- **US5** — (fallout) As a maintainer, I order/filter notes by importance beyond prime.
  `mtt note list --sort priority` / `mtt note list --priority high`

## Decisions

### D1 — Ranking = `Note.Priority` (primary) + backlink-count (tiebreak); opt-in

- **Primary axis: reuse the task `Priority` value object** (`pkg/mtt` `priority.go` — `high`/`medium`/`low`,
  `Rank()` high=0/medium=1/low=2, `Valid()`). The maintainer's "important vs normal" maps onto it (richer:
  three levels). This is the DDD-consistent choice — a **value object**, not a magic tag; zero new vocabulary;
  identical idiom to s008.6.
- **Rejected — a reserved tag** (`pinned`): a magic string is fragile, ungraded, and against the project's
  value-objects-over-primitives ethos.
- **Tiebreak: backlink-count** — reuse t1's computed `core.Backlinks`; a note referenced by more carriers
  ranks higher within a priority band. Emergent relevance, but **secondary** ("important" ≠ "popular");
  recency is the final tiebreak.
- **Opt-in safety (the "dump-all" answer):** a note enters `prime` **only if it has an explicit priority** at
  or above the threshold (default `high`). An **unset** note is **never** primed (D7). Bodies are never
  emitted (pointer-only, D3), and the digest is capped (D4). So nothing leaks unless a human deliberately
  marks it important.

### D2 — `Note.Priority` domain field + serialization (mirror s008.6)

- Add **`Note.Priority Priority`** to `pkg/mtt` `Note` (after `Tags` in the struct is cosmetic; on-disk order
  is the DTO's). Purely additive; **unset = medium in ordering, `omitempty` on disk** — existing note files
  and goldens are **byte-identical**.
- YAML: `ymlNote.Priority` (`yaml:"priority,omitempty"`, after `tags`, before `refs`) — a **plain-copy** map
  (`mtt.Priority(yn.Priority)`), exactly like `ymlTask.Priority` (s008.6). A **corrupt on-disk value is
  tolerated** (round-trips as-is, ranks medium); validity is a **CLI-boundary** concern, not a load error
  (mirrors the task-priority rule). Golden pins the field position.

### D3 — Digest content = pointer-only (no bodies)

- Each entry is a **pointer**: `slug`, `priority`, `tags`, `title`. **No body snippet, no summary field.** The
  agent reads the index and `mtt note show <slug>` for detail — the beads-`prime` model (a compact index, not
  a dump). This is the core of the safety + budget answer: full bodies never enter the injected context.
- **Deferred (follow-up):** an optional bounded snippet (`--full`/first-paragraph) and a dedicated `summary`
  frontmatter field are **out** — speculative for v1 (YAGNI).

### D4 — `mtt prime` command surface + format

- **`mtt prime [--min-priority high|medium|low] [--limit N] [--json]`** — a **pure read** (no usecase
  mutation): `KnowledgeStore.ListNotes` + `TaskStore.List` → `core.NewBacklinks` → `core.Prime` → render.
  (It lists tasks too, because a note's backlinks come from **any** carrier — task or note — per t1.)
- **Defaults:** `--min-priority high` (strict opt-in), `--limit 20`.
- **Human format (markdown, for injection) — pinned:**
  ```
  # Knowledge base — important notes
  - **auth-design**  [high]  (auth, design)  — Auth design
  - **positioning-vs-beads**  [high]  (strategy)  — Positioning vs beads
  (2 of 5 important notes shown — `mtt note show <slug>` for detail)
  ```
  Tags omitted when empty; title omitted when absent (slug is the identity). When **no** notes qualify, print a
  single line (`# Knowledge base — no important notes (mark one: mtt note edit <slug> --priority high)`) — a
  useful, non-empty digest.
- **`--json`:** a **non-null** array of `PrimeEntry` objects (`{slug, title, tags, priority, backlinks}`),
  `tags` a non-null array, for tooling.

### D5 — `core.Prime` (pure derived read; not in the contract)

- **`Prime(notes []mtt.Note, bl Backlinks, opts PrimeOptions) []PrimeEntry`** — pure, no store/clock, **not**
  in the `pkg/mtt` contract (like `Roadmap`/`Backlinks`). `PrimeOptions{MinPriority mtt.Priority, Limit int}`.
- **Selection (D7):** keep a note iff it has an **explicit** (stored, non-empty) priority whose `Rank()` ≤
  `MinPriority.Rank()`. Unset notes are excluded.
- **Order:** by `Priority.Rank()` asc (high first); tiebreak **backlink-count desc** (`len(bl.To(RefNote,
  slug))`); final tiebreak recency (`Created` desc) then slug (deterministic).
- **Cap:** the first `Limit` after ordering (`Limit <= 0` ⇒ no cap). `PrimeEntry{Slug, Title, Tags, Priority,
  Backlinks int}`; the CLI also needs the **total eligible count** (for the "N of M" footer) — return it
  alongside (e.g. `Prime` returns `([]PrimeEntry, total int)` where `total` is the eligible count before the
  cap).

### D6 — note-priority CLI (parity with task priority, DRY)

- **`--priority high|medium|low`** on `mtt note add` (→ `NoteParams.Priority`) and `mtt note edit` (→
  `NoteEditParams.Priority`; `--priority ""` clears — `Changed("priority")` true). Reuse the existing
  `parsePriority` (CLI-boundary validation — an unknown value is a usage error; never leak a bare string into
  core). `mtt note show`/`--json` surface it (`noteJSON.priority`, `omitempty`).
- **`mtt note list [--priority <p>]… [--sort priority]`** — mirror task `list`: `--priority` filters on the
  **stored** label (an unset note matches only when no filter is given), `--sort priority` orders by
  `Rank()` (high first) with the existing recency tiebreak. Extend `core.NoteFilter{Tags, Priorities}` and
  add a `SortNotesPriority` beside `SelectNotes` (mirrors `Select`/`SortPriority`).
- Core note **Adder/Editor** carry the priority through (canonicalize nothing — a scalar VO), exactly as the
  task layer does.

### D7 — threshold semantics: explicit-only, strict opt-in

- `prime`'s `--min-priority` is a **threshold on the stored label**: eligible ⇔ `note.Priority != "" && note.Priority.Rank() <= MinPriority.Rank()`.
  - `--min-priority high` (default) → only explicit `high` notes.
  - `--min-priority medium` → explicit `high` + `medium` (still **not** unset).
  - `--min-priority low` → all explicitly-prioritized notes.
  - **Unset notes are never in `prime`** (regardless of `--min-priority`) — this is deliberate (the safety
    model): you opt a note in by giving it a priority. It differs from `note list --priority`, which is exact-
    match on the stored label; both consult the stored label, never the ordering default.

### D8 — the `sessionStart` hook is config, not code

- mtt ships only the **`mtt prime`** command. The hook is a documented `settings.json` snippet (a `sessionStart`
  hook that runs `mtt prime` — and optionally `mtt roadmap` — and injects stdout). Documented in
  `CLI_REFERENCE` (and a natural fit for t46's "how-to-use-mtt agent docs"); **no hook code in the binary.**

### D9 — KB-only (no roadmap bundle)

- `prime` is **knowledge**; `mtt roadmap` already answers "what to do". Keep them separate (SRP) — the hook
  composes both if the integrator wants. `prime` does **not** print tasks (it only *reads* them for the
  backlink signal).

## Scope

**In:** `Note.Priority` (domain + frontmatter, mirror s008.6); `--priority` on `note add`/`note edit` +
`note list --priority`/`--sort priority` (+ `noteJSON.priority`, `note show`); `core.Prime` + note
priority in `NoteParams`/`NoteEditParams`/`NoteFilter` + `SortNotesPriority`; the `mtt prime` command
(markdown + `--json`); unit + golden + e2e tests; docs sync (incl. the hook snippet).

**Out:** body snippets / a `summary` field in the digest (follow-up); the `sessionStart` hook **as code**
(config/docs only); a roadmap bundle in `prime`; a `--tag` scope on `prime` (YAGNI v1); note versioning/search
(t6).

## Acceptance criteria

1. **Note priority round-trips:** `note add spec --priority high` → `note show spec` shows `priority: high`;
   the frontmatter has `priority: high` (after `tags`, before `refs`); a note **without** a priority is
   byte-identical to the pre-t51 golden (`omitempty`). `note edit --priority ""` clears it.
2. **`mtt prime` default:** with notes at mixed priorities (some `high`, some `medium`, some unset), `mtt prime`
   lists **only** the `high` notes, ordered (backlink-count desc within the band), pointer-style, with the
   "N of M" footer; unset notes never appear. `--min-priority medium` additionally includes `medium` (still not
   unset). `--limit 1` caps to one and the footer reflects the total.
3. **Backlink tiebreak:** two `high` notes, one referenced by more carriers (task or note), sort the
   more-referenced first.
4. **Empty digest:** with no eligible notes, `mtt prime` prints the single "no important notes" line (exit 0),
   not an empty output.
5. **`--json`:** `mtt prime --json` emits a **non-null** array of `{slug, title, tags, priority, backlinks}`
   (`tags` non-null); `[]`-shaped when empty.
6. **`note list`:** `--priority high` filters to stored-high notes; `--sort priority` orders high→low with the
   recency tiebreak. An invalid `--priority` value is a usage error (exit 1) at the CLI boundary.
7. **Safety:** a note with a body but **no** priority never appears in any `mtt prime` output; bodies are never
   emitted by `prime` (pointer-only).
8. `make check` green. Docs synced (below), including the `sessionStart` hook snippet.

## Testing approach

- **Unit (`pkg/mtt`):** `Note.Priority` is additive; `Priority.Valid()`/`Rank()` already covered — no VO change.
- **Unit (`internal/core`):** `Prime` — threshold (high default excludes medium+unset; medium includes both
  explicit bands, still excludes unset; low includes all explicit); order (priority band, then backlink-count
  desc, then recency); cap + the returned `total`; empty input. `SelectNotes` priority filter (stored-label
  match, unset only when unfiltered) + `SortNotesPriority`. Note `Adder`/`Editor` carry `Priority` (edit clear).
- **Golden / round-trip (`internal/adapter/yaml`):** a note with `priority` (field position); a note without
  (byte-identical to the existing golden); corrupt priority tolerated (loads, ranks medium).
- **e2e (`internal/cli`, testscript `prime.txt`):** AC-2…AC-5 + AC-7 flows; `note --priority` add/edit/clear +
  `note list --priority`/`--sort priority` (AC-6); the markdown format + `--json` shape; the empty-digest line.

## Docs to sync (docs-sync judgment, `impl_review`)

Grep **all** parallel occurrences (EN + RU) before editing.

- **`CLI_REFERENCE.md ↔ .ru.md`:** a **`mtt prime`** section (surface, defaults, format, `--json`, the opt-in
  threshold semantics); `--priority` on `note add`/`note edit` + `note list --priority`/`--sort priority`; a
  **`sessionStart` hook snippet** (how to wire `mtt prime` into session start).
- **`DESIGN.md ↔ .ru.md`:** a "**Shipped (t51)**" note under the KB section (note-importance axis + `mtt prime`
  as a curated, opt-in, pointer-only digest; ranking = `Note.Priority` + backlink tiebreak; the hook is
  config). Reaffirm KB stays a **supporting** feature (positioning).
- **`docs/architecture/model.go`:** add `Priority` to the `Note` block; note `Prime` is core-derived (not
  contract), like `Roadmap`.
- **CLAUDE.md files:** `pkg/mtt` (`Note.Priority`), `internal/core` (`Prime`, note priority in filter/sort/
  usecases), `internal/cli` (`mtt prime`, note `--priority`/list), `internal/adapter/yaml` (note frontmatter
  `priority`). Keep each thin.

## Sequencing & tracking (process, not code)

`t51` is already `speccing` on `task/t51` (its file rode onto the branch — main stays clean). This document is
the `speccing` deliverable. Next: commit it, run an adversarial subagent **spec review**, address findings,
then `spec_human_review` → `planning`. Builds on t47 + t1 (both terminal); no open deps.
