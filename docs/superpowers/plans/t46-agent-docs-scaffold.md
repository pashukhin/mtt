# t46 — Scaffold agent docs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax. The
> whole implementation lands inside the mtt `implementing` status (one `mtt submit` at the end).

**Goal:** `mtt agent docs` (and `mtt init` by default) scaffold a project-agnostic "Working under mtt" runbook
into `AGENTS.md` plus pointer blocks into `CLAUDE.md`/`GEMINI.md`, via an idempotent marked-block merge —
reusing (and consolidating) t52's `internal/scaffold` seam.

**Architecture:** Reuse `scaffold.Run` (read→Merge→AtomicWrite). Consolidate the seam's naming pre-release
(`Harness`→`Target`, `Registry`→`HookTargets`, add `DocTargets`, `Result.Harness`→`Result.Name`, CLI JSON
`{harness}`→`{name}`, `initJSON.Scaffold`→`Hooks`+`Docs`). A new pure `blockMerge` injects/regenerates a
`<!-- mtt:begin -->…<!-- mtt:end -->` block; the doc content is `//go:embed`-ed markdown (backticks preclude a
Go raw literal).

**Tech Stack:** Go 1.23, cobra, `//go:embed`, `testscript` e2e, table-driven unit tests.

**Spec:** `docs/superpowers/specs/t46-agent-docs-scaffold.md` (revision 1, approved).

## Global Constraints

- **Go floor stays 1.23.1** — stdlib only (`embed` is stdlib).
- **Onboarding infra, not domain** — `internal/scaffold` stays out of `pkg/mtt` and `.mtt/` storage; writes via
  `internal/fsutil.AtomicWrite`.
- **Generic content only** — the runbook must NOT hardcode this repo's statuses (`speccing`/…) and must NOT
  assume a named edge (`start`) exists; it points at `mtt show <id>`'s `next:` line and `mtt types`.
- **No clobber** — marked-block merge preserves all surrounding content; a malformed marker pair refuses.
- **`make check` green before every commit** (gofmt + vet + golangci-lint v2 + `go test -race` + build).
- **gosec G304** — `internal/scaffold`'s file-IO already carries `#nosec G304` (t52); no new file-open is added
  in scaffold (embed is compile-time).
- **CHANGELOG `[Unreleased]` entry** before the implementing→impl_review `mtt submit`.
- **Commit under the user's git identity**; trailer `Co-Authored-By: Claude Opus 4.8 (1M context)
  <noreply@anthropic.com>`.

---

### Task 1: Consolidate the `internal/scaffold` seam (pre-release rename)

A behavior-preserving rename that makes the seam honest for a second consumer (docs). The generalized shared
reporter (`reportScaffold` gains a `note` param) lands here too, since t46 needs a docs-specific note. This is
a **refactor**: the existing (updated) tests are the safety net — `make check` green proves behavior identical.

**Files:**
- Modify: `internal/scaffold/scaffold.go` (`Harness`→`Target`, `Registry`→`HookTargets`, `Result.Harness`→`Result.Name`, `Run` sig)
- Modify: `internal/scaffold/claude.go` (`claudeHarness`→`claudeTarget`)
- Modify: `internal/scaffold/claude_test.go` (global replace `claudeHarness`→`claudeTarget` — 13 code uses + 1 comment)
- Modify: `internal/scaffold/run_test.go` (`Registry()`→`HookTargets()`, 3 uses)
- Modify: `internal/cli/agent.go` (`scaffoldJSON` `{name}`, `toScaffoldJSON`/`reportScaffold` use `r.Name`; `reportScaffold` gains `note`; hooks caller passes the hook note)
- Modify: `internal/cli/init.go` (`Registry()`→`HookTargets()`, `initJSON.Scaffold`→`Hooks`, `reportScaffold(…, hookNote)`)
- Modify: `internal/cli/json.go` (`initJSON.Scaffold`→`Hooks`)
- Modify: `internal/cli/testdata/scripts/agent_hooks.txt` (`"harness"`→`"name"`)
- Modify: `internal/cli/testdata/scripts/init.txt` (`"scaffold"`→`"hooks"`, `"harness"`→`"name"`, `"scaffold": []`→`"hooks": []`)

**Interfaces:**
- Produces: `scaffold.Target` (was `Harness`), `scaffold.HookTargets() []Target` (was `Registry`),
  `scaffold.Result{Name, Path, Action}`, `scaffold.Run(root string, targets []Target)`; CLI
  `reportScaffold(cmd, results, note string)`, `scaffoldJSON{name,path,action}`, `initJSON.Hooks`.

- [ ] **Step 1: Rename in `internal/scaffold/scaffold.go`** — apply verbatim:

```go
// Result is one target's scaffold outcome.
type Result struct {
	Name   string
	Path   string
	Action Action
}

