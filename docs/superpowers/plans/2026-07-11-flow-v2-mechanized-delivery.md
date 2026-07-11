# Flow v2 (mechanized delivery) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the committed flow v1 with the approved flow v2 (delivery tail, `chore` type, id-keyed artifact gates, branch-context mechanics) plus the matching guard test, e2e, repo setting, and docs — all riding PR #23 on `feat/s009-dogfood`.

**Architecture:** Zero production-code change. The change is DATA (`.mtt/config.yaml`), TESTS (`internal/adapter/yaml/dogfood_test.go`, `internal/cli/testdata/scripts/dogfood.txt`), one GitHub repo setting, and DOCS. Spec (authoritative): `docs/superpowers/specs/2026-07-11-flow-v2-mechanized-delivery-design.md`.

**Tech Stack:** Go 1.23, testscript (txtar), gh CLI, YAML.

## Global Constraints

- `make check` green before EVERY commit (gofmt + vet + golangci-lint + `go test -race` + build).
- Branch `feat/s009-dogfood`; no version change (stays `0.9.0-dev`).
- Gate command scalars in YAML are **single-quoted** (double-quoting breaks `\"` sequences).
- Exact command strings in the guard test must match `.mtt/config.yaml` **byte-for-byte** (the guard exists to catch YAML mangling).
- Agent-facing docs English; `DESIGN.md` ↔ `DESIGN.ru.md` stay in sync paragraph-for-paragraph.
- testscript has no shell pipes at script level (pipes INSIDE gate commands are fine — they run via `sh -c`).
- Commit trailer: `Co-Authored-By:` the acting model, per current session convention.

---

### Task 1: Guard test v2 (red) → config v2 (green)

**Files:**
- Modify: `internal/adapter/yaml/dogfood_test.go` (full rewrite of `TestRepoDogfoodConfig`)
- Modify: `.mtt/config.yaml` (full rewrite to flow v2)

**Interfaces:**
- Consumes: `FindRoot`, `Load` (existing `internal/adapter/yaml` API); `pkg/mtt` types (`StatusName`, `StatusKind`, `Transition`, `CurrentSet/CurrentClear`).
- Produces: the committed flow-v2 config that Tasks 2–6 describe; exact gate-string constants live in the test.

- [ ] **Step 1: Rewrite the guard test (full replacement of the file body)**

