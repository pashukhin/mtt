# Session 008 — Rollback / compensation (design spec)

Date: 2026-07-06 · Branch: `feat/s008-rollback` · Version bump: `0.7.0-dev → 0.8.0-dev`

Authoritative design for s008. Prose in [DESIGN.md](../../../DESIGN.md) stays the source of truth; this spec
is the resolved decision record the plan and implementation follow. It builds directly on s007's `Command`
value object (the shape s007 deliberately left additive room in).

## Goal

Give a **partially-applied transition the ability to undo its own side effects**. A transition's gate is a
pipeline of commands run in order (s006); s007 made each a `Command{Run, Timeout}` with placeholder
expansion. s008 adds **intra-pipeline compensation**: a command may declare a **rollback** command, and when
a *later* command in the same pipeline fails, the already-succeeded commands' rollbacks run **in reverse
order** — undoing what the partial gate did.

Killer shape (the content of forward/reverse commands is the **user's** responsibility, not ours — git is
merely one example):

```yaml
transitions:
  - from: tbd
    to: in_progress
    commands:
      - run: git checkout -b task/{{.ID}}
        rollback: git branch -D task/{{.ID}}   # compensator for THIS command
      - make test                               # a later gate; no side effect, no rollback
```

If `make test` fails, the gate is **blocked** (task stays `tbd`, s006 invariant) **and** `git branch -D
task/t1` runs to remove the branch the first command created. One additive optional field on the s007
`Command` VO (`Rollback`), plus a best-effort compensation path in the executor's abort branch.

## Architecture (resolved)

`cli → core → port ← adapter`. Everything typed (`mtt.TaskID`/`TypeName`/`StatusName`); string conversion
only at the cli/adapter boundary. Five open questions were brainstormed and locked, plus two follow-up
refinements:

- **Q1 — the rollback lives on the `Command` (per-command `Command.Rollback`), not on the `Transition`.** The
  seam's defining semantic is "reverse order over the **already-succeeded** commands", which maps 1:1 only if
  each forward command carries its own compensator: on a block we walk the succeeded commands in reverse and
  run each one's rollback. A flat per-transition list cannot express "which subset of compensators
  corresponds to the commands that actually ran" — it would run the whole list regardless of how far the
  pipeline got (risking an undo of a side effect that never happened). s007 and DESIGN/model.go already framed
  it as "the `Command` VO gains an additive `Rollback` field". **Shape:** `Rollback *Command` — a compensator
  is itself a structured command (its own `run` + per-command `timeout`, placeholder-expanded), so it reuses
  the whole `Command`/`ymlCommand` machinery. A pointer breaks the self-reference (an inline `Command` field
  would be an infinite-size struct). A compensator is a **leaf**: its own `Rollback` MUST be nil (no
  second-level compensation) — enforced in `Valid()`.

- **Q2 — `core.Transitioner` orchestrates compensation; the exec adapter executes it.** The s006/s007 split
  holds: a non-zero exit is **data** (a `Check`), the `Runner` is dumb, and the blocked-vs-applied decision is
  core policy. Compensation is likewise policy: core has the `[]Check` (1:1 with executed commands, so it
  knows which succeeded) and the already-expanded `Command`s (so it knows each one's `Rollback`); it computes
  the **plan** — the reversed list of succeeded commands' rollbacks — and hands it to the runner. *(Refined by
  Q-C below: the plan is executed by a single dedicated best-effort runner method, not a core-side loop over
  `Run`.)*

- **Q3 — compensation is best-effort and never masks the gate failure; the exit code stays 3.** If a
  compensator itself exits non-zero or times out, we **continue** running the remaining compensators (stopping
  would leave earlier side effects un-undone — a worse partial state). The transition's outcome is unchanged:
  `ErrBlocked` → exit **3** (the original gate failure), *not* a new code. Rollback outcomes are surfaced in
  the live progress stream and summarized in the block message (`compensated N commands (M failed)`), so a
  failed compensator is visible (manual cleanup may be needed) without changing what the transition "means"
  (it is blocked).

