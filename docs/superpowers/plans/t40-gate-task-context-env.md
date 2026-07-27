# t40 — task-context environment for gate/post/event commands — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the task's read-only context to gate/`post:`/event commands as environment variables (`MTT_TASK_JSON` + `MTT_TASK_CHILDREN_JSON` + convenience scalars), keeping the `{{...}}` template whitelist shape-safe and unchanged.

**Architecture:** The `core.Runner` port gains a per-`Run` `env map[string]string` (the env varies per task/entity, so it rides the method, not the constructor). The exec adapter sets it on the `sh -c` process. `core` stays serialization-free: an injected `func(mtt.Task) map[string]string` — a **CLI closure that captures the store** — does the children-`List` and the `toTaskJSON` serialization; `Transitioner` and `EventEmitter` only decide *when* to call it and forward the map to the runner.

**Tech Stack:** Go 1.23; `text/template` (unchanged); `os/exec` (`cmd.Env`); `encoding/json` (CLI only). No new deps.

## Global Constraints

- **Injection boundary is sacred.** `{{...}}` stays `cmdContext{ID,Type,From,To}` — free text NEVER enters the command string. Existing tests asserting `echo {{.Title}}` is a template error (`internal/core/transition_test.go:250,265`) MUST stay green.
- **`core` never marshals domain types.** `pkg/mtt` has no json tags; all JSON goes through the CLI's `toTaskJSON`. Core imports no `encoding/json` for this.
- **`make check` green before every commit** (gofmt/vet/golangci-lint/`go test -race`/build). No red commit.
- **Env values are data.** The exec adapter sets `cmd.Env`; a nil/empty env means "inherit parent env" (today's behavior) — no regression.
- Commit trailer: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`. Attribution via `.mtt/config.local.yaml` author (do NOT export `MTT_BY`).

---

## File structure

- `internal/core/runner.go` — `Runner` interface: `Run`/`Compensate` gain `env map[string]string`.
- `internal/adapter/exec/exec.go` — `Run`/`Compensate`/`runReport`/`runOne` thread env → `cmd.Env`; new `envSlice` helper.
- `internal/core/transition.go` — `Transitioner` gains `envFn func(mtt.Task) map[string]string`; gate/post/compensate call it and pass the map.
- `internal/core/event.go` — `EventEmitter` gains `envFn`; `TaskEvent` passes `envFn(t)`, `NoteEvent` passes nil; `run` threads env.
- `internal/cli/taskenv.go` (new) — `taskEnvBuilder(store mtt.TaskStore) func(mtt.Task) map[string]string` (children-`List` + `toTaskJSON` + scalars + NUL strip).
- `internal/cli/status.go`, `internal/cli/events.go` — wire the builder into `NewTransitioner`/`NewEventEmitter`.
- `docs/architecture/model.go` — doc-model `Runner` kept in sync.
- Fakes/tests: `internal/core/transition_test.go`, `internal/core/remove_test.go`, `internal/adapter/exec/exec_test.go`, `internal/adapter/exec/exec_pgid_test.go`.
- Docs: `internal/core/CLAUDE.md`, `internal/adapter/exec/CLAUDE.md`, `internal/cli/CLAUDE.md`, `CHANGELOG.md`.

---

## Task 1: `Runner` port carries env; exec sets it on `sh -c`

**Files:** Modify `internal/core/runner.go`, `internal/adapter/exec/exec.go`, `docs/architecture/model.go`, and every fake/caller (`transition.go`, `event.go`, `transition_test.go`, `remove_test.go`, `exec_test.go`, `exec_pgid_test.go`). Test: `internal/adapter/exec/exec_test.go`.

**Interfaces:**
- Produces: `Run(commands []mtt.Command, env map[string]string) ([]mtt.Check, error)` and `Compensate(commands []mtt.Command, env map[string]string) []mtt.Check`.
- Consumes: nothing new.

- [ ] **Step 1: Write the failing adapter test**

Append to `internal/adapter/exec/exec_test.go`:

```go
func TestRunPassesEnvToCommand(t *testing.T) {
	r := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard, 0)
	// The command reads an env var the runner injected. If env is not set,
	// $MTT_TASK_TITLE is empty and the grep fails (exit 1 -> a non-nil Check.Exit).
	checks, err := r.Run(
		[]mtt.Command{{Run: `test "$MTT_TASK_TITLE" = "hello world"`}},
		map[string]string{"MTT_TASK_TITLE": "hello world"},
	)
	if err != nil {
		t.Fatalf("operational error: %v", err)
	}
	if len(checks) != 1 || checks[0].Exit != 0 {
		t.Fatalf("gate did not see env: checks=%+v", checks)
	}
}
```

- [ ] **Step 2: Run it — expect a COMPILE failure** (`Run` takes one arg today)

Run: `go test ./internal/adapter/exec/ -run TestRunPassesEnvToCommand`
Expected: build error — `too many arguments in call to r.Run`.

- [ ] **Step 3: Change the `core.Runner` interface**

In `internal/core/runner.go`, update the interface (keep the doc comments; add the env note):

```go
type Runner interface {
	// Run executes the commands in order with the given environment (KEY=VALUE
	// appended to the process env; nil = inherit only), stopping at the first
	// non-zero exit. CONTRACT unchanged: on an operational failure the failing
	// command's Check is the LAST element (Exit -1).
	Run(commands []mtt.Command, env map[string]string) ([]mtt.Check, error)
	// Compensate runs already-expanded rollbacks best-effort with the same env.
	Compensate(commands []mtt.Command, env map[string]string) []mtt.Check
}
```

- [ ] **Step 4: Thread env through the exec adapter**

In `internal/adapter/exec/exec.go`:
- `func (r *Runner) Run(commands []mtt.Command, env map[string]string) ([]mtt.Check, error)` — pass `env` into each `r.runReport(cmd, true, env)`.
- `func (r *Runner) Compensate(commands []mtt.Command, env map[string]string) []mtt.Check` — pass `env` into `r.runReport(cmd, false, env)`.
- `func (r *Runner) runReport(cmd mtt.Command, showTail bool, env map[string]string) (mtt.Check, error)` — pass `env` into `r.runOne(cmd.Run, timeout, out, env)`.
- `func (r *Runner) runOne(cmd string, timeout time.Duration, out io.Writer, env map[string]string) (int, error)` — after building `c`, set env:

```go
	c := exec.CommandContext(ctx, name, args...) //nolint:gosec
	c.Dir = r.dir
	if len(env) > 0 {
		c.Env = envSlice(env) // parent env + our KEY=VALUE (ours win: last occurrence)
	}
	c.Stdout = out
	// ... unchanged
