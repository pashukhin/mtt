# 007 — Structured commands (placeholders + per-command timeout)

Status: in progress   ·   Branch: `feat/s007-structured-commands`

## Target

Let the **agent work in task terms**: evolve a transition's `commands` from bare strings into a `Command`
value object `{run, timeout?}` with (1) **placeholder expansion** on `run` (`.ID`/`.Type`/`.From`/`.To` —
shape-safe fields) and (2) a **per-command timeout** overriding the global `command_timeout`. Killer example:
`git checkout -b task/{{.ID}}` on the `tbd → in_progress` edge, and a fast command that fails fast on its own
tight timeout.

## Scope

- **In:**
  - `pkg/mtt`: `Transition.Commands` `[]string` → `[]Command`, a value object `Command{Run, Timeout}` (a
    **domain-shape change**). Back-compat: a bare YAML string ⇒ `{run: …}`.
  - Placeholder expansion of `run` against shape-safe fields (`.ID`/`.Type`/`.From`/`.To`) — **never** free
    text.
  - Per-command `timeout` overriding the adapter-level global `command_timeout` (fallback when unset).
  - `adapter/yaml`: custom `UnmarshalYAML` on the command DTO (scalar **or** map); duration parse.
  - `core`/`adapter/exec`: expansion + Runner honoring per-command timeout.
  - Version `0.6.7-dev` → `0.7.0-dev`.
- **Out (PARKED, do not build):** rollback/compensation (s008 — the `Command` shape leaves additive room for
  a `rollback?` field); `advance`/`start`/`done`/`cancel` + modes; node-level status actions; roles-on-edges.
  The `coding` template's fully-powered task-aware DoD demo stays **e5_t6**.

## Open questions (resolve in brainstorm — do not decide here)

1. **Per-command timeout home:** domain `Command` VO (`pkg/mtt`) vs adapter-level? Reconcile with the s006
   lesson "execution policy (`command_timeout`) rides the config-layer `Settings`, NOT `pkg/mtt`".
2. **Placeholder expansion locus:** `core.Transitioner` (has task+edge) vs the exec adapter. Core-expansion
   keeps the adapter dumb; then `pkg/mtt` must stay template-agnostic (stores the template, core expands
   before `runner.Run`).
3. **Injection policy:** whitelist shape-safe fields (id/type/status — no spaces/meta) vs shell-quote
   arbitrary ones (title). Never interpolate raw free text unquoted.
4. **Runner timeout plumbing:** `NewRunner` takes one global timeout today. How to thread per-command timeout
   — `Command` carries it, Runner honors per-cmd (fallback to global)?

## Acceptance (must pass)

- **User scenario:** `mtt init` → a gate config with `git checkout -b task/{{.ID}}` on `tbd → in_progress`
  and a per-command `timeout` on a slow command → `mtt add A` (t1) → `mtt in_progress t1` creates branch
  `task/t1`; a command that overruns its per-command timeout fails fast (blocked, exit 3), independent of the
  larger global `command_timeout`.
- **e2e:** `testscript` scenario `structured_commands.txt` covering the above.
- `make check` green.

## Plan (refine at session start — test-first; brainstorm → writing-plans)

Authoritative spec:
[../docs/superpowers/specs/2026-07-06-session-007-structured-commands-design.md](../docs/superpowers/specs/2026-07-06-session-007-structured-commands-design.md).

- [ ] Brainstorm the open questions — resolve in the spec.
- [ ] `pkg/mtt`: `Command` VO + `Valid()`; `Transition.Commands []Command`; `Config.Validate` on commands.
- [ ] `adapter/yaml`: `ymlCommand` custom `UnmarshalYAML` (scalar|map) + duration parse; DTO mapping.
- [ ] `core`: placeholder expansion (`text/template`, shape-safe fields only); `Runner.Run([]mtt.Command)`.
- [ ] `adapter/exec`: per-command timeout with global fallback; update fake.
- [ ] e2e `structured_commands.txt`; unit tests per the spec.
- [ ] Docs: DESIGN.md/.ru, CLI_REFERENCE.md/.ru, CLAUDE.md ×N, model.go, TASKS.md, sessions/README.md,
      NEXT_SESSION.md; bump `0.6.7-dev` → `0.7.0-dev`.

## Done (fill during/after the session)

<pending>