```go
package yaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Exact gate strings — byte-for-byte copies of .mtt/config.yaml. A mangled or
// inverted gate must fail HERE, not at runtime.
const (
	cmdEntrySwitch = `git switch task/{{.ID}} || (git switch main && git switch -c task/{{.ID}})`
	cmdEntryGuard  = `test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on this branch — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }`
	cmdSpecGlob    = `ls docs/superpowers/specs/{{.ID}}-*.md`
	cmdPlanGlob    = `ls docs/superpowers/plans/{{.ID}}-*.md`
	cmdMakeCheck   = `make check`
	cmdDeclineBack = `git switch task/{{.ID}}`
	cmdSwitchMain  = `git switch main`
	cmdDeliverLog  = `git log -n 200 --format=%s | grep "^{{.ID}}: " || { echo "no squash commit \"{{.ID}}: …\" on local main: git pull first, and check the PR/merge title started with \"{{.ID}}: \"" >&2; false; }`
	cmdCancelGuard = `test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }`
)

// TestRepoDogfoodConfig is the SOLE guard of this repo's committed
// .mtt/config.yaml (Config.Validate runs on add/types, never on Load). It
// pins the root to THIS repo (go.mod beside .mtt), copies the committed
// config into a temp root, and loads it THERE — the gitignored
// .mtt/config.local.yaml overlay can neither redden nor mask the committed
// artifact — then asserts the full flow-v2 shape for both types.
func TestRepoDogfoodConfig(t *testing.T) {
	root, err := FindRoot(".")
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("found root %q is not this repo (no go.mod): %v", root, err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".mtt", "config.yaml"))
	if err != nil {
		t.Fatalf("read committed config: %v", err)
	}
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".mtt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".mtt", "config.yaml"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, settings, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if len(cfg.Types) != 2 {
		t.Fatalf("types = %d, want 2 (task, chore)", len(cfg.Types))
	}
	task, chore := cfg.Types[0], cfg.Types[1]
	if task.Name != "task" || chore.Name != "chore" {
		t.Fatalf("type names = %q, %q; want task, chore", task.Name, chore.Name)
	}
	if !task.Default || chore.Default {
		t.Fatalf("default flags: task=%v chore=%v, want true/false", task.Default, chore.Default)
	}
	if settings.Prefixes["task"] != "t" || settings.Prefixes["chore"] != "c" {
		t.Fatalf("prefixes = %v, want task:t chore:c", settings.Prefixes)
	}
	if !settings.Require.Who {
		t.Fatalf("require.who must be true")
	}
	if settings.CommandTimeout != 5*time.Minute {
		t.Fatalf("global command_timeout = %v, want the 5m code default (D8)", settings.CommandTimeout)
	}

	taskKinds := map[mtt.StatusName]mtt.StatusKind{
		"tbd": mtt.KindInitial, "speccing": mtt.KindActive,
		"spec_review": mtt.KindActive, "spec_human_review": mtt.KindActive,
		"spec_fix": mtt.KindActive, "planning": mtt.KindActive,
		"plan_review": mtt.KindActive, "plan_human_review": mtt.KindActive,
		"plan_fix": mtt.KindActive, "implementing": mtt.KindActive,
		"impl_review": mtt.KindActive, "impl_fix": mtt.KindActive,
		"approved": mtt.KindActive,
		"done":     mtt.KindTerminal, "cancelled": mtt.KindTerminal,
	}
	choreKinds := map[mtt.StatusName]mtt.StatusKind{
		"tbd": mtt.KindInitial, "implementing": mtt.KindActive,
		"impl_review": mtt.KindActive, "impl_fix": mtt.KindActive,
		"approved": mtt.KindActive,
		"done":     mtt.KindTerminal, "cancelled": mtt.KindTerminal,
	}
	assertKinds(t, task, taskKinds)
	assertKinds(t, chore, choreKinds)

	if got := len(task.Transitions); got != 27 {
		t.Fatalf("task transitions = %d, want 27", got)
	}
	if got := len(chore.Transitions); got != 11 {
		t.Fatalf("chore transitions = %d, want 11", got)
	}

	// entry edges: name/current + exact two-command pipeline (both types).
	for _, tc := range []struct {
		typ mtt.Type
		to  mtt.StatusName
	}{{task, "speccing"}, {chore, "implementing"}} {
		e := edge(t, tc.typ, "tbd", tc.to)
		if e.Name != "start" || e.Current != mtt.CurrentSet {
			t.Fatalf("%s entry = {name:%q current:%q}, want {start set}", tc.typ.Name, e.Name, e.Current)
		}
		assertRuns(t, tc.typ, e, cmdEntrySwitch, cmdEntryGuard)
	}

	// spec/plan submit edges: exact glob gates.
	for _, sp := range [][3]mtt.StatusName{
		{"speccing", "spec_review", ""}, {"spec_fix", "spec_review", ""},
	} {
		assertRuns(t, task, namedEdge(t, task, sp[0], sp[1], "submit"), cmdSpecGlob)
	}
	for _, sp := range [][2]mtt.StatusName{
		{"planning", "plan_review"}, {"plan_fix", "plan_review"},
	} {
		assertRuns(t, task, namedEdge(t, task, sp[0], sp[1], "submit"), cmdPlanGlob)
	}

	// impl submit edges: make check with the 10m per-command timeout (D8), both types.
	for _, tc := range []mtt.Type{task, chore} {
		for _, from := range []mtt.StatusName{"implementing", "impl_fix"} {
			e := namedEdge(t, tc, from, "impl_review", "submit")
			if len(e.Commands) != 1 || e.Commands[0].Run != cmdMakeCheck {
				t.Fatalf("%s %s->impl_review = %+v, want single %q", tc.Name, from, e.Commands, cmdMakeCheck)
			}
			if e.Commands[0].Timeout != 10*time.Minute {
				t.Fatalf("%s %s->impl_review timeout = %v, want 10m", tc.Name, from, e.Commands[0].Timeout)
			}
		}
	}

	// delivery tail (both types): approve, decline-back-to-branch, deliver.
	for _, tc := range []mtt.Type{task, chore} {
		if e := namedEdge(t, tc, "impl_review", "approved", "approve"); len(e.Commands) != 0 {
			t.Fatalf("%s impl_review->approved must carry no commands", tc.Name)
		}
		assertRuns(t, tc, namedEdge(t, tc, "approved", "impl_fix", "decline"), cmdDeclineBack)
		d := namedEdge(t, tc, "approved", "done", "deliver")
		if d.Current != mtt.CurrentClear {
			t.Fatalf("%s deliver current = %q, want clear", tc.Name, d.Current)
		}
		assertRuns(t, tc, d, cmdSwitchMain, cmdDeliverLog)
	}

	// full cancel matrix (no forward-trap): every non-terminal except _review pairs.
	taskCancels := []mtt.StatusName{"tbd", "speccing", "planning", "implementing", "spec_fix", "plan_fix", "impl_fix", "approved"}
	choreCancels := []mtt.StatusName{"tbd", "implementing", "impl_fix", "approved"}
	assertCancels(t, task, taskCancels)
	assertCancels(t, chore, choreCancels)

	// descriptions are load-bearing (self-instructing runbook): all present,
	// plus the two key instruction strings.
	for _, tc := range []mtt.Type{task, chore} {
		for _, s := range tc.Statuses {
			if strings.TrimSpace(s.Description) == "" {
				t.Fatalf("%s status %q has no description", tc.Name, s.Name)
			}
		}
		for _, tr := range tc.Transitions {
			if strings.TrimSpace(tr.Description) == "" {
				t.Fatalf("%s edge %s->%s has no description", tc.Name, tr.From, tr.To)
			}
		}
	}
	if s, _ := chore.StatusByName("impl_review"); !strings.Contains(s.Description, "it must be a") {
		t.Fatalf("chore impl_review description lost the type-boundary police line: %q", s.Description)
	}
	if d := namedEdge(t, task, "approved", "done", "deliver"); !strings.Contains(d.Description, "pull main") {
		t.Fatalf("task deliver description lost the pull-main hint: %q", d.Description)
	}
}

func assertKinds(t *testing.T, typ mtt.Type, want map[mtt.StatusName]mtt.StatusKind) {
	t.Helper()
	if len(typ.Statuses) != len(want) {
		t.Fatalf("%s statuses = %d, want %d", typ.Name, len(typ.Statuses), len(want))
	}
	for name, kind := range want {
		s, ok := typ.StatusByName(name)
		if !ok {
			t.Fatalf("%s status %q missing", typ.Name, name)
		}
		if s.Kind != kind {
			t.Fatalf("%s status %q kind = %q, want %q", typ.Name, name, s.Kind, kind)
		}
	}
}

func edge(t *testing.T, typ mtt.Type, from, to mtt.StatusName) mtt.Transition {
	t.Helper()
	tr, ok := typ.FindTransition(from, to)
	if !ok {
		t.Fatalf("%s edge %s -> %s missing", typ.Name, from, to)
	}
	return tr
}

func namedEdge(t *testing.T, typ mtt.Type, from, to mtt.StatusName, name string) mtt.Transition {
	t.Helper()
	tr := edge(t, typ, from, to)
	if tr.Name != name {
		t.Fatalf("%s edge %s->%s name = %q, want %q", typ.Name, from, to, tr.Name, name)
	}
	return tr
}

func assertRuns(t *testing.T, typ mtt.Type, tr mtt.Transition, want ...string) {
	t.Helper()
	if len(tr.Commands) != len(want) {
		t.Fatalf("%s edge %s->%s: %d commands, want %d", typ.Name, tr.From, tr.To, len(tr.Commands), len(want))
	}
	for i, w := range want {
		if tr.Commands[i].Run != w {
			t.Fatalf("%s edge %s->%s cmd[%d] = %q, want %q", typ.Name, tr.From, tr.To, i, tr.Commands[i].Run, w)
		}
	}
}

func assertCancels(t *testing.T, typ mtt.Type, froms []mtt.StatusName) {
	t.Helper()
	for _, from := range froms {
		e := namedEdge(t, typ, from, "cancelled", "cancel")
		if e.Current != mtt.CurrentClear {
			t.Fatalf("%s cancel from %s: current = %q, want clear", typ.Name, from, e.Current)
		}
		assertRuns(t, typ, e, cmdSwitchMain, cmdCancelGuard)
	}
}
```

