# references — structured, verifiable refs on tasks & notes (`t1`)

Status: spec (decision record). Type: task (`t1`). Branch: `task/t1`.

## Context / problem

`mtt` is a "tasks + knowledge" pairing. After `t47` **both** halves exist as real stores (`TaskStore` +
`KnowledgeStore`), but they are **not connected**: a note is an island, a task cannot point at it, and there
is no way to ask "what references this design?". The reference vocabulary has been reserved in the domain
since before `t47` — `pkg/mtt/task.go` carries `Ref{Kind,ID,Label}`, `RefKind` (`note`/`task`/`comment`/`url`
+ `Valid()`), and reserved `Task.Refs` / `Comment.Refs` — and the YAML task adapter **already round-trips**
`Task.Refs`/`Comment.Refs` (`ymlRef`, `fromDomainRefs`/`toDomainRefs`). What is missing is the **wiring and
verification**: a CLI to author refs, capability-aware resolution, backlinks, a repo-wide integrity sweep, and
a deletion guard so a delete never silently strands a reference.

`t1` is that wiring. It is **not** the introduction of the reference types (those exist); it is the
usecases + verification + CLI + integrity around them. Following the `t47` philosophy ("don't ship a kind
whose target doesn't exist"), `t1` ships the three kinds whose targets exist **now** — `note`, `task`, `url`
— and defers `comment` (both as a target and as a carrier) to `t2`, where comments become real.

Why refs before comments (`t2`): refs are the **connective tissue** that turns two stores into the promised
*pairing* (link a task to its design note, link related/duplicate tasks); they unblock **recategorization**
(DESIGN: re-typing = close old + create new + link via `refs` kind `task`), which is otherwise impossible;
and they serve the **agent-primary** audience (agents link and verify; threaded discussion is a
human-collaboration feature). `comment` is also the **rarest** ref target, so completing the 4th kind is not
worth reordering two features. Recorded decision (maintainer): keep `t1` first.

## User stories

Primary user = the coding **agent**; secondary = the human **maintainer**.

Closed by this task:

- **US1** — As an agent, link a task to a durable note / another task / a URL so the relationship is explicit
  and integrity-checked. `mtt ref add t5 note:auth-design --label spec`
- **US2** — As an agent, see a task's outgoing refs **and** its incoming backlinks ("what references this").
  `mtt ref list t5` / `mtt show t5`
- **US3** — As an agent, link **notes** too (a note referencing a task/other note/URL).
  `mtt note ref add auth-design task:t5`
- **US4** — As an agent, attach refs at creation time. `mtt add '…' --ref task:t2 --ref url:https://…`
- **US5** — As a maintainer, sweep the whole repo for broken links in one command, usable as a gate/CI check.
  `mtt check` (exit `7` if any dangling).
- **US6** — As an agent, be **prevented** from deleting a referenced task/note (so I never silently strand a
  link), with an explicit signed override. `mtt rm t5` (refused) → `mtt rm t5 --force --who … --why …`.
- **US7** — (recategorization enabler) link the old and new task on a type change. `mtt ref add tNew task:tOld`.

Deferred to `t2` (comments) — explicitly out:

- Referencing a **comment** (`comment:t5#3`) and carrying refs **on** a comment (`mtt comment add … --ref …`).
  The domain seam is already reserved (`Comment.Refs` exists, `RefComment` constant stays); `t1` only rejects
  `comment:` at the CLI boundary with a clear "comments arrive in t2" message.

## Decisions

### D1 — Scope: kinds `note`/`task`/`url`; carriers `task`/`note`; `comment` → `t2`

- **Live ref kinds:** `note`, `task`, `url`. `comment` is **rejected at the CLI input boundary** (exit 1,
  "comments arrive in t2") — its target does not exist yet, exactly as `note` was a dead kind before `t47`.
  The `RefComment` constant and `RefKind.Valid()` are **unchanged** (the domain vocabulary stays complete);
  only the *input acceptance* excludes it.
- **Carriers of refs in `t1`:** tasks and notes. `Task.Refs`/`Comment.Refs` already exist; **`Note.Refs` is
  added** (D2). Comment-as-carrier → `t2`.
- **Verification dispatch is table-driven / per-kind** so `t2` adds `comment` as **one** resolver branch plus
  lifting the input rejection — no three-places edit. This open-vocabulary seam is a hard requirement of the
  design (not an incidental shape).

### D2 — `Note.Refs` domain field + serialization (additive)

- Add **`Note.Refs []Ref`** to `pkg/mtt` `Note` — additive; the domain struct field position is cosmetic (no
  serialization tags in the domain). Existing notes round-trip unchanged (`omitempty`); on-disk order is the
  `ymlNote` DTO's, pinned below.
- YAML adapter: `ymlNote` gains **`Refs []ymlRef `yaml:"refs,omitempty"`**, mapped via the **existing**
  `fromDomainRefs`/`toDomainRefs` (same package — DRY, no second ref DTO). Frontmatter field order is fixed by
  the struct; `refs` sits in the frontmatter block (the body stays verbatim after the closing `---`). Golden:
  a note with refs, a note without (byte-identical to today), round-trip.
- **Frontmatter placement:** `refs` goes **after** `tags` and **before** `created`/`updated` in `ymlNote`
  (keep the always-present timestamps last so the "first closing `---`" read rule and existing goldens for
  timestamp position are least disturbed). `pkg/mtt/CLAUDE.md` KB-port line updated (`Note` is no longer
  "refs-free").

### D3 — Reference identity = the natural key `(kind, target)`; canonical order

- A reference has **no separate id**; its identity is the pair **`(kind, target)`** (`target` = `Ref.ID`,
  a plain string per `Kind`). An entity may hold many refs of the same kind to different targets; only an
  exact `(kind, target)` duplicate collapses.
- **`--label` is annotation, not identity.** `ref add` **upserts** by key: a new key is appended; an existing
  key has its label **overwritten when `--label` is given** and **left unchanged when `--label` is omitted**
  (idempotent re-add). A brand-new ref with no `--label` stores an empty label.
- Stored refs are kept in a **canonical order — sorted by `(kind, target)`** (mirrors the sorted-set
  discipline of `Tags`) for clean, machine-independent diffs. (Task refs today are stored in author order;
  `t1` makes ref writes canonicalize — a one-time, deterministic normalization applied whenever the ref set is
  mutated.)

### D4 — CLI surface: mirror the note namespace (variant A) + creation-time `--ref`

Two symmetric sub-command groups (the `mtt dep` / `mtt note` pattern), plus a creation flag (the
`add --depends-on` pattern):

```
mtt ref add  <task-id> <kind>:<target> [--label <text>] [--json]
mtt ref rm   <task-id> <kind>:<target>
mtt ref list <task-id> [--json]                       # outgoing refs + backlinks
mtt note ref add <slug> <kind>:<target> [--label <t>] [--json]
mtt note ref rm  <slug> <kind>:<target>
mtt note ref list <slug> [--json]
mtt add       … --ref <kind>:<target> …               # repeatable, creation-time
mtt note add  … --ref <kind>:<target> …               # repeatable, creation-time
```

- **Rejected — variant B (unified `mtt ref <entity>` with task/note auto-detect):** a note slug like `t1` is
  valid kebab-ASCII and can collide with a task id `t1`; auto-detection is ambiguous and against the "explicit
  over clever" ethos. Notes keep their own namespace (`mtt note …`), as everywhere else.
- **Target syntax `kind:target`:** split on the **first** `:` (`strings.Cut`) — a URL contains `:` and `/`.
  Missing `:` → exit 1 ("expected `<kind>:<target>`"). `kind` ∉ {note,task,url} → exit 1; `kind == comment`
  → exit 1 with the "comments arrive in t2" message.
- **Per-kind target validation at the CLI boundary** (each kind validated by its own identity rule, before
  the store is touched): `task` → `mtt.NewTaskID` (non-empty); `note` → `mtt.NewNoteSlug` (kebab-ASCII — the
  same traversal guard); `url` → `net/url` parse requiring a **scheme and host** (rejects bare `example.com`;
  scheme is not restricted). A malformed target of any kind is a usage error (exit 1). This validation is the
  **only** hard failure on the write path (D6).
- The two groups share **one pure ref-set algebra** (upsert/remove/dedup/sort on `[]Ref`) and one rendering
  path; only the carrier store (`TaskStore.Update` vs `KnowledgeStore.UpdateNote`) differs (DRY).

### D5 — Verification is capability-aware; `RefStatus` is computed (not in the contract)

- Resolution status of a ref is a **derived** value `RefStatus ∈ {ok, dangling, unverified}` computed in
  `internal/core` — **not** part of the `pkg/mtt` contract (like `Ready`/`Roadmap` results, and like the
  resolved graph). Nothing about a ref's status is stored on disk.
- Capability-aware resolution:
  - `task` → resolve via `TaskStore` (always available). Missing target ⇒ **dangling**.
  - `note` → resolve via `KnowledgeStore` **if wired**; missing target ⇒ **dangling**; **no KB wired** ⇒
    **unverified** ("cannot verify: no KB").
  - `url` → external, **not resolved** ⇒ always **unverified** (optional HEAD check is a later follow-up; no
    network in tests — AGENTS rule).
  - `comment` → not reachable in `t1` (rejected at input; no such ref can exist).
- For single-op verification (`ref add`) resolution is a direct `Get`/`GetNote`; for the sweep (`check`) and
  backlinks, existence is read from the in-memory snapshot (D7) to avoid N store round-trips.

### D6 — Write policy: warn-not-block

- The **carrier must exist**: `ref add t99 …` / `note ref add missing …` on a non-existent carrier → exit 4
  (`ErrNotFound`, via `taskNotFound`/`noteNotFound`).
- The **target may dangle**: a `task`/`note` target that does not resolve, or a `note` with no KB, is **still
  stored**, with a **warning to stderr** (`warning: task:t9 is dangling (no such task)` /
  `unverified (no KB)`) and **exit 0** — informational links are never a hard block (DESIGN: "on write — warn
  about a dangling reference, not a hard block").
- The **only** hard failure on the write path is a **malformed input** (bad `kind:target`, `comment:`,
  malformed URL / empty id / non-kebab slug) → exit 1.
- **Self-reference** (`ref add t5 task:t5`) is **allowed** — refs are informational and have no cycle concept
  (unlike `depends_on`, which rejects self- and cycle-edges). A `note ref` to its own slug is likewise allowed.

### D7 — Backlinks are a pure, computed value (never stored); it powers everything

- A single derived structure — call it **`Backlinks`** — is built in `internal/core` from a **snapshot of all
  tasks + all notes** (`TaskStore.List` + `KnowledgeStore.ListNotes`), exactly as `Index`/`DepGraph` are pure
  values built from a `List` snapshot. It maps a target key `(kind, target)` → the referents pointing at it,
  each `{carrier-kind (task|note), carrier-id, label}`, in a deterministic order.
- **Not part of the `pkg/mtt` contract; never stored on disk.** This honors the standing invariant "back
  references are **computed**, never stored — forward refs are the single source of truth" (AGENTS/DESIGN).
  Materialized/bidirectional backlinks are **explicitly rejected** for `t1` (they would make every `ref
  add/rm` write two files that can desync — the exact bug class the invariant avoids); a separate refs
  table / embedded DB is likewise **out** (DESIGN: files are the source of truth; an index is derived,
  in-memory, rebuilt per call — our scale fits memory).
- The **same** `Backlinks` value serves: `show`/`note show` (the Backlinks section), `ref list`/`note ref
  list`, the `check` sweep, **and** the deletion guard (D9) — built once per invocation, reused (DRY).
- Backlinks are shown for `task` and `note` targets (ownable entities); a `url` has no entity page.

### D8 — `mtt check`: read-only cross-store integrity sweep

- Sweeps **all** carriers (task `Refs` + note `Refs`; comment refs are `t2`, none exist), resolves each ref
  (D5), and reports the **dangling** ones grouped by carrier, plus a count of **unverified**.
- **Exit code:** `0` when clean **or only `unverified`** (url / no-KB are not failures); **`7`** when any
  **dangling** ref exists. `7` is a **new, dedicated** taxonomy code ("integrity: dangling references") — kept
  distinct from `1` (usage) so CI / a flow gate can branch on "found broken links" unambiguously. mtt's whole
  premise is executable gates, and `mtt check` is meant to be gate-usable.
- Output: human lines `t3 → note:missing-slug   [dangling]` + a summary (`N dangling, M unverified across K
  entities`); `--json` emits a structured array of the dangling (and, for completeness, unverified) entries.
- **`--fix` is deferred** to a follow-up: an interactive fix hangs an agent (anti-pattern), and a
  non-interactive mass mutation of carriers raises its own attribution/audit questions. `t1` ships the
  read-only sweep — the core integrity value.

### D9 — Deletion guard (refuse-by-default + `--force`), unified & cross-store

Chosen over warn-and-proceed because deletes are **irreversible** (no undo): `rm` must **prevent** creating
dangling links, not merely warn about them (`check` catches them post-hoc, but the target file is already
gone). Recorded decision (maintainer).

- **The only operations that can orphan a ref are `mtt rm <task>` and `mtt note rm <slug>`** (enumerated:
  `edit` touches title/description only; re-parent changes `parent`, not the id; `cancel` is a *status* — the
  file lives on; a note slug is immutable). Both get the **same** guard.
- **Guard = a query over refs, answered by the computed `Backlinks` (D7)** — **not** a per-store scan and
  **not** a new `KnowledgeStore` port on the remover. Both removers consume the pure `Backlinks` value
  assembled at the composition root from the task **and** note snapshots. This makes the guard **cross-store
  for free**: `mtt rm t5` refuses on an incoming `task:t5` ref from a **note** carrier as well as from a task
  carrier — which a task-only scan (`Index`+`DepGraph`) would miss.
- **`mtt rm <task>`:** the existing `core.Remover` refuse-set (children via `Index`, dependents via
  `DepGraph`) is **extended** with **ref-referents** (`Backlinks[{task, id}]`). Refuse-by-default; `--force`
  overrides (unchanged: forces `--who`/`--why` pre-flight, writes an audit record before deleting, leaves the
  refs dangling — tolerated). **Subgraph-ignore preserved:** in a bulk `mtt rm {t3,t5}`, a ref between the two
  deleted tasks does not block (referents inside the deletion set are excluded, as today).
- **`mtt note rm <slug>`:** currently unconditional. A new **`core.NoteRemover`** (KnowledgeStore + AuditStore
  + injected `now`) mirrors `Remover`: refuse-by-default when `Backlinks[{note, slug}]` is non-empty; add a
  **`--force`** flag that overrides and — for a uniform "no destructive override without a signed trail" —
  **forces `--who`/`--why`** and writes an audit record (`action: "note rm --force"`) **before** deleting.
  Missing slug → exit 4 (`noteNotFound`, unchanged).
- **`core.Remover` gains no store port** — it receives the `Backlinks` (or the note-refs snapshot needed to
  build it) as data, staying store-agnostic exactly like its current `Index`/`DepGraph` inputs. For an
  external adapter without a KB, the note contribution is simply empty (capability-aware).

### D10 — Exit-code taxonomy (reuse + one new code)

| code | when |
|---|---|
| `1` | malformed `kind:target`, unknown kind, `comment:` kind, malformed URL / id / slug (usage) |
| `2` | missing `--who`/`--why` under `rm --force` **or** `note rm --force` (existing dangerous-ops policy) |
| `4` | carrier not found (`ref add/list/rm` on a missing task/note); `ref rm` with **no such `(kind,target)`** |
| `7` | **new:** `mtt check` found ≥1 **dangling** reference |

### D11 — `--json` shapes (pinned)

- A **ref object**: `{kind, id, label, status}` (`status` ∈ `ok`/`dangling`/`unverified`; `label` may be `""`).
- `ref add`/`note ref add --json`: the single upserted ref object.
- `ref list`/`note ref list --json`: `{refs: [ref…], backlinks: [{kind, id, label}…]}` — both **non-null**
  arrays (`[]` when empty, the house rule). Backlink entries carry the **carrier** kind+id (who points here).
- `check --json`: a **non-null** array of `{carrier: {kind, id}, ref: {kind, id, label}, status}` for each
  dangling (and unverified) entry.
- `show`/`note show` gain **Refs** and **Backlinks** sections (human view) and, in `--json`, `refs`/`backlinks`
  arrays consistent with the above (non-null).

## Scope

**In:** `Note.Refs` (domain + frontmatter, reusing `ymlRef`); the pure ref-set algebra (upsert/remove/dedup/
canonical-sort); capability-aware verification + `RefStatus` (core); the computed cross-store `Backlinks`
value; `mtt ref` and `mtt note ref` groups (add/rm/list); creation-time `--ref` on `add`/`note add`;
`mtt check` (read-only sweep, exit 7); Refs + Backlinks in `show`/`note show`; the unified cross-store
deletion guard (`Remover` extended + new `NoteRemover` with `--force`/who/why/audit); unit + golden + e2e
tests; docs sync.

**Out:**
- **`comment` kind** (as target **and** carrier), `mtt comment add --ref` → **`t2`**.
- **`mtt check --fix`** → follow-up (interactive hangs agents; mass mutation needs its own attribution/audit
  design).
- **URL liveness (HEAD) resolution** → follow-up; `url` stays `unverified`.
- **Materialized/bidirectional backlinks** and a **separate refs table / embedded DB** → rejected for `t1`
  (violate the "back-refs computed, forward refs single-source-of-truth" invariant / the files-are-truth
  decision); a deliberate DESIGN change if ever, not a `t1` smuggle-in.
- **Roles/attribution changes** beyond reusing the existing dangerous-ops policy.

## Acceptance criteria

1. **Task refs loop (e2e):** `mtt ref add t2 task:t1 --label blocks` → stored, `ok`; `mtt ref list t2` shows
   the ref (with `ok`) and `mtt ref list t1` shows the **backlink** from t2; `mtt ref rm t2 task:t1` removes
   it; a second `rm` of the same key → exit 4.
2. **Note refs loop (e2e):** `mtt note ref add auth-design task:t2` round-trips through
   `.mtt/knowledge/auth-design.md` frontmatter (`refs:` block), and `mtt show t2` lists the incoming backlink
   from the **note**. `mtt note ref list auth-design` shows outgoing + backlinks.
3. **Kinds & validation:** `note`/`task`/`url` accepted; `comment:x` → exit 1 ("comments arrive in t2");
   unknown kind / missing `:` / malformed URL (`url:example.com`, no scheme) / non-kebab slug / empty id →
   exit 1. A well-formed `url:https://a/b:c?x=1` parses (split on first `:`) and stores as `unverified`.
4. **Write policy (warn-not-block):** `mtt ref add t2 task:t999` (missing target) → stored, **warning**,
   **exit 0**, status `dangling`; `mtt ref add t2 note:x` with no KB target → stored, `unverified`. Carrier
   missing (`mtt ref add t999 …`) → exit 4. Re-adding an existing key with a new `--label` updates the label;
   without `--label` is an idempotent no-op. Refs are stored sorted by `(kind,target)`.
5. **`mtt check`:** with a dangling ref present → prints it and exits **7**; with only `unverified`
   (url / no-KB) or none → exits **0**. `--json` emits a non-null array with `status` per entry. Cross-store:
   a dangling `task:` ref **from a note** is reported.
6. **Deletion guard (task):** `mtt rm t1` when `t2` (task) **or** a note references `task:t1` → **refused**
   (message names the referents), nothing deleted; `mtt rm t1 --force --who a --why b` deletes and writes an
   audit record, leaving the ref dangling; `--force` without who/why → exit 2. Bulk `mtt rm t1 t2` where they
   reference each other is **not** blocked by that mutual ref (subgraph-ignore).
7. **Deletion guard (note):** `mtt note rm auth-design` when a task/note references `note:auth-design` →
   **refused**; `mtt note rm auth-design --force --who a --why b` deletes + audits; `--force` without who/why
   → exit 2; missing slug → exit 4.
8. **Backlinks are computed:** deleting/adding a referent changes `show`/`ref list` backlinks with no stored
   back-reference anywhere on disk (verified by inspecting the referent's file — only forward `refs:` present).
9. **Serialization:** a note with `refs` has a struct-ordered frontmatter (`title`/`tags`/`refs`/`created`/
   `updated`, `omitempty` where applicable) + verbatim body; a note **without** refs is byte-identical to the
   pre-`t1` golden. Task/comment ref round-trip goldens still pass.
10. `make check` green. Docs synced (below).

## Testing approach

- **Unit (`pkg/mtt`):** `Note.Refs` presence is additive (no constructor change). `RefKind.Valid()` unchanged.
- **Unit (`internal/core`):**
  - ref-set algebra: upsert appends a new key, overwrites label only when provided, dedups exact key, sorts by
    `(kind,target)`; remove returns `found=false` for an absent key.
  - verification table: `task` present/absent; `note` with KB present/absent; `note` with **nil** KB →
    `unverified`; `url` → `unverified` (well-formed) and rejected (malformed, at the parse boundary).
  - `Backlinks`: task→task, note→task, task→note referents; deterministic order; empty when no refs.
  - `check` sweep: mixes `ok`/`dangling`/`unverified` across task and note carriers; result set is exactly the
    danglings; unverified counted, not failed.
  - `Remover` extended guard: refuses on a ref-referent (task carrier **and** note carrier); subgraph-ignore;
    `--force` audit-before-delete unchanged.
  - `NoteRemover`: refuse-by-default; `--force` forces who/why (missing → `ErrMissingAttribution`) and audits
    before delete; missing slug → `ErrNotFound`.
- **Golden / round-trip (`internal/adapter/yaml`):** note with refs; note without refs (byte-identical to the
  existing golden); a note whose body contains `---` still round-trips with a `refs:` frontmatter.
- **e2e (`internal/cli`, testscript):** AC-1…AC-9 flows; the `kind:target` parse table (incl. URL with
  embedded `:`); warn-on-dangling exit 0; `check` exit 7 vs 0; both deletion guards refuse then `--force`
  (with exit 2 on missing attribution); creation-time `--ref`; `--json` shapes (non-null arrays). No network,
  no shell pipes (model stdin via `cp`/`stdin`).

## Docs to sync (docs-sync judgment, `impl_review`)

Grep **all** parallel occurrences (EN + RU) before editing — the "parallel occurrences" trap.

- **`CLI_REFERENCE.md ↔ .ru.md`:** the **References** section — drop the phase markers (`field: phase 1;
  commands: phase 2; note targets need a KB, phase 5`) now that refs ship; document `mtt ref add/rm/list`,
  the new **`mtt note ref`** group, creation-time `--ref`, the `(kind,target)` key, statuses, and exit codes;
  update **`mtt check`** (drop "phase 5", pin exit 7, note `--fix` deferred); update `mtt note rm` (the "refuse
  if a task references it" guard now ships — refuse+`--force`+who/why/audit); note `comment` refs are `t2`.
- **`DESIGN.md ↔ .ru.md`:** §"Knowledge base and references" — the **Phases** bullet and the "KB & refs"
  decision row (refs wired in `t1` for `note`/`task`/`url`; `comment` → `t2`); add a "**Shipped (t1)**" block
  (kinds, carriers, computed cross-store backlinks, `check` exit 7, refuse-guard); the deletion note (`rm`
  guard now covers refs, cross-store). Reaffirm the "back-refs computed, never stored" invariant (t1 upholds).
- **`docs/architecture/model.go`:** add `Refs []Ref` to the `Note` block; note `RefStatus`/`Backlinks` are
  `core`-derived (not contract), like `Index`/`DepGraph`.
- **CLAUDE.md files:** `pkg/mtt` (`Note` no longer refs-free; the `comment`-input carve-out lives in the CLI,
  not the VO); `internal/core` (ref-set algebra, verification, `Backlinks`, `Checker`, `NoteRemover`, extended
  `Remover`); `internal/cli` (`ref` + `note ref` groups, `check`, `--ref`); `internal/adapter/yaml` (note
  frontmatter `refs`). Keep each thin.
- **`AGENTS.md`:** no new flow rule expected; touch only if a convention changes (it should not).

## Sequencing & tracking (process, not code)

`t1` is already `speccing` on `task/t1`. This document is the `speccing` deliverable. Next: commit it, run an
adversarial subagent **spec review**, address findings, then `spec_human_review` (maintainer sign-off) →
`planning` (writing-plans). Dependency already encoded: `t1 depends_on t47` (terminal), so `t1` is unblocked.
