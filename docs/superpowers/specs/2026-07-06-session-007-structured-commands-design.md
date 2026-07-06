# Session 007 — Structured commands (design spec)

Date: 2026-07-06 · Branch: `feat/s007-structured-commands` · Version bump: `0.6.7-dev → 0.7.0-dev`

Authoritative design for s007. Prose in [DESIGN.md](../../../DESIGN.md) stays the source of truth; this spec
is the resolved decision record the plan and implementation follow.

## Goal

Make the **agent work in task terms**: a transition can create the task's branch and run per-task gates
without the agent stitching shell together. Two capabilities, one shape change:

1. **Placeholder expansion** on a command's `run` — `.ID` / `.Type` / `.From` / `.To` (shape-safe fields only)
   — so `git checkout -b task/{{.ID}}` on `tbd → in_progress` creates `task/t1`.
2. A **per-command timeout** that overrides the global `command_timeout` — so a fast command fails fast on its
   own tight bound instead of waiting out the project-wide default.

Both converge on evolving `Transition.Commands` from `[]string` into a value object
`Command{Run, Timeout}` — an additive, back-compatible **domain-shape change** in `pkg/mtt` (a bare YAML
string ⇒ `{run: …}`).

## Architecture (resolved)

`cli → core → port ← adapter`. Everything typed (`mtt.TaskID`/`TypeName`/`StatusName`); string conversion
only at the cli/adapter boundary. Four open questions were brainstormed and locked:

- **Q1 — per-command timeout lives in the domain `Command` VO (`pkg/mtt`).** The reconciliation with the s006
  lesson ("execution policy `command_timeout` rides the adapter `Settings`, not `pkg/mtt`") rests on a real
  distinction: the **global** `command_timeout` is an execution-policy knob of the runner, applied to *every*
  command regardless of flow authoring (an external tracker runs none) → stays `yaml.Settings.CommandTimeout`,
  untouched. The **per-command** timeout is an authored property of *one command on one edge*, inseparable
  from its `run` — and `run` already lives in the domain (`Transition.Commands` is in `pkg/mtt` even though
  commands are "our local gate augmentation, absent from external trackers"). Splitting the VO across layers
  (`run`=domain, `timeout`=adapter) is worse than keeping the whole VO in the domain. It is optional and
  provider-agnostic (an external provider sets neither commands nor timeouts; the mandatory minimum is
  unaffected).
- **Q2 — placeholder expansion happens in `core.Transitioner`.** Core holds the task + edge (id/type/from/to),
  so it can produce concrete strings; the exec adapter stays **dumb** (receives already-expanded commands).
  The consequence, accepted: **`pkg/mtt` stays template-agnostic** — `Command.Run` stores the raw template;
  core expands it before `runner.Run`. Expanding in the adapter would require threading task data into exec
  (a smart adapter, a layer leak), and the injection policy (Q3) is policy — it belongs in core.
