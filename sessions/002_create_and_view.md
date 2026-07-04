# 002 — Create & view tasks

Status: planned   ·   Branch: `feat/s002-create-view`

## Target

Create a task and view it — the task CRUD skeleton: `Task` model, the YAML task store with ID minting,
the core `add` usecase, and `mtt add` / `mtt show`.

## Scope

- **In:** `Task` model (title/type/status/parent/created/updated/description; reserve the `history`/`refs`/
  `comments` fields); YAML adapter task save/load (deterministic serialization, atomic write, `<id>.yaml`),
  ID minting from config (`<prefix><N>`, `O_EXCL`); core `add` usecase (default type, initial status from
  the type's flow); `mtt add <title> [--type] [--description]`, `mtt show <id>`.
- **Out (deferred):** hierarchy / `--parent` → 004; `list`/`edit` → 003; dependencies, comments/refs
  population, flow gates.
- **Open questions (resolve at brainstorm — see [NEXT_SESSION.md](../NEXT_SESSION.md)):** (1) with ≥1
  `initial` status allowed per type, how does `add` choose the entry status? (2) the default type `task`
  has `parents: [epic]`, so a bare `add` (no `--parent`, deferred to 004) can't mint an `<epic>_t<N>` id and
  would break parent-type hierarchy — does bare `add` create a root type, seed an epic, or require a minimal
  `--type`/`--parent` now? The `e1` in the acceptance below reflects this **unresolved** point, not a fixed
  decision.

## Acceptance (must pass)

- **User scenario:** `mtt init` → `mtt add "fix login"` prints a new id (`e1`) → `mtt show e1` shows its
  title, type, and status (the type's initial status).
- **e2e:** `testscript` `add_show.txt`.
- Golden test for a serialized task file (deterministic).
- `make check` green.

## Plan (refine at session start — test-first)

- [ ] `pkg/mtt`: `Task` (with `history`/`refs`/`comments` field stubs)
- [ ] `internal/adapter/yaml`: task save/load + ID minting + golden
- [ ] `internal/core`: `add` usecase (default type, initial status)
- [ ] `internal/cli`: `add`, `show`
- [ ] `testscript` add → show

## Done (fill during/after the session)

—
