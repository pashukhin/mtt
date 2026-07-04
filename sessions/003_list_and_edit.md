# 003 — List & edit

Status: done   ·   Branch: `feat/s003-list-edit`

## Target

List tasks with filters, and edit non-flow fields. Rounds out flat task CRUD before hierarchy.

## Scope

- **In:** `mtt list` (filters `--status`/`--type`; stable order); `mtt edit <id> [--title] [--description]`
  (non-flow fields only — status changes go through flow in session 006).
- **Out (deferred):** hierarchy filters / `--parent` → 004; status transitions → 006.

## Acceptance (must pass)

- **User scenario:** add several tasks → `mtt list` shows them in a stable order; `--type bug` filters;
  `mtt edit e1 --title "…"` → `mtt show e1` reflects the change.
- **e2e:** `testscript` `list_edit.txt`.
- `make check` green.

## Plan (refine at session start — test-first)

- [x] `internal/core`: `list` (filter + stable sort), `edit` usecases
- [x] `internal/cli`: `list`, `edit`
- [x] `testscript` list/filter/edit

## Done

- **Port grew:** `TaskStore.List`/`Update` in `pkg/mtt`; the YAML adapter implements both (`List` reads
  `.mtt/tasks/*.yaml`, order unspecified; `Update` overwrites by ID, `ErrNotFound` if absent; `Create`/
  `Update` share one private `write` — one place for atomic temp+rename serialization).
- **`core.Select`** (pure read, no usecase): filter by `--status`/`--type` (AND across dimensions, OR
  within), default order `Created` desc (or `Updated` desc with `--sort updated`), tie-broken by ID as an
  opaque string — deterministic and provider-agnostic (no ID-structure parsing). Unit-tested for order
  determinism.
- **`core.Editor`** (mutation usecase): loads via `Get`, applies only the provided title/description (nil =
  unchanged), enforces the title-or-description invariant, bumps `updated` from an injected clock, persists
  via `Update`.
- **CLI:** `mtt list` (`--status`, `--type`, `--sort created|updated`, `--json`) composes `TaskStore.List` →
  `core.Select` and renders human text or a `taskJSON` array. `mtt edit <id> [--title] [--description]`
  goes through `core.Editor`, prints `updated <id>` or the JSON object.
- **Global flags:** root persistent `--dir`/`MTT_DIR`, `--version`, `--json`; the `projectRoot(cmd)` helper
  DRYs `--dir`/`MTT_DIR`-or-`FindRoot` resolution (used by `list`/`edit`/`show`; `baseDir` is the `init`-only
  variant, no `.mtt/` required); `taskJSON` is the shared JSON view for `show`/`list`/`edit`.
- **e2e:** `internal/cli/testdata/scripts/list_edit.txt` — init → add (epic ×2, task) → `list` (presence,
  `--type`, `--status` incl. an unknown status returning `[]` under `--json`, `--sort updated`, invalid
  `--sort` error, `--json`) → `edit` (title, description, "nothing to edit", missing id) → `--dir`/
  `MTT_DIR` from outside the project.
- **Deferred (see DESIGN.md → "Listing and editing" / backlog):** a durable, git-independent audit of
  edits (a change-log or field versioning — `history` stays transition-only) and the subject-identity
  (`By`) source (likely `.mtt/config.local.yaml`, distinct from `--role`) — both scheduled as a design
  slice, not scoped into this session.