// Target is a scaffold target (a file + a pure merge from current to desired
// bytes + a name for reporting). Merge is pure.
type Target interface {
	Name() string
	RelPath() string
	Merge(existing []byte, exists bool) (content []byte, action Action, err error)
}

// HookTargets is the set of agent-tool config targets (t52; the hooks facet).
func HookTargets() []Target { return []Target{claudeTarget{}} }

// Run scaffolds each target under root and returns per-target results. It is
// write-as-you-go: a read/merge/write failure aborts the loop and returns the
// results collected so far alongside a wrapped error. Only os.ErrNotExist marks
// a file absent; any other read error propagates.
func Run(root string, targets []Target) ([]Result, error) {
	var results []Result
	for _, t := range targets {
		path := filepath.Join(root, filepath.FromSlash(t.RelPath()))
		// #nosec G304 -- path is <root>/<target RelPath> under a locally-resolved
		// project root (projectRoot/baseDir), not network/untrusted input.
		existing, err := os.ReadFile(path)
		exists := true
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				exists, existing = false, nil
			} else {
				return results, fmt.Errorf("%s: read %s: %w", t.Name(), path, err)
			}
		}
		content, action, err := t.Merge(existing, exists)
		if err != nil {
			return results, fmt.Errorf("%s: %s: %w", t.Name(), path, err)
		}
		if action != Unchanged {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return results, fmt.Errorf("%s: mkdir %s: %w", t.Name(), filepath.Dir(path), err)
			}
			if err := fsutil.AtomicWrite(path, content, 0o644); err != nil {
				return results, fmt.Errorf("%s: write %s: %w", t.Name(), path, err)
			}
		}
		results = append(results, Result{Name: t.Name(), Path: path, Action: action})
	}
	return results, nil
}
```

  (Also update the package/`Action` doc comments that say "harness" → "target".)

- [ ] **Step 2: Rename the concrete type in `internal/scaffold/claude.go`** — `claudeHarness` → `claudeTarget`
  (the struct decl + its three method receivers `func (claudeTarget) …`). Update the doc comment. In
  `internal/scaffold/claude_test.go`, **global-replace every `claudeHarness` → `claudeTarget`** (13 code uses
  + 1 comment; e.g. `sed -i 's/claudeHarness/claudeTarget/g'`) — a residual `claudeHarness` is a compile error,
  so a partial edit fails the gate.

- [ ] **Step 3: Rename in `internal/scaffold/run_test.go`** — `Registry()` → `HookTargets()` (3 call sites).

- [ ] **Step 4: Update the CLI reporter in `internal/cli/agent.go`**:

```go
// scaffoldJSON is the machine-readable per-target scaffold result.
type scaffoldJSON struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Action string `json:"action"`
}

// toScaffoldJSON maps results to the non-null JSON view (empty slice, not null).
func toScaffoldJSON(results []scaffold.Result) []scaffoldJSON {
	out := make([]scaffoldJSON, 0, len(results))
	for _, r := range results {
		out = append(out, scaffoldJSON{Name: r.Name, Path: r.Path, Action: string(r.Action)})
	}
	return out
}