```

Add the helper (deterministic order for testability):

```go
// envSlice returns os.Environ() with the given vars appended as KEY=VALUE
// (sorted for determinism). A later occurrence wins in os/exec, so ours override
// a same-named parent var. NUL bytes are stripped from values (os/exec refuses a
// NUL, which would fail the launch operationally); the CLI builder also strips,
// this is defense in depth.
func envSlice(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := os.Environ()
	for _, k := range keys {
		out = append(out, k+"="+strings.ReplaceAll(env[k], "\x00", ""))
	}
	return out
}
```

Add `os`, `sort`, `strings` to the imports if missing.

- [ ] **Step 5: Fix every other caller/fake to the new signature (pass nil for now)**
- `internal/core/transition.go`: `tr.runner.Run(expanded, nil)` (both call sites, lines ~94 and ~130) and `tr.runner.Compensate(rbs, nil)` (line ~206). (Task 2 replaces `nil` with `tr.envFn(t)`.)
- `internal/core/event.go`: `e.runner.Run(expanded, nil)` (line ~106). (Task 3 replaces `nil`.)
- `docs/architecture/model.go`: update its `Runner` interface to match (env params on `Run`/`Compensate`).
- Fakes: `internal/core/transition_test.go` `fakeRunner.Run`/`Compensate`; `internal/core/remove_test.go` `orderRunner.Run`; `internal/adapter/exec/exec_test.go` + `exec_pgid_test.go` every `.Run(...)`/`.Compensate(...)` call — add the `env` arg (`nil` in existing tests).

- [ ] **Step 6: Run the new test + full gate**

Run: `go test ./internal/adapter/exec/ -run TestRunPassesEnvToCommand -v` → PASS.
Run: `make check` (write to a file / check `$?` separately). Expected: green (all existing tests pass with the threaded-but-nil env; no behavior change).

- [ ] **Step 7: Commit**

```bash
git add internal/core/runner.go internal/adapter/exec/ internal/core/transition.go internal/core/event.go docs/architecture/model.go internal/core/transition_test.go internal/core/remove_test.go
git commit -m "t40: Runner port carries per-Run env; exec sets it on sh -c

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: CLI env-builder + `Transitioner` injection (gate/post/compensate)

