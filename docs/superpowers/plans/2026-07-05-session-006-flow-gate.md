# Session 006 — Flow gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `mtt status <id> <new>` — a single flow transition validated against the type's `transitions`, gated on its `commands` (all exit `0` or the move is blocked), recording a `history` entry.

**Architecture:** `cli → core → port ← adapter`. `core` gains a `Runner` port + flow sentinel errors + a `Transitioner` usecase (single-edge lookup, gate via `Runner`, append `history`, persist via `TaskStore.Update`). `internal/adapter/exec` implements `Runner` over `os/exec`; `core` tests fake it. The per-command timeout is a config-driven, adapter-level setting; `pkg/mtt` is unchanged (it already carries `HistoryEntry`/`Check`).

**Tech Stack:** Go 1.23, cobra, `gopkg.in/yaml.v3`, `os/exec`, `go-internal/testscript` (e2e).

## Global Constraints

- **TDD**: write the failing test first, watch it fail, implement minimal, watch it pass, commit. `make check` green before every commit.
- **Layering**: `core` imports only `pkg/mtt`; never `adapter/*`. Adapters carry no business rules. `pkg/mtt` gets **no** change this session.
- **No new store port**: `history` rides the `Task.History` field + `TaskStore.Update` (same rule as `depends_on` in s005).
- **Typed identities everywhere** (`mtt.TaskID`/`TypeName`/`StatusName`); convert `string↔typed` only at the cli/adapter boundary.
- **CLI output** via `fmt.Fprint(cmd.OutOrStdout(), …)`; errors via `RunE` return (never print+os.Exit in a command).
- **`golangci-lint` `unused`**: declare each new symbol in the task that first *uses* it.
- **testscript**: anchored assertions (e.g. `'t1: tbd → in_progress'`), not bare substrings.
- **Zero-match `--json`** = `[]` not `null` (already handled by existing `toTaskJSON`).
- **Version**: bump `0.5.0-dev` → `0.6.0-dev` (Task 7).
- **Default per-command timeout**: `5m`, sourced from config key `command_timeout`, overridable via `config.local.yaml`.
- **Exit codes**: `3` = gate blocked (`core.ErrBlocked`); `6` = invalid transition (`core.ErrInvalidTransition`); `1` = any other error; `0` = success.
- Every new `internal/` package keeps a thin `CLAUDE.md` current.

---

### Task 1: `core.Runner` port, flow sentinels, and `core.Transitioner`

**Files:**
- Create: `internal/core/runner.go` (the `Runner` port + `ErrBlocked`/`ErrInvalidTransition`)
- Create: `internal/core/transition.go` (the `Transitioner` usecase)
- Test: `internal/core/transition_test.go`

**Interfaces:**
- Consumes: `mtt.TaskStore` (`Get`/`Update`), `mtt.Config` (`TypeByName`), `mtt.Task`, `mtt.Transition`, `mtt.HistoryEntry`, `mtt.Check`, `mtt.ErrNotFound`.
- Produces:
  - `type Runner interface { Run(commands []string) ([]mtt.Check, error) }`
  - `var ErrBlocked, ErrInvalidTransition error`
  - `type TransitionOptions struct { Role string; By string; NoRun bool }`
  - `func NewTransitioner(store mtt.TaskStore, cfg mtt.Config, runner Runner, now func() time.Time) *Transitioner`
  - `func (tr *Transitioner) Transition(id mtt.TaskID, to mtt.StatusName, opts TransitionOptions) (mtt.Task, error)`

- [ ] **Step 1: Write the failing test** (`internal/core/transition_test.go`)

