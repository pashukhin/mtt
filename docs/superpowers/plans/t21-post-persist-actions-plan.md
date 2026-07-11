# Post-Persist Flow Actions (t21) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a per-edge `post:` command list that runs AFTER the status is persisted, so a move commits its own `.mtt` change — a second phase alongside the existing pre-persist `commands:` gate.

**Architecture:** `core.Transitioner` gains a post phase after `store.Update`, gated by `!opts.NoRun`. Post failure returns a new sentinel `ErrPostAction` with the **persisted** task (the move happened; only finalization failed); the CLI renders the move, surfaces the post error, and maps exit **5**. The repo `.mtt/config.yaml` then gets `post:` auto-commit on every edge.

**Tech Stack:** Go 1.23, cobra, `text/template` gates, `testscript` (txtar) e2e, `gopkg.in/yaml.v3`.

## Global Constraints

- **TDD, red→green→refactor.** Failing test first; run it; minimal impl; run; commit.
- **`make check` green before EVERY commit** (gofmt + vet + golangci-lint v2 + `go test -race -cover` + build over `./...`). No unused vars; check/discard every `Fprintf`/`Fprint` return (errcheck).
- **Hexagonal:** policy in `core`; `pkg/mtt` holds domain + ports; adapters carry no rules; CLI thin.
- **New return contract:** `ErrPostAction` is the **only** case where `Transition` returns a **valid task with a non-nil error**. Every other error path returns `mtt.Task{}`. Document it (godoc + `docs/architecture/model.go` + `internal/core/CLAUDE.md`).
- **Two phases:** `commands:` gate the entry (fail → no persist, compensate); `post:` finalize after (fail → status kept, exit 5). `--no-run` skips **both**.
- **`--no-run` forces who+why (t5):** any test/e2e driving a `--no-run` move must pass `--who`/`--why` (or `MTT_BY`) or it exits 2, not what you expect.
- **Reuse the real core test helpers:** `newMemStore`, `baseTask` (t1@tbd), `testClock`, `&fakeRunner{}` (pointer receiver), `flowCfg(cmdsA,cmdsB)` (edge0 `tbd→in_progress`), `strCmds([]string) []mtt.Command`.

---

## File Structure

- `pkg/mtt/config.go` — **modify**: add `Transition.Post []Command`.
- `pkg/mtt/validate.go` — **modify**: `Post[].Valid()` loop with a distinct "invalid post command" error.
- `internal/core/runner.go` — **modify**: add `ErrPostAction` sentinel.
- `internal/core/transition.go` — **modify**: post phase after `store.Update`; godoc contract note.
- `internal/adapter/yaml/dto.go` — **modify**: `ymlTransition.Post` + `toDomain` map.
- `internal/cli/status.go` — **modify**: `runTransition` restructure (render on post-fail, `txErr`).
- `internal/cli/root.go` — **modify**: `exitCode` `ErrPostAction → 5`.
- `internal/cli/types.go` — **modify (optional)**: render `⇢ <post>` under an edge.
- `.mtt/config.yaml` — **modify**: `post:` auto-commit on every edge.
- `docs/architecture/model.go` — **modify**: `Transition.Post`, `ErrPostAction`, Transitioner contract.
- Tests: `internal/core/transition_test.go`, `internal/adapter/yaml/dto_post_test.go`, `internal/cli/testdata/scripts/post_actions.txt`, `internal/adapter/yaml/dogfood_test.go` (guard).
- Docs sync: `AGENTS.md`, `DESIGN.md`/`.ru`, `CLI_REFERENCE.md`/`.ru`, `internal/core/CLAUDE.md`, `internal/cli/CLAUDE.md`, `internal/adapter/yaml/CLAUDE.md`.

---

## Task 1: Domain + core post-phase

**Files:**
- Modify: `pkg/mtt/config.go` (`Transition.Post`), `pkg/mtt/validate.go` (post loop),
  `internal/core/runner.go` (`ErrPostAction`), `internal/core/transition.go` (post phase + godoc),
  `docs/architecture/model.go` (mirror)
- Test: `internal/core/transition_test.go`

**Interfaces:**
- Produces: `mtt.Transition.Post []Command`; `core.ErrPostAction`; `Transition` may now return `(persistedTask, ErrPostAction)`.
- Consumes: existing `expandCommands`, `Runner`, `firstFailure`, `cmdContext{ID,Type,From,To}`.

- [ ] **Step 1: Add the domain field.** In `pkg/mtt/config.go`, add to `Transition` (after `Require`):

```go
	Post []Command // commands run AFTER persist (finalization, e.g. git commit); non-transactional (t21)
```