// reportScaffold renders results (shared by `agent hooks`, `agent docs`, `init`).
// JSON: the non-null array. Human: one line per target + `note` on stderr when
// anything changed (note "" suppresses it).
func reportScaffold(cmd *cobra.Command, results []scaffold.Result, note string) error {
	if jsonFlag(cmd) {
		return writeJSON(cmd.OutOrStdout(), toScaffoldJSON(results))
	}
	changed := false
	for _, r := range results {
		if r.Action != scaffold.Unchanged {
			changed = true
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s %s\n", r.Name, r.Action, r.Path); err != nil {
			return err
		}
	}
	if changed && note != "" {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), note)
	}
	return nil
}
```

  And define the hook note as a package const near `newAgentHooksCmd`, and pass it:

```go
const hookScaffoldNote = "→ a SessionStart/PreCompact hook now runs `mtt prime` at session start; review the settings file."
```

  In `newAgentHooksCmd`'s RunE, change `return reportScaffold(cmd, results)` →
  `return reportScaffold(cmd, results, hookScaffoldNote)`. Also change its `scaffold.Run(root,
  scaffold.Registry())` → `scaffold.Run(root, scaffold.HookTargets())`.

- [ ] **Step 5: Update `internal/cli/init.go`** — `scaffold.Registry()` → `scaffold.HookTargets()`; the
  `initJSON` literal field `Scaffold: toScaffoldJSON(scaffoldResults)` → `Hooks: toScaffoldJSON(scaffoldResults)`;
  the final human line `return reportScaffold(cmd, scaffoldResults)` → `return reportScaffold(cmd,
  scaffoldResults, hookScaffoldNote)`.

- [ ] **Step 6: Update `internal/cli/json.go`** — rename the `initJSON` field:

```go
// initJSON is `mtt init --json`: the created-config summary + agent-scaffold
// results (non-null arrays; [] when the respective opt-out is set).
type initJSON struct {
	Path     string         `json:"path"`
	Template string         `json:"template"`
	Source   string         `json:"source,omitempty"`
	Name     string         `json:"name"`
	Created  bool           `json:"created"`
	Hooks    []scaffoldJSON `json:"hooks"`
}
```

  (The `Docs` field is added in Task 3.)

- [ ] **Step 7: Update the t52 test assertions**:
  - `internal/cli/testdata/scripts/agent_hooks.txt:23` — `stdout '"harness": "claude"'` → `stdout '"name": "claude"'`.
  - `internal/cli/testdata/scripts/init.txt:124` — `stdout '"scaffold"'` → `stdout '"hooks"'`.
  - `internal/cli/testdata/scripts/init.txt:125` — `stdout '"harness": "claude"'` → `stdout '"name": "claude"'`.
  - `internal/cli/testdata/scripts/init.txt:132` — `stdout '"scaffold": \[\]'` → `stdout '"hooks": \[\]'`.

- [ ] **Step 8: Run the gate**

Run: `make check`
Expected: green (a pure rename — the updated tests pass unchanged in behavior).

- [ ] **Step 9: Commit**

```bash
git add internal/scaffold internal/cli/agent.go internal/cli/init.go internal/cli/json.go internal/cli/testdata/scripts/agent_hooks.txt internal/cli/testdata/scripts/init.txt
git commit -m "t46: consolidate internal/scaffold seam (Target/HookTargets, Result.Name, init.hooks)"
```

---

### Task 2: `blockMerge` + `DocTargets` (the doc content + marked-block merge)

The pure markdown block merge and the three doc targets, with `//go:embed`-ed content.

**Files:**
- Create: `internal/scaffold/docs.go` (`blockMerge`, `docTarget`, `DocTargets`, embedded content)
- Create: `internal/scaffold/docdata/agents.md` (the runbook content — no markers)
- Create: `internal/scaffold/docdata/pointer.md` (the CLAUDE.md/GEMINI.md pointer content — no markers)
- Create: `internal/scaffold/docs_test.go` (table-driven `blockMerge` + `DocTargets`)
- Modify: `internal/scaffold/CLAUDE.md` (document the doc facet)

**Interfaces:**
- Consumes: `scaffold.Target`/`Action`/`Created`/`Merged`/`Unchanged` (Task 1).
- Produces: `func DocTargets() []Target`; the pure `blockMerge(existing []byte, exists bool, content string)
  ([]byte, Action, error)`.

- [ ] **Step 1: Write the failing `blockMerge` tests** — `internal/scaffold/docs_test.go`