Note: `mtt.Transition.Name` is a `string` per s008.98 — if it is a named type, cast the comparisons accordingly (check `pkg/mtt/config.go`). `Settings.CommandTimeout` and `Settings.Prefixes` exist per `load.go` — but check WHERE the 5m default is applied: if `Load` leaves `CommandTimeout` zero when the `command_timeout:` key is absent (default applied at the runner/CLI instead), change the assert to expect the actual `Load` behavior for an absent key (the intent of the assert: the config must NOT carry its own global override anymore).

- [ ] **Step 2: Run to verify it FAILS against the v1 config**

Run: `go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig -v`
Expected: FAIL with `types = 1, want 2 (task, chore)`

- [ ] **Step 3: Write `.mtt/config.yaml` v2 (full replacement)**

```yaml
version: 1
project:
  name: mtt
require:
  who: true
types:
  - name: task
    prefix: t
    parents: []
    default: true
    description: "A unit of product change whose design is OPEN — decisions to make and record (spec + plan artifacts, each reviewed). If the what+how are already decided and recorded elsewhere, use `chore`."
    statuses:
      - {name: tbd,               kind: initial,  description: "queued; `mtt start` to take into work"}
      - {name: speccing,          kind: active,   description: "write the design spec to docs/superpowers/specs/<this-task-id>-<slug>.md (commit early and often), then `mtt submit`"}
      - {name: spec_review,       kind: active,   description: "run an adversarial subagent review of the spec; `mtt approve` when it passes, `mtt decline` to send back"}
      - {name: spec_human_review, kind: active,   description: "get human sign-off on the spec; `mtt approve` or `mtt decline`"}
      - {name: spec_fix,          kind: active,   description: "address spec findings, then `mtt submit` to re-review"}
      - {name: planning,          kind: active,   description: "write the implementation plan to docs/superpowers/plans/<this-task-id>-<slug>.md, then `mtt submit`"}
      - {name: plan_review,       kind: active,   description: "run an adversarial subagent review of the plan; `mtt approve` when it passes, `mtt decline` to send back"}
      - {name: plan_human_review, kind: active,   description: "get human sign-off on the plan; `mtt approve` or `mtt decline`"}
      - {name: plan_fix,          kind: active,   description: "address plan findings, then `mtt submit` to re-review"}
      - {name: implementing,      kind: active,   description: "implement test-first (TDD), make check green between commits, then `mtt submit`"}
      - {name: impl_review,       kind: active,   description: "run an adversarial code review; `mtt approve` when it passes, `mtt decline` to send back"}
      - {name: impl_fix,          kind: active,   description: "address review findings, then `mtt submit` to re-review"}
      - {name: approved,          kind: active,   description: "open/update the PR titled '<this-task-id>: <title>', push (including this state change), ask the human to merge; after the squash-merge run `mtt deliver`; human-requested changes -> `mtt decline`"}
      - {name: done,              kind: terminal, description: "delivered to main"}
      - {name: cancelled,         kind: terminal, description: "abandoned"}
    transitions:
      # --- entry ---
      - from: tbd
        to: speccing
        name: start
        current: set
        description: "enter (or create from main) the task branch task/<id>, then write the spec"
        commands:
          - 'git switch task/{{.ID}} || (git switch main && git switch -c task/{{.ID}})'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on this branch — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
      # --- design stage ---
      - from: speccing
        to: spec_review
        name: submit
        description: "spec written; run an adversarial subagent spec review"
        commands: ['ls docs/superpowers/specs/{{.ID}}-*.md']
      - {from: spec_review,       to: spec_human_review, name: approve, description: "agent review passed; get human sign-off on the spec"}
      - {from: spec_review,       to: spec_fix,          name: decline, description: "review found issues; address them, then `mtt submit`"}
      - {from: spec_human_review, to: planning,          name: approve, description: "spec approved; write the implementation plan"}
      - {from: spec_human_review, to: spec_fix,          name: decline, description: "human declined the spec; address feedback"}
      - from: spec_fix
        to: spec_review
        name: submit
        description: "spec fixes done; re-review"
        commands: ['ls docs/superpowers/specs/{{.ID}}-*.md']
      # --- plan stage ---
      - from: planning
        to: plan_review
        name: submit
        description: "plan written; run an adversarial subagent plan review"
        commands: ['ls docs/superpowers/plans/{{.ID}}-*.md']
      - {from: plan_review,       to: plan_human_review, name: approve, description: "agent review passed; get human sign-off on the plan"}
      - {from: plan_review,       to: plan_fix,          name: decline, description: "review found issues; address them, then `mtt submit`"}
      - {from: plan_human_review, to: implementing,      name: approve, description: "plan approved; implement test-first (TDD)"}
      - {from: plan_human_review, to: plan_fix,          name: decline, description: "human declined the plan; address feedback"}
      - from: plan_fix
        to: plan_review
        name: submit
        description: "plan fixes done; re-review"
        commands: ['ls docs/superpowers/plans/{{.ID}}-*.md']
      # --- implementation stage ---
      - from: implementing
        to: impl_review
        name: submit
        description: "implementation done; run an adversarial code review"
        commands: [{run: 'make check', timeout: 10m}]
      - {from: impl_review, to: approved, name: approve, description: "code review passed; open the PR and hand to the human"}
      - {from: impl_review, to: impl_fix, name: decline, description: "review found issues; address them, then `mtt submit`"}
      - from: impl_fix
        to: impl_review
        name: submit
        description: "impl fixes done; re-review"
        commands: [{run: 'make check', timeout: 10m}]
      # --- delivery tail ---
      - from: approved
        to: impl_fix
        name: decline
        description: "human requested changes on the PR (returns you to the task branch)"
        commands: ['git switch task/{{.ID}}']
      - from: approved
        to: done
        name: deliver
        current: clear
        description: "after the squash-merge: pull main, then deliver (moves your tree to main and writes done there — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'git log -n 200 --format=%s | grep "^{{.ID}}: " || { echo "no squash commit \"{{.ID}}: …\" on local main: git pull first, and check the PR/merge title started with \"{{.ID}}: \"" >&2; false; }'
      # --- cancel (every non-terminal except the _review pairs; moves your tree to main) ---
      - from: tbd
        to: cancelled
        name: cancel
        current: clear
        description: "abandon the task (moves your tree to main — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
      - from: speccing
        to: cancelled
        name: cancel
        current: clear
        description: "abandon the task (moves your tree to main — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
      - from: planning
        to: cancelled
        name: cancel
        current: clear
        description: "abandon the task (moves your tree to main — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
      - from: implementing
        to: cancelled
        name: cancel
        current: clear
        description: "abandon the task (moves your tree to main — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
      - from: spec_fix
        to: cancelled
        name: cancel
        current: clear
        description: "abandon the task (moves your tree to main — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
      - from: plan_fix
        to: cancelled
        name: cancel
        current: clear
        description: "abandon the task (moves your tree to main — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
      - from: impl_fix
        to: cancelled
        name: cancel
        current: clear
        description: "abandon the task (moves your tree to main — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
      - from: approved
        to: cancelled
        name: cancel
        current: clear
        description: "abandon the task (moves your tree to main — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
  - name: chore
    prefix: c
    parents: []
    description: "A unit of product change whose design is ALREADY FIXED elsewhere (a review finding with a fix sketch, a recorded backlog decision, docs sync, a dependency bump, a mechanical refactor). If you need to make design decisions, use `task`."
    statuses:
      - {name: tbd,          kind: initial,  description: "queued; `mtt start` to take into work"}
      - {name: implementing, kind: active,   description: "implement test-first (TDD), make check green between commits, then `mtt submit`"}
      - {name: impl_review,  kind: active,   description: "run an adversarial code review; if the diff contains design decisions not recorded elsewhere — decline: it must be a `task`. `mtt approve` / `mtt decline`"}
      - {name: impl_fix,     kind: active,   description: "address review findings, then `mtt submit` to re-review"}
      - {name: approved,     kind: active,   description: "open/update the PR titled '<this-task-id>: <title>', push (including this state change), ask the human to merge; after the squash-merge run `mtt deliver`; human-requested changes -> `mtt decline`"}
      - {name: done,         kind: terminal, description: "delivered to main"}
      - {name: cancelled,    kind: terminal, description: "abandoned"}
    transitions:
      - from: tbd
        to: implementing
        name: start
        current: set
        description: "enter (or create from main) the task branch task/<id>, then implement"
        commands:
          - 'git switch task/{{.ID}} || (git switch main && git switch -c task/{{.ID}})'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on this branch — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
      - from: implementing
        to: impl_review
        name: submit
        description: "implementation done; run an adversarial code review"
        commands: [{run: 'make check', timeout: 10m}]
      - {from: impl_review, to: approved, name: approve, description: "code review passed; open the PR and hand to the human"}
      - {from: impl_review, to: impl_fix, name: decline, description: "review found issues; address them, then `mtt submit`"}
      - from: impl_fix
        to: impl_review
        name: submit
        description: "impl fixes done; re-review"
        commands: [{run: 'make check', timeout: 10m}]
      - from: approved
        to: impl_fix
        name: decline
        description: "human requested changes on the PR (returns you to the task branch)"
        commands: ['git switch task/{{.ID}}']
      - from: approved
        to: done
        name: deliver
        current: clear
        description: "after the squash-merge: pull main, then deliver (moves your tree to main and writes done there — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'git log -n 200 --format=%s | grep "^{{.ID}}: " || { echo "no squash commit \"{{.ID}}: …\" on local main: git pull first, and check the PR/merge title started with \"{{.ID}}: \"" >&2; false; }'
      - from: tbd
        to: cancelled
        name: cancel
        current: clear
        description: "abandon the task (moves your tree to main — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
      - from: implementing
        to: cancelled
        name: cancel
        current: clear
        description: "abandon the task (moves your tree to main — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
      - from: impl_fix
        to: cancelled
        name: cancel
        current: clear
        description: "abandon the task (moves your tree to main — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
      - from: approved
        to: cancelled
        name: cancel
        current: clear
        description: "abandon the task (moves your tree to main — commit .mtt after)"
        commands:
          - 'git switch main'
          - 'test -f .mtt/tasks/{{.ID}}.yaml || { echo "task file absent on main — its add has not reached main (merge/commit the branch that created it first)" >&2; false; }'
```

