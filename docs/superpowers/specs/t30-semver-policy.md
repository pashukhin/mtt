# t30 — Adopt SemVer and embed it in the dev process

Status: spec (decision record). Type: task. Branch: `task/t30`.

## Context / problem

Today the version **mirrors the dev session**: `sN → 0.N.0-dev`, a point-session `sN.M → 0.N.M-dev`
(origin: `sessions/README.md:15-16`, echoed in `RELEASING.md`, `CHANGELOG.md`, `DESIGN.md`/`.ru`). The
literal `0.9.0-dev` lives in `internal/cli/root.go:16` and is hand-bumped; release binaries are ldflag-stamped
from the git tag by `make release` / `release.yml`.

Session-mirroring is wrong for an external release: (a) the "session" is an internal process artifact that is
meaningless to an adopter; (b) the hand-maintained literal drifts from the tag; (c) the session apparatus
itself is slated to retire as self-host lands (t31). The first external-ready release needs a real,
adopter-legible versioning contract.

Release framing (settled with the maintainer): **stay on the 0.x line** (do not cut 1.0), and make the
**git tag the single source of truth** (derive the version, stop committing it).

## Decisions

### D1 — Scheme: SemVer 2.0.0, pre-1.0 (`0.y.z`)

- Adopt SemVer; remain on **0.x** for now. Rationale: the CLI / config / flow surface is still moving (epics,
  arg-resolution grammar, boards, multiagent all parked) — 0.x honestly signals "young, may still change"
  without binding us to a MAJOR bump on every break.
- Pre-1.0 bump rules:
  - **MINOR** (`0.→Y←.0`): any new feature and/or any backward-incompatible change to the public surface
    (D2). Resets PATCH to 0.
  - **PATCH** (`0.y.→Z←`): backward-compatible bug fixes, security fixes, docs, internal-only refactors.
  - A breaking change is **never** silently shipped in a PATCH: it forces a MINOR and MUST carry a CHANGELOG
    `Changed`/`Removed` entry with a one-line migration note.
- Path to 1.0: cut `1.0.0` once the store schema + CLI contract are declared stable — a **separate future
  decision**; only the intent is recorded here, not the trigger.
- The session-mirror rule is retired entirely.

### D2 — Compat surface: what "backward-incompatible" means

SemVer governs this public contract:

- **CLI UX** — command names, flags, positional grammar, exit codes. Removing/renaming a command or flag, or
  changing an exit-code meaning, is breaking.
- **`--json` output** — field names and types. Removing/renaming/retyping a field is breaking; purely
  additive fields are compatible.
- **Store schema** — `.mtt/config.yaml` + task-file YAML on disk. A change that makes an existing committed
  store fail to load, or that requires a migration, is breaking. *Highest-stakes surface: adopters commit
  their store into their repo.*
- **`pkg/mtt` Go API** (public: domain types + ports) — pre-1.0 **best-effort**: changes are noted in the
  CHANGELOG but are **not** a hard compat promise until 1.0, so the ports can still be refined.
- Explicitly **not** covered: `internal/**`, gate-command output tails, `-v`/log formatting, timing, and any
  `git describe` build-metadata string.

### D3 — Source of truth = the git tag; the version is derived, not committed

- The annotated git tag **`vX.Y.Z` is the single source of truth**. No version number is hand-maintained in
  source.
- Runtime resolution chain (first hit wins):
  1. **ldflags-injected** value (release + explicit `make` builds): `-X …/internal/cli.version=<v>`.
  2. **`runtime/debug.ReadBuildInfo()`** main-module version — populated for
     `go install github.com/pashukhin/mtt/cmd/mtt@vX.Y.Z` (and pseudo-versions for `@latest`), so go-install
     users get a real version with no ldflags. Treat `""` / `"(devel)"` as absent and fall through.
  3. Fallback literal **`"dev"`**.
- `internal/cli/root.go`: default `var version = "dev"` (drop `0.9.0-dev` — the drift source). A small
  `resolveVersion()` implements the chain; **both** cobra's `Version:` field and the `mtt version` subcommand
  consume it (single path, no divergence).