- **Q4 — no `history` entry on a blocked-and-compensated transition; the task file is untouched.** `Task.History`
  is an append-only journal of **transitions** (`from→to/at/by/checks`). A blocked transition did not happen
  (the status did not change), so a `HistoryEntry` for it is semantically wrong (there is no `from→to`), and
  the s006 invariant "blocked → no history, task file untouched" holds. Compensation is a **side-effect-level**
  event, not a transition; putting it in the transition journal is a category error. It is surfaced live
  (progress) and in the block message; a durable side-effect audit is the parked edit-audit slice, not this.

- **Q5 — rollback placeholders reuse `cmdContext{ID,Type,From,To}`, expanded eagerly (before the gate runs).**
  A rollback references what its forward command created (`git branch -D task/{{.ID}}`), fully covered by the
  four shape-safe fields with the **pre-move** `.From` (identical to the forward context). `expandCommands`
  expands both `Run` **and** `Rollback.Run` in one pass, **up front** — so a malformed rollback template aborts
  the whole transition as a plain error (exit 1) **before any side effect runs**, never leaving a "created a
  branch, can't clean up because the rollback template is broken" state. Lazy (compensation-time) expansion
  would surface that error late and add code paths — rejected.

Two refinements resolved after the five (brainstorm follow-up):

- **Q-B — the e2e proves compensation for *arbitrary* commands, not git.** The killer feature is the
  *mechanism* (any command can declare a compensator); the content of forward/rollback commands is the user's
  concern. So the automated e2e uses generic POSIX shell (file sentinels — `touch`/`rm`/`false`), needs **no**
  `[exec:git]` guard, and exercises a **multi-command** reverse compensation the single-branch example cannot.
  The git-branch narrative stays a documented example in DESIGN/CLI_REFERENCE, not a test dependency. Precise
  mechanics (reverse order, best-effort, succeeded-only, no-history) are **unit** tests (the s006/s007 split:
  unit for mechanics, e2e for scenario).

- **Q-C — the exec `Runner` gains a dedicated best-effort `Compensate` method (owns the labeled phase).** The
  live `↩ compensating (N)` header must be printed by whoever holds the progress writer at the right moment;
  core must stay free of I/O, and the dumb `Run` (which stops at the first non-zero) cannot express best-effort.
  A single `Runner.Compensate([]Command) []Check` cleanly owns the compensation phase: it prints the header +
  per-command progress, runs **best-effort** (never stops, no error return; an operational failure is recorded
  as `Exit -1`). This is a mild, *more faithful* reading of Q2 — core computes the plan (which succeeded,
  reversed); the adapter executes it. Cost: one new method on the `Runner` port (exec impl + the core test
  fake + one call site). Accepted.

## 1. `pkg/mtt` — the additive VO field

### 1a. `Command.Rollback` (`command.go`)

```go
// Command is one gate step of a transition: a shell command (Run) with an
// optional per-command timeout and an optional compensator (Rollback) run in
// reverse over the already-succeeded commands when a later command in the same
// pipeline fails (s008). Run/Rollback.Run hold raw templates; core expands the
// placeholders before the runner runs them.
type Command struct {
	Run      string
	Timeout  time.Duration
	Rollback *Command // optional compensator for THIS command; nil = none
}

// Valid reports whether the command is well-formed: a non-empty Run, a
// non-negative Timeout, and — if present — a well-formed LEAF compensator (a
// compensator is not itself compensated: its own Rollback must be nil).
func (c Command) Valid() bool {
	if c.Run == "" || c.Timeout < 0 {
		return false
	}
	if c.Rollback != nil {
		return c.Rollback.Run != "" && c.Rollback.Timeout >= 0 && c.Rollback.Rollback == nil
	}
	return true
}
```

