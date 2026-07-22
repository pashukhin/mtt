# Actionable errors (t28) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make mtt's errors self-evidently actionable — exit 2 tells you how to set who/why, exit 5 tells you the move is saved and prints the exact commands to finish by hand, exit 4 points at the discovery commands, and exit 6's terminal-status message reads cleanly.

**Architecture:** Hexagon preserved. One `core` change: a typed `*PostActionError{Remaining, Cause}` replaces the plain-wrapped `ErrPostAction`, carrying the unfinished post commands (expansion stays in `core`). All hint *phrasing* lives in `internal/cli`: a context-free `exitHint(err)` hook in `Execute` (exit 2 / exit 4), and a contextual recovery block in `runTransition` (exit 5). exit 6 gets a small empty-list parity fix in `core`.

**Tech Stack:** Go 1.23+, cobra CLI, stdlib `errors`/`fmt`, table-driven unit tests, `testscript` (txtar) e2e.

## Global Constraints

- **Spec of record:** `docs/superpowers/specs/t28-actionable-errors.md`. Every decision (D1–D6) is binding.
- **TDD:** red → green → refactor. Failing test first, watch it fail, then implement. `make check` green before every commit.
- **Hexagon:** `core` stays policy; **no** config/env/flag names in `core`. `PostActionError` carries *data* (`Remaining` command strings, verbatim `Cause`), never CLI phrasing. Hints live in `internal/cli`.
- **No exit-code change:** exit 2/3/4/5/6/7 stay as `exitCode` maps them; the typed `*PostActionError` still maps to 5 via `Is()`.
- **No new flag / config / color** (color/verbosity is `t55`).
- **Docs bilingual** (EN + RU): `DESIGN`, `CLI_REFERENCE`. Grep all parallel occurrences before editing.
- **Imports:** when a step appends code to a file, merge the shown imports into the existing block — don't add a second `import (...)` or re-declare a package.

---

## File structure

**Modify:**
- `internal/core/runner.go` — add the `PostActionError` type (+ `fmt` import).
- `internal/core/transition.go` — `runsOf` helper; build `*PostActionError` at the 3 POST failure points; empty-`allowedTargets` guard for exit 6.
- `internal/core/transition_test.go` — extend post-fail tests + new `Remaining`/operational/terminal cases.
- `internal/cli/errors.go` — `exitHint(err)` + `attributionHint`/`notFoundHint` consts.
- `internal/cli/root.go` — call `exitHint` in `Execute`.
- `internal/cli/status.go` — exit-5 recovery block in `runTransition` (both text + `--json`).
- `internal/cli/errors_test.go` (new) — `exitHint` table; `internal/cli/status_test.go` — `TestExitCode` gains the `*PostActionError`→5 case.
- `internal/cli/testdata/scripts/post_actions.txt` — replace the terse assertion with the recovery block (text + `--json`).
- `internal/cli/testdata/scripts/actionable_errors.txt` (new) — exit-2 / exit-4 / exit-6 hints.
- Docs: `CLI_REFERENCE.md`↔`.ru.md`, `DESIGN.md`↔`.ru.md`, `CHANGELOG.md`, `internal/cli/CLAUDE.md`, `internal/core/CLAUDE.md`.

---

## Task 1: core — typed `PostActionError` + POST wiring + exit-6 terminal parity

**Files:**
- Modify: `internal/core/runner.go`, `internal/core/transition.go`
- Test: `internal/core/transition_test.go`

**Interfaces:**
- Produces:
  - `type PostActionError struct { Remaining []string; Cause string }` with `Error() string` and `Is(error) bool` (→ `ErrPostAction`).
  - `func runsOf(cmds []mtt.Command) []string` (extract `.Run`).

- [ ] **Step 1: Write failing tests** — first extend `fakeRunner`, then append the tests, to `internal/core/transition_test.go`.

**Extend `fakeRunner`** (L14–22) with a **post-scoped** operational error — the plain `err` field fires on
*every* `Run` including the empty pre-gate (which would block before POST), so we need one that fires only for
a non-empty command slice (the post phase; the empty pre-gate stays clean). Add the field:
```go
	postOpErr  error         // when set, Run returns this operational error ONLY for a non-empty command slice (the post phase; the empty pre-gate passes)
```
and, as the **first** check inside `Run` (before the `failSubstr` derive-path), add:
```go
	if f.postOpErr != nil && len(commands) > 0 {
		return nil, f.postOpErr
	}
```