**Files:** Create `internal/cli/taskenv.go`; modify `internal/core/transition.go`, `internal/cli/status.go`. Test: `internal/cli/taskenv_test.go` (new), `internal/core/transition_test.go`, `internal/cli/testdata/script/*` (e2e).

**Interfaces:**
- Consumes: `Runner.Run(_, env)` from Task 1; `toTaskJSON` (`internal/cli/json.go`); `mtt.TaskStore`.
- Produces: `taskEnvBuilder(store mtt.TaskStore) func(mtt.Task) map[string]string`; `NewTransitioner(store, cfg, runner, now, envFn)`.

- [ ] **Step 1: Write the failing unit test for the builder**

Create `internal/cli/taskenv_test.go`:

```go
package cli

import (
	"encoding/json"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

type envStubStore struct{ tasks []mtt.Task }

func (s envStubStore) Create(t mtt.Task) (mtt.Task, error) { return t, nil }
func (s envStubStore) Get(id mtt.TaskID) (mtt.Task, error) {
	for _, t := range s.tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return mtt.Task{}, mtt.ErrNotFound
}
func (s envStubStore) List() ([]mtt.Task, error)      { return s.tasks, nil }
func (s envStubStore) Update(t mtt.Task) (mtt.Task, error) { return t, nil }
func (s envStubStore) Delete(id mtt.TaskID) error     { return nil }

func TestTaskEnvBuilder(t *testing.T) {
	parent := mtt.Task{ID: "t1", Type: "task", Title: "hello, world", Status: "in_progress", Priority: mtt.PriorityHigh, Tags: []string{"a", "b"}}
	child := mtt.Task{ID: "t2", Type: "task", Status: "done", Parent: "t1"}
	env := taskEnvBuilder(envStubStore{tasks: []mtt.Task{parent, child}})(parent)

	if env["MTT_TASK_ID"] != "t1" || env["MTT_TASK_STATUS"] != "in_progress" || env["MTT_TASK_TITLE"] != "hello, world" {
		t.Fatalf("scalars wrong: %+v", env)
	}
	if env["MTT_TASK_TAGS"] != "a,b" {
		t.Fatalf("tags: %q", env["MTT_TASK_TAGS"])
	}
	var children []map[string]string
	if err := json.Unmarshal([]byte(env["MTT_TASK_CHILDREN_JSON"]), &children); err != nil {
		t.Fatalf("children json: %v", err)
	}
	if len(children) != 1 || children[0]["id"] != "t2" || children[0]["status"] != "done" {
		t.Fatalf("children: %+v", children)
	}
	if !json.Valid([]byte(env["MTT_TASK_JSON"])) {
		t.Fatalf("MTT_TASK_JSON not valid: %q", env["MTT_TASK_JSON"])
	}
}
```

