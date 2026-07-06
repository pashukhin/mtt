# Structured Commands (s007) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Evolve a transition's `commands` from `[]string` into a value object `Command{Run, Timeout}` with placeholder expansion on `Run` (`.ID`/`.Type`/`.From`/`.To`) and a per-command timeout overriding the global `command_timeout` — so a transition can create the task's branch (`git checkout -b task/{{.ID}}`) and a fast gate can fail fast.

**Architecture:** `cli → core → port ← adapter`. `pkg/mtt` gains the pure `Command` VO (stores the raw template; template-agnostic). The YAML adapter's DTO custom-unmarshals a bare scalar **or** a `{run, timeout}` map into one `Command` (back-compat). `core.Transitioner` expands placeholders (via `text/template` over a 4-field whitelist struct) before calling the `Runner`; the exec adapter resolves the per-command timeout (falling back to the global it was constructed with).

**Tech Stack:** Go 1.23, `gopkg.in/yaml.v3`, `text/template`, cobra, `go-internal/testscript` (e2e). Storage: YAML file-per-task.

## Global Constraints

- Test before code (TDD: red → green → refactor). `make check` (gofmt + vet + golangci-lint v2 + `go test -race -cover` + build) green before every commit.
- SOLID / DRY / KISS / clean architecture (hexagonal). `core` never imports `adapter/*`; adapters carry no business rules; ID/serialization live in the adapter.
- Everything typed (`mtt.TaskID`/`TypeName`/`StatusName`); string conversion only at the cli/adapter boundary.
- `pkg/mtt` stays pure: no yaml/json tags, no adapter fields, **no template knowledge** (it stores the raw command template; core expands).
- Per-task branch `feat/s007-structured-commands` (already created) → PR → CI green → squash.
- Commit trailer: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.
- Version bump `0.6.7-dev → 0.7.0-dev` (Task 9).
- Authoritative spec: `docs/superpowers/specs/2026-07-06-session-007-structured-commands-design.md`.

---

## File Structure

- `pkg/mtt/command.go` (create) — the `Command` value object + `Valid()`.
- `pkg/mtt/config.go` (modify) — `Transition.Commands` becomes `[]Command`.
- `pkg/mtt/validate.go` (modify) — validate each command in `validateFlow`.
- `internal/adapter/yaml/dto.go` (modify) — `ymlCommand` + custom `UnmarshalYAML` (scalar|map + duration parse); `ymlTransition.Commands []ymlCommand`; `toDomain` mapping.
- `internal/core/runner.go` (modify) — `Runner.Run([]mtt.Command)`.
- `internal/core/expand.go` (create) — `cmdContext` + `expandCommands` (the whitelist + template expansion).
- `internal/core/transition.go` (modify) — expand before the gate; pass expanded commands.
- `internal/adapter/exec/exec.go` (modify) — per-command timeout with global fallback.
- `internal/cli/types.go` (modify) — render `c.Run` (+ per-command timeout annotation).
- `internal/cli/root.go` (modify) — version bump (Task 9).
- Tests: `pkg/mtt/command_test.go`, `pkg/mtt/validate_test.go`, `internal/adapter/yaml/dto_test.go`, `internal/core/expand_test.go`, `internal/core/transition_test.go`, `internal/adapter/exec/exec_test.go`, `internal/cli/types_test.go`, `internal/cli/testdata/structured_commands.txt`.
- Docs (Task 9): DESIGN.md/.ru, CLI_REFERENCE.md/.ru, CLAUDE.md ×5, docs/architecture/model.go, TASKS.md, sessions/README.md, sessions/007_structured_commands.md, NEXT_SESSION.md.

---

## Task 1: `Command` value object (pkg/mtt)

**Files:**
- Create: `pkg/mtt/command.go`
- Test: `pkg/mtt/command_test.go`

**Interfaces:**
- Produces: `type Command struct { Run string; Timeout time.Duration }`; `func (c Command) Valid() bool`.

- [ ] **Step 1: Write the failing test** — `pkg/mtt/command_test.go`:

```go
package mtt

import (
	"testing"
	"time"
)

func TestCommandValid(t *testing.T) {
	cases := []struct {
		name string
		cmd  Command
		want bool
	}{
		{"run only", Command{Run: "make test"}, true},
		{"run + timeout", Command{Run: "make test", Timeout: 30 * time.Second}, true},
		{"empty run", Command{Run: ""}, false},
		{"negative timeout", Command{Run: "make test", Timeout: -1}, false},
	}
	for _, tc := range cases {
		if got := tc.cmd.Valid(); got != tc.want {
			t.Errorf("%s: Valid() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/mtt/ -run TestCommandValid`
Expected: FAIL — `undefined: Command`.

- [ ] **Step 3: Write minimal implementation** — `pkg/mtt/command.go`:

```go
package mtt

import "time"

// Command is one gate step of a transition: a shell command (Run) with an
// optional per-command timeout that overrides the adapter's global
// command_timeout (zero = fall back to the global). Run holds a raw template
// (e.g. "git checkout -b task/{{.ID}}"); the domain does not interpret it —
// core expands the placeholders before the runner runs it.
type Command struct {
	Run     string
	Timeout time.Duration
}

// Valid reports whether the command is well-formed: a non-empty Run and a
// non-negative Timeout. (Mirrors the StatusKind/CurrentAction Valid() idiom.)
func (c Command) Valid() bool { return c.Run != "" && c.Timeout >= 0 }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/mtt/ -run TestCommandValid`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/mtt/command.go pkg/mtt/command_test.go
git commit -m "feat(s007): add pkg/mtt.Command value object {Run, Timeout}"
```

---

## Task 2: Flip `Transition.Commands` to `[]Command` + migrate all consumers

This is a mechanical, behavior-preserving type migration (no expansion, no per-command timeout yet — every command is `{Run, Timeout:0}`). It compiles and all existing tests pass unchanged in behavior. Expansion (Task 6) and per-command timeout (Task 5) build on it; the `{run, timeout}` YAML form arrives in Task 3.

**Files:**
- Modify: `pkg/mtt/config.go` (Transition.Commands type)
- Modify: `internal/core/runner.go`, `internal/core/transition.go`
- Modify: `internal/adapter/exec/exec.go`
- Modify: `internal/adapter/yaml/dto.go` (toDomain mapping)
- Modify: `internal/cli/types.go`
- Test (update): `internal/core/transition_test.go`, `internal/adapter/exec/exec_test.go`, `internal/cli/types_test.go`

**Interfaces:**
- Consumes: `mtt.Command` (Task 1).
- Produces: `Transition.Commands []mtt.Command`; `core.Runner.Run([]mtt.Command) ([]mtt.Check, error)`; `exec.(*Runner).Run([]mtt.Command)`.

- [ ] **Step 1: Change the domain field** — `pkg/mtt/config.go`, in `Transition`:

```go
type Transition struct {
	From        StatusName
	To          StatusName
	Description string
	Commands    []Command
	Current     CurrentAction // set|clear the personal current pointer when traversed (empty = no effect)
}
```

- [ ] **Step 2: Verify the build breaks (drives the migration)**

Run: `go build ./...`
Expected: FAIL — type mismatches at `internal/core/runner.go`, `internal/core/transition.go`, `internal/adapter/exec/exec.go`, `internal/adapter/yaml/dto.go`, `internal/cli/types.go`.

- [ ] **Step 3: Update the `Runner` port** — `internal/core/runner.go`, change the interface method:

```go
// Runner executes a transition's commands in order and reports each result. It is
// defined here (only core uses it), implemented in internal/adapter/exec, and
// faked in tests. A non-zero exit is DATA (recorded in a Check), not a Go error;
// the returned error signals an operational failure (a command could not launch
// or timed out). At this boundary each Command's Run is ALREADY EXPANDED by core
// (see internal/core/expand.go); the runner only runs and reports.
type Runner interface {
	Run(commands []mtt.Command) ([]mtt.Check, error)
}
```

- [ ] **Step 4: Update `transition.go` call site** — the gate call still passes `edge.Commands` directly (both sides are now `[]mtt.Command`; expansion is added in Task 6). No code change is needed on line 68 itself (`tr.runner.Run(edge.Commands)` now type-checks). Confirm it compiles after Steps 5–6.

- [ ] **Step 5: Update the exec adapter** — `internal/adapter/exec/exec.go`, change `Run` to iterate `mtt.Command` (still one global timeout for all — per-command comes in Task 5):

```go
func (r *Runner) Run(commands []mtt.Command) ([]mtt.Check, error) {
	checks := make([]mtt.Check, 0, len(commands))
	for _, cmd := range commands {
		_, _ = fmt.Fprintf(r.progress, "▶ %s\n", cmd.Run)
		start := time.Now()
		exit, err := r.runOne(cmd.Run)
		elapsed := time.Since(start).Round(time.Millisecond)
		mark := "✓"
		if exit != 0 || err != nil {
			mark = "✗"
		}
		_, _ = fmt.Fprintf(r.progress, "%s %s (exit %d, %s)\n", mark, cmd.Run, exit, elapsed)
		checks = append(checks, mtt.Check{Cmd: cmd.Run, Exit: exit})
		if err != nil {
			return checks, err
		}
		if exit != 0 {
			return checks, nil
		}
	}
	return checks, nil
}
```

(`runOne(cmd string)` is unchanged in this task; it still uses `r.timeout`.)

- [ ] **Step 6: Update the YAML `toDomain` mapping** — `internal/adapter/yaml/dto.go`, in the transitions loop (`ymlTransition.Commands` is still `[]string` in this task), map each string to a `Command`:

```go
for _, yr := range yt.Transitions {
	cmds := make([]mtt.Command, 0, len(yr.Commands))
	for _, run := range yr.Commands {
		cmds = append(cmds, mtt.Command{Run: run})
	}
	t.Transitions = append(t.Transitions, mtt.Transition{
		From: mtt.StatusName(yr.From), To: mtt.StatusName(yr.To),
		Description: yr.Description, Commands: cmds, Current: mtt.CurrentAction(yr.Current),
	})
}
```

- [ ] **Step 7: Update the `mtt types` render** — `internal/cli/types.go`, the command loop:

```go
for _, c := range tr.Commands {
	fmt.Fprintf(b, "        $ %s\n", c.Run)
}
```

- [ ] **Step 8: Update the core tests** — `internal/core/transition_test.go`: change the fake runner and `flowCfg` to the new type. Replace the fake and helper:

```go
type fakeRunner struct {
	checks  []mtt.Check
	err     error
	called  bool
	gotCmds []mtt.Command
}

func (f *fakeRunner) Run(commands []mtt.Command) ([]mtt.Check, error) {
	f.called = true
	f.gotCmds = commands
	return f.checks, f.err
}