**Append the tests:**

```go
func TestTransition_PostActionErrorRemaining(t *testing.T) {
	// 3 post commands; the 2nd fails -> Remaining = the failed + the untried (2 of 3).
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo one", "boom-two", "echo three"})
	runner := &fakeRunner{failSubstr: "boom"} // empty pre-gate passes; post "boom-two" fails
	_, err := NewTransitioner(store, cfg, runner, testClock).Transition("t1", "in_progress", TransitionOptions{})
	var pe *PostActionError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PostActionError, got %T (%v)", err, err)
	}
	if !errors.Is(err, ErrPostAction) {
		t.Fatalf("PostActionError must map to ErrPostAction")
	}
	want := []string{"boom-two", "echo three"}
	if !slices.Equal(pe.Remaining, want) {
		t.Fatalf("Remaining = %v, want %v", pe.Remaining, want)
	}
	if !strings.Contains(pe.Cause, "boom-two") {
		t.Fatalf("Cause = %q, want it to name the failed command", pe.Cause)
	}
}

func TestTransition_PostActionOperationalZeroChecks(t *testing.T) {
	// Operational error in the POST phase with NO recorded checks -> Remaining = all
	// expanded (guarded against len(pchecks)==0, no panic). postOpErr fires only for a
	// non-empty command slice, so the empty pre-gate passes and only the post errors.
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Post = strCmds([]string{"echo a", "echo b"})
	runner := &fakeRunner{postOpErr: errors.New("boom timeout")}
	_, err := NewTransitioner(store, cfg, runner, testClock).Transition("t1", "in_progress", TransitionOptions{})
	var pe *PostActionError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PostActionError, got %T (%v)", err, err)
	}
	if want := []string{"echo a", "echo b"}; !slices.Equal(pe.Remaining, want) {
		t.Fatalf("Remaining = %v, want %v (all, guarded against len(pchecks)==0)", pe.Remaining, want)
	}
}

func TestTransition_InvalidMoveOutOfTerminalReadsCleanly(t *testing.T) {
	// A move requested out of a terminal status: empty allowedTargets -> no dangling list.
	store := newMemStore(func() mtt.Task { tk := baseTask(); tk.Status = "done"; return tk }())
	cfg := flowCfg(nil, nil)
	_, err := NewTransitioner(store, cfg, &fakeRunner{}, testClock).Transition("t1", "in_progress", TransitionOptions{})
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
	if strings.Contains(err.Error(), "allowed from done: )") || strings.HasSuffix(err.Error(), ": ") {
		t.Fatalf("terminal message has a dangling empty list: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Fatalf("terminal message should say so: %q", err.Error())
	}
}
```

Also extend the EXISTING `TestTransition_PostFailureKeepsMove` (L429) — after the `ErrPostAction` assert, add:
```go
	var pe *PostActionError
	if !errors.As(err, &pe) || len(pe.Remaining) != 1 || pe.Remaining[0] != "false" {
		t.Fatalf("Remaining should be [false]; pe=%+v", pe)
	}
```
And `TestTransition_PostExpandErrorIsPostAction` (L456) — assert `Remaining` is the raw template:
```go
	var pe *PostActionError
	if !errors.As(err, &pe) || len(pe.Remaining) != 1 || pe.Remaining[0] != "echo {{.Nope}}" {
		t.Fatalf("expand-error Remaining should be the raw post run; got %+v", pe)
	}
```

Add `"slices"` to the test file's imports (and `"strings"` if not already present — it is, used by `fakeRunner`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/core/ -run 'TestTransition_PostAction|TestTransition_InvalidMoveOutOfTerminal|TestTransition_PostFailureKeepsMove|TestTransition_PostExpandError' -count=1`
Expected: FAIL — `PostActionError`/`runsOf` undefined (compile error), and the terminal message still has the dangling list.

- [ ] **Step 3: Add the `PostActionError` type** — append to `internal/core/runner.go` (merge `"fmt"` into its imports):