```go
package core

import (
	"errors"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// fakeRunner is the test double for the Runner port: it records the commands it
// was handed and returns canned checks/error without spawning a process.
type fakeRunner struct {
	checks   []mtt.Check
	err      error
	called   bool
	gotCmds  []string
}

func (f *fakeRunner) Run(commands []string) ([]mtt.Check, error) {
	f.called = true
	f.gotCmds = commands
	return f.checks, f.err
}

// memStore is a minimal in-memory TaskStore for usecase tests.
type memStore struct{ tasks map[mtt.TaskID]mtt.Task }

func newMemStore(ts ...mtt.Task) *memStore {
	m := &memStore{tasks: map[mtt.TaskID]mtt.Task{}}
	for _, t := range ts {
		m.tasks[t.ID] = t
	}
	return m
}
func (m *memStore) Create(t mtt.Task) (mtt.Task, error) { m.tasks[t.ID] = t; return t, nil }
func (m *memStore) Get(id mtt.TaskID) (mtt.Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return mtt.Task{}, mtt.ErrNotFound
	}
	return t, nil
}
func (m *memStore) List() ([]mtt.Task, error) {
	out := make([]mtt.Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		out = append(out, t)
	}
	return out, nil
}
func (m *memStore) Update(t mtt.Task) (mtt.Task, error) {
	if _, ok := m.tasks[t.ID]; !ok {
		return mtt.Task{}, mtt.ErrNotFound
	}
	m.tasks[t.ID] = t
	return t, nil
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
					{From: "tbd", To: "in_progress", Commands: cmdsA},
					{From: "tbd", To: "cancelled"},
					{From: "in_progress", To: "done", Commands: cmdsB},
					{From: "in_progress", To: "cancelled"},
				},
			},
		}},
	}
}

var testClock = func() time.Time { return time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC) }

func baseTask() mtt.Task {
	return mtt.Task{ID: "t1", Type: "task", Title: "A", Status: "tbd",
		Created: testClock(), Updated: testClock()}
}

func TestTransitionAppliesAndRecordsHistory(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{checks: []mtt.Check{{Cmd: "make lint", Exit: 0}}}
	tr := NewTransitioner(store, flowCfg([]string{"make lint"}, nil), runner, testClock)

	got, err := tr.Transition("t1", "in_progress", TransitionOptions{Role: "impl", By: "grisha"})
	if err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if got.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", got.Status)
	}
	if len(got.History) != 1 {
		t.Fatalf("history len = %d, want 1", len(got.History))
	}
	h := got.History[0]
	if h.From != "tbd" || h.To != "in_progress" || h.By != "grisha" || h.Role != "impl" {
		t.Fatalf("history entry = %+v", h)
	}
	if len(h.Checks) != 1 || h.Checks[0].Cmd != "make lint" || h.Checks[0].Exit != 0 {
		t.Fatalf("history checks = %+v", h.Checks)
	}
	if !got.Updated.Equal(testClock()) {
		t.Fatalf("updated = %v, want %v", got.Updated, testClock())
	}
}

func TestTransitionBlockedOnFailedGate(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{checks: []mtt.Check{{Cmd: "make test", Exit: 1}}}
	tr := NewTransitioner(store, flowCfg([]string{"make test"}, nil), runner, testClock)

	_, err := tr.Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	reloaded, _ := store.Get("t1")
	if reloaded.Status != "tbd" {
		t.Fatalf("status = %q, want unchanged tbd", reloaded.Status)
	}
	if len(reloaded.History) != 0 {
		t.Fatalf("history written on block: %+v", reloaded.History)
	}
}

func TestTransitionInvalidEdge(t *testing.T) {
	store := newMemStore(baseTask())
	tr := NewTransitioner(store, flowCfg(nil, nil), &fakeRunner{}, testClock)

	_, err := tr.Transition("t1", "done", TransitionOptions{}) // tbd → done not allowed
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
}

func TestTransitionNoRunBypassesRunner(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{err: errors.New("must not be called")}
	tr := NewTransitioner(store, flowCfg([]string{"make test"}, nil), runner, testClock)

	got, err := tr.Transition("t1", "in_progress", TransitionOptions{NoRun: true})
	if err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if runner.called {
		t.Fatalf("runner was called under --no-run")
	}
	if got.Status != "in_progress" || len(got.History) != 1 || len(got.History[0].Checks) != 0 {
		t.Fatalf("no-run result = %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core -run TestTransition -v`
Expected: FAIL — `undefined: NewTransitioner`, `undefined: Runner`, `undefined: ErrBlocked`, etc.

- [ ] **Step 3: Write the `Runner` port + sentinels** (`internal/core/runner.go`)

```go
package core

import (
	"errors"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Runner executes a transition's commands in order and reports each result. It is
// defined here (only core uses it), implemented in internal/adapter/exec, and
// faked in tests. A non-zero exit is DATA (recorded in a Check), not a Go error;
// the returned error signals an operational failure (a command could not launch
// or timed out).
type Runner interface {
	Run(commands []string) ([]mtt.Check, error)
}

// ErrBlocked is returned when a transition's gate does not pass (a command exited
// non-zero, or the runner failed operationally). The task is left unchanged.
var ErrBlocked = errors.New("mtt: transition blocked by a failed gate")

// ErrInvalidTransition is returned when the requested edge is not in the type's
// flow (no transition from the current status to the target).
var ErrInvalidTransition = errors.New("mtt: transition not allowed by the flow")
```

- [ ] **Step 4: Write the `Transitioner` usecase** (`internal/core/transition.go`)

