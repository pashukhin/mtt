# 001 — Init & inspect

Status: planned   ·   Branch: `feat/s001-init-and-types`
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

- [ ] `pkg/mtt`: pure `Config`/`Type`/`Flow`/`Status`(`StatusKind`)/`Transition` + `Validate()` + helpers
      (`DefaultType`/`ChildrenIn`) + `pkg/mtt/CLAUDE.md`
- [ ] `internal/adapter/yaml`: root discovery, embedded `default`/`coding` templates (text/template,
      `{{.Name}}`), atomic write, DTO→domain load + overlay merge, adapter checks (prefix, one default)
      + `internal/adapter/yaml/CLAUDE.md`
- [ ] `internal/cli`: `init`, `types` (composition root; calls `Validate()`, formats output)
- [ ] golden config test (`default`/`coding`) + `testscript` `init` scenario
- `internal/core` is **deferred to session 002** (see spec §9) — no task usecases yet in 001

## Done (fill during/after the session)

—
