# AGENTS.md

Rules for agents and humans working in this repository.
Architecture and decisions live in [DESIGN.md](DESIGN.md). This file is about how to work.

## TL;DR

0. **Mechanize the process into mtt.** If any part of *how we work* can live in mtt — a flow gate, a status, a
   status/edge **description**, a **dependency**, a priority, or (later) a **post-action** — move it there
   instead of keeping it a manual convention. The tool encodes the process; agents and humans shouldn't have
   to remember it. Every recurring manual tick between transitions is a bug against this rule — file it (see
   "Working under mtt").
1. Work on a per-task branch, not on `main`.
2. **Test before code** (TDD: red → green → refactor). `make check` must be **green** before you commit.
3. Fanatically: **SOLID, DRY, KISS, clean architecture** (see "Principles").
4. Thin CLI layer; logic lives in `core`; storage sits behind a port (adapter) — never touch `.mtt/` directly.
5. Changing behavior? Update `DESIGN.md` and the affected `CLAUDE.md` files.

## Principles (non-negotiable)

We fanatically follow **SOLID, DRY, KISS, TDD, DDD, clean architecture** (hexagonal). Dependencies point
inward: `cli → core → port ← adapter`. Domain types and ports live in the public `pkg/mtt`; they know nothing
about the CLI, files, or YAML. `core` never imports `adapter/*`; adapters carry no business rules.

**DDD in practice here:** model the domain explicitly — closed vocabularies are **value objects** (e.g.
`StatusKind`), not bare strings/primitives; keep the domain **free of serialization/infrastructure** (no
yaml/json tags, no adapter-specific fields like `prefix` in `pkg/mtt` — adapters map via DTOs); **reference
across aggregates by identity** (names/IDs), never by pointer; **back-references are computed**, not stored.
The domain requires a **mandatory minimum** of fields and treats the rest as optional, so an external
provider can satisfy it (**provider-agnostic**).

Before you consider a task done — an explicit self-check (answer honestly):

- "Is this *really* clean architecture — or can I do cleaner? Where do the layers leak?"
- "Any duplication (**DRY**)? Any needless complexity (**KISS**)?"
- "Was the test written **before** the code (**TDD**)?"
- "Does each exported type/function have one responsibility (**SRP**)? Are the abstractions right?"
- "Is the **domain (DDD)** modeled explicitly — value objects over primitives, free of serialization/infra,
  references by identity, right mandatory-minimum vs optional?"

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
- In the YAML adapter, `.mtt/` is committed and is the source of truth. Task files are written only by
  mtt — don't hand-edit them. The repo's `.mtt/config.yaml` is the exception: it is hand-authored,
  reviewed like code (see "Working under mtt"), and guarded by `TestRepoDogfoodConfig`.
- IDs are **flat, per-prefix** (`e1`, `t17`) and independent of `title` **and of position** — re-parenting
  changes only the `parent` field, never the ID. (Hierarchy is stored in `parent`, computed for display.)
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
PR → squash. The roadmap and current target live in mtt itself (see "Working under mtt" below);
sessions/README.md keeps the narrative history. TASKS.md is frozen history since s009.

## Working under mtt (self-host)

Since **s009** this repo tracks its own work in a committed `.mtt/` (config + tasks). `TASKS.md` is frozen;
the live queue is mtt. Practical rules:

- **The backlog is in mtt.** `mtt roadmap` is the "what next?" view; `mtt list --tag backlog` is the
  backlog-only view; promote by `mtt tag rm <id> backlog`. A task is the unit of **product** change;
  sessions/phases (how *we* work) stay in `sessions/*.md` — they are not mtt tasks.
- **Two types (`mtt types` shows both flows).** `task` = design is OPEN (spec → plan → implement, each stage
  reviewed by an agent, spec/plan also by the human). `chore` = design is ALREADY FIXED elsewhere (a review
  finding, a recorded decision, docs sync) — implement → review → deliver. If a chore's diff turns out to
  contain design decisions, the reviewer declines it: cancel and recreate as a task.
- **Move by edge verb** (`mtt start/submit/approve/decline/deliver/cancel [<id>]`) or `mtt status`. The flow
  mechanizes the git context: `start` re-enters or creates `task/<id>` from main; `deliver` and `cancel`
  move your tree to main and write the terminal state there; `approved → decline` returns you to the task
  branch. Mid-flight resumption is a plain `git switch task/<id>` (start only fires from tbd).
- **Artifacts are id-keyed and committed early.** A task's spec/plan live at
  `docs/superpowers/specs|plans/<id>-<slug>.md` (the submit gates check exactly that); commit them as you
  go — nothing requires an uncommitted tree.
- **Delivery is verified.** The PR title starts with `<id>: ` (the repo squash setting propagates it to the
  squash subject); `mtt deliver` checks that trace on local main — pull first. Push the `approved` state
  commit before asking for the merge, or deliver will find a stale status.
- **Attribution is required** (`require: {who}`, every move, `--no-run` does not bypass): set `author:` in
  `.mtt/config.local.yaml` or `MTT_BY=<you>` before your first move.
- **Dangerous ops force who+why (t5).** A gate bypass (`--no-run`) and a destructive `rm --force` each demand
  **both** `--who` and `--why` (missing → exit 2), independent of the global `require` policy. `rm --force`
  writes an audit record to `.mtt/audit.log` (JSONL, committed, `merge=union`) **before** deleting — no
  destruction without a trail. A transition can also be marked critical in the config with a per-edge
  `require: {who, why}` (unioned with the global policy — tighten-only).
- **Commit `.mtt` with the branch.** Mid-flow moves (`start`/`submit`/`approve`/`decline`) rewrite
  `.mtt/tasks/*.yaml` on the task branch — commit them as you go (they ride the PR).
- **Two manual steps remain** (until post-persist actions land): after `deliver` and after `cancel`, run
  `git add .mtt && git commit` on main — the state write is otherwise uncommitted and would ride the next
  task's branch.
- **Config is code (SEC2).** Review `.mtt/config.yaml` diffs like a Makefile; a gate may invoke read-only
  `mtt` only (never an mtt transition). Gate commands are single-quoted YAML scalars. The committed config is
  guarded by `TestRepoDogfoodConfig` — keep it green.