```go
package core

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// TransitionOptions carry the non-flow inputs to a transition: the roles seam
// (Role) and the subject-identity seam (By), both recorded into history, and
// NoRun to bypass the edge's command gate.
type TransitionOptions struct {
	Role  string
	By    string
	NoRun bool
}

// Transitioner applies a SINGLE flow edge: validate id's current status → to
// against the type's transitions, run that edge's commands as a gate (unless
// NoRun), append a history entry, and persist via TaskStore.Update. No new port —
// history rides the Task.History field (like depends_on in s005).
type Transitioner struct {
	store  mtt.TaskStore
	cfg    mtt.Config
	runner Runner
	now    func() time.Time
}

// NewTransitioner wires the usecase; now is injected for deterministic tests.
func NewTransitioner(store mtt.TaskStore, cfg mtt.Config, runner Runner, now func() time.Time) *Transitioner {
	return &Transitioner{store: store, cfg: cfg, runner: runner, now: now}
}

// Transition moves id across one edge to `to`. Errors: task not found; unknown
// type (config drift); ErrInvalidTransition (no such edge); ErrBlocked (a gate
// command exited non-zero or the runner failed). On a block the task is not
// changed and no history is written.
func (tr *Transitioner) Transition(id mtt.TaskID, to mtt.StatusName, opts TransitionOptions) (mtt.Task, error) {
	t, err := tr.store.Get(id)
	if err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return mtt.Task{}, fmt.Errorf("task %q not found", id)
		}
		return mtt.Task{}, fmt.Errorf("load task %q: %w", id, err)
	}
	typ, ok := tr.cfg.TypeByName(t.Type)
	if !ok {
		return mtt.Task{}, fmt.Errorf("unknown type %q for task %q", t.Type, id)
	}
	edge, ok := findTransition(typ, t.Status, to)
	if !ok {
		return mtt.Task{}, fmt.Errorf("%w: %s cannot move %s → %s (allowed from %s: %s)",
			ErrInvalidTransition, id, t.Status, to, t.Status, strings.Join(allowedTargets(typ, t.Status), ", "))
	}
	var checks []mtt.Check
	if !opts.NoRun {
		checks, err = tr.runner.Run(edge.Commands)
		if err != nil {
			return mtt.Task{}, fmt.Errorf("%w: %v", ErrBlocked, err)
		}
		if c, failed := firstFailure(checks); failed {
			return mtt.Task{}, fmt.Errorf("%w: command %q exited %d", ErrBlocked, c.Cmd, c.Exit)
		}
	}
	from := t.Status
	ts := tr.now().UTC().Truncate(time.Second)
	t.Status = to
	t.History = append(t.History, mtt.HistoryEntry{
		At: ts, By: opts.By, Role: opts.Role, From: from, To: to, Checks: checks,
	})
	t.Updated = ts
	return tr.store.Update(t)
}

// findTransition returns the edge from → to in typ's flow, if any.
func findTransition(typ mtt.Type, from, to mtt.StatusName) (mtt.Transition, bool) {
	for _, e := range typ.Transitions {
		if e.From == from && e.To == to {
			return e, true
		}
	}
	return mtt.Transition{}, false
}

// allowedTargets lists the statuses reachable from `from` in one edge (for a
// helpful invalid-transition message).
func allowedTargets(typ mtt.Type, from mtt.StatusName) []string {
	var out []string
	for _, e := range typ.Transitions {
		if e.From == from {
			out = append(out, string(e.To))
		}
	}
	return out
}

// firstFailure returns the first non-zero check, if any.
func firstFailure(checks []mtt.Check) (mtt.Check, bool) {
	for _, c := range checks {
		if c.Exit != 0 {
			return c, true
		}
	}
	return mtt.Check{}, false
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/core -run TestTransition -v`
Expected: PASS (all four).

- [ ] **Step 6: Commit**

```bash
git add internal/core/runner.go internal/core/transition.go internal/core/transition_test.go
git commit -m "feat(core): Runner port + Transitioner (single-edge gated transition)"
```

---

### Task 2: `internal/adapter/exec` — the real `Runner`

**Files:**
- Create: `internal/adapter/exec/exec.go`
- Create: `internal/adapter/exec/exec_test.go`
- Create: `internal/adapter/exec/CLAUDE.md`

**Interfaces:**
- Consumes: `mtt.Check`.
- Produces: `func NewRunner(dir string, timeout time.Duration) *Runner` and `func (r *Runner) Run(commands []string) ([]mtt.Check, error)` (satisfies `core.Runner`).

- [ ] **Step 1: Write the failing test** (`internal/adapter/exec/exec_test.go`)

```go
package exec

import (
	"testing"
	"time"
)

func TestRunAllPass(t *testing.T) {
	checks, err := NewRunner(t.TempDir(), time.Minute).Run([]string{"true", "true"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(checks) != 2 || checks[0].Exit != 0 || checks[1].Exit != 0 {
		t.Fatalf("checks = %+v", checks)
	}
}

func TestRunStopsAtFirstNonZero(t *testing.T) {
	checks, err := NewRunner(t.TempDir(), time.Minute).Run([]string{"true", "false", "true"})
	if err != nil {
		t.Fatalf("Run: %v (non-zero exit is data, not an error)", err)
	}
	if len(checks) != 2 {
		t.Fatalf("ran %d commands, want to stop after 2", len(checks))
	}
	if checks[0].Exit != 0 || checks[1].Exit == 0 {
		t.Fatalf("checks = %+v", checks)
	}
	if checks[1].Cmd != "false" {
		t.Fatalf("failed cmd = %q, want false", checks[1].Cmd)
	}
}

func TestRunTimeout(t *testing.T) {
	_, err := NewRunner(t.TempDir(), time.Millisecond).Run([]string{"sleep 1"})
	if err == nil {
		t.Fatalf("want a timeout error, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/exec -v`
Expected: FAIL — `undefined: NewRunner`.

- [ ] **Step 3: Write the implementation** (`internal/adapter/exec/exec.go`)

