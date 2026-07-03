# 001 — Init & inspect

Status: planned   ·   Branch: `feat/s001-init-and-types`

## Target

Initialize a project and inspect its configured task types and flow. The first vertical slice through the
contract, the YAML adapter's config layer, and the CLI — the foundation everything else builds on.

## Scope

- **In:**
  - `mtt init [--template default|coding] [--force] [--name <name>]`
  - `mtt types [<type>]`
  - `pkg/mtt` contract: `Config`, `Type`, `Status` (+ `kind`), `Transition`, `Flow` — enough to load/validate.
  - YAML adapter: find the `.mtt/` root, write the default config, load + validate it, merge the optional
    gitignored `.mtt/config.local.yaml` overlay.
  - Config invariants: a default `task`; anchors `tbd → in_progress → done`; exactly one `initial`, ≥1 `terminal`.
- **Out (deferred):** tasks (`add`/`show`/`list`) → 002; capabilities / `mtt caps`; command gates; any
  adapter other than YAML.

## Acceptance (must pass)

- **User scenario:** in an empty dir, `mtt init` creates `.mtt/config.yaml`; `mtt types` prints
  `epic`/`task`/`subtask` with their statuses (kinds) and transitions. `mtt init --template coding`
  yields `feature`/`bugfix`/`refactor` with a gated per-type DoD, visible via `mtt types`.
- **e2e:** `testscript` `init.txt` — init → assert the config file + `types` output; `init --force`
  overwrites; `init` in an already-initialized dir errors without `--force`.
- Golden test for the generated default config (deterministic).
- `make check` green.

## Plan (refine at session start — test-first)

- [ ] `pkg/mtt`: `Config`/`Type`/`Status`(kind)/`Transition`/`Flow` types + `pkg/mtt/CLAUDE.md`
- [ ] `internal/adapter/yaml`: root discovery, default-config template(s) (`default`, `coding`), write,
      load, validate invariants, local-overlay merge + `internal/adapter/yaml/CLAUDE.md`
- [ ] `internal/core`: config load/validate usecase + `internal/core/CLAUDE.md`
- [ ] `internal/cli`: `init`, `types`
- [ ] golden config test + `testscript` `init` scenario

## Done (fill during/after the session)

—