(Adjust the `Priority`/`Tags` field names to the real `mtt.Task` shape if they differ — check `pkg/mtt/task.go` before writing.)

- [ ] **Step 2: Run it — expect FAIL** (`taskEnvBuilder` undefined)

Run: `go test ./internal/cli/ -run TestTaskEnvBuilder`
Expected: build error / undefined.

- [ ] **Step 3: Implement the builder**

Create `internal/cli/taskenv.go`:

```go
package cli

import (
	"encoding/json"
	"strings"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// taskEnvBuilder returns the injected env-builder core (Transitioner/EventEmitter)
// calls per move/event: given a task it returns the MTT_TASK_* environment for a
// gate/post/event command. The closure captures the store so it can compute
// children-with-statuses; serialization lives here (core stays serialization-free).
// Best-effort: a List error just omits children. NUL is stripped from scalar
// values (os/exec refuses a NUL in an env value).
func taskEnvBuilder(store mtt.TaskStore) func(mtt.Task) map[string]string {
	return func(t mtt.Task) map[string]string {
		env := map[string]string{
			"MTT_TASK_ID":       string(t.ID),
			"MTT_TASK_TYPE":     string(t.Type),
			"MTT_TASK_STATUS":   string(t.Status),
			"MTT_TASK_TITLE":    t.Title,
			"MTT_TASK_PARENT":   string(t.Parent),
			"MTT_TASK_PRIORITY": string(t.Priority),
			"MTT_TASK_TAGS":     strings.Join(t.Tags, ","),
		}
		if b, err := json.Marshal(toTaskJSON(t)); err == nil {
			env["MTT_TASK_JSON"] = string(b)
		}
		if all, err := store.List(); err == nil {
			type child struct {
				ID     string `json:"id"`
				Type   string `json:"type"`
				Status string `json:"status"`
			}
			kids := []child{} // non-nil -> "[]" not "null"
			for _, c := range all {
				if c.Parent == t.ID {
					kids = append(kids, child{string(c.ID), string(c.Type), string(c.Status)})
				}
			}
			if b, err := json.Marshal(kids); err == nil {
				env["MTT_TASK_CHILDREN_JSON"] = string(b)
			}
		}
		for k, v := range env {
			env[k] = strings.ReplaceAll(v, "\x00", "") // exec refuses NUL in env values
		}
		return env
	}
}
```