```go
// Package exec implements core.Runner: it runs a transition's commands as gates,
// in the project root, each with a per-command timeout, stopping at the first
// non-zero exit. Commands are trusted project config (like a Makefile), never
// network input.
package exec

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Runner runs shell commands in dir, each bounded by timeout.
type Runner struct {
	dir     string
	timeout time.Duration
}

// NewRunner returns a Runner that executes commands with cwd=dir and the given
// per-command timeout.
func NewRunner(dir string, timeout time.Duration) *Runner {
	return &Runner{dir: dir, timeout: timeout}
}

// Run executes commands in order, recording a Check per executed command. It
// stops at the first non-zero exit (a Check, not an error). An operational
// failure (launch error or timeout) returns the checks so far plus a non-nil
// error.
func (r *Runner) Run(commands []string) ([]mtt.Check, error) {
	checks := make([]mtt.Check, 0, len(commands))
	for _, cmd := range commands {
		exit, err := r.runOne(cmd)
		checks = append(checks, mtt.Check{Cmd: cmd, Exit: exit})
		if err != nil {
			return checks, err
		}
		if exit != 0 {
			return checks, nil
		}
	}
	return checks, nil
}

// runOne runs a single command, returning its exit code. A clean non-zero exit
// yields (code, nil); a timeout or launch failure yields (-1, error).
func (r *Runner) runOne(cmd string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	name, args := shell(cmd)
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = r.dir
	err := c.Run()
	if err == nil {
		return 0, nil
	}
	if ctx.Err() == context.DeadlineExceeded {
		return -1, fmt.Errorf("command %q timed out after %s", cmd, r.timeout)
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), nil // clean non-zero exit: data, not an error
	}
	return -1, fmt.Errorf("command %q failed to run: %w", cmd, err)
}

// shell selects the platform shell that runs a command string: cmd /c on
// Windows, sh -c elsewhere. (CI is Linux; the Windows branch is documented, not
// CI-tested.)
func shell(cmd string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", cmd}
	}
	return "sh", []string{"-c", cmd}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/adapter/exec -v`
Expected: PASS (all three).

- [ ] **Step 5: Write the package `CLAUDE.md`** (`internal/adapter/exec/CLAUDE.md`)

```markdown
# internal/adapter/exec

The default **driven adapter for `core.Runner`** — the first driven port beyond storage. Runs a
transition's `commands` as gates.

## Responsibilities

- `NewRunner(dir, timeout)` / `Run(commands)` — run each command with `cwd=dir` and a **per-command**
  timeout (`context.WithTimeout`), in order, **stopping at the first non-zero exit**. Records a
  `mtt.Check{Cmd, Exit}` per executed command.
- A **non-zero exit is data** (a `Check`), not a Go error; the returned `error` signals only an
  **operational** failure (the command could not launch, or timed out — exit recorded as `-1`).
- Cross-platform shell seam `shell(cmd)`: `sh -c` on Unix, `cmd /c` on Windows. Commands are trusted
  project config (like a Makefile), never network input.

## Boundaries

- No flow logic, no history, no gating decision — `core.Transitioner` decides blocked-vs-applied from the
  returned checks/error. This package only *runs* and *reports*.
- The project root (`dir`) and timeout are injected by the CLI (from `.mtt/` config); this package holds no
  config knowledge.
```

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/exec/
git commit -m "feat(exec): Runner adapter (per-command timeout, cwd=root, shell seam)"
```

---

### Task 3: `command_timeout` config (adapter-level setting)

**Files:**
- Modify: `internal/adapter/yaml/dto.go:16-19` (add `CommandTimeout` field to `ymlConfig`)
- Modify: `internal/adapter/yaml/load.go` (add `Settings` + parse + change `Load` return)
- Modify: `internal/adapter/yaml/task.go:28` and `internal/cli/types.go:24` (use `Settings.Prefixes`)
- Modify: `internal/adapter/yaml/load_test.go:15,40,50` (destructure `Settings`)
- Modify: `internal/adapter/yaml/templates/default.yaml`, `internal/adapter/yaml/templates/coding.yaml` (add `command_timeout: 5m`)
- Modify: goldens `internal/adapter/yaml/testdata/golden/default.yaml`, `coding.yaml` (regenerated)
- Test: `internal/adapter/yaml/load_test.go` (new timeout cases)

**Interfaces:**
- Consumes: existing `Load` internals (`ymlConfig`, `toDomain`, `checkPrefixes`).
- Produces:
  - `type Settings struct { Prefixes map[string]string; CommandTimeout time.Duration }`
  - `func Load(root string) (mtt.Config, Settings, error)` (signature change)
  - `const defaultCommandTimeout = 5 * time.Minute`

- [ ] **Step 1: Write the failing test** (append to `internal/adapter/yaml/load_test.go`)

```go
func TestLoadCommandTimeoutDefault(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	// remove the explicit command_timeout to prove the default kicks in
	writeConfigWithout(t, root, "command_timeout")
	_, s, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.CommandTimeout != 5*time.Minute {
		t.Fatalf("timeout = %s, want 5m default", s.CommandTimeout)
	}
}

