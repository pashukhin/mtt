# Session 005 — Dependencies Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add blocking dependency edges (`depends_on`) between tasks — `mtt dep add/rm/list`, cycle rejection, and the actionable-set view `mtt ready` / `list --ready`.

**Architecture:** No new port (GAP #1): the edge rides the existing `Task.DependsOn` field and round-trips via `TaskStore.Update`, exactly as `parent` did in s004. Policy lives in `internal/core`: `Ready` (pure read), `DependencyEditor` (mutation owning the cycle-check), and a derived `DepGraph` over `depends_on` (mirroring the s004 `Index` over `parent`, kept separate — GAP #6 not extracted). The CLI is thin: parse → wire adapter → call core → format.

**Tech Stack:** Go 1.23, cobra, `gopkg.in/yaml.v3`, `go-internal/testscript` (e2e). Storage: YAML file-per-task under `.mtt/`.

## Global Constraints

- **TDD, red→green→refactor.** Each task: failing test first, then minimal code. `make check` green before every commit.
- **`make check`** = gofmt + `go vet` + golangci-lint(v2) + `go test -race -cover` + `go build`. It is the gate; CI runs the same.
- **Layering:** `cli → core → port ← adapter`; `internal/core` must NOT import `internal/adapter/*`.
- **Typed identities everywhere:** `mtt.TaskID`/`TypeName`/`StatusName`. Convert `string↔typed` only at the cli (arg parsing) and adapter (DTO) boundaries — never in core.
- **CLI output** via `fmt.Fprint(cmd.OutOrStdout(), …)` (NOT `cmd.Print`); errors via `RunE`/`Execute` (stderr).
- **testscript asserts anchor**, never bare substrings: assert `'t2  task  \[tbd\]  B'`, not `'B'`.
- **Zero-match `--json` = `[]`, not `null`:** build slices with `make([]T, 0, …)` before appending.
- **`golangci unused`:** declare each new unexported symbol in the change that first *uses* it (TDD satisfies this — the test uses it).
- **Ready is conservative:** any unresolvable status (config drift) or dangling blocker leaves a task not-ready.
- **Commit trailer** on every commit: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`
- Branch: `feat/s005-dependencies` (already created; plan/spec already committed as `eac8cd0`).

Reusable test helpers already in `internal/core` (same package — do NOT redefine): `cfg()` and `flow()` (add_test.go — types epic/task/subtask, flow `tbd`(initial)→`doing`(active)→`done`(terminal)), `fixed()` (add_test.go — a fixed `time.Time`), `node(id, parent, created)` and `ids([]Task)` (index_test.go). The existing `fakeStore` (add_test.go) has `List() → nil`; the dependency tests need a store whose `List` returns its contents, so Task 3 adds a separate `memStore`.

---

### Task 1: `core.Ready` + `kindOf` helper (DRY with `matchesKind`)

**Files:**
- Create: `internal/core/ready.go`
- Create: `internal/core/ready_test.go`
- Modify: `internal/core/list.go` (refactor `matchesKind` to use `kindOf`)

**Interfaces:**
- Produces: `func Ready(tasks []mtt.Task, cfg mtt.Config) []mtt.Task` — actionable tasks, Created-desc order. `func kindOf(t mtt.Task, cfg mtt.Config) (mtt.StatusKind, bool)` — resolve a task's status category (false on config drift).
- Consumes: `lessByRecency` (index.go), `cfg()`/`flow()`/`fixed()` test helpers.

- [ ] **Step 1: Write the failing test** — `internal/core/ready_test.go`:

```go
package core

import (
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// dep builds a task of the default "task" type with a status and blockers.
func dep(id mtt.TaskID, status mtt.StatusName, blockers ...mtt.TaskID) mtt.Task {
	return mtt.Task{ID: id, Type: "task", Status: status, DependsOn: blockers, Created: fixed()}
}

func TestReadyConservative(t *testing.T) {
	tasks := []mtt.Task{
		dep("t1", "tbd"),                  // no blockers, non-terminal → ready
		dep("t2", "tbd", "t1"),            // blocker t1 is tbd (non-terminal) → not ready
		dep("t3", "done"),                 // terminal itself → not ready
		dep("t4", "tbd", "ghost"),         // dangling blocker → not ready
		dep("t5", "weird"),                // status not in flow (drift) → not ready
	}
	got := ids(Ready(tasks, cfg()))
	if len(got) != 1 || got[0] != "t1" {
		t.Fatalf("Ready = %v; want [t1]", got)
	}
}

func TestReadyBlockerDoneUnblocks(t *testing.T) {
	tasks := []mtt.Task{
		dep("t1", "done"),      // terminal blocker
		dep("t2", "tbd", "t1"), // blocker terminal → ready
	}
	got := ids(Ready(tasks, cfg()))
	if len(got) != 1 || got[0] != "t2" {
		t.Fatalf("Ready = %v; want [t2] (t1 is terminal, excluded; t2 unblocked)", got)
	}
}

func TestKindOf(t *testing.T) {
	if k, ok := kindOf(dep("x", "done"), cfg()); !ok || k != mtt.KindTerminal {
		t.Fatalf("kindOf(done) = %v,%v; want terminal,true", k, ok)
	}
	if _, ok := kindOf(dep("x", "nope"), cfg()); ok {
		t.Fatalf("kindOf(unknown status) ok=true; want false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ -run 'TestReady|TestKindOf' -v`
Expected: FAIL — `undefined: Ready`, `undefined: kindOf`.

- [ ] **Step 3: Write minimal implementation** — `internal/core/ready.go`:

```go
package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// kindOf resolves the status category of t via its type's flow in cfg. It reports
// false when the type or the status is unknown to cfg (config drift). Shared by
// Ready and matchesKind so the "resolve a task's kind" lookup lives in one place.
func kindOf(t mtt.Task, cfg mtt.Config) (mtt.StatusKind, bool) {
	typ, ok := cfg.TypeByName(t.Type)
	if !ok {
		return "", false
	}
	return typ.StatusKind(t.Status)
}

// Ready returns the actionable tasks in deterministic order (Created desc, ID
// tiebreak): a task is ready iff its own status resolves to a non-terminal kind
// AND every DependsOn resolves to a present task whose status is terminal.
// Conservative — any unresolvable status or dangling blocker leaves a task
// not-ready. Pure: no store, no clock.
func Ready(tasks []mtt.Task, cfg mtt.Config) []mtt.Task {
	byID := make(map[mtt.TaskID]mtt.Task, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
	}
	out := make([]mtt.Task, 0, len(tasks))
	for _, t := range tasks {
		if isReady(t, byID, cfg) {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return lessByRecency(out[i], out[j], SortCreated)
	})
	return out
}

// isReady reports whether t is actionable (see Ready), conservative on anything
// unresolvable.
func isReady(t mtt.Task, byID map[mtt.TaskID]mtt.Task, cfg mtt.Config) bool {
	k, ok := kindOf(t, cfg)
	if !ok || k == mtt.KindTerminal {
		return false
	}
	for _, blockerID := range t.DependsOn {
		blocker, ok := byID[blockerID]
		if !ok {
			return false // dangling blocker
		}
		bk, ok := kindOf(blocker, cfg)
		if !ok || bk != mtt.KindTerminal {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Refactor `matchesKind` to use `kindOf`** — in `internal/core/list.go`, replace the body of `matchesKind`:

```go
func matchesKind(t mtt.Task, kinds []mtt.StatusKind, cfg mtt.Config) bool {
	k, ok := kindOf(t, cfg)
	if !ok {
		return false
	}
	for _, want := range kinds {
		if want == k {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/core/ -v`
Expected: PASS (new Ready/kindOf tests + existing match/list tests still green).

- [ ] **Step 6: Commit**

```bash
git add internal/core/ready.go internal/core/ready_test.go internal/core/list.go
git commit -m "feat(core): Ready pure read + shared kindOf helper

Ready reports actionable tasks (non-terminal, all blockers terminal by
kind), conservative on dangling/drift. kindOf DRYs the type→StatusKind
lookup shared with matchesKind.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: `core.DepGraph` (derived graph over `depends_on`)

**Files:**
- Create: `internal/core/depgraph.go`
- Create: `internal/core/depgraph_test.go`

**Interfaces:**
- Produces: `func NewDepGraph(tasks []mtt.Task) DepGraph` with methods `Get(id) (mtt.Task, bool)`, `DependsOn(id) []mtt.TaskID` (stored order, dangling ids kept), `Dependents(id) []mtt.Task` (computed reverse edges, Created-desc order), `Reaches(from, to mtt.TaskID) bool` (path of ≥1 depends_on edge, cycle-safe), `Cycles() [][]mtt.TaskID` (dependency cycles, deterministic entry order).
- Consumes: `lessByRecency` (index.go), `node`/`ids` test helpers.

- [ ] **Step 1: Write the failing test** — `internal/core/depgraph_test.go`:

```go
package core

import (
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func withDeps(id mtt.TaskID, blockers ...mtt.TaskID) mtt.Task {
	return mtt.Task{ID: id, Type: "task", Status: "tbd", DependsOn: blockers, Created: fixed()}
}

func TestDepGraphDependentsAndReaches(t *testing.T) {
	// t3 -> t2 -> t1  (t3 depends on t2, t2 depends on t1)
	g := NewDepGraph([]mtt.Task{withDeps("t1"), withDeps("t2", "t1"), withDeps("t3", "t2")})

	if got := ids(g.Dependents("t1")); len(got) != 1 || got[0] != "t2" {
		t.Fatalf("Dependents(t1) = %v; want [t2]", got)
	}
	if !g.Reaches("t3", "t1") {
		t.Fatalf("Reaches(t3,t1) = false; want true (transitive)")
	}
	if g.Reaches("t1", "t3") {
		t.Fatalf("Reaches(t1,t3) = true; want false (wrong direction)")
	}
	if got := g.DependsOn("t2"); len(got) != 1 || got[0] != "t1" {
		t.Fatalf("DependsOn(t2) = %v; want [t1]", got)
	}
}

func TestDepGraphCyclesHandBuilt(t *testing.T) {
	// a -> b -> a  (hand-broken data; the CLI cannot create this)
	g := NewDepGraph([]mtt.Task{withDeps("a", "b"), withDeps("b", "a")})
	cycles := g.Cycles()
	if len(cycles) == 0 {
		t.Fatalf("Cycles() = []; want at least one cycle")
	}
	// acyclic graph reports none
	clean := NewDepGraph([]mtt.Task{withDeps("t1"), withDeps("t2", "t1")})
	if got := clean.Cycles(); len(got) != 0 {
		t.Fatalf("Cycles(acyclic) = %v; want []", got)
	}
}

func TestDepGraphReachesCycleSafe(t *testing.T) {
	g := NewDepGraph([]mtt.Task{withDeps("a", "b"), withDeps("b", "a")})
	// must terminate even though a<->b cycle; b is reachable from a
	if !g.Reaches("a", "b") {
		t.Fatalf("Reaches(a,b) = false; want true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ -run TestDepGraph -v`
Expected: FAIL — `undefined: NewDepGraph`.

- [ ] **Step 3: Write minimal implementation** — `internal/core/depgraph.go`:

```go
package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// DepGraph is a derived, read-only view of the depends_on blocking graph over a
// set of tasks (a pure value: no store, no clock). It is not part of the pkg/mtt
// contract — the resolved graph is derived. Forward edges are each task's stored
// DependsOn; Dependents (reverse edges) are computed. Cycle-safe throughout.
// Kept separate from Index: parent is a single-parent tree walked upward,
// depends_on is a multi-edge DAG walked downward (GAP #6 not extracted).
type DepGraph struct {
	byID       map[mtt.TaskID]mtt.Task
	dependents map[mtt.TaskID][]mtt.Task // keyed by blocker id: tasks that depend on it
}

// NewDepGraph builds the dependency view. Dependent buckets are ordered by
// lessByRecency (Created desc, ID tiebreak) so output matches list/tree order.
func NewDepGraph(tasks []mtt.Task) DepGraph {
	g := DepGraph{
		byID:       make(map[mtt.TaskID]mtt.Task, len(tasks)),
		dependents: make(map[mtt.TaskID][]mtt.Task),
	}
	for _, t := range tasks {
		g.byID[t.ID] = t
	}
	for _, t := range tasks {
		for _, blocker := range t.DependsOn {
			g.dependents[blocker] = append(g.dependents[blocker], t)
		}
	}
	for k := range g.dependents {
		bucket := g.dependents[k]
		sort.SliceStable(bucket, func(i, j int) bool {
			return lessByRecency(bucket[i], bucket[j], SortCreated)
		})
	}
	return g
}

// Get returns the task with id, or false when absent.
func (g DepGraph) Get(id mtt.TaskID) (mtt.Task, bool) {
	t, ok := g.byID[id]
	return t, ok
}

// DependsOn returns id's direct blocker ids in stored order (dangling ids kept —
// the caller resolves and flags them). Nil when id is absent or has no blockers.
func (g DepGraph) DependsOn(id mtt.TaskID) []mtt.TaskID {
	t, ok := g.byID[id]
	if !ok {
		return nil
	}
	return t.DependsOn
}

// Dependents returns the tasks that directly depend on id, in sibling order.
func (g DepGraph) Dependents(id mtt.TaskID) []mtt.Task { return g.dependents[id] }

// Reaches reports whether to is reachable from `from` by following depends_on
// edges (a path of one or more edges). Cycle-safe (visited-set). Powers the add
// cycle-check: adding id → dependsOn cycles iff Reaches(dependsOn, id).
func (g DepGraph) Reaches(from, to mtt.TaskID) bool {
	seen := map[mtt.TaskID]bool{}
	var dfs func(cur mtt.TaskID) bool
	dfs = func(cur mtt.TaskID) bool {
		t, ok := g.byID[cur]
		if !ok {
			return false
		}
		for _, dep := range t.DependsOn {
			if dep == to {
				return true
			}
			if !seen[dep] {
				seen[dep] = true
				if dfs(dep) {
					return true
				}
			}
		}
		return false
	}
	return dfs(from)
}

// Cycles returns the dependency cycles in the graph, each as the id chain of the
// nodes on the cycle (traversal order). Empty when the graph is acyclic. Entry
// order is deterministic (ids sorted as opaque strings). Defensive: the CLI's add
// rejects cycles, so this only fires on hand-edited data.
func (g DepGraph) Cycles() [][]mtt.TaskID {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[mtt.TaskID]int{}
	var stack []mtt.TaskID
	var cycles [][]mtt.TaskID
	var dfs func(cur mtt.TaskID)
	dfs = func(cur mtt.TaskID) {
		color[cur] = gray
		stack = append(stack, cur)
		for _, dep := range g.byID[cur].DependsOn {
			if _, ok := g.byID[dep]; !ok {
				continue // dangling — not a cycle
			}
			switch color[dep] {
			case white:
				dfs(dep)
			case gray:
				cycles = append(cycles, extractCycle(stack, dep))
			}
		}
		stack = stack[:len(stack)-1]
		color[cur] = black
	}
	entries := make([]mtt.TaskID, 0, len(g.byID))
	for id := range g.byID {
		entries = append(entries, id)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i] < entries[j] })
	for _, id := range entries {
		if color[id] == white {
			dfs(id)
		}
	}
	return cycles
}

// extractCycle slices the recursion stack from the back-edge target to the top.
func extractCycle(stack []mtt.TaskID, start mtt.TaskID) []mtt.TaskID {
	for i, id := range stack {
		if id == start {
			cyc := make([]mtt.TaskID, len(stack)-i)
			copy(cyc, stack[i:])
			return cyc
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/core/ -run TestDepGraph -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/depgraph.go internal/core/depgraph_test.go
git commit -m "feat(core): DepGraph derived over depends_on

Pure derived graph mirroring Index: Get/DependsOn/Dependents (computed
reverse)/Reaches (cycle-check)/Cycles (defensive). Kept separate from
Index (GAP #6 not extracted).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: `core.DependencyEditor` (mutation + cycle-check) + core `CLAUDE.md`

**Files:**
- Create: `internal/core/dependency.go`
- Create: `internal/core/dependency_test.go`
- Modify: `internal/core/CLAUDE.md`

**Interfaces:**
- Produces: `func NewDependencyEditor(store mtt.TaskStore, now func() time.Time) *DependencyEditor` with `AddDependency(id, dependsOn mtt.TaskID) (mtt.Task, error)` and `RemoveDependency(id, dependsOn mtt.TaskID) (mtt.Task, error)`.
- Consumes: `NewDepGraph`/`Reaches` (Task 2), `mtt.TaskStore`, `mtt.ErrNotFound`.

- [ ] **Step 1: Write the failing test** — `internal/core/dependency_test.go`:

```go
package core

import (
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// memStore is an in-memory TaskStore whose List returns its contents (the
// add_test fakeStore returns nil, which the cycle-check cannot use).
type memStore struct {
	byID    map[mtt.TaskID]mtt.Task
	updated mtt.Task
}

func newMemStore(tasks ...mtt.Task) *memStore {
	m := &memStore{byID: map[mtt.TaskID]mtt.Task{}}
	for _, t := range tasks {
		m.byID[t.ID] = t
	}
	return m
}

func (m *memStore) Create(t mtt.Task) (mtt.Task, error) { m.byID[t.ID] = t; return t, nil }
func (m *memStore) Get(id mtt.TaskID) (mtt.Task, error) {
	if t, ok := m.byID[id]; ok {
		return t, nil
	}
	return mtt.Task{}, mtt.ErrNotFound
}
func (m *memStore) List() ([]mtt.Task, error) {
	out := make([]mtt.Task, 0, len(m.byID))
	for _, t := range m.byID {
		out = append(out, t)
	}
	return out, nil
}
func (m *memStore) Update(t mtt.Task) (mtt.Task, error) { m.byID[t.ID] = t; m.updated = t; return t, nil }

func laterClock() time.Time { return time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC) }

func TestAddDependencyHappyPath(t *testing.T) {
	m := newMemStore(withDeps("t1"), withDeps("t2"))
	got, err := NewDependencyEditor(m, laterClock).AddDependency("t2", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.DependsOn) != 1 || got.DependsOn[0] != "t1" {
		t.Fatalf("DependsOn = %v; want [t1]", got.DependsOn)
	}
	if !got.Updated.Equal(laterClock().Truncate(time.Second)) {
		t.Fatalf("Updated not bumped: %v", got.Updated)
	}
	if len(m.updated.DependsOn) != 1 {
		t.Fatalf("not persisted via Update: %+v", m.updated)
	}
}

func TestAddDependencySelfRejected(t *testing.T) {
	m := newMemStore(withDeps("t1"))
	_, err := NewDependencyEditor(m, laterClock).AddDependency("t1", "t1")
	if err == nil || !strings.Contains(err.Error(), "itself") {
		t.Fatalf("err = %v; want a self-dependency error", err)
	}
}

func TestAddDependencyDuplicateNoop(t *testing.T) {
	m := newMemStore(withDeps("t1"), withDeps("t2", "t1"))
	before := m.byID["t2"].Updated
	got, err := NewDependencyEditor(m, laterClock).AddDependency("t2", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.DependsOn) != 1 {
		t.Fatalf("duplicate add changed DependsOn: %v", got.DependsOn)
	}
	if !got.Updated.Equal(before) {
		t.Fatalf("duplicate add bumped Updated: %v", got.Updated)
	}
}

func TestAddDependencyNotFound(t *testing.T) {
	m := newMemStore(withDeps("t1"))
	if _, err := NewDependencyEditor(m, laterClock).AddDependency("t1", "ghost"); err == nil ||
		!strings.Contains(err.Error(), "dependency") {
		t.Fatalf("err = %v; want dependency-not-found", err)
	}
	if _, err := NewDependencyEditor(m, laterClock).AddDependency("ghost", "t1"); err == nil ||
		!strings.Contains(err.Error(), "task") {
		t.Fatalf("err = %v; want task-not-found", err)
	}
}

func TestAddDependencyCycleRejected(t *testing.T) {
	// t2 depends on t1; adding t1 -> t2 would close a cycle.
	m := newMemStore(withDeps("t1"), withDeps("t2", "t1"))
	_, err := NewDependencyEditor(m, laterClock).AddDependency("t1", "t2")
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("err = %v; want a cycle rejection", err)
	}
}

func TestRemoveDependency(t *testing.T) {
	m := newMemStore(withDeps("t1"), withDeps("t2", "t1"))
	got, err := NewDependencyEditor(m, laterClock).RemoveDependency("t2", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.DependsOn) != 0 {
		t.Fatalf("DependsOn = %v; want empty", got.DependsOn)
	}
	if _, err := NewDependencyEditor(m, laterClock).RemoveDependency("t2", "t1"); err == nil ||
		!strings.Contains(err.Error(), "does not depend") {
		t.Fatalf("err = %v; want absent-edge error", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ -run 'TestAddDependency|TestRemoveDependency' -v`
Expected: FAIL — `undefined: NewDependencyEditor`.

- [ ] **Step 3: Write minimal implementation** — `internal/core/dependency.go`:

```go
package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// DependencyEditor mutates a task's blocking edges (depends_on) and persists via
// TaskStore.Update. The cycle rule is enforced here (core policy). No new port:
// the edge is a Task field, round-tripped by the adapter DTO (mirrors s004's
// parent).
type DependencyEditor struct {
	store mtt.TaskStore
	now   func() time.Time
}

// NewDependencyEditor wires the usecase. now is injected for deterministic tests.
func NewDependencyEditor(store mtt.TaskStore, now func() time.Time) *DependencyEditor {
	return &DependencyEditor{store: store, now: now}
}

// AddDependency makes id depend on dependsOn. Both tasks must exist; a self-edge
// and any edge that would create a cycle are rejected; an already-present edge is
// an idempotent no-op (no write, no timestamp bump). On a real change it bumps
// Updated and persists.
func (d *DependencyEditor) AddDependency(id, dependsOn mtt.TaskID) (mtt.Task, error) {
	if id == dependsOn {
		return mtt.Task{}, fmt.Errorf("a task cannot depend on itself")
	}
	t, err := d.load(id, "task")
	if err != nil {
		return mtt.Task{}, err
	}
	if _, err := d.load(dependsOn, "dependency"); err != nil {
		return mtt.Task{}, err
	}
	for _, dep := range t.DependsOn {
		if dep == dependsOn {
			return t, nil // idempotent: already present
		}
	}
	tasks, err := d.store.List()
	if err != nil {
		return mtt.Task{}, fmt.Errorf("list tasks: %w", err)
	}
	if NewDepGraph(tasks).Reaches(dependsOn, id) {
		return mtt.Task{}, fmt.Errorf("adding dependency %q → %q would create a cycle", id, dependsOn)
	}
	t.DependsOn = append(t.DependsOn, dependsOn)
	t.Updated = d.now().UTC().Truncate(time.Second)
	return d.store.Update(t)
}

// RemoveDependency drops the dependsOn edge from id. Errors when id is absent or
// the edge is not present. On removal it bumps Updated and persists.
func (d *DependencyEditor) RemoveDependency(id, dependsOn mtt.TaskID) (mtt.Task, error) {
	t, err := d.load(id, "task")
	if err != nil {
		return mtt.Task{}, err
	}
	idx := -1
	for i, dep := range t.DependsOn {
		if dep == dependsOn {
			idx = i
			break
		}
	}
	if idx == -1 {
		return mtt.Task{}, fmt.Errorf("task %q does not depend on %q", id, dependsOn)
	}
	t.DependsOn = append(t.DependsOn[:idx], t.DependsOn[idx+1:]...)
	t.Updated = d.now().UTC().Truncate(time.Second)
	return d.store.Update(t)
}

// load fetches a task, mapping ErrNotFound to a caller-labelled message (role is
// "task" or "dependency").
func (d *DependencyEditor) load(id mtt.TaskID, role string) (mtt.Task, error) {
	t, err := d.store.Get(id)
	if err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return mtt.Task{}, fmt.Errorf("%s %q not found", role, id)
		}
		return mtt.Task{}, fmt.Errorf("load %s %q: %w", role, id, err)
	}
	return t, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/core/ -v`
Expected: PASS (all core tests).

- [ ] **Step 5: Update `internal/core/CLAUDE.md`** — add these bullets under "Responsibilities" (after the `Editor` bullet):

```markdown
- `Ready` (pure read): actionable tasks — status non-terminal AND every `depends_on` terminal, resolved by
  category (`kindOf` → `Type.StatusKind`). **Conservative**: a dangling blocker or a config-drift status
  (unresolvable) leaves a task not-ready. No store/clock; ordered by the shared `lessByRecency`. One
  primitive behind both `mtt ready` and `list --ready` (`Select(Ready(...), filter, cfg)`).
- `DependencyEditor` (mutation): `AddDependency`/`RemoveDependency` edit `Task.DependsOn` and persist via
  `TaskStore.Update` (**no new port** — the edge rides the field, like `parent` in s004). Rejects a
  self-edge and, via `DepGraph.Reaches`, any edge that would create a **cycle**; a duplicate add is an
  idempotent no-op; a missing edge on remove errors. Bumps `updated` from the injected clock on a real change.
- `DepGraph` (pure derived graph over `depends_on`, parallel to `Index` over `parent`): built from a task
  slice (`NewDepGraph`) — no store/clock, not in the `pkg/mtt` contract. Exposes `Get`/`DependsOn` (stored
  order, dangling kept)/`Dependents` (**computed** reverse edges)/`Reaches` (cycle-check)/`Cycles`
  (defensive). Cycle-safe (visited-set); sibling order matches `Select`. Kept **separate** from `Index`
  (GAP #6 not extracted — `parent` is a single-parent tree, `depends_on` a multi-edge DAG).
```

Also add to the "Identities" section a sentence: `Ready`, `DependencyEditor`, and `DepGraph` use `mtt.TaskID` throughout; string conversion stays at the cli/adapter boundary.

- [ ] **Step 6: Run the full gate**

Run: `make check`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/core/dependency.go internal/core/dependency_test.go internal/core/CLAUDE.md
git commit -m "feat(core): DependencyEditor (add/rm + cycle-check)

AddDependency/RemoveDependency mutate Task.DependsOn via TaskStore.Update
(no new port). Rejects self + cycles (DepGraph.Reaches), duplicate add is
idempotent, absent-edge remove errors. Updates core CLAUDE.md.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: adapter `depends_on` round-trip test

**Files:**
- Modify: `internal/adapter/yaml/task_dto_test.go` (add a round-trip test)

**Interfaces:**
- Consumes: `fromDomainTask`, `ymlTask.toDomain` (already exist; already map `DependsOn`).

- [ ] **Step 1: Read the existing test file** to match its style and confirm no duplicate exists.

Run: `sed -n '1,40p' internal/adapter/yaml/task_dto_test.go`
Expected: see the package/imports and the existing round-trip pattern.

- [ ] **Step 2: Write the failing test** — append to `internal/adapter/yaml/task_dto_test.go`:

```go
func TestTaskDTODependsOnRoundTrip(t *testing.T) {
	in := mtt.Task{
		ID: "t3", Type: "task", Title: "c", Status: "tbd",
		DependsOn: []mtt.TaskID{"t1", "t2"},
		Created:   time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC),
		Updated:   time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC),
	}
	out, err := fromDomainTask(in).toDomain()
	if err != nil {
		t.Fatal(err)
	}
	if len(out.DependsOn) != 2 || out.DependsOn[0] != "t1" || out.DependsOn[1] != "t2" {
		t.Fatalf("DependsOn round-trip = %v; want [t1 t2]", out.DependsOn)
	}
}
```

(If the file does not already import `time`/`mtt`, add them — check the existing imports first.)

- [ ] **Step 3: Run test to verify it passes** (it should pass immediately — the mapping already exists; this test *locks* it against regression):

Run: `go test ./internal/adapter/yaml/ -run TestTaskDTODependsOnRoundTrip -v`
Expected: PASS. (If it fails to compile for a missing import, add the import and re-run.)

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/yaml/task_dto_test.go
git commit -m "test(yaml): lock depends_on DTO round-trip

Confirms the edge round-trips through the adapter with no store change
(GAP #1: depends_on rides the Task field).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: CLI `mtt dep add/rm` + `dep list` (basic) + e2e

**Files:**
- Create: `internal/cli/dep.go`
- Create: `internal/cli/dep_test.go` (unit for `renderDepList`/`buildDepListJSON`)
- Create: `internal/cli/testdata/scripts/dep.txt`
- Modify: `internal/cli/root.go` (register `newDepCmd()`)

**Interfaces:**
- Consumes: `core.NewDependencyEditor` (Task 3), `core.NewDepGraph`/`Get`/`DependsOn`/`Dependents` (Task 2), `projectRoot`, `jsonFlag`, `writeJSON`, `taskLine`, `toTaskJSON`.
- Produces: `newDepCmd() *cobra.Command`; `renderDepList(g core.DepGraph, id mtt.TaskID) string`; `buildDepListJSON(g core.DepGraph, id mtt.TaskID) depListJSON`.

- [ ] **Step 1: Write the failing unit test** — `internal/cli/dep_test.go`:

```go
package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

func depTask(id mtt.TaskID, blockers ...mtt.TaskID) mtt.Task {
	return mtt.Task{ID: id, Type: "task", Status: "tbd", DependsOn: blockers,
		Created: time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC)}
}

func TestRenderDepList(t *testing.T) {
	g := core.NewDepGraph([]mtt.Task{depTask("t1"), depTask("t2", "t1", "ghost"), depTask("t3", "t2")})
	out := renderDepList(g, "t2")
	if !strings.Contains(out, "t2 depends on:") {
		t.Fatalf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "t1  task  [tbd]") {
		t.Fatalf("missing resolved blocker:\n%s", out)
	}
	if !strings.Contains(out, "ghost  (missing)") {
		t.Fatalf("missing dangling flag:\n%s", out)
	}
	if !strings.Contains(out, "required by:") || !strings.Contains(out, "t3  task  [tbd]") {
		t.Fatalf("missing dependents:\n%s", out)
	}
}

func TestBuildDepListJSONEmpty(t *testing.T) {
	g := core.NewDepGraph([]mtt.Task{depTask("t1")})
	v := buildDepListJSON(g, "t1")
	if v.DependsOn == nil || v.RequiredBy == nil {
		t.Fatalf("slices must be non-nil (marshal to [] not null): %+v", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run 'TestRenderDepList|TestBuildDepListJSON' -v`
Expected: FAIL — `undefined: renderDepList`.

- [ ] **Step 3: Write minimal implementation** — `internal/cli/dep.go`:

```go
package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newDepCmd builds `mtt dep` with add/rm/list subcommands.
func newDepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage blocking dependencies (depends_on)",
	}
	cmd.AddCommand(newDepAddCmd(), newDepRmCmd(), newDepListCmd())
	return cmd
}

func twoIDs(usage string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != 2 {
			return errors.New(usage)
		}
		return nil
	}
}

func oneID(usage string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New(usage)
		}
		return nil
	}
}

func newDepAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <id> <depends-on-id>",
		Short: "Add a blocking dependency",
		Args:  twoIDs("provide two task ids (example: mtt dep add t2 t1)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			id, dep := mtt.TaskID(args[0]), mtt.TaskID(args[1])
			task, err := core.NewDependencyEditor(yaml.NewTaskStore(root), time.Now).AddDependency(id, dep)
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "added %s to %s\n", dep, id)
			return err
		},
	}
}

func newDepRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id> <depends-on-id>",
		Short: "Remove a blocking dependency",
		Args:  twoIDs("provide two task ids (example: mtt dep rm t2 t1)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			id, dep := mtt.TaskID(args[0]), mtt.TaskID(args[1])
			task, err := core.NewDependencyEditor(yaml.NewTaskStore(root), time.Now).RemoveDependency(id, dep)
			if err != nil {
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed %s from %s\n", dep, id)
			return err
		},
	}
}

func newDepListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <id>",
		Short: "List a task's dependencies and dependents",
		Args:  oneID("provide exactly one task id (example: mtt dep list t2)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			g := core.NewDepGraph(tasks)
			id := mtt.TaskID(args[0])
			if _, ok := g.Get(id); !ok {
				return fmt.Errorf("task %q not found", id)
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), buildDepListJSON(g, id))
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderDepList(g, id))
			return err
		},
	}
	return cmd
}