```go
package scaffold

import (
	"strings"
	"testing"
)

func TestBlockMergeCreatedFromAbsent(t *testing.T) {
	got, action, err := blockMerge(nil, false, "hello")
	if err != nil {
		t.Fatalf("blockMerge: %v", err)
	}
	if action != Created {
		t.Fatalf("action = %q, want created", action)
	}
	want := "<!-- mtt:begin -->\nhello\n<!-- mtt:end -->\n"
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBlockMergeEmptyTreatedAsAbsent(t *testing.T) {
	got, action, err := blockMerge([]byte("  \n\t"), true, "hi")
	if err != nil {
		t.Fatalf("blockMerge: %v", err)
	}
	if action != Created || !strings.Contains(string(got), "<!-- mtt:begin -->") {
		t.Fatalf("empty should create; action %q:\n%s", action, got)
	}
}

func TestBlockMergeAppendsPreservingContent(t *testing.T) {
	existing := []byte("# My Project\n\nSome notes.\n")
	got, action, err := blockMerge(existing, true, "runbook")
	if err != nil {
		t.Fatalf("blockMerge: %v", err)
	}
	if action != Merged {
		t.Fatalf("action = %q, want merged", action)
	}
	if !strings.HasPrefix(string(got), "# My Project\n\nSome notes.\n") {
		t.Fatalf("original content not preserved:\n%s", got)
	}
	if !strings.Contains(string(got), "<!-- mtt:begin -->\nrunbook\n<!-- mtt:end -->\n") {
		t.Fatalf("block not appended:\n%s", got)
	}
}

func TestBlockMergeConverges(t *testing.T) {
	// append then re-merge => Unchanged (one-step convergence).
	existing := []byte("# P\n")
	first, _, err := blockMerge(existing, true, "runbook")
	if err != nil {
		t.Fatalf("blockMerge(1): %v", err)
	}
	_, action, err := blockMerge(first, true, "runbook")
	if err != nil {
		t.Fatalf("blockMerge(2): %v", err)
	}
	if action != Unchanged {
		t.Fatalf("second = %q, want unchanged", action)
	}
}

func TestBlockMergeRegeneratesStale(t *testing.T) {
	existing := []byte("intro\n\n<!-- mtt:begin -->\nOLD\n<!-- mtt:end -->\ntail\n")
	got, action, err := blockMerge(existing, true, "NEW")
	if err != nil {
		t.Fatalf("blockMerge: %v", err)
	}
	if action != Merged {
		t.Fatalf("action = %q, want merged", action)
	}
	want := "intro\n\n<!-- mtt:begin -->\nNEW\n<!-- mtt:end -->\ntail\n"
	if string(got) != want {
		t.Fatalf("regenerate mismatch:\ngot  %q\nwant %q", got, want)
	}
}

func TestBlockMergeRefusesMalformed(t *testing.T) {
	for _, in := range []string{
		"x\n<!-- mtt:begin -->\nno end\n",
		"x\n<!-- mtt:end -->\nno begin\n",
		"<!-- mtt:end -->\n<!-- mtt:begin -->\nreversed\n",
		"<!-- mtt:begin -->\na\n<!-- mtt:end -->\n<!-- mtt:begin -->\nb\n<!-- mtt:end -->\n",
	} {
		if _, _, err := blockMerge([]byte(in), true, "x"); err == nil {
			t.Fatalf("malformed markers %q must refuse", in)
		}
	}
}

func TestDocTargets(t *testing.T) {
	targets := DocTargets()
	wantPaths := map[string]string{"agents": "AGENTS.md", "claude": "CLAUDE.md", "gemini": "GEMINI.md"}
	if len(targets) != 3 {
		t.Fatalf("want 3 doc targets, got %d", len(targets))
	}
	for _, tg := range targets {
		if wantPaths[tg.Name()] != tg.RelPath() {
			t.Fatalf("target %q RelPath = %q, want %q", tg.Name(), tg.RelPath(), wantPaths[tg.Name()])
		}
		got, action, err := tg.Merge(nil, false)
		if err != nil || action != Created {
			t.Fatalf("target %q create: action %q err %v", tg.Name(), action, err)
		}
		if !strings.Contains(string(got), "Working under mtt") {
			t.Fatalf("target %q content missing the runbook heading:\n%s", tg.Name(), got)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/scaffold/ -run 'TestBlockMerge|TestDocTargets'`
Expected: FAIL — `undefined: blockMerge` / `DocTargets`.

- [ ] **Step 3: Create the content assets.** `internal/scaffold/docdata/agents.md` (verbatim — this is the
  runbook body, no markers; markers are added by `canonicalBlock`):

```markdown
## Working under mtt

This project tracks its tasks and workflow in **mtt** — a local, file-backed task **state machine**. You do
work by moving a task through its type's flow; each transition can run **gate commands that BLOCK the move on
failure**, so "done" means the checks passed.

**Discover THIS project's flow (don't assume it):**
- `mtt roadmap` — what to work on next (dependency + priority order).
- `mtt ready` — tasks that are unblocked and can be started.
- `mtt types` — the task types with their statuses, transitions, and gate commands (the Definition of Done).
- `mtt show <id>` — a task's details, current status, and the exact next moves available from here.

**The work loop:**
1. Pick a task from `mtt roadmap`.
2. Take it into work using the flow's first move (see the `next:` line in `mtt show <id>` for the exact verb).
3. Do the work, then advance with `mtt <status> <id>` or `mtt <edge> <id>` — one of the moves `mtt show`
   lists. A transition's gate commands run first and BLOCK the move if any fails; read the guidance printed on
   each move.
4. Close a task only by reaching a terminal status through a flow edge. Never delete a task to "finish" it.

**Attribution:** every move records who made it. Set `author:` once in `.mtt/config.local.yaml` (personal,
gitignored), or pass `--who <you>`. Record a reason with `--why "<why>"`; the project may **require** who/why
on some moves and always on dangerous operations (a forced delete, a gate bypass) — mtt tells you when.

**Storage:** `.mtt/` is committed project data. Task files are written **only** by mtt — never hand-edit them;
use `mtt add` / `mtt edit` / the flow moves, and mtt keeps history and determinism.

**Knowledge:** record durable decisions and lessons with `mtt note add`; `mtt prime` prints the important
notes — wire it into your agent's session start.

Run `mtt --help` or `mtt <command> --help` for the full surface. This section is scaffolded by
`mtt agent docs` and is regenerated on re-run — put your own notes OUTSIDE the mtt:begin/mtt:end markers.
```

  And `internal/scaffold/docdata/pointer.md`:

```markdown
## Working under mtt

This project uses **mtt** for its tasks and workflow. The full runbook is in **AGENTS.md** (the "Working under
mtt" section). Start with `mtt roadmap` (what's next), `mtt types` (this project's flow + gates), and
`mtt show <id>` (a task and its next moves). This block is scaffolded by `mtt agent docs`; edit OUTSIDE the
mtt:begin/mtt:end markers.
```