- [ ] **Step 2: Add the sentinel.** In `internal/core/runner.go`, after `ErrMissingAttribution`:

```go
// ErrPostAction is returned when a transition's POST phase (post: commands, run
// AFTER persist) fails. Unlike ErrBlocked, the move IS persisted — only the
// finalization failed; the CLI keeps the move and maps it to exit 5. This is the
// ONLY case where Transition returns a valid task with a non-nil error.
var ErrPostAction = errors.New("mtt: post-action failed after the move")
```

- [ ] **Step 3: Write the failing tests.** In `internal/core/transition_test.go`, add:

```go
func TestTransition_PostRunsAfterPersist(t *testing.T) {
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil) // edge0 tbd→in_progress, no pre-commands
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo hi"})
	runner := &fakeRunner{}
	got, err := NewTransitioner(store, cfg, runner, testClock).Transition("t1", "in_progress", TransitionOptions{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", got.Status)
	}
	if len(runner.gotCmds) != 1 || runner.gotCmds[0].Run != "echo hi" {
		t.Fatalf("post not run: %+v", runner.gotCmds)
	}
	if reloaded, _ := store.Get("t1"); reloaded.Status != "in_progress" {
		t.Fatalf("not persisted: %q", reloaded.Status)
	}
}

func TestTransition_PostFailureKeepsMove(t *testing.T) {
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"false"})
	runner := &fakeRunner{checks: []mtt.Check{{Cmd: "false", Exit: 1}}}
	_, err := NewTransitioner(store, cfg, runner, testClock).Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrPostAction) {
		t.Fatalf("want ErrPostAction, got %v", err)
	}
	// move is KEPT despite the post failure (persisted before post ran)
	if reloaded, _ := store.Get("t1"); reloaded.Status != "in_progress" {
		t.Fatalf("post failure must not roll back; status = %q", reloaded.Status)
	}
}

func TestTransition_PostExpandsPlaceholders(t *testing.T) {
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo {{.ID}}"})
	runner := &fakeRunner{}
	if _, err := NewTransitioner(store, cfg, runner, testClock).Transition("t1", "in_progress", TransitionOptions{}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if runner.gotCmds[0].Run != "echo t1" {
		t.Fatalf("post placeholder not expanded: %q", runner.gotCmds[0].Run)
	}
}

func TestTransition_PostExpandErrorIsPostAction(t *testing.T) {
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo {{.Nope}}"}) // unknown field → template error
	_, err := NewTransitioner(store, cfg, &fakeRunner{}, testClock).Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrPostAction) {
		t.Fatalf("expand error must be ErrPostAction, got %v", err)
	}
}

func TestTransition_NoRunSkipsPost(t *testing.T) {
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo hi"})
	runner := &fakeRunner{}
	// --no-run forces who+why (t5); supply them.
	got, err := NewTransitioner(store, cfg, runner, testClock).Transition("t1", "in_progress",
		TransitionOptions{NoRun: true, By: "a", Why: "b"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if runner.called {
		t.Fatal("--no-run must skip the post phase")
	}
	if got.Status != "in_progress" {
		t.Fatalf("persist must still happen; status = %q", got.Status)
	}
}

func TestTransition_NoPostUnchanged(t *testing.T) {
	store := newMemStore(baseTask())
	runner := &fakeRunner{}
	if _, err := NewTransitioner(store, flowCfg(nil, nil), runner, testClock).Transition("t1", "in_progress", TransitionOptions{}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if runner.called {
		t.Fatal("no pre-commands and no post → runner must not be called")
	}
}
```

- [ ] **Step 4: Run — verify they fail.**

Run: `go test ./internal/core/ -run 'TestTransition_(Post|NoRunSkipsPost|NoPostUnchanged)' -v`
Expected: FAIL (`Post` field unknown until Step 1 compiles, then behavior fails — post never runs).

- [ ] **Step 5: Add the post phase.** In `internal/core/transition.go`, replace the final line `return tr.store.Update(t)` with:

```go
	updated, uerr := tr.store.Update(t)
	if uerr != nil {
		return mtt.Task{}, uerr
	}
	// POST phase (t21): after persist, gated by !NoRun. A post failure returns the
	// PERSISTED task with ErrPostAction — the move is kept (finalization only).
	if opts.NoRun || len(edge.Post) == 0 {
		return updated, nil
	}
	expanded, eerr := expandCommands(edge.Post, cmdContext{
		ID: string(t.ID), Type: string(t.Type), From: string(from), To: string(to),
	})
	if eerr != nil {
		return updated, fmt.Errorf("%w: expand post for %s (%s->%s): %v", ErrPostAction, id, from, to, eerr)
	}
	pchecks, rerr := tr.runner.Run(expanded)
	if rerr != nil {
		return updated, fmt.Errorf("%w: %v", ErrPostAction, rerr)
	}
	if _, c, failed := firstFailure(pchecks); failed {
		return updated, fmt.Errorf("%w: command %q exited %d", ErrPostAction, c.Cmd, c.Exit)
	}
	return updated, nil
```

