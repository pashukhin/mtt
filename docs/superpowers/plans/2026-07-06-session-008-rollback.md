# Session 008 — Rollback / compensation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A transition's gate command may declare a `rollback` compensator; when a later command in the same pipeline fails, the already-succeeded commands' rollbacks run in reverse order (intra-pipeline compensation), while the transition stays blocked (exit 3, task unchanged, no history).

**Architecture:** Additive per-command `Command.Rollback *Command` (pkg/mtt). `core.Transitioner` computes the compensation plan (succeeded-only, reversed) from the gate's checks and hands it to the exec `Runner`'s new best-effort `Compensate` method. Rollbacks are expanded eagerly with the forward commands. `cli → core → port ← adapter`; everything typed.

**Tech Stack:** Go 1.23, `gopkg.in/yaml.v3`, cobra, `go-internal/testscript` (e2e). Authoritative spec: `docs/superpowers/specs/2026-07-06-session-008-rollback-design.md`.

## Global Constraints

- **Test before code** (TDD: red → green → refactor). `make check` must be **green** before every commit.
- **`make check`** = gofmt + go vet + golangci-lint v2 + `go test -race -cover ./...` + build. Run it before each commit.
- **Layers:** `cli → core → port ← adapter`; `core` imports only `pkg/mtt`, never an adapter. `pkg/mtt` carries no yaml/json tags and stays template-agnostic (stores raw templates; core expands).
- **Typed identities** (`mtt.TaskID`/`TypeName`/`StatusName`); string conversion only at the cli/adapter boundary.
- **CLI output** via `cmd.OutOrStdout()` / `cmd.ErrOrStderr()`; commands return errors via `RunE` (never `os.Exit`).
- **Commit trailer:** `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.
- **Version:** bump `internal/cli/root.go` `var version` from `0.7.0-dev` to `0.8.0-dev` (Task 7).
- **Green between commits:** each task is behavior-preserving + additive; the tree compiles and `make check` passes after every task (the s007 "behavior-preserving slices" lesson).
- Branch `feat/s008-rollback` already exists (off fresh `main`); the spec + session file are already committed.

## File Structure

- `pkg/mtt/command.go` — **modify**: add `Rollback *Command`; strengthen `Valid()` (leaf invariant). [Task 1]
- `pkg/mtt/command_test.go` — **modify**: leaf-rollback `Valid` cases. [Task 1]
- `pkg/mtt/validate_test.go` — **modify**: `Config.Validate` rejects a bad (nested) rollback. [Task 1]
- `pkg/mtt/validate.go` — **modify** (1 line): broaden the command error message. [Task 1]
- `internal/adapter/yaml/dto.go` — **modify**: `ymlCommand.Rollback *ymlCommand`; recursive `UnmarshalYAML`; `ymlCommand.toDomain()` (recursive); use it in the transitions loop. [Task 2]
- `internal/adapter/yaml/dto_test.go` — **modify**: rollback scalar/map unmarshal + `toDomain` deep-copy. [Task 2]
- `internal/core/expand.go` — **modify**: `expandCommands` → `expandOne`/`expandTemplate`; expand `Rollback.Run`. [Task 3]
- `internal/core/expand_test.go` — **modify**: rollback expansion + malformed-rollback error + nil-stays-nil. [Task 3]
- `internal/adapter/exec/exec.go` — **modify**: extract `runReport`; add `Compensate` + `plural`. [Task 4]
- `internal/adapter/exec/exec_test.go` — **modify**: `Compensate` best-effort/empty/timeout. [Task 4]
- `internal/core/runner.go` — **modify**: `Runner` port gains `Compensate` + the Run CONTRACT doc. [Task 5]
- `internal/core/transition.go` — **modify**: `firstFailure` returns index; add `rollbacksBefore`/`compSummary`/`block`; compensate on a block. [Task 5]
- `internal/core/transition_test.go` — **modify**: `fakeRunner.Compensate` + compensation tests. [Task 5]
- `internal/cli/types.go` — **modify**: render `↩ <rollback>` under a command. [Task 6]
- `internal/cli/types_test.go` — **modify**: `↩` render test. [Task 6]
- `internal/cli/testdata/scripts/rollback.txt` — **create**: acceptance e2e. [Task 6]
- Docs (DESIGN/.ru, CLI_REFERENCE/.ru, CLAUDE.md ×5, model.go, TASKS.md, sessions/README.md, NEXT_SESSION.md, sessions/008_rollback.md) + `internal/cli/root.go` version. [Task 7]

---

### Task 1: `pkg/mtt` — `Command.Rollback` + leaf-invariant `Valid()`

**Files:**
- Modify: `pkg/mtt/command.go`
- Modify: `pkg/mtt/validate.go:71`
- Test: `pkg/mtt/command_test.go`, `pkg/mtt/validate_test.go`

**Interfaces:**
- Produces: `mtt.Command{Run string, Timeout time.Duration, Rollback *Command}`; `Command.Valid() bool` (now also validates a leaf compensator: non-empty `Run`, non-negative `Timeout`, `Rollback.Rollback == nil`).

- [ ] **Step 1: Write the failing tests** — extend the `TestCommandValid` table in `pkg/mtt/command_test.go` with these cases (inside the existing `cases := []struct{...}{ ... }`):

```go
		{"rollback leaf", Command{Run: "git checkout -b x", Rollback: &Command{Run: "git branch -D x"}}, true},
		{"rollback with timeout", Command{Run: "a", Rollback: &Command{Run: "b", Timeout: 5 * time.Second}}, true},
		{"rollback empty run", Command{Run: "a", Rollback: &Command{Run: ""}}, false},
		{"rollback negative timeout", Command{Run: "a", Rollback: &Command{Run: "b", Timeout: -1}}, false},
		{"nested rollback rejected", Command{Run: "a", Rollback: &Command{Run: "b", Rollback: &Command{Run: "c"}}}, false},