- **Additive, back-compatible.** Field order stays `Run, Timeout, Rollback`. A command without a rollback is
  unchanged (`Rollback == nil`); every s007 command/YAML keeps working. The bare-scalar form (`- make lint`)
  still produces `{Run: "make lint"}` (no rollback).
- **Leaf invariant in `Valid()`.** A compensator is a `*Command` for machinery reuse, but conceptually it is a
  single command with no compensator of its own. `Valid()` rejects a second-level `rollback.rollback` via
  `Config.Validate` (which already loops `cmd.Valid()`). No smart constructor (the
  `StatusKind`/`CurrentAction`/s007-`Command` idiom).
- **Where the leaf invariant actually bites (accurate).** `Config.Validate` is called on `mtt add`
  (`add.go`) and `mtt types` (`types.go`) — **not** in `yaml.Load` and **not** on the gate/transition path
  (`status`/sugar). This is the pre-existing s006/s007 status quo (`load.go`'s own comment: "domain
  invariants are the caller's"); s008 does **not** change it (validating the whole config on every
  transition would be its own slice). Runtime is safe regardless: a stray second-level `rollback.rollback`
  *parses* but `expandOne` recurses harmlessly and `succeededRollbacks` reads only one level — so it is
  **silently ignored** on the gate path and **rejected** on `add`/`types`. Do not claim it is enforced "at
  Load."

### 1b. `Config.Validate` — unchanged call site

`validate.go` already does `for _, cmd := range tr.Commands { if !cmd.Valid() { … } }`. The stronger `Valid()`
now also covers the rollback; **no new loop, no signature change**. (Optionally the error message distinguishes
a bad rollback from a bad command — a nicety, not required.)

## 2. `internal/adapter/yaml` — recursive back-compat DTO (`dto.go`)

### 2a. `ymlCommand.Rollback` + recursive `UnmarshalYAML`

```go
type ymlCommand struct {
	Run      string
	Timeout  time.Duration
	Rollback *ymlCommand // nil = none
}

func (c *ymlCommand) UnmarshalYAML(value *goyaml.Node) error {
	if value.Kind == goyaml.ScalarNode { // bare command string (back-compat)
		c.Run = value.Value
		return nil
	}
	var raw struct {
		Run      string      `yaml:"run"`
		Timeout  string      `yaml:"timeout"`
		Rollback *ymlCommand `yaml:"rollback"` // recurses into UnmarshalYAML (scalar or {run,timeout})
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	c.Run = raw.Run
	if raw.Timeout != "" {
		d, err := time.ParseDuration(raw.Timeout)
		if err != nil {
			return fmt.Errorf("command %q: timeout %q: %w", raw.Run, raw.Timeout, err)
		}
		c.Timeout = d
	}
	c.Rollback = raw.Rollback
	return nil
}
```

- **The `rollback` value is itself a `ymlCommand`** — so it accepts the same **scalar** (`rollback: git branch
  -D task/{{.ID}}`) **or** map (`rollback: {run: …, timeout: 30s}`) forms, decoded through the same
  `UnmarshalYAML` (natural recursion). yaml.v3 recurses fine (the `raw` alias keeps `Timeout` a string to dodge
  the `30s`-into-`time.Duration` and self-recursion traps, exactly as s007). A second-level `rollback.rollback`
  *parses* (arbitrary depth) but is caught by the domain `Valid()` (§1a) — checked in `Config.Validate` (on
  `add`/`types`, not the gate path; see the validation note in §1a), and harmlessly ignored at runtime.
- **Cyclic YAML anchors are a non-concern.** The DTO is now self-recursive, so a hand-crafted
  `&a {rollback: *a}` is conceivable, but config is trusted local input (Makefile-equivalent) and yaml.v3's
  alias-expansion limits guard against an alias bomb; no explicit cycle check is warranted.

### 2b. `toDomain` — recursive pure copy