- **Makefile**: default `VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null)` so a bare
  `make build` stamps a describe string (`v0.9.0-3-gabc123`, `-dirty` when the tree is uncommitted); an
  explicit `make release VERSION=vX.Y.Z` still wins (highest precedence via the existing `?=`/ldflags gate).
- **CI** (`release.yml`) is unchanged: it passes `VERSION=${GITHUB_REF_NAME}` (the tag).
- Before the first `v*` tag exists, `git describe --always` yields the short SHA and go-install yields the
  module pseudo-version — both honest.

### D4 — Bump discipline: CHANGELOG-driven, computed at release cut

- Keep the Keep-a-Changelog format. Every PR touching the public surface (D2) adds an entry to `[Unreleased]`
  under the right category (Added / Changed / Deprecated / Removed / Fixed / Security). **This is the per-task
  obligation.**
- At **release cut** (not per task) the accumulated `[Unreleased]` categories dictate the bump:
  - any `Added` / `Changed` / `Deprecated` / `Removed` → **MINOR** (pre-1.0 collapses feature and breaking to
    the same bump);
  - only `Fixed` / `Security` / docs / internal → **PATCH**.
- The bump is a **release-cut chore** (a RELEASING.md step), deliberately **not** a per-task flow step: tasks
  batch into one release, so per-task version moves would be noise. No new tooling is in scope — a "suggest
  the bump from `[Unreleased]`" helper is a possible later chore (YAGNI now).

### D5 — Doc reconciliation (living docs only; frozen session plans/specs untouched)

Replace the session-mirror rule and the hand-bump instruction in the **living** docs (dated
`docs/superpowers/plans|specs/*` artifacts are frozen history — left as-is):

- `sessions/README.md:15-16` — the origin statement. Stop asserting version == session; replace with a pointer
  to the semver policy (or drop, if the sessions apparatus is being retired under t31).
- `RELEASING.md:30-31` **and step 1** — the "bump the dev version in `root.go`" step becomes "no source edit;
  the tag is the version; pick the bump from `[Unreleased]` per D4".
- `CHANGELOG.md` — the session-mirror sentence in the header.
- `DESIGN.md:963` + `DESIGN.ru.md:979` — "Pre-1.0 versions mirror the session." → the semver one-liner + a
  pointer to RELEASING.md. EN is source of truth; keep `.ru` in sync (docs-language rule).
- The exhaustive EN+RU / README / CLI_REFERENCE sweep for stragglers is **t42**'s job (t42 depends on t30);
  t30 fixes the authoritative statements and the mechanism.

## Scope boundaries / cross-refs

- **In:** the policy (D1–D2, D4); the version-resolution code (`root.go` + `version.go` + `resolveVersion()` +
  tests); the Makefile `VERSION` default (D3); the living-doc reconciliation (D5).
- **Out:** `mtt version --json` consistency (**t45**); `mtt self-update`, which consumes this scheme (**t44**,
  blocked by t30); the exhaustive docs audit (**t42**, blocked by t30); actually cutting/tagging a release (a
  release-cut chore, not this task); deciding the 1.0 trigger.

## Acceptance criteria

1. No living doc asserts "version mirrors the session"; `RELEASING.md` describes the tag-as-SoT model and the
   bump-from-`[Unreleased]` process (D4), and states the D2 compat surface + D1 bump rules where a releaser /
   adopter will find them (RELEASING.md, with a short pointer from DESIGN.md's Releasing section).
2. `internal/cli/root.go` holds **no** hardcoded version number; `resolveVersion()` implements ldflags →
   build-info → `"dev"`, with unit tests exercising all three branches.
3. `make build` (no VERSION) stamps a `git describe` string; `make release VERSION=vX.Y.Z` stamps the tag;
   `mtt version` and `mtt --version` print the same resolved value; `make smoke` stays green.
4. `make check` is green.

## Testing approach

- Unit-test `resolveVersion()` over its inputs (ldflags value present; build-info version present;
  neither → `"dev"`). Parameterize the resolver over its two inputs rather than reading globals, so the
  branches are injectable.
- Update `TestVersionCommand` (`internal/cli/root_test.go`) to assert `mtt version` == the resolver's output
  instead of the raw `version` literal.
- The Makefile `git describe` path is covered end-to-end by the existing `make smoke` (installs, asserts a
  non-empty `mtt version`); no unit test for the shell substitution.