// renderDepList renders a task's direct blockers ("depends on") and its computed
// dependents ("required by"). Dangling blockers are flagged (missing).
func renderDepList(g core.DepGraph, id mtt.TaskID) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s depends on:\n", id)
	deps := g.DependsOn(id)
	if len(deps) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, dep := range deps {
		if t, ok := g.Get(dep); ok {
			fmt.Fprintf(&b, "  %s\n", taskLine(t))
		} else {
			fmt.Fprintf(&b, "  %s  (missing)\n", dep)
		}
	}
	b.WriteString("required by:\n")
	dependents := g.Dependents(id)
	if len(dependents) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, t := range dependents {
		fmt.Fprintf(&b, "  %s\n", taskLine(t))
	}
	return b.String()
}

// depListJSON is the machine-readable view of `dep list <id>`.
type depListJSON struct {
	ID         string       `json:"id"`
	DependsOn  []depRefJSON `json:"depends_on"`
	RequiredBy []taskJSON   `json:"required_by"`
}

// depRefJSON is one blocker: the resolved task view, or an id flagged missing.
type depRefJSON struct {
	ID      string    `json:"id"`
	Missing bool      `json:"missing,omitempty"`
	Task    *taskJSON `json:"task,omitempty"`
}

// buildDepListJSON builds the flat dep-list view; slices are non-nil so an empty
// result marshals to [] (never null).
func buildDepListJSON(g core.DepGraph, id mtt.TaskID) depListJSON {
	out := depListJSON{ID: string(id), DependsOn: make([]depRefJSON, 0), RequiredBy: make([]taskJSON, 0)}
	for _, dep := range g.DependsOn(id) {
		entry := depRefJSON{ID: string(dep)}
		if t, ok := g.Get(dep); ok {
			v := toTaskJSON(t)
			entry.Task = &v
		} else {
			entry.Missing = true
		}
		out.DependsOn = append(out.DependsOn, entry)
	}
	for _, t := range g.Dependents(id) {
		out.RequiredBy = append(out.RequiredBy, toTaskJSON(t))
	}
	return out
}
```

- [ ] **Step 4: Register the command** — in `internal/cli/root.go`, add `newDepCmd()` to the `root.AddCommand(...)` call:

```go
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd(), newShowCmd(), newListCmd(), newEditCmd(), newTreeCmd(), newDepCmd())
```

- [ ] **Step 5: Run the unit test to verify it passes**

Run: `go test ./internal/cli/ -run 'TestRenderDepList|TestBuildDepListJSON' -v`
Expected: PASS.

- [ ] **Step 6: Write the e2e script** — `internal/cli/testdata/scripts/dep.txt`:

```
# dependencies: add / list / rm, self + cycle rejection
mkdir proj
cd proj
exec mtt init
stdout 'initialized'