Introduce a small method to map the DTO recursively (replacing the inline `mtt.Command{Run, Timeout}` in the
transitions loop):

```go
func (c ymlCommand) toDomain() mtt.Command {
	m := mtt.Command{Run: c.Run, Timeout: c.Timeout}
	if c.Rollback != nil {
		rb := c.Rollback.toDomain()
		m.Rollback = &rb
	}
	return m
}
```

The transitions loop becomes `cmds = append(cmds, c.toDomain())`. Still a pure copy, **no error path**, no
`toDomain(Config)` signature change (the duration was parsed in `UnmarshalYAML`).

### 2c. Templates — unchanged

`default`/`coding` ship no rollbacks (the killer demo is exercised via a swapped e2e config, the s006/s007
pattern). Existing scalar/map commands keep parsing unchanged.

## 3. `internal/core` — eager rollback expansion + compensation orchestration

### 3a. `expand.go` — expand `Run` **and** `Rollback.Run`

Refactor `expandCommands` to expand each command recursively, reusing one template helper for `Run` and
`Rollback.Run`:

```go
func expandCommands(cmds []mtt.Command, ctx cmdContext) ([]mtt.Command, error) {
	if len(cmds) == 0 {
		return nil, nil
	}
	out := make([]mtt.Command, 0, len(cmds))
	for _, c := range cmds {
		ec, err := expandOne(c, ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, ec)
	}
	return out, nil
}

// expandOne expands a command's Run and (recursively) its Rollback against ctx.
// A compensator is a leaf (Valid() guarantees rollback.Rollback == nil), so the
// recursion is at most one level deep.
func expandOne(c mtt.Command, ctx cmdContext) (mtt.Command, error) {
	run, err := expandTemplate(c.Run, ctx)
	if err != nil {
		return mtt.Command{}, err
	}
	out := mtt.Command{Run: run, Timeout: c.Timeout}
	if c.Rollback != nil {
		rb, err := expandOne(*c.Rollback, ctx)
		if err != nil {
			return mtt.Command{}, err
		}
		out.Rollback = &rb
	}
	return out, nil
}

// expandTemplate renders one raw template string against ctx (Parse+Execute
// errors wrapped as before).
func expandTemplate(raw string, ctx cmdContext) (string, error) { /* the current body, extracted */ }
```

- **Eager, up-front, same `cmdContext` (pre-move `.From`).** Expansion runs before the gate (`Transitioner`
  already calls `expandCommands` before `runner.Run`), so a malformed `Run` **or** `Rollback.Run` aborts the
  transition as a plain error (exit 1) before any command runs. The expanded commands then carry
  ready-to-run expanded rollbacks — compensation needs no second expansion pass.

### 3b. `runner.go` — the `Runner` port gains `Compensate`

```go
type Runner interface {
	// Run executes the gate commands in order, stopping at the first non-zero
	// exit; a non-zero exit is data (a Check), the error is operational. CONTRACT
	// (compensation relies on it): on an operational failure the returned checks
	// include a Check for the failing command as the LAST element (Exit -1) — the
	// exec adapter already does this (exec.go: append then return err). A fake
	// Runner MUST replicate it (a guard test keeps it honest).
	Run(commands []mtt.Command) ([]mtt.Check, error)
	// Compensate runs the given (already-expanded) commands best-effort: in
	// order, NEVER stopping, NEVER returning an error — an operational failure
	// is recorded as Exit -1. It reports a labeled compensation phase to the
	// progress writer. core passes the reversed, succeeded-only rollbacks.
	Compensate(commands []mtt.Command) []mtt.Check
}
```

The `CONTRACT` line is new documentation of a dependency that already exists in the exec adapter; the §3c
math and the test fake both rely on it.

### 3c. `transition.go` — compensate on a block