```

- [ ] **Step 2: Run the test to verify it fails to compile**

Run: `go test ./pkg/mtt/ -run TestCommandValid`
Expected: build error — `unknown field 'Rollback' in struct literal of type Command`.

- [ ] **Step 3: Add the field + strengthen `Valid()`** — replace the whole body of `pkg/mtt/command.go` with:

```go
package mtt

import "time"

// Command is one gate step of a transition: a shell command (Run) with an
// optional per-command timeout that overrides the adapter's global
// command_timeout (zero = fall back to the global), and an optional compensator
// (Rollback) run in reverse over the already-succeeded commands when a later
// command in the same pipeline fails (s008). Run/Rollback.Run hold raw templates
// (e.g. "git checkout -b task/{{.ID}}"); the domain does not interpret them —
// core expands the placeholders before the runner runs them.
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

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./pkg/mtt/ -run TestCommandValid -v`
Expected: PASS (all table cases).

- [ ] **Step 5: Broaden the validate error message** — in `pkg/mtt/validate.go`, the command loop already calls `cmd.Valid()`; only update the message text at line ~71:

```go
			for _, cmd := range tr.Commands {
				if !cmd.Valid() {
					errs = append(errs, fmt.Errorf("type %q transition %q->%q: invalid command (empty/negative timeout or bad rollback)", t.Name, tr.From, tr.To))
				}
			}
```

- [ ] **Step 6: Write the failing `Config.Validate` test** — add to `pkg/mtt/validate_test.go`:

```go
func TestValidateRejectsBadRollback(t *testing.T) {
	cfg := Config{Version: 1, Types: []Type{{
		Name: "task", Default: true,
		Flow: Flow{
			Statuses: []Status{
				{Name: "tbd", Kind: KindInitial},
				{Name: "doing", Kind: KindActive},
				{Name: "done", Kind: KindTerminal},
			},
			Transitions: []Transition{
				{From: "tbd", To: "doing", Commands: []Command{
					{Run: "a", Rollback: &Command{Run: "b", Rollback: &Command{Run: "c"}}}, // nested → invalid
				}},
				{From: "doing", To: "done"},
			},
		},
	}}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("want an error for a command with a nested (second-level) rollback")
	}
}
```

- [ ] **Step 7: Run the package tests**

Run: `go test ./pkg/mtt/ -v`
Expected: PASS (incl. the existing tests — the new `Rollback` field defaults nil, so all existing struct comparisons still hold).

- [ ] **Step 8: Gate + commit**

```bash
make check
git add pkg/mtt/command.go pkg/mtt/command_test.go pkg/mtt/validate.go pkg/mtt/validate_test.go
git commit -m "feat(s008): pkg/mtt Command.Rollback + leaf-invariant Valid()

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: `internal/adapter/yaml` — recursive `ymlCommand.Rollback` + `toDomain`

**Files:**
- Modify: `internal/adapter/yaml/dto.go`
- Test: `internal/adapter/yaml/dto_test.go`

**Interfaces:**
- Consumes: `mtt.Command.Rollback` (Task 1).
- Produces: `ymlCommand{Run, Timeout, Rollback *ymlCommand}`; `ymlCommand.toDomain() mtt.Command` (recursive deep copy). A YAML command's `rollback` is a scalar or `{run, timeout}` map.

- [ ] **Step 1: Write the failing tests** — add to `internal/adapter/yaml/dto_test.go`:

```go
func TestYmlCommandUnmarshalRollbackScalar(t *testing.T) {
	var c ymlCommand
	if err := goyaml.Unmarshal([]byte("{run: git checkout -b x, rollback: git branch -D x}"), &c); err != nil {
		t.Fatal(err)
	}
	if c.Rollback == nil || c.Rollback.Run != "git branch -D x" || c.Rollback.Timeout != 0 {
		t.Fatalf("rollback = %+v, want {Run: git branch -D x}", c.Rollback)
	}
}

func TestYmlCommandUnmarshalRollbackMap(t *testing.T) {
	var c ymlCommand
	if err := goyaml.Unmarshal([]byte("{run: a, rollback: {run: b, timeout: 30s}}"), &c); err != nil {
		t.Fatal(err)
	}
	if c.Rollback == nil || c.Rollback.Run != "b" || c.Rollback.Timeout != 30*time.Second {
		t.Fatalf("rollback = %+v, want {Run: b, Timeout: 30s}", c.Rollback)
	}
}

func TestToDomainRollbackDeepCopy(t *testing.T) {
	yc := ymlCommand{Run: "a", Rollback: &ymlCommand{Run: "b"}}
	m := yc.toDomain()
	if m.Rollback == nil || m.Rollback.Run != "b" {
		t.Fatalf("rollback not mapped: %+v", m.Rollback)
	}
	yc.Rollback.Run = "changed" // mutating the DTO must not affect the domain copy
	if m.Rollback.Run != "b" {
		t.Fatal("toDomain aliased the rollback pointer instead of deep-copying")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/adapter/yaml/ -run 'Rollback'`
