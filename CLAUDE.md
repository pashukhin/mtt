# CLAUDE.md — mtt

Thin entry point for agents. Full rules — in [AGENTS.md](AGENTS.md), architecture — in
[DESIGN.md](DESIGN.md), task plan — in [TASKS.md](TASKS.md).

**Read at the start of a session:** AGENTS.md → DESIGN.md → TASKS.md.

## What it is

`mtt` is an agent-friendly, lightweight "tasks + knowledge" pairing (Go CLI), like Jira+Confluence without
the bulk. Storage is abstracted behind ports; the default is YAML, one file per task in `.mtt/`, but an
external pairing (Jira+Confluence, etc.) can be plugged in via an adapter.

## Non-negotiable rules (details in AGENTS.md)

- **Test before code** (TDD: red → green → refactor). `make check` green before commit.
- Fanatically: **SOLID, DRY, KISS, clean architecture** (hexagonal). Dependencies point inward:
  `cli → core → port ← adapter`; the contract (domain types + ports) lives in the public `pkg/mtt`.
- Per-task branch → PR → CI green → squash into `main`.
- Storage **only through a port** (`TaskStore`/`KnowledgeStore`); YAML adapter by default.
- Every package under `internal/` keeps its own thin `CLAUDE.md` current.

## Gate

`make check` = gofmt + go vet + golangci-lint(v2) + `go test -race -cover` + build.

## Docs language

Agent-facing docs (this file, AGENTS.md, TASKS.md, NEXT_SESSION.md) are English. Human-facing docs are
bilingual (English primary, keep in sync): `README.md` ↔ `README.ru.md`, `DESIGN.md` ↔ `DESIGN.ru.md`,
`CLI_REFERENCE.md` ↔ `CLI_REFERENCE.ru.md`.

## Skills / guards

The **superpowers** plugin (skills: TDD, brainstorming, debugging, planning) is a **personal**
development-process preference, not a project one: enabled in `.claude/settings.local.json` (per-user,
gitignored). Activation instructions — in [NEXT_SESSION.md](NEXT_SESSION.md).