- [ ] **Step 4: Create `internal/scaffold/docs.go`**

```go
package scaffold

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed docdata/agents.md
var runbookContent string

//go:embed docdata/pointer.md
var pointerContent string

// Block markers (HTML comments — invisible in rendered markdown).
const (
	blockBegin = "<!-- mtt:begin -->"
	blockEnd   = "<!-- mtt:end -->"
)

// docTarget scaffolds one markdown doc file by injecting a marked block.
type docTarget struct {
	name    string
	relPath string
	content string
}

func (t docTarget) Name() string    { return t.name }
func (t docTarget) RelPath() string { return t.relPath }
func (t docTarget) Merge(existing []byte, exists bool) ([]byte, Action, error) {
	return blockMerge(existing, exists, t.content)
}

// DocTargets returns the agent-doc scaffold targets: the runbook (AGENTS.md) and
// the pointer stubs (CLAUDE.md, GEMINI.md, sharing the pointer content).
func DocTargets() []Target {
	return []Target{
		docTarget{name: "agents", relPath: "AGENTS.md", content: runbookContent},
		docTarget{name: "claude", relPath: "CLAUDE.md", content: pointerContent},
		docTarget{name: "gemini", relPath: "GEMINI.md", content: pointerContent},
	}
}

// canonicalBlock wraps content in the markers, normalizing content to end with
// exactly one newline: begin\n + content + \n + end\n.
func canonicalBlock(content string) string {
	body := strings.TrimRight(content, "\n") + "\n"
	return blockBegin + "\n" + body + blockEnd + "\n"
}

// blockMerge injects/regenerates the marked block into existing markdown. Pure:
// create-if-absent(or empty), append-if-no-markers, regenerate-if-one-pair,
// refuse a malformed marker pair — never clobbering surrounding content.
func blockMerge(existing []byte, exists bool, content string) ([]byte, Action, error) {
	block := canonicalBlock(content)
	if !exists || len(bytes.TrimSpace(existing)) == 0 {
		return []byte(block), Created, nil
	}
	s := string(existing)
	nBegin := strings.Count(s, blockBegin)
	nEnd := strings.Count(s, blockEnd)
	switch {
	case nBegin == 0 && nEnd == 0:
		prefix := s
		if !strings.HasSuffix(prefix, "\n") {
			prefix += "\n"
		}
		return []byte(prefix + "\n" + block), Merged, nil
	case nBegin == 1 && nEnd == 1:
		i := strings.Index(s, blockBegin)
		j := strings.Index(s, blockEnd)
		if j < i {
			return nil, "", fmt.Errorf("cannot merge: %q appears before %q", blockEnd, blockBegin)
		}
		k := j + len(blockEnd)
		if k < len(s) && s[k] == '\n' {
			k++
		}
		out := s[:i] + block + s[k:]
		if out == s {
			return existing, Unchanged, nil
		}
		return []byte(out), Merged, nil
	default:
		return nil, "", fmt.Errorf("cannot merge: %d %q / %d %q markers (expected one pair)",
			nBegin, blockBegin, nEnd, blockEnd)
	}
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/scaffold/ -race -run 'TestBlockMerge|TestDocTargets' -v`
Expected: PASS.

- [ ] **Step 6: Update `internal/scaffold/CLAUDE.md`** — add a paragraph:

```markdown
## Doc facet (t46)

`DocTargets()` (`docs.go`) returns the agent-doc targets — `AGENTS.md` (the project-agnostic "Working under
mtt" runbook) + `CLAUDE.md`/`GEMINI.md` (pointer stubs). Each `docTarget.Merge` delegates to the pure
`blockMerge`: a `<!-- mtt:begin -->…<!-- mtt:end -->` block that is created / appended / **regenerated** (never
clobbering surrounding content; a malformed marker pair refuses). Content is `//go:embed`-ed from `docdata/`
(backticks preclude a Go raw literal). The seam interface is `Target` (was `Harness`); `HookTargets()` (t52) +
`DocTargets()` (t46) are its two producers, both consumed by `Run`.
```

  (Also flip the existing `Harness`/`Registry` references in this file to `Target`/`HookTargets`.)

- [ ] **Step 7: Gate & commit**

```bash
make check
git add internal/scaffold/docs.go internal/scaffold/docdata internal/scaffold/docs_test.go internal/scaffold/CLAUDE.md
git commit -m "t46: internal/scaffold docs facet — blockMerge + DocTargets (embedded runbook)"
```

---

### Task 3: `mtt agent docs` command + init integration

Wire `mtt agent docs` over `scaffold.Run(root, DocTargets())` and make `mtt init` scaffold docs by default.

**Files:**
- Modify: `internal/cli/agent.go` (`newAgentDocsCmd`, register it, `docScaffoldNote`)
- Modify: `internal/cli/init.go` (`--no-agent-docs`, docs scaffold + failure contract, `initJSON.Docs`, output)
- Modify: `internal/cli/json.go` (`initJSON.Docs`)
- Modify: `internal/cli/CLAUDE.md` (document `agent docs` + the renamed reporting)
- Create: `internal/cli/testdata/scripts/agent_docs.txt`
- Modify: `internal/cli/testdata/scripts/init.txt` (docs-by-default, `--no-agent-docs`, `docs` JSON)

**Interfaces:**
- Consumes: `scaffold.DocTargets` (Task 2), `reportScaffold`/`toScaffoldJSON` (Task 1).

- [ ] **Step 1: Write the failing e2e** — `internal/cli/testdata/scripts/agent_docs.txt`

```
# agent docs scaffolds the runbook + pointers into an initialized project
mkdir proj
cd proj
exec mtt init --no-agent-docs --no-agent-hooks
! exists AGENTS.md
exec mtt agent docs
stdout 'agents: created'
stdout 'claude: created'
stdout 'gemini: created'
exists AGENTS.md
exists CLAUDE.md
exists GEMINI.md
grep 'Working under mtt' AGENTS.md
grep 'mtt:begin' AGENTS.md
grep 'mtt roadmap' AGENTS.md
grep 'AGENTS.md' CLAUDE.md

# idempotent second run
exec mtt agent docs
stdout 'agents: unchanged'

# --json surfaces the result array
exec mtt agent docs --json
stdout '"name": "agents"'
stdout '"action": "unchanged"'

# merge into a pre-existing AGENTS.md preserves the original content
cd $WORK
mkdir proj2
cd proj2
exec mtt init --no-agent-docs --no-agent-hooks
cp $WORK/existing-agents.md AGENTS.md
exec mtt agent docs
stdout 'agents: merged'
grep 'My existing runbook' AGENTS.md
grep 'Working under mtt' AGENTS.md

# outside a project: actionable error
cd $WORK
mkdir empty
cd empty
! exec mtt agent docs
stderr 'mtt init'

-- existing-agents.md --
# AGENTS.md

My existing runbook. Do not clobber me.
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run 'TestScripts/agent_docs'`
Expected: FAIL — `unknown command "docs"` for `mtt agent`.

- [ ] **Step 3: Add `newAgentDocsCmd` and register it** in `internal/cli/agent.go`:

```go
const docScaffoldNote = "→ a `Working under mtt` runbook was scaffolded into AGENTS.md (+ CLAUDE.md/GEMINI.md pointers); review it."

// newAgentDocsCmd builds `mtt agent docs` — scaffold a project-agnostic
// "Working under mtt" runbook (AGENTS.md) + pointer stubs (CLAUDE.md, GEMINI.md).
// Idempotent; re-runnable.
func newAgentDocsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "docs",
		Short: "Scaffold generic how-to-use-mtt agent docs (AGENTS.md runbook + CLAUDE.md/GEMINI.md pointers)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			results, err := scaffold.Run(root, scaffold.DocTargets())
			if err != nil {
				return err
			}
			return reportScaffold(cmd, results, docScaffoldNote)
		},
	}
}
```

  and add it to `newAgentCmd`: `cmd.AddCommand(newAgentHooksCmd(), newAgentDocsCmd())`. Update the group's
  `Short`/comment to mention both facets.

- [ ] **Step 4: Add `initJSON.Docs`** in `internal/cli/json.go`:

```go
	Hooks    []scaffoldJSON `json:"hooks"`
	Docs     []scaffoldJSON `json:"docs"`
```

- [ ] **Step 5: Wire docs into `internal/cli/init.go`** — add the flag var + registration, and extend the
  scaffold tail. Add to the `var (...)` block: `noAgentDocs bool`; and register:

```go
	cmd.Flags().BoolVar(&noAgentDocs, "no-agent-docs", false, "skip scaffolding agent docs (AGENTS.md/CLAUDE.md/GEMINI.md)")