(Verify the exact `mtt.Task` field names — `Priority`/`Parent`/`Tags` — and `toTaskJSON`'s signature; adjust casts.)

- [ ] **Step 4: Run the unit test → PASS**

Run: `go test ./internal/cli/ -run TestTaskEnvBuilder -v`

- [ ] **Step 5: Inject `envFn` into `Transitioner`**

In `internal/core/transition.go`:
- Add field `envFn func(mtt.Task) map[string]string` to `Transitioner`.
- `func NewTransitioner(store mtt.TaskStore, cfg mtt.Config, runner Runner, now func() time.Time, envFn func(mtt.Task) map[string]string) *Transitioner` — store it.
- Add a nil-safe helper:

```go
func (tr *Transitioner) env(t mtt.Task) map[string]string {
	if tr.envFn == nil {
		return nil
	}
	return tr.envFn(t)
}
```
- Gate call (line ~94): `checks, err = tr.runner.Run(expanded, tr.env(t))` — `t` is pre-move (Status = from). ✓
- Post call (line ~130): `pchecks, rerr := tr.runner.Run(expanded, tr.env(t))` — here `t.Status` is already `to` (post-move context). ✓
- Compensate (line ~206, inside `block`): thread the task in. `block` currently takes `(expanded, failIdx, cause)`; add the task or the env. Simplest: pass the built env into `block` — change to `block(expanded []mtt.Command, failIdx int, cause string, env map[string]string)` and call `tr.runner.Compensate(rbs, env)`; the two `block(...)` call sites (lines ~98, ~101) pass `tr.env(t)`.

- [ ] **Step 6: Update the core fakes + fix `NewTransitioner` callers**
- `internal/core/transition_test.go`: `fakeRunner` — capture env (`gotEnv map[string]string`) for assertions; update `NewTransitioner(...)` calls to pass an `envFn` (a stub returning a known map, or `nil`).
- Add a core test asserting the gate env is threaded: a `fakeRunner` records the `env` arg; `NewTransitioner(store, cfg, fake, now, func(t mtt.Task) map[string]string { return map[string]string{"MTT_TASK_ID": string(t.ID)} })`; after a move, assert `fake.gotEnv["MTT_TASK_ID"] == "<id>"`.
- `internal/cli/status.go` `runTransition`: build the store once and thread it:

```go
	store := yaml.NewTaskStore(root)
	tr := core.NewTransitioner(store, cfg, runner, time.Now, taskEnvBuilder(store))
```

- [ ] **Step 7: e2e — a gate reads task env**

Add an `testscript` case (`internal/cli/testdata/script/gate_task_env.txt` or extend an existing gate script): a config whose `tbd -> in_progress` gate is `test "$MTT_TASK_TITLE" = "Ship auth"`, and a roll-up gate over `$MTT_TASK_CHILDREN_JSON`. Assert the move passes only when the env matches (mirror the existing gate e2e style; stub nothing — this is pure mtt + `sh`). Confirm a `{{.Title}}` in a command is still a template error (reuse/keep the existing assertion).

- [ ] **Step 8: gate + commit**

Run: `make check` (green). Then:
```bash
git add internal/cli/taskenv.go internal/cli/taskenv_test.go internal/core/transition.go internal/core/transition_test.go internal/cli/status.go internal/cli/testdata/script/
git commit -m "t40: task-context env for gate/post commands via injected builder

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: task-context env for lifecycle events; note events excluded

**Files:** Modify `internal/core/event.go`, `internal/cli/events.go`. Test: `internal/core/event_test.go` (or wherever `EventEmitter` is tested), e2e event scripts.

**Interfaces:**
- Consumes: `envFn` from Task 2; `Runner.Run(_, env)`.
- Produces: `NewEventEmitter(cfg, runner, audit, now, envFn)`.

- [ ] **Step 1: Write the failing test**

In the EventEmitter test file, assert a **task** create/update event sees `MTT_TASK_*` and a **note** event does not. Use a fake runner capturing env. Expected FAIL: `NewEventEmitter` takes 4 args today.

- [ ] **Step 2: Inject `envFn` and thread env**

In `internal/core/event.go`:
- Add field `envFn func(mtt.Task) map[string]string` to `EventEmitter`.
- `func NewEventEmitter(cfg mtt.Config, runner Runner, audit mtt.AuditStore, now func() time.Time, envFn func(mtt.Task) map[string]string) *EventEmitter`.
- `run(post, ctx, env)` — add an `env map[string]string` param; `e.runner.Run(expanded, env)` (line ~106).
- `TaskEvent(...)`: compute `env := map[string]string(nil); if e.envFn != nil { env = e.envFn(t) }` and pass to `run(hook.Post, taskEventContext{...}, env)`.
- `NoteEvent(...)`: pass `nil` env to `run(...)` (note events get NO `MTT_TASK_*`).

- [ ] **Step 3: Wire the builder in the CLI**

In `internal/cli/events.go` `newEventEmitter`, build the store once and pass the builder:
```go
	store := yaml.NewTaskStore(root)
	return core.NewEventEmitter(cfg, runner, yaml.NewAuditStore(root), time.Now, taskEnvBuilder(store)), closeOut, nil
```

- [ ] **Step 4: Update EventEmitter fakes/callers** — any test constructing `NewEventEmitter` gets the extra `envFn` arg (nil or a stub). Nil-emitter paths (tests passing a nil `*EventEmitter`) are unaffected.

- [ ] **Step 5: run tests + `make check`** — task-event env present, note-event env absent; existing event tests green.

- [ ] **Step 6: Commit**
```bash
git add internal/core/event.go internal/cli/events.go internal/core/event_test.go
git commit -m "t40: task-context env for task lifecycle events (note events excluded)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: docs + CHANGELOG

**Files:** `internal/core/CLAUDE.md`, `internal/adapter/exec/CLAUDE.md`, `internal/cli/CLAUDE.md`, `CHANGELOG.md`.

- [ ] **Step 1: Update the package CLAUDE.md files**
- `internal/core/CLAUDE.md`: the `Runner` port note — `Run`/`Compensate` now take `env map[string]string`; `Transitioner`/`EventEmitter` hold an injected `envFn func(mtt.Task) map[string]string` (core stays serialization-free; the CLI closure owns children-`List` + `toTaskJSON`).
- `internal/adapter/exec/CLAUDE.md`: `Run`/`Compensate` set the given env on `sh -c` (`cmd.Env = os.Environ() + KEY=VALUE`, ours win, NUL stripped); nil env = inherit only.
- `internal/cli/CLAUDE.md`: `taskenv.go` — `taskEnvBuilder(store)` builds the `MTT_TASK_*` env (blob `toTaskJSON` + `MTT_TASK_CHILDREN_JSON` + scalars) injected into `Transitioner`/`EventEmitter`; `{{}}` whitelist unchanged; note events get none.

- [ ] **Step 2: CHANGELOG [Unreleased] → ### Added**

```markdown
- **Task-context env for gate/post/event commands (t40).** Gate `commands:`, edge `post:`, and task
  lifecycle-event pipelines now run with the task's read-only context in the environment: `MTT_TASK_JSON`
  (the whole task), `MTT_TASK_CHILDREN_JSON` (children with statuses — a roll-up "all children done" gate),
  and scalars `MTT_TASK_ID/TYPE/STATUS/TITLE/PARENT/PRIORITY/TAGS`. The `{{...}}` template surface is
  unchanged and still shape-safe (`ID/Type/From/To` only) — free text rides the environment as data, never
  interpolated into the command string, so the injection boundary is preserved. Note events get no
  `MTT_TASK_*`. (Facet a — a per-invocation `--arg` value channel — is tracked separately in t77.)
```

- [ ] **Step 3: gate + commit**

Run: `make check` (green). Then:
```bash
git add internal/core/CLAUDE.md internal/adapter/exec/CLAUDE.md internal/cli/CLAUDE.md CHANGELOG.md
git commit -m "t40: docs + CHANGELOG for task-context env

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-review

- **Spec coverage:** env surface (Task 2/3), `{{}}` unchanged + injection boundary (Global Constraints + Task 2 §7), injected `func(mtt.Task)` builder with CLI-owned serialization + EventEmitter needs no store (Task 2/3), `Runner` env + doc-model (Task 1), note events excluded (Task 3), NUL strip (Task 1 `envSlice` + Task 2 builder), lean `toTaskJSON` blob (Task 2), children-with-statuses `{id,type,status}` (Task 2), CHANGELOG + CLAUDE.md (Task 4). t77 (facet a) already recorded on main.
- **Placeholder scan:** every code step has real Go; the two "verify the exact field names" notes point at `pkg/mtt/task.go`/`internal/cli/json.go` to confirm before writing (not a placeholder — a pre-flight check).
- **Type consistency:** `taskEnvBuilder(store) func(mtt.Task) map[string]string`, `Run(cmds, env)`, `Compensate(cmds, env)`, `NewTransitioner(store,cfg,runner,now,envFn)`, `NewEventEmitter(cfg,runner,audit,now,envFn)` used consistently across tasks.
- **Green-gate honesty:** Task 1 is a pure refactor (env threaded, callers pass nil) + one new adapter test — green at commit; Tasks 2/3 add behavior with tests; no red commit.
