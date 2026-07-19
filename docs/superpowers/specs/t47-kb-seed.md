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
  `KnowledgeStore` port (capabilities / `mtt caps` come later, e4_t6).

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

- `NoteSlug` is a **structurally validated** value object: **non-empty**, **kebab-ASCII**
  `^[a-z0-9]+(-[a-z0-9]+)*$` (filesystem- and URL-safe — it *is* the file name). A validating constructor
  `NewNoteSlug(string) (NoteSlug, error)` rejects anything else.
- **This is a deliberate EXCEPTION to the opaque-identity rule.** Other identities (`TaskID`/`TypeName`/…)
  "reject empty, never transform / parse structure" (`pkg/mtt/identity.go`, `pkg/mtt/CLAUDE.md`), and task
  `toDomain` validates only emptiness. `NoteSlug` parses structure **on purpose**, because it is a filesystem
  path segment, not a provider-minted opaque id — the regex *is* the traversal defense (finding #2). Record
  this carve-out in `pkg/mtt/CLAUDE.md` (docs-sync).
- **Validated at every boundary, defense-in-depth** (the type cannot force construction — a raw
  `NoteSlug("../x")` cast compiles): (a) the CLI runs every user-supplied slug through `NewNoteSlug` **before**
  calling the store (never a raw cast); (b) the adapter **re-validates** the slug in every path-building method
  (`Create/Get/Update/Delete`); (c) `load`/`List` validates each **filename-derived** slug (a hand-planted
  malicious/corrupt file fails fast — and is **not** `ErrNotFound`). An invalid slug from the CLI is a usage
  error (exit 1); a corrupt on-disk slug is a load error. No path is ever built from an unvalidated slug.

### D2 — `Note` value object (no versioning, no refs)

```
Note{ Slug NoteSlug; Title string; Tags []string; Body string; Created, Updated time.Time }
```

- **No** `Version`/`Predecessor` — a note has one current version; its history is **git** (as a task's flow
  history is git). Versioning is a re-scoped `t6` concern, added later as a *separate optional capability*
  (the `HistoryStore` pattern), not baked into the base port.
- **No** `Refs` field — all ref handling (on tasks, comments, **and** notes) belongs to `t1`. The seed's note
  is refs-free; `t1` adds `Note.Refs` additively when it does refs end-to-end.
- **Tags reuse the task vocabulary** (normalized + deduped + **sorted** via `core.canonicalTags`; each value
  via `pkg/mtt` `NormalizeTag`) — DRY. See D5 for the exact helper split.
  **Explicit `--tag` only; no hashtag extraction from title/body.** A markdown body is full of `#` headings,
  which would produce garbage tags — so unlike tasks (which extract `#hashtags` from title/description), notes
  do not scan text for tags.
- `Title` is **optional** (a stub note may be slug-only; `show` falls back to the slug for display).

### D3 — Base `KnowledgeStore` port (mandatory minimum)

New driven port in `pkg/mtt` (e.g. `knowledge.go`):

```
KnowledgeStore interface {
    CreateNote(n Note) (Note, error)   // reserve-then-write; errors if the slug exists
    GetNote(slug NoteSlug) (Note, error)   // mtt.ErrNotFound when absent — no version param
    ListNotes() ([]Note, error)            // order unspecified; caller orders
    UpdateNote(n Note) (Note, error)
    DeleteNote(slug NoteSlug) error
}
```

- **No version parameter** on `GetNote` (deviates from model.go's T3 sketch on purpose; versioning is `t6`).
- **No `CapKnowledge` / `Capability` type in this task (finding #4).** Real `pkg/mtt` has no capability
  machinery yet (it lives only in the aspirational `model.go`); introducing a `Capability` type for a single
  unused constant is speculative generality (KISS/YAGNI). `t1` decides whether `note` refs are verifiable by
  whether a `KnowledgeStore` is **wired** (a nil / type-assertion check on the injected port) — which needs
  neither the constant nor the type. The capability constant lands with its first consumer, `Capabilities()` /
  `mtt caps` (`e4_t6`). `model.go` already reserves the name, so nothing is lost by deferring.
- **Not-found is `mtt.ErrNotFound`** → uniform **exit 4** (s008.5 taxonomy). The shared sentinel's message is
  currently task-specific (`"mtt: task not found"`); generalize its doc comment to cover notes and add a CLI
  `noteNotFound(slug)` wrapper (`note %q: %w`, mirroring `taskNotFound`) so the user sees the right noun
  (docs-sync + finding #7).

### D4 — YAML adapter: one markdown file per note

- Layout: `.mtt/knowledge/<slug>.md` — YAML **frontmatter** followed by the markdown body.
  `.mtt/knowledge/` is created lazily on first `add` and is committed (project data, like `.mtt/tasks/`).
- Frontmatter carries `title`, `tags`, `created`, `updated` in **fixed field order** via a **struct DTO**
  (`note_dto.go`) — NOT a `map` (yaml.v3 does not order map keys; a struct does), `omitempty` for `title` and
  `tags`. `created`/`updated` are always present, so the frontmatter block is never empty. **The slug is the
  file name and is NOT in the frontmatter** (single source of truth; the file name is authoritative — it is
  what refs resolve against). `toDomain` fails fast on a corrupt time; the slug comes from the **filename**
  (validated per D1, not from the DTO).

**Serialization contract (finding #1 — a hybrid document, no prior art; pin it exactly):**
- **Write:** `---\n` + `yaml.Marshal(frontmatterDTO)` + `---\n` + `body` **verbatim**. The body is written
  byte-for-byte; its trailing newline (or absence) is preserved exactly (determinism / no data mutation).
- **Read:** the file MUST begin with a line that is exactly `---`. The frontmatter is the bytes up to the
  **first subsequent** line that is exactly `---`; the body is **everything after that delimiter line,
  byte-for-byte**. Only the frontmatter bytes go to `yaml.Unmarshal` — **never** unmarshal the whole file
  (`---` is yaml.v3's *document separator*; feeding the whole file makes the body a second document →
  corruption/error). A file not beginning with `---` is corrupt → load error (fail-fast, **not** `ErrNotFound`).
- **`---` inside the body is safe:** only the *first* closing `---` delimits; every later `---` (markdown
  thematic breaks, fenced examples) stays in the body verbatim. This case gets a round-trip golden.
- **Reserve-then-write (no clobber, finding #6):** `atomicWrite` (temp+rename) alone would **overwrite**
  (`os.Rename` replaces the destination) — so `CreateNote` first reserves the final `<slug>.md` with
  `O_CREATE|O_EXCL` (mirroring task `mint`); `ErrExist` → a plain "note slug exists" error (exit 1, **not**
  `ErrNotFound`). Then it `atomicWrite`s the content. `UpdateNote` uses `atomicWrite` directly (overwrite is
  correct for an edit). No ID minting — the slug is supplied.

### D5 — Core usecases (pure, clocked) mirror the task layer

- Mutations are `core` usecases with an injected `now func() time.Time` (as `Adder`/`Editor` are):
  a note **adder** (validate slug via `NewNoteSlug`, set `Created`/`Updated`, canonicalize tags) and a note
  **editor** (touch only provided fields — title/tags/body — bump `Updated`, keep `Created`).
- **Tag helpers (correct names, finding #9):** the sorted+deduped set is `core.canonicalTags`
  (`internal/core/tags.go`, unexported — the note usecases are in the same package, so reuse is in-package);
  each value is normalized by `pkg/mtt.NormalizeTag` at the CLI boundary (as `toTags` does). `NormalizeTag`
  normalizes a **single** tag — it is not the whole set operation.
- List is a **pure** filter/order: a `NoteFilter{Tags}` over `anyOrEmptyIntersect` (the s008.7 generic
  slice-filter helper in `internal/core/list.go`), ordered `Created` desc with a slug tiebreak (mirrors
  `Select`'s provider-agnostic ordering). Get/Delete are thin pass-throughs (no reference logic here).
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
  refs, so it lands with `t1`). Missing slug → exit 4 via `noteNotFound(slug)`.
- **`--json` shape (finding #8, pin it):** a note JSON object = `slug` (**always** present — it is the
  identity, absent from the DTO so it must be added explicitly), `title` (`omitempty`, matching `taskJSON`),
  `tags` (a **non-null** array — `[]` when empty, the `dep`/`roadmap` house rule for JSON views), `body`,
  `created`, `updated`. `note add`/`note show` emit the single object; `note list --json` emits a **non-null**
  array (`[]` when empty, never `null`). The e2e asserts this shape.

## Scope

**In:** the `Note` VO + `NoteSlug` (validating constructor); the base `KnowledgeStore` port; the YAML
knowledge adapter (`.mtt/knowledge/<slug>.md`, frontmatter DTO, serialization contract, atomic
reserve-then-write, load); core note usecases (add/edit) + pure list filter; the `mtt note` CLI group;
unit + golden + e2e tests; docs sync.

**Out:**
- **All `refs`** (add/list/verify, on tasks/comments/notes) → **`t1`**.
- **Full-text search** (`SearchStore`/`CapSearch`, `mtt search`) → **`t6`**.
- **Note versioning** (`Version`/`Predecessor`, `NoteHistory`, `GetNote(version)`) → **`t6`** (separate
  optional capability).
- **`CapKnowledge` / `Capability` type / `mtt caps` / `Capabilities()` surface** → `e4_t6` (no consumer yet).
- **Mass knowledge migration** out of DESIGN/AGENTS/NEXT_SESSION → a **separate** decision after the seed
  (avoid a double source of truth — the "parallel occurrences" trap). At most **one** small demonstration
  note may be committed as a smoke/dogfood artifact; it is optional, not the migration.

## Acceptance criteria

1. The full CRUD loop works end-to-end (e2e): `note add <slug>` → `note show <slug>` (body + frontmatter) →
   `note list --tag <t>` finds it → `note edit` changes a field and bumps `updated` (keeps `created`) →
   `note rm` deletes the file.
2. `.mtt/knowledge/<slug>.md` is a reviewable committed file: struct-ordered frontmatter
   (`title`/`tags`/`created`/`updated`, `omitempty` title/tags) + markdown body; slug **not** in frontmatter.
   Goldens pin: (a) a minimal note (no title/tags); (b) a full note; (c) a note whose **body contains `---`
   lines** — round-trips byte-for-byte (only the first closing `---` delimits); (d) **trailing-newline**
   preservation (body with and without a trailing `\n` round-trip exactly).
3. **Slug validation, defense-in-depth:** invalid slugs — empty, uppercase, spaces, leading/trailing `-`,
   `--`, and the **traversal/path** cases (`../x`, `a/b`, `/abs`, embedded newline) — are rejected. Enforced
   by `NewNoteSlug` at the CLI (`add`/`edit`/`show`/`rm` → exit 1) **and** re-validated in the adapter's
   path-building methods and on load; a hand-planted corrupt/malicious file fails `load` fast (and is **not**
   `ErrNotFound`). No path is ever built from an unvalidated slug.
4. `note show`/`note rm`/`note edit` on a missing slug exits **4** (`mtt.ErrNotFound` via `noteNotFound`,
   `note %q: …`), matching the task taxonomy.
5. `CreateNote` is **reserve-then-write** (`O_CREATE|O_EXCL` on the final path, then `atomicWrite`): an
   existing slug errors (exit 1, "note slug exists"; **not** `ErrNotFound`), never a silent overwrite.
   `UpdateNote` overwrites via `atomicWrite`.
6. Tags: `--tag` normalized + deduped + sorted (`core.canonicalTags`); **no** tags extracted from title/body;
   `--tag` filter on `list` is OR-within-tags.
7. Body input via `--body` / `--file` / stdin `-` (mutually exclusive); empty body allowed.
8. **`--json` shape:** `add`/`show` emit a note object with `slug` always present and `tags` a non-null array;
   `list --json` emits a non-null array (`[]` when empty). An e2e asserts the shape.
9. `make check` green. Docs synced (see below).

## Testing approach

- **Unit** (`pkg/mtt`): `NewNoteSlug` validity table — valid + every invalid class incl. the **traversal**
  cases (`../x`, `a/b`, `/abs`, embedded newline, uppercase, spaces, `--`, empty). Run the regex against the
  table (RE2), as s008.7 did for tags.
- **Unit** (`internal/core`): note adder (slug validation, clock, `canonicalTags`), editor
  (provided-fields-only, `Updated` bumped / `Created` kept), pure list filter (tag intersection, order +
  tiebreak, empty filter).
- **Golden / round-trip** (`internal/adapter/yaml`): minimal note (no title/tags); full note; **body
  containing `---` lines** (round-trips byte-for-byte); **trailing-newline** present vs absent; a corrupt file
  not beginning with `---` → load error that is **not** `ErrNotFound`; the adapter rejects an unvalidated
  `NoteSlug("../x")` in a path-building method; `CreateNote` existing-slug rejection (no clobber).
- **e2e** (`internal/cli`, testscript `note.txt`): the AC-1 loop; slug-rejection (incl. a traversal slug);
  not-found exit 4 (with the `note %q` wording); the stdin/`--file` body paths; the `--json` shape (`slug`
  present, `tags`/list non-null `[]`). (No shell pipes in testscript — model stdin via `cp`/`stdin`.)

## Docs to sync (docs-sync judgment, `impl_review`)

- **`docs/architecture/model.go`** (contract snapshot — required when a `pkg/mtt` T1 signature changes):
  update the `Note` block (fields `Slug/Title/Tags/Body/Created/Updated`; drop `Version/Predecessor`) and the
  `KnowledgeStore` block (no `version` param; add `ListNotes/UpdateNote/DeleteNote`; versioning + `NoteHistory`
  deferred to t6). Keep the note that `CapKnowledge`/`Capabilities()` remain T3/deferred.
- **DESIGN.md ↔ DESIGN.ru.md:** the KB is no longer only "phase 5" — the base `KnowledgeStore` ships. Update
  the "Data layout" `knowledge/` line (drop `[phase 5]` for the base store), the "KB & refs" decision row, and
  the "Adapter capabilities" list (`KnowledgeStore` now real in YAML; versioning/search still optional). Grep
  **all** parallel occurrences (EN + RU) before editing (the "parallel occurrences" trap).
- **CLI_REFERENCE.md ↔ CLI_REFERENCE.ru.md:** add the `mtt note` group.
- **`pkg/mtt/store.go`:** generalize the `ErrNotFound` doc comment to name notes (kept minimal; message text
  unchanged — the noun is surfaced via the CLI `noteNotFound` wrapper).
- **CLAUDE.md files:** `internal/adapter/yaml` (knowledge files + the serialization contract), `internal/cli`
  (`note` group), and **`pkg/mtt/CLAUDE.md`** — record the `NoteSlug` **structural-validation carve-out** to
  the otherwise-opaque-identity rule (D1). Keep each thin.

## Sequencing & tracking (process, not code)

1. `mtt add 'KB seed — notes CRUD: KnowledgeStore port + note add/list/show/edit/rm over .mtt/knowledge/<slug>.md' --priority high --tag kb --tag core --tag release`
2. `mtt dep add t1 t47` — encode "seed before references" in the roadmap (AGENTS.md rule 0: mechanize
   the process in mtt, not in an agent's head).
3. Re-scope `t6`: `mtt edit t6 --title 'knowledge base: full-text search + note versioning (Phase 5)'`
   (stays `low`, `backlog`, `kb`).
4. Drive the seed through flow v2 (`start → speccing → …`). This spec is the `speccing` deliverable.
