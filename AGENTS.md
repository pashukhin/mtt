# AGENTS.md

Rules for agents and humans working in this repository.
Architecture and decisions live in [DESIGN.md](DESIGN.md). This file is about how to work.

## TL;DR

1. Work on a per-task branch, not on `main`.
2. **Test before code** (TDD: red → green → refactor). `make check` must be **green** before you commit.
3. Fanatically: **SOLID, DRY, KISS, clean architecture** (see "Principles").
4. Thin CLI layer; logic lives in `core`; storage sits behind a port (adapter) — never touch `.mtt/` directly.
5. Changing behavior? Update `DESIGN.md` and the affected `CLAUDE.md` files.

## Principles (non-negotiable)

We fanatically follow **SOLID, DRY, KISS, TDD, clean architecture** (hexagonal). Dependencies point inward:
`cli → core → port ← adapter`. Domain types and ports live in the public `pkg/mtt`; they know nothing
about the CLI, files, or YAML. `core` never imports `adapter/*`; adapters carry no business rules.

Before you consider a task done — an explicit self-check (answer honestly):

- "Is this *really* clean architecture — or can I do cleaner? Where do the layers leak?"
- "Any duplication (**DRY**)? Any needless complexity (**KISS**)?"
- "Was the test written **before** the code (**TDD**)?"
- "Does each exported type/function have one responsibility (**SRP**)? Are the abstractions right?"

Any "not sure" → refactor before committing, not after.

## Commands

```bash
make check     # THE GATE: fmt-check + vet + lint + test -race + build  (required before commit)
make test      # go test -race -cover ./...
make build     # build to ./bin/mtt
make fmt       # gofmt + goimports (format in place)
make lint      # golangci-lint run
```

Requires: Go 1.23+, `golangci-lint` v2, `goimports`.

## Definition of Done (per task)

- [ ] Test written **before** the code (TDD: red → green → refactor).
- [ ] Self-check from "Principles" passed (layer cleanliness, DRY, KISS, SRP).
- [ ] `make check` green locally.
- [ ] `DESIGN.md` and the affected `CLAUDE.md` updated if behavior/data model changed.
- [ ] Branch → PR → CI green → squash-merge into `main`.

## Quality gate

`make check` is exactly what CI runs. Don't commit if it's red. Components:

- `gofmt -l` — fail on unformatted code;
- `go vet ./...`;
- `golangci-lint run` (config in `.golangci.yml`, v2 format);
- `go test -race -cover ./...`;
- `go build ./...`.

## Go conventions

- Wrap errors with `fmt.Errorf("...: %w", err)`; never ignore `err`.
- No `panic` in library code (`core`/`adapter`/`pkg`); a panic means a programmer error only.
- CLI commands stay thin: flag parsing and output; all logic in `core`.
- Small packages, export the minimum. Everything exported gets a doc comment.
- Deterministic serialization: field order = struct order; don't reorder arbitrarily.
- Don't pull in heavy dependencies without reason — justify any new dependency briefly in the PR.

## Storage invariants

- Read/write storage **only through a port** (`TaskStore`/`KnowledgeStore`), never directly.
- In the YAML adapter, `.mtt/` is committed and is the source of truth; don't hand-edit files.
- IDs are stable (`e1_t3_s2`) and independent of `title`.
- File writes are atomic (temp + rename); a new ID is created via `O_EXCL`.

## Tests

- Unit, table-driven: `core` (usecase) / `adapter/yaml`.
- Golden tests for YAML serialization (`-update` flag to regenerate goldens).
- CLI e2e via `testscript` (txtar scripts) in temp dirs.
- No network in tests.

## In-code doc hierarchy (CLAUDE.md)

- Root [CLAUDE.md](CLAUDE.md) — a thin entry point: what to read and the key rules.
- Every package under `internal/` has its own thin `CLAUDE.md`: the package's responsibility, invariants,
  boundaries (what it does and what it does NOT do).
- Create a package → create its `CLAUDE.md`; change a package's behavior → update it.
- CLAUDE.md files are **complete in substance but thin**: no filler, no duplication of DESIGN.md
  (architecture lives there; CLAUDE.md is local orientation).

## Documentation language

- **Agent-facing docs are English only:** `AGENTS.md`, the `CLAUDE.md` files, `TASKS.md`, `NEXT_SESSION.md`.
- **Bilingual docs (English primary + Russian mirror):** `README.md` ↔ `README.ru.md`,
  `DESIGN.md` ↔ `DESIGN.ru.md`, `CLI_REFERENCE.md` ↔ `CLI_REFERENCE.ru.md`. English is the source of
  truth; when either changes, update both and keep them consistent.

## Git

- Branches: `feat/…`, `fix/…`, `chore/…`. Small commits, imperative mood.
- Don't push or create a remote without an explicit request from the user.
- Commit under the user's configured git identity (don't override it).
- Commit trailer:
  `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

## Sessions

Work in **compact sessions** — each small and ending with something practical and **immediately
verifiable** (a user-runnable command with an e2e test). One file per session in
[sessions/](sessions/) (`NNN_<slug>.md`): write the target up front, fill in what was actually done.
Start a session by refining its plan (superpowers), then work test-first; branch `feat/sNNN-<slug>` →
PR → squash. The roadmap and current target live in [sessions/README.md](sessions/README.md); the design
backlog stays in [DESIGN.md](DESIGN.md) / [TASKS.md](TASKS.md). After phase 4 the backlog itself moves
onto mtt (dogfooding).