On a gate block (operational error **or** a non-zero check), before returning `ErrBlocked`, compute the
compensation plan from an **explicit failure index** and run it. The succeeded set is `expanded[:failIdx]`;
its members' rollbacks run in **reverse**. This derives "which command failed" from a **single source**
(§ Issue-2 hardening): the failure index — never re-inferred as "the last check" at one site and "the first
non-zero" at another (those coincide only because the exec adapter stops at the first non-zero; a divergent
Runner must not misclassify the failed command as succeeded and run its rollback).

`firstFailure` is extended to also return the index, and one helper walks the succeeded prefix in reverse:

```go
// firstFailure returns the index and Check of the first non-zero exit (incl. an
// operational -1), and whether one was found.
func firstFailure(checks []mtt.Check) (int, mtt.Check, bool) {
	for i, c := range checks {
		if c.Exit != 0 {
			return i, c, true
		}
	}
	return 0, mtt.Check{}, false
}

// rollbacksBefore returns the rollbacks of expanded[:failIdx] in reverse order
// (compensation order) — never including the failing command itself. Safe for
// failIdx <= 0 (no compensation).
func rollbacksBefore(expanded []mtt.Command, failIdx int) []mtt.Command {
	var rbs []mtt.Command
	for i := failIdx - 1; i >= 0; i-- {
		if rb := expanded[i].Rollback; rb != nil {
			rbs = append(rbs, *rb)
		}
	}
	return rbs
}
```

The two block sites resolve `failIdx` from their own path and funnel through one helper:

```go
func (tr *Transitioner) block(expanded []mtt.Command, failIdx int, cause string) (mtt.Task, error) {
	if rbs := rollbacksBefore(expanded, failIdx); len(rbs) > 0 {
		comp := tr.runner.Compensate(rbs)
		return mtt.Task{}, fmt.Errorf("%w: %s; %s", ErrBlocked, cause, compSummary(comp))
	}
	return mtt.Task{}, fmt.Errorf("%w: %s", ErrBlocked, cause)
}

// operational error: the failing command is the last recorded check (port
// contract); if the runner recorded none, len(checks)-1 == -1 → no compensation.
//   return tr.block(expanded, len(checks)-1, rerr.Error())
// non-zero check: failIdx is the FIRST non-zero (single source of truth).
//   idx, c, _ := firstFailure(checks)
//   return tr.block(expanded, idx, fmt.Sprintf("command %q exited %d", c.Cmd, c.Exit))
```

The task is **not** changed and **no** history is written (s006 invariant preserved); `block` returns before
any `tr.store.Update`.

```go
// compSummary counts failed compensators (Exit != 0) for the block message.
func compSummary(checks []mtt.Check) string {
	failed := 0
	for _, c := range checks {
		if c.Exit != 0 {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Sprintf("compensated %d commands (%d failed)", len(checks), failed)
	}
	return fmt.Sprintf("compensated %d commands", len(checks))
}
```

- **Succeeded-only:** the failing command's own rollback is **never** run (it did not "succeed"; its side
  effect is uncertain) — structurally, `rollbacksBefore` starts at `failIdx-1`. If the **first** command
  fails, `failIdx==0` → no compensation. If **all** succeed, there is no block → the transition applies
  normally (no compensation).
- **Exit code unchanged:** the error stays `ErrBlocked` → exit 3 regardless of compensator outcomes.
- **`--no-run`:** skips the gate entirely → nothing ran → nothing to compensate (unchanged path).

## 4. `internal/adapter/exec` — best-effort `Compensate` (`exec.go`)

Extract the per-command run+report out of `Run` and reuse it in `Compensate` (DRY):

