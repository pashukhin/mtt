# t4 — coding-template demo: a runnable, tested end-to-end showcase

- **Task:** t4 (unblocked by t23)
- **Status:** spec (speccing)
- **Tags:** demo, release

## Goal

Show mtt's core pitch — *a fuse between an agent and the word "done"* — made concrete with the shipped
`coding` template. The `coding` template (`feature`/`bugfix`/`refactor`) enforces a **gated Definition of
Done** per type; today DESIGN mentions it but nothing demonstrates it. t4 ships a **runnable, tested** demo
that walks the coding flow end-to-end, with the gates actually firing (blocking, then passing).

## Scope

- A **runnable + tested** demo (not a narrated-only doc): a readable shell script + a Go test that runs it in
  CI so it cannot rot.
- Showcase **all three** coding types (feature, bugfix, refactor), each highlighting a **distinct gate story**.
- **Bilingual** demo README (EN + RU).
- The `coding` template itself is **NOT modified** (t23 confirmed it coherent). If the demo surfaces a rough
  edge, file it separately rather than changing the template under a demo task.
- No engine/domain change.

## Artifacts

| File | Responsibility |
|---|---|
| `demo/coding-flow.sh` | The narrated, runnable walkthrough. Uses `${MTT_BIN:-mtt}` so humans run it with an installed `mtt` while the test injects a freshly-built binary. |
| `demo/coding_flow_test.go` | Go test (`package demo`): builds `mtt`, runs the script in a temp dir with `MTT_BIN` set, asserts exit 0 + key markers. Runs under `make check` (`go test ./...`). |
| `demo/doc.go` | `package demo` doc comment — a non-test Go file so `go build ./...` / `go vet ./...` don't choke on a test-only dir. |
| `demo/README.md` / `demo/README.ru.md` | What the demo shows + how to run it (`make demo` or `./demo/coding-flow.sh`). EN+RU in lockstep. |
| `Makefile` (repo) | New `demo:` target that runs the script visibly (narration on) for humans. |
| `README.md` / `README.ru.md` | A one-line pointer to the demo from the `## Docs` section (EN+RU). |

## Mechanism

- **`${MTT_BIN:-mtt}`** — the script calls mtt via this indirection. Humans: defaults to `mtt` on PATH.
  Test: builds `mtt` (from repo root, `go build -o <tmp>/mtt ./cmd/mtt`) and runs the script with
  `MTT_BIN=<tmp>/mtt`.
- **The scaffold** (created by the script inside a fresh temp dir): `git init` (needed for refactor's
  `git diff -- pkg/` gate), a `go.mod`, a `Makefile` whose `lint` = "fail if `gofmt -l` lists anything" and
  `test` = `go test ./...`, and a tiny package under `pkg/` with a source file + test. Then
  `mtt init --template coding`. The scaffold's Makefile is what the coding template's `make lint`/`make test`
  gate commands actually run — so the gates need only `go`, `make`, `gofmt`, `git` (all standard), **not**
  golangci-lint.

## The three gate stories (the heart of the demo)

Grounded in the shipped `coding` template's transitions:

**feature** — *DoD = a green gate.*
`tbd → in_progress` ("create a feature branch", no gate) → `in_progress → done` (gate: `make lint`,
`make test`). The demo adds a feature with a **still-failing test**, **attempts `done` → the gate BLOCKS
(exit 3, task stays)**; then makes the test pass → `done` passes. (Clean contrast with bugfix below: a
feature needs the test **green to finish**; a bugfix needs it **red to start**.)

**bugfix (the hero)** — *red-first, enforced.*
`tbd → in_progress` (gate: `! make test` — the test must currently FAIL) → `in_progress → done` (gate:
`make lint`, `make test`). The demo adds a bugfix and **attempts `in_progress` before writing a failing
test → `! make test` fails → the gate BLOCKS**; then writes a failing test reproducing the bug → `in_progress`
passes → fixes the code → `done` passes. This is the money shot: mtt refuses to let you "start" a bugfix
without a reproducing test.

**refactor** — *behavior-preserving, no public-API change.*
`tbd → in_progress` ("create a branch", no gate) → `in_progress → done` (gate: `git diff --exit-code -- pkg/`,
`make lint`, `make test`). The demo makes a **public** change under `pkg/` and attempts `done` → the
`git diff -- pkg/` gate **BLOCKS**; then reverts the pkg/ surface (keeping only an internal change) → `done`
passes.

Each segment prints a short narration line (e.g. `▶ bugfix: starting without a failing test → expect BLOCK`).
The demo does **not** hardcode expected gate output — the gates genuinely execute; the narration explains what
just happened.

## Testing

`demo/coding_flow_test.go`:
1. Build `mtt` to a temp path.
2. Run `demo/coding-flow.sh` in a fresh temp dir with `MTT_BIN` set (and `MTT_BY=demo` for safety, though the
   coding template declares no `require`).
3. Assert the script exits 0 and stdout contains the expected markers — at minimum: each type reaches `done`,
   and each of the three deliberate BLOCK attempts was actually blocked (e.g. a `blocked` / exit-3 marker the
   script echoes after each expected-fail step).

The test is part of `make check` (via `go test ./...`). It builds mtt and shells out to `make`/`go`/`git`/
`gofmt`; a `//go:build !windows` constraint (or a `bash`-availability skip) keeps it honest on the supported
platform (linux) without breaking others.

## Non-goals / deferred

- No change to the `coding` template (or any config/engine). Rough edges → a separate task.
- No asciinema/GIF recording (no infra); the script's narration is the "recording".
- Not a substitute for the CLI reference — it's a showcase, linked from README `## Docs`.

## Risks

- **Toolchain deps in CI.** The demo runs `go`, `make`, `git`, `gofmt` — all present in this repo's CI
  (already used by `make check`). No new dependency (notably **not** golangci-lint — the scaffold's `lint` is
  `gofmt`-based).
- **Nested `go test`.** The wrapper `go test` shells out to a scaffold `go test`; runs in an isolated temp
  module outside the repo, so no module/cache interference. Slower but bounded (a one-file package).
- **Portability.** Bash + POSIX tools; guard the Go test to the supported platform so a non-bash CI doesn't
  red-fail.
- **Demo-rot.** Mitigated by design — the Go test IS the guarantee; a template/gate change that breaks the
  flow breaks `make check`.

## Docs sync

- `README.md` / `README.ru.md` `## Docs` — add the demo pointer (EN+RU).
- `DESIGN.md` / `DESIGN.ru.md` `:679` (the `mtt init --template coding` paragraph) — add a "see `demo/`"
  pointer (EN+RU).
- `demo/` is at the repo root, not under `internal/`, so the per-package `CLAUDE.md` rule does not apply; the
  bilingual `demo/README.md` provides orientation.