```go
// PostActionError is the typed form of ErrPostAction: it carries the post commands
// that did NOT complete (the failed one + those never reached) so the CLI can print
// exact recovery steps, plus the underlying Cause. Is() maps it to ErrPostAction, so
// the CLI still exits 5. This is the single case where Transition returns a valid
// task with a non-nil error (the move IS persisted).
type PostActionError struct {
	Remaining []string // unfinished post commands (expanded where available)
	Cause     string   // the underlying failure, verbatim
}

func (e *PostActionError) Error() string       { return fmt.Sprintf("%s: %s", ErrPostAction, e.Cause) }
func (e *PostActionError) Is(target error) bool { return target == ErrPostAction }
```

- [ ] **Step 4: Wire the POST phase + `runsOf`** — in `internal/core/transition.go`, replace the three POST failure returns (L109–121) and add the helper. New POST block:

```go
	expanded, eerr := expandCommands(edge.Post, cmdContext{
		ID: string(t.ID), Type: string(t.Type), From: string(from), To: string(to),
	})
	if eerr != nil {
		return updated, &PostActionError{
			Remaining: runsOf(edge.Post), // raw templates — expansion is what failed
			Cause:     fmt.Sprintf("expand post for %s (%s->%s): %v", id, from, to, eerr),
		}
	}
	pchecks, rerr := tr.runner.Run(expanded)
	if rerr != nil {
		i := len(pchecks) - 1 // failing command is last (Runner CONTRACT); guard the empty case
		if i < 0 {
			i = 0
		}
		return updated, &PostActionError{Remaining: runsOf(expanded[i:]), Cause: rerr.Error()}
	}
	if i, c, failed := firstFailure(pchecks); failed {
		return updated, &PostActionError{
			Remaining: runsOf(expanded[i:]),
			Cause:     fmt.Sprintf("command %q exited %d", c.Cmd, c.Exit),
		}
	}
	return updated, nil
```

Add the helper near `firstFailure` (`transition.go`):
```go
// runsOf extracts each command's .Run string — the copy-paste-ready form the CLI
// prints as a post-failure recovery hint.
func runsOf(cmds []mtt.Command) []string {
	out := make([]string, len(cmds))
	for i, c := range cmds {
		out[i] = c.Run
	}
	return out
}
```

- [ ] **Step 5: Fix the exit-6 terminal message** — in `internal/core/transition.go`, replace the `findTransition` miss (L61–64):

```go
	edge, ok := findTransition(typ, t.Status, to)
	if !ok {
		if targets := allowedTargets(typ, t.Status); len(targets) > 0 {
			return mtt.Task{}, fmt.Errorf("%w: %s cannot move %s → %s (allowed from %s: %s)",
				ErrInvalidTransition, id, t.Status, to, t.Status, strings.Join(targets, ", "))
		}
		return mtt.Task{}, fmt.Errorf("%w: %s cannot move %s → %s (no moves out of %s — it is terminal)",
			ErrInvalidTransition, id, t.Status, to, t.Status)
	}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/core/ -run 'TestTransition' -race -count=1`
Expected: PASS (new + existing post/transition tests).

- [ ] **Step 7: `make check` + commit**

Run: `make check`
```bash
git add internal/core/runner.go internal/core/transition.go internal/core/transition_test.go
git commit -m "t28: typed PostActionError (recovery commands) + exit-6 terminal-status message"
```

---

## Task 2: CLI — `exitHint` hook (exit 2 attribution, exit 4 not-found)

**Files:**
- Modify: `internal/cli/errors.go`, `internal/cli/root.go`, `internal/cli/status_test.go`
- Test: `internal/cli/errors_test.go` (new)

**Interfaces:**
- Consumes: `core.ErrMissingAttribution`, `mtt.ErrNotFound`, `core.PostActionError` (Task 1).
- Produces: `func exitHint(err error) string`.

- [ ] **Step 1: Write failing unit test** — `internal/cli/errors_test.go`:

```go
package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/internal/core"
)

// (No pkg/mtt import: taskNotFound takes an untyped string constant, so mtt.TaskID
// is never named here — importing pkg/mtt would be an unused-import compile error.)

func TestExitHint(t *testing.T) {
	attrib := fmt.Errorf("%w: who, why", core.ErrMissingAttribution)
	if h := exitHint(attrib); !strings.Contains(h, "MTT_BY") || !strings.Contains(h, "--why") {
		t.Fatalf("attribution hint missing who/why setup: %q", h)
	}
	if h := exitHint(taskNotFound("t9")); !strings.Contains(h, "mtt roadmap") {
		t.Fatalf("not-found hint should point at discovery: %q", h)
	}
	// No bleed: a post-action error and an invalid-transition are handled with context.
	if h := exitHint(&core.PostActionError{Cause: "x"}); h != "" {
		t.Fatalf("PostActionError must get no generic hint, got %q", h)
	}
	if h := exitHint(core.ErrInvalidTransition); h != "" {
		t.Fatalf("invalid-transition must get no generic hint, got %q", h)
	}
	if h := exitHint(errors.New("boom")); h != "" {
		t.Fatalf("unrelated error must get no hint, got %q", h)
	}
}
```

Also add to `TestExitCode` (`status_test.go` L64 cases) — prove the typed error still maps to 5:
```go
		{core.ErrPostAction, 5},
		{&core.PostActionError{Cause: "x"}, 5},
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run 'TestExitHint|TestExitCode' -count=1`
Expected: FAIL — `exitHint` undefined.

- [ ] **Step 3: Implement `exitHint` + consts** — append to `internal/cli/errors.go` (merge imports; add `"github.com/pashukhin/mtt/internal/core"`):

```go
// attributionHint tells the user how to supply who/why after an exit-2 refusal.
const attributionHint = "" +
	"  set 'who': add `author: <name>` to .mtt/config.local.yaml, or `export MTT_BY=<name>`, or pass `--who <name>`\n" +
	"  set 'why': pass `--why \"<reason>\"`\n"

// notFoundHint points at the discovery commands after an exit-4 miss.
const notFoundHint = "" +
	"  check the id — 'mtt roadmap' or 'mtt list' show existing task ids ('mtt note list' for notes)\n"

// exitHint returns a trailing, actionable hint block for the context-free error
// sentinels, or "" when the error carries its own context (post-action, invalid
// transition) or is unrelated. Printed by Execute under the `error:` line.
func exitHint(err error) string {
	switch {
	case errors.Is(err, core.ErrMissingAttribution):
		return attributionHint
	case errors.Is(err, mtt.ErrNotFound):
		return notFoundHint
	default:
		return ""
	}
}
```

- [ ] **Step 4: Call it in `Execute`** — in `internal/cli/root.go`, replace the error branch (L57–60):

```go
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(root.ErrOrStderr(), "error:", err)
		if h := exitHint(err); h != "" {
			_, _ = fmt.Fprint(root.ErrOrStderr(), h)
		}
		return exitCode(err)
	}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run 'TestExitHint|TestExitCode' -race -count=1`
Expected: PASS.

- [ ] **Step 6: Add e2e (exit-2 + exit-4)** — `internal/cli/testdata/scripts/actionable_errors.txt`:

```
# t28 — actionable exit-2 (attribution) + exit-4 (not-found) hints.
exec mtt init
cp req.yaml .mtt/config.yaml
exec mtt add 'a task'
stdout 'created t1'

# exit 2: a transition with required who/why but none supplied -> hint how to set them.
! exec mtt in_progress t1
stderr 'missing required attribution'
stderr 'MTT_BY'
stderr '--why'

# exit 2 via a dangerous op (rm --force forces who+why) -> same shared hint.
! exec mtt rm t1 --force
stderr 'MTT_BY'

# exit 4: a bogus id -> not-found hint points at discovery.
! exec mtt show t404
stderr 'not found'
stderr 'mtt roadmap'

# exit 4: a bogus note slug -> same hint.
! exec mtt note show nope
stderr 'mtt roadmap'

-- req.yaml --
version: 1
project: {name: reqtest}
require: {who: true, why: true}
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: tbd, kind: initial, default: true}
      - {name: in_progress, kind: active}
      - {name: done, kind: terminal}
    transitions:
      - {from: tbd, to: in_progress}
      - {from: in_progress, to: done}
```

- [ ] **Step 7: Run the e2e**

Run: `go test ./internal/cli/ -run 'TestScripts/actionable_errors' -count=1`
Expected: PASS. (If the harness runs all scripts by a single test name, run `go test ./internal/cli/ -run TestScript -count=1`.)

- [ ] **Step 8: `make check` + commit**

Run: `make check`
```bash
git add internal/cli/errors.go internal/cli/root.go internal/cli/errors_test.go \
        internal/cli/status_test.go internal/cli/testdata/scripts/actionable_errors.txt