// strCmds wraps bare command strings as Commands (no per-command timeout).
func strCmds(ss []string) []mtt.Command {
	if ss == nil {
		return nil
	}
	out := make([]mtt.Command, len(ss))
	for i, s := range ss {
		out[i] = mtt.Command{Run: s}
	}
	return out
}

// flowCfg is a one-type config: tbd →(a)→ in_progress →(b)→ done, plus cancelled.
func flowCfg(cmdsA, cmdsB []string) mtt.Config {
	return mtt.Config{
		Version: 1,
		Types: []mtt.Type{{
			Name:    "task",
			Default: true,
			Flow: mtt.Flow{
				Statuses: []mtt.Status{
					{Name: "tbd", Kind: mtt.KindInitial},
					{Name: "in_progress", Kind: mtt.KindActive},
					{Name: "done", Kind: mtt.KindTerminal},
					{Name: "cancelled", Kind: mtt.KindTerminal},
				},
				Transitions: []mtt.Transition{
					{From: "tbd", To: "in_progress", Commands: strCmds(cmdsA)},
					{From: "tbd", To: "cancelled"},
					{From: "in_progress", To: "done", Commands: strCmds(cmdsB)},
					{From: "in_progress", To: "cancelled"},
				},
			},
		}},
	}
}
```

(The existing `TestTransition*` cases keep their `flowCfg([]string{...}, nil)` call sites unchanged.)

- [ ] **Step 9: Update the exec tests** — `internal/adapter/exec/exec_test.go`: add the `mtt` import and wrap every `Run([]string{...})` as `Run([]mtt.Command{...})`. Replace the four `Run(...)` calls:

```go
import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)
```

- `TestRunAllPass`: `Run([]mtt.Command{{Run: "true"}, {Run: "true"}})`
- `TestRunStopsAtFirstNonZero`: `Run([]mtt.Command{{Run: "true"}, {Run: "false"}, {Run: "true"}})`
- `TestRunTimeout`: `Run([]mtt.Command{{Run: "sleep 1"}})`
- `TestRunStreamsProgressAndSeparatesOutput`: `Run([]mtt.Command{{Run: "echo $((3+4))"}, {Run: "true"}})`
- `TestRunProgressMarksFailure`: `Run([]mtt.Command{{Run: "false"}})`

(Progress/`Check.Cmd` assertions like `"false"` and `"▶ echo $((3+4))"` stay valid — `cmd.Run` equals the old string.)

- [ ] **Step 10: Update the cli types test** — `internal/cli/types_test.go`, in `TestFormatTypes` change the transition literal:

```go
{From: "doing", To: "done", Description: "gate", Commands: []mtt.Command{{Run: "make test"}}},
```

(The expected `"        $ make test\n"` line is unchanged.)

- [ ] **Step 11: Run the gate**

Run: `make check`
Expected: PASS (build + all tests green; behavior identical to before the migration).

- [ ] **Step 12: Commit**

```bash
git add pkg/mtt/config.go internal/core/runner.go internal/adapter/exec/exec.go internal/adapter/yaml/dto.go internal/cli/types.go internal/core/transition_test.go internal/adapter/exec/exec_test.go internal/cli/types_test.go
git commit -m "refactor(s007): Transition.Commands []string -> []mtt.Command; migrate consumers"
```

---

## Task 3: `ymlCommand` custom `UnmarshalYAML` (scalar | map) + duration parse

Gives the YAML DTO the ability to read a bare scalar **or** a `{run, timeout}` map per command element, parsing the duration string. Both collapse to one `mtt.Command`; nothing above the adapter branches on the form.

**Files:**
- Modify: `internal/adapter/yaml/dto.go`
- Test: `internal/adapter/yaml/dto_test.go`

**Interfaces:**
- Produces: `type ymlCommand struct { Run string; Timeout time.Duration }` implementing `UnmarshalYAML`; `ymlTransition.Commands []ymlCommand`.

- [ ] **Step 1: Write the failing tests** — append to `internal/adapter/yaml/dto_test.go` (add imports `time` and `goyaml "gopkg.in/yaml.v3"` if absent):

```go
func TestYmlCommandUnmarshalScalar(t *testing.T) {
	var c ymlCommand
	if err := goyaml.Unmarshal([]byte(`"make test"`), &c); err != nil {
		t.Fatal(err)
	}
	if c.Run != "make test" || c.Timeout != 0 {
		t.Fatalf("got %+v, want {Run: make test, Timeout: 0}", c)
	}
}

func TestYmlCommandUnmarshalMap(t *testing.T) {
	var c ymlCommand
	if err := goyaml.Unmarshal([]byte("{run: make test, timeout: 30s}"), &c); err != nil {
		t.Fatal(err)
	}
	if c.Run != "make test" || c.Timeout != 30*time.Second {
		t.Fatalf("got %+v, want {Run: make test, Timeout: 30s}", c)
	}
}

func TestYmlCommandUnmarshalBadDuration(t *testing.T) {
	var c ymlCommand
	if err := goyaml.Unmarshal([]byte("{run: x, timeout: banana}"), &c); err == nil {
		t.Fatal("want error for a bad duration")
	}
}