Note the global `command_timeout: 10m` line from v1 is GONE (D8: 5m code default; `make check` carries its own 10m).

- [ ] **Step 4: Run the guard to verify it PASSES**

Run: `go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig -v`
Expected: PASS

- [ ] **Step 5: Sanity-render both flows**

Run: `make build && ./bin/mtt types | head -30`
Expected: `task` block with 15 statuses and `[start]`/`[submit]`/`[deliver]` edge names; then `chore` block. No error.

- [ ] **Step 6: Full gate + commit**

Run: `make check`
Expected: `OK: make check passed`

```bash
git add .mtt/config.yaml internal/adapter/yaml/dogfood_test.go
git commit -m "feat(s009): flow v2 — delivery tail, chore type, id-keyed artifact gates + guard v2"
```

---

### Task 2: e2e `dogfood.txt` v2

**Files:**
- Modify: `internal/cli/testdata/scripts/dogfood.txt` (full rewrite)

**Interfaces:**
- Consumes: the built `mtt` binary behavior (start/submit/deliver edge verbs, `require`, current pointer); testscript harness (`go test ./internal/cli -run TestScripts/dogfood`).
- Produces: nothing downstream; this is the acceptance e2e.

- [ ] **Step 1: Replace the script with the v2 version**

```txt
# s009 flow-v2 e2e: prove the mechanism on a scratch config with fake commands —
# entry (re-enter-or-create-from-main, current:set), a blocking/passing gate
# (with require:{who} active — the real config's per-move policy), and the
# delivery tail (deliver moves the tree to main, verifies the merge trace,
# clears current). Real make check / squash gates need a real repo (s006-s008
# e2e strategy). Exercises the s008.98 edge-verb sugar.
[!exec:git] skip

# v2 entry needs a BORN main + git identity (M1): -b main + an initial commit.
exec git init -b main
exec git config user.email test@example.com
exec git config user.name tester
exec git commit --allow-empty -m init

env MTT_BY=tester

exec mtt init
cp gated.yaml .mtt/config.yaml

# precondition: the config validates (mtt types runs Validate).
exec mtt types
stdout 'task'

exec mtt add 'demo task'
stdout '"id": "t1"'

# attribution is enforced per-move: without MTT_BY the move exits BEFORE the
# gate with the attribution error (m2: pin the cause, not just non-zero).
env MTT_BY=
! exec mtt start t1
stderr 'missing required attribution'
env MTT_BY=tester

# entry edge 'start' (create-from-main path): creates+enters task/t1, sets current.
exec mtt start t1
stdout 't1: tbd → speccing'
exec git symbolic-ref --short HEAD
stdout 'task/t1'
exec mtt use
stdout 't1'

# gate BLOCKS (no 'proceed' file): cause pinned to the gate (not attribution),
# status unchanged (header-anchored — the history line would also match a bare
# regexp), NO new history entry, current still set.
! exec mtt submit t1
stderr 'transition blocked by a failed gate'
exec mtt show t1
stdout '\[speccing\]'
! stdout 'speccing → approved'
exec mtt use
stdout 't1'

# gate PASSES: speccing → approved.
exec touch proceed
exec mtt submit t1
stdout 't1: speccing → approved'

# deliver BLOCKS while the merge trace is absent (fake git-log stand-in file),
# but the tree HAS moved to main (documented side effect m4).
! exec mtt deliver t1
stderr 'no merge trace'
exec git symbolic-ref --short HEAD
stdout 'main'
exec mtt show t1
stdout '\[approved\]'

# the squash lands (fake trace); deliver moves to done on main, clears current.
exec git switch task/t1
cp trace.log merge.log
exec mtt deliver t1
stdout 't1: approved → done'
exec git symbolic-ref --short HEAD
stdout 'main'
exec mtt use
stdout 'no current task'

# entry edge re-enter path (second alternative): a pre-existing branch is
# re-entered, not re-created.
exec mtt add 'second demo'
stdout '"id": "t2"'
exec git switch -c task/t2
exec git switch main
exec mtt start t2
stdout 't2: tbd → speccing'
exec git symbolic-ref --short HEAD
stdout 'task/t2'

-- gated.yaml --
version: 1
project:
  name: dogfood-demo
require:
  who: true
types:
  - name: task
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: tbd,      kind: initial,  description: "queued"}
      - {name: speccing, kind: active,   description: "work, then submit"}
      - {name: approved, kind: active,   description: "awaiting merge"}
      - {name: done,     kind: terminal, description: "delivered"}
      - {name: cancelled, kind: terminal, description: "abandoned"}
    transitions:
      - from: tbd
        to: speccing
        name: start
        current: set
        description: "enter or create the task branch"
        commands: ['git switch task/{{.ID}} || (git switch main && git switch -c task/{{.ID}})']
      - from: speccing
        to: approved
        name: submit
        description: "fake artifact gate"
        commands: ['test -f proceed']
      - from: approved
        to: done
        name: deliver
        current: clear
        description: "fake delivery: switch main + merge trace"
        commands:
          - 'git switch main'
          - 'grep "^{{.ID}}: " merge.log || { echo "no merge trace" >&2; false; }'
      - {from: tbd,      to: cancelled, name: cancel, current: clear, description: "abandon"}
      - {from: speccing, to: cancelled, name: cancel, current: clear, description: "abandon"}
-- trace.log --
t1: demo task (#1)
```