func TestLoadCommandTimeoutFromConfig(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	_, s, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.CommandTimeout != 5*time.Minute { // the template sets command_timeout: 5m
		t.Fatalf("timeout = %s, want 5m from template", s.CommandTimeout)
	}
}
```

Also add the helper at the bottom of the test file:

```go
// writeConfigWithout rewrites .mtt/config.yaml dropping any line whose key is
// `key:` at column 0 — a crude way to simulate an absent top-level field.
func writeConfigWithout(t *testing.T, root, key string) {
	t.Helper()
	path := filepath.Join(root, dirName, configName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var kept []string
	for _, ln := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(ln, key+":") {
			continue
		}
		kept = append(kept, ln)
	}
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
```

Ensure the test file imports `os`, `path/filepath`, `strings`, `time` (add any missing).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/yaml -run TestLoadCommandTimeout -v`
Expected: FAIL — `Load` returns a `map[string]string`, not `Settings` (compile error: assignment/`.CommandTimeout` undefined).

- [ ] **Step 3: Add the DTO field** (`internal/adapter/yaml/dto.go`, in `ymlConfig`)

```go
type ymlConfig struct {
	Version        int        `yaml:"version"`
	Project        ymlProject `yaml:"project"`
	CommandTimeout string     `yaml:"command_timeout,omitempty"`
	Types          []ymlType  `yaml:"types"`
}
```

- [ ] **Step 4: Add `Settings` + parsing, change `Load`** (`internal/adapter/yaml/load.go`)

Replace the imports and `Load` body:

```go
import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	goyaml "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// defaultCommandTimeout is the per-command gate timeout when config omits
// command_timeout.
const defaultCommandTimeout = 5 * time.Minute

// Settings are the YAML adapter's non-domain, execution-level settings, returned
// alongside the pure domain Config: the type→prefix map (ID encoding) and the
// per-command gate timeout. Kept out of pkg/mtt (an external tracker adapter runs
// no local commands).
type Settings struct {
	Prefixes       map[string]string
	CommandTimeout time.Duration
}

// Load reads .mtt/config.yaml (+ the optional config.local.yaml overlay), maps to
// the domain Config, and returns the adapter Settings (prefixes + command
// timeout). Domain invariants (Config.Validate) are the caller's.
func Load(root string) (mtt.Config, Settings, error) {
	var yc ymlConfig
	if err := decodeInto(filepath.Join(root, dirName, configName), &yc, true); err != nil {
		return mtt.Config{}, Settings{}, err
	}
	if err := decodeInto(filepath.Join(root, dirName, localConfigName), &yc, false); err != nil {
		return mtt.Config{}, Settings{}, err
	}
	cfg, prefixes := yc.toDomain()
	if err := checkPrefixes(cfg, prefixes); err != nil {
		return mtt.Config{}, Settings{}, err
	}
	timeout, err := parseCommandTimeout(yc.CommandTimeout)
	if err != nil {
		return mtt.Config{}, Settings{}, err
	}
	return cfg, Settings{Prefixes: prefixes, CommandTimeout: timeout}, nil
}

// parseCommandTimeout parses the command_timeout string; empty yields the
// built-in default.
func parseCommandTimeout(s string) (time.Duration, error) {
	if s == "" {
		return defaultCommandTimeout, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("command_timeout %q: %w", s, err)
	}
	return d, nil
}
```

- [ ] **Step 5: Update `Load` callers that use the second return**

`internal/adapter/yaml/task.go:28` — change to use `Settings.Prefixes`:

```go
	_, settings, err := Load(s.root)
	if err != nil {
		return mtt.Task{}, err
	}
	prefix := settings.Prefixes[string(t.Type)]
```

`internal/cli/types.go` — change line 24 and its one usage (line 35):

```go
			cfg, settings, err := yaml.Load(root)
			// ...
			out, err := formatTypes(cfg, settings.Prefixes, filter)
```

(`formatTypes(cfg mtt.Config, prefixes map[string]string, filter string)` keeps its signature — only the argument passed changes.) The `cfg, _, err` callers in `add.go`, `tree.go`, `ready.go`, `list.go`, and `list_test`/`load_test` `_`-discard sites need **no** change (the ignored slot accepts any type). Update the two non-discarding `load_test.go` sites (`:15`, `:40`) to `cfg, s, err := Load(root)` and use `s.Prefixes` where they used `prefixes`; `:50` (`_, _, err`) is unchanged.

- [ ] **Step 6: Add `command_timeout: 5m` to both templates**

`internal/adapter/yaml/templates/default.yaml` and `coding.yaml` — insert after the `project:` block, before `types:`:

```yaml
project:
  name: {{.Name}}
command_timeout: 5m
types:
```

- [ ] **Step 7: Regenerate goldens and run the package tests**

Run: `go test ./internal/adapter/yaml -run TestRenderGolden -update`
Then: `go test ./internal/adapter/yaml -v`
Expected: PASS. The goldens `testdata/golden/default.yaml` and `coding.yaml` now contain `command_timeout: 5m`.

- [ ] **Step 8: Commit**

```bash
git add internal/adapter/yaml/ internal/cli/types.go
git commit -m "feat(yaml): config-driven command_timeout (adapter Settings, default 5m)"
```

---

### Task 4: CLI `mtt status` + `--role`/`--by` + exit codes

**Files:**
- Create: `internal/cli/status.go`
- Create: `internal/cli/status_test.go` (unit test for the exit-code mapping + role/by resolver)
- Modify: `internal/cli/root.go` (register command, add `--role`/`--by` persistent flags, `Execute() int`)
- Modify: `cmd/mtt/main.go` (`os.Exit(cli.Execute())`)
- Modify: `internal/cli/script_test.go` (`TestMain`'s `mtt` func now `os.Exit(Execute())` — the harness must track the new `int` signature)

**Interfaces:**
- Consumes: `projectRoot`, `jsonFlag`, `resolveDir` pattern, `yaml.Load` (→ `Settings.CommandTimeout`), `yaml.NewTaskStore`, `exec.NewRunner`, `core.NewTransitioner`, `core.TransitionOptions`, `core.ErrBlocked`, `core.ErrInvalidTransition`, `toTaskJSON`, `writeJSON`.
- Produces: `func newStatusCmd() *cobra.Command`, `func resolveRoleBy(cmd *cobra.Command) (role, by string)`, `func exitCode(err error) int`, `func Execute() int`.

- [ ] **Step 1: Write the failing test** (`internal/cli/status_test.go`)

```go
package cli

import (
	"errors"
	"testing"

	"github.com/pashukhin/mtt/internal/core"
)

func TestExitCode(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{nil, 0},
		{errors.New("boom"), 1},
		{core.ErrBlocked, 3},
		{errors.New("wrap: " + core.ErrInvalidTransition.Error()), 1}, // plain string does not match
		{core.ErrInvalidTransition, 6},
	}
	for _, c := range cases {
		if got := exitCode(c.err); got != c.want {
			t.Fatalf("exitCode(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}
```

Note: `exitCode(nil)` must return `0`; the switch handles `nil` last via the default only if it is not called with nil — implement `exitCode` to return `0` for a nil error explicitly (see Step 3).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestExitCode -v`
Expected: FAIL — `undefined: exitCode`.

- [ ] **Step 3: Change `Execute` to return an exit code** (`internal/cli/root.go`)

Replace `Execute` and add `exitCode`, register the command and the persistent flags:

```go
import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/core"
)

// ...inside NewRootCmd, after the existing persistent flags:
	root.PersistentFlags().String("role", "", "acting role, recorded in history (env MTT_ROLE)")
	root.PersistentFlags().String("by", "", "acting subject, recorded in history (env MTT_BY)")
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd(), newShowCmd(),
		newListCmd(), newEditCmd(), newTreeCmd(), newDepCmd(), newReadyCmd(), newStatusCmd())

