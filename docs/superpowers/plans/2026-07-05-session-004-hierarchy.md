# Session 004 — Hierarchy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give tasks a place in the tree — create a child under a parent (`mtt add --parent`), render the epic → task → subtask hierarchy (`mtt tree`), show lineage in `mtt show`, and add `list --parent`/`--kind` filters.

**Architecture:** Full hexagon `cli → core → port ← adapter`. `add --parent` is a mutation through `core.Adder`; `tree`/`show`/`list` are pure reads composing a derived `core.Index` (parent→children, ancestors; no store/clock) and a shared `core.Match` predicate. Children are computed, never stored. No new port method — `Create` already persists `Parent`.

**Tech Stack:** Go 1.23, cobra, `gopkg.in/yaml.v3`, `rogpeppe/go-internal/testscript` (e2e). Storage: YAML file-per-task under `.mtt/`.

**Authoritative spec:** [../specs/2026-07-04-session-004-hierarchy-design.md](../specs/2026-07-04-session-004-hierarchy-design.md).

## Global Constraints

- **TDD**: every task is red → green → refactor. Write the failing test first, watch it fail, then implement.
- **`make check` green before any commit** (gofmt + `go vet` + golangci-lint v2 + `go test -race -cover` + build). It is exactly what CI runs.
- **Layers**: `core` imports only `pkg/mtt`, never `adapter/*`. `pkg/mtt` carries no yaml/json tags and no adapter fields. CLI output only via `fmt.Fprint(cmd.OutOrStdout(), …)`; errors via `RunE`.
- **No name literals**: type/status names come from config; only the `StatusKind` value object (`initial`/`active`/`terminal`) is code-level vocabulary.
- **testscript**: anchor asserts (`e1  epic  \[tbd\]`, escape `\[ \]`); never assert sibling **order** against wall-clock timestamps (prove order in unit tests with a fixed clock).
- **golangci `unused`**: declare each new symbol in the task that first uses it.
- **Zero-match `--json` → `[]`, never `null`** (build slices with `make([]T, 0, …)`).
- **Preserve reserved task fields** on mutation (already handled by the round-trip DTO; do not regress).
- **Commit trailer** on every commit:
  `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

- `pkg/mtt/type_query.go` (**new**) — `Type.AcceptsParent`, `Type.StatusKind` (pure predicates) + `type_query_test.go`.
- `internal/core/index.go` (**new**) — `Index` (derived hierarchy) + shared `lessByRecency` comparator + `index_test.go`.
- `internal/core/list.go` (**modify**) — `ListFilter` gains `Parent`/`Kinds`; add `Match`; `Select` uses `Match` + `lessByRecency`, takes `cfg`.
- `internal/core/match_test.go` (**new**) — `Match` dimension tests.
- `internal/core/add.go` (**modify**) — `AddParams.Parent` + parent validation.
- `internal/core/add_test.go` (**modify**) — flexible `fakeStore.Get`, `cfg()` gains `subtask`, parent cases.
- `internal/core/list_test.go` (**modify**) — update `Select` call sites to the new signature.
- `internal/cli/add.go` (**modify**) — `--parent` flag, mutual exclusion, pass through.
- `internal/cli/format.go` (**new**) — shared `taskLine(t)` used by `list` and `tree`.
- `internal/cli/list.go` (**modify**) — load `cfg`, pass to `Select`; `writeList` uses `taskLine`; add `--parent`/`--kind` flags (Task 8).
- `internal/cli/tree.go` (**new**) — `mtt tree`, `renderTree`, keep-ancestors, depth, `parseKinds`, nested `--json`.
- `internal/cli/tree_test.go` (**new**) — `renderTree` unit tests (fixed clock).
- `internal/cli/show.go` (**modify**) — lineage breadcrumb.
- `internal/cli/root.go` (**modify**) — register `newTreeCmd()`; bump `version`.
- `internal/cli/testdata/scripts/{add_show.txt,tree.txt,list_edit.txt}` — e2e.
- Docs: `DESIGN.md`+`.ru`, `CLI_REFERENCE.md`+`.ru`, `internal/core/CLAUDE.md`, `internal/cli/CLAUDE.md`, `pkg/mtt/CLAUDE.md`, `sessions/004_hierarchy.md`, `NEXT_SESSION.md`.

---

## Task 1: Domain predicates (`pkg/mtt`)

**Files:**
- Create: `pkg/mtt/type_query.go`
- Test: `pkg/mtt/type_query_test.go`

**Interfaces:**
- Produces: `func (t Type) AcceptsParent(parentType string) bool`; `func (t Type) StatusKind(status string) (StatusKind, bool)`.

- [ ] **Step 1: Write the failing test**

Create `pkg/mtt/type_query_test.go`:

```go
package mtt

import "testing"

func typeQueryFixture() Type {
	return Type{
		Name:    "task",
		Parents: []string{"epic"},
		Flow: Flow{Statuses: []Status{
			{Name: "tbd", Kind: KindInitial},
			{Name: "doing", Kind: KindActive},
			{Name: "done", Kind: KindTerminal},
		}},
	}
}

func TestAcceptsParent(t *testing.T) {
	task := typeQueryFixture()
	if !task.AcceptsParent("epic") {
		t.Fatal("task should accept an epic parent")
	}
	if task.AcceptsParent("subtask") {
		t.Fatal("task must not accept a subtask parent")
	}
	epic := Type{Name: "epic"} // root: no parents
	if epic.AcceptsParent("epic") || epic.AcceptsParent("task") {
		t.Fatal("a root type accepts no parent")
	}
}

