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
[../docs/superpowers/specs/2026-07-06-session-007-structured-commands-design.md](../docs/superpowers/specs/2026-07-06-session-007-structured-commands-design.md);
plan:
[../docs/superpowers/plans/2026-07-06-session-007-structured-commands.md](../docs/superpowers/plans/2026-07-06-session-007-structured-commands.md).

- [x] Brainstorm the open questions — resolved in the spec (four decisions locked; independent subagent review).
- [x] `pkg/mtt`: `Command` VO + `Valid()`; `Transition.Commands []Command`; `Config.Validate` on commands.
- [x] `adapter/yaml`: `ymlCommand` custom `UnmarshalYAML` (scalar|map) + duration parse; DTO mapping.
- [x] `core`: placeholder expansion (`text/template`, shape-safe fields only); `Runner.Run([]mtt.Command)`.
- [x] `adapter/exec`: per-command timeout with global fallback; update fake.
- [x] e2e `structured_commands.txt`; unit tests per the spec.
- [x] Docs: DESIGN.md/.ru, CLI_REFERENCE.md/.ru, CLAUDE.md ×5, model.go, TASKS.md, sessions/README.md,
      NEXT_SESSION.md; bump `0.6.7-dev` → `0.7.0-dev`.

## Done (fill during/after the session)

Shipped (all test-first, `make check` green), version `0.6.7-dev` → `0.7.0-dev`:

- **`pkg/mtt`**: the `Command` value object (`{Run string, Timeout time.Duration}`, `command.go`) — `Run` holds
  a **raw template** (domain is template-agnostic), `Timeout` an optional per-command override; `Valid()`
  (non-empty run, non-negative timeout) checked in `Config.Validate`. `Transition.Commands` is now `[]Command`.
- **`internal/adapter/yaml`**: `ymlCommand.UnmarshalYAML` accepts a bare scalar **or** a `{run, timeout}` map
  (back-compat), decoding the map branch into a string-`Timeout` alias then `time.ParseDuration` (bad duration →
  `Load` error; `toDomain` stays error-free). No `MarshalYAML` (config is never marshaled).
- **`internal/core`**: `expandCommands` (`expand.go`) renders each `Command.Run` via `text/template` over
  `cmdContext{ID, Type, From, To}` — a self-enforcing shape-safe whitelist; `Transitioner` expands before the
  gate using the **pre-move** status for `.From` (expansion error → plain error, exit 1, not `ErrBlocked`).
  `Runner.Run([]mtt.Command)` (Run expanded at the boundary).
- **`internal/adapter/exec`**: the `Runner` resolves the effective timeout per command
  (`cmd.Timeout` else the constructor global) — a tight per-command timeout fails fast independent of the
  global; `Check.Cmd` records the expanded command.
- **`internal/cli`**: `mtt types` renders `$ <run>` + `(timeout <d>)`; no other CLI wiring change (the runner
  still gets `settings.CommandTimeout` — now the fallback).
- **Tests**: unit — `Command.Valid`; `Config.Validate` rejects a bad command; `ymlCommand` scalar/map/bad-duration
  + mixed-list `toDomain`; `expandCommands` (substitution, all fields, unknown-field/malformed errors);
  `Transitioner` (expands, pre-move `.From`, unknown-placeholder aborts without change, `--no-run` skips
  expansion); exec per-command-timeout override + global fallback; `mtt types` timeout annotation. e2e —
  `structured_commands.txt` (`git checkout -b task/{{.ID}}` creates `task/t1` asserted via `git symbolic-ref`;
  a 100ms per-command timeout blocks a `sleep 1` gate under a 5m global; a bare-string edge still gates; `mtt
  types` shows the timeout; guarded by `[!exec:git] skip`).

**Deviations / notes:**
- `Command` ships without a smart constructor — a plain VO with `Valid()` checked in `Config.Validate` (the
  `StatusKind`/`CurrentAction` idiom), so `toDomain` needs no error path; the DTO's `UnmarshalYAML` parses the
  duration and carries the error.
- `rollback?` deliberately **not** added — s008 adds it as a new optional field (additive).
- e2e uses `sleep 1` (not `sleep 5`): the per-command timeout kills at 100ms, but `exec.CommandContext` blocks
  `Run()` until the orphaned `sleep` closes the inherited output pipe (same as the existing `TestRunTimeout`),
  so a shorter sleep keeps the e2e ~1s. The timeout still fires (the move blocks, exit 3).

**Next:** s008 rollback/compensation (reverse-order compensating commands; the `Command` VO gains `Rollback`).