Notes for the implementer:
- `mtt add` under `--json`? No — `add` prints JSON only with `--json`; the plain form prints `created t1`. If the plain form is used, replace the two `stdout '"id": "t1"'` asserts with `stdout 'created t1'` / `stdout 'created t2'` — check the actual output of `mtt add` first (s008.97 changed `--json` only).
- The scratch entry edge intentionally omits the `test -f .mtt/tasks/...` guard (the M2a case needs a second branch to stage — covered by the guard-test string assert instead; the e2e proves the switch mechanics).
- `cp trace.log merge.log` happens while on `task/t1` (we `git switch task/t1` first) so the deliver's own `git switch main` is exercised from the branch posture; untracked files travel.

- [ ] **Step 2: Run the script**

Run: `go test ./internal/cli -run 'TestScripts/dogfood' -v`
Expected: PASS. If `stdout 'created t1'` vs JSON mismatch fires, fix per the note above and re-run.

- [ ] **Step 3: Full gate + commit**

Run: `make check`
Expected: `OK: make check passed`

```bash
git add internal/cli/testdata/scripts/dogfood.txt
git commit -m "test(s009): dogfood e2e v2 — born-main entry, require active, blocked-cause pinned, delivery tail"
```

---

### Task 3: Repo squash-title setting (D3 prerequisite, review B2)