```go
// runReport runs one command, reports ▶ then ✓|✗ with timing, and returns its
// Check plus any operational error. Shared by Run and Compensate.
func (r *Runner) runReport(cmd mtt.Command) (mtt.Check, error) {
	_, _ = fmt.Fprintf(r.progress, "▶ %s\n", cmd.Run)
	start := time.Now()
	timeout := cmd.Timeout
	if timeout <= 0 {
		timeout = r.timeout
	}
	exit, err := r.runOne(cmd.Run, timeout)
	elapsed := time.Since(start).Round(time.Millisecond)
	mark := "✓"
	if exit != 0 || err != nil {
		mark = "✗"
	}
	_, _ = fmt.Fprintf(r.progress, "%s %s (exit %d, %s)\n", mark, cmd.Run, exit, elapsed)
	return mtt.Check{Cmd: cmd.Run, Exit: exit}, err
}

func (r *Runner) Run(commands []mtt.Command) ([]mtt.Check, error) {
	checks := make([]mtt.Check, 0, len(commands))
	for _, cmd := range commands {
		ck, err := r.runReport(cmd)
		checks = append(checks, ck)
		if err != nil {
			return checks, err // operational failure
		}
		if ck.Exit != 0 {
			return checks, nil // stop at first non-zero (data)
		}
	}
	return checks, nil
}

func (r *Runner) Compensate(commands []mtt.Command) []mtt.Check {
	if len(commands) == 0 {
		return nil
	}
	_, _ = fmt.Fprintf(r.progress, "↩ compensating (%d command%s)\n", len(commands), plural(len(commands)))
	checks := make([]mtt.Check, 0, len(commands))
	for _, cmd := range commands {
		ck, _ := r.runReport(cmd) // best-effort: ignore the operational error, never stop
		checks = append(checks, ck)
	}
	return checks
}
```

- **Best-effort:** `Compensate` ignores `runReport`'s error and the exit code — it always runs every
  compensator and returns all checks (operational failure recorded as `Exit -1` by `runOne`). It never returns
  an error.
- **Per-command timeout** resolution is identical to `Run` (via `runReport`): `cmd.Timeout` else the global
  fallback — a rollback honors its own `timeout`.
- **`plural(n)`** is a tiny helper (`"" `/`"s"`); the header matches the approved UX (`↩ compensating (2
  commands)`), then normal `▶`/`✓`/`✗` lines per compensator.
- The **fake** `Runner` in `core` tests implements `Compensate` too (records the commands it was asked to
  compensate + returns configurable checks, so tests can assert order/best-effort).

## 5. `internal/cli` — surface

- **`mtt types`** renders a command's compensator: under the `$ <run>` (+ `(timeout <d>)`) line, when
  `cmd.Rollback != nil`, print `↩ <rollback.Run>` (+ its own `(timeout <d>)` when set). Mirrors the s007
  timeout annotation; a small addition to the existing command renderer.
- **Block message** now carries the compensation summary (from §3c) — surfaced via the usual `Execute` →
  stderr path; exit code stays 3.
- **No new flags, no new exit codes.** Live progress (the `↩ compensating` phase + per-compensator lines) comes
  from the exec `Runner` on the existing progress writer; the commands' own output still honors `-v`/`--log-file`.

## 6. Tests (test-first)

**Unit — the precise mechanics:**
- `pkg/mtt`: `Command.Valid` — accepts a leaf rollback (`{run}` / `{run, timeout}`), rejects a second-level
  `rollback.rollback`, rejects empty/negative in the rollback; `Config.Validate` rejects a command with a bad
  rollback.
- `adapter/yaml`: `ymlCommand` parses a scalar rollback and a `{run, timeout}` rollback; `toDomain` deep-copies
  the rollback (pointer, not aliased); a bad rollback duration errors at `Load`.
- `core.expandCommands`: expands `Rollback.Run` with the same context; a malformed rollback template errors
  (up-front, before any run); `Timeout` copied through; nil rollback stays nil.
