# s009 Dogfood / self-host — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make mtt track its own development — a committed `.mtt/config.yaml` (one `task` type with a 15-status gated maturation flow), a migrated flat product backlog, a Go guard test, and a mechanism e2e — with no production logic change.

**Architecture:** Hand-author the committed config + task data (data + config, not code). Guard the config with a Go test (`Config.Validate` runs on `add`/`types`, never on `Load` — the test is the sole load-time guard). Prove the branch/gate/current mechanism with a `testscript` e2e on a scratch config with fake commands (the real `make check`/git gates need a real repo). Then migrate the backlog via `./bin/mtt add`, and sync docs + freeze `TASKS.md`.

**Tech Stack:** Go 1.23, cobra, `gopkg.in/yaml.v3`, `github.com/rogpeppe/go-internal/testscript` (e2e). Domain in `pkg/mtt`; YAML adapter in `internal/adapter/yaml`.

## Global Constraints

- **No production logic change:** only new tests + the version string may touch `pkg/`/`core`/`adapter`/`cli`. Everything else is expressible with shipped capabilities. If a real gap surfaces, STOP and raise it (a separate in-scope enabler), don't smuggle logic into this session.
- **Spec is authoritative:** `docs/superpowers/specs/2026-07-09-session-009-dogfood-design.md`.
- **Gate commands in YAML are single-quoted scalars** (never double-quoted: `\.mtt/` is an invalid escape → `Load` fails; never a plain `!`-leading scalar: the `!` is dropped as a YAML tag). Inner single-quotes are doubled (`''`).
- **`make check` green before every commit** (`gofmt` + `vet` + `golangci-lint` v2 + `go test -race -cover` + build).
- **Branch:** work on the existing `feat/s009-dogfood` (already created from `main`; the design commits are on it). One commit per task.
- **Version bump:** `0.8.98-dev → 0.9.0-dev` ([internal/cli/root.go:16](../../../internal/cli/root.go#L16)).
- **Module path** for imports: `github.com/pashukhin/mtt`.
- **Status names (exact, 15):** `tbd`; `speccing`, `spec_review`, `spec_human_review`, `spec_fix`; `planning`, `plan_review`, `plan_human_review`, `plan_fix`; `implementing`, `impl_review`, `impl_human_review`, `impl_fix`; `done`, `cancelled`. Do-status→stem: `speccing`→`spec_`, `planning`→`plan_`, `implementing`→`impl_` (NOT literal).
- **Edge names:** `start` (entry), `submit` (do/fix→review), `approve`/`decline` (review forks), `cancel`.

---

### Task 1: Committed `.mtt/config.yaml` + `TestRepoDogfoodConfig` guard

**Files:**
- Create: `internal/adapter/yaml/dogfood_test.go`
- Create: `.mtt/config.yaml` (repo root)

**Interfaces:**
- Consumes (shipped, do not modify): `yaml.FindRoot(start string) (string, error)`; `yaml.Load(root string) (mtt.Config, yaml.Settings, error)` where `Settings{Prefixes map[string]string, Require RequireAttribution{Who,Why bool}, …}`; `mtt.Config.Validate() error`; `mtt.Type{Name, Default, Parents, Flow{Statuses,Transitions}}`; `mtt.Type.StatusByName(StatusName) (Status,bool)`; `mtt.Type.FindTransition(from,to StatusName) (Transition,bool)`; `mtt.Status{Name,Kind}`; `mtt.Transition{From,To,Name,Commands,Current}`; `mtt.Command{Run}`; consts `mtt.KindInitial|KindActive|KindTerminal`, `mtt.CurrentSet|CurrentClear`.
- Produces: the committed `.mtt/config.yaml` that all later tasks and the repo's own `mtt` invocations use.

- [ ] **Step 1: Write the failing guard test**

Create `internal/adapter/yaml/dogfood_test.go`:

```go
package yaml

import (
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// TestRepoDogfoodConfig is the SOLE guard of this repo's committed
// .mtt/config.yaml: Config.Validate runs on add/types, never on Load, so a
// broken committed config would ship silently otherwise. It locates the repo
// root (FindRoot walks up from this package dir), Load+Validate, and asserts the
// single `task` type's full 15-status gated flow with EXACT gate command strings
// (a YAML-mangled or inverted gate must fail HERE, not at runtime).
func TestRepoDogfoodConfig(t *testing.T) {
	root, err := FindRoot(".")
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	cfg, settings, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if len(cfg.Types) != 1 {
		t.Fatalf("types = %d, want 1", len(cfg.Types))
	}
	task := cfg.Types[0]
	if task.Name != "task" {
		t.Fatalf("type name = %q, want task", task.Name)
	}
	if !task.Default {
		t.Fatalf("task type must be default")
	}
	if settings.Prefixes["task"] != "t" {
		t.Fatalf("prefix = %q, want t", settings.Prefixes["task"])
	}
	if !settings.Require.Who {
		t.Fatalf("require.who must be true")
	}

	wantKind := map[mtt.StatusName]mtt.StatusKind{
		"tbd":               mtt.KindInitial,
		"speccing":          mtt.KindActive,
		"spec_review":       mtt.KindActive,
		"spec_human_review": mtt.KindActive,
		"spec_fix":          mtt.KindActive,
		"planning":          mtt.KindActive,
		"plan_review":       mtt.KindActive,
		"plan_human_review": mtt.KindActive,
		"plan_fix":          mtt.KindActive,
		"implementing":      mtt.KindActive,
		"impl_review":       mtt.KindActive,
		"impl_human_review": mtt.KindActive,
		"impl_fix":          mtt.KindActive,
		"done":              mtt.KindTerminal,
		"cancelled":         mtt.KindTerminal,
	}
	if len(task.Statuses) != len(wantKind) {
		t.Fatalf("statuses = %d, want %d", len(task.Statuses), len(wantKind))
	}
	for name, kind := range wantKind {
		s, ok := task.StatusByName(name)
		if !ok {
			t.Fatalf("status %q missing", name)
		}
		if s.Kind != kind {
			t.Fatalf("status %q kind = %q, want %q", name, s.Kind, kind)
		}
	}

	edge := func(from, to mtt.StatusName) mtt.Transition {
		tr, ok := task.FindTransition(from, to)
		if !ok {
			t.Fatalf("edge %s -> %s missing", from, to)
		}
		return tr
	}
	run1 := func(tr mtt.Transition) string {
		if len(tr.Commands) != 1 {
			t.Fatalf("edge %s->%s: %d commands, want 1", tr.From, tr.To, len(tr.Commands))
		}
		return tr.Commands[0].Run
	}

	// entry: named `start`, current:set, EXACT idempotent branch command
	start := edge("tbd", "speccing")
	if start.Name != "start" || start.Current != mtt.CurrentSet {
		t.Fatalf("entry edge = {name:%q current:%q}, want {start set}", start.Name, start.Current)
	}
	if got := run1(start); got != `git switch -c task/{{.ID}} || git switch task/{{.ID}}` {
		t.Fatalf("entry branch command = %q", got)
	}

	// spec/plan submit edges carry the EXACT proxy; named `submit`
	const proxy = `git status --porcelain | grep -qv '\.mtt/'`
	for _, e := range [][2]mtt.StatusName{
		{"speccing", "spec_review"}, {"spec_fix", "spec_review"},
		{"planning", "plan_review"}, {"plan_fix", "plan_review"},
	} {
		tr := edge(e[0], e[1])
		if tr.Name != "submit" {
			t.Fatalf("edge %s->%s name = %q, want submit", e[0], e[1], tr.Name)
		}
		if got := run1(tr); got != proxy {
			t.Fatalf("edge %s->%s gate = %q, want proxy", e[0], e[1], got)
		}
	}

	// impl-review edges gate on EXACT `make check`
	for _, e := range [][2]mtt.StatusName{
		{"implementing", "impl_review"}, {"impl_fix", "impl_review"},
	} {
		tr := edge(e[0], e[1])
		if got := run1(tr); got != "make check" {
			t.Fatalf("edge %s->%s gate = %q, want make check", e[0], e[1], got)
		}
	}

	// done edge: named `approve`, current:clear
	toDone := edge("impl_human_review", "done")
	if toDone.Name != "approve" || toDone.Current != mtt.CurrentClear {
		t.Fatalf("done edge = {name:%q current:%q}, want {approve clear}", toDone.Name, toDone.Current)
	}

	// review forks are named approve/decline (spot-check the design stage)
	if tr := edge("spec_review", "spec_human_review"); tr.Name != "approve" {
		t.Fatalf("spec_review->spec_human_review name = %q, want approve", tr.Name)
	}
	if tr := edge("spec_review", "spec_fix"); tr.Name != "decline" {
		t.Fatalf("spec_review->spec_fix name = %q, want decline", tr.Name)
	}

	// no forward-trap: cancel fires from the three _fix statuses
	for _, from := range []mtt.StatusName{"spec_fix", "plan_fix", "impl_fix"} {
		if tr := edge(from, "cancelled"); tr.Name != "cancel" {
			t.Fatalf("%s->cancelled name = %q, want cancel", from, tr.Name)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails (RED)**

Run: `go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig -v`
Expected: FAIL — `FindRoot: mtt: not initialized (no .mtt directory found)` (no repo `.mtt/` yet).

- [ ] **Step 3: Author the committed config**

Create `.mtt/config.yaml` (at the repo root). **Gate commands are single-quoted; inner `'` doubled as `''`.**

```yaml
version: 1
project:
  name: mtt
command_timeout: 10m
require:
  who: true
types:
  - name: task
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: tbd,               kind: initial}
      - {name: speccing,          kind: active}
      - {name: spec_review,       kind: active}
      - {name: spec_human_review, kind: active}
      - {name: spec_fix,          kind: active}
      - {name: planning,          kind: active}
      - {name: plan_review,       kind: active}
      - {name: plan_human_review, kind: active}
      - {name: plan_fix,          kind: active}
      - {name: implementing,      kind: active}
      - {name: impl_review,       kind: active}
      - {name: impl_human_review, kind: active}
      - {name: impl_fix,          kind: active}
      - {name: done,              kind: terminal}
      - {name: cancelled,         kind: terminal}
    transitions:
      # --- entry ---
      - from: tbd
        to: speccing
        name: start
        current: set
        description: "create/enter the task branch; write the design spec to docs/superpowers/specs/, then `mtt submit`"
        commands: ['git switch -c task/{{.ID}} || git switch task/{{.ID}}']
      # --- design stage ---
      - from: speccing
        to: spec_review
        name: submit
        description: "spec written; run an adversarial subagent spec review"
        commands: ['git status --porcelain | grep -qv ''\.mtt/''']
      - {from: spec_review,       to: spec_human_review, name: approve, description: "agent review passed; get human sign-off on the spec"}
      - {from: spec_review,       to: spec_fix,          name: decline, description: "review found issues; address them, then `mtt submit`"}
      - {from: spec_human_review, to: planning,          name: approve, description: "spec approved; write the implementation plan (writing-plans)"}
      - {from: spec_human_review, to: spec_fix,          name: decline, description: "human declined the spec; address feedback"}
      - from: spec_fix
        to: spec_review
        name: submit
        description: "spec fixes done; re-review"
        commands: ['git status --porcelain | grep -qv ''\.mtt/''']
      # --- plan stage ---
      - from: planning
        to: plan_review
        name: submit
        description: "plan written; run an adversarial subagent plan review"
        commands: ['git status --porcelain | grep -qv ''\.mtt/''']
      - {from: plan_review,       to: plan_human_review, name: approve, description: "agent review passed; get human sign-off on the plan"}
      - {from: plan_review,       to: plan_fix,          name: decline, description: "review found issues; address them, then `mtt submit`"}
      - {from: plan_human_review, to: implementing,      name: approve, description: "plan approved; implement test-first (TDD)"}
      - {from: plan_human_review, to: plan_fix,          name: decline, description: "human declined the plan; address feedback"}
      - from: plan_fix
        to: plan_review
        name: submit
        description: "plan fixes done; re-review"
        commands: ['git status --porcelain | grep -qv ''\.mtt/''']
      # --- implementation stage ---
      - from: implementing
        to: impl_review
        name: submit
        description: "implementation done + make check green; run an adversarial code review"
        commands: ['make check']
      - {from: impl_review,       to: impl_human_review, name: approve, description: "code review passed; get human sign-off"}
      - {from: impl_review,       to: impl_fix,          name: decline, description: "review found issues; address them, then `mtt submit`"}
      - from: impl_human_review
        to: done
        name: approve
        current: clear
        description: "approved; squash-merge the PR into main"
      - {from: impl_human_review, to: impl_fix,          name: decline, description: "human declined; address feedback"}
      - from: impl_fix
        to: impl_review
        name: submit
        description: "impl fixes done; re-review"
        commands: ['make check']
      # --- cancel (from tbd, the do-statuses, and the _fix statuses; no forward-trap) ---
      - {from: tbd,          to: cancelled, name: cancel, current: clear, description: "abandon the task"}
      - {from: speccing,     to: cancelled, name: cancel, current: clear, description: "abandon the task"}
      - {from: planning,     to: cancelled, name: cancel, current: clear, description: "abandon the task"}
      - {from: implementing, to: cancelled, name: cancel, current: clear, description: "abandon the task"}
      - {from: spec_fix,     to: cancelled, name: cancel, current: clear, description: "abandon the task"}
      - {from: plan_fix,     to: cancelled, name: cancel, current: clear, description: "abandon the task"}
      - {from: impl_fix,     to: cancelled, name: cancel, current: clear, description: "abandon the task"}
```

- [ ] **Step 4: Run the guard test to verify it passes (GREEN)**

Run: `go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig -v`
Expected: PASS. If it fails on a gate string, the YAML quoting mangled it — fix per the Global Constraints (single-quote, double the inner `'`).

- [ ] **Step 5: Run the FULL suite (regression check for the new repo `.mtt/`)**

Run: `make check`
Expected: all green. Rationale: committing a repo-root `.mtt/` means `FindRoot` now succeeds from within the repo tree; confirm no existing test relied on "no project found" from a relative path (audit: `root_test.go` uses `t.TempDir()`, so none should — verify).

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/yaml/dogfood_test.go .mtt/config.yaml
git commit -m "feat(s009): committed .mtt/config.yaml (single task, 15-status gated flow) + guard test"
```

---

### Task 2: Mechanism e2e — `dogfood.txt`

**Files:**
- Create: `internal/cli/testdata/scripts/dogfood.txt`

**Interfaces:**
- Consumes: the `testscript` harness ([internal/cli/script_test.go](../../../internal/cli/script_test.go): `TestScripts` runs `testdata/scripts/*.txt`; the `mtt` command is registered in `TestMain`). Pattern reference: [structured_commands.txt](../../../internal/cli/testdata/scripts/structured_commands.txt).
- Produces: nothing (a test).

Notes: the scratch config is a **minimal valid flow** (`initial → active → terminal`) with **fake** commands and **no `require`** (a `require` without an author would exit 2 *before* the gate, passing the "blocks" step for the wrong reason). It exercises the edge-verb sugar (`mtt start`/`mtt submit`) — the first real run of s008.98 on a live flow.

- [ ] **Step 1: Write the e2e script**

Create `internal/cli/testdata/scripts/dogfood.txt`:

```
# s009 dogfood: prove the task-flow mechanism (entry branch + current:set, a
# blocking/passing gate, current:clear) on a scratch config with fake commands.
# The real config's make check / git gates need a real repo (s006/s007/s008
# strategy). Exercises the s008.98 edge-verb sugar (mtt start / mtt submit).
[!exec:git] skip

exec git init

exec mtt init
cp gated.yaml .mtt/config.yaml

# precondition: the config validates before the first move (mtt types validates).
exec mtt types
stdout 'task'

exec mtt add 'demo task'
stdout 'created t1'

# entry edge 'start': creates+enters the branch and sets current.
exec mtt start t1
stdout 't1: tbd → speccing'
exec git symbolic-ref --short HEAD
stdout 'task/t1'
exec mtt use
stdout 't1'

# gate BLOCKS when the fake gate command fails (no 'proceed' file): non-zero,
# task unchanged (still speccing), current still set.
! exec mtt submit t1
exec mtt show t1
stdout 'speccing'
exec mtt use
stdout 't1'

# gate PASSES once the command succeeds: moves to done and clears current.
exec touch proceed
exec mtt submit t1
stdout 't1: speccing → done'
exec mtt use
stdout 'no current task'

-- gated.yaml --
version: 1
project:
  name: dogfood-demo
command_timeout: 5m
types:
  - name: task
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: tbd,      kind: initial}
      - {name: speccing, kind: active}
      - {name: done,     kind: terminal}
    transitions:
      - {from: tbd,      to: speccing, name: start,  current: set,   commands: ['git switch -c task/{{.ID}} || git switch task/{{.ID}}']}
      - {from: speccing, to: done,     name: submit, current: clear, commands: ['test -f proceed']}
```

- [ ] **Step 2: Run the e2e to verify it passes**

Run: `go test ./internal/cli/ -run 'TestScripts/dogfood' -v`
Expected: PASS. If `git switch` is unavailable the whole script is skipped (`[!exec:git] skip`); on CI (modern git) it runs.

- [ ] **Step 3: Commit**

```bash
git add internal/cli/testdata/scripts/dogfood.txt
git commit -m "test(s009): e2e dogfood.txt — task-flow branch/gate/current mechanism"
```

---

### Task 3: Migrate the forward product backlog → `.mtt/tasks/*.yaml`

**Files:**
- Create: `.mtt/tasks/*.yaml` (produced by `./bin/mtt add`, then committed)

**Interfaces:**
- Consumes: the committed `.mtt/config.yaml` (Task 1) via the repo's own `mtt`; `mtt add [title] --priority high|medium|low --tag <t>… --description <d> --depends-on <id>…` (all shipped). The default type is `task` (no `--type` needed).
- Produces: the committed live backlog that `mtt list`/`roadmap`/`ready` render.

Notes: all tasks are created `tbd`; `current` stays unset. **Split:** the **active queue** carries no `backlog` tag (priority-ordered); the **backlog** (further-out chunks + design think-items + self-host meta-tasks) carries `--tag backlog --priority low`. The `add` script must be **dependency-ordered** if any `--depends-on` is used (targets must exist first) — the migration below uses **no artificial deps** (sequencing is by priority), so order is free.

**Title hygiene (verified footgun):** `mtt add` extracts `#hashtag`s from the title/description (s008.7), so a `#<token>` silently becomes a tag — invisible in the `taskLine`/`roadmap` verification. Migration titles must contain **no `#`** (`#2` → a stray `2` tag). Keep each title a **single-quoted single argv** so flag-looking substrings (`--who`/`--no-run`/`;` in the dangerous-ops title) never reach pflag.

- [ ] **Step 1: Build the binary**

Run: `make build`
Expected: `./bin/mtt` exists. (`make build` stamps the version; not required for the migration.)

- [ ] **Step 2: Add the active-queue tasks**

Run (from the repo root, so `mtt` finds the committed `.mtt/`):

```bash
./bin/mtt add 'references — structured, verifiable refs on tasks/comments (note/task/comment/url)' --priority high --tag core
./bin/mtt add 'comments — threaded discussion on tasks' --priority medium --tag core
./bin/mtt add 'actor profiles — named actors / assignees' --priority medium --tag core
./bin/mtt add 'coding-template demo — showcase the coding flow end-to-end' --priority low --tag demo
./bin/mtt add 'dangerous-ops attribution — force --who/--why on flow-bypassing / risk actions (--no-run over a real gate, --force); the parked roles seam, first concrete trigger' --priority high --tag core
```

Expected: each prints `created tN`. (Titles mirror `sessions/README.md` / TASKS rows; adjust wording to the current `sessions/README.md` if it has drifted.)

- [ ] **Step 3: Add the backlog tasks (further-out chunks + think-items + self-host meta)**

Read `TASKS.md` "Later (think)" and the former Phases 5–8; add each as a `backlog` task. The known set (verify/extend against `TASKS.md` at run time):

```bash
# former Phases 5-8 (coarse chunks)
./bin/mtt add 'knowledge base + text search (Phase 5)' --priority low --tag backlog --tag kb
./bin/mtt add 'text/ASCII Gantt + richer query (Phase 6)' --priority low --tag backlog
./bin/mtt add 'mtt-ui — optional web UI (Phase 7)' --priority low --tag backlog --tag ux
./bin/mtt add 'external adapters (Jira/Confluence) + indexer hook (Phase 8)' --priority low --tag backlog --tag adapter
# design think-items (TASKS "Later (think)")
./bin/mtt add 'monotonic id minting / no id-reuse after rm --force (before dogfooding at volume / multi-agent)' --priority low --tag backlog
./bin/mtt add 'lost-update / write-concurrency (optimistic token on Update; before two writing agents share a store)' --priority low --tag backlog
./bin/mtt add 'roles-on-edges (semantic routing; unpark trigger = human-review in the self-host flow)' --priority low --tag backlog
./bin/mtt add 'node-level status actions (named rollback-able pipelines on a status; blocked on arg-resolution grammar)' --priority low --tag backlog
./bin/mtt add 'SEC1 — kill the process group on gate timeout (Setpgid)' --priority low --tag backlog
./bin/mtt add 'roadmap heap — replace per-pop sort (O(N^2 log N)); fix at volume' --priority low --tag backlog
# self-host meta (this session surfaced these)
./bin/mtt add 'migrate agent-process rules into the flow descriptions (self-instructing runbook — flow-note insight 2)' --priority low --tag backlog
./bin/mtt add 'ready/list --exclude-tag filter (de-noise the queue: hide backlog from ready)' --priority low --tag backlog
```

Expected: each prints `created tN`. **Do not invent items** beyond what `TASKS.md` records + this session's two meta-tasks; if `TASKS.md` "Later (think)" lists more, add them the same way (`--tag backlog --priority low`).

- [ ] **Step 4: Hand-run the roadmap and eyeball the order (S3)**

Run: `./bin/mtt roadmap`
Expected: the active queue (high `references`/`dangerous-ops`, then medium `comments`/`actor-profiles`, then low `coding-demo`) sorts **above** the low-priority `backlog` block. Same-priority peers tie by recency (freshest first) — cosmetic; if two peers whose order matters render inverted, adjust their priorities and re-run. Confirm no empty far-future item precedes an actionable one.

- [ ] **Step 5: Confirm the queries render and the backlog filters**

Run:
```bash
./bin/mtt list
./bin/mtt list --tag backlog
./bin/mtt tree
```
Expected: `list` shows all tasks (flat); `list --tag backlog` shows only the backlog block; `tree` shows a flat list (no hierarchy). (`ready` will include backlog — expected, `roadmap` is the "what next" view.)

- [ ] **Step 6: Run make check + commit the task data**

Run: `make check`
Expected: green (task data does not affect any test; `TestRepoDogfoodConfig` guards only the config).

```bash
git add .mtt/tasks
git commit -m "feat(s009): migrate the forward product backlog onto mtt (active queue + backlog)"
```

---

### Task 4: Version bump + docs sync + freeze TASKS.md

**Files:**
- Modify: `internal/cli/root.go:16` (version)
- Modify: `DESIGN.md`, `DESIGN.ru.md` (dogfood note)
- Modify: `AGENTS.md` (new "Working under mtt (self-host)" section)
- Modify: `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md` (brief self-host mention)
- Modify: `TASKS.md` (freeze banner + `e5_t2 ✅`)
- Modify: `sessions/README.md` (009 ✅, 010 ← next)
- Modify: `NEXT_SESSION.md` ("Where we are" + next + carry-over 009)
- Modify: `sessions/009_dogfood.md` (Done section)

**Interfaces:** none (docs + version string). No code logic.

- [ ] **Step 1: Bump the version**

In [internal/cli/root.go:16](../../../internal/cli/root.go#L16): `var version = "0.8.98-dev"` → `var version = "0.9.0-dev"`.

- [ ] **Step 2: DESIGN.md + DESIGN.ru.md — "Dogfooding / self-host" note**

Add a note (English primary, Russian mirror) under the implementation-order / flow section covering: the single `task` type + 15-status gated maturation flow (process-vs-product model note); the gates (`task/{{.ID}}` branch on entry, `git status … | grep -qv '\.mtt/'` proxy on spec/plan submit — artifact uncommitted until human review, `make check` on impl-review); **SEC2** (gates invoke read-only mtt only); **S4** (`.mtt` mutations commit with the task PR); the bootstrap caveats (mtt ids ≠ `sNNN`; no slug in the branch; s009 ran on the manual `feat/s009-dogfood`); attribution = global `require:{who}` (per-edge/roles parked). Keep it concise; the spec holds the full detail.

- [ ] **Step 3: AGENTS.md — "Working under mtt (self-host)" section**

Add a section: the `task` flow + gates; **`task/{{.ID}}` branches** for mtt-driven work (alongside manual `feat/`/`fix/`/`chore/` for non-task branches); the one-time `require:{who}` **author setup** (`config.local.yaml` `author:` or `MTT_BY`/`--by`) — a fresh checkout exits 2 on the first move without it; how to move a task (`mtt <edge> <id>` / `mtt status <id> <status>`) and promote a `backlog` item (`mtt tag rm <id> backlog` + start work); `.mtt` commits with the PR (S4).

- [ ] **Step 4: CLI_REFERENCE.md + .ru — brief self-host mention**

One short paragraph: this repo dogfoods mtt; `mtt roadmap` is the "what next" view; `mtt list --tag backlog` for the backlog. Minimal.

- [ ] **Step 5: Freeze TASKS.md**

Add a banner at the top: **"FROZEN (s009, 2026-07-10). The task plan is superseded by mtt (self-host); the live queue is `mtt roadmap`/`mtt list`. The design backlog migrated as `backlog` tasks. This file is a historical record."** Mark `e5_t2` ✅ (dogfood done).

- [ ] **Step 6: sessions/README.md + NEXT_SESSION.md + sessions/009_dogfood.md**

- `sessions/README.md`: mark 009 ✅ (dogfood/self-host), next = 010 (references).
- `NEXT_SESSION.md`: update "Where we are" (s009 done, version `0.9.0-dev`), "Next task = s010 references (now `t1` in mtt)", and add a "Carry-over lessons (009)" block (the process-vs-product re-model; single-`task` full flow; YAML single-quote gate trap; fail-closed self-ref gates; the guard test is the sole config validation; backlog-as-tagged-tasks; next = s009.5 release positioning → tag v0.9.0).
- `sessions/009_dogfood.md`: fill the "Done" section (what shipped: config, migration, guard test, e2e, docs, version).

- [ ] **Step 7: Run make check**

Run: `make check`
Expected: green (version bump is a plain var; docs don't affect tests). If a test hardcodes the old version, update it (audit: none found).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "docs(s009): version 0.9.0-dev, dogfood docs, AGENTS mtt section, freeze TASKS"
```

---

## Self-Review

**Spec coverage:**
- Goal 1 (committed config, single task, gated flow, require:{who}) → Task 1. ✓
- Goal 2 (migrate forward backlog, active + backlog split) → Task 3. ✓
- Goal 3 (guard test + mechanism e2e) → Task 1 (guard) + Task 2 (e2e). ✓
- Docs sync + freeze + version → Task 4. ✓
- SEC2 / S4 / bootstrap caveats / process-vs-product note → Task 4 Steps 2–3. ✓
- AGENTS.md mtt section → Task 4 Step 3. ✓

**Placeholder scan:** the guard test and both configs are complete code; the migration `add` commands are exact (the backlog set is enumerated + a "verify against TASKS.md" instruction — a deterministic procedure, not a placeholder). No TBD/TODO.

**Type consistency:** status names, edge names, and gate strings are identical across the config (Task 1 Step 3), the guard test (Task 1 Step 1), and the Global Constraints. The e2e (Task 2) deliberately uses a *different, minimal* scratch flow (documented) — it proves the mechanism, not the real config.

**Known judgment points (resolved at execution):** exact migration priorities (Task 3 Step 4 hand-run + eyeball); the full backlog enumeration (Task 3 Step 3, verified against `TASKS.md`).