git commit -m "t28: actionable exit-2 (attribution setup) + exit-4 (not-found) hints via Execute"
```

---

## Task 3: CLI — exit-5 recovery block (both modes) + exit-6 e2e

**Files:**
- Modify: `internal/cli/status.go`, `internal/cli/testdata/scripts/post_actions.txt`
- Test: extend `post_actions.txt`; add exit-6 cases to `actionable_errors.txt`

**Interfaces:**
- Consumes: `*core.PostActionError` (Task 1) via `errors.As`.

- [ ] **Step 1: Write the failing e2e first — migrate `post_actions.txt`**

In `internal/cli/testdata/scripts/post_actions.txt`, replace the terse assertion (L21) and extend the `--json`
block (around L29–32). Change L18–21:
```
# code renders the move THEN surfaces the recovery block (t28): the move IS saved,
# plus the exact remaining post commands to finish by hand.
! exec mtt submit t1
stdout 't1: speccing → review'
stderr 'the status change IS saved'
stderr 'do NOT re-run the move'
stderr '^  false$'
```
And in the `--json` block (after L30 `stdout '"status": *"review"'`), add — the recovery block must appear on
stderr in `--json` mode too (finding-1 guard):
```
stderr 'the status change IS saved'
stderr '^  false$'
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/cli/ -run 'TestScript' -count=1`
Expected: FAIL — the old code prints `move applied, but a post-action failed`, not the recovery block; and the
`--json` path prints no recovery block at all.

- [ ] **Step 3: Implement the recovery block** — in `internal/cli/status.go` `runTransition`, replace the post-fail handling. Insert the recovery print **before** the `--json` return (currently L110–134). New body from `applyCurrent` onward:

```go
	if err := applyCurrent(root, cfg, task, id); err != nil {
		return fmt.Errorf("transition applied but updating the current pointer failed: %w", err)
	}
	// exit-5 (t28): the move IS persisted; print the recovery block on stderr in BOTH
	// text and --json mode (an agent driving --json must still see how to finish).
	if postFailed {
		var pe *core.PostActionError
		if errors.As(txErr, &pe) {
			w := cmd.ErrOrStderr()
			_, _ = fmt.Fprintln(w, "move applied — the status change IS saved; do NOT re-run the move.")
			_, _ = fmt.Fprintln(w, "finish the finalization by hand:")
			for _, c := range pe.Remaining {
				_, _ = fmt.Fprintf(w, "  %s\n", c)
			}
		}
	}
	if jsonFlag(cmd) {
		if err := writeJSON(cmd.OutOrStdout(), toTaskJSON(task)); err != nil {
			return err
		}
		return txErr // ErrPostAction → exit 5 even in --json
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
	return txErr
```

This removes the old terse `if postFailed { Fprintf("move applied, but a post-action failed: %v") }` block at
L131–133 (superseded by the recovery block above, which now runs in both modes). Keep the `postFailed :=
errors.Is(txErr, core.ErrPostAction)` computation and the early `return txErr` for non-post errors unchanged.

- [ ] **Step 4: Add exit-6 e2e** — append to `internal/cli/testdata/scripts/actionable_errors.txt` (reuse its
`req.yaml` task type). No script re-sequencing is needed: `t1` is still in `tbd` here (the earlier
`! exec mtt in_progress t1` failed with exit 2 for missing who/why, and `! exec mtt rm t1 --force` failed with
exit 2 for the same reason — a `--force` pre-flight, so **`t1` is never actually removed**).

```
# exit 6: an impossible move from tbd (there is no tbd->done edge) lists the allowed
# targets. findTransition is checked BEFORE attribution, so this needs no --who/--why.
! exec mtt done t1
stderr 'allowed from tbd'

# drive t1 to a terminal (with attribution), then an impossible move OUT of it reads
# cleanly — no dangling 'allowed from done: '.
exec mtt in_progress t1 --who a --why b
exec mtt done t1 --who a --why b
! exec mtt in_progress t1 --who a --why b
stderr 'no moves out of done'
! stderr 'allowed from done:'
```

- [ ] **Step 5: Run the e2e**

Run: `go test ./internal/cli/ -run 'TestScript' -race -count=1`
Expected: PASS (post_actions + actionable_errors).

- [ ] **Step 6: `make check` + commit**

Run: `make check`
```bash
git add internal/cli/status.go internal/cli/testdata/scripts/post_actions.txt \
        internal/cli/testdata/scripts/actionable_errors.txt
git commit -m "t28: exit-5 recovery block (remaining post commands, both modes) + exit-6 terminal e2e"
```

---

## Task 4: docs sync

**Files:** `CLI_REFERENCE.md`↔`.ru.md`, `DESIGN.md`↔`.ru.md`, `CHANGELOG.md`, `internal/cli/CLAUDE.md`, `internal/core/CLAUDE.md`

- [ ] **Step 1: `CLI_REFERENCE.md` + `.ru.md`** — grep the exit-code list: `grep -n "exit 2\|exit 5\|exit-code\|exit code" CLI_REFERENCE.md CLI_REFERENCE.ru.md`. Where exit 2 / exit 5 are documented, add: exit 2 prints how to set who/why; exit 5 prints "the move is saved" + the exact remaining commands. Keep EN and RU in sync (parallel edit).

- [ ] **Step 2: `DESIGN.md` + `.ru.md`** — grep `ErrPostAction`/`post-persist`: `grep -n "ErrPostAction\|post-action\|post-persist" DESIGN.md DESIGN.ru.md`. In the t21 "Shipped" block, add a clause: the typed `PostActionError` carries the unfinished commands so the CLI prints exact recovery steps; the attribution note gains "the exit-2 message tells you how to set who/why". One parallel clause each (EN + RU).

- [ ] **Step 3: `CHANGELOG.md`** — under `[Unreleased]`, add a **Changed** entry:
```markdown
### Changed
- Errors are now actionable: exit 2 (missing attribution) prints how to set who/why (config.local `author:` /
  `MTT_BY` / `--who` / `--why`); exit 5 (post-action failed) says the move is already saved and prints the exact
  commands to finish by hand; exit 4 (not found) points at `mtt roadmap`/`mtt list`; an invalid move out of a
  terminal status now reads cleanly.
```

- [ ] **Step 4: CLAUDE.md** — `internal/cli/CLAUDE.md`: note `exitHint` (the `Execute` hook for exit-2/4) and the exit-5 recovery block in `runTransition`. `internal/core/CLAUDE.md`: note the typed `PostActionError` (Transitioner returns it on a post failure; `Is()`→`ErrPostAction`). Keep thin.

- [ ] **Step 5: `make check` + commit**

Run: `make check`
```bash
git add CLI_REFERENCE.md CLI_REFERENCE.ru.md DESIGN.md DESIGN.ru.md CHANGELOG.md \
        internal/cli/CLAUDE.md internal/core/CLAUDE.md
git commit -m "t28: docs — actionable errors (CLI_REFERENCE EN/RU, DESIGN EN/RU, CHANGELOG, CLAUDE)"
```

---

## Acceptance criteria mapping (spec → tasks)

- **AC1** (exit-2 hint) → Task 2 (Steps 1–8).
- **AC2** (exit-4 hint) → Task 2.
- **AC3** (exit-5 recovery, unit + e2e text + `--json`) → Task 1 (unit) + Task 3 (e2e).
- **AC4** (exit-6 verified + terminal parity) → Task 1 (core message) + Task 3 (e2e: core-path non-terminal +
  terminal). The `mtt do <bogus-edge>` path is **already** exercised by the existing `do.txt`
  (`stderr 'available:'`), so it needs no new script — Task 3 covers only the core/status path and the
  terminal-status parity fix.
- **AC5** (no hint bleed) → Task 2 (`TestExitHint` PostActionError/invalid→"").
- **AC6** (exit codes unchanged) → Task 2 (`TestExitCode` + `*PostActionError`→5).
- **AC7** (`make check` + docs) → every task's commit + Task 4.

## impl_review checklist

- Principles self-check (SOLID/DRY/KISS/TDD); hexagon (no config/env names in `core`); docs-sync judgment
  (CLI_REFERENCE EN+RU, DESIGN EN+RU, CHANGELOG, both CLAUDE.md); `make check` green.
- Sanity-run the real binary: trigger an exit-2 (missing who/why), an exit-4 (bogus id), and an exit-5 (a
  failing `post:`) and eyeball the hints/recovery block.