Update the `Transition` method godoc to note: "On `ErrPostAction` the returned task is the **persisted** state (the move happened; only the post phase failed) — the single case where a non-nil error carries a valid task."

- [ ] **Step 6: Add the validation loop.** In `pkg/mtt/validate.go`, after the `tr.Commands` loop (~line 73), add:

```go
		for _, cmd := range tr.Post {
			if !cmd.Valid() {
				errs = append(errs, fmt.Errorf("type %q transition %q->%q: invalid post command (empty/negative timeout or bad rollback)", t.Name, tr.From, tr.To))
			}
		}
```

- [ ] **Step 7: Mirror in the architecture reference.** In `docs/architecture/model.go`: add `Post []Command // finalization commands run AFTER persist (t21)` to `Transition` (line ~216); add `ErrPostAction` to the sentinels prose near the Transitioner block (~line 662), noting it carries a valid persisted task.

- [ ] **Step 8: Run — verify green.**

Run: `go test ./internal/core/ ./pkg/mtt/`
Expected: PASS (new + existing).

- [ ] **Step 9: `make check`, then commit.**

```bash
make check
git add pkg/mtt/config.go pkg/mtt/validate.go internal/core/runner.go internal/core/transition.go internal/core/transition_test.go docs/architecture/model.go
git commit -m "feat(t21): core post-persist phase + ErrPostAction (per-edge post:, --no-run skips)"
```

---

## Task 2: Adapter — decode `post:`

**Files:**
- Modify: `internal/adapter/yaml/dto.go`
- Test: `internal/adapter/yaml/dto_post_test.go`

**Interfaces:** Consumes `mtt.Transition.Post` (Task 1). Produces: `post:` in config decodes into `Transition.Post`.

- [ ] **Step 1: Write the failing test.** Create `internal/adapter/yaml/dto_post_test.go`:

```go
package yaml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DecodesPostCommands(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".mtt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `version: 1
project: {name: demo}
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: a, kind: initial, default: true}
      - {name: b, kind: terminal}
    transitions:
      - from: a
        to: b
        name: go
        post:
          - 'git add .mtt && git commit -m "{{.ID}}: {{.From}} → {{.To}}" -- .mtt'
          - {run: echo done, timeout: 30s}
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	post := cfg.Types[0].Transitions[0].Post
	if len(post) != 2 || post[0].Run == "" || post[1].Run != "echo done" || post[1].Timeout == 0 {
		t.Fatalf("post not decoded: %+v", post)
	}
}
```

- [ ] **Step 2: Run — verify it fails.**

Run: `go test ./internal/adapter/yaml/ -run TestLoad_DecodesPostCommands -v`
Expected: FAIL (`post` is empty — not mapped).

- [ ] **Step 3: Add the DTO field + mapping.** In `internal/adapter/yaml/dto.go`, add to `ymlTransition` (after `Require`):

```go
	Post []ymlCommand `yaml:"post,omitempty"`
```

and in `toDomain`, build the post slice beside `cmds` and add it to the `mtt.Transition{…}` literal:

```go
			post := make([]mtt.Command, 0, len(yr.Post))
			for _, c := range yr.Post {
				post = append(post, c.toDomain())
			}
```
then add `Post: post,` to the `mtt.Transition{…}` literal.

- [ ] **Step 4: Run — verify green.**

Run: `go test ./internal/adapter/yaml/ -run TestLoad_DecodesPostCommands -v && go test ./internal/adapter/yaml/`
Expected: PASS (existing config tests unaffected — `post` is `omitempty`/absent).

- [ ] **Step 5: `make check`, then commit.**

```bash
make check
git add internal/adapter/yaml/dto.go internal/adapter/yaml/dto_post_test.go
git commit -m "feat(t21): decode per-edge post: commands in the YAML config"
```

---

## Task 3: CLI — post-error rendering + exit 5

**Files:**
- Modify: `internal/cli/status.go` (`runTransition`), `internal/cli/root.go` (`exitCode`),
  `internal/cli/types.go` (optional `⇢` render)
- Test: `internal/cli/testdata/scripts/post_actions.txt`

**Interfaces:** Consumes `core.ErrPostAction` (Task 1). Produces: a post failure renders the move, surfaces the error on stderr, exits 5 (text **and** `--json`); `--no-run` skips post.

- [ ] **Step 1: Write the failing e2e.** Create `internal/cli/testdata/scripts/post_actions.txt`:

```
# t21 — per-edge post: runs after persist; failure → exit 5, move kept.
exec mtt init
cp post.yaml .mtt/config.yaml
exec mtt types
stdout 'task'