- **Q3 — injection defense is a structural whitelist, no shell-quoting.** Expansion uses `text/template` over
  a data struct exposing **only** `.ID`/`.Type`/`.From`/`.To`. Free-text fields (`title`/`description`) are
  **not** exposed, so there is no injection vector; a stray `{{.Title}}` in a config errors at expansion
  (fail-fast — the whitelist self-enforces through the struct's shape). The four exposed values are structural
  identifiers (an adapter-minted id, config-defined type/status names) and commands are **trusted project
  config** (Makefile-equivalent), so raw interpolation is correct; shell-quoting would complicate expansion
  and surprise (quotes appearing in commands) for little gain. If a free-text field is ever exposed, it MUST
  be shell-quoted (documented seam). Template knowledge lives in **core**, never in `pkg/mtt`.
- **Q4 — `core.Runner.Run([]mtt.Command)` (was `[]string`); the per-command-vs-global resolution stays in the
  adapter.** Core expands and passes `mtt.Command`s (with `Run` already expanded, `Timeout` copied). The exec
  `Runner` keeps its constructor global timeout as the **fallback**: per command, effective =
  `cmd.Timeout > 0 ? cmd.Timeout : r.timeout`. This keeps the global default (execution policy) in the
  adapter — core never learns it. The runner boundary **reuses `mtt.Command`** (one shape, DRY/KISS) with a
  doc note that `Run` is expanded at that boundary; a parallel `core.ResolvedCommand` was rejected as a
  near-duplicate that does not yet earn its keep.

## 1. `pkg/mtt` — the domain-shape change

### 1a. `Command` value object (`command.go`)

```go
// Command is one gate step: a shell command (Run) with an optional per-command
// timeout overriding the adapter's global command_timeout (zero = use the global).
// Run holds a raw template (e.g. "git checkout -b task/{{.ID}}"); the domain does
// not interpret it — core expands it before the runner runs it.
type Command struct {
    Run     string
    Timeout time.Duration
}

// Valid reports whether the command is well-formed: a non-empty Run and a
// non-negative Timeout. (Mirrors the StatusKind/CurrentAction Valid() idiom.)
func (c Command) Valid() bool { return c.Run != "" && c.Timeout >= 0 }
```

- `pkg/mtt` already imports `time` (`Task.Created/Updated`), so `time.Duration` adds no new import concern.
  On disk the timeout is a duration string (`30s`); the adapter parses it (§2). Zero = "unset, use the
  global."
- No smart constructor. Following the s006.7 lesson (`CurrentAction`), the VO is a plain struct with
  `Valid()` checked in `Config.Validate` — **no** `toDomain` signature churn.

### 1b. `Transition.Commands` type change

- `Transition.Commands` becomes `[]Command` (was `[]string`); field order unchanged (before `Current`). This
  is the deliberate, single domain-shape slice this session takes.

### 1c. `Config.Validate` — validate commands (`validate.go`)

- In the existing per-transition loop, for each `Command`: if `!c.Valid()` → append a clear error (empty
  `run`, or a negative `timeout`). Structural, name-agnostic, consistent with the existing validation style.

### `rollback?` — NOT added now

The eventual VO shape is `{run, timeout?, rollback?}` (DESIGN seam), but `rollback` belongs to **s008** and is
added then as a new optional field — trivially additive. YAGNI: no unused field now.

## 2. `internal/adapter/yaml` — back-compat DTO (`dto.go`)

### 2a. `ymlCommand` with custom `UnmarshalYAML` (scalar or map)

```go
type ymlCommand struct {
    Run     string
    Timeout time.Duration // parsed from the on-disk "timeout" string (0 = unset)
}

func (yc *ymlCommand) UnmarshalYAML(value *yaml.Node) error {
    // Scalar node: a bare command string ⇒ {Run: value, Timeout: 0}  (back-compat).
    // Mapping node: decode into a LOCAL alias whose Timeout is a STRING
    //   (type raw struct { Run string `yaml:"run"`; Timeout string `yaml:"timeout"` }),
    //   then time.ParseDuration(raw.Timeout) (empty ⇒ 0) into yc.Timeout. A bad
    //   duration returns an error → surfaces at Load.
}
```

- **Decode the map into a string-Timeout alias, never back into `ymlCommand`.** yaml.v3 cannot unmarshal a
  string like `30s` into a `time.Duration` (an int64), and decoding the mapping node back into `ymlCommand`
  would recurse into `UnmarshalYAML` infinitely. So the map branch decodes into a private `raw` struct with a
  `Timeout string`, then `time.ParseDuration` fills `yc.Timeout` — mirroring `parseCommandTimeout` (load.go)
  for the global one.

- Back-compat is at the **element** level: `commands: ["make lint", "make test"]` decodes each scalar element
  through `UnmarshalYAML` → `{Run: "make lint"}`. A map element `{run: "...", timeout: 30s}` decodes to
  `{Run, Timeout}`. Mixed lists work. Absent `commands` ⇒ nil.
- **The scalar form is a serialization convenience only — both forms collapse to the same `mtt.Command` at
  this boundary.** A bare string is the degenerate structured command: only `Run` is set; everything else
  takes its default — `Timeout == 0` means **fall back** to the global `command_timeout` (resolved in the exec
  runner, §4), *not* a zero timeout; a future `Rollback` (s008) defaults to empty (no compensation). Nothing
  above the adapter branches on string-vs-map: `core`, `exec`, history, and `mtt types` see only `[]Command`,
  and placeholder expansion applies uniformly to a scalar command's `Run` too. So the full form is never
  mandatory — write a bare string whenever there is no per-command timeout (or later, rollback).
- Parsing the duration in `UnmarshalYAML` (which already must exist for scalar-or-map) means the error
  surfaces at YAML-unmarshal time inside `Load` — the same layer as the global `parseCommandTimeout` — and
  keeps `toDomain` error-free.
- **No `MarshalYAML` needed.** Verified: the adapter never marshals `Config` (config is read via `Load` and
  written only as rendered template *text* by `Init`; only `Task` and the `config.local` node are marshaled).

### 2b. DTO wiring

- `ymlTransition.Commands` becomes `[]ymlCommand` (was `[]string`).
- `toDomain` maps each `ymlCommand → mtt.Command{Run, Timeout}` — a **pure copy**, no error path, no
  signature change (the duration was already parsed in `UnmarshalYAML`).

### 2c. Templates — unchanged

- `default`/`coding` templates are **not** modified in s007 (the default ships no commands by design; the
  killer demo is exercised via a swapped e2e config, the s006 pattern). The fully task-aware `coding` demo
  (branch creation + gated DoD via placeholders) remains **e5_t6**. Existing `coding.yaml` commands
  (scalar lists) keep parsing unchanged through the new element unmarshal.

## 3. `internal/core` — expansion + Runner signature

### 3a. `Runner` port (`runner.go`)

- `Run(commands []mtt.Command) ([]mtt.Check, error)` (was `Run([]string)`). Doc note: at this boundary each
  command's `Run` is **already expanded** by core; the runner only runs and reports. A non-zero exit is still
  DATA (a `Check`); the error is still operational (launch/timeout).

### 3b. Placeholder expansion (`core.Transitioner`)

- New pure helper (in `transition.go`), e.g.
  `expandCommands(cmds []mtt.Command, ctx cmdContext) ([]mtt.Command, error)`:
  - `cmdContext struct { ID, Type, From, To string }` — the **only** exposed fields (the structural whitelist).
  - For each command, run `Run` through `text/template` (`Parse` + `Execute`) against `ctx`; the result is a
    new `mtt.Command{Run: expanded, Timeout: cmd.Timeout}`. An empty command list expands to an empty list.
  - The whitelist **self-enforces**: `text/template` errors on a struct-field access to an unexposed name, so
    `{{.Title}}` (or a typo) is a template error at `Execute` — no free-text field can be reached. A malformed
    template (`{{`) errors at `Parse`. Either aborts expansion.
- In `Transition`, after the edge is found and attribution is checked, **before** the gate: build
  `ctx` from the task and the *pre-move* status —
  `cmdContext{ID: string(t.ID), Type: string(t.Type), From: string(from), To: string(to)}` (where
  `from := t.Status` is already captured before `t.Status = to`) — expand `edge.Commands`, then
  `runner.Run(expanded)`. An expansion error aborts the transition (task unchanged, no history) as an
  ordinary error (exit 1) — it is a config/authoring fault, **not** `ErrBlocked` (a gate never ran). Under
  `NoRun`, expansion is skipped along with the gate (nothing runs).

### 3c. Fake runner (core tests)

- The in-test fake's `Run` signature updates to `[]mtt.Command`; tests assert it received the **expanded**
  commands (with substituted ids) and the copied per-command timeouts.

## 4. `internal/adapter/exec` — per-command timeout (`exec.go`)

- `Run(commands []mtt.Command)`: per command, `effective := cmd.Timeout; if effective <= 0 { effective = r.timeout }`
  (global fallback), then `context.WithTimeout(ctx, effective)`. The constructor `NewRunner(dir, timeout,
  progress, cmdOut)` is **unchanged** — `timeout` is now explicitly the fallback.
- `Check{Cmd: cmd.Run, Exit: …}` — records the **expanded** command (truthful audit: `git checkout -b task/t1`
  appears in history, not the template). Progress lines (`▶`/`✓`/`✗`) use `cmd.Run`. Timeout-message text
  reports the effective duration used.

## 5. `internal/cli` — `mtt types` rendering (`types.go`)

- The single ripple consumer: `types.go` iterates `tr.Commands` printing `$ %s`. Update to `c.Run`; when
  `c.Timeout > 0`, append ` (timeout <d>)` so the new capability is visible in `mtt types`.
- `runTransition` / `applyCurrent` are otherwise untouched — the runner is still constructed with
  `exec.NewRunner(root, settings.CommandTimeout, …)`, now the per-command fallback.

## Error / exit taxonomy (unchanged codes)

No new exit codes. A template-expansion error is an ordinary error (exit 1) — a malformed command is a config
fault. Gate outcomes keep s006/s006.5 codes (3 blocked / 6 invalid / 2 attribution). A per-command timeout
manifests as an operational runner error → `ErrBlocked` (exit 3), exactly like the global timeout does today.

## Testing (strict test-first)

Unit:
- `pkg/mtt`: `Command.Valid` (empty `Run` → false; negative `Timeout` → false; ok otherwise);
  `Config.Validate` rejects a transition carrying an invalid command.
- `adapter/yaml`: `ymlCommand.UnmarshalYAML` — a scalar element → `{Run, Timeout:0}`; a map element
  `{run, timeout: 30s}` → parsed duration; a bad duration (`timeout: "banana"`) → error; a mixed list;
  `toDomain` round-trips `Transition.Commands` including a per-command timeout.
- `core`: `expandCommands` substitutes `.ID/.Type/.From/.To`, copies `Timeout`, errors on `{{.Title}}` and on
  a malformed template, and uses the **pre-move** status for `.From`; `Transitioner` (fake Runner) passes the
  **expanded** commands to the runner and gates as before; `NoRun` skips expansion+gate.
- `adapter/exec`: a command that overruns a **tight per-command** timeout fails fast (operational error)
  independent of a larger global; a command with no per-command timeout falls back to the global; `Check.Cmd`
  holds the expanded string. (Uses `sleep`/`true`/`false`; the Windows branch stays documented, not
  CI-tested.)
- `cli`: `mtt types` renders a command's `run` and its per-command `timeout` annotation.

e2e `testscript` `structured_commands.txt` (swapped `-- gated.yaml --` config `cp`'d over `.mtt/config.yaml`,
the s006 pattern; a single root type so `mtt add A --no-parent` is unnecessary — mirror the working s006/006.7
configs). Guard the script with `[!exec:git] skip` (the one script that shells out to `git`; testscript
propagates the host `PATH`, and the work dir is not a git repo by default):
- `tbd → in_progress` carries `commands: ["git checkout -b task/{{.ID}}"]`; the script `exec git init`s the
  work dir so the branch command works. `mtt add A` (t1) → `mtt in_progress t1 --who a --why w` → the gate
  runs and creates the branch. **Assert via `exec git symbolic-ref --short HEAD` → `stdout 'task/t1'`**, NOT
  `git branch --list` — on a fresh `git init` with no commits the new branch is *unborn*, so `git branch
  --list task/t1` prints nothing (empirically verified); `symbolic-ref` reports the current branch and needs
  no user config. The transition is applied and history shows the **expanded** command (`git checkout -b
  task/t1`).
- A second edge (or a second task) carries a command with a **tight** per-command `timeout` that overruns
  (e.g. `{run: "sleep 2", timeout: 100ms}`) while the global `command_timeout` is large (e.g. `5m`): the move
  is **blocked**, `! mtt … ` asserts non-zero (exit 3), the task stays put. (Proves per-command fails fast,
  independent of the global.)
- Back-compat arm: a bare-string command edge still gates as in s006 (unchanged behavior).

`make check` green.

## Docs to update

DESIGN.md/.ru ("Flow: executable transitions" + the "Seam (deferred): structured commands" note — flip from
*deferred* to *shipped s007*, record the four resolved decisions; the placeholder/injection policy; keep the
`rollback?`/node-actions seams as still-deferred); CLI_REFERENCE.md/.ru (`command_timeout` note gains the
per-command override; document the `commands` structured form `{run, timeout}` and the placeholder whitelist;
`mtt types` shows per-command timeout); CLAUDE.md ×N (pkg/mtt: the `Command` VO; adapter/yaml: `ymlCommand`
unmarshal; core: expansion + Runner signature; adapter/exec: per-command timeout; cli: `types` render);
`docs/architecture/model.go` (`Command` VO, `Transition.Commands []Command`, `Runner.Run([]Command)`,
`ResolvedEdge.Commands`); TASKS.md (e4_t9 tick + any think-items); sessions/README.md (007 ✅);
NEXT_SESSION.md; bump `0.6.7-dev → 0.7.0-dev`.

## Backlog / think (capture, do not build)

- **Rollback / compensation (s008)** — the `Command` VO gains an additive `Rollback` field; reverse-order
  compensation on a failed pipeline / atomic abort. The executor's abort path is the hook.
- **Fully task-aware `coding` template demo (e5_t6)** — branch creation on take-into-work + gated DoD via
  placeholders, as the ready-made enforcement demo. Now unblocked by structured commands.
- **More placeholders** — `.Title`/`.Description` would need shell-quoting (the documented seam); other
  context (parent id, timestamps) only if a real need appears. Not now.
- **Config-load-time template validation** — today a malformed command template surfaces at transition time,
  not at `Load`/`Config.Validate` (which stays template-agnostic per Q2). A pre-flight `text/template` parse
  check could live in a core/cli validation pass if early feedback is wanted. Deferred.
- **Scale/complexity** — the s006.7 stress-test think-item notes expansion adds a per-command `text/template`
  Parse+Execute; fold into that audit (small/sparse, not a hotspot expected).

## Non-goals / PARKED (do not build)

Rollback/compensation (s008); `advance`/`start`/`done`/`cancel` + modes; node-level status actions;
roles-on-edges; config verb→status mapping; any template change to the shipped `init` templates. Structured
commands only.
