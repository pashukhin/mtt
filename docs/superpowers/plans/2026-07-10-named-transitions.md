# Named transitions + edge-verb sugar (s008.98) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let an agent move a task by naming the **edge** out of its current status (`mtt decline`, `mtt do decline t1`), symmetric to the existing move-by-target-status (`mtt fix`, `mtt status t1 fix`).

**Architecture:** Additive optional `Transition.Name`; a pure `Type.FindTransitionByName(from, name)`; three new `validateFlow` invariants that keep the two namespaces disjoint and `(from,to)` unique. The CLI resolves an edge name to its target status `to` and rides the **existing** `runTransition(to)` — `core.Transitioner` and the gate path are untouched.

**Tech Stack:** Go 1.23, cobra, `gopkg.in/yaml.v3`, `go-internal/testscript` (e2e). Storage: YAML file-per-task.

**Spec:** `docs/superpowers/specs/2026-07-10-named-transitions-design.md` (adversarially reviewed).

## Global Constraints

- **TDD, red → green → refactor.** `make check` green (no pipe) **before every commit**.
- **Version:** `0.8.97-dev` → `0.8.98-dev` ([internal/cli/root.go:16](../../../internal/cli/root.go#L16)).
- **Precondition (from the spec):** route-by-`to` is correct **iff** the three new invariants hold; the transition path does **not** re-validate (validation runs on `add`/`types` only — the s006/s008 rule). This is the same trust boundary `applyCurrent`/`FindTransition` already rely on. Do NOT re-validate on the move and do NOT touch `core.Transitioner`. Mitigate by (a) documenting it (DESIGN) and (b) the e2e running `mtt types` (validates) before the first move + a validation-rejection e2e.
- **Namespace rules:** edge `Name` optional; when set, **unique per source status** and **disjoint from status names** in the type; `(from,to)` **unique per type**.
- **Docs language:** bilingual pairs in sync (`CLI_REFERENCE`/`DESIGN`/`README` ↔ `.ru`).
- **Carry-over traps:** CLI output via `fmt.Fprint(cmd.OutOrStdout(), …)`; `golangci unused` (declare a symbol where first used — exported methods are exempt); testscript has no pipes, asserts relationships; a bulk/aggregate error must keep its `errors.Is` mapping; reflect new behavior in `--help`.

## File Structure

**Modified:**
- `pkg/mtt/config.go` — `Transition.Name` field (after `Description`).
- `pkg/mtt/type_query.go` — new `FindTransitionByName`.
- `pkg/mtt/validate.go` — 3 new invariants in `validateFlow`.
- `internal/adapter/yaml/dto.go` — `ymlTransition.Name` + `toDomain` map.
- `internal/cli/root.go` — edge-first step in `classifyStatusMove`; register `newDoCmd`; extend the two "plausible verb" branches.
- `internal/cli/resolve.go` — `edgeNameInAnyFlow` helper (next to `statusInAnyFlow`).
- `internal/cli/guidance.go` — `formatNextMoves` shows the verb.
- `internal/cli/types.go` — `writeTypeBlock` shows `[name]`.
- `internal/cli/json.go` — `nextMoveJSON.Name` + `toShowJSON`.
- Docs: `CLI_REFERENCE.md`/`.ru`, `DESIGN.md`/`.ru`, `README.md`/`.ru`, `TASKS.md`, `sessions/README.md`, `NEXT_SESSION.md`, `CHANGELOG.md`, three `CLAUDE.md`.

**Created:**
- `internal/cli/do.go` — `newDoCmd` + `availableActions`.
- Tests: `pkg/mtt/type_query_test.go` (add), `pkg/mtt/validate_test.go` (add), `internal/adapter/yaml/dto_test.go` (add), `internal/cli/show_json_test.go` (add), `internal/cli/do_test.go` (new).
- e2e: `internal/cli/testdata/scripts/edge_verb.txt`, `do.txt`, `named_validation.txt`.
- `sessions/008.98_named_transitions.md`.

---

### Task 1: Domain — `Transition.Name` + `FindTransitionByName`

**Files:**
- Modify: `pkg/mtt/config.go:47-53` (the `Transition` struct)
- Modify: `pkg/mtt/type_query.go` (add `FindTransitionByName`)
- Test: `pkg/mtt/type_query_test.go`

**Interfaces:**
- Produces: `Transition.Name string`; `func (t Type) FindTransitionByName(from StatusName, name string) (Transition, bool)` — the outgoing edge from `from` with `Name == name`; empty `name` never matches; at most one match (guaranteed by Task 2's validation). Name-agnostic (no verb literal in the domain).

- [ ] **Step 1: Write the failing test**

Add to `pkg/mtt/type_query_test.go` (create the file if absent — `package mtt`, import `testing` and this package's types are in-package so no import of `mtt` needed; it IS package `mtt`):

```go
package mtt

import "testing"

func TestFindTransitionByName(t *testing.T) {
	typ := Type{Flow: Flow{Transitions: []Transition{
		{From: "review", To: "fix", Name: "decline"},
		{From: "review", To: "done", Name: "approve"},
		{From: "qa", To: "fix", Name: "decline"}, // same name, different source
		{From: "tbd", To: "review"},              // unnamed
	}}}
	if e, ok := typ.FindTransitionByName("review", "decline"); !ok || e.To != "fix" {
		t.Fatalf("review/decline = %+v, %v; want -> fix", e, ok)
	}
	if e, ok := typ.FindTransitionByName("qa", "decline"); !ok || e.To != "fix" {
		t.Fatalf("qa/decline = %+v, %v; want the qa-sourced edge", e, ok)
	}
	if _, ok := typ.FindTransitionByName("review", "cancel"); ok {
		t.Fatal("review/cancel must miss")
	}
	if _, ok := typ.FindTransitionByName("tbd", "decline"); ok {
		t.Fatal("tbd/decline must miss (decline is not out of tbd)")
	}
	if _, ok := typ.FindTransitionByName("tbd", ""); ok {
		t.Fatal("empty name must never match")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./pkg/mtt/ -run TestFindTransitionByName`
Expected: FAIL to compile — `Transition` has no field `Name`; `FindTransitionByName` undefined.

- [ ] **Step 3: Add the field**

In `pkg/mtt/config.go`, the `Transition` struct:

```go
type Transition struct {
	From        StatusName
	To          StatusName
	Description string
	Name        string // optional edge label — the verb for `mtt <name>` / `mtt do <name>` (empty = unnamed)
	Commands    []Command
	Current     CurrentAction // set|clear the personal current pointer when traversed (empty = no effect)
}
```

- [ ] **Step 4: Add the primitive**

In `pkg/mtt/type_query.go` (after `TransitionsFrom`):

```go
// FindTransitionByName returns the edge leaving `from` whose Name equals `name`,
// if any. Name-agnostic (the domain knows no specific verb string). An empty
// name never matches — unnamed edges are not addressable by name. Config.Validate
// enforces name uniqueness per source status, so at most one edge matches. Used
// by the edge-verb sugar (`mtt <name>`) and `mtt do <name>`.
func (t Type) FindTransitionByName(from StatusName, name string) (Transition, bool) {
	if name == "" {
		return Transition{}, false
	}
	for _, e := range t.Transitions {
		if e.From == from && e.Name == name {
			return e, true
		}
	}
	return Transition{}, false
}
```

- [ ] **Step 5: Run to verify it passes + gate**

Run: `go test ./pkg/mtt/ -run TestFindTransitionByName -v` → PASS
Run: `make check` → PASS

- [ ] **Step 6: Commit**

```bash
git add pkg/mtt/config.go pkg/mtt/type_query.go pkg/mtt/type_query_test.go
git commit -m "feat(domain): Transition.Name + Type.FindTransitionByName [s008.98]"
```

---

### Task 2: Domain — validation invariants

**Files:**
- Modify: `pkg/mtt/validate.go:57-76` (the transition loop in `validateFlow`)
- Test: `pkg/mtt/validate_test.go`

**Interfaces:**
- Consumes: `known map[StatusName]bool` (status-name set, already built in `validateFlow`), `Transition.Name`.
- Produces: `Config.Validate()` now rejects (1) duplicate edge name per source, (2) edge name equal to a status name, (3) duplicate `(from,to)` per type.

- [ ] **Step 1: Write the failing tests**

Add to `pkg/mtt/validate_test.go` (a helper builds a valid named flow, each test injects one violation):

```go
func validNamedType() Type {
	return Type{
		Name: "task", Default: true,
		Flow: Flow{
			Statuses: []Status{
				{Name: "tbd", Kind: KindInitial},
				{Name: "review", Kind: KindActive},
				{Name: "fix", Kind: KindActive},
				{Name: "done", Kind: KindTerminal},
			},
			Transitions: []Transition{
				{From: "tbd", To: "review"},
				{From: "review", To: "fix", Name: "decline"},
				{From: "review", To: "done", Name: "approve"},
				{From: "fix", To: "review"},
			},
		},
	}
}

func TestValidateNamedFlowOK(t *testing.T) {
	if err := (Config{Types: []Type{validNamedType()}}).Validate(); err != nil {
		t.Fatalf("valid named flow rejected: %v", err)
	}
}

func TestValidateRejectsDuplicateEdgeNamePerSource(t *testing.T) {
	typ := validNamedType()
	// a second edge out of review also named "decline" (to a distinct target)
	typ.Transitions = append(typ.Transitions, Transition{From: "review", To: "tbd", Name: "decline"})
	err := (Config{Types: []Type{typ}}).Validate()
	if err == nil || !contains(err.Error(), "duplicate transition name") {
		t.Fatalf("want duplicate-name error, got: %v", err)
	}
}

func TestValidateRejectsEdgeNameEqualToStatusName(t *testing.T) {
	typ := validNamedType()
	typ.Transitions[0].Name = "fix" // tbd->review named "fix", collides with the status "fix"
	err := (Config{Types: []Type{typ}}).Validate()
	if err == nil || !contains(err.Error(), "collides with a status name") {
		t.Fatalf("want name/status collision error, got: %v", err)
	}
}

func TestValidateRejectsDuplicateFromTo(t *testing.T) {
	typ := validNamedType()
	typ.Transitions = append(typ.Transitions, Transition{From: "review", To: "fix", Name: "reject"})
	err := (Config{Types: []Type{typ}}).Validate()
	if err == nil || !contains(err.Error(), "duplicate transition \"review\"->\"fix\"") {
		t.Fatalf("want duplicate-(from,to) error, got: %v", err)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
```

Ensure `validate_test.go` imports `strings` (add if missing). If a `contains` helper already exists in the package's tests, drop the local one and reuse it.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./pkg/mtt/ -run 'TestValidate(NamedFlowOK|RejectsDuplicateEdgeNamePerSource|RejectsEdgeNameEqualToStatusName|RejectsDuplicateFromTo)' -v`
Expected: the three `Rejects…` FAIL (no such validation yet); `NamedFlowOK` passes.

- [ ] **Step 3: Add the invariants**

In `pkg/mtt/validate.go`, replace the transition loop (currently building `in`/`out`) with one that also tracks pairs and edge names. The loop starts at `in := make(map[StatusName]int)`:

```go
	in := make(map[StatusName]int)
	out := make(map[StatusName]int)
	pairs := make(map[string]bool)                    // (from,to) uniqueness
	edgeNames := make(map[StatusName]map[string]bool) // name uniqueness per source
	for _, tr := range t.Transitions {
		if !known[tr.From] {
			errs = append(errs, fmt.Errorf("type %q: transition from unknown status %q", t.Name, tr.From))
		}
		if !known[tr.To] {
			errs = append(errs, fmt.Errorf("type %q: transition to unknown status %q", t.Name, tr.To))
		}
		if !tr.Current.Valid() {
			errs = append(errs, fmt.Errorf("type %q transition %q->%q: invalid current action %q", t.Name, tr.From, tr.To, tr.Current))
		}
		for _, cmd := range tr.Commands {
			if !cmd.Valid() {
				errs = append(errs, fmt.Errorf("type %q transition %q->%q: invalid command (empty/negative timeout or bad rollback)", t.Name, tr.From, tr.To))
			}
		}
		key := string(tr.From) + "->" + string(tr.To)
		if pairs[key] {
			errs = append(errs, fmt.Errorf("type %q: duplicate transition %q->%q", t.Name, tr.From, tr.To))
		}
		pairs[key] = true
		if tr.Name != "" {
			if known[StatusName(tr.Name)] {
				errs = append(errs, fmt.Errorf("type %q transition %q->%q: name %q collides with a status name", t.Name, tr.From, tr.To, tr.Name))
			}
			if edgeNames[tr.From] == nil {
				edgeNames[tr.From] = make(map[string]bool)
			}
			if edgeNames[tr.From][tr.Name] {
				errs = append(errs, fmt.Errorf("type %q status %q: duplicate transition name %q", t.Name, tr.From, tr.Name))
			}
			edgeNames[tr.From][tr.Name] = true
		}
		out[tr.From]++
		in[tr.To]++
	}
```

- [ ] **Step 4: Run to verify they pass + gate**

Run: `go test ./pkg/mtt/ -run 'TestValidate' -v` → PASS (all)
Run: `make check` → PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/mtt/validate.go pkg/mtt/validate_test.go
git commit -m "feat(domain): flow invariants — unique edge name per source, name≠status, unique (from,to) [s008.98]"
```

---

### Task 3: Adapter — `ymlTransition.Name`

**Files:**
- Modify: `internal/adapter/yaml/dto.go:55-61` (`ymlTransition`), `:130` (`toDomain` transition map)
- Test: `internal/adapter/yaml/dto_test.go`

**Interfaces:**
- Produces: on-disk `name:` round-trips into `Transition.Name`.

- [ ] **Step 1: Write the failing test**

Add to `internal/adapter/yaml/dto_test.go`:

```go
func TestToDomainMapsTransitionName(t *testing.T) {
	src := `
version: 1
project: {name: demo}
types:
  - name: task
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: tbd,    kind: initial}
      - {name: review, kind: active}
      - {name: fix,    kind: active}
      - {name: done,   kind: terminal}
    transitions:
      - {from: tbd,    to: review}
      - {from: review, to: fix,  name: decline}
      - {from: review, to: done, name: approve}
      - {from: fix,    to: review}
`
	var yc ymlConfig
	if err := goyaml.Unmarshal([]byte(src), &yc); err != nil {
		t.Fatal(err)
	}
	cfg, _ := yc.toDomain()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("named flow invalid: %v", err)
	}
	e, ok := cfg.Types[0].FindTransitionByName("review", "decline")
	if !ok || e.To != "fix" {
		t.Fatalf("name not mapped: %+v %v", e, ok)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/adapter/yaml/ -run TestToDomainMapsTransitionName`
Expected: FAIL — `FindTransitionByName` misses (name dropped by the DTO).

- [ ] **Step 3: Add the field + map it**

In `internal/adapter/yaml/dto.go`, `ymlTransition`:

```go
type ymlTransition struct {
	From        string       `yaml:"from"`
	To          string       `yaml:"to"`
	Name        string       `yaml:"name,omitempty"`
	Description string       `yaml:"description"`
	Commands    []ymlCommand `yaml:"commands"`
	Current     string       `yaml:"current,omitempty"`
}
```

In `ymlConfig.toDomain`, the transition append (the `t.Transitions = append(...)` line):

```go
			t.Transitions = append(t.Transitions, mtt.Transition{From: mtt.StatusName(yr.From), To: mtt.StatusName(yr.To), Name: yr.Name, Description: yr.Description, Commands: cmds, Current: mtt.CurrentAction(yr.Current)})
```

- [ ] **Step 4: Run to verify it passes + gate**

Run: `go test ./internal/adapter/yaml/ -run TestToDomainMapsTransitionName -v` → PASS
Run: `make check` → PASS

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/yaml/dto.go internal/adapter/yaml/dto_test.go
git commit -m "feat(yaml): map ymlTransition.name to Transition.Name [s008.98]"
```

---

### Task 4: CLI — edge-verb sugar (`mtt <edge> [<id>]`)

**Files:**
- Modify: `internal/cli/root.go:142-155` (`classifyStatusMove`), `:80-104` (`trySugarCurrent`), `:111-135` (`trySugar`)
- Modify: `internal/cli/resolve.go` (add `edgeNameInAnyFlow`)
- Create: `internal/cli/testdata/scripts/edge_verb.txt`

**Interfaces:**
- Consumes: `Type.FindTransitionByName`, `runTransition`, `statusInAnyFlow`.
- Produces: `func edgeNameInAnyFlow(cfg mtt.Config, name string) bool`; the sugar routes an edge name (from the task's current status) before a target status.

- [ ] **Step 1: Write the failing e2e**

Create `internal/cli/testdata/scripts/edge_verb.txt`:

```
# s008.98: the verb sugar `mtt <edge>` moves along a NAMED edge out of the current
# status. `mtt types` runs first — it validates the committed config (the trust-
# model precondition: the move path does not re-validate).

exec mtt init
cp named.yaml .mtt/config.yaml
exec mtt types
stdout 'review -> fix'

exec mtt add 'a task' --no-parent
stdout 'created t1'

# move by STATUS name (existing behavior): tbd -> review
exec mtt review t1
stdout 't1: tbd → review'

# move by EDGE name (new): `decline` is the review->fix edge
exec mtt decline t1
stdout 't1: review → fix'
exec mtt show t1
stdout '\[fix\]'

-- named.yaml --
version: 1
project:
  name: demo
types:
  - name: task
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: tbd,    kind: initial}
      - {name: review, kind: active}
      - {name: fix,    kind: active}
      - {name: done,   kind: terminal}
    transitions:
      - {from: tbd,    to: review}
      - {from: review, to: fix,  name: decline, commands: ["true"]}
      - {from: review, to: done, name: approve, commands: ["true"]}
      - {from: fix,    to: review}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run 'TestScript/edge_verb'`
Expected: FAIL at `mtt decline t1` — `decline` is not a status, so the sugar declines → `unknown command`.

- [ ] **Step 3: Add `edgeNameInAnyFlow`**

In `internal/cli/resolve.go`, after `statusInAnyFlow`:

```go
// edgeNameInAnyFlow reports whether name is a transition name anywhere in the
// config (any type, any source). Lets the sugar treat a bare arg as a plausible
// edge verb (claim the command with an actionable error) vs an unknown command.
func edgeNameInAnyFlow(cfg mtt.Config, name string) bool {
	if name == "" {
		return false
	}
	for _, t := range cfg.Types {
		for _, tr := range t.Transitions {
			if tr.Name == name {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 4: Edge-first resolution in `classifyStatusMove`**

In `internal/cli/root.go`, `classifyStatusMove` — add the edge-name lookup before the status classification:

```go
func classifyStatusMove(cmd *cobra.Command, root string, cfg mtt.Config, settings yaml.Settings, task mtt.Task, statusArg string) (bool, error) {
	typ, ok := cfg.TypeByName(task.Type)
	if !ok {
		return false, nil
	}
	// Edge-verb first: an edge named statusArg leaving the task's CURRENT status.
	// Disjoint namespaces (Config.Validate) make this unobservable for status names.
	if edge, ok := typ.FindTransitionByName(task.Status, statusArg); ok {
		return true, runTransition(cmd, root, cfg, settings, task.ID, edge.To, false)
	}
	to, err := mtt.NewStatusName(statusArg)
	if err != nil {
		return false, nil
	}
	if _, ok := typ.StatusKind(to); !ok {
		return false, nil
	}
	return true, runTransition(cmd, root, cfg, settings, task.ID, to, false)
}
```

- [ ] **Step 5: Extend the "plausible verb" branches**

In `trySugarCurrent` (the `if !ok {` block for no current set):

```go
	if !ok {
		if statusInAnyFlow(cfg, statusArg) || edgeNameInAnyFlow(cfg, statusArg) {
			return true, errors.New("no current task set; run `mtt use <id>` or give an id")
		}
		return false, nil
	}
```

In `trySugar` (the 2-arg missing-task branch):

```go
		if errors.Is(err, mtt.ErrNotFound) && (statusInAnyFlow(cfg, statusArg) || edgeNameInAnyFlow(cfg, statusArg)) {
			return true, taskNotFound(id)
		}
```

- [ ] **Step 6: Run to verify pass + gate**

Run: `go test ./internal/cli/ -run 'TestScript/edge_verb' -v` → PASS
Run: `make check` → PASS

- [ ] **Step 7: Commit**

```bash
git add internal/cli/root.go internal/cli/resolve.go internal/cli/testdata/scripts/edge_verb.txt
git commit -m "feat(cli): edge-verb sugar — mtt <edge> moves along a named edge [s008.98]"
```

---

### Task 5: CLI — `mtt do [<id>] <edge>` + validation-rejection e2e

**Files:**
- Create: `internal/cli/do.go`
- Modify: `internal/cli/root.go:45-46` (register `newDoCmd`)
- Test: `internal/cli/do_test.go`, `internal/cli/testdata/scripts/do.txt`, `internal/cli/testdata/scripts/named_validation.txt`

**Interfaces:**
- Consumes: `resolveTaskID`, `yaml.NewTaskStore(root).Get`, `taskNotFound`, `cfg.TypeByName`, `Type.FindTransitionByName`, `Type.TransitionsFrom`, `runTransition`, `core.ErrInvalidTransition`.
- Produces: `func newDoCmd() *cobra.Command`; `func availableActions(typ mtt.Type, from mtt.StatusName) string`.

- [ ] **Step 1: Write the failing unit test**

Create `internal/cli/do_test.go`:

```go
package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestAvailableActionsListsNamedEdges(t *testing.T) {
	typ := mtt.Type{Flow: mtt.Flow{Transitions: []mtt.Transition{
		{From: "review", To: "fix", Name: "decline"},
		{From: "review", To: "done", Name: "approve"},
		{From: "review", To: "tbd"}, // unnamed — not listed
	}}}
	got := availableActions(typ, "review")
	if !strings.Contains(got, "decline") || !strings.Contains(got, "approve") {
		t.Fatalf("availableActions = %q, want decline+approve", got)
	}
	// a status with no named edges says so, no dangling "available: ".
	none := availableActions(typ, "fix")
	if strings.Contains(none, "available:") {
		t.Fatalf("no-named-edges case must not print an empty list: %q", none)
	}
}

func TestDoUnknownEdgeIsInvalidTransition(t *testing.T) {
	// The miss error wraps core.ErrInvalidTransition (exit 6). Built here the way
	// do.go builds it, to lock the taxonomy without spinning up a project.
	typ := mtt.Type{Flow: mtt.Flow{Transitions: []mtt.Transition{{From: "review", To: "fix", Name: "decline"}}}}
	_, ok := typ.FindTransitionByName("review", "bogus")
	if ok {
		t.Fatal("precondition: bogus must miss")
	}
	err := doMissError(typ, "bogus", "review")
	if !errors.Is(err, core.ErrInvalidTransition) {
		t.Fatalf("do miss must map to exit 6: %v", err)
	}
	if !strings.Contains(err.Error(), "not allowed by the flow") {
		t.Fatalf("message must align with the sentinel: %v", err)
	}
}
```

(`doMissError` is a tiny helper extracted so the taxonomy is unit-testable without a cobra run — see Step 3.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run 'TestAvailableActions|TestDoUnknownEdge'`
Expected: FAIL to compile — `availableActions` / `doMissError` undefined.

- [ ] **Step 3: Create `do.go`**

```go
package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newDoCmd builds `mtt do [<id>] <edge>`: move a task along the NAMED edge leaving
// its current status — the explicit form of the `mtt <edge> [<id>]` sugar,
// symmetric to `mtt status` for target-status moves. Edge-name only (no status
// fallback); it rides the shared runTransition (gate/attribution/--json inherited).
func newDoCmd() *cobra.Command {
	var noRun bool
	cmd := &cobra.Command{
		Use:   "do [<id>] <edge>",
		Short: "Move a task along a named flow edge out of its current status",
		Long: `Move a task along the NAMED edge leaving its current status — the explicit form
of the 'mtt <edge> [<id>]' shorthand (symmetric to 'mtt status' for moves by target
status). The id is optional (resolves to the current task). Edge names are defined
per flow in the config; 'mtt types' lists them.`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 && len(args) != 2 {
				return errors.New("provide an edge name (and optionally a task id): mtt do [<id>] <edge>")
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
			explicit, edgeName := "", args[0]
			if len(args) == 2 {
				explicit, edgeName = args[0], args[1]
			}
			id, err := resolveTaskID(root, explicit)
			if err != nil {
				return err
			}
			task, err := yaml.NewTaskStore(root).Get(id)
			if err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return taskNotFound(id)
				}
				return err
			}
			typ, ok := cfg.TypeByName(task.Type)
			if !ok {
				return fmt.Errorf("unknown type %q for task %q", task.Type, id)
			}
			edge, ok := typ.FindTransitionByName(task.Status, edgeName)
			if !ok {
				return doMissError(typ, edgeName, task.Status)
			}
			return runTransition(cmd, root, cfg, settings, id, edge.To, noRun)
		},
	}
	cmd.Flags().BoolVar(&noRun, "no-run", false, "skip the edge's commands (bypass the gate)")
	return cmd
}

// doMissError reports that `edge` is not a named action out of `from`, wrapping
// core.ErrInvalidTransition (exit 6) and listing the available actions.
func doMissError(typ mtt.Type, edge string, from mtt.StatusName) error {
	return fmt.Errorf("%w: no action %q from status %q%s", core.ErrInvalidTransition, edge, from, availableActions(typ, from))
}

// availableActions renders the named edges leaving `from` for a `mtt do` miss.
func availableActions(typ mtt.Type, from mtt.StatusName) string {
	var names []string
	for _, e := range typ.TransitionsFrom(from) {
		if e.Name != "" {
			names = append(names, e.Name)
		}
	}
	if len(names) == 0 {
		return "; no named actions from this status"
	}
	return "; available: " + strings.Join(names, ", ")
}
```

Register it in `internal/cli/root.go` `AddCommand` (append `newDoCmd()`):

```go
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd(), newShowCmd(),
		newListCmd(), newEditCmd(), newTreeCmd(), newDepCmd(), newReadyCmd(), newStatusCmd(),
		newUseCmd(), newRmCmd(), newRoadmapCmd(), newTagCmd(), newDoCmd())
```

- [ ] **Step 4: Run the unit test + gate**

Run: `go test ./internal/cli/ -run 'TestAvailableActions|TestDoUnknownEdge' -v` → PASS

- [ ] **Step 5: Write the `do` e2e**

Create `internal/cli/testdata/scripts/do.txt`:

```
# s008.98: explicit `mtt do [<id>] <edge>` — edge-name-only, inherits the gate,
# --no-run, and --json from the shared runTransition. Validate first (mtt types).

exec mtt init
cp named.yaml .mtt/config.yaml
exec mtt types

exec mtt add 'task one' --no-parent
stdout 'created t1'
exec mtt review t1
stdout 't1: tbd → review'

# a bad edge from review -> exit 6, message aligned with the sentinel + lists actions
! exec mtt do t1 nonesuch
stderr 'not allowed by the flow'
stderr 'available:'
stderr 'decline'

# good explicit edge move (gate passes)
exec mtt do t1 decline
stdout 't1: review → fix'
exec mtt show t1
stdout '\[fix\]'

# --json emits the task object (inherited)
exec mtt add 'task two' --no-parent
stdout 'created t2'
exec mtt review t2
exec mtt --json do t2 decline
stdout '"status": "fix"'

# a FAILING gate blocks (exit 3); --no-run bypasses it (inherited)
exec mtt add 'task three' --no-parent
stdout 'created t3'
exec mtt review t3
! exec mtt do t3 approve
stderr 'blocked'
exec mtt show t3
stdout '\[review\]'
exec mtt do t3 approve --no-run
stdout 't3: review → done'

-- named.yaml --
version: 1
project:
  name: demo
types:
  - name: task
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: tbd,    kind: initial}
      - {name: review, kind: active}
      - {name: fix,    kind: active}
      - {name: done,   kind: terminal}
    transitions:
      - {from: tbd,    to: review}
      - {from: review, to: fix,  name: decline, commands: ["true"]}
      - {from: review, to: done, name: approve, commands: ["false"]}
      - {from: fix,    to: review}
```

- [ ] **Step 6: Write the validation-rejection e2e**

Create `internal/cli/testdata/scripts/named_validation.txt`:

```
# s008.98: the new flow invariants are surfaced by mtt add / mtt types
# (Config.Validate runs there). A #2 (name==status) and a #3 (dup (from,to))
# config are each rejected.

exec mtt init

cp collide.yaml .mtt/config.yaml
! exec mtt types
stderr 'collides with a status name'

cp dup.yaml .mtt/config.yaml
! exec mtt types
stderr 'duplicate transition'

-- collide.yaml --
version: 1
project: {name: demo}
types:
  - name: task
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: tbd,    kind: initial}
      - {name: review, kind: active}
      - {name: fix,    kind: active}
      - {name: done,   kind: terminal}
    transitions:
      - {from: tbd,    to: review, name: fix}
      - {from: review, to: fix}
      - {from: review, to: done}
      - {from: fix,    to: done}
-- dup.yaml --
version: 1
project: {name: demo}
types:
  - name: task
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: tbd,    kind: initial}
      - {name: review, kind: active}
      - {name: fix,    kind: active}
      - {name: done,   kind: terminal}
    transitions:
      - {from: tbd,    to: review}
      - {from: review, to: fix, name: decline}
      - {from: review, to: fix, name: reject}
      - {from: fix,    to: done}
      - {from: review, to: done}
```

(Note: `mtt types` calls `Config.Validate` before rendering, so a bad config errors there — no `mtt add` needed. Verify each fixture is otherwise structurally valid so the ONLY error is the intended one — e.g. `collide.yaml` keeps `review->fix`, `review->done`, `fix->done` so kinds resolve.)

- [ ] **Step 7: Run the e2e + gate**

Run: `go test ./internal/cli/ -run 'TestScript/(do|named_validation)' -v` → PASS
Run: `make check` → PASS

- [ ] **Step 8: Commit**

```bash
git add internal/cli/do.go internal/cli/root.go internal/cli/do_test.go \
        internal/cli/testdata/scripts/do.txt internal/cli/testdata/scripts/named_validation.txt
git commit -m "feat(cli): mtt do <edge> — explicit named-edge move (exit 6 on miss) [s008.98]"
```

---

### Task 6: Discoverability — `mtt types`, `next:`, and `show --json`

**Files:**
- Modify: `internal/cli/types.go:91-96` (`writeTypeBlock` transition line)
- Modify: `internal/cli/guidance.go:53-63` (`formatNextMoves`)
- Modify: `internal/cli/json.go` (`nextMoveJSON` + `toShowJSON`)
- Test: `internal/cli/show_json_test.go` (add), extend `internal/cli/testdata/scripts/edge_verb.txt`

**Interfaces:**
- Produces: `nextMoveJSON.Name string` (omitempty); `mtt types` prints `[name] from -> to`; `next:` prints `name → to (desc)`.

- [ ] **Step 1: Write the failing unit test**

Add to `internal/cli/show_json_test.go`:

```go
func TestNextMoveJSONCarriesName(t *testing.T) {
	ts := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	task := mtt.Task{ID: "t1", Type: "task", Status: "review", Created: ts, Updated: ts}
	onward := []mtt.Transition{{To: "fix", Name: "decline", Description: "send back"}, {To: "done"}}
	sj := toShowJSON(task, "", onward)
	if len(sj.Next) != 2 || sj.Next[0].Name != "decline" || sj.Next[0].To != "fix" {
		t.Fatalf("next[0] = %+v, want name=decline to=fix", sj.Next[0])
	}
	// an unnamed onward move omits name
	data, _ := json.Marshal(sj.Next[1])
	if strings.Contains(string(data), "name") {
		t.Fatalf("unnamed move must omit name: %s", data)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run TestNextMoveJSONCarriesName`
Expected: FAIL — `nextMoveJSON` has no `Name` field.

- [ ] **Step 3: `nextMoveJSON.Name` + `toShowJSON`**

In `internal/cli/json.go`, `nextMoveJSON`:

```go
type nextMoveJSON struct {
	Name        string `json:"name,omitempty"`
	To          string `json:"to"`
	Description string `json:"description,omitempty"`
}
```

In `toShowJSON`, the `sj.Next` build:

```go
	for _, e := range onward {
		sj.Next = append(sj.Next, nextMoveJSON{Name: e.Name, To: string(e.To), Description: e.Description})
	}
```

- [ ] **Step 4: `formatNextMoves` shows the verb**

In `internal/cli/guidance.go`, `formatNextMoves`:

```go
func formatNextMoves(onward []mtt.Transition) string {
	parts := make([]string, 0, len(onward))
	for _, e := range onward {
		s := string(e.To)
		if e.Name != "" {
			s = e.Name + " → " + string(e.To)
		}
		if e.Description != "" {
			s += " (" + e.Description + ")"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " · ")
}
```

- [ ] **Step 5: `writeTypeBlock` shows `[name]`**

In `internal/cli/types.go`, the transition line:

```go
	for _, tr := range t.Transitions {
		if tr.Name != "" {
			fmt.Fprintf(b, "    [%s] %s -> %s", tr.Name, tr.From, tr.To)
		} else {
			fmt.Fprintf(b, "    %s -> %s", tr.From, tr.To)
		}
		if tr.Description != "" {
			fmt.Fprintf(b, "  # %s", tr.Description)
		}
		b.WriteString("\n")
		// ... (command lines unchanged)
	}
```

- [ ] **Step 6: Extend the e2e to assert discoverability**

Add to `internal/cli/testdata/scripts/edge_verb.txt` after the `exec mtt types` line:

```
stdout '\[decline\] review -> fix'
```

And after the first move (`t1: tbd → review`), assert the onward guidance shows the verb (the move footer prints `next:` for the destination `review`):

```
stdout 'decline → fix'
```

- [ ] **Step 7: Run + gate**

Run: `go test ./internal/cli/ -run 'TestNextMoveJSONCarriesName|TestScript/edge_verb' -v` → PASS
Run: `make check` → PASS (the `default`/`coding` `types` goldens are unaffected — their edges are unnamed, so the `[name]` branch never fires; confirm no golden asserts the transitions block verbatim, else `-update` it).

- [ ] **Step 8: Commit**

```bash
git add internal/cli/types.go internal/cli/guidance.go internal/cli/json.go \
        internal/cli/show_json_test.go internal/cli/testdata/scripts/edge_verb.txt
git commit -m "feat(cli): surface edge verbs in types, next: guidance, and show --json [s008.98]"
```

---

### Task 7: Docs sync + session record + version bump + final gate

**Files:**
- Modify: `internal/cli/root.go:16` (version), `CLI_REFERENCE.md`/`.ru`, `DESIGN.md`/`.ru`, `README.md`/`.ru`, `TASKS.md`, `sessions/README.md`, `NEXT_SESSION.md`, `CHANGELOG.md`, `pkg/mtt/CLAUDE.md`, `internal/cli/CLAUDE.md`, `internal/adapter/yaml/CLAUDE.md`
- Create: `sessions/008.98_named_transitions.md`

- [ ] **Step 1: Version bump**

`internal/cli/root.go:16`: `var version = "0.8.98-dev"`. `README.md`/`.ru` status blurb + `CLI_REFERENCE.md`/`.ru:9` version line → `0.8.98-dev` / `session 008.98`.

- [ ] **Step 2: CLI_REFERENCE (both languages)**

Add a `mtt do [<id>] <edge>` command section under "Flow"; note the edge-verb sugar `mtt <edge> [<id>]` alongside the existing `mtt <status>`; document `name:` on a transition in the config/flow section; add `next[].name` to the `show --json` bullet. Keep `.ru` in sync.

- [ ] **Step 3: DESIGN (both languages)**

Add the resolution triad (status vs edge, explicit vs sugar); the three new flow invariants; and the precondition/trust-model paragraph (route-by-`to` is safe iff the invariants hold; the move path does not re-validate — same boundary as `applyCurrent`). Keep `.ru` in sync.

- [ ] **Step 4: CLAUDE.md files**

`pkg/mtt/CLAUDE.md` (the Invariants line + `Transition.Name`/`FindTransitionByName`); `internal/cli/CLAUDE.md` (the `do` command, edge sugar, `edgeNameInAnyFlow`); `internal/adapter/yaml/CLAUDE.md` (the `ymlTransition.name` mapping).

- [ ] **Step 5: Backlog / session**

`TASKS.md`: add `- [x] e5_t1e — named transitions + edge-verb sugar (s008.98) — DONE`. `sessions/README.md`: add an `008.98 ✅` row. Create `sessions/008.98_named_transitions.md` from `sessions/000_template.md` (Target/Scope/Acceptance/Done). `NEXT_SESSION.md`: "Where we are" bullet + a "Carry-over lessons (008.98)" block. `CHANGELOG.md`: Unreleased entries (edge names; `mtt do`; verbs in types/next/json; the three invariants).

- [ ] **Step 6: Final gate + binary verify**

Run: `make check` → PASS
Run: `make build && ./bin/mtt types` in a named-flow scratch dir shows `[decline] …`; `./bin/mtt do --help` shows `do [<id>] <edge>`. Clean up `rm -rf bin/.mtt` if created.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "docs(s008.98): CLI_REFERENCE/DESIGN/CLAUDE, session record, version 0.8.98-dev"
```

---

## Self-Review

**Spec coverage:** Transition.Name (T1) · FindTransitionByName (T1) · validation #1/#2/#3 (T2) · yaml DTO (T3) · sugar edge resolution + edgeNameInAnyFlow (T4) · mtt do + failure taxonomy (T5) · validation-rejection e2e (T5) · discoverability types/next/json (T6) · precondition mitigation = `mtt types` first in every move e2e + named_validation.txt (T4/T5) · docs/version (T7). All covered.

**Type consistency:** `FindTransitionByName(from StatusName, name string) (Transition, bool)` used identically in T1/T4/T5; `nextMoveJSON.Name` defined (T6) and set in `toShowJSON` (T6); `doMissError`/`availableActions` defined and used in T5; `edgeNameInAnyFlow` defined (T4) used in T4.

**Placeholder scan:** no TBD/"handle edge cases"; every code step has full code; e2e are complete txtar.

**Risks flagged for the plan reviewer:**
1. T6 Step 7 — confirm no golden test asserts a `types` transitions block verbatim that would flip when the `[name]` branch is added (it won't fire for unnamed template edges, but check `internal/adapter/yaml/testdata/golden/*` and any `types` e2e).
2. T5 `named_validation.txt` — each fixture must fail on EXACTLY the intended invariant (structurally valid otherwise); a second latent error would still make `! exec mtt types` pass but muddy the assertion — verify kinds resolve in `collide.yaml`/`dup.yaml`.
3. T4/T6 — the move footer `next:` assertion depends on the destination status having onward moves; `review` does (→fix/→done), so `mtt review t1` prints `next: decline → fix · approve → done` — confirm the exact separator/format against `moveGuidance`.
