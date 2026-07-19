# KB seed — notes CRUD (the minimal `KnowledgeStore`)

Status: spec (decision record). Type: task (`t47`). Branch: `task/t47`.

## Context / problem

mtt is a "tasks + knowledge" pairing, but only the **tasks** half exists. The **knowledge** half is a
reserved seam: `docs/architecture/model.go` sketches `Note`, `NoteSlug`, `KnowledgeStore`, `CapKnowledge`
(all T3, aspirational), and `pkg/mtt/task.go` already carries the reference vocabulary — `Ref{Kind,ID,Label}`,
`RefKind` (`note`/`task`/`comment`/`url` + `Valid()`), and reserved `Task.Refs` / `Comment.Refs`. Nothing of
the KB is implemented in real code: `pkg/mtt/store.go` has only `TaskStore`.

Two drivers make a **minimal KB now** worthwhile (both wanted by the maintainer):

1. **Complete `t1` (references).** `t1` wires and verifies the already-reserved `Ref` types. One of its four
   ref kinds — `note` — resolves *only* against a `KnowledgeStore` ([model.go:111](../../architecture/model.go)).
   Without a KB, `note` is a **dead kind**: nothing to point at, nothing to verify, so `t1` would ship 3/4.
2. **Dogfood project knowledge.** A git-versioned, tagged, queryable note store with stable handles is
   useful on its own — a home for durable design knowledge, referenceable later from tasks.

The wedge is deliberately narrow: **make notes *exist* (a real target and a real store); leave all `refs`
work — including refs *on* notes — to `t1`; leave full-text search and note *versioning* to a re-scoped
`t6`.** This is the "mandatory minimum + optional capability" philosophy from DESIGN applied to the KB port.

## User stories

Primary user = the coding **agent**; secondary = the human **maintainer**.

Closed by this task (the seed alone):

- **US1** — As an agent, store durable project knowledge as a note with a **stable slug**, to return to and
  (later) reference. `mtt note add positioning-vs-beads --title "…"`
- **US2** — Retrieve a note by slug, for human and machine. `mtt note show <slug> [--json]`
- **US3** — Find relevant notes by topic. `mtt note list --tag design`
- **US4** — Keep knowledge current / purge stale. `mtt note edit <slug> …` / `mtt note rm <slug>`
- **US5** — As a maintainer, review knowledge changes as a normal git diff. `.mtt/knowledge/*.md`,
  deterministic serialization.
- **US6** — (adaptivity seam) so the same CLI can later target an external KB (Confluence). The
  `KnowledgeStore` port + `CapKnowledge`.

Unlocked here, **closed by `t1`** (out of scope):

- **US7** — As an agent, reference a note from a task so broken links are caught.
  `refs: [{kind: note, id: <slug>}]` + verification.

So driver #2 (dogfood) is satisfied by the seed; driver #1 (verifiable links) gets its **target** here and
its **link + verification** in `t1`.

## Decisions

### D1 — Note identity is an **explicit, author-supplied slug**

The slug is given at creation (`mtt note add <slug> …`), not minted (`n1`) and not derived from a title.
Rationale: the slug is the note's durable handle that `refs` will point at; keeping it independent of the
title means retitling never re-mints it and never breaks a ref — the same "identity decoupled from text"
principle tasks follow. Minted `n1` is opaque for a KB and contradicts `NoteSlug` (a *slug*) in model.go.

- `NoteSlug` is a validated identity value object: **non-empty**, **kebab-ASCII** `^[a-z0-9]+(-[a-z0-9]+)*$`
  (filesystem- and URL-safe, since it is the file name). Validated at the CLI boundary and **fail-fast on
  adapter load** (mirrors `toDomain` rejecting a corrupt on-disk id). Empty/invalid slug → usage error.

### D2 — `Note` value object (no versioning, no refs)

```
Note{ Slug NoteSlug; Title string; Tags []string; Body string; Created, Updated time.Time }
```

- **No** `Version`/`Predecessor` — a note has one current version; its history is **git** (as a task's flow
  history is git). Versioning is a re-scoped `t6` concern, added later as a *separate optional capability*
  (the `HistoryStore` pattern), not baked into the base port.
- **No** `Refs` field — all ref handling (on tasks, comments, **and** notes) belongs to `t1`. The seed's note
  is refs-free; `t1` adds `Note.Refs` additively when it does refs end-to-end.