exec mtt add 'a task'
stdout 'created t1'

# a passing post runs and its output is visible (-v streams gate/post output)
exec mtt start t1 -v
stdout 't1: tbd → speccing'
stderr 'POSTRAN'

# a failing post → exit 5, but the move is KEPT (status advanced)
! exec mtt submit t1
stderr 'post-action failed'
exec mtt show t1
stdout '\[review\]'

# --json on the failing post also exits non-zero (not swallowed); needs a task in speccing
exec mtt add 'b task'
exec mtt start t2
! exec mtt submit t2 --json
exec mtt show t2
stdout '\[review\]'

-- post.yaml --
version: 1
project: {name: posttest}
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: tbd, kind: initial, default: true}
      - {name: speccing, kind: active}
      - {name: review, kind: active}
      - {name: done, kind: terminal}
    transitions:
      - from: tbd
        to: speccing
        name: start
        post:
          - 'echo POSTRAN 1>&2'
      - from: speccing
        to: review
        name: submit
        post:
          - 'false'
      - {from: review, to: done, name: approve}
```

> Match the `testscript` idiom (mirror `dogfood.txt`). The fixture is flat (task is root/default), so `mtt add` needs no `--parent`. `-v` streams the post command's stdout/stderr (the runner hides it otherwise). Adjust the exact matchers to the runner's real progress format if needed.

- [ ] **Step 2: Run — verify it fails.**

Run: `go test ./internal/cli/ -run 'TestScripts/post_actions' -v`
Expected: FAIL (post not wired in the CLI yet; submit's failing post doesn't exit 5 the intended way).

- [ ] **Step 3: Add exit 5.** In `internal/cli/root.go`'s `exitCode`, add before `default`:

```go
	case errors.Is(err, core.ErrPostAction):
		return 5
```

- [ ] **Step 4: Restructure `runTransition`.** In `internal/cli/status.go`, replace the tail of `runTransition` (from `task, err := tr.Transition(...)` through the final `return nil`) with:

```go
	task, txErr := tr.Transition(id, to, core.TransitionOptions{
		Role: role, By: by, Why: why, NoRun: noRun,
		RequireWho: settings.Require.Who, RequireWhy: settings.Require.Why,
	})
	postFailed := errors.Is(txErr, core.ErrPostAction)
	if txErr != nil && !postFailed {
		if hidden && errors.Is(txErr, core.ErrBlocked) {
			return fmt.Errorf("%w\n  hint: re-run with -v or --log-file to see the command's full output", txErr)
		}
		return txErr
	}
	// txErr == nil OR postFailed: the move is persisted → render it.
	if err := applyCurrent(root, cfg, task, id); err != nil {
		return fmt.Errorf("transition applied but updating the current pointer failed: %w", err)
	}
	if jsonFlag(cmd) {
		if err := writeJSON(cmd.OutOrStdout(), toTaskJSON(task)); err != nil {
			return err
		}
		return txErr // exit 5 on post failure, even in --json
	}
	last := task.History[len(task.History)-1]
	out := cmd.OutOrStdout()
	if _, e := fmt.Fprintf(out, "%s: %s → %s\n", id, last.From, last.To); e != nil {
		return e
	}
	if g := moveGuidance(cfg, task.Type, last.From, last.To); g != "" {
		if _, e := fmt.Fprint(out, g); e != nil {
			return e
		}
	}
	if postFailed {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "move applied, but a post-action failed: %v\n", txErr)
	}
	return txErr