Expected: build error — `unknown field 'Rollback'` / `yc.toDomain undefined`.

- [ ] **Step 3: Add the field + recursion** — in `internal/adapter/yaml/dto.go`, change the `ymlCommand` struct and its `UnmarshalYAML`:

```go
// ymlCommand is one gate command on disk: a bare scalar (a command string,
// back-compat) or a mapping {run, timeout, rollback}; both collapse to a single
// mtt.Command. The duration is parsed here so toDomain stays error-free. The
// optional rollback is itself a ymlCommand (scalar or map).
type ymlCommand struct {
	Run      string
	Timeout  time.Duration
	Rollback *ymlCommand // nil = none
}

// UnmarshalYAML decodes a scalar command string or a {run, timeout, rollback}
// mapping. The map branch decodes into a LOCAL string-Timeout alias (never back
// into ymlCommand — that would recurse; and yaml.v3 cannot decode "30s" into a
// time.Duration); the rollback field is a *ymlCommand, so yaml.v3 recurses into
// this same UnmarshalYAML for it (scalar or map).
func (c *ymlCommand) UnmarshalYAML(value *goyaml.Node) error {
	if value.Kind == goyaml.ScalarNode {
		c.Run = value.Value
		return nil
	}
	var raw struct {
		Run      string      `yaml:"run"`
		Timeout  string      `yaml:"timeout"`
		Rollback *ymlCommand `yaml:"rollback"`
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

// toDomain maps the command DTO to the pure domain Command, recursively copying
// the optional rollback compensator (a deep copy — a fresh *Command, not the
// DTO's pointer).
func (c ymlCommand) toDomain() mtt.Command {
	m := mtt.Command{Run: c.Run, Timeout: c.Timeout}
	if c.Rollback != nil {
		rb := c.Rollback.toDomain()
		m.Rollback = &rb
	}
	return m
}
```

- [ ] **Step 4: Use `toDomain` in the transitions loop** — in `internal/adapter/yaml/dto.go`, replace the inline command mapping inside the `toDomain()` (Config) transitions loop:

```go
			for _, yr := range yt.Transitions {
				cmds := make([]mtt.Command, 0, len(yr.Commands))
				for _, c := range yr.Commands {
					cmds = append(cmds, c.toDomain())
				}
				t.Transitions = append(t.Transitions, mtt.Transition{From: mtt.StatusName(yr.From), To: mtt.StatusName(yr.To), Description: yr.Description, Commands: cmds, Current: mtt.CurrentAction(yr.Current)})
			}
```

- [ ] **Step 5: Run the package tests**

Run: `go test ./internal/adapter/yaml/ -v`
Expected: PASS (new rollback tests + existing `TestToDomainCommandsMixed`, `TestYmlCommandUnmarshal*` — the scalar/map back-compat is unchanged, and struct comparisons still hold with `Rollback` nil).

- [ ] **Step 6: Gate + commit**