// Execute runs the root command and returns a process exit code (0 success; 3
// gate blocked; 6 invalid transition; 1 any other error).
func Execute() int {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(root.ErrOrStderr(), "error:", err)
		return exitCode(err)
	}
	return 0
}

// exitCode maps an error to the CLI's exit-code taxonomy.
func exitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, core.ErrBlocked):
		return 3
	case errors.Is(err, core.ErrInvalidTransition):
		return 6
	default:
		return 1
	}
}
```

- [ ] **Step 4: Update `main`** (`cmd/mtt/main.go`)

```go
package main

import (
	"os"

	"github.com/pashukhin/mtt/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
```

- [ ] **Step 4b: Update the testscript harness** (`internal/cli/script_test.go`)

`Execute()` is now `int`, so the `TestMain` command func must not treat it as an `error`:

```go
	testscript.Main(m, map[string]func(){
		"mtt": func() { os.Exit(Execute()) },
	})
```

This also propagates the real exit code (3/6) to `! exec` assertions in the e2e.

- [ ] **Step 5: Write the `status` command** (`internal/cli/status.go`)

```go
package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/exec"
	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newStatusCmd builds `mtt status <id> <new>`: one gated flow transition.
func newStatusCmd() *cobra.Command {
	var noRun bool
	cmd := &cobra.Command{
		Use:   "status <id> <new-status>",
		Short: "Move a task across one flow edge (runs & gates the edge's commands)",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("provide a task id and a target status (example: mtt status t1 in_progress)")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			cfg, settings, err := yaml.Load(root)
			if err != nil {
				return err
			}
			id, err := mtt.NewTaskID(args[0])
			if err != nil {
				return err
			}
			to, err := mtt.NewStatusName(args[1])
			if err != nil {
				return err
			}
			role, by := resolveRoleBy(cmd)
			runner := exec.NewRunner(root, settings.CommandTimeout)
			tr := core.NewTransitioner(yaml.NewTaskStore(root), cfg, runner, time.Now)
			task, err := tr.Transition(id, to, core.TransitionOptions{Role: role, By: by, NoRun: noRun})
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
			}
			last := task.History[len(task.History)-1]
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s → %s\n", id, last.From, last.To); err != nil {
				return err
			}
			for _, c := range last.Checks {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s (exit %d)\n", c.Cmd, c.Exit); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noRun, "no-run", false, "skip the edge's commands (bypass the gate)")
	return cmd
}

// resolveRoleBy resolves --role/--by, falling back to MTT_ROLE/MTT_BY (mirrors
// resolveDir).
func resolveRoleBy(cmd *cobra.Command) (role, by string) {
	role, _ = cmd.Flags().GetString("role")
	if role == "" {
		role = os.Getenv("MTT_ROLE")
	}
	by, _ = cmd.Flags().GetString("by")
	if by == "" {
		by = os.Getenv("MTT_BY")
	}
	return role, by
}
```

- [ ] **Step 6: Run the CLI unit tests**

Run: `go test ./internal/cli -run 'TestExitCode|TestVersion' -v`
Expected: PASS. Then `go build ./...` to confirm `main` compiles.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/status.go internal/cli/status_test.go internal/cli/root.go internal/cli/script_test.go cmd/mtt/main.go
git commit -m "feat(cli): mtt status + --role/--by + exit codes 3/6"
```

---

### Task 5: `mtt show` history/audit section

**Files:**
- Modify: `internal/cli/show.go` (`formatTask` — add a `history:` block)
- Test: `internal/cli/show_test.go` (a unit test on `formatTask` with a history entry)

**Interfaces:**
- Consumes: `mtt.Task.History`, `mtt.HistoryEntry`, `mtt.Check`.
- Produces: (no new exported symbol; extends `formatTask` output).

- [ ] **Step 1: Write the failing test** (append to `internal/cli/show_test.go`)

```go
func TestFormatTaskRendersHistory(t *testing.T) {
	task := mtt.Task{
		ID: "t1", Type: "task", Title: "A", Status: "in_progress",
		Created: time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
		Updated: time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
		History: []mtt.HistoryEntry{{
			At:   time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
			By:   "grisha", Role: "impl", From: "tbd", To: "in_progress",
			Checks: []mtt.Check{{Cmd: "make lint", Exit: 0}},
		}},
	}
	out := formatTask(task, nil, nil)
	for _, want := range []string{"history:", "tbd → in_progress", "by grisha", "role impl", "make lint(0)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("formatTask output missing %q:\n%s", want, out)
		}
	}
}
```

Confirm the test file imports `time` and `strings` and `github.com/pashukhin/mtt/pkg/mtt`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestFormatTaskRendersHistory -v`
Expected: FAIL — output does not contain `history:`.

- [ ] **Step 3: Add the history block** (`internal/cli/show.go`, in `formatTask`, after the `updated:` line and before the description block)

```go
	if len(t.History) > 0 {
		b.WriteString("  history:\n")
		for _, h := range t.History {
			line := fmt.Sprintf("    %s  %s → %s", h.At.UTC().Format(time.RFC3339), h.From, h.To)
			var who []string
			if h.By != "" {
				who = append(who, "by "+h.By)
			}
			if h.Role != "" {
				who = append(who, "role "+h.Role)
			}
			if len(who) > 0 {
				line += "  (" + strings.Join(who, ", ") + ")"
			}
			fmt.Fprintln(&b, line)
			if len(h.Checks) > 0 {
				parts := make([]string, len(h.Checks))
				for i, c := range h.Checks {
					parts[i] = fmt.Sprintf("%s(%d)", c.Cmd, c.Exit)
				}
				fmt.Fprintf(&b, "      checks: %s\n", strings.Join(parts, " "))
			}
		}
	}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/cli -run TestFormatTaskRendersHistory -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/show.go internal/cli/show_test.go
git commit -m "feat(cli): render transition history in mtt show"
```

---

### Task 6: e2e — `status.txt` + `cancel_unblock.txt`

**Files:**
- Create: `internal/cli/testdata/scripts/status.txt`
- Create: `internal/cli/testdata/scripts/cancel_unblock.txt`

(The existing `script_test.go` runs every `testdata/scripts/*.txt` — no Go change needed. Confirm by reading it.)

- [ ] **Step 1: Write `status.txt`**

```
# mtt status: single gated flow transition
exec mtt init
# install a single-type flow with gate commands (task is a root default type)
cp gated.yaml .mtt/config.yaml
exec mtt add --title A
stdout 'created t1'

# green gate: tbd -> in_progress runs `true`, moves, records history
exec mtt status t1 in_progress
stdout 't1: tbd → in_progress'
exec mtt show t1
stdout '\[in_progress\]'
stdout 'history:'
stdout 'tbd → in_progress'

# red gate: in_progress -> done runs `false` -> blocked; task stays put
! exec mtt status t1 done
stderr 'blocked'
exec mtt show t1
stdout '\[in_progress\]'

# --no-run bypasses the gate
exec mtt status t1 done --no-run
stdout 't1: in_progress → done'

# invalid transition: done is terminal (no outgoing edge)
! exec mtt status t1 cancelled
stderr 'not allowed'

-- gated.yaml --
version: 1
project:
  name: demo
command_timeout: 5m
types:
  - name: task
    prefix: t
    default: true
    parents: []
    statuses:
      - {name: tbd, kind: initial}
      - {name: in_progress, kind: active}
      - {name: done, kind: terminal}
      - {name: cancelled, kind: terminal}
    transitions:
      - {from: tbd, to: in_progress, commands: ["true"]}
      - {from: tbd, to: cancelled}
      - {from: in_progress, to: done, commands: ["false"]}
      - {from: in_progress, to: cancelled}
```

- [ ] **Step 2: Write `cancel_unblock.txt`** (documents the kept cancelled-unblocks semantics with a now-reachable terminal)

```
# cancelling a blocker (a reachable terminal) unblocks its dependent
exec mtt init
cp gated.yaml .mtt/config.yaml
exec mtt add --title A
stdout 'created t1'
exec mtt add --title B
stdout 'created t2'
exec mtt dep add t2 t1

# t2 is blocked while t1 is tbd
exec mtt ready
! stdout 't2 '
stdout 't1 '

# cancel t1 (tbd -> cancelled, no gate) -> t2 becomes ready
exec mtt status t1 cancelled
stdout 't1: tbd → cancelled'
exec mtt ready
stdout 't2 '

-- gated.yaml --
version: 1
project:
  name: demo
command_timeout: 5m
types:
  - name: task
    prefix: t
    default: true
    parents: []
    statuses:
      - {name: tbd, kind: initial}
      - {name: in_progress, kind: active}
      - {name: done, kind: terminal}
      - {name: cancelled, kind: terminal}
    transitions:
      - {from: tbd, to: in_progress, commands: ["true"]}
      - {from: tbd, to: cancelled}
      - {from: in_progress, to: done, commands: ["false"]}
      - {from: in_progress, to: cancelled}
```

- [ ] **Step 3: Run the e2e**

Run: `go test ./internal/cli -run TestScript -v`
Expected: PASS (`status` and `cancel_unblock` among them).
If `ready` row anchoring fails, inspect the actual `mtt ready` line format via `taskLine` and adjust the anchor (keep it anchored to the id prefix, e.g. `'t2 '`).

- [ ] **Step 4: Commit**

```bash
git add internal/cli/testdata/scripts/status.txt internal/cli/testdata/scripts/cancel_unblock.txt
git commit -m "test(cli): e2e status gate + cancelled-unblock"
```

---

### Task 7: Version bump + docs sync + `make check`

**Files:**
- Modify: `internal/cli/root.go:11` (`version = "0.6.0-dev"`)
- Modify: `DESIGN.md`, `DESIGN.ru.md` (flow enforcement shipped; cancelled-blocker decision restated)
- Modify: `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md` (`mtt status` implemented; exit codes 3/6; `--role`/`--by`; `--no-run`; `command_timeout`)
- Modify: `internal/core/CLAUDE.md`, `internal/cli/CLAUDE.md` (Runner/Transitioner; status/role/by/exit codes)
- Modify: `docs/architecture/model.go` (Runner signature `Run(commands)`; Transitioner shipped; GAP #5 By partial; exit-code taxonomy)
- Modify: `TASKS.md` (tick e4_t1–e4_t4; note e4_t5 partial — single-edge `status` done, meta-walk is s007)
- Modify: `sessions/006_flow_gate.md` (fill Done), `sessions/README.md` (006 🔄 → ✅)
- Modify: `NEXT_SESSION.md` (handoff to s007; carry-over lessons)

- [ ] **Step 1: Bump the version** (`internal/cli/root.go`)

```go
var version = "0.6.0-dev"
```

Run: `go test ./internal/cli -run TestVersion -v` → PASS (the test compares against the `version` var).

- [ ] **Step 2: Update `docs/architecture/model.go`** — change the `Runner` interface to drop `dir`:

```go
// Runner executes a transition's Commands and reports each result. It is defined
// in CORE (only core uses it), implemented in internal/adapter/exec, faked in
// tests. Commands run in order with cwd=project root (held by the exec adapter,
// not passed here) and a per-command timeout, aborting on the first non-zero
// exit. [shipped s006]
type Runner interface {
	Run(commands []string) ([]Check, error)
}
```

Add a shipped note near `Advancer`/GAP #5 that `Transitioner` (single-edge `status`) shipped in s006, `By` is now written from `--by`/`MTT_BY` (durable config.local source still open), and exit codes 3/6 are realized.

- [ ] **Step 3: Update prose docs** — apply the "Docs to update" list from the spec: `DESIGN.md`(+ru), `CLI_REFERENCE.md`(+ru) (mark `mtt status` and exit codes 3/6 implemented, document `command_timeout` under Configuration, `--role`/`--by`/`MTT_ROLE`/`MTT_BY`), `CLAUDE.md` ×2 (core: `Runner`/`Transitioner`; cli: `status`, role/by resolver, `exitCode`/`Execute() int`), `TASKS.md` (tick e4_t1–t4; e4_t5 partial), `sessions/006_flow_gate.md` (Done section), `sessions/README.md` (`006 🔄` → `006 ✅`), `NEXT_SESSION.md` (handoff to s007 + carry-over lessons: first driven port + fake; config-driven adapter setting via `Settings`; exit-code plumbing via `Execute() int`; cancelled-unblock still deferred; single-edge lookup vs `ResolvedFlow` for s007).

- [ ] **Step 4: Run the full gate**

Run: `make check`
Expected: PASS (gofmt + vet + golangci-lint + `go test -race -cover` + build all green).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "docs(s006): sync DESIGN/CLI_REFERENCE/model.go/TASKS/CLAUDE + bump 0.6.0-dev"
```

---

## Self-review

**Spec coverage:** `mtt status` single edge → Task 1+4; `Runner` port + `exec` + fake → Task 1+2; `command_timeout` config → Task 3; `--role`/`--by` seams → Task 4; exit codes 3/6 → Task 4; history rendering in `show` → Task 5; cancelled-unblock documented via reachable e2e → Task 6; version bump + docs (DESIGN/CLI_REFERENCE/model.go/TASKS/CLAUDE ×3/sessions/NEXT_SESSION) → Task 2 (exec CLAUDE.md) + Task 7. All spec sections map to a task.

**Placeholder scan:** every code/test step shows real code; no TBD/TODO. The `sessions/006` Done section and `NEXT_SESSION` prose are genuine writing tasks (Task 7), not code placeholders.

**Type consistency:** `Runner.Run(commands []string) ([]mtt.Check, error)` identical in core (Task 1), exec (Task 2), and the CLI wiring (Task 4). `NewTransitioner(store, cfg, runner, now)` and `TransitionOptions{Role, By, NoRun}` consistent across Task 1 and Task 4. `Load(root) (mtt.Config, Settings, error)` with `Settings{Prefixes, CommandTimeout}` consistent across Task 3 and its consumer in Task 4. `exitCode`/`Execute() int` consistent across Task 4 and `main`.

**Layering:** `pkg/mtt` untouched (model already sufficient). `core` imports only `pkg/mtt`. `exec` imports only `pkg/mtt`. CLI wires them. No new store port (history rides `Task.History`).