```

> Note: local `e` for the render writes — never reuse `txErr` (a successful write would clobber `ErrPostAction` to nil and defeat exit 5 in text mode).

- [ ] **Step 5: (optional) `mtt types` render.** In `internal/cli/types.go`'s `writeTypeBlock`, after the `↩ <rollback>` line, add a `⇢ <post.Run>` line per `edge.Post` (+ `(timeout …)` when set), mirroring the rollback render. Skip if it complicates the task; not required for correctness.

- [ ] **Step 6: Run — verify green.**

Run: `go test ./internal/cli/ -run 'TestScripts/post_actions' -v && go test ./internal/cli/`
Expected: PASS (new e2e + existing scripts — existing scripts have no `post:` so behavior is unchanged).

- [ ] **Step 7: `make check`, then commit.**

```bash
make check
git add internal/cli/status.go internal/cli/root.go internal/cli/types.go internal/cli/testdata/scripts/post_actions.txt
git commit -m "feat(t21): CLI renders the move + surfaces post failure (exit 5); mtt types shows post"
```

---

## Task 4: Repo config auto-commit + guard + docs

**Files:**
- Modify: `.mtt/config.yaml` (post on every edge), `internal/adapter/yaml/dogfood_test.go` (guard)
- Docs: `AGENTS.md`, `DESIGN.md`/`.ru`, `CLI_REFERENCE.md`/`.ru`, `internal/core/CLAUDE.md`,
  `internal/cli/CLAUDE.md`, `internal/adapter/yaml/CLAUDE.md`

**Interfaces:** none new — applies the mechanism to this repo and documents it.

- [ ] **Step 1: Add `post:` to every edge in `.mtt/config.yaml`.** For each transition under both `task` and `chore`, add (single-quoted scalar, `-- .mtt` pathspec):

```yaml
        post:
          - 'git add .mtt && git commit -m "{{.ID}}: {{.From}} → {{.To}}" -- .mtt'
```

Do this for all edges: `start`, the three `submit`, the `approve` edges, the `decline` edges, `deliver`, and every `cancel` edge (~38 total). For `deliver`/`cancel` the pre-gate already `git switch main`, so the commit lands on main.

- [ ] **Step 2: Verify the config still validates + the flow still runs.**

Run: `bin/mtt types` (after `go build -o bin/mtt ./cmd/mtt`)
Expected: no validation error; the flow prints (each edge now shows a `⇢` post line if Task 3 Step 5 was done).

- [ ] **Step 3: Add the guard assertion.** In `internal/adapter/yaml/dogfood_test.go` (`TestRepoDogfoodConfig`), assert every transition carries the expected `post` (e.g. iterate all `Transitions`, require `len(tr.Post) == 1` and the `Run` matches the committed line). Run:

```bash
go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig -v
```
Expected: PASS (green with the new assertion; reddens if a `post` block is dropped).

- [ ] **Step 4: Update the docs.**
  - `AGENTS.md` ("Working under mtt"): **remove** the "two manual steps remain (after deliver and cancel)" bullet; note that moves auto-commit `.mtt` via per-edge `post:`.
  - `CLI_REFERENCE.md`/`.ru`: document `post:` (runs after persist; failure → exit 5, move kept; `--no-run` skips it).
  - `DESIGN.md`/`.ru`: a short "post-persist actions (t21)" subsection — two phases, `ErrPostAction`, exit 5.
  - `internal/core/CLAUDE.md` (Transitioner: post phase + `ErrPostAction` valid-task-with-error contract),
    `internal/cli/CLAUDE.md` (runTransition renders move + surfaces post, exit 5),
    `internal/adapter/yaml/CLAUDE.md` (`ymlTransition.Post` decode).

- [ ] **Step 5: `make check`.**

Run: `make check`
Expected: all green (the new repo `post:` blocks run real `git commit`s only via actual moves, not in tests; `TestRepoDogfoodConfig` loads a copy and checks structure).

- [ ] **Step 6: Commit.**

```bash
git add .mtt/config.yaml internal/adapter/yaml/dogfood_test.go AGENTS.md DESIGN.md DESIGN.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md internal/core/CLAUDE.md internal/cli/CLAUDE.md internal/adapter/yaml/CLAUDE.md
git commit -m "feat(t21): auto-commit .mtt via per-edge post: on every edge; guard + docs sync"
```

---

## Notes for the implementer

- **Order:** Task 1 (domain+core+model, atomic) → Task 2 (adapter) → Task 3 (CLI+e2e) → Task 4 (repo config+docs). Each is independently `make check`-green: Tasks 1-3 don't touch `.mtt/config.yaml`, so the repo flow keeps working with no post phase until Task 4 turns it on.
- **`--no-run` + who/why:** every `--no-run` move needs `--who`/`--why` (t5). The e2e/tests above account for this.
- **Don't reuse `txErr`** for the render `Fprintf`s in `runTransition` — use a local `e`, or a successful write clobbers `ErrPostAction` and exit 5 is lost in text mode.
- **Meta-recursion:** once Task 4 lands, *this very task's* remaining moves (`submit`→impl_review, etc.) will auto-commit — but only after the branch merges to main and you're running the new binary. During implementation you're still on the old binary, so keep committing `.mtt` by hand until then.