```bash
make check
git add internal/adapter/yaml/dto.go internal/adapter/yaml/dto_test.go
git commit -m "feat(s008): yaml ymlCommand.rollback (recursive scalar|map) + toDomain deep-copy

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: `internal/core` — eager rollback expansion

**Files:**
- Modify: `internal/core/expand.go`
- Test: `internal/core/expand_test.go`

**Interfaces:**
- Consumes: `mtt.Command.Rollback`, `cmdContext{ID,Type,From,To}`.
- Produces: `expandCommands([]mtt.Command, cmdContext) ([]mtt.Command, error)` now also expands `Rollback.Run` (recursively, one level), returning expanded rollbacks ready to run. Unchanged signature.

- [ ] **Step 1: Write the failing tests** — add to `internal/core/expand_test.go`:

```go
func TestExpandCommandsExpandsRollback(t *testing.T) {
	ctx := cmdContext{ID: "t1", Type: "task", From: "tbd", To: "in_progress"}
	out, err := expandCommands([]mtt.Command{{
		Run:      "git checkout -b task/{{.ID}}",
		Rollback: &mtt.Command{Run: "git branch -D task/{{.ID}}", Timeout: 5 * time.Second},
	}}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Rollback == nil || out[0].Rollback.Run != "git branch -D task/t1" {
		t.Fatalf("rollback run = %+v, want git branch -D task/t1", out[0].Rollback)
	}
	if out[0].Rollback.Timeout != 5*time.Second {
		t.Fatalf("rollback timeout dropped: %v", out[0].Rollback.Timeout)
	}
}

func TestExpandCommandsMalformedRollback(t *testing.T) {
	_, err := expandCommands([]mtt.Command{{
		Run:      "true",
		Rollback: &mtt.Command{Run: "echo {{.Title}}"}, // unexposed field → error up-front
	}}, cmdContext{ID: "t1"})
	if err == nil {
		t.Fatal("want an error for a malformed rollback template (before any run)")
	}
}

func TestExpandCommandsNilRollbackStaysNil(t *testing.T) {
	out, err := expandCommands([]mtt.Command{{Run: "true"}}, cmdContext{})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Rollback != nil {
		t.Fatal("nil rollback became non-nil")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/core/ -run TestExpandCommands`
Expected: FAIL — `TestExpandCommandsExpandsRollback` (rollback is nil, the current `expandCommands` drops it) and `TestExpandCommandsMalformedRollback` (no error).

- [ ] **Step 3: Refactor `expand.go`** — replace `expandCommands` with the recursive `expandOne`/`expandTemplate` form (keep the file's package + imports: `fmt`, `strings`, `text/template`, `mtt`; and the existing `cmdContext` doc/struct above `expandCommands`):

```go
// expandCommands renders each command's Run (and, recursively, its Rollback.Run)
// against ctx, returning new commands with expanded strings and unchanged
// timeouts. Expansion is eager and up-front (before the gate), so a malformed
// template in a command OR its rollback aborts the transition before any side
// effect runs. A malformed template (Parse) or a reference to an unexposed field
// (Execute) is an error.
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
// A compensator is a leaf (Config.Validate guarantees rollback.Rollback == nil),
// so the recursion is at most one level deep.
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

// expandTemplate renders one raw template string against ctx.
func expandTemplate(raw string, ctx cmdContext) (string, error) {
	tmpl, err := template.New("cmd").Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse command %q: %w", raw, err)
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, ctx); err != nil {
		return "", fmt.Errorf("expand command %q: %w", raw, err)
	}
	return b.String(), nil
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/core/ -run TestExpandCommands -v`
Expected: PASS (new + existing expansion tests — `Run`/`Timeout` behavior unchanged).

- [ ] **Step 5: Gate + commit**

```bash
make check
git add internal/core/expand.go internal/core/expand_test.go
git commit -m "feat(s008): core expandCommands expands Rollback.Run eagerly (expandOne/expandTemplate)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: `internal/adapter/exec` — best-effort `Compensate`

**Files:**
- Modify: `internal/adapter/exec/exec.go`
- Test: `internal/adapter/exec/exec_test.go`

**Interfaces:**
- Produces: `(*exec.Runner).Compensate(commands []mtt.Command) []mtt.Check` (best-effort: runs all, never stops, never errors; prints `↩ compensating (N command[s])` + per-command `▶`/`✓`/`✗`; operational failure → `Exit -1`). `runReport` is a private helper shared with `Run` (behavior of `Run` unchanged).

- [ ] **Step 1: Write the failing tests** — add to `internal/adapter/exec/exec_test.go`:

```go
func TestCompensateBestEffortRunsAll(t *testing.T) {
	var prog bytes.Buffer
	// The middle compensator fails (exit 1); best-effort must still run the last.
	checks := NewRunner(t.TempDir(), time.Minute, &prog, io.Discard).
		Compensate([]mtt.Command{{Run: "true"}, {Run: "false"}, {Run: "true"}})
	if len(checks) != 3 {
		t.Fatalf("ran %d compensators, want all 3 (best-effort)", len(checks))
	}
	if checks[0].Exit != 0 || checks[1].Exit == 0 || checks[2].Exit != 0 {
		t.Fatalf("checks = %+v", checks)
	}
	if !strings.Contains(prog.String(), "↩ compensating (3 commands)") {
		t.Fatalf("progress missing the compensation header:\n%s", prog.String())
	}
}

func TestCompensateEmptyIsNoOp(t *testing.T) {
	var prog bytes.Buffer
	if checks := NewRunner(t.TempDir(), time.Minute, &prog, io.Discard).Compensate(nil); checks != nil {
		t.Fatalf("checks = %+v, want nil", checks)
	}
	if prog.Len() != 0 {
		t.Fatalf("empty compensation should print nothing:\n%s", prog.String())
	}
}

func TestCompensateHonorsPerCommandTimeout(t *testing.T) {
	// A tight per-command timeout on a compensator fires; best-effort records -1.
	checks := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard).
		Compensate([]mtt.Command{{Run: "sleep 1", Timeout: 20 * time.Millisecond}})
	if len(checks) != 1 || checks[0].Exit != -1 {
		t.Fatalf("checks = %+v, want a single -1 (timed-out) check", checks)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/adapter/exec/ -run TestCompensate`
Expected: build error — `r.Compensate undefined`.

- [ ] **Step 3: Extract `runReport` and add `Compensate` + `plural`** — in `internal/adapter/exec/exec.go`, replace the existing `Run` method with the `runReport`-based `Run`, and add `Compensate` and `plural` after it (leave `runOne` and `shell` untouched):

```go
// runReport runs one command, reports ▶ then ✓|✗ with timing to progress, and
// returns its Check plus any operational error. Shared by Run and Compensate.
func (r *Runner) runReport(cmd mtt.Command) (mtt.Check, error) {
	_, _ = fmt.Fprintf(r.progress, "▶ %s\n", cmd.Run)
	start := time.Now()
	timeout := cmd.Timeout
	if timeout <= 0 {
		timeout = r.timeout // fall back to the global command_timeout
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

// Run executes commands in order, recording a Check per executed command and
// reporting live progress. It stops at the first non-zero exit (a Check, not an
// error). An operational failure (launch error or timeout) returns the checks so
// far — with the failing command's Check as the LAST element — plus a non-nil
// error (core's compensation relies on this ordering).
func (r *Runner) Run(commands []mtt.Command) ([]mtt.Check, error) {
	checks := make([]mtt.Check, 0, len(commands))
	for _, cmd := range commands {
		ck, err := r.runReport(cmd)
		checks = append(checks, ck)
		if err != nil {
			return checks, err
		}
		if ck.Exit != 0 {
			return checks, nil
		}
	}
	return checks, nil
}

// Compensate runs already-expanded compensators best-effort: in order, NEVER
// stopping and NEVER returning an error (an operational failure is recorded as
// Exit -1 by runOne). It prints a labeled compensation phase to progress. core
// passes the reversed, succeeded-only rollbacks.
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

// plural returns "s" unless n == 1.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/adapter/exec/ -v`
Expected: PASS (new `TestCompensate*` + all existing `TestRun*` — `Run` behavior is unchanged after the `runReport` extraction).

- [ ] **Step 5: Gate + commit**

```bash
make check
git add internal/adapter/exec/exec.go internal/adapter/exec/exec_test.go
git commit -m "feat(s008): exec Runner.Compensate (best-effort, labeled phase) + runReport extraction

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: `internal/core` — `Runner.Compensate` port + `Transitioner` compensation

**Files:**
- Modify: `internal/core/runner.go`
- Modify: `internal/core/transition.go`
- Test: `internal/core/transition_test.go`

**Interfaces:**
- Consumes: `(*exec.Runner).Compensate` (Task 4); `mtt.Command.Rollback`; `expandCommands` (expands rollbacks, Task 3).
- Produces: `core.Runner` port method `Compensate(commands []mtt.Command) []mtt.Check`; `firstFailure(checks) (int, mtt.Check, bool)`; `rollbacksBefore(expanded []mtt.Command, failIdx int) []mtt.Command`; `compSummary(checks []mtt.Check) string`; `(*Transitioner).block(expanded, failIdx, cause) (mtt.Task, error)`.

- [ ] **Step 1: Add `Compensate` to the `Runner` port** — in `internal/core/runner.go`, replace the `Runner` interface (keep the surrounding sentinels untouched):

```go
// Runner executes a transition's commands in order and reports each result. It is
// defined here (only core uses it), implemented in internal/adapter/exec, and
// faked in tests. A non-zero exit is DATA (recorded in a Check), not a Go error;
// the returned error signals an operational failure (a command could not launch
// or timed out). At this boundary each Command's Run is ALREADY EXPANDED by core
// (see expand.go); the runner only runs and reports.
type Runner interface {
	// Run executes the commands in order, stopping at the first non-zero exit.
	// CONTRACT (compensation relies on it): on an operational failure the returned
	// checks include a Check for the failing command as the LAST element (Exit -1).
	Run(commands []mtt.Command) ([]mtt.Check, error)
	// Compensate runs the given already-expanded commands best-effort: in order,
	// NEVER stopping, NEVER returning an error (an operational failure is recorded
	// as Exit -1). It reports a labeled compensation phase. core passes the
	// reversed, succeeded-only rollbacks.
	Compensate(commands []mtt.Command) []mtt.Check
}
```

- [ ] **Step 2: Add `Compensate` to the test fake + write the failing tests** — in `internal/core/transition_test.go`, extend `fakeRunner`, add the `Compensate` method and two helpers, then the compensation tests:

```go
// (extend the existing struct)
type fakeRunner struct {
	checks     []mtt.Check
	err        error
	called     bool
	gotCmds    []mtt.Command
	compCmds   []mtt.Command // commands passed to Compensate (nil = never called)
	compChecks []mtt.Check   // canned Compensate result (nil = all succeed)
}

func (f *fakeRunner) Compensate(commands []mtt.Command) []mtt.Check {
	f.compCmds = commands
	if f.compChecks != nil {
		return f.compChecks
	}
	out := make([]mtt.Check, len(commands))
	for i, c := range commands {
		out[i] = mtt.Check{Cmd: c.Run, Exit: 0}
	}
	return out
}

// rbCmd is a command with a leaf compensator, for compensation tests.
func rbCmd(run, rollback string) mtt.Command {
	return mtt.Command{Run: run, Rollback: &mtt.Command{Run: rollback}}
}

// flowCfgA is flowCfg with explicit Commands on the tbd→in_progress edge (index 0).
func flowCfgA(cmdsA []mtt.Command) mtt.Config {
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Commands = cmdsA
	return cfg
}

func TestTransitionCompensatesSucceededInReverse(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{checks: []mtt.Check{
		{Cmd: "c1", Exit: 0}, {Cmd: "c2", Exit: 0}, {Cmd: "c3", Exit: 1},
	}}
	cfg := flowCfgA([]mtt.Command{rbCmd("c1", "r1"), rbCmd("c2", "r2"), {Run: "c3"}})
	tr := NewTransitioner(store, cfg, runner, testClock)

	_, err := tr.Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	if len(runner.compCmds) != 2 || runner.compCmds[0].Run != "r2" || runner.compCmds[1].Run != "r1" {
		t.Fatalf("compensated %+v, want [r2 r1] (reverse over succeeded)", runner.compCmds)
	}
	reloaded, _ := store.Get("t1")
	if reloaded.Status != "tbd" || len(reloaded.History) != 0 {
		t.Fatalf("task changed on a blocked+compensated transition: %+v", reloaded)
	}
	if !strings.Contains(err.Error(), "compensated 2 commands") {
		t.Fatalf("block error missing compensation summary: %v", err)
	}
}

func TestTransitionFirstCommandFailNoCompensation(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{checks: []mtt.Check{{Cmd: "c1", Exit: 1}}}
	cfg := flowCfgA([]mtt.Command{rbCmd("c1", "r1")})
	tr := NewTransitioner(store, cfg, runner, testClock)

	if _, err := tr.Transition("t1", "in_progress", TransitionOptions{}); !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	if runner.compCmds != nil {
		t.Fatalf("compensated %+v, want none (first command failed)", runner.compCmds)
	}
}

func TestTransitionOperationalErrorCompensates(t *testing.T) {
	store := newMemStore(baseTask())
	// c1 ok, c2 operational failure (recorded last as -1 per the Runner CONTRACT).
	runner := &fakeRunner{
		checks: []mtt.Check{{Cmd: "c1", Exit: 0}, {Cmd: "c2", Exit: -1}},
		err:    errors.New(`command "c2" timed out`),
	}
	cfg := flowCfgA([]mtt.Command{rbCmd("c1", "r1"), rbCmd("c2", "r2")})
	tr := NewTransitioner(store, cfg, runner, testClock)

	_, err := tr.Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	if len(runner.compCmds) != 1 || runner.compCmds[0].Run != "r1" {
		t.Fatalf("compensated %+v, want [r1] (succeeded-before-failure only)", runner.compCmds)
	}
}

func TestTransitionBestEffortCompensatorFailureKeepsBlocked(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{
		checks:     []mtt.Check{{Cmd: "c1", Exit: 0}, {Cmd: "c2", Exit: 1}},
		compChecks: []mtt.Check{{Cmd: "r1", Exit: 1}}, // the compensator itself fails
	}
	cfg := flowCfgA([]mtt.Command{rbCmd("c1", "r1"), {Run: "c2"}})
	tr := NewTransitioner(store, cfg, runner, testClock)

	_, err := tr.Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("a failed compensator must not change the outcome; err = %v", err)
	}
	if !strings.Contains(err.Error(), "compensated 1 command (1 failed)") {
		t.Fatalf("summary should report the failed compensator: %v", err)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/core/ -run 'TestTransition(Compensates|FirstCommandFail|OperationalError|BestEffort)'`
Expected: FAIL — the current `Transitioner` returns `ErrBlocked` without calling `Compensate` (`compCmds` stays nil; the summary is absent).

- [ ] **Step 4: Rewrite the block path in `transition.go`** — change `firstFailure` to return the index, replace the two block sites, and add `rollbacksBefore`/`compSummary`/`block`. In `internal/core/transition.go`, the gate section inside `if !opts.NoRun { ... }` becomes:

```go
		checks, err = tr.runner.Run(expanded)
		if err != nil {
			// operational failure: the failing command is the last recorded check
			// (Runner CONTRACT); if none was recorded, len(checks)-1 == -1 → no comp.
			return tr.block(expanded, len(checks)-1, err.Error())
		}
		if i, c, failed := firstFailure(checks); failed {
			return tr.block(expanded, i, fmt.Sprintf("command %q exited %d", c.Cmd, c.Exit))
		}
```

Replace `firstFailure` and add the three helpers (at the bottom of the file, near the existing `firstFailure`):

```go
// block runs best-effort compensation over the commands that succeeded before
// failIdx (their rollbacks, in reverse) and returns ErrBlocked with a summary.
// The task is left unchanged and no history is written (s006 invariant): block
// returns before any tr.store.Update.
func (tr *Transitioner) block(expanded []mtt.Command, failIdx int, cause string) (mtt.Task, error) {
	if rbs := rollbacksBefore(expanded, failIdx); len(rbs) > 0 {
		comp := tr.runner.Compensate(rbs)
		return mtt.Task{}, fmt.Errorf("%w: %s; %s", ErrBlocked, cause, compSummary(comp))
	}
	return mtt.Task{}, fmt.Errorf("%w: %s", ErrBlocked, cause)
}

// rollbacksBefore returns the rollbacks of expanded[:failIdx] in reverse order
// (compensation order) — never including the failing command itself. Safe for
// failIdx <= 0 (returns nil).
func rollbacksBefore(expanded []mtt.Command, failIdx int) []mtt.Command {
	var rbs []mtt.Command
	for i := failIdx - 1; i >= 0; i-- {
		if rb := expanded[i].Rollback; rb != nil {
			rbs = append(rbs, *rb)
		}
	}
	return rbs
}

// compSummary reports how many compensators ran and how many failed (Exit != 0).
func compSummary(checks []mtt.Check) string {
	noun := "commands"
	if len(checks) == 1 {
		noun = "command"
	}
	failed := 0
	for _, c := range checks {
		if c.Exit != 0 {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Sprintf("compensated %d %s (%d failed)", len(checks), noun, failed)
	}
	return fmt.Sprintf("compensated %d %s", len(checks), noun)
}

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
```

- [ ] **Step 5: Run the package tests**

Run: `go test ./internal/core/ -v`
Expected: PASS (new compensation tests + all existing `TestTransition*` — the non-compensating block path still returns `ErrBlocked`; note the block message format changed from `command %q exited %d` to include it after `blocked:`, still matched by `errors.Is(err, ErrBlocked)`).

- [ ] **Step 6: Gate + commit**

```bash
make check
git add internal/core/runner.go internal/core/transition.go internal/core/transition_test.go
git commit -m "feat(s008): core Transitioner compensates succeeded rollbacks on block (Runner.Compensate)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: `internal/cli` — `mtt types` `↩` + acceptance e2e

**Files:**
- Modify: `internal/cli/types.go:97-103`
- Test: `internal/cli/types_test.go`
- Create: `internal/cli/testdata/scripts/rollback.txt`

**Interfaces:**
- Consumes: everything from Tasks 1–5 (the block error already carries the `compensated N …` summary and reaches stderr via `Execute`; no CLI wiring change for the summary).
- Produces: `mtt types` renders `↩ <rollback.Run>` (+ `(timeout <d>)`) under a command; the e2e `rollback.txt`.

- [ ] **Step 1: Write the failing render test** — add to `internal/cli/types_test.go`:

```go
func TestFormatTypesShowsRollback(t *testing.T) {
	cfg := mtt.Config{Types: []mtt.Type{{
		Name: "task",
		Flow: mtt.Flow{
			Statuses: []mtt.Status{{Name: "tbd", Kind: mtt.KindInitial}, {Name: "doing", Kind: mtt.KindActive}},
			Transitions: []mtt.Transition{{From: "tbd", To: "doing", Commands: []mtt.Command{
				{Run: "git checkout -b x", Rollback: &mtt.Command{Run: "git branch -D x"}},
			}}},
		},
	}}}
	out, err := formatTypes(cfg, map[string]string{"task": "t"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "↩ git branch -D x") {
		t.Fatalf("types output missing the rollback annotation:\n%s", out)
	}
}
```

(If `types_test.go` does not already import `strings`/`mtt`, add them to its import block.)

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/cli/ -run TestFormatTypesShowsRollback`
Expected: FAIL — the output has no `↩` line.

- [ ] **Step 3: Render the rollback** — in `internal/cli/types.go`, replace the command loop inside `writeTypeBlock`:

```go
		for _, c := range tr.Commands {
			if c.Timeout > 0 {
				fmt.Fprintf(b, "        $ %s  (timeout %s)\n", c.Run, c.Timeout)
			} else {
				fmt.Fprintf(b, "        $ %s\n", c.Run)
			}
			if c.Rollback != nil {
				if c.Rollback.Timeout > 0 {
					fmt.Fprintf(b, "        ↩ %s  (timeout %s)\n", c.Rollback.Run, c.Rollback.Timeout)
				} else {
					fmt.Fprintf(b, "        ↩ %s\n", c.Rollback.Run)
				}
			}
		}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/cli/ -run TestFormatTypesShowsRollback -v`
Expected: PASS.

- [ ] **Step 5: Write the acceptance e2e** — create `internal/cli/testdata/scripts/rollback.txt`:

```
# s008 rollback/compensation: a later gate failure blocks the transition AND runs
# the succeeded commands' rollbacks in reverse (intra-pipeline compensation).
# Generic commands (touch/rm/false) — no git dependency, no guard.

exec mtt init
cp gated.yaml .mtt/config.yaml

exec mtt add 'implement feature'
stdout 'created t1'

# take-into-work gate: touch a-t1 (rb rm a-t1), touch b-t1 (rb rm b-t1), then
# `false` fails -> blocked (non-zero exit), and the two touches are compensated.
! exec mtt in_progress t1
stderr 'blocked'
stderr '↩ compensating \(2 commands\)'
stderr 'compensated 2 commands'

# the created sentinels were removed by their rollbacks (placeholders expanded).
! exists a-t1
! exists b-t1

# the transition did not apply: the task is still tbd (no status change).
exec mtt show t1
stdout '\[tbd\]'

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

- [ ] **Step 6: Run the e2e**

Run: `go test ./internal/cli/ -run 'TestScript/rollback' -v`
Expected: PASS. (If the script harness name differs, run `go test ./internal/cli/ -run TestScript -v` and confirm `rollback` is listed and green.)

- [ ] **Step 7: Full package test + gate + commit**

```bash
go test ./internal/cli/ -v
make check
git add internal/cli/types.go internal/cli/types_test.go internal/cli/testdata/scripts/rollback.txt
git commit -m "feat(s008): mtt types renders rollback; acceptance e2e rollback.txt

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Docs sync + version bump

**Files:**
- Modify: `internal/cli/root.go:16` (version)
- Modify: `DESIGN.md`, `DESIGN.ru.md`, `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md`
- Modify: `pkg/mtt/CLAUDE.md`, `internal/core/CLAUDE.md`, `internal/adapter/exec/CLAUDE.md`, `internal/adapter/yaml/CLAUDE.md`, `internal/cli/CLAUDE.md`
- Modify: `docs/architecture/model.go`, `TASKS.md`, `sessions/README.md`, `NEXT_SESSION.md`, `sessions/008_rollback.md`

**Interfaces:** none (docs + version constant).

- [ ] **Step 1: Bump the version** — in `internal/cli/root.go`, line 16:

```go
var version = "0.8.0-dev"
```

- [ ] **Step 2: DESIGN.md / DESIGN.ru.md** — flip the "Seam (deferred): rollback / compensation" block to a **Shipped (s008)** note, with the resolved semantics:
  - per-command `Command.Rollback` (`{run, timeout?}`, scalar or map); reverse-over-succeeded on an intra-pipeline failure; best-effort (a failed compensator does not change the outcome); **exit 3 preserved**, task unchanged, **no history**; rollback placeholders expanded **eagerly** (a bad rollback template is exit 1 before any side effect); the exec `Runner.Compensate` owns the `↩ compensating` phase.
  - keep the multi-step `--atomic` / `advance` compensation across several edges as the **still-parked** remainder. Update the `--atomic` cross-reference. Keep `.md` and `.ru.md` in sync (English is source of truth).

- [ ] **Step 3: CLI_REFERENCE.md / .ru** — under "Transition commands", document the `rollback:` sub-field (scalar or `{run, timeout}`), a worked example, the reverse-over-succeeded + best-effort semantics, that a block with compensation stays **exit 3**, and the `mtt types` `↩` annotation. Update the `--atomic` row note ("rollback/compensation seam planned") to "intra-pipeline compensation shipped (s008); cross-edge atomic abort still planned".

- [ ] **Step 4: CLAUDE.md files** — one or two lines each:
  - `pkg/mtt/CLAUDE.md`: the `Command` VO gains `Rollback *Command` (an optional leaf compensator; `Valid()` rejects a nested rollback).
  - `internal/core/CLAUDE.md`: `expandCommands` also expands `Rollback.Run` (eager); `Runner` port gains `Compensate`; `Transitioner` compensates the succeeded-prefix rollbacks (reverse, best-effort) on a block — task unchanged, no history, exit 3.
  - `internal/adapter/exec/CLAUDE.md`: `Compensate` (best-effort, labeled `↩` phase, operational failure → -1); `runReport` shared with `Run`.
  - `internal/adapter/yaml/CLAUDE.md`: `ymlCommand.rollback` (recursive scalar|map) + `toDomain` deep-copy.
  - `internal/cli/CLAUDE.md`: `mtt types` renders `↩ <rollback>`; the block message carries the compensation summary (from core).

- [ ] **Step 5: model.go / TASKS.md / sessions** —
  - `docs/architecture/model.go`: add `Rollback *Command` to the `Command` VO block; add `Compensate([]Command) []Check` to the `Runner` interface with a doc note; update the rollback seam/gap note to "shipped s008".
  - `TASKS.md`: tick **e4_t10** (rollback/compensation shipped — intra-pipeline; multi-step abort still later); update the s008 mention in the phase-3 summary.
  - `sessions/README.md`: mark row 008 ✅ (drop the leading blank in the `008 |` row → `008 ✅`).
  - `NEXT_SESSION.md`: move the s008 line into "Where we are"; add a "Carry-over lessons (008)" block (per-command compensator; core computes the plan / exec executes best-effort; single-source `failIdx`; no-history-on-block preserved; generic-command e2e); set next up = **s008.5 dogfood enablers**.
  - `sessions/008_rollback.md`: fill the **Done** section (what shipped, deviations, follow-ups) and flip Status to `done`.

- [ ] **Step 6: Gate + commit**

```bash
make check
git add -A
git commit -m "docs(s008): rollback shipped — DESIGN/.ru, CLI_REFERENCE/.ru, CLAUDE x5, model.go, TASKS, sessions, NEXT_SESSION; bump 0.8.0-dev

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 7: Final verification**

Run: `make check`
Expected: green (gofmt + vet + lint + `go test -race -cover ./...` + build).
Then confirm the acceptance scenario once by hand or via `go test ./internal/cli/ -run TestScript -v` (rollback green).

---

## Self-Review

**1. Spec coverage** — every spec section maps to a task:
- §1 `Command.Rollback` + `Valid()` leaf → Task 1. §1b `Config.Validate` note → Task 1 (Step 6, message + test).
- §2 `ymlCommand.Rollback` recursion + `toDomain` → Task 2.
- §3a eager rollback expansion → Task 3. §3b `Runner.Compensate` port + CONTRACT → Task 5 (Step 1) / Task 4 (impl). §3c `firstFailure`-index / `rollbacksBefore` / `compSummary` / `block` → Task 5.
- §4 exec `Compensate` + `runReport` → Task 4.
- §5 `mtt types` `↩` + block summary (via core error) → Task 6.
- §6 tests: `Command.Valid` (T1), `ymlCommand`/`toDomain` (T2), `expandCommands` rollback (T3), `Transitioner` compensation incl. operational path + best-effort (T5), `exec.Compensate` (T4), `mtt types` `↩` (T6), e2e `rollback.txt` (T6).
- §7 docs + version → Task 7.

**2. Placeholder scan** — no TBD/TODO; every code step shows complete code; every test step shows the assertion.

**3. Type consistency** — `Command.Rollback *Command` (T1) used identically in `ymlCommand.toDomain` (T2), `expandOne` (T3), `rollbacksBefore` (T5), `writeTypeBlock` (T6). `Runner.Compensate(commands []mtt.Command) []mtt.Check` matches across the port (T5), exec impl (T4), and `fakeRunner` (T5). `firstFailure` returns `(int, mtt.Check, bool)` at its one call site (T5). `plural` lives in `exec` (T4); `core.compSummary` uses an inline `noun` (no cross-package dependency).