exec mtt add --type epic 'E'
stdout 'created e1'
exec mtt add --type task --parent e1 'A'
stdout 'created t1'
exec mtt add --type task --parent e1 'B'
stdout 'created t2'

# t2 depends on t1
exec mtt dep add t2 t1
stdout 'added t1 to t2'

# dep list shows the blocker and the reverse dependent
exec mtt dep list t2
stdout 't2 depends on:'
stdout '  t1  task  \[tbd\]  A'
exec mtt dep list t1
stdout 'required by:'
stdout '  t2  task  \[tbd\]  B'

# a self-edge is rejected
! exec mtt dep add t1 t1
stderr 'cannot depend on itself'

# closing the loop (t1 -> t2) is rejected as a cycle
! exec mtt dep add t1 t2
stderr 'cycle'

# a duplicate add is a no-op (still succeeds)
exec mtt dep add t2 t1
stdout 'added t1 to t2'

# removing a non-existent edge errors
! exec mtt dep rm t1 t2
stderr 'does not depend'

# rm the real edge
exec mtt dep rm t2 t1
stdout 'removed t1 from t2'
exec mtt dep list t2
stdout '  \(none\)'

# --json for dep list emits non-null arrays
exec mtt dep list t1 --json
stdout '"depends_on": \[\]'
stdout '"required_by": \[\]'
```

- [ ] **Step 7: Run the e2e + full gate**

Run: `go test ./internal/cli/ -run TestScript -v` then `make check`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/dep.go internal/cli/dep_test.go internal/cli/root.go internal/cli/testdata/scripts/dep.txt
git commit -m "feat(cli): mtt dep add/rm/list

dep add/rm route through core.DependencyEditor (self/cycle rejected, dup
no-op, absent-edge rm errors); dep list renders depends-on + computed
required-by (dangling flagged) with a non-null --json. e2e dep.txt.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: CLI `dep list --tree` + `--cycles`

**Files:**
- Modify: `internal/cli/dep.go` (add `--tree`/`--cycles` flags, `renderDepTree`, `buildDepTreeJSON`, `writeDepCycles`)
- Modify: `internal/cli/dep_test.go` (unit for tree render + cycles render)
- Modify: `internal/cli/testdata/scripts/dep.txt` (append `--tree`/`--cycles` cases)

**Interfaces:**
- Consumes: `core.DepGraph.DependsOn`/`Get`/`Cycles` (Task 2), `taskLine`, `jsonFlag`, `writeJSON`.
- Produces: `renderDepTree(g core.DepGraph, id mtt.TaskID) string`; `buildDepTreeJSON(g core.DepGraph, id mtt.TaskID) depTreeJSON`; `writeDepCycles(cmd *cobra.Command, g core.DepGraph) error`.

- [ ] **Step 1: Write the failing unit test** — append to `internal/cli/dep_test.go`:

```go
func TestRenderDepTree(t *testing.T) {
	// t3 -> t2 -> t1  (transitive blockers of t3)
	g := core.NewDepGraph([]mtt.Task{depTask("t1"), depTask("t2", "t1"), depTask("t3", "t2")})
	out := renderDepTree(g, "t3")
	if !strings.Contains(out, "t3  task  [tbd]") ||
		!strings.Contains(out, "└─ t2  task  [tbd]") ||
		!strings.Contains(out, "t1  task  [tbd]") {
		t.Fatalf("transitive tree wrong:\n%s", out)
	}
}