func TestToDomainCommandsMixed(t *testing.T) {
	src := `
version: 1
project: {name: p}
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: tbd, kind: initial}
      - {name: doing, kind: active}
      - {name: done, kind: terminal}
    transitions:
      - {from: tbd, to: doing, commands: ["make lint", {run: "make test", timeout: 30s}]}
      - {from: doing, to: done}
`
	var yc ymlConfig
	if err := goyaml.Unmarshal([]byte(src), &yc); err != nil {
		t.Fatal(err)
	}
	cfg, _ := yc.toDomain()
	cmds := cfg.Types[0].Transitions[0].Commands
	if len(cmds) != 2 {
		t.Fatalf("cmds = %+v, want 2", cmds)
	}
	if cmds[0] != (mtt.Command{Run: "make lint"}) {
		t.Fatalf("cmd0 = %+v", cmds[0])
	}
	if cmds[1] != (mtt.Command{Run: "make test", Timeout: 30 * time.Second}) {
		t.Fatalf("cmd1 = %+v", cmds[1])
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/adapter/yaml/ -run 'TestYmlCommand|TestToDomainCommandsMixed'`
Expected: FAIL — `undefined: ymlCommand`.

- [ ] **Step 3: Add `ymlCommand` + `UnmarshalYAML`** — `internal/adapter/yaml/dto.go` (add imports `time` and `goyaml "gopkg.in/yaml.v3"`):

```go
// ymlCommand is one gate command on disk. It accepts either a bare scalar (a
// command string, back-compat) or a mapping {run, timeout}; both collapse to a
// single mtt.Command. The duration is parsed here so toDomain stays error-free.
type ymlCommand struct {
	Run     string
	Timeout time.Duration
}

// UnmarshalYAML decodes a scalar command string or a {run, timeout} mapping. The
// map branch decodes into a LOCAL string-Timeout alias (never back into
// ymlCommand — that would recurse; and yaml.v3 cannot decode "30s" into a
// time.Duration) then parses the duration.
func (c *ymlCommand) UnmarshalYAML(value *goyaml.Node) error {
	if value.Kind == goyaml.ScalarNode {
		c.Run = value.Value
		return nil
	}
	var raw struct {
		Run     string `yaml:"run"`
		Timeout string `yaml:"timeout"`
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
	return nil
}
```

- [ ] **Step 4: Switch the DTO field + mapping** — in `ymlTransition`, change `Commands []string` to `Commands []ymlCommand`; in `toDomain` map each element (replacing the Task 2 string loop):

```go
type ymlTransition struct {
	From        string       `yaml:"from"`
	To          string       `yaml:"to"`
	Description string       `yaml:"description"`
	Commands    []ymlCommand `yaml:"commands"`
	Current     string       `yaml:"current,omitempty"`
}
```

```go
cmds := make([]mtt.Command, 0, len(yr.Commands))
for _, c := range yr.Commands {
	cmds = append(cmds, mtt.Command{Run: c.Run, Timeout: c.Timeout})
}
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/adapter/yaml/ -run 'TestYmlCommand|TestToDomainCommandsMixed'`
Expected: PASS.

- [ ] **Step 6: Run the gate**

Run: `make check`
Expected: PASS (existing template/config loads still parse — every current command is a bare scalar).

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/yaml/dto.go internal/adapter/yaml/dto_test.go
git commit -m "feat(s007): ymlCommand custom UnmarshalYAML (scalar|map) + per-command timeout parse"
```

---

## Task 4: Validate commands in `Config.Validate`

**Files:**
- Modify: `pkg/mtt/validate.go`
- Test: `pkg/mtt/validate_test.go`

**Interfaces:**
- Consumes: `Command.Valid()` (Task 1).

- [ ] **Step 1: Write the failing test** — append to `pkg/mtt/validate_test.go`:

```go
func TestValidateRejectsInvalidCommand(t *testing.T) {
	cfg := Config{
		Version: 1,
		Types: []Type{{
			Name: "task", Default: true,
			Flow: Flow{
				Statuses: []Status{
					{Name: "tbd", Kind: KindInitial},
					{Name: "doing", Kind: KindActive},
					{Name: "done", Kind: KindTerminal},
				},
				Transitions: []Transition{
					{From: "tbd", To: "doing", Commands: []Command{{Run: ""}}}, // empty run
					{From: "doing", To: "done"},
				},
			},
		}},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("want error for a command with an empty run")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./pkg/mtt/ -run TestValidateRejectsInvalidCommand`
Expected: FAIL (Validate returns nil — the empty command is not yet checked).

- [ ] **Step 3: Add the check** — `pkg/mtt/validate.go`, inside `validateFlow`'s transitions loop, after the `tr.Current.Valid()` block:

```go
for _, cmd := range tr.Commands {
	if !cmd.Valid() {
		errs = append(errs, fmt.Errorf("type %q transition %q->%q: invalid command (empty run or negative timeout)", t.Name, tr.From, tr.To))
	}
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./pkg/mtt/ -run TestValidateRejectsInvalidCommand`
Expected: PASS.

- [ ] **Step 5: Run the gate**

Run: `make check`
Expected: PASS (default/coding templates carry only non-empty scalar commands → still valid).

- [ ] **Step 6: Commit**

```bash
git add pkg/mtt/validate.go pkg/mtt/validate_test.go
git commit -m "feat(s007): validate per-transition commands in Config.Validate"
```

---

## Task 5: Per-command timeout in the exec adapter (global fallback)

**Files:**
- Modify: `internal/adapter/exec/exec.go`
- Test: `internal/adapter/exec/exec_test.go`

**Interfaces:**
- Consumes: `mtt.Command.Timeout`.
- Produces: `runOne(cmd string, timeout time.Duration) (int, error)`.

- [ ] **Step 1: Write the failing tests** — append to `internal/adapter/exec/exec_test.go`:

```go
func TestRunPerCommandTimeoutOverridesGlobal(t *testing.T) {
	// Global is generous; a tight per-command timeout must fire first.
	_, err := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard).
		Run([]mtt.Command{{Run: "sleep 1", Timeout: 20 * time.Millisecond}})
	if err == nil {
		t.Fatal("want a per-command timeout error, got nil")
	}
}

func TestRunFallsBackToGlobalTimeout(t *testing.T) {
	// No per-command timeout -> the (tight) global applies and fires.
	_, err := NewRunner(t.TempDir(), 20*time.Millisecond, io.Discard, io.Discard).
		Run([]mtt.Command{{Run: "sleep 1"}})
	if err == nil {
		t.Fatal("want a global timeout error, got nil")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/adapter/exec/ -run 'TestRunPerCommandTimeoutOverridesGlobal|TestRunFallsBackToGlobalTimeout'`
Expected: `TestRunPerCommandTimeoutOverridesGlobal` FAILs (the per-command timeout is ignored → `sleep 1` completes under the 1m global → no error). (`FallsBack` may already pass — the global is honored today.)

- [ ] **Step 3: Resolve the effective timeout** — `internal/adapter/exec/exec.go`. In `Run`, compute the effective timeout per command and pass it to `runOne`:

```go
		timeout := cmd.Timeout
		if timeout <= 0 {
			timeout = r.timeout
		}
		exit, err := r.runOne(cmd.Run, timeout)
```

And change `runOne` to take it:

```go
// runOne runs a single command with the given timeout, streaming its output to
// cmdOut and returning its exit code. A clean non-zero exit yields (code, nil); a
// timeout or launch failure yields (-1, error).
func (r *Runner) runOne(cmd string, timeout time.Duration) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	name, args := shell(cmd)
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = r.dir
	c.Stdout = r.cmdOut
	c.Stderr = r.cmdOut
	err := c.Run()
	if err == nil {
		return 0, nil
	}
	if ctx.Err() == context.DeadlineExceeded {
		return -1, fmt.Errorf("command %q timed out after %s", cmd, timeout)
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), nil // clean non-zero exit: data, not an error
	}
	return -1, fmt.Errorf("command %q failed to run: %w", cmd, err)
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/adapter/exec/ -run 'TestRun'`
Expected: PASS (all exec tests, including the existing `TestRunTimeout`).

- [ ] **Step 5: Run the gate**

Run: `make check`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/exec/exec.go internal/adapter/exec/exec_test.go
git commit -m "feat(s007): exec Runner honors per-command timeout (global as fallback)"
```

---

## Task 6: Placeholder expansion in core + wire into `Transitioner`

**Files:**
- Create: `internal/core/expand.go`
- Modify: `internal/core/transition.go`
- Test: `internal/core/expand_test.go`, `internal/core/transition_test.go`

**Interfaces:**
- Consumes: `mtt.Command`.
- Produces: `type cmdContext struct { ID, Type, From, To string }`; `func expandCommands(cmds []mtt.Command, ctx cmdContext) ([]mtt.Command, error)`.

- [ ] **Step 1: Write the failing unit tests** — `internal/core/expand_test.go`:

```go
package core

import (
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestExpandCommands(t *testing.T) {
	ctx := cmdContext{ID: "t1", Type: "task", From: "tbd", To: "in_progress"}
	out, err := expandCommands([]mtt.Command{{Run: "git checkout -b task/{{.ID}}", Timeout: 5 * time.Second}}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Run != "git checkout -b task/t1" {
		t.Fatalf("run = %q, want git checkout -b task/t1", out[0].Run)
	}
	if out[0].Timeout != 5*time.Second {
		t.Fatalf("timeout dropped: %v", out[0].Timeout)
	}
}

func TestExpandCommandsAllFields(t *testing.T) {
	ctx := cmdContext{ID: "t1", Type: "task", From: "tbd", To: "in_progress"}
	out, err := expandCommands([]mtt.Command{{Run: "echo {{.From}} {{.To}} {{.Type}} {{.ID}}"}}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Run != "echo tbd in_progress task t1" {
		t.Fatalf("run = %q", out[0].Run)
	}
}

func TestExpandCommandsUnknownField(t *testing.T) {
	if _, err := expandCommands([]mtt.Command{{Run: "echo {{.Title}}"}}, cmdContext{}); err == nil {
		t.Fatal("want an error for an unexposed field {{.Title}}")
	}
}

func TestExpandCommandsMalformed(t *testing.T) {
	if _, err := expandCommands([]mtt.Command{{Run: "echo {{.ID"}}, cmdContext{}); err == nil {
		t.Fatal("want a parse error for a malformed template")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/core/ -run TestExpandCommands`
Expected: FAIL — `undefined: cmdContext` / `undefined: expandCommands`.

- [ ] **Step 3: Implement `expandCommands`** — `internal/core/expand.go`:

```go
package core

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// cmdContext is the whitelist of placeholder fields exposed to a command
// template. Only these shape-safe identifiers are available — never free text
// (title/description) — so expansion cannot inject shell metacharacters, and a
// stray {{.Title}} is a template error (the struct's shape self-enforces the
// whitelist).
type cmdContext struct {
	ID   string
	Type string
	From string
	To   string
}

// expandCommands renders each command's Run template against ctx, returning new
// commands with the expanded Run and the unchanged Timeout. A malformed template
// (Parse) or a reference to an unexposed field (Execute) is an error.
func expandCommands(cmds []mtt.Command, ctx cmdContext) ([]mtt.Command, error) {
	if len(cmds) == 0 {
		return nil, nil
	}
	out := make([]mtt.Command, 0, len(cmds))
	for _, c := range cmds {
		tmpl, err := template.New("cmd").Parse(c.Run)
		if err != nil {
			return nil, fmt.Errorf("parse command %q: %w", c.Run, err)
		}
		var b strings.Builder
		if err := tmpl.Execute(&b, ctx); err != nil {
			return nil, fmt.Errorf("expand command %q: %w", c.Run, err)
		}
		out = append(out, mtt.Command{Run: b.String(), Timeout: c.Timeout})
	}
	return out, nil
}
```

- [ ] **Step 4: Run to verify the unit tests pass**

Run: `go test ./internal/core/ -run TestExpandCommands`
Expected: PASS.

- [ ] **Step 5: Wire expansion into `Transitioner`** — `internal/core/transition.go`. Hoist `from := t.Status` above the gate and expand before `runner.Run`. Replace the block from `var checks []mtt.Check` down to (but not including) `ts := tr.now()...`:

```go
	from := t.Status
	var checks []mtt.Check
	if !opts.NoRun {
		expanded, eerr := expandCommands(edge.Commands, cmdContext{
			ID:   string(t.ID),
			Type: string(t.Type),
			From: string(from),
			To:   string(to),
		})
		if eerr != nil {
			return mtt.Task{}, fmt.Errorf("expand commands for %s (%s->%s): %w", id, from, to, eerr)
		}
		checks, err = tr.runner.Run(expanded)
		if err != nil {
			return mtt.Task{}, fmt.Errorf("%w: %v", ErrBlocked, err)
		}
		if c, failed := firstFailure(checks); failed {
			return mtt.Task{}, fmt.Errorf("%w: command %q exited %d", ErrBlocked, c.Cmd, c.Exit)
		}
	}
	ts := tr.now().UTC().Truncate(time.Second)
	t.Status = to
```

(Delete the now-duplicate `from := t.Status` that was on the line above `ts := ...`.)

- [ ] **Step 6: Write the failing transition tests** — append to `internal/core/transition_test.go`:

```go
func TestTransitionExpandsPlaceholders(t *testing.T) {
	store := newMemStore(baseTask()) // t1, type task, status tbd
	runner := &fakeRunner{checks: []mtt.Check{{Cmd: "git checkout -b task/t1", Exit: 0}}}
	tr := NewTransitioner(store, flowCfg([]string{"git checkout -b task/{{.ID}}"}, nil), runner, testClock)

	if _, err := tr.Transition("t1", "in_progress", TransitionOptions{}); err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if len(runner.gotCmds) != 1 || runner.gotCmds[0].Run != "git checkout -b task/t1" {
		t.Fatalf("runner got %+v, want expanded 'git checkout -b task/t1'", runner.gotCmds)
	}
}

func TestTransitionExpandsFromTo(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{}
	tr := NewTransitioner(store, flowCfg([]string{"echo {{.From}} {{.To}}"}, nil), runner, testClock)

	if _, err := tr.Transition("t1", "in_progress", TransitionOptions{}); err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if runner.gotCmds[0].Run != "echo tbd in_progress" {
		t.Fatalf("expanded = %q, want 'echo tbd in_progress' (From = pre-move status)", runner.gotCmds[0].Run)
	}
}

func TestTransitionUnknownPlaceholderErrors(t *testing.T) {
	store := newMemStore(baseTask())
	tr := NewTransitioner(store, flowCfg([]string{"echo {{.Title}}"}, nil), &fakeRunner{}, testClock)

	_, err := tr.Transition("t1", "in_progress", TransitionOptions{})
	if err == nil || errors.Is(err, ErrBlocked) {
		t.Fatalf("want a plain expansion error (not ErrBlocked), got %v", err)
	}
	reloaded, _ := store.Get("t1")
	if reloaded.Status != "tbd" || len(reloaded.History) != 0 {
		t.Fatalf("task changed on an expansion error: %+v", reloaded)
	}
}

func TestTransitionNoRunSkipsExpansion(t *testing.T) {
	store := newMemStore(baseTask())
	// A template that would fail expansion; --no-run must skip expansion + gate.
	tr := NewTransitioner(store, flowCfg([]string{"echo {{.Title}}"}, nil), &fakeRunner{}, testClock)

	got, err := tr.Transition("t1", "in_progress", TransitionOptions{NoRun: true})
	if err != nil {
		t.Fatalf("--no-run must skip expansion; err = %v", err)
	}
	if got.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", got.Status)
	}
}
```

- [ ] **Step 7: Run to verify pass**

Run: `go test ./internal/core/ -run TestTransition`
Expected: PASS (new expansion tests + all existing transition tests).

- [ ] **Step 8: Run the gate**

Run: `make check`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/core/expand.go internal/core/expand_test.go internal/core/transition.go internal/core/transition_test.go
git commit -m "feat(s007): expand .ID/.Type/.From/.To placeholders in core before the gate"
```

---

## Task 7: Show per-command timeout in `mtt types`

**Files:**
- Modify: `internal/cli/types.go`
- Test: `internal/cli/types_test.go`

- [ ] **Step 1: Write the failing test** — append to `internal/cli/types_test.go` (add `"strings"` and `"time"` imports):

```go
func TestFormatTypesShowsCommandTimeout(t *testing.T) {
	cfg := mtt.Config{Types: []mtt.Type{{
		Name: "task", Default: true,
		Flow: mtt.Flow{
			Statuses: []mtt.Status{
				{Name: "tbd", Kind: mtt.KindInitial},
				{Name: "doing", Kind: mtt.KindActive},
				{Name: "done", Kind: mtt.KindTerminal},
			},
			Transitions: []mtt.Transition{
				{From: "tbd", To: "doing", Commands: []mtt.Command{{Run: "slow", Timeout: 30 * time.Second}}},
				{From: "doing", To: "done"},
			},
		},
	}}}
	out, err := formatTypes(cfg, map[string]string{"task": "t"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "$ slow  (timeout 30s)") {
		t.Fatalf("missing timeout annotation:\n%s", out)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run TestFormatTypesShowsCommandTimeout`
Expected: FAIL (no timeout annotation rendered).

- [ ] **Step 3: Render the annotation** — `internal/cli/types.go`, the command loop:

```go
for _, c := range tr.Commands {
	if c.Timeout > 0 {
		fmt.Fprintf(b, "        $ %s  (timeout %s)\n", c.Run, c.Timeout)
	} else {
		fmt.Fprintf(b, "        $ %s\n", c.Run)
	}
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/cli/ -run TestFormatTypes`
Expected: PASS (both the existing `TestFormatTypes` and the new timeout test).

- [ ] **Step 5: Run the gate**

Run: `make check`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/types.go internal/cli/types_test.go
git commit -m "feat(s007): mtt types shows a command's per-command timeout"
```

---

## Task 8: e2e — `structured_commands.txt`

**Files:**
- Create: `internal/cli/testdata/structured_commands.txt`

**Interfaces:**
- Consumes: the built `mtt` binary via the existing `testscript` harness (`internal/cli/script_test.go`), the `-- gated.yaml --` config-swap pattern, and host `git` (guarded by `[!exec:git] skip`).

- [ ] **Step 1: Write the e2e script** — `internal/cli/testdata/structured_commands.txt`:

```
# s007: placeholder branch creation on take-into-work + per-command timeout fail-fast.
[!exec:git] skip

exec git init
exec git config user.email t@example.com
exec git config user.name tester

exec mtt init --name demo
cp gated.yaml .mtt/config.yaml

exec mtt add 'implement feature'
stdout 'created t1'

# take into work: the tbd->in_progress gate runs `git checkout -b task/{{.ID}}`.
exec mtt in_progress t1
stdout 't1: tbd -> in_progress'

# the branch task/t1 was created. On a fresh repo with no commits the branch is
# UNBORN, so `git branch --list` is empty — assert the current branch instead.
exec git symbolic-ref --short HEAD
stdout '^task/t1$'

# the in_progress->done gate is `sleep 5` bounded by a tight 100ms per-command
# timeout under a 5m global: it must fail fast and block (exit non-zero), and the
# task must stay in_progress.
! exec mtt done t1
exec mtt show t1
stdout 'in_progress'

# back-compat: a bare-string command edge (tbd->cancelled: ["true"]) still gates.
exec mtt add 'scrap this'
stdout 'created t2'
exec mtt cancelled t2
stdout 't2: tbd -> cancelled'

-- gated.yaml --
version: 1
project:
  name: demo
command_timeout: 5m
types:
  - name: task
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, commands: ["git checkout -b task/{{.ID}}"]}
      - {from: tbd,         to: cancelled,   commands: ["true"]}
      - {from: in_progress, to: done,        commands: [{run: "sleep 5", timeout: 100ms}]}
      - {from: in_progress, to: cancelled}
```

- [ ] **Step 2: Run the e2e**

Run: `go test ./internal/cli/ -run 'TestScript/structured_commands'`
Expected: PASS. (If the runner assertion `stdout 'in_progress'` is too loose or too tight versus `mtt show`'s actual format, adjust the matcher to `mtt show`'s status line — anchor it, per the testscript-assert lesson. Verify branch creation runs in the project root: the exec `Runner` uses `cwd = projectRoot`, which is the script's work dir where `git init` ran.)

- [ ] **Step 3: Run the gate**

Run: `make check`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/testdata/structured_commands.txt
git commit -m "test(s007): e2e structured_commands (placeholder branch, per-command timeout, back-compat)"
```

---

## Task 9: Docs + version bump + session Done

No behavior change — documentation, the domain-model snapshot, and the version string. `make check` (build) must stay green.

**Files:**
- Modify: `internal/cli/root.go` — `version = "0.7.0-dev"`.
- Modify: `docs/architecture/model.go` — the snapshot.
- Modify: `DESIGN.md`, `DESIGN.ru.md` — flip the "structured commands" seam to shipped.
- Modify: `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md` — the `commands` structured form + placeholders.
- Modify: `pkg/mtt/CLAUDE.md`, `internal/adapter/yaml/CLAUDE.md`, `internal/core/CLAUDE.md`, `internal/adapter/exec/CLAUDE.md`, `internal/cli/CLAUDE.md`.
- Modify: `TASKS.md`, `sessions/README.md`, `sessions/007_structured_commands.md`, `NEXT_SESSION.md`.

- [ ] **Step 1: Bump the version** — `internal/cli/root.go`:

```go
var version = "0.7.0-dev"
```

- [ ] **Step 2: Update the domain-model snapshot** — `docs/architecture/model.go`:
  - In the value-objects section (near `CurrentAction`), add the `Command` VO:

```go
// Command is one gate step of a transition: a shell command (Run, a raw
// template) with an optional per-command timeout overriding the adapter's global
// command_timeout (zero = fall back). core expands Run's placeholders before the
// runner runs it; pkg/mtt stays template-agnostic. [shipped s007]
type Command struct {
	Run     string
	Timeout time.Duration
}
```

  - `Transition.Commands` → `[]Command` (update the field + its comment to note the VO and s007).
  - `Runner.Run(commands []string)` → `Run(commands []Command)` (note: Run is expanded at this boundary; the exec adapter resolves per-command vs global timeout).
  - `ResolvedEdge.Commands []string` → `[]Command`.

- [ ] **Step 3: Update DESIGN.md** — in "Flow: executable transitions", note that a command is now `{run, timeout?}` with placeholder expansion (`.ID`/`.Type`/`.From`/`.To`, shape-safe whitelist) and a per-command timeout overriding the global. Flip the "**Seam (deferred): structured commands**" blockquote to a **Shipped (s007)** note recording the four resolved decisions (timeout in the domain VO; expansion in core; structural whitelist, no quoting; Runner honors per-command timeout with global fallback) and the injection policy (never expose free text; shell-quote if ever exposed). Keep the `rollback?`/node-actions seams still deferred. Mirror into `DESIGN.ru.md` (keep bilingual sync).

- [ ] **Step 4: Update CLI_REFERENCE.md** — under Configuration, extend `command_timeout` to note the per-command override; document the structured `commands` form (`- {run: "…", timeout: 30s}` alongside a bare string) and the placeholder whitelist (`{{.ID}}`/`{{.Type}}`/`{{.From}}`/`{{.To}}`, expanded before the gate; free text is never interpolated); note `mtt types` shows a per-command timeout. Mirror into `CLI_REFERENCE.ru.md`.

- [ ] **Step 5: Update the CLAUDE.md files**:
  - `pkg/mtt/CLAUDE.md` — add the `Command` VO to the Task-model / value-objects bullet; note the domain stores the raw template (template-agnostic).
  - `internal/adapter/yaml/CLAUDE.md` — note `ymlCommand`'s custom `UnmarshalYAML` (scalar|map + duration parse; back-compat) under Responsibilities.
  - `internal/core/CLAUDE.md` — note `expandCommands` (whitelist expansion via `text/template`) and the `Runner.Run([]mtt.Command)` signature (Run expanded at the boundary; expansion before the gate).
  - `internal/adapter/exec/CLAUDE.md` — note the per-command timeout with the constructor global as fallback.
  - `internal/cli/CLAUDE.md` — note `mtt types` renders a command's `run` + optional timeout.

- [ ] **Step 6: Update the trackers**:
  - `TASKS.md` — tick `e4_t9` `[x]` with a one-line shipped summary; move the `coding` full-demo note to reference e5_t6.
  - `sessions/README.md` — mark the `007` roadmap row ✅ and add a "Decisions carried" line if useful.
  - `sessions/007_structured_commands.md` — fill the **Done** section (what shipped, deviations, follow-ups: rollback s008, coding-demo e5_t6), tick the plan checkboxes.
  - `NEXT_SESSION.md` — update "Where we are" (s007 shipped, `0.7.0-dev`), set the next task to **s008 rollback**, and add a "Carry-over lessons (007)" block (e.g. the domain-vs-policy split for per-command timeout; the self-enforcing template whitelist; `git symbolic-ref` for unborn-branch e2e asserts; scalar/map back-compat via element `UnmarshalYAML`).

- [ ] **Step 7: Run the gate**

Run: `make check`
Expected: PASS (build compiles with the new version + model.go; no test regressions).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "docs(s007): structured commands shipped — DESIGN/.ru, CLI_REFERENCE/.ru, CLAUDE ×5, model.go, TASKS, sessions, NEXT_SESSION; bump 0.7.0-dev"
```

---

## Self-Review

**Spec coverage:**
- Q1 (per-command timeout in domain VO) → Task 1 (`Command`), Task 5 (exec fallback). ✓
- Q2 (expansion in core) → Task 6 (`expandCommands` + wiring). ✓
- Q3 (structural whitelist, no quoting) → Task 6 (`cmdContext` 4-field struct; unknown-field test). ✓
- Q4 (`Runner.Run([]mtt.Command)`, per-command timeout resolved in adapter) → Task 2 (signature), Task 5 (resolution). ✓
- Back-compat scalar|map + duration parse → Task 3; back-compat proven in Task 8 e2e (cancel edge `["true"]`). ✓
- `Command.Valid()` + `Config.Validate` → Task 1 + Task 4. ✓
- `mtt types` render → Task 2 (`c.Run`) + Task 7 (timeout). ✓
- e2e (branch creation via `git symbolic-ref`; per-command timeout fail-fast; `[!exec:git] skip`) → Task 8. ✓
- No `MarshalYAML` (Config never marshaled) → confirmed; not in any task. ✓
- No `rollback` field now (s008) → excluded; noted in Task 9 docs. ✓
- Docs/model.go/version/session Done → Task 9. ✓

**Placeholder scan:** No TBD/TODO; every code and test step has concrete content; docs steps name exact files and the exact edit.

**Type consistency:** `Command{Run, Timeout}` used identically across tasks; `Runner.Run([]mtt.Command)` matches in `runner.go`/`exec.go`/`fakeRunner`; `cmdContext{ID,Type,From,To}` matches between `expand.go` and its wiring; `ymlCommand{Run, Timeout}` matches DTO + tests; `expandCommands`/`strCmds` names consistent.

---

## Execution Handoff

(Offered after saving — see below.)