- `core.Transitioner` (fake `Runner`): on a **non-zero mid-pipeline** failure, `Compensate` is called with the
  succeeded commands' rollbacks **in reverse order**; the **failing** command's rollback is not included;
  **first** command fails → no compensation; **all pass** → no compensation + normal apply; a **best-effort**
  compensator failure does not change the outcome (still `ErrBlocked`, task unchanged, **no history**, task
  file's status preserved); the block error contains the `compensated N …` summary.
- `core.Transitioner` — **operational-error path** (separate test): the fake returns `(checks-with-failing-last,
  err != nil)`; assert reverse compensation over the succeeded prefix and still `ErrBlocked` (exit 3), task
  unchanged. This exercises the `block(expanded, len(checks)-1, err.Error())` branch (§3c) the non-zero test
  does not.
- `core` — **port-contract guard:** the test fake's `Run` appends the failing command's `Check` as the last
  element on the operational path (mirrors the exec adapter, §3b CONTRACT); a small assertion documents that the
  fake honors it, so `rollbacksBefore`'s `len(checks)-1` locates the failure.
- `exec.Runner.Compensate`: runs every command despite a non-zero/timeout in the middle (best-effort), records
  a `Check` per command (operational → `-1`), never returns an error; honors a per-command timeout; prints the
  `↩ compensating` header. (`exec.Run` behavior unchanged after the `runReport` extraction — keep its tests
  green.)
- `cli` — **`mtt types` `↩` rendering** (mirror `TestFormatTypesShowsCommandTimeout` in `types_test.go`): a
  command with a `rollback` renders the `↩ <rollback.Run>` line (and its own `(timeout <d>)` when set); a
  command without one renders no `↩` line.

**e2e — `rollback.txt` (generic commands, no git guard):** a swapped **full** single-root-type config
(`cp`'d over `.mtt/config.yaml`, the s006/s007 pattern — the fragment must be a complete valid flow, else
`mtt add` fails; note `mtt add` itself calls `Config.Validate`, so the e2e also exercises the leaf check):
```
-- gated.yaml --
version: 1
project:
  name: rb
command_timeout: 5m
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: tbd, kind: initial}
      - {name: in_progress, kind: active}
      - {name: done, kind: terminal}
    transitions:
      - from: tbd
        to: in_progress
        commands:
          - run: touch a-{{.ID}}
            rollback: rm a-{{.ID}}
          - run: touch b-{{.ID}}
            rollback: rm b-{{.ID}}
          - "false"
      - {from: in_progress, to: done}
```
`mtt init` → `cp gated.yaml .mtt/config.yaml` → `mtt add 'A'` (t1, single root type → no `--no-parent`) →
`! exec mtt in_progress t1` (blocked, exit 3). Assert: the created sentinels are gone (`! exists a-t1`,
`! exists b-t1` — compensation ran, placeholders expanded); the status is still `tbd` and no `in_progress`
history (s006 invariant, via `mtt show t1`); `stderr` shows `↩ compensating (2 commands)` and the block
message shows `compensated 2 commands`. (Precise reverse order + best-effort are the unit tests above.)

## 7. Docs + version

- **DESIGN.md / DESIGN.ru.md:** flip the "Seam (deferred): rollback / compensation" note to a **Shipped
  (s008)** note (per-command `Command.Rollback`, reverse-over-succeeded, best-effort, exit 3 preserved, no
  history, eager expansion, `Runner.Compensate`); keep the multi-step `--atomic`/`advance` compensation as the
  still-parked remainder.
- **CLI_REFERENCE.md / .ru:** document the `rollback:` sub-field on a structured command (scalar or `{run,
  timeout}`), the reverse-over-succeeded + best-effort semantics, that a block with compensation stays exit 3,
  and the `mtt types` `↩` annotation; note the `--atomic` row's "rollback planned" caveat now partially
  shipped (intra-pipeline only).
- **CLAUDE.md:** `pkg/mtt` (Command VO gains `Rollback`), `internal/core` (`expandCommands` rollback +
  `Transitioner` compensation + `Runner.Compensate`), `internal/adapter/exec` (`Compensate`, `runReport`),
  `internal/adapter/yaml` (`ymlCommand.Rollback` recursion), `internal/cli` (`types` `↩`).
- **model.go:** `Command.Rollback *Command`, `Runner.Compensate` in the port, `ResolvedEdge.Commands` note
  (unchanged type — `Command` now carries rollback), gap/seam note update.
- **TASKS.md:** tick **e4_t10** (rollback/compensation shipped — intra-pipeline; multi-step abort still later).
- **sessions/README.md:** row 008 ✅. **NEXT_SESSION.md:** move "Where we are" + add s008 carry-over lessons;
  next up = s008.5 dogfood enablers. **sessions/008_rollback.md:** fill Done.
- **Version:** bump `internal/cli/root.go` `var version` from `0.7.0-dev` to `0.8.0-dev` (the single
  definition; drives `mtt --version` / `mtt version`).

## Non-goals (explicitly out)

- Multi-step / `--atomic` / `advance` compensation after side effects across **several** edges — still parked
  (this is **intra-pipeline** only, one edge's pipeline).
- Second-level compensation (a compensator's compensator) — rejected by `Valid()`.
- A durable side-effect audit (history/edit-audit) of compensation — parked (Q4).
- Node-level status actions, roles-on-edges, `advance`/`start`/`done` — parked.
- Forcing `--who`/`--why` on rollback-risky ops — the deferred "dangerous ops must mandate attribution" slice.

## Commit plan (green between commits — see the writing-plans output)

1. `pkg/mtt`: `Command.Rollback` + `Valid()` leaf invariant + `Config.Validate` coverage (unit red→green).
2. `adapter/yaml`: `ymlCommand.Rollback` recursive unmarshal + `toDomain` (unit).
3. `core`: `expandCommands` rollback expansion (refactor to `expandOne`/`expandTemplate`) (unit).
4. `core` + `exec`: `Runner.Compensate` (port + exec `runReport`/`Compensate` + fake) (unit).
5. `core`: `Transitioner` compensation on block (`firstFailure`-index / `rollbacksBefore` / `compSummary`)
   (unit — incl. the operational-error path).
6. `cli`: `mtt types` `↩` + block summary surfaced; e2e `rollback.txt`.
7. Docs + version bump.

Each commit keeps `make check` green (the s007 "behavior-preserving slices" lesson).

## Review addendum (post-brainstorm subagent review, 2026-07-06)

An independent adversarial review verified the sketches against the real code (verdict: implementation-ready
with fixes, no blockers). Applied to this spec:
- **Issue 1 (SHOULD-FIX):** corrected the false "enforced at Load" claim — `Config.Validate` runs on
  `add`/`types`, not `yaml.Load` nor the gate path (pre-existing s006/s007 status quo). The leaf invariant is
  caught there; a stray second-level rollback on the gate path is harmlessly ignored at runtime (§1a, §2a).
- **Issue 2 (SHOULD-FIX):** hardened the succeeded/failed derivation to a **single source of truth** — an
  explicit `failIdx` (`firstFailure` index for a non-zero check; `len(checks)-1` for an operational error) with
  `rollbacksBefore(expanded, failIdx)` — so the failed command's rollback is never run even if a future Runner
  does not stop at the first non-zero (§3c).
- **Issue 3 (NIT):** documented the `Runner.Run` port contract (operational failure records the failing Check
  last) that compensation relies on, plus a fake guard test (§3b, §6).
- **Issues 4–5 (NITs):** added the operational-error compensation test, the `mtt types` `↩` render test, and a
  full valid e2e config (§6).
- **Open questions answered:** (1) the gate path does **not** call `Config.Validate` — status quo kept, scope
  unchanged; (2) yes, the Run-appends-failing-check-last contract is now explicit; (3) cyclic YAML anchors are
  a non-concern for trusted config (yaml.v3 alias limits) (§2a).