```

  Replace the scaffold tail (from the `var scaffoldResults` line through the final `return reportScaffold`)
  with hooks-then-docs:

```go
			// Config (the switch above) is init's primary job and has succeeded.
			// Scaffold agent hooks then docs by default (opt-out flags); a scaffold
			// failure is reported AFTER config-success and exits 1 — config kept (R3).
			var hookResults, docResults []scaffold.Result
			if !noAgentHooks {
				var serr error
				hookResults, serr = scaffold.Run(base, scaffold.HookTargets())
				if serr != nil {
					// hooks failed: config-success only (nothing scaffolded landed).
					return reportInitConfigThen(cmd, tmpl, nil, serr)
				}
			}
			if !noAgentDocs {
				var derr error
				docResults, derr = scaffold.Run(base, scaffold.DocTargets())
				if derr != nil {
					// docs failed: config-success + the hook lines that DID land.
					return reportInitConfigThen(cmd, tmpl, hookResults, derr)
				}
			}
			if jsonFlag(cmd) {
				absBase, err := filepath.Abs(base)
				if err != nil {
					return err
				}
				return writeJSON(cmd.OutOrStdout(), initJSON{
					Path:     filepath.Join(absBase, ".mtt", "config.yaml"),
					Template: tmpl, Source: source, Name: projectName, Created: true,
					Hooks: toScaffoldJSON(hookResults), Docs: toScaffoldJSON(docResults),
				})
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "initialized .mtt/config.yaml (template %q)\n", tmpl); err != nil {
				return err
			}
			if err := reportScaffold(cmd, hookResults, hookScaffoldNote); err != nil {
				return err
			}
			return reportScaffold(cmd, docResults, docScaffoldNote)
		},
	}
```

  Add one small helper in `init.go` (keeps the failure contract DRY):

```go
// reportInitConfigThen prints the config-success line then the per-target human
// lines of what DID land (human mode), and returns the scaffold error → exit 1;
// config is written and kept, no partial JSON (R3). landed is nil on a hooks
// failure (nothing scaffolded), or the hook results on a docs failure.
func reportInitConfigThen(cmd *cobra.Command, tmpl string, landed []scaffold.Result, serr error) error {
	if !jsonFlag(cmd) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "initialized .mtt/config.yaml (template %q)\n", tmpl)
		for _, r := range landed {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s %s\n", r.Name, r.Action, r.Path)
		}
	}
	return serr
}
```

  Order on a docs failure: config-success line, then the hook lines that landed, then the error on stderr →
  exit 1, no partial JSON. On a hooks failure: config-success line + the error (no scaffold lines).

- [ ] **Step 6: Extend the init e2e** — in `internal/cli/testdata/scripts/init.txt`, the existing t52 scaffold
  block already `cd`s into fresh dirs. Add docs assertions. After the `scaffoldproj` block's
  `grep 'mtt prime 2>/dev/null' .claude/settings.json` line, add:

```
# t46: init also scaffolds docs by default
exists AGENTS.md
stdout 'agents: created'
grep 'Working under mtt' AGENTS.md
```

  Update the `noscaffold` block (`--no-agent-hooks`) to also opt out of docs (so it stays a clean no-scaffold
  case) — change its `exec mtt init --no-agent-hooks` to `exec mtt init --no-agent-hooks --no-agent-docs`, and
  add `! exists AGENTS.md`. Add a dedicated `--no-agent-docs` case:

```
# --no-agent-docs opts docs out (hooks still scaffold)
cd $WORK
mkdir nodocs
cd nodocs
exec mtt init --no-agent-docs
exists .claude/settings.json
! exists AGENTS.md
```

  In the `jsonscaffold` block, add `stdout '"docs"'` and `stdout '"name": "agents"'`. Add a
  `--no-agent-docs --json` case (full boilerplate):

```
# --no-agent-docs --json: docs is a non-null EMPTY array
cd $WORK
mkdir nodocsjson
cd nodocsjson
exec mtt init --no-agent-docs --json
stdout '"docs": \[\]'
stdout '"hooks"'
```

- [ ] **Step 7: Run the e2e to verify it passes**

Run: `go test ./internal/cli/ -run 'TestScripts/(agent_docs|init)' -v`
Expected: PASS.

- [ ] **Step 8: Document in `internal/cli/CLAUDE.md`** — extend the Agent-scaffolding paragraph:

```markdown
`mtt agent docs` (`agent.go`, `newAgentDocsCmd`) scaffolds the generic runbook (AGENTS.md) + pointer stubs
(CLAUDE.md/GEMINI.md) via `scaffold.Run(root, scaffold.DocTargets())` and the shared `reportScaffold` (with a
docs note). `mtt init` runs BOTH `HookTargets()` (unless `--no-agent-hooks`) and `DocTargets()` (unless
`--no-agent-docs`) by default, folding `hooks`+`docs` into `initJSON` and the output; the config-then-scaffold
failure contract (config written & kept, exit 1, no partial JSON) extends to the docs phase. e2e: `agent_docs.txt`.
```

- [ ] **Step 9: Gate & commit**

```bash
make check
git add internal/cli/agent.go internal/cli/init.go internal/cli/json.go internal/cli/CLAUDE.md internal/cli/testdata/scripts/agent_docs.txt internal/cli/testdata/scripts/init.txt
git commit -m "t46: mtt agent docs command + init scaffolds docs by default (--no-agent-docs)"
```

---

### Task 4: Docs sync (behavior change + t52 consolidation)

**Files:**
- Modify: `DESIGN.md` (+ `DESIGN.ru.md`) — Shipped(t46) note + amend t52 seam-name wording.
- Modify: `FLOW_GUIDE.md` (+ `FLOW_GUIDE.ru.md`) — flip the "Agent-usage docs" bullet; name all three files.
- Modify: `CLI_REFERENCE.md` (+ `.ru`) — `mtt agent docs` + `--no-agent-docs`; update t52 `agent hooks` `--json`
  `{harness}`→`{name}` and `init --json` `scaffold`→`hooks`+`docs`.
- Modify: `CHANGELOG.md` — `[Unreleased]` entry for `mtt agent docs`; amend the t52 entry's `--json` wording +
  its prose `Harness` seam → `Target`.
- Modify: `README.md` (+ `.ru`) — Quickstart `mtt init` line already notes hooks; add docs + `--no-agent-docs`.

(No code, no TDD; prose. `make check` still run to keep build/CHANGELOG-gate honest.)

- [ ] **Step 1: DESIGN.md (+ ru)** — add a **Shipped (t46)** note after the Shipped(t52) block: `mtt agent
  docs` (sibling in the `mtt agent` group), init default + `--no-agent-docs`, the doc set (AGENTS.md runbook +
  CLAUDE.md/GEMINI.md pointers), the project-agnostic content that points at `mtt types`/`mtt roadmap`, the
  marked-block `blockMerge` (create/append/regenerate/refuse), and the seam consolidation
  (`Target`/`HookTargets`/`DocTargets`, reporting `name`, init `hooks`+`docs`). Grep the Shipped(t52) note for
  the JSON key `harness`/`scaffold` and update to `name`/`hooks`/`docs`.

- [ ] **Step 2: FLOW_GUIDE.md:~291 (+ ru:~294)** — flip the "Agent-usage docs — a generic tool-level
  `AGENTS.md`/`CLAUDE.md`…" bullet to *shipped (t46)*, naming **AGENTS.md / CLAUDE.md / GEMINI.md**.

- [ ] **Step 3: CLI_REFERENCE.md (+ ru)** — add a `mtt agent docs` entry (what it writes, idempotent,
  no-clobber marked block, `projectRoot`) and `mtt init --no-agent-docs`. Update the t52 `mtt agent hooks`
  `--json` shape `[{harness, path, action}]` → `[{name, …}]`, and the `init --json` `scaffold` field →
  `hooks` + `docs`.

- [ ] **Step 4: CHANGELOG.md** — under `[Unreleased] ### Added`, a `mtt agent docs` bullet; amend the t52
  bullet's `init --json` wording (`scaffold` → `hooks`/`docs`) and its prose "extensible `Harness` seam" →
  "`Target` seam".