- **Tags reuse the task vocabulary** (`pkg/mtt` `NormalizeTag`, normalized + deduped + **sorted** set) — DRY.
  **Explicit `--tag` only; no hashtag extraction from title/body.** A markdown body is full of `#` headings,
  which would produce garbage tags — so unlike tasks (which extract `#hashtags` from title/description), notes
  do not scan text for tags.
- `Title` is **optional** (a stub note may be slug-only; `show` falls back to the slug for display).

### D3 — Base `KnowledgeStore` port (mandatory minimum) + `CapKnowledge`

New driven port in `pkg/mtt` (e.g. `knowledge.go`):

```
KnowledgeStore interface {
    CreateNote(n Note) (Note, error)   // rejects an existing slug
    GetNote(slug NoteSlug) (Note, error)   // mtt.ErrNotFound when absent — no version param
    ListNotes() ([]Note, error)            // order unspecified; caller orders
    UpdateNote(n Note) (Note, error)
    DeleteNote(slug NoteSlug) error
}
```

- **No version parameter** on `GetNote` (deviates from model.go's T3 sketch on purpose; versioning is `t6`).
- Reuse the shared `mtt.ErrNotFound` sentinel → uniform **exit 4** for a missing note (the s008.5 taxonomy).
- Add a `CapKnowledge` capability constant (aligning with model.go). The full `Capabilities()` surface and a
  `mtt caps` command stay **out of scope** (that is `e4_t6`); `t1` verifies `note` refs by whether a
  `KnowledgeStore` is wired at all — capability-aware degradation, per DESIGN.

### D4 — YAML adapter: one markdown file per note

- Layout: `.mtt/knowledge/<slug>.md` — YAML **frontmatter** (`--- … ---`) followed by the markdown body.
  `.mtt/knowledge/` is created lazily on first `add` and is committed (project data, like `.mtt/tasks/`).
- Frontmatter carries `title`, `tags`, `created`, `updated` in **fixed field order** (deterministic diff),
  `omitempty` for `title` and `tags`. **The slug is the file name and is NOT duplicated in frontmatter**
  (single source of truth; the file name is authoritative for identity, i.e. what refs resolve against).
- Writes are **atomic** (temp + rename), like tasks. `CreateNote` refuses an existing slug (`O_EXCL` on the
  file). No ID minting — the slug is supplied.
- A new DTO (`note_dto.go`) maps frontmatter ↔ `Note`; `toDomain` fails fast on a corrupt slug/time.

### D5 — Core usecases (pure, clocked) mirror the task layer

- Mutations are `core` usecases with an injected `now func() time.Time` (as `Adder`/`Editor` are):
  a note **adder** (validate slug, set `Created`/`Updated`, canonicalize tags) and a note **editor**
  (touch only provided fields — title/tags/body — bump `Updated`, keep `Created`).
- List is a **pure** filter/order: a `NoteFilter{Tags}` over an `anyOrEmptyIntersect` tag match (reuse the
  s008.7 slice-filter helper), ordered `Created` desc with a slug tiebreak (mirrors `Select`'s
  provider-agnostic ordering). Get/Delete are thin pass-throughs (no reference logic here).
- Layer invariant holds: `core` talks only to the `KnowledgeStore` port; the CLI assembles the YAML
  `KnowledgeStore` at the composition root and injects it.

### D6 — CLI: `mtt note` command group

Parent `mtt note` with subcommands (the `mtt dep` pattern):

- `mtt note add <slug> [--title <t>] [--tag <t>]… [--body <s> | --file <path> | -]` — body via flag, file, or
  stdin (`-`); the three are mutually exclusive; an empty body is allowed. Agent-friendly (no `$EDITOR`).
- `mtt note list [--tag <t>]… [--json]`
- `mtt note show <slug> [--json]` — frontmatter fields + body (JSON: a note object).
- `mtt note edit <slug> [--title …] [--tag …]… [--body … | --file … | -]` — updates only **changed** flags
  (cobra `Changed()`, as task `edit` does). When `--tag` is provided it **replaces** the whole tag set
  (declarative, not additive); clearing all tags is a minor gap deferred to a later `note tag` command.
- `mtt note rm <slug>` — **unconditional** delete in the seed (a "reject if a task references it" guard needs
  refs, so it lands with `t1`). Missing slug → exit 4.
- `--json` on read paths; a shared note JSON view.

## Scope

**In:** the `Note` VO + `NoteSlug`; the base `KnowledgeStore` port + `CapKnowledge`; the YAML knowledge
adapter (`.mtt/knowledge/<slug>.md`, frontmatter DTO, atomic write, load); core note usecases (add/edit) +
pure list filter; the `mtt note` CLI group; unit + golden + e2e tests; docs sync.

**Out:**
- **All `refs`** (add/list/verify, on tasks/comments/notes) → **`t1`**.
- **Full-text search** (`SearchStore`/`CapSearch`, `mtt search`) → **`t6`**.
- **Note versioning** (`Version`/`Predecessor`, `NoteHistory`, `GetNote(version)`) → **`t6`** (separate
  optional capability).
- **`mtt caps` / `Capabilities()` surface** → `e4_t6`.
- **Mass knowledge migration** out of DESIGN/AGENTS/NEXT_SESSION → a **separate** decision after the seed
  (avoid a double source of truth — the "parallel occurrences" trap). At most **one** small demonstration
  note may be committed as a smoke/dogfood artifact; it is optional, not the migration.

## Acceptance criteria

1. The full CRUD loop works end-to-end (e2e): `note add <slug>` → `note show <slug>` (body + frontmatter) →
   `note list --tag <t>` finds it → `note edit` changes a field and bumps `updated` (keeps `created`) →
   `note rm` deletes the file.
2. `.mtt/knowledge/<slug>.md` is a reviewable committed file: frontmatter (`title`/`tags`/`created`/`updated`,
   fixed order, `omitempty`) + markdown body; slug **not** in frontmatter; a golden pins the layout.
3. Slug validation: an invalid slug (empty, uppercase, spaces, leading/trailing/`--`) is rejected as a usage
   error at `add`; a corrupt on-disk slug fails `load` fast (and is **not** `ErrNotFound`).
4. `note show`/`note rm` on a missing slug exits **4** (`mtt.ErrNotFound`), matching the task taxonomy.
5. `CreateNote` refuses an existing slug (no silent overwrite).
6. Tags: `--tag` normalized + deduped + sorted; **no** tags extracted from title/body; `--tag` filter on
   `list` is OR-within-tags. `--json` present on `add`/`show`/`list`.
7. Body input via `--body` / `--file` / stdin `-` (mutually exclusive); empty body allowed.
8. `make check` green. Docs synced (see below).

## Testing approach

- **Unit** (`pkg/mtt`): `NoteSlug` validity table (valid/invalid slugs); `Note` tag canonicalization.
- **Unit** (`internal/core`): note adder (slug validation, clock, tag canon), editor (provided-fields-only,
  `Updated` bumped / `Created` kept), pure list filter (tag intersection, order + tiebreak, empty filter).
- **Golden** (`internal/adapter/yaml`): a minimal note (no title/tags) and a full note; round-trip; a
  corrupt-slug `load` error that is **not** `ErrNotFound`; `CreateNote` existing-slug rejection.
- **e2e** (`internal/cli`, testscript `note.txt`): the AC-1 loop; slug-rejection; not-found exit 4; the
  stdin/`--file` body paths; `--json` shape. (No shell pipes in testscript — model stdin via `cp`/`stdin`.)

## Docs to sync (docs-sync judgment, `impl_review`)

- **DESIGN.md ↔ DESIGN.ru.md:** the KB is no longer only "phase 5" — the base `KnowledgeStore` ships. Update
  the "Data layout" `knowledge/` line (drop `[phase 5]` for the base store), the "KB & refs" decision row, and
  the "Adapter capabilities" list (`KnowledgeStore` now real in YAML; versioning/search still optional). Grep
  **all** parallel occurrences (EN + RU) before editing.
- **CLI_REFERENCE.md ↔ CLI_REFERENCE.ru.md:** add the `mtt note` group.
- **CLAUDE.md:** new/changed packages — `internal/adapter/yaml` (knowledge files), `internal/cli` (`note`
  group), and a `pkg/mtt` note re: the new port; keep each thin.

## Sequencing & tracking (process, not code)

1. `mtt add 'KB seed — notes CRUD: KnowledgeStore port + note add/list/show/edit/rm over .mtt/knowledge/<slug>.md' --priority high --tag kb --tag core --tag release`
2. `mtt dep add t1 t47` — encode "seed before references" in the roadmap (AGENTS.md rule 0: mechanize
   the process in mtt, not in an agent's head).
3. Re-scope `t6`: `mtt edit t6 --title 'knowledge base: full-text search + note versioning (Phase 5)'`
   (stays `low`, `backlog`, `kb`).
4. Drive the seed through flow v2 (`start → speccing → …`). This spec is the `speccing` deliverable.