**Files:** none (GitHub repo setting).

- [ ] **Step 1: Flip the setting**

Run: `gh api -X PATCH repos/pashukhin/mtt -f squash_merge_commit_title=PR_TITLE -f squash_merge_commit_message=COMMIT_MESSAGES`
Expected: JSON response echoing the repo; no error.

- [ ] **Step 2: Verify**

Run: `gh api repos/pashukhin/mtt --jq '.squash_merge_commit_title'`
Expected: `PR_TITLE`

---

### Task 4: AGENTS.md + yaml CLAUDE.md (docs pass 1)

**Files:**
- Modify: `AGENTS.md` (stale preamble at ~lines 122-124; "Working under mtt" section; the "don't hand-edit" storage invariant at ~line 80)
- Modify: `internal/adapter/yaml/CLAUDE.md` (config-authoring invariant; guard-test mention)

- [ ] **Step 1: Fix the stale preamble.** In AGENTS.md, replace the sentence
`The roadmap and current target live in [sessions/README.md](sessions/README.md); the design backlog stays in [DESIGN.md](DESIGN.md) / [TASKS.md](TASKS.md). After phase 4 the backlog itself moves onto mtt (dogfooding).`
with:
`The roadmap and current target live in mtt itself (see "Working under mtt" below); sessions/README.md keeps the narrative history. TASKS.md is frozen history since s009.`

- [ ] **Step 2: Scope the hand-edit rule.** In AGENTS.md "Storage invariants", replace
`In the YAML adapter, .mtt/ is committed and is the source of truth; don't hand-edit files.`
with:
`In the YAML adapter, .mtt/ is committed and is the source of truth. Task files are written only by mtt — don't hand-edit them. The repo's .mtt/config.yaml is the exception: it is hand-authored, reviewed like code (see "Working under mtt"), and guarded by TestRepoDogfoodConfig.`

- [ ] **Step 3: Replace the "Working under mtt (self-host)" section body** with:

```markdown
## Working under mtt (self-host)

Since **s009** this repo tracks its own work in a committed `.mtt/` (config + tasks). `TASKS.md` is frozen;
the live queue is mtt. Practical rules:

- **The backlog is in mtt.** `mtt roadmap` is the "what next?" view; `mtt list --tag backlog` is the
  backlog-only view; promote by `mtt tag rm <id> backlog`. A task is the unit of **product** change;
  sessions/phases (how *we* work) stay in `sessions/*.md` — they are not mtt tasks.
- **Two types (`mtt types` shows both flows).** `task` = design is OPEN (spec → plan → implement, each stage
  reviewed by an agent, spec/plan also by the human). `chore` = design is ALREADY FIXED elsewhere (a review
  finding, a recorded decision, docs sync) — implement → review → deliver. If a chore's diff turns out to
  contain design decisions, the reviewer declines it: cancel and recreate as a task.
- **Move by edge verb** (`mtt start/submit/approve/decline/deliver/cancel [<id>]`) or `mtt status`. The flow
  mechanizes the git context: `start` re-enters or creates `task/<id>` from main; `deliver` and `cancel`
  move your tree to main and write the terminal state there; `approved → decline` returns you to the task
  branch. Mid-flight resumption is a plain `git switch task/<id>` (start only fires from tbd).
- **Artifacts are id-keyed and committed early.** A task's spec/plan live at
  `docs/superpowers/specs|plans/<id>-<slug>.md` (the submit gates check exactly that); commit them as you
  go — nothing requires an uncommitted tree.
- **Delivery is verified.** The PR title starts with `<id>: ` (the repo squash setting propagates it to the
  squash subject); `mtt deliver` checks that trace on local main — pull first. Push the `approved` state
  commit before asking for the merge, or deliver will find a stale status.
- **Attribution is required** (`require: {who}`, every move, `--no-run` does not bypass): set `author:` in
  `.mtt/config.local.yaml` or `MTT_BY=<you>` before your first move.
- **Two manual steps remain** (until post-persist actions land): after `deliver` and after `cancel`, run
  `git add .mtt && git commit` on main — the state write is otherwise uncommitted and would ride the next
  task's branch.
- **Config is code (SEC2).** Review `.mtt/config.yaml` diffs like a Makefile; a gate may invoke read-only
  `mtt` only (never an mtt transition). Gate commands are single-quoted YAML scalars. The committed config is
  guarded by `TestRepoDogfoodConfig` — keep it green.
```

- [ ] **Step 4: yaml CLAUDE.md.** In `internal/adapter/yaml/CLAUDE.md` Boundaries, replace
`.mtt/config.yaml is edited only through this adapter (determinism + validation).`
with:
`A project's .mtt/config.yaml is normally produced by Init templates; this repo's own committed config is hand-authored, reviewed like code, and guarded by TestRepoDogfoodConfig (dogfood_test.go — a deliberately non-hermetic test: it pins the repo root via go.mod and loads a temp COPY of the committed config, bypassing the config.local overlay). Task files are written only through the adapter.`

- [ ] **Step 5: Gate + commit**

Run: `make check` → `OK`.

```bash
git add AGENTS.md internal/adapter/yaml/CLAUDE.md
git commit -m "docs(s009): AGENTS working-under-mtt v2 + yaml CLAUDE.md config-authoring invariant"
```

---

### Task 5: DESIGN.md + DESIGN.ru.md flow-v2 note (docs pass 2)

**Files:**
- Modify: `DESIGN.md` (the s009 "Dogfooding / self-host" note — replace its flow/conventions paragraphs)
- Modify: `DESIGN.ru.md` (mirror, paragraph-for-paragraph)

- [ ] **Step 1: Locate the dogfood note** (`grep -n 'dogfood\|self-host' DESIGN.md`) and replace its flow-v1 description with:

```markdown
> **Shipped (s009, revised by the flow-v2 spec):** the repo self-hosts on a committed `.mtt/` with TWO types.
> `task` (design OPEN): `tbd → speccing → spec_review → spec_human_review ⇄ spec_fix → planning →
> plan_review → plan_human_review ⇄ plan_fix → implementing → impl_review ⇄ impl_fix → approved → done`
> (+`cancelled`); `chore` (design ALREADY FIXED elsewhere): the impl stage + delivery tail only. Gates check
> **form, never content** (content = the review statuses): id-keyed artifact presence
> (`ls docs/superpowers/specs/<id>-*.md`) on spec/plan submits, `make check` (per-command 10m timeout) on
> impl submits, and a **verified delivery**: `deliver` moves the tree to main and greps the squash subject
> (`<id>: …` — the PR title, propagated by the repo's `squash_merge_commit_title=PR_TITLE` setting) from
> local `git log`, so **`done` truthfully means "in main"**. Branch context is mechanized: `start` re-enters
> or creates `task/<id>` from main (guarded: the task file must exist on the tree); `cancel` and `deliver`
> write their terminal state ON main; `approved → decline` returns to the task branch. Conventions that
> remain: PR title starts with `<id>: `; push the `approved` state before the merge; two interim manual
> state-commits (after deliver/cancel) until post-persist actions land. Known limits (recorded): the human's
> impl-stage act leaves no mtt history entry (the audit is git's merge trace — spec/plan human sign-offs keep
> theirs); a mid-flight blocked task has no parking status (it rests where it is); cross-branch stale
> reads are the documented lost-update class (multi-agent cluster). Spec:
> `docs/superpowers/specs/2026-07-11-flow-v2-mechanized-delivery-design.md`.
```

- [ ] **Step 2: Mirror the same paragraph in Russian in DESIGN.ru.md** at the corresponding location (translate faithfully; keep command strings, status names, and file paths verbatim).

- [ ] **Step 3: Gate + commit**

```bash
make check
git add DESIGN.md DESIGN.ru.md
git commit -m "docs(s009): DESIGN flow-v2 note (delivery verified, two types, known limits) + ru mirror"
```

---

### Task 6: Session records + NEXT_SESSION handoff

**Files:**
- Modify: `sessions/009_dogfood.md` (status header; Done corrections; flow-v2 addendum)
- Modify: `NEXT_SESSION.md` ("Where we are" tail + carry-over + post-merge checklist)

- [ ] **Step 1: sessions/009_dogfood.md.** (a) Header: `Status: planned` → `Status: done`. (b) In the Done
section, DELETE the false claim `CLI_REFERENCE minimal mention;` (CLI_REFERENCE was not touched — review
D3b). (c) Append to Done:

```markdown
- **Flow v2 (same PR, post-review):** the adversarial branch review (10 confirmed findings) triggered a
  redesign — delivery tail (`approved → deliver → done` verified against the squash trace on main; `done` =
  "in main"), second type `chore` (design-closed work), id-keyed artifact gates replacing the porcelain proxy
  (+ commit-early convention), mechanized branch context (start from main / cancel + deliver write on main /
  decline returns to branch), per-command 10m timeout on `make check`, guard test v2 (overlay-proof, full
  cancel matrix + edge counts + description guards), e2e v2 (born-main entry, require active, block causes
  pinned, delivery tail). Spec: `docs/superpowers/specs/2026-07-11-flow-v2-mechanized-delivery-design.md`;
  plan: `docs/superpowers/plans/2026-07-11-flow-v2-mechanized-delivery.md`.
```

- [ ] **Step 2: NEXT_SESSION.md.** Update the "Where we are" s009 sentence to mention flow v2 (one line:
`s009 shipped self-host with flow v2 (delivery tail, chore type, verified done) — see the flow-v2 spec`).
Then REPLACE the current "Next task" block's post-s009 content with:

```markdown
## Next task — post-merge follow-ups, then the mtt queue

> **First actions after PR #23 squash-merges** (each is an `mtt add` on main; commit `.mtt` after the batch):
> 1. `mtt add 'post-persist actions — after: commands on transitions (run AFTER the task-file write; retires
>    the two manual state-commits and S4)' --priority high` — type `task` (design open: ordering, failure
>    semantics, rollback interplay).
> 2. `mtt add 'team semantics of the YAML store — state visibility across branches (state-branch / auto-push
>    / claim mechanics)' --tag backlog --priority low`
> 3. Migration completeness (review finding #7): `mtt add` the orphaned items — `epics/hierarchy return (+
>    re-parenting, corrected self-ref phase gate)`, `advance + modes + cross-edge compensation (parked —
>    unpark trigger recorded)`, `cancelled-blocker semantics revisit`, `durable edit audit`, `boards/views`
>    — all `--tag backlog --priority low`; widen t18's title with the dropped sub-items (resolve current for
>    all single-task ops; multi-assignee providers) via `mtt edit`.
> 4. The review-fix chores that did NOT ride PR #23, as type `chore`: session-record hygiene elsewhere, the
>    dead-assert cleanups listed in the 2026-07-10 review (see the flow-v2 spec's findings table).
> Then work the queue: `mtt roadmap` (t1 references is next), driving each item through flow v2 — the first
> live run of `start → … → deliver`.
```

### Carry-over lessons (s009 flow v2)
Add a short carry-over block (following the file's existing pattern) with these five lessons:
- GitHub `squash_merge_commit_title=COMMIT_OR_PR_TITLE` takes the SUBJECT FROM THE COMMIT on single-commit
  PRs — any convention keyed to "PR title reaches the squash subject" requires the `PR_TITLE` repo setting.
- Commands run PRE-write: an edge that must land its state write elsewhere does it by switching the tree
  BEFORE the write (`git switch main` on deliver/cancel), and a guard `test -f` after the switch converts a
  would-be not-found-after-side-effects into a diagnosed block.
- `git switch -c` in testscript needs a BORN branch + identity: `git init -b main`, `git config user.*`, one
  `--allow-empty` commit.
- A blocked-move e2e must pin the CAUSE (stderr text): with `require` active, `! exec` alone cannot
  distinguish an attribution exit-2 from a gate exit-3.
- Descriptions are load-bearing (self-instructing runbook): guard-test them like commands.

- [ ] **Step 3: Gate + commit**

```bash
make check
git add sessions/009_dogfood.md NEXT_SESSION.md
git commit -m "docs(s009): session record fixed (status, CLI_REFERENCE claim), flow-v2 addendum, post-merge checklist + carry-over"
```

---

### Task 7: Final verification + push

- [ ] **Step 1:** `make check` → `OK: make check passed`.
- [ ] **Step 2:** Re-run the two suites explicitly:
`go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig -v` → PASS;
`go test ./internal/cli -run 'TestScripts/dogfood' -v` → PASS.
- [ ] **Step 3:** `./bin/mtt roadmap | head -5` — unchanged queue (20 tbd tasks; the config change must not
affect them).
- [ ] **Step 4:** Push and update the PR:

```bash
git push origin feat/s009-dogfood
gh pr comment 23 --body "Flow v2 landed per the approved spec (docs/superpowers/specs/2026-07-11-flow-v2-mechanized-delivery-design.md): delivery tail (done = in main, squash-trace verified), chore type, id-keyed artifact gates, mechanized branch context, guard/e2e v2, docs pass. Repo squash setting flipped to PR_TITLE."
```

- [ ] **Step 5:** Report: CI status (`gh pr checks 23`), and hand the merge decision to the human.