- [ ] **Step 5: README.md (+ ru)** — extend the Quickstart `mtt init` comment (hooks + docs; `--no-agent-hooks`/
  `--no-agent-docs` to skip).

- [ ] **Step 6: Gate & commit**

```bash
make check
git add DESIGN.md DESIGN.ru.md FLOW_GUIDE.md FLOW_GUIDE.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md CHANGELOG.md README.md README.ru.md
git commit -m "t46: docs — mtt agent docs + init default; sync DESIGN/FLOW_GUIDE/CLI_REFERENCE/README (EN+RU); t52 seam-name"
```

---

## Self-Review

**Spec coverage:**
- Surface (`mtt agent docs`, init default + `--no-agent-docs`, root resolution) → Task 3.
- Doc set + content (AGENTS.md runbook + CLAUDE.md/GEMINI.md pointers; generic; points at `mtt types`) → Task 2
  (assets + `DocTargets`).
- Merge semantics (`blockMerge`: create/append/regenerate/refuse/converge) → Task 2 tests.
- Architecture / API consolidation (`Target`/`HookTargets`/`DocTargets`, `Result.Name`, JSON `name`,
  `hooks`+`docs`) → Task 1 (+ the change-inventory) and Task 3.
- init failure contract (config kept, exit 1, no partial JSON) → Task 3.
- No silent traps (opt-out, no-clobber, reported) → Tasks 2, 3.
- Docs (EN+RU, CLAUDE.md files, CHANGELOG, t52 seam-name sweep) → Tasks 2, 3, 4.

**Placeholder scan:** no TBD/TODO; full code for the new logic; Task 1/4 are precise rename/prose edits naming
exact files.

**Type consistency:** `Target`/`HookTargets`/`DocTargets`/`Result.Name`/`Run` (Task 1) used by `docs.go`
(Task 2) and the CLI (Tasks 1, 3); `reportScaffold(cmd, results, note)` signature is set in Task 1 and reused
in Task 3; `initJSON.Hooks` (Task 1) + `Docs` (Task 3); `blockMerge`/`DocTargets` (Task 2) consumed by Task 3.