func TestBuildDepTreeJSONNested(t *testing.T) {
	g := core.NewDepGraph([]mtt.Task{depTask("t1"), depTask("t2", "t1")})
	v := buildDepTreeJSON(g, "t2")
	if v.ID != "t2" || len(v.DependsOn) != 1 || v.DependsOn[0].ID != "t1" {
		t.Fatalf("nested tree json wrong: %+v", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run 'TestRenderDepTree|TestBuildDepTreeJSON' -v`
Expected: FAIL — `undefined: renderDepTree`.

- [ ] **Step 3: Add the flags and helpers** — in `internal/cli/dep.go`, replace `newDepListCmd` with the flagged version and add the new functions:

```go
func newDepListCmd() *cobra.Command {
	var (
		tree   bool
		cycles bool
	)
	cmd := &cobra.Command{
		Use:   "list <id>",
		Short: "List a task's dependencies and dependents",
		Args:  oneID("provide exactly one task id (example: mtt dep list t2)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			g := core.NewDepGraph(tasks)
			id := mtt.TaskID(args[0])
			if _, ok := g.Get(id); !ok {
				return fmt.Errorf("task %q not found", id)
			}
			if cycles {
				return writeDepCycles(cmd, g)
			}
			if jsonFlag(cmd) {
				if tree {
					return writeJSON(cmd.OutOrStdout(), buildDepTreeJSON(g, id))
				}
				return writeJSON(cmd.OutOrStdout(), buildDepListJSON(g, id))
			}
			if tree {
				_, err = fmt.Fprint(cmd.OutOrStdout(), renderDepTree(g, id))
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderDepList(g, id))
			return err
		},
	}
	cmd.Flags().BoolVar(&tree, "tree", false, "show the transitive dependency tree")
	cmd.Flags().BoolVar(&cycles, "cycles", false, "report dependency cycles project-wide")
	return cmd
}

// renderDepTree renders id's transitive blockers as an ASCII tree, cycle-safe.
func renderDepTree(g core.DepGraph, id mtt.TaskID) string {
	root, ok := g.Get(id)
	if !ok {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", taskLine(root))
	var walk func(cur mtt.TaskID, prefix string, seen map[mtt.TaskID]bool)
	walk = func(cur mtt.TaskID, prefix string, seen map[mtt.TaskID]bool) {
		deps := g.DependsOn(cur)
		for i, dep := range deps {
			last := i == len(deps)-1
			branch, childPrefix := "├─ ", prefix+"│  "
			if last {
				branch, childPrefix = "└─ ", prefix+"   "
			}
			if t, ok := g.Get(dep); ok {
				fmt.Fprintf(&b, "%s%s%s\n", prefix, branch, taskLine(t))
			} else {
				fmt.Fprintf(&b, "%s%s%s  (missing)\n", prefix, branch, dep)
			}
			if seen[dep] {
				continue // cycle guard (hand-broken data)
			}
			seen[dep] = true
			walk(dep, childPrefix, seen)
		}
	}
	walk(id, "", map[mtt.TaskID]bool{id: true})
	return b.String()
}

// depTreeJSON is the nested machine-readable transitive tree.
type depTreeJSON struct {
	taskJSON
	Missing   bool          `json:"missing,omitempty"`
	DependsOn []depTreeJSON `json:"depends_on,omitempty"`
}

// buildDepTreeJSON builds the nested transitive tree, cycle-safe.
func buildDepTreeJSON(g core.DepGraph, id mtt.TaskID) depTreeJSON {
	var build func(cur mtt.TaskID, seen map[mtt.TaskID]bool) depTreeJSON
	build = func(cur mtt.TaskID, seen map[mtt.TaskID]bool) depTreeJSON {
		node := depTreeJSON{}
		if t, ok := g.Get(cur); ok {
			node.taskJSON = toTaskJSON(t)
		} else {
			node.taskJSON = taskJSON{ID: string(cur)}
			node.Missing = true
			return node
		}
		for _, dep := range g.DependsOn(cur) {
			if seen[dep] {
				continue
			}
			seen[dep] = true
			node.DependsOn = append(node.DependsOn, build(dep, seen))
		}
		return node
	}
	return build(id, map[mtt.TaskID]bool{id: true})
}

// writeDepCycles reports the project's dependency cycles (or "no cycles").
func writeDepCycles(cmd *cobra.Command, g core.DepGraph) error {
	cycles := g.Cycles()
	if jsonFlag(cmd) {
		out := make([][]string, 0, len(cycles))
		for _, cyc := range cycles {
			chain := make([]string, len(cyc))
			for i, id := range cyc {
				chain[i] = string(id)
			}
			out = append(out, chain)
		}
		return writeJSON(cmd.OutOrStdout(), out)
	}
	if len(cycles) == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "no cycles")
		return err
	}
	var b strings.Builder
	for _, cyc := range cycles {
		chain := make([]string, len(cyc))
		for i, id := range cyc {
			chain[i] = string(id)
		}
		fmt.Fprintf(&b, "cycle: %s\n", strings.Join(chain, " -> "))
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), b.String())
	return err
}
```

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `go test ./internal/cli/ -run 'TestRenderDepTree|TestBuildDepTreeJSON' -v`
Expected: PASS.

- [ ] **Step 5: Append e2e cases** — add to the end of `internal/cli/testdata/scripts/dep.txt`:

```
# re-establish t2 -> t1 for tree/cycles checks
exec mtt dep add t2 t1
exec mtt add --type task --parent e1 'C'
stdout 'created t3'
exec mtt dep add t3 t2

# --tree shows the transitive chain t3 -> t2 -> t1
exec mtt dep list t3 --tree
stdout 't3  task  \[tbd\]  C'
stdout '└─ t2  task  \[tbd\]  B'
stdout 't1  task  \[tbd\]  A'

# --cycles on an acyclic graph reports none (the CLI cannot build a cycle)
exec mtt dep list t1 --cycles
stdout 'no cycles'
```

- [ ] **Step 6: Run the e2e + full gate**

Run: `go test ./internal/cli/ -run TestScript -v` then `make check`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/dep.go internal/cli/dep_test.go internal/cli/testdata/scripts/dep.txt
git commit -m "feat(cli): dep list --tree and --cycles

--tree renders the transitive depends_on chain (cycle-safe, nested
--json); --cycles reports project-wide cycles (defensive; the CLI's add
rejects cycles).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: CLI `mtt ready` + `list --ready` + cli `CLAUDE.md`

**Files:**
- Create: `internal/cli/ready.go`
- Create: `internal/cli/testdata/scripts/ready.txt`
- Modify: `internal/cli/list.go` (add `--ready` flag, gate through `core.Ready`)
- Modify: `internal/cli/root.go` (register `newReadyCmd()`)
- Modify: `internal/cli/CLAUDE.md`

**Interfaces:**
- Consumes: `core.Ready` (Task 1), `core.Select`, `core.ListFilter`, `parseKinds`, `taskJSON`/`toTaskJSON`, `writeList`, `writeJSON`, `jsonFlag`, `projectRoot`.
- Produces: `newReadyCmd() *cobra.Command`; a `--ready` bool on `list`.

- [ ] **Step 1: Write the e2e script (the primary test for this task)** — `internal/cli/testdata/scripts/ready.txt`:

```
# ready: a blocked task is excluded; blockers with no deps are ready
mkdir proj
cd proj
exec mtt init
stdout 'initialized'

exec mtt add --type epic 'E'
stdout 'created e1'
exec mtt add --type task --parent e1 'A'
stdout 'created t1'
exec mtt add --type task --parent e1 'B'
stdout 'created t2'

# no deps yet: e1, t1, t2 are all ready
exec mtt ready
stdout 'e1  epic  \[tbd\]  E'
stdout 't1  task  \[tbd\]  A'
stdout 't2  task  \[tbd\]  B'

# t2 depends on t1 (t1 is tbd, non-terminal) -> t2 is no longer ready
exec mtt dep add t2 t1
exec mtt ready
stdout 't1  task  \[tbd\]  A'
! stdout 't2  task  \[tbd\]  B'

# list --ready is the same subset; a --type filter still applies
exec mtt list --ready --type task
stdout 't1  task  \[tbd\]  A'
! stdout 't2  task'

# --json on ready is a (possibly empty) array, never null
exec mtt ready --type subtask --json
stdout '\[\]'
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run TestScript -v`
Expected: FAIL — `unknown command "ready"` / `unknown flag: --ready`.

- [ ] **Step 3: Write `mtt ready`** — `internal/cli/ready.go`:

```go
package cli

import (
	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newReadyCmd builds `mtt ready`: list actionable tasks (non-terminal, all
// blockers terminal). Accepts the list filters.
func newReadyCmd() *cobra.Command {
	var (
		statuses []string
		types    []string
		kinds    []string
		parent   string
	)
	cmd := &cobra.Command{
		Use:   "ready",
		Short: "List actionable tasks (no open blockers)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			kindVals, err := parseKinds(kinds)
			if err != nil {
				return err
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			cfg, _, err := yaml.Load(root)
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			filter := core.ListFilter{
				Statuses: toStatusNames(statuses), Types: toTypeNames(types),
				Kinds: kindVals, Parent: mtt.TaskID(parent),
			}
			selected := core.Select(core.Ready(tasks, cfg), filter, cfg)
			if jsonFlag(cmd) {
				views := make([]taskJSON, 0, len(selected))
				for _, t := range selected {
					views = append(views, toTaskJSON(t))
				}
				return writeJSON(cmd.OutOrStdout(), views)
			}
			return writeList(cmd.OutOrStdout(), selected)
		},
	}
	cmd.Flags().StringArrayVar(&statuses, "status", nil, "filter by status (repeatable)")
	cmd.Flags().StringArrayVar(&types, "type", nil, "filter by type (repeatable)")
	cmd.Flags().StringArrayVar(&kinds, "kind", nil, "filter by status category: initial|active|terminal (repeatable)")
	cmd.Flags().StringVar(&parent, "parent", "", "only direct children of this task id")
	return cmd
}

// toStatusNames / toTypeNames convert CLI string slices to typed identities.
func toStatusNames(ss []string) []mtt.StatusName {
	out := make([]mtt.StatusName, len(ss))
	for i, s := range ss {
		out[i] = mtt.StatusName(s)
	}
	return out
}

func toTypeNames(ss []string) []mtt.TypeName {
	out := make([]mtt.TypeName, len(ss))
	for i, s := range ss {
		out[i] = mtt.TypeName(s)
	}
	return out
}
```

- [ ] **Step 4: Refactor `list.go` to reuse the converters and add `--ready`** — in `internal/cli/list.go`: (a) replace the inline `typeNames`/`statusNames` construction with `toTypeNames(types)` / `toStatusNames(statuses)`; (b) add the flag and gate. The RunE body becomes:

```go
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			cfg, _, err := yaml.Load(root)
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			if ready {
				tasks = core.Ready(tasks, cfg)
			}
			selected := core.Select(tasks, core.ListFilter{
				Statuses: toStatusNames(statuses), Types: toTypeNames(types), Kinds: kindVals, Parent: mtt.TaskID(parent), Sort: core.SortKey(sortKey),
			}, cfg)
```

and add to the flag block and the `var` block:

```go
	var (
		statuses []string
		types    []string
		kinds    []string
		parent   string
		sortKey  string
		ready    bool
	)
```
```go
	cmd.Flags().BoolVar(&ready, "ready", false, "only tasks that are ready (no open blockers)")
```

- [ ] **Step 5: Register `mtt ready`** — in `internal/cli/root.go`:

```go
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd(), newShowCmd(), newListCmd(), newEditCmd(), newTreeCmd(), newDepCmd(), newReadyCmd())
```

- [ ] **Step 6: Run the e2e + full gate**

Run: `go test ./internal/cli/ -run TestScript -v` then `make check`
Expected: PASS.

- [ ] **Step 7: Update `internal/cli/CLAUDE.md`** — append to "Current state":

```markdown
Dependencies & ready (session 005): `dep add/rm <id> <dep-id>` route through `core.DependencyEditor`
(self/cycle rejected, duplicate no-op, absent-edge rm errors); `dep list <id>` builds `core.DepGraph` from
`TaskStore.List` and renders `depends on:` (dangling → `(missing)`) + computed `required by:`, with `--tree`
(transitive, cycle-safe), `--cycles` (project-wide, defensive), and a non-null `--json`. `mtt ready` and
`list --ready` share one primitive — `core.Select(core.Ready(tasks, cfg), filter, cfg)` — so readiness and
the list filters compose (AND). `toStatusNames`/`toTypeNames` are the shared string→identity converters for
`list`/`ready`. Pure reads (`dep list`/`ready`) call the store directly; mutations (`dep add/rm`) go through
`core`.
```

- [ ] **Step 8: Commit**

```bash
git add internal/cli/ready.go internal/cli/list.go internal/cli/root.go internal/cli/CLAUDE.md internal/cli/testdata/scripts/ready.txt
git commit -m "feat(cli): mtt ready + list --ready

One shared primitive Select(Ready(tasks,cfg), filter, cfg) backs both
mtt ready and list --ready. e2e ready.txt (blocking + filters + non-null
json). Updates cli CLAUDE.md.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Documentation sync (prose + snapshot + backlog + handoff)

**Files:**
- Modify: `DESIGN.md`, `DESIGN.ru.md` (expand "Dependencies"; add the `cancelled`-blocker deferred marker)
- Modify: `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md` (mark `dep`/`ready`/`list --ready` implemented in s005)
- Modify: `docs/architecture/model.go` (GAP #1, Ready/DependencyEditor notes, GAP #6)
- Modify: `TASKS.md` (tick e3_t2/t3/t4; add the `cancelled`-blocker marker under "Later")
- Modify: `sessions/005_dependencies.md` (fill "Done")
- Modify: `NEXT_SESSION.md` (handoff: done / next = s006 flow gate)

**Interfaces:** none (docs only). No code — this task's gate is `make check` (docs don't break the build) + a manual read.

- [ ] **Step 1: Update `DESIGN.md` → "Dependencies"** — replace that section's body with:

```markdown
## Dependencies

- `depends_on: [id, …]` is a **blocking** edge (distinct from the hierarchy `parent` and the informational
  `refs`). It rides the `Task` field and round-trips via `TaskStore.Update` — **no dedicated port** for the
  YAML reference (a `DependencyStore` capability exists only for external adapters that cannot embed the
  field). Adding an edge is rejected if it would create a **cycle** (a `core` rule), and a self-edge is
  rejected; the cycle-check builds a derived `core.DepGraph` over `depends_on` (parallel to the s004 `Index`
  over `parent`, kept separate — the two graphs have different shapes).
- A task is **ready** ⇔ its status is not `terminal` AND all `depends_on` are in a `terminal` status (by
  category `kind`: `done` or `cancelled`, not by the literal). Readiness is **conservative**: an
  unresolvable blocker (a dangling `depends_on`, or a status not in the current flow) leaves the task
  **not** ready — `ready` requires positive confirmation.
- `mtt ready` — "what can be picked up for work"; `mtt list --ready` is the shorthand companion (one shared
  `core.Ready` primitive backs both). `mtt dep list <id>` shows a task's direct blockers and its computed
  dependents, with `--tree` (transitive) and `--cycles` (project-wide, defensive).

> **Deferred design question — `cancelled` blocker semantics.** A `terminal` blocker unblocks its dependent,
> and `cancelled` is a terminal `kind`, so a task whose blockers are `done` **and** `cancelled` is formally
> ready — yet a *cancelled* (abandoned, not completed) blocker arguably means the dependent needs
> re-evaluation, not silent unblocking. Current behaviour keeps terminal-by-`kind` (cancelled unblocks);
> revisiting this (a succeeded-vs-abandoned distinction, a hard/soft edge, or a warning on `ready`) is
> deferred to flow enforcement (s006), where terminal statuses first become reachable. See TASKS.md → Later.
```

- [ ] **Step 2: Mirror the change in `DESIGN.ru.md`** — translate the same section (Russian mirror; keep it consistent with the English source of truth).

- [ ] **Step 3: Update `CLI_REFERENCE.md`** — in the "Dependencies" section, change the phase tag `*(phase 2; capability `DependencyStore`)*` to note s005 and mark the three subcommands implemented; add the `--missing`/dangling and `required by` output note; in `mtt ready` change `*(phase 2)*` to `*(session 005, implemented)*`; in `mtt list` change the `--ready` line from `*(later)*` to `*(implemented, session 005)*`. Mirror in `CLI_REFERENCE.ru.md`.

- [ ] **Step 4: Update `docs/architecture/model.go`** — (a) in GAP #1, append `RESOLVED (s005): accepted — DependencyEditor + Ready shipped, no port; the DependencyStore param was dropped from NewDependencyEditor (YAGNI).`; (b) change the `NewDependencyEditor` var signature to `func(store TaskStore, now func() time.Time) DependencyEditor` and update its doc comment; (c) in GAP #6 append `s005 landed the second graph (DepGraph over DependsOn) and kept it SEPARATE from Index — a shared primitive would be forced (single-parent tree vs multi-edge DAG); revisit if a third graph (flow, s006) shares it.`; (d) drop `[T1/s005]` "aspirational" wording on `Ready`/`DependencyEditor` to reflect shipped.

- [ ] **Step 5: Update `TASKS.md`** — tick `e3_t2`, `e3_t3`, `e3_t4` to `[x]` (add/remove + cycle detection + ready); under "Later (coarse)" add:

```markdown
- later — **`cancelled`-blocker semantics**: a `cancelled` (abandoned) `depends_on` currently unblocks its
  dependent (terminal by `kind`), which may be wrong — the dependent may need re-evaluation. Revisit with
  flow enforcement (s006), when terminal statuses become reachable. See DESIGN.md → "Dependencies".
```

- [ ] **Step 6: Fill `sessions/005_dependencies.md` → "Done"** — replace the placeholder with a summary of what shipped (core `Ready`/`DependencyEditor`/`DepGraph`; cli `dep add/rm/list` +`--tree`/`--cycles`, `ready`, `list --ready`; e2e `dep.txt`/`ready.txt`; the conservative-ready + no-port + graphs-separate decisions; the deferred `cancelled`-blocker marker) and set `Status: done`.

- [ ] **Step 7: Update `NEXT_SESSION.md`** — move s005 into the "done" narrative ("Where we are"); set next up = **session 006 (flow gate)**; carry forward any new lessons (e.g. conservative-ready; one `Ready` primitive for `ready`/`list --ready`; DepGraph kept separate from Index).

- [ ] **Step 8: Run the full gate**

Run: `make check`
Expected: PASS (docs-only; build/tests unaffected).

- [ ] **Step 9: Commit**

```bash
git add DESIGN.md DESIGN.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md docs/architecture/model.go TASKS.md sessions/005_dependencies.md NEXT_SESSION.md
git commit -m "docs(005): sync DESIGN/CLI_REFERENCE/model.go/TASKS/handoff

Dependencies section (conservative ready, no port, DepGraph separate) +
cancelled-blocker deferred marker; CLI_REFERENCE marks dep/ready/list
--ready implemented; model.go GAP #1/#6 resolved; TASKS ticks e3_t2-4;
session Done + NEXT_SESSION handoff to s006.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Final verification (after Task 8)

- [ ] `make check` green on the branch tip.
- [ ] Acceptance scenario (from the spec) runs by hand: `mtt init` → epic + two tasks → `dep add t2 t1` → `ready` lists e1,t1 not t2 → `dep add t1 t2` rejected (cycle) → `dep list t2` shows t1 → `list --ready` excludes t2.
- [ ] Push branch, open PR, CI green, squash-merge into `main`.

## Self-review notes (author check against the spec)

- **Spec coverage:** dep add/rm/list (Tasks 5–6) ✓; cycle rejection (Task 3 core + Task 5 e2e) ✓; conservative ready (Task 1) ✓; `mtt ready` + `list --ready` shared primitive (Task 7) ✓; `--tree`/`--cycles` (Task 6) ✓; no new port / GAP #1 (Tasks 3–4) ✓; GAP #6 separate (Task 2) ✓; `cancelled`-blocker deferred marker (Task 8) ✓; docs incl. bilingual mirrors + model.go (Task 8) ✓; e2e limitation (unit-covered unblock/cycles) — unblock in Task 1 test `TestReadyBlockerDoneUnblocks`, hand-built cycle in Task 2 `TestDepGraphCyclesHandBuilt` ✓.
- **Type consistency:** `Ready(tasks, cfg)`, `kindOf(t, cfg)`, `NewDepGraph(tasks)`, `DepGraph.{Get,DependsOn,Dependents,Reaches,Cycles}`, `NewDependencyEditor(store, now).{AddDependency,RemoveDependency}`, `renderDepList`/`buildDepListJSON`/`renderDepTree`/`buildDepTreeJSON`/`writeDepCycles`, `toStatusNames`/`toTypeNames` — names used identically across tasks.
- **No placeholders:** every code step shows full code; every test step shows the assertion and the run command with expected result.