func TestStatusKind(t *testing.T) {
	task := typeQueryFixture()
	if k, ok := task.StatusKind("doing"); !ok || k != KindActive {
		t.Fatalf("StatusKind(doing) = %q,%v; want active,true", k, ok)
	}
	if _, ok := task.StatusKind("ghost"); ok {
		t.Fatal("unknown status must return ok=false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/mtt/ -run 'TestAcceptsParent|TestStatusKind' -v`
Expected: FAIL — `t.AcceptsParent undefined` / `t.StatusKind undefined` (build error).

- [ ] **Step 3: Write minimal implementation**

Create `pkg/mtt/type_query.go`:

```go
package mtt

// AcceptsParent reports whether a task of type t may sit under a parent whose
// type is named parentType — i.e. parentType is one of t.Parents. A root type
// (empty Parents) accepts no parent, so this also rejects giving an epic a parent.
func (t Type) AcceptsParent(parentType string) bool {
	for _, p := range t.Parents {
		if p == parentType {
			return true
		}
	}
	return false
}

// StatusKind returns the category of the named status within t's flow, or false
// when the status is not part of the flow (e.g. config drift on a stored task).
// Status identity is per-flow, so the lookup stays name-agnostic at the call site.
func (t Type) StatusKind(status string) (StatusKind, bool) {
	for _, s := range t.Statuses {
		if s.Name == status {
			return s.Kind, true
		}
	}
	return "", false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/mtt/ -run 'TestAcceptsParent|TestStatusKind' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/mtt/type_query.go pkg/mtt/type_query_test.go
git commit -m "feat(pkg/mtt): AcceptsParent + StatusKind pure predicates

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Core `Index` (derived hierarchy)

**Files:**
- Create: `internal/core/index.go`
- Test: `internal/core/index_test.go`

**Interfaces:**
- Consumes: `mtt.Task`, `core.SortKey`/`SortCreated`/`SortUpdated` (from `list.go`).
- Produces:
  - `func lessByRecency(a, b mtt.Task, key SortKey) bool` (package-internal; reused by `Select` in Task 3).
  - `type Index struct{}`; `func NewIndex(tasks []mtt.Task) Index`; `func (x Index) Get(id string) (mtt.Task, bool)`; `func (x Index) Roots() []mtt.Task`; `func (x Index) Children(id string) []mtt.Task`; `func (x Index) Ancestors(id string) []mtt.Task`.

- [ ] **Step 1: Write the failing test**

Create `internal/core/index_test.go`:

```go
package core

import (
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func node(id, parent string, created time.Time) mtt.Task {
	return mtt.Task{ID: id, Type: "task", Status: "tbd", Parent: parent, Created: created, Updated: created}
}

func TestIndexRootsChildrenAncestors(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		node("e1", "", base),
		node("t1", "e1", base.Add(2*time.Hour)), // newer
		node("t2", "e1", base.Add(time.Hour)),   // older
		node("s1", "t1", base),
		node("x1", "ghost", base), // orphan: parent absent
	}
	x := NewIndex(tasks)

	roots := x.Roots()
	if len(roots) != 2 {
		t.Fatalf("roots = %d, want 2 (e1 + orphan x1)", len(roots))
	}

	kids := x.Children("e1")
	if len(kids) != 2 || kids[0].ID != "t1" || kids[1].ID != "t2" {
		t.Fatalf("children(e1) = %v; want [t1 t2] (Created desc)", ids(kids))
	}

	anc := x.Ancestors("s1")
	if len(anc) != 2 || anc[0].ID != "e1" || anc[1].ID != "t1" {
		t.Fatalf("ancestors(s1) = %v; want [e1 t1] (root-first)", ids(anc))
	}
	if got := x.Ancestors("e1"); len(got) != 0 {
		t.Fatalf("ancestors(root) = %v; want empty", ids(got))
	}
}

func TestIndexAncestorsCycleSafe(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	// a <-> b mutually parent each other (hand-broken data).
	x := NewIndex([]mtt.Task{node("a", "b", base), node("b", "a", base)})
	if got := x.Ancestors("a"); len(got) > 2 {
		t.Fatalf("cycle walk did not terminate: %v", ids(got))
	}
}

func ids(ts []mtt.Task) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ -run 'TestIndex' -v`
Expected: FAIL — `NewIndex undefined` (build error).

- [ ] **Step 3: Write minimal implementation**

Create `internal/core/index.go`:

```go
package core

import (
	"sort"
	"strings"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// lessByRecency reports whether a should sort before b: the chosen timestamp
// descending (freshest first), tie-broken by ID as an opaque string. Shared by
// Select and the Index sibling order so both agree (never parses ID structure).
func lessByRecency(a, b mtt.Task, key SortKey) bool {
	ta, tb := a.Created, b.Created
	if key == SortUpdated {
		ta, tb = a.Updated, b.Updated
	}
	if !ta.Equal(tb) {
		return ta.After(tb)
	}
	return strings.Compare(a.ID, b.ID) < 0
}

// Index is a derived, read-only view of the parent→children hierarchy over a set
// of tasks. It is built once from a task slice (a pure value: no store, no clock)
// and is not part of the pkg/mtt contract — the resolved graph is derived.
// Children are computed (the inverse of Parent), never stored.
type Index struct {
	byID     map[string]mtt.Task
	children map[string][]mtt.Task // keyed by parent ID; roots live under key ""
}

// NewIndex builds the hierarchy index. A task with an empty Parent, or a Parent
// that does not resolve to a present task (an orphan), is treated as a root.
// Sibling buckets are ordered by lessByRecency (Created desc, ID tiebreak) so
// tree order matches Select.
func NewIndex(tasks []mtt.Task) Index {
	x := Index{
		byID:     make(map[string]mtt.Task, len(tasks)),
		children: make(map[string][]mtt.Task),
	}
	for _, t := range tasks {
		x.byID[t.ID] = t
	}
	for _, t := range tasks {
		key := t.Parent
		if key != "" {
			if _, ok := x.byID[key]; !ok {
				key = "" // orphan → root
			}
		}
		x.children[key] = append(x.children[key], t)
	}
	for k := range x.children {
		bucket := x.children[k]
		sort.SliceStable(bucket, func(i, j int) bool {
			return lessByRecency(bucket[i], bucket[j], SortCreated)
		})
	}
	return x
}

// Get returns the task with id, or false when absent.
func (x Index) Get(id string) (mtt.Task, bool) {
	t, ok := x.byID[id]
	return t, ok
}

// Roots returns the top-level tasks (no parent, or a dangling parent), in sibling order.
func (x Index) Roots() []mtt.Task { return x.children[""] }

// Children returns the direct children of id in sibling order (nil when none).
func (x Index) Children(id string) []mtt.Task { return x.children[id] }

// Ancestors returns id's parent chain from the outermost root down to the
// immediate parent (a breadcrumb, excluding id itself). Cycle-safe: a repeated
// id or a missing parent stops the walk.
func (x Index) Ancestors(id string) []mtt.Task {
	seen := map[string]bool{id: true}
	var chain []mtt.Task
	cur, ok := x.byID[id]
	for ok && cur.Parent != "" && !seen[cur.Parent] {
		seen[cur.Parent] = true
		parent, found := x.byID[cur.Parent]
		if !found {
			break
		}
		chain = append(chain, parent)
		cur = parent
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/ -run 'TestIndex' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/index.go internal/core/index_test.go
git commit -m "feat(core): Index — derived parent/children/ancestor hierarchy

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Core `Match` + `Select` refactor (`cfg`, shared comparator)

**Files:**
- Modify: `internal/core/list.go`
- Create: (tests) `internal/core/match_test.go`
- Modify: `internal/core/list_test.go` (update `Select` call sites)
- Modify: `internal/cli/list.go` (keep the caller compiling: load `cfg`, pass through)

**Interfaces:**
- Consumes: `lessByRecency` (Task 2), `mtt.Config`, `mtt.StatusKind`, `mtt.Type.StatusKind` (Task 1).
- Produces:
  - `type ListFilter struct{ Statuses, Types []string; Kinds []mtt.StatusKind; Parent string; Sort SortKey }`.
  - `func Match(t mtt.Task, f ListFilter, cfg mtt.Config) bool`.
  - `func Select(tasks []mtt.Task, f ListFilter, cfg mtt.Config) []mtt.Task` (**signature change**: adds `cfg`).

- [ ] **Step 1: Write the failing test**

Create `internal/core/match_test.go`:

```go
package core

import (
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func matchCfg() mtt.Config {
	return mtt.Config{Types: []mtt.Type{
		{Name: "task", Flow: mtt.Flow{Statuses: []mtt.Status{
			{Name: "tbd", Kind: mtt.KindInitial},
			{Name: "doing", Kind: mtt.KindActive},
			{Name: "done", Kind: mtt.KindTerminal},
		}}},
	}}
}

func TestMatchParent(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	child := mtt.Task{ID: "t1", Type: "task", Status: "tbd", Parent: "e1", Created: base}
	if !Match(child, ListFilter{Parent: "e1"}, mtt.Config{}) {
		t.Fatal("child of e1 should match Parent=e1")
	}
	if Match(child, ListFilter{Parent: "e2"}, mtt.Config{}) {
		t.Fatal("child of e1 must not match Parent=e2")
	}
}

func TestMatchKind(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	task := mtt.Task{ID: "t1", Type: "task", Status: "doing", Created: base}
	if !Match(task, ListFilter{Kinds: []mtt.StatusKind{mtt.KindActive}}, matchCfg()) {
		t.Fatal("doing is active — should match")
	}
	if Match(task, ListFilter{Kinds: []mtt.StatusKind{mtt.KindTerminal}}, matchCfg()) {
		t.Fatal("doing is not terminal — should not match")
	}
	// unknown type in cfg -> kind unresolved -> non-match
	ghost := mtt.Task{ID: "g1", Type: "ghost", Status: "doing", Created: base}
	if Match(ghost, ListFilter{Kinds: []mtt.StatusKind{mtt.KindActive}}, matchCfg()) {
		t.Fatal("unknown type must fail a kind filter")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ -run 'TestMatch' -v`
Expected: FAIL — `Match undefined` / too few arguments (build error).

- [ ] **Step 3: Rewrite `internal/core/list.go`**

Replace the whole file with:

```go
package core

import (
	"sort"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// SortKey selects the list ordering. Ordering is always descending on the chosen
// timestamp (freshest first), tie-broken by ID for determinism.
type SortKey string

// The supported sort keys. An empty key defaults to SortCreated.
const (
	SortCreated SortKey = "created"
	SortUpdated SortKey = "updated"
)

// ListFilter holds the list predicates and ordering. Empty slices/zero Parent
// match everything; within a field the values are OR-ed, across fields AND-ed.
type ListFilter struct {
	Statuses []string
	Types    []string
	Kinds    []mtt.StatusKind
	Parent   string
	Sort     SortKey
}

// Match reports whether t satisfies f. Within a dimension the values are OR-ed;
// across dimensions AND-ed. cfg is consulted only for the Kinds dimension (to
// resolve t's status category via its type's flow); a task whose type or status
// is unknown to cfg fails a non-empty Kinds filter. Shared by Select and tree.
func Match(t mtt.Task, f ListFilter, cfg mtt.Config) bool {
	if !anyOrEmpty(f.Statuses, t.Status) || !anyOrEmpty(f.Types, t.Type) {
		return false
	}
	if f.Parent != "" && t.Parent != f.Parent {
		return false
	}
	if len(f.Kinds) > 0 && !matchesKind(t, f.Kinds, cfg) {
		return false
	}
	return true
}

func matchesKind(t mtt.Task, kinds []mtt.StatusKind, cfg mtt.Config) bool {
	typ, ok := cfg.TypeByName(t.Type)
	if !ok {
		return false
	}
	k, ok := typ.StatusKind(t.Status)
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

// Select returns the tasks matching f in a deterministic order, without mutating
// the input. Order: the chosen timestamp descending, tie-broken by ID as an
// opaque string, so equal timestamps never reorder between runs. Select never
// interprets ID structure, so it stays provider-agnostic.
func Select(tasks []mtt.Task, f ListFilter, cfg mtt.Config) []mtt.Task {
	out := make([]mtt.Task, 0, len(tasks))
	for _, t := range tasks {
		if Match(t, f, cfg) {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return lessByRecency(out[i], out[j], f.Sort)
	})
	return out
}

// anyOrEmpty reports whether values is empty (match everything) or contains v.
func anyOrEmpty(values []string, v string) bool {
	if len(values) == 0 {
		return true
	}
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Update existing `Select` call sites in `internal/core/list_test.go`**

Every `Select(tasks, ListFilter{…})` call becomes `Select(tasks, ListFilter{…}, mtt.Config{})`. The five calls are on lines 21, 24, 27, 31 (`TestSelectFilters`), 39 (`TestSelectOrderCreatedDesc`), 47 (`TestSelectTieBreakByIDString`), 64 (`TestSelectSortUpdated`), 73 (`TestSelectDoesNotMutateInput`). Add the trailing `, mtt.Config{}` argument to each. Example:

```go
	if got := Select(tasks, ListFilter{}, mtt.Config{}); len(got) != 3 {
```

- [ ] **Step 5: Keep the CLI caller compiling — `internal/cli/list.go`**

Load config and pass it to `Select` (no new flags yet — those land in Task 8). In the `RunE`, after `root, err := projectRoot(cmd)`:

```go
			cfg, _, err := yaml.Load(root)
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			selected := core.Select(tasks, core.ListFilter{
				Statuses: statuses, Types: types, Sort: core.SortKey(sortKey),
			}, cfg)
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/core/ ./internal/cli/ -run 'TestMatch|TestSelect|TestScripts' -v`
Expected: PASS (Match tests pass; existing Select tests still green under the new signature; `list_edit.txt` e2e unaffected).

- [ ] **Step 7: `make check`, then commit**

```bash
make check
git add internal/core/list.go internal/core/list_test.go internal/core/match_test.go internal/cli/list.go
git commit -m "refactor(core): shared Match predicate; Select takes cfg (Kinds/Parent filters)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: `Adder` parent validation

**Files:**
- Modify: `internal/core/add.go`
- Modify: `internal/core/add_test.go`

**Interfaces:**
- Consumes: `mtt.Type.AcceptsParent` (Task 1), `mtt.TaskStore.Get`, `mtt.ErrNotFound`.
- Produces: `AddParams` gains `Parent string`; `Adder.Add` sets `Task.Parent` and validates the parent.

- [ ] **Step 1: Make `fakeStore.Get` lookup-based and extend `cfg()` in `internal/core/add_test.go`**

Replace the `fakeStore` type + its `Get`, and the `cfg()` helper, at the top of `add_test.go`:

```go
type fakeStore struct {
	got   mtt.Task
	retID string
	byID  map[string]mtt.Task
}

func (f *fakeStore) Create(t mtt.Task) (mtt.Task, error) {
	f.got = t
	t.ID = f.retID
	return t, nil
}
func (f *fakeStore) Get(id string) (mtt.Task, error) {
	if t, ok := f.byID[id]; ok {
		return t, nil
	}
	return mtt.Task{}, mtt.ErrNotFound
}
func (f *fakeStore) List() ([]mtt.Task, error) { return nil, nil }
func (f *fakeStore) Update(t mtt.Task) (mtt.Task, error) {
	f.got = t
	return t, nil
}
```

And extend `cfg()` to include `subtask` (parents `[task]`):

```go
func cfg() mtt.Config {
	return mtt.Config{Types: []mtt.Type{
		{Name: "epic", Flow: flow()},
		{Name: "task", Parents: []string{"epic"}, Default: true, Flow: flow()},
		{Name: "subtask", Parents: []string{"task"}, Flow: flow()},
	}}
}
```

- [ ] **Step 2: Write the failing tests (append to `add_test.go`)**

```go
func TestAddUnderParentOK(t *testing.T) {
	fs := &fakeStore{retID: "t1", byID: map[string]mtt.Task{"e1": {ID: "e1", Type: "epic"}}}
	got, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "child", TypeName: "task", Parent: "e1"})
	if err != nil {
		t.Fatalf("valid parent should succeed: %v", err)
	}
	if got.ID != "t1" || fs.got.Parent != "e1" {
		t.Fatalf("parent not set: id=%q parent=%q", got.ID, fs.got.Parent)
	}
}

func TestAddParentMissing(t *testing.T) {
	fs := &fakeStore{retID: "t1", byID: map[string]mtt.Task{}}
	_, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "x", TypeName: "task", Parent: "e9"})
	if err == nil || !strings.Contains(err.Error(), `parent "e9" not found`) {
		t.Fatalf("want parent-not-found, got %v", err)
	}
}

func TestAddParentWrongType(t *testing.T) {
	// subtask.parents = [task]; placing it under an epic must fail.
	fs := &fakeStore{retID: "s1", byID: map[string]mtt.Task{"e1": {ID: "e1", Type: "epic"}}}
	_, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "x", TypeName: "subtask", Parent: "e1"})
	if err == nil || !strings.Contains(err.Error(), "cannot be placed under") {
		t.Fatalf("want placement error, got %v", err)
	}
}

func TestAddRootTypeRejectsParent(t *testing.T) {
	// epic is a root type; giving it a parent must fail.
	fs := &fakeStore{retID: "e2", byID: map[string]mtt.Task{"e1": {ID: "e1", Type: "epic"}}}
	_, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "x", TypeName: "epic", Parent: "e1"})
	if err == nil || !strings.Contains(err.Error(), "cannot be placed under") {
		t.Fatalf("want placement error for root+parent, got %v", err)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/core/ -run 'TestAddUnderParent|TestAddParent|TestAddRootType' -v`
Expected: FAIL — `unknown field Parent in struct literal` (build error).

- [ ] **Step 4: Implement in `internal/core/add.go`**

Add `"errors"` to the imports. Add `Parent string` to `AddParams`:

```go
type AddParams struct {
	Title       string
	TypeName    string
	Parent      string
	NoParent    bool
	Description string
}
```

Replace the current placement block (the `if !typ.IsRoot() && !p.NoParent { … }` lines) with:

```go
	parent := ""
	switch {
	case p.Parent != "":
		pt, err := a.store.Get(p.Parent)
		if err != nil {
			if errors.Is(err, mtt.ErrNotFound) {
				return mtt.Task{}, fmt.Errorf("parent %q not found", p.Parent)
			}
			return mtt.Task{}, fmt.Errorf("load parent %q: %w", p.Parent, err)
		}
		if !typ.AcceptsParent(pt.Type) {
			return mtt.Task{}, fmt.Errorf("type %q cannot be placed under type %q (allowed parents: %v)", typ.Name, pt.Type, typ.Parents)
		}
		parent = pt.ID
	case !typ.IsRoot() && !p.NoParent:
		return mtt.Task{}, fmt.Errorf("type %q requires a parent; use --parent <id> (or --no-parent to create it at the top level)", typ.Name)
	}
```

Set the parent in the `Create` call by adding the `Parent` field:

```go
	return a.store.Create(mtt.Task{
		Type:        typ.Name,
		Title:       p.Title,
		Status:      initial.Name,
		Parent:      parent,
		Description: p.Description,
		Created:     now,
		Updated:     now,
	})
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/core/ -run 'TestAdd' -v`
Expected: PASS (new parent tests + existing add tests — the "requires a parent" message still contains that substring).

- [ ] **Step 6: Commit**

```bash
git add internal/core/add.go internal/core/add_test.go
git commit -m "feat(core): Adder validates --parent (exists + type allowed)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: CLI `add --parent`

**Files:**
- Modify: `internal/cli/add.go`
- Modify: `internal/cli/testdata/scripts/add_show.txt`

**Interfaces:**
- Consumes: `core.AddParams.Parent` (Task 4).
- Produces: `mtt add --parent <id>`; mutually exclusive with `--no-parent`.

- [ ] **Step 1: Extend the e2e script `add_show.txt`**

After the existing block that creates `e1`/`e2` and before "outside a project", insert (place it after line 34, the `show t2` block):

```
# --parent places a task under an existing epic -> t3
exec mtt add --type task --parent e1 'child of e1'
stdout 'created t3'
exec mtt show t3
stdout 't3  task  \[tbd\]'
stdout 'parent:   e1'

# a missing parent errors
! exec mtt add --type task --parent nope 'x'
stderr 'parent "nope" not found'

# a disallowed parent type errors (epic cannot sit under an epic)
! exec mtt add --type epic --parent e1 'x'
stderr 'cannot be placed under'

# --parent and --no-parent are mutually exclusive
! exec mtt add --type task --parent e1 --no-parent 'x'
stderr 'if any flags in the group'
```

(Note: `t1`/`t2` were created earlier with `--no-parent`, so the parent-carrying task mints as `t3`.)

- [ ] **Step 2: Run the e2e to verify it fails**

Run: `go test ./internal/cli/ -run 'TestScripts/add_show' -v`
Expected: FAIL — `unknown flag: --parent`.

- [ ] **Step 3: Implement the flag in `internal/cli/add.go`**

Add a `parent` variable, its flag, and mutual exclusion; thread it into `AddParams`:

```go
	var (
		typeName string
		parent   string
		noParent bool
		desc     string
	)
```

In the flag section (after the `--type` flag, before `--no-parent`):

```go
	cmd.Flags().StringVar(&parent, "parent", "", "place under an existing parent task (by id)")
	cmd.Flags().BoolVar(&noParent, "no-parent", false, "create a parent-requiring type at top level (conscious exception)")
	cmd.Flags().StringVar(&desc, "description", "", "task description")
	cmd.MarkFlagsMutuallyExclusive("parent", "no-parent")
	return cmd
```

And pass it through:

```go
			task, err := adder.Add(core.AddParams{Title: title, TypeName: typeName, Parent: parent, NoParent: noParent, Description: desc})
```

- [ ] **Step 4: Run the e2e to verify it passes**

Run: `go test ./internal/cli/ -run 'TestScripts/add_show' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/add.go internal/cli/testdata/scripts/add_show.txt
git commit -m "feat(cli): mtt add --parent (mutually exclusive with --no-parent)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: CLI `mtt tree` (human render, keep-ancestors, depth)

**Files:**
- Create: `internal/cli/format.go` (shared `taskLine`)
- Modify: `internal/cli/list.go` (use `taskLine` in `writeList`)
- Create: `internal/cli/tree.go`
- Create: `internal/cli/tree_test.go`
- Modify: `internal/cli/root.go` (register `newTreeCmd`)
- Create: `internal/cli/testdata/scripts/tree.txt`

**Interfaces:**
- Consumes: `core.Index`, `core.NewIndex`, `core.Match`, `core.ListFilter` (Tasks 2–3); `mtt.Task`, `mtt.StatusKind` + `Valid` (Task 1 / kind.go).
- Produces:
  - `func taskLine(t mtt.Task) string` — `"<id>  <type>  [<status>]  <title>"` (title omitted when empty).
  - `func parseKinds(vals []string) ([]mtt.StatusKind, error)` (reused by `list` in Task 8).
  - `func renderTree(x core.Index, roots []mtt.Task, f core.ListFilter, cfg mtt.Config, maxDepth int) string`.
  - `func newTreeCmd() *cobra.Command`.

- [ ] **Step 1: Write the failing unit test — `internal/cli/tree_test.go`**

```go
package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

func treeCfg() mtt.Config {
	return mtt.Config{Types: []mtt.Type{
		{Name: "epic", Flow: mtt.Flow{Statuses: []mtt.Status{
			{Name: "tbd", Kind: mtt.KindInitial}, {Name: "doing", Kind: mtt.KindActive}, {Name: "done", Kind: mtt.KindTerminal},
		}}},
		{Name: "task", Parents: []string{"epic"}, Flow: mtt.Flow{Statuses: []mtt.Status{
			{Name: "tbd", Kind: mtt.KindInitial}, {Name: "doing", Kind: mtt.KindActive}, {Name: "done", Kind: mtt.KindTerminal},
		}}},
	}}
}

func treeTasks() []mtt.Task {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	return []mtt.Task{
		{ID: "e1", Type: "epic", Status: "doing", Title: "E", Created: base},
		{ID: "t1", Type: "task", Status: "done", Title: "T1", Parent: "e1", Created: base.Add(2 * time.Hour)},
		{ID: "t2", Type: "task", Status: "tbd", Title: "T2", Parent: "e1", Created: base.Add(time.Hour)},
	}
}

func TestRenderTreeFull(t *testing.T) {
	x := core.NewIndex(treeTasks())
	out := renderTree(x, x.Roots(), core.ListFilter{}, treeCfg(), 0)
	for _, want := range []string{"e1  epic  [doing]  E", "t1  task  [done]  T1", "t2  task  [tbd]  T2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	// sibling order is deterministic: t1 (newer) before t2 (older)
	if strings.Index(out, "t1") > strings.Index(out, "t2") {
		t.Fatalf("sibling order wrong (want t1 before t2):\n%s", out)
	}
}

func TestRenderTreeKeepAncestors(t *testing.T) {
	// filter status=done: only t1 matches, but e1 (its non-matching parent) is kept as the path.
	x := core.NewIndex(treeTasks())
	out := renderTree(x, x.Roots(), core.ListFilter{Statuses: []string{"done"}}, treeCfg(), 0)
	if !strings.Contains(out, "e1  epic") {
		t.Fatalf("keep-ancestors: e1 should remain as path to t1:\n%s", out)
	}
	if !strings.Contains(out, "t1  task  [done]") {
		t.Fatalf("keep-ancestors: matching t1 should show:\n%s", out)
	}
	if strings.Contains(out, "t2") {
		t.Fatalf("keep-ancestors: non-matching t2 with no matching descendant should be dropped:\n%s", out)
	}
}

func TestRenderTreeDepth(t *testing.T) {
	x := core.NewIndex(treeTasks())
	out := renderTree(x, x.Roots(), core.ListFilter{}, treeCfg(), 1) // roots only
	if !strings.Contains(out, "e1  epic") {
		t.Fatalf("depth 1 should show the root:\n%s", out)
	}
	if strings.Contains(out, "t1") || strings.Contains(out, "t2") {
		t.Fatalf("depth 1 must not show children:\n%s", out)
	}
}
```

- [ ] **Step 2: Run the unit test to verify it fails**

Run: `go test ./internal/cli/ -run 'TestRenderTree' -v`
Expected: FAIL — `renderTree undefined` (build error).

- [ ] **Step 3: Create the shared `taskLine` — `internal/cli/format.go`**

```go
package cli

import (
	"fmt"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// taskLine renders a task as one compact row: "<id>  <type>  [<status>]  <title>"
// (the title is omitted when empty). Shared by `list` and `tree` so both agree.
func taskLine(t mtt.Task) string {
	s := fmt.Sprintf("%s  %s  [%s]", t.ID, t.Type, t.Status)
	if t.Title != "" {
		s += "  " + t.Title
	}
	return s
}
```

- [ ] **Step 4: Use `taskLine` in `writeList` (`internal/cli/list.go`)**

Replace the body of `writeList`'s loop so it reuses `taskLine` (drop the inline formatting):

```go
func writeList(w io.Writer, tasks []mtt.Task) error {
	var b strings.Builder
	for _, t := range tasks {
		b.WriteString(taskLine(t))
		b.WriteString("\n")
	}
	_, err := fmt.Fprint(w, b.String())
	return err
}
```

- [ ] **Step 5: Create `internal/cli/tree.go`**

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

// newTreeCmd builds `mtt tree [id]`: render the epic → task → subtask hierarchy.
func newTreeCmd() *cobra.Command {
	var (
		statuses []string
		kinds    []string
		depth    int
	)
	cmd := &cobra.Command{
		Use:   "tree [id]",
		Short: "Show the task hierarchy",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return errors.New("provide at most one task id (example: mtt tree e1)")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
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
			idx := core.NewIndex(tasks)
			var roots []mtt.Task
			if len(args) == 1 {
				t, ok := idx.Get(args[0])
				if !ok {
					return fmt.Errorf("task %q not found", args[0])
				}
				roots = []mtt.Task{t}
			} else {
				roots = idx.Roots()
			}
			f := core.ListFilter{Statuses: statuses, Kinds: kindVals}
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderTree(idx, roots, f, cfg, depth))
			return err
		},
	}
	cmd.Flags().StringArrayVar(&statuses, "status", nil, "filter by status (repeatable)")
	cmd.Flags().StringArrayVar(&kinds, "kind", nil, "filter by status category: initial|active|terminal (repeatable)")
	cmd.Flags().IntVar(&depth, "depth", 0, "limit visible levels (0 = unlimited)")
	return cmd
}

// parseKinds validates the --kind values against the closed StatusKind vocabulary.
func parseKinds(vals []string) ([]mtt.StatusKind, error) {
	if len(vals) == 0 {
		return nil, nil
	}
	out := make([]mtt.StatusKind, 0, len(vals))
	for _, v := range vals {
		k := mtt.StatusKind(v)
		if !k.Valid() {
			return nil, fmt.Errorf("invalid --kind %q: want initial|active|terminal", v)
		}
		out = append(out, k)
	}
	return out, nil
}

// renderTree renders the forest rooted at roots as an ASCII tree. With a filter,
// keep-ancestors semantics apply: a node shows iff it matches or any descendant
// matches (non-matching ancestors remain as the path). maxDepth <= 0 is
// unlimited; maxDepth n shows n levels below (and including) each root.
func renderTree(x core.Index, roots []mtt.Task, f core.ListFilter, cfg mtt.Config, maxDepth int) string {
	keep := map[string]bool{}
	for _, r := range roots {
		markVisible(x, r.ID, f, cfg, keep, map[string]bool{})
	}
	var b strings.Builder
	var walk func(t mtt.Task, prefix string, isLast, root bool, level int, seen map[string]bool)
	walk = func(t mtt.Task, prefix string, isLast, root bool, level int, seen map[string]bool) {
		if !keep[t.ID] || seen[t.ID] {
			return
		}
		seen[t.ID] = true
		if root {
			fmt.Fprintf(&b, "%s\n", taskLine(t))
		} else {
			branch := "├─ "
			if isLast {
				branch = "└─ "
			}
			fmt.Fprintf(&b, "%s%s%s\n", prefix, branch, taskLine(t))
		}
		if maxDepth > 0 && level+1 > maxDepth {
			return
		}
		kids := visibleChildren(x, t.ID, keep)
		childPrefix := prefix
		if !root {
			if isLast {
				childPrefix += "   "
			} else {
				childPrefix += "│  "
			}
		}
		for i, c := range kids {
			walk(c, childPrefix, i == len(kids)-1, false, level+1, seen)
		}
	}
	for _, r := range roots {
		walk(r, "", true, true, 1, map[string]bool{})
	}
	return b.String()
}

// markVisible memoizes into keep whether id should appear: it matches the filter
// or some descendant does. seen guards against cycles in hand-broken data.
func markVisible(x core.Index, id string, f core.ListFilter, cfg mtt.Config, keep, seen map[string]bool) bool {
	if seen[id] {
		return keep[id]
	}
	seen[id] = true
	t, ok := x.Get(id)
	visible := ok && core.Match(t, f, cfg)
	for _, c := range x.Children(id) {
		if markVisible(x, c.ID, f, cfg, keep, seen) {
			visible = true
		}
	}
	keep[id] = visible
	return visible
}

// visibleChildren returns id's direct children that survive the keep set.
func visibleChildren(x core.Index, id string, keep map[string]bool) []mtt.Task {
	all := x.Children(id)
	out := make([]mtt.Task, 0, len(all))
	for _, c := range all {
		if keep[c.ID] {
			out = append(out, c)
		}
	}
	return out
}
```

- [ ] **Step 6: Register the command — `internal/cli/root.go`**

Add `newTreeCmd()` to the `AddCommand` call:

```go
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd(), newShowCmd(), newListCmd(), newEditCmd(), newTreeCmd())
```

- [ ] **Step 7: Run the unit tests to verify they pass**

Run: `go test ./internal/cli/ -run 'TestRenderTree' -v`
Expected: PASS.

- [ ] **Step 8: Create the e2e script `internal/cli/testdata/scripts/tree.txt`**

```
# build an epic -> task -> subtask chain and render it
mkdir proj
cd proj
exec mtt init
stdout 'initialized'

exec mtt add --type epic 'E'
stdout 'created e1'
exec mtt add --type task --parent e1 'T'
stdout 'created t1'
exec mtt add --type subtask --parent t1 'S'
stdout 'created s1'

# tree renders all three nodes (presence, not order)
exec mtt tree
stdout 'e1  epic  \[tbd\]  E'
stdout 't1  task  \[tbd\]  T'
stdout 's1  subtask  \[tbd\]  S'

# rooting at t1 shows t1 + s1 but not e1
exec mtt tree t1
stdout 't1  task  \[tbd\]  T'
stdout 's1  subtask'
! stdout 'e1  epic'

# depth 1 from the root shows only e1
exec mtt tree --depth 1
stdout 'e1  epic'
! stdout 't1  task'

# a missing root errors
! exec mtt tree nope
stderr 'task "nope" not found'

# an invalid --kind errors
! exec mtt tree --kind bogus
stderr 'invalid --kind'
```

- [ ] **Step 9: Run the e2e + `make check`**

Run: `go test ./internal/cli/ -run 'TestScripts/tree' -v && make check`
Expected: PASS; `make check` green.

- [ ] **Step 10: Commit**

```bash
git add internal/cli/format.go internal/cli/list.go internal/cli/tree.go internal/cli/tree_test.go internal/cli/root.go internal/cli/testdata/scripts/tree.txt
git commit -m "feat(cli): mtt tree — ASCII hierarchy with keep-ancestors + --depth

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: `tree --json` (nested tree)

**Files:**
- Modify: `internal/cli/tree.go` (add `--json` branch + `buildTreeJSON`)
- Modify: `internal/cli/tree_test.go` (JSON unit test)
- Modify: `internal/cli/testdata/scripts/tree.txt` (JSON e2e)

**Interfaces:**
- Consumes: `taskJSON`, `toTaskJSON`, `writeJSON`, `jsonFlag` (existing `json.go`); `keep`/`markVisible`/`visibleChildren` (Task 6).
- Produces: `type treeNodeJSON struct{ taskJSON; Children []treeNodeJSON }`; `func buildTreeJSON(x core.Index, roots []mtt.Task, f core.ListFilter, cfg mtt.Config, maxDepth int) []treeNodeJSON`.

- [ ] **Step 1: Write the failing unit test (append to `tree_test.go`)**

```go
func TestBuildTreeJSONNested(t *testing.T) {
	x := core.NewIndex(treeTasks())
	nodes := buildTreeJSON(x, x.Roots(), core.ListFilter{}, treeCfg(), 0)
	if len(nodes) != 1 || nodes[0].ID != "e1" {
		t.Fatalf("want one root e1, got %+v", nodes)
	}
	if len(nodes[0].Children) != 2 || nodes[0].Children[0].ID != "t1" {
		t.Fatalf("want e1 -> [t1 t2], got %+v", nodes[0].Children)
	}
	if len(nodes[0].Children[0].Children) != 0 {
		t.Fatalf("t1 is a leaf: children should be empty")
	}
}

func TestBuildTreeJSONEmptyIsSlice(t *testing.T) {
	x := core.NewIndex(nil)
	if nodes := buildTreeJSON(x, x.Roots(), core.ListFilter{}, treeCfg(), 0); nodes == nil {
		t.Fatal("empty tree must be a non-nil slice so it marshals to [] not null")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run 'TestBuildTreeJSON' -v`
Expected: FAIL — `buildTreeJSON undefined`.

- [ ] **Step 3: Implement in `internal/cli/tree.go`**

Add the type and builder (below `renderTree`):

```go
// treeNodeJSON is the nested JSON shape for `tree --json`: a task view plus its
// (filtered) children. Empty children are omitted so leaves stay clean.
type treeNodeJSON struct {
	taskJSON
	Children []treeNodeJSON `json:"children,omitempty"`
}

// buildTreeJSON builds the nested JSON forest, honoring the same keep-ancestors
// filter and depth as renderTree. The top level is always a non-nil slice so an
// empty result marshals to [] (never null).
func buildTreeJSON(x core.Index, roots []mtt.Task, f core.ListFilter, cfg mtt.Config, maxDepth int) []treeNodeJSON {
	keep := map[string]bool{}
	for _, r := range roots {
		markVisible(x, r.ID, f, cfg, keep, map[string]bool{})
	}
	var build func(t mtt.Task, level int, seen map[string]bool) treeNodeJSON
	build = func(t mtt.Task, level int, seen map[string]bool) treeNodeJSON {
		node := treeNodeJSON{taskJSON: toTaskJSON(t)}
		seen[t.ID] = true
		if maxDepth > 0 && level+1 > maxDepth {
			return node
		}
		for _, c := range visibleChildren(x, t.ID, keep) {
			if seen[c.ID] {
				continue
			}
			node.Children = append(node.Children, build(c, level+1, seen))
		}
		return node
	}
	out := make([]treeNodeJSON, 0, len(roots))
	for _, r := range roots {
		if keep[r.ID] {
			out = append(out, build(r, 1, map[string]bool{}))
		}
	}
	return out
}
```

Wire the `--json` branch in `RunE`, replacing the single `renderTree` print with:

```go
			f := core.ListFilter{Statuses: statuses, Kinds: kindVals}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), buildTreeJSON(idx, roots, f, cfg, depth))
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderTree(idx, roots, f, cfg, depth))
			return err
```

- [ ] **Step 4: Run the unit test to verify it passes**

Run: `go test ./internal/cli/ -run 'TestBuildTreeJSON' -v`
Expected: PASS.

- [ ] **Step 5: Add JSON e2e to `tree.txt`** (append at the end)

```
# --json emits a nested tree: e1 with a children array down to s1
exec mtt tree --json
stdout '"id": "e1"'
stdout '"children"'
stdout '"id": "t1"'
stdout '"id": "s1"'

# an empty subtree filter still yields a JSON array, never null
exec mtt tree --status ghost --json
stdout '\[\]'
```

- [ ] **Step 6: Run e2e + `make check`**

Run: `go test ./internal/cli/ -run 'TestScripts/tree' -v && make check`
Expected: PASS; green.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/tree.go internal/cli/tree_test.go internal/cli/testdata/scripts/tree.txt
git commit -m "feat(cli): tree --json (nested tree, [] on empty)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: `list --parent` / `--kind`

**Files:**
- Modify: `internal/cli/list.go`
- Modify: `internal/cli/testdata/scripts/list_edit.txt`

**Interfaces:**
- Consumes: `core.ListFilter.Parent`/`Kinds` (Task 3), `parseKinds` (Task 6).
- Produces: `mtt list --parent <id>` and `mtt list --kind <…>`.

- [ ] **Step 1: Extend the e2e `list_edit.txt`**

After the existing epic/task setup (the script already creates `e1`, `e2`, and a `--no-parent` `t1`), add a child of `e1` and the new filters. Insert after the `exec mtt add --no-parent 'fix login'` block:

```
# a real child of e1 for the --parent filter
exec mtt add --type task --parent e1 'child of e1'
stdout 'created t2'

# --parent lists only direct children of e1
exec mtt list --parent e1
stdout 't2  task  \[tbd\]  child of e1'
! stdout 't1  task  \[tbd\]  fix login'

# --kind initial matches tbd tasks; --kind terminal matches none yet
exec mtt list --kind initial
stdout 'e1  epic'
exec mtt list --kind terminal
! stdout 'e1  epic'

# an invalid --kind errors
! exec mtt list --kind bogus
stderr 'invalid --kind'
```

(Note: the later `--type task` assertion in the script now also matches `t2`; it asserts `t1` presence and epic absence, both still hold. Confirm the run stays green.)

- [ ] **Step 2: Run the e2e to verify it fails**

Run: `go test ./internal/cli/ -run 'TestScripts/list_edit' -v`
Expected: FAIL — `unknown flag: --parent`.

- [ ] **Step 3: Implement in `internal/cli/list.go`**

Add `parent` and `kinds` variables and flags, parse the kinds, and thread them into `ListFilter`:

```go
	var (
		statuses []string
		types    []string
		kinds    []string
		parent   string
		sortKey  string
	)
```

In `RunE`, after the `--sort` validation switch, parse kinds:

```go
			kindVals, err := parseKinds(kinds)
			if err != nil {
				return err
			}
```

Update the `core.Select` call to include the new fields:

```go
			selected := core.Select(tasks, core.ListFilter{
				Statuses: statuses, Types: types, Kinds: kindVals, Parent: parent, Sort: core.SortKey(sortKey),
			}, cfg)
```

Register the flags (after the `--sort` flag):

```go
	cmd.Flags().StringArrayVar(&kinds, "kind", nil, "filter by status category: initial|active|terminal (repeatable)")
	cmd.Flags().StringVar(&parent, "parent", "", "only direct children of this task id")
```

- [ ] **Step 4: Run the e2e to verify it passes**

Run: `go test ./internal/cli/ -run 'TestScripts/list_edit' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/list.go internal/cli/testdata/scripts/list_edit.txt
git commit -m "feat(cli): list --parent (direct children) and --kind (category)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: `mtt show` lineage

**Files:**
- Modify: `internal/cli/show.go`
- Modify: `internal/cli/testdata/scripts/tree.txt` (lineage e2e)

**Interfaces:**
- Consumes: `core.NewIndex`, `Index.Ancestors` (Task 2), `TaskStore.List`.
- Produces: `mtt show <id>` prints a `lineage:` breadcrumb when the task has ancestors.

- [ ] **Step 1: Add the lineage e2e to `tree.txt`** (append at the end)

```
# show renders the lineage breadcrumb (root-first) for a nested task
exec mtt show s1
stdout 's1  subtask  \[tbd\]'
stdout 'lineage:  e1 › t1'

# a root task has no lineage line
exec mtt show e1
! stdout 'lineage:'
```

- [ ] **Step 2: Run the e2e to verify it fails**

Run: `go test ./internal/cli/ -run 'TestScripts/tree' -v`
Expected: FAIL — no `lineage:` line in `show s1` output.

- [ ] **Step 3: Implement in `internal/cli/show.go`**

The lineage needs the full task set, so build an `Index` from `TaskStore.List`. Change `RunE` to load the list and compute the breadcrumb, and extend `formatTask` to accept ancestors.

Replace the `RunE` body's rendering section (after a successful `Get`) so it composes the lineage:

```go
			store := yaml.NewTaskStore(root)
			task, err := store.Get(args[0])
			if err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return fmt.Errorf("task %q not found", args[0])
				}
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
			}
			tasks, err := store.List()
			if err != nil {
				return err
			}
			lineage := core.NewIndex(tasks).Ancestors(task.ID)
			_, err = fmt.Fprint(cmd.OutOrStdout(), formatTask(task, lineage))
			return err
```

Add the `core` import (`"github.com/pashukhin/mtt/internal/core"`). Update `formatTask` to render the breadcrumb:

```go
// formatTask renders a task as a human-readable block. ancestors is the
// root-first parent chain (empty for a root task); it prints a "lineage" line.
func formatTask(t mtt.Task, ancestors []mtt.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s  [%s]\n", t.ID, t.Type, t.Status)
	if t.Title != "" {
		fmt.Fprintf(&b, "  title:    %s\n", t.Title)
	}
	if len(ancestors) > 0 {
		ids := make([]string, len(ancestors))
		for i, a := range ancestors {
			ids[i] = a.ID
		}
		fmt.Fprintf(&b, "  lineage:  %s\n", strings.Join(ids, " › "))
	}
	if t.Parent != "" {
		fmt.Fprintf(&b, "  parent:   %s\n", t.Parent)
	}
	fmt.Fprintf(&b, "  created:  %s\n", t.Created.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "  updated:  %s\n", t.Updated.UTC().Format(time.RFC3339))
	if t.Description != "" {
		fmt.Fprintf(&b, "\n  %s\n", t.Description)
	}
	return b.String()
}
```

Note: the existing `show_test.go`/`show_json_test.go` may call `formatTask(task)` with one argument. Update any such call to pass `nil` for ancestors (e.g. `formatTask(task, nil)`).

- [ ] **Step 4: Update `formatTask` callers in tests**

Run: `grep -rn 'formatTask(' internal/cli` — for each call with a single argument, add `, nil`. (The e2e drives the binary and is unaffected.)

- [ ] **Step 5: Run the e2e + unit tests to verify they pass**

Run: `go test ./internal/cli/ -run 'TestScripts/tree|TestShow' -v`
Expected: PASS.

- [ ] **Step 6: `make check`, then commit**

```bash
make check
git add internal/cli/show.go internal/cli/show_test.go internal/cli/show_json_test.go internal/cli/testdata/scripts/tree.txt
git commit -m "feat(cli): mtt show lineage breadcrumb (root-first ancestor chain)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: Docs, CLAUDE.md, version bump, finalize

**Files:**
- Modify: `internal/cli/root.go` (version bump)
- Modify: `DESIGN.md`, `DESIGN.ru.md`
- Modify: `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md`
- Modify: `pkg/mtt/CLAUDE.md`, `internal/core/CLAUDE.md`, `internal/cli/CLAUDE.md`
- Modify: `sessions/004_hierarchy.md`, `NEXT_SESSION.md`

**Interfaces:** none (docs + version string).

- [ ] **Step 1: Bump the dev version — `internal/cli/root.go`**

```go
var version = "0.4.0-dev"
```

Update the `version_test.go` expectation if it asserts the exact string (run `grep -n '0.3.0-dev' internal/cli` and update matches).

- [ ] **Step 2: Update `CLI_REFERENCE.md`** — move the hierarchy surface from "phase 2 / later" to implemented:
  - `mtt add`: mark `--parent <id>` implemented (session 004); it is the normal placement path, mutually exclusive with `--no-parent`.
  - `mtt list`: mark `--parent <id>` and `--kind <initial|active|terminal>` implemented (session 004).
  - `mtt show`: note the lineage breadcrumb is now printed.
  - `mtt tree`: mark implemented (session 004); document `[<id>]` rooting, `--status`/`--kind`/`--depth` (levels, like `tree -L`), keep-ancestors filtering, and nested `--json`.

- [ ] **Step 3: Mirror the same edits into `CLI_REFERENCE.ru.md`** (Russian mirror; English is source of truth — keep them consistent).

- [ ] **Step 4: Update `DESIGN.md`** — in "Types and hierarchy" / "Listing and editing", note that hierarchy is now **rendered** (`mtt tree`, `mtt show` lineage) via a derived `core.Index` (children computed, back-refs never stored) and that `add --parent` is the normal placement path. Mirror into `DESIGN.ru.md`.

- [ ] **Step 5: Update the CLAUDE.md files:**
  - `pkg/mtt/CLAUDE.md`: add `AcceptsParent`/`StatusKind` to the type-query primitives.
  - `internal/core/CLAUDE.md`: add `Index` (derived hierarchy: children/ancestors/roots, cycle-safe) and `Match` (shared status/type/kind/parent predicate; `Select` now takes `cfg`).
  - `internal/cli/CLAUDE.md`: add `tree` (renders `core.Index`, keep-ancestors, `--depth`, nested `--json`); note `list` now loads config for `--kind`/`--parent`, `show` prints lineage, and `taskLine` is the shared row formatter.

- [ ] **Step 6: Fill `sessions/004_hierarchy.md` "Done" section** — list what shipped (`add --parent`, `tree`, `show` lineage, `list --parent`/`--kind`), the e2e (`tree.txt`), and the key decisions. Update `NEXT_SESSION.md` "Where we are" (session 004 done; version `0.4.0-dev`; next up session 005 — dependencies/`ready`) and carry forward any new lessons.

- [ ] **Step 7: Final gate**

Run: `make check`
Expected: all green (fmt + vet + lint + `go test -race -cover` + build).

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "docs(session-004): hierarchy — CLI_REFERENCE/DESIGN (+ru), CLAUDE.md, version 0.4.0-dev

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review (author checklist — completed)

**1. Spec coverage** — every spec section maps to a task:
- §4 domain predicates → Task 1. §5.1 `Index` → Task 2. §5.2 `Match`/`Select` → Task 3. §6 `add --parent` → Tasks 4–5. §7 `tree` (render/keep-ancestors/depth) → Task 6; nested `--json` → Task 7. §8 `list --parent`/`--kind` → Task 8. §9 `show` lineage → Task 9. §10 errors — covered across Tasks 4/6/8. §11 docs → Task 10. §12 tests — each task is test-first; e2e `tree.txt` built in Tasks 6/7/9. §2 "no port change" — honored (no `pkg/mtt/store.go` edit).

**2. Placeholder scan** — no TBD/TODO/"handle edge cases"; every code step shows complete code.

**3. Type consistency** — `Select(tasks, f, cfg)` signature is consistent across Tasks 3/6/8; `ListFilter` fields (`Statuses`/`Types`/`Kinds`/`Parent`/`Sort`) match all call sites; `renderTree`/`buildTreeJSON`/`markVisible`/`visibleChildren`/`taskLine`/`parseKinds` signatures are consistent between Tasks 6 and 7; `formatTask(task, ancestors)` updated with its callers in Task 9; `Index` method names (`Get`/`Roots`/`Children`/`Ancestors`) match between Tasks 2 and 6/9.

**Known call-site ripples flagged in-plan:** `Select` signature change updates `list_test.go` (Task 3 Step 4) and `list.go` (Task 3 Step 5); `formatTask` arity change updates `show` tests (Task 9 Step 4); version-string test (Task 10 Step 1).
