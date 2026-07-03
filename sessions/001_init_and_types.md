# 001 — Init & inspect

Status: done   ·   Branch: `feat/s001-init-and-types`
Design (authoritative): [../docs/superpowers/specs/2026-07-03-session-001-init-and-types-design.md](../docs/superpowers/specs/2026-07-03-session-001-init-and-types-design.md)

## Target

Initialize a project and inspect its configured task types and flow. The first vertical slice through the
contract, the YAML adapter's config layer, and the CLI — the foundation everything else builds on.

## Scope

- **In:**
  - `mtt init [--template default|coding] [--force] [--name <name>]`
  - `mtt types [<type>]`
  - `pkg/mtt` **pure** contract: `Config`, `Type` (`name/description/parents/default`), `Flow`, `Status`
    (`name`/`kind` as a `StatusKind` value object/`description`), `Transition` — enough to load/validate.
  - YAML adapter: find the `.mtt/` root, write the default config, load via DTO→domain mapping (adapter
    holds `prefix`) + validate, merge the optional gitignored `.mtt/config.local.yaml` overlay.
  - Config invariants (**structural, name-agnostic**): kind↔topology; ≥1 of each kind (no 2-status flow;
    multiple initials ok); per-flow status identity, no cross-flow transitions; at-most-one `default` at the
    domain / exactly-one at the YAML provider; prefix present+unique (adapter). No literal type/status names.
- **Out (deferred):** tasks (`add`/`show`/`list`) → 002; capabilities / `mtt caps`; command gates; any
  adapter other than YAML.

## Acceptance (must pass)

- **User scenario:** in an empty dir, `mtt init` creates `.mtt/config.yaml`; `mtt types` prints
  `epic`/`task`/`subtask` with their statuses (kinds) and transitions (names come from the template, not
  asserted as fixed anchors). `mtt init --template coding` yields `feature`/`bugfix`/`refactor` with a gated
  per-type DoD, visible via `mtt types`.
- **e2e:** `testscript` `init.txt` — init → assert the config file + `types` output; `init --force`
  overwrites; `init` in an already-initialized dir errors without `--force`.
- Golden test for the generated default config (deterministic).
- `make check` green.

## Plan (refine at session start — test-first)

- [x] `pkg/mtt`: pure `Config`/`Type`/`Flow`/`Status`(`StatusKind`)/`Transition` + `Validate()` + helpers
      (`DefaultType`/`ChildrenIn`) + `pkg/mtt/CLAUDE.md`
- [x] `internal/adapter/yaml`: root discovery, embedded `default`/`coding` templates (text/template,
      `{{.Name}}`), atomic write, DTO→domain load + overlay merge, adapter checks (prefix, one default)
      + `internal/adapter/yaml/CLAUDE.md`
- [x] `internal/cli`: `init`, `types` (composition root; calls `Validate()`, formats output)
- [x] golden config test (`default`/`coding`) + `testscript` `init` scenario
- `internal/core` is **deferred to session 002** (see spec §9) — no task usecases yet in 001

## Done (fill during/after the session)

Shipped `mtt init [--template default|coding] [--force] [--name <name>]` and `mtt types [<type>]`,
composed in `internal/cli` as a thin composition root (parses flags, wires the YAML adapter, calls
`Config.Validate()`, formats blocks to `cmd.OutOrStdout()`).

Packages created:
- `pkg/mtt` — the pure, provider-agnostic contract: `Config`/`Type`/`Flow`/`Status`/`Transition`,
  the `StatusKind` value object (`initial`/`active`/`terminal`), structural `Config.Validate()`
  (kind↔topology, ≥1 of each kind, per-flow status identity, at-most-one `default`), and
  `DefaultType`/`ChildrenIn` helpers.
- `internal/adapter/yaml` — `FindRoot` (walk-up discovery like git), embedded `default`/`coding`
  `text/template` starters, atomic `Init` (temp+rename, refuses overwrite without `--force`), and
  `Load` (DTO→domain mapping, gitignored `config.local.yaml` overlay merge, provider-only checks:
  prefix present+unique, exactly-one `default`).
- `internal/cli` — `init`/`types` commands plus a private block formatter (`formatTypes`).

Key decisions:
- **Config-as-data**: no literal type/status names anywhere outside the two embedded templates; the
  domain and CLI are name-agnostic and driven entirely by what's in `config.yaml`.
- **DTO↔domain mapping** lives only in the adapter (`toDomain`); `pkg/mtt` never imports YAML. The
  adapter is also the sole owner of `prefix` (an ID-encoding concern, not a domain concept).
- Command output goes to stdout via `cmd.OutOrStdout()` (not `fmt.Println`), keeping commands
  testable and error reporting centralized in `Execute()` (stderr only, on failure).
- `internal/core` stayed deferred to session 002 as planned — 001 has no task usecases, only the
  config/inspect vertical slice.

Test coverage: unit tests per package (`pkg/mtt`, `internal/adapter/yaml`, `internal/cli`), a golden
test for the generated `default`/`coding` configs (`internal/adapter/yaml/testdata/golden/`), and a
`testscript` e2e (`internal/cli/testdata/scripts/init.txt`) driving the real `mtt` binary through
init → types → filter → re-init-without-force error → `--force --template coding` → types-outside-a-
project error. `make check` is green (fmt, vet, lint, `go test -race -cover ./...`, build).

Deviations from plan: none functional. `go get github.com/rogpeppe/go-internal/testscript@latest`
initially pulled a version requiring `go >= 1.25`, which this environment's toolchain (go1.23.1, no
`covdata` in its auto-downloaded 1.25 module toolchain) can't run cleanly through `-race -cover`;
pinned `go-internal@v1.14.1` instead (same `testscript.Main`/`Run` API, requires only `go 1.23`) to
keep `go.mod`'s `go 1.23.1` directive unchanged. `script_test.go`'s `TestMain` uses `testscript.Main`
(not the deprecated `RunMain` from the task brief's snippet) to keep `golangci-lint`'s `staticcheck`
check clean.
