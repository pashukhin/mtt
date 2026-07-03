# 003 — List & edit

Status: planned   ·   Branch: `feat/s003-list-edit`

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

- [ ] `internal/core`: `list` (filter + stable sort), `edit` usecases
- [ ] `internal/cli`: `list`, `edit`
- [ ] `testscript` list/filter/edit

## Done (fill during/after the session)

—
