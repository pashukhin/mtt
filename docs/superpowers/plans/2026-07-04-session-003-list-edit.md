# Session 003 — List & edit + global flags — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `mtt list` (filters + deterministic order + `--json`) and `mtt edit` (non-flow fields), and wire the global-flags surface (`--dir`/`MTT_DIR`, `--version`, `--json`) as root persistent flags.

**Architecture:** Full hexagon `cli → core → mtt.TaskStore ← yaml.Store`. The port grows `List()`/`Update()`. `core` gets a pure `Select` (filter + order — ID-agnostic, provider-agnostic) and an `Editor` mutation usecase (injected clock). The CLI adds the two commands, a `projectRoot` helper (DRYs `Getwd → FindRoot`), and a JSON output view. `core` imports only `pkg/mtt`; the domain stays pure (no json/yaml tags).

**Tech Stack:** Go 1.23, cobra, `gopkg.in/yaml.v3`, `encoding/json` (stdlib), `rogpeppe/go-internal/testscript` (e2e).

**Spec:** [../specs/2026-07-04-session-003-list-edit-design.md](../specs/2026-07-04-session-003-list-edit-design.md).

## Global Constraints

- **Test before code** (TDD red → green → refactor). `make check` green before any commit.
- Fanatical **SOLID / DRY / KISS / DDD / clean architecture (hexagonal)**. `core` imports **only** `pkg/mtt`, **never** `adapter/*`.
- `pkg/mtt` is **pure**: no yaml/json tags, no `prefix`, no I/O. Adapters/CLI own DTOs/views and map to/from the domain.
- **No literals** for type/status names or ID structure in `core`/domain. `list` order defaults to the domain `Created` timestamp (desc); the ID tie-break is an **opaque string compare** (`strings.Compare`), never a `<prefix><N>` parse.
- CLI output → `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` (never `cmd.Print*`). Commands return errors via `RunE`; `Execute` prints `error: <msg>` to stderr (exit 1).
- Wrap errors: `fmt.Errorf("...: %w", err)`. No `panic` in library code. Every exported symbol gets a doc comment.
- Deterministic serialization/output: field order = struct order; two-space JSON indent + trailing newline. Writes atomic (temp + rename).
- Module path: `github.com/pashukhin/mtt`. Go floor `1.23.1`.
- **Order determinism is a unit concern** (fixed-clock `core.Select` tests); the e2e asserts row presence/absence, **never** inter-row order (wall-clock in e2e).
- Commit trailer on every commit: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.

## File Structure

**Create:**
- `internal/core/list.go` — `ListFilter`, `SortKey`, pure `Select` (filter + deterministic order).
- `internal/core/edit.go` — `Editor`, `NewEditor`, `EditParams`, `Edit`.
- `internal/cli/project.go` — `projectRoot`, `baseDir`, `jsonFlag` helpers.
- `internal/cli/json.go` — `taskJSON`, `toTaskJSON`, `writeJSON`.
- `internal/cli/list.go` — `newListCmd`, `writeList`.
- `internal/cli/edit.go` — `newEditCmd`.
- Test files alongside each; e2e `internal/cli/testdata/scripts/list_edit.txt`.

**Modify:**
- `pkg/mtt/store.go` — grow `TaskStore` with `List`/`Update`.
- `internal/adapter/yaml/task.go` — `List`, `Update`, shared private `write`; refactor `Create`.
- `internal/adapter/yaml/root.go` — export `HasProject`.
- `internal/core/add_test.go` — stub `List`/`Update` on the existing `fakeStore` (interface grew).
- `internal/cli/root.go` — persistent flags `--dir`/`--json`, `root.Version`, register `list`/`edit`.
- `internal/cli/add.go`, `show.go`, `types.go`, `init.go` — use `projectRoot`/`baseDir`; `show` honors `--json`.
- CLAUDE.md files (`pkg/mtt`, `internal/core`, `internal/adapter/yaml`, `internal/cli`); project docs (Task 8).

---

### Task 1: Port grows + YAML adapter `List`/`Update` (shared `write`)

**Files:**
- Modify: `internal/adapter/yaml/task.go`
- Test: `internal/adapter/yaml/task_test.go` (append)
- Modify: `pkg/mtt/store.go`
- Modify: `internal/core/add_test.go` (fakeStore stubs)
- Modify: `pkg/mtt/CLAUDE.md`, `internal/adapter/yaml/CLAUDE.md`

**Interfaces:**
- Consumes: `mtt.Task`, `Load`, `mint`, `atomicWrite`, `fromDomainTask`, `ymlTask.toDomain`, `dirName`, `tasksDirName`, `fixedTime`/`initDefault` (test helpers from 002).
- Produces: `func (s *Store) List() ([]mtt.Task, error)`; `func (s *Store) Update(t mtt.Task) (mtt.Task, error)`; private `func (s *Store) write(t mtt.Task) (mtt.Task, error)`; grown `mtt.TaskStore` interface (`List`, `Update`).

- [ ] **Step 1: Write failing adapter tests** — append to `internal/adapter/yaml/task_test.go`:

```go
func TestStoreList(t *testing.T) {
	root := initDefault(t)
	s := NewTaskStore(root)

	if tasks, err := s.List(); err != nil || len(tasks) != 0 {
		t.Fatalf("empty list = %v, %v; want 0 tasks", tasks, err)
	}
	if _, err := s.Create(mtt.Task{Type: "epic", Title: "a", Status: "tbd", Created: fixedTime(), Updated: fixedTime()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(mtt.Task{Type: "epic", Title: "b", Status: "tbd", Created: fixedTime(), Updated: fixedTime()}); err != nil {
		t.Fatal(err)
	}
	tasks, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("list len = %d, want 2", len(tasks))
	}
	titles := map[string]bool{}
	for _, tk := range tasks {
		titles[tk.Title] = true
	}
	if !titles["a"] || !titles["b"] {
		t.Fatalf("round-trip titles = %v, want a and b", titles)
	}
}

func TestStoreUpdate(t *testing.T) {
	root := initDefault(t)
	s := NewTaskStore(root)

	created, err := s.Create(mtt.Task{Type: "epic", Title: "old", Status: "tbd", Created: fixedTime(), Updated: fixedTime()})
	if err != nil {
		t.Fatal(err)
	}
	created.Title = "new"
	if _, err := s.Update(created); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "new" {
		t.Fatalf("title after update = %q, want new", got.Title)
	}
	if _, err := s.Update(mtt.Task{ID: "e999", Type: "epic", Status: "tbd", Created: fixedTime(), Updated: fixedTime()}); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("update missing = %v, want ErrNotFound", err)
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/adapter/yaml/ -run 'TestStoreList|TestStoreUpdate' -v`
Expected: FAIL (undefined: `s.List`, `s.Update`).

- [ ] **Step 3: Refactor `Create` + add `List`/`Update`/`write`** — in `internal/adapter/yaml/task.go`, add `"strings"` to the import block, replace the `Create` body to use a shared `write`, and append the new methods:

```go
// Create mints a flat per-prefix ID for the logical task, persists it atomically,
// and returns the stored task.
func (s *Store) Create(t mtt.Task) (mtt.Task, error) {
	_, prefixes, err := Load(s.root)
	if err != nil {
		return mtt.Task{}, err
	}
	prefix := prefixes[t.Type]
	if prefix == "" {
		return mtt.Task{}, fmt.Errorf("type %q: no prefix (unknown or prefixless type)", t.Type)
	}
	id, err := mint(s.root, prefix)
	if err != nil {
		return mtt.Task{}, err
	}
	t.ID = id
	return s.write(t)
}

// write serializes t and atomically persists it to .mtt/tasks/<id>.yaml. t.ID
// must already be set. It is the single serialization+write path for the store.
func (s *Store) write(t mtt.Task) (mtt.Task, error) {
	data, err := goyaml.Marshal(fromDomainTask(t))
	if err != nil {
		return mtt.Task{}, fmt.Errorf("marshal task %s: %w", t.ID, err)
	}
	path := filepath.Join(s.root, dirName, tasksDirName, t.ID+".yaml")
	if err := atomicWrite(path, data); err != nil {
		return mtt.Task{}, err
	}
	return t, nil
}

// Update overwrites an existing task by t.ID; it never mints and never creates.
// A task that does not exist yields mtt.ErrNotFound.
func (s *Store) Update(t mtt.Task) (mtt.Task, error) {
	path := filepath.Join(s.root, dirName, tasksDirName, t.ID+".yaml")
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mtt.Task{}, mtt.ErrNotFound
		}
		return mtt.Task{}, fmt.Errorf("stat %s: %w", path, err)
	}
	return s.write(t)
}

// List returns all tasks under .mtt/tasks/, mapping each file to the domain. The
// order is unspecified (os.ReadDir's lexical order); callers impose their own
// deterministic order. A missing tasks/ directory yields an empty slice.
func (s *Store) List() ([]mtt.Task, error) {
	dir := filepath.Join(s.root, dirName, tasksDirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var tasks []mtt.Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var yt ymlTask
		if err := goyaml.Unmarshal(data, &yt); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		task, err := yt.toDomain()
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}
```

- [ ] **Step 4: Run — verify adapter tests pass**

Run: `go test ./internal/adapter/yaml/ -v`
Expected: PASS (all yaml tests, including the two new ones).

- [ ] **Step 5: Grow the `TaskStore` port** — in `pkg/mtt/store.go`, add the two methods to the interface (with doc comments):

```go
type TaskStore interface {
	// Create persists a logical task (empty ID); the adapter mints the ID and
	// returns the stored task.
	Create(t Task) (Task, error)
	// Get loads a task by ID, returning ErrNotFound when it does not resolve.
	Get(id string) (Task, error)
	// List returns all tasks. The order is unspecified — callers impose their
	// own deterministic order (an adapter is not required to sort).
	List() ([]Task, error)
	// Update overwrites an existing task identified by t.ID, returning the stored
	// task; it never mints and never creates. Missing task -> ErrNotFound.
	Update(t Task) (Task, error)
}
```

- [ ] **Step 6: Fix the existing core fake** — the grown interface breaks `internal/core/add_test.go`'s `fakeStore`. Append stubs so it satisfies the interface again:

```go
func (f *fakeStore) List() ([]mtt.Task, error)       { return nil, nil }
func (f *fakeStore) Update(t mtt.Task) (mtt.Task, error) { f.got = t; return t, nil }
```

- [ ] **Step 7: Run — whole tree green**

Run: `go build ./... && go test ./... 2>&1 | tail -20`
Expected: build OK; all packages PASS (adapter, core, cli, pkg all compile against the grown interface).

- [ ] **Step 8: Update CLAUDE.md** — in `pkg/mtt/CLAUDE.md`, change the `TaskStore` bullet to:

```markdown
- **Port**: `TaskStore` — `Create` (mints the ID in the adapter), `Get` (`ErrNotFound`), `List` (all tasks; order unspecified — callers order), `Update` (overwrite existing by ID; `ErrNotFound` if absent). Pure — no prefix/YAML leaks through it.
```

In `internal/adapter/yaml/CLAUDE.md`, extend the `NewTaskStore`/`Store` bullet with:

```markdown
`List` reads `.mtt/tasks/*.yaml` → domain (order unspecified; `core` orders). `Update` overwrites an existing task by ID (`ErrNotFound` if absent). `Create`/`Update` share one private `write` (marshal + atomic temp+rename) — serialization lives in exactly one place.
```

- [ ] **Step 9: Commit**

```bash
git add pkg/mtt/store.go pkg/mtt/CLAUDE.md internal/adapter/yaml/task.go internal/adapter/yaml/task_test.go internal/adapter/yaml/CLAUDE.md internal/core/add_test.go
git commit -m "feat(store): grow TaskStore with List/Update; YAML List/Update + shared write

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Core — `Select` (pure filter + deterministic order)

**Files:**
- Create: `internal/core/list.go`
- Test: `internal/core/list_test.go`

**Interfaces:**
- Consumes: `mtt.Task`.
- Produces: `type SortKey string` (`SortCreated`, `SortUpdated`); `type ListFilter struct{ Statuses, Types []string; Sort SortKey }`; `func Select(tasks []mtt.Task, f ListFilter) []mtt.Task`.

- [ ] **Step 1: Write the failing test** — `internal/core/list_test.go`:

```go
package core

import (
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func tsk(id, typ, status string, created time.Time) mtt.Task {
	return mtt.Task{ID: id, Type: typ, Status: status, Created: created, Updated: created}
}

func TestSelectFilters(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{
		tsk("e1", "epic", "tbd", base),
		tsk("t1", "task", "tbd", base),
		tsk("t2", "task", "done", base),
	}
	if got := Select(tasks, ListFilter{}); len(got) != 3 {
		t.Fatalf("no filter len = %d, want 3", len(got))
	}
	if got := Select(tasks, ListFilter{Types: []string{"task"}}); len(got) != 2 {
		t.Fatalf("type=task len = %d, want 2", len(got))
	}
	got := Select(tasks, ListFilter{Types: []string{"task"}, Statuses: []string{"done"}})
	if len(got) != 1 || got[0].ID != "t2" {
		t.Fatalf("task AND done -> %+v", got)
	}
	if got := Select(tasks, ListFilter{Statuses: []string{"ghost"}}); len(got) != 0 {
		t.Fatalf("no match len = %d, want 0", len(got))
	}
}

func TestSelectOrderCreatedDesc(t *testing.T) {
	older := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	got := Select([]mtt.Task{tsk("e1", "epic", "tbd", older), tsk("e2", "epic", "tbd", newer)}, ListFilter{})
	if got[0].ID != "e2" || got[1].ID != "e1" {
		t.Fatalf("created desc = %s,%s; want e2,e1", got[0].ID, got[1].ID)
	}
}

func TestSelectTieBreakByIDString(t *testing.T) {
	same := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	got := Select([]mtt.Task{
		tsk("t2", "task", "tbd", same),
		tsk("t1", "task", "tbd", same),
		tsk("e1", "epic", "tbd", same),
	}, ListFilter{})
	want := []string{"e1", "t1", "t2"} // equal Created -> opaque ID string ascending
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("tie-break[%d] = %s, want %s", i, got[i].ID, id)
		}
	}
}

func TestSelectSortUpdated(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	a := mtt.Task{ID: "e1", Type: "epic", Status: "tbd", Created: base.Add(2 * time.Hour), Updated: base}
	b := mtt.Task{ID: "e2", Type: "epic", Status: "tbd", Created: base, Updated: base.Add(2 * time.Hour)}
	got := Select([]mtt.Task{a, b}, ListFilter{Sort: SortUpdated})
	if got[0].ID != "e2" {
		t.Fatalf("sort=updated top = %s, want e2", got[0].ID)
	}
}

func TestSelectDoesNotMutateInput(t *testing.T) {
	base := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	tasks := []mtt.Task{tsk("e1", "epic", "tbd", base), tsk("e2", "epic", "tbd", base.Add(time.Hour))}
	_ = Select(tasks, ListFilter{})
	if tasks[0].ID != "e1" || tasks[1].ID != "e2" {
		t.Fatalf("input reordered: %s,%s", tasks[0].ID, tasks[1].ID)
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/core/ -run TestSelect -v`
Expected: FAIL (undefined: `Select`, `ListFilter`, `SortUpdated`).

- [ ] **Step 3: Create `internal/core/list.go`**

```go
package core

import (
	"sort"
	"strings"

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

// ListFilter holds the list predicates and ordering. Empty Statuses/Types match
// everything; within a field the values are OR-ed, across fields AND-ed.
type ListFilter struct {
	Statuses []string
	Types    []string
	Sort     SortKey
}

// Select returns the tasks matching f, in a deterministic order, without
// mutating the input. Primary order: the chosen timestamp (Created, or Updated
// when Sort==SortUpdated) descending. Tie-break: ID ascending as an opaque
// string compare — so equal timestamps never reorder between runs. Select never
// interprets ID structure, so it stays provider-agnostic.
func Select(tasks []mtt.Task, f ListFilter) []mtt.Task {
	out := make([]mtt.Task, 0, len(tasks))
	for _, t := range tasks {
		if anyOrEmpty(f.Statuses, t.Status) && anyOrEmpty(f.Types, t.Type) {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ti, tj := out[i].Created, out[j].Created
		if f.Sort == SortUpdated {
			ti, tj = out[i].Updated, out[j].Updated
		}
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return strings.Compare(out[i].ID, out[j].ID) < 0
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

- [ ] **Step 4: Run — verify pass**

Run: `go test ./internal/core/ -run TestSelect -v`
Expected: PASS (all five tests).

- [ ] **Step 5: Commit**

```bash
git add internal/core/list.go internal/core/list_test.go
git commit -m "feat(core): pure Select — filter + provider-agnostic deterministic order

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Core — `Editor` (edit usecase)

**Files:**
- Create: `internal/core/edit.go`
- Test: `internal/core/edit_test.go`
- Modify: `internal/core/CLAUDE.md`

**Interfaces:**
- Consumes: `mtt.TaskStore` (`Get`/`Update`), `mtt.Task`, `mtt.ErrNotFound`.
- Produces: `type Editor`; `func NewEditor(store mtt.TaskStore, now func() time.Time) *Editor`; `type EditParams struct{ Title, Description *string }`; `func (e *Editor) Edit(id string, p EditParams) (mtt.Task, error)`.

- [ ] **Step 1: Write the failing test** — `internal/core/edit_test.go`:

```go
package core

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// editStore is a fake TaskStore: Get returns a seeded task, Update records it.
type editStore struct {
	get     mtt.Task
	getErr  error
	updated mtt.Task
}

func (s *editStore) Create(t mtt.Task) (mtt.Task, error) { return t, nil }
func (s *editStore) Get(string) (mtt.Task, error)        { return s.get, s.getErr }
func (s *editStore) List() ([]mtt.Task, error)           { return nil, nil }
func (s *editStore) Update(t mtt.Task) (mtt.Task, error) { s.updated = t; return t, nil }

func strptr(s string) *string { return &s }
func later() time.Time        { return time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC) }

func TestEditTitleOnly(t *testing.T) {
	orig := mtt.Task{ID: "e1", Type: "epic", Title: "old", Status: "tbd", Description: "d",
		Created: fixed(), Updated: fixed()}
	fs := &editStore{get: orig}
	got, err := NewEditor(fs, later).Edit("e1", EditParams{Title: strptr("new")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "new" || got.Description != "d" {
		t.Fatalf("title/desc = %q/%q", got.Title, got.Description)
	}
	if !got.Updated.Equal(later().Truncate(time.Second)) {
		t.Fatalf("updated not bumped: %v", got.Updated)
	}
	if !got.Created.Equal(fixed().Truncate(time.Second)) {
		t.Fatalf("created changed: %v", got.Created)
	}
	if !fs.updated.Updated.Equal(got.Updated) {
		t.Fatal("store.Update did not receive the bumped task")
	}
}

func TestEditNothing(t *testing.T) {
	_, err := NewEditor(&editStore{}, later).Edit("e1", EditParams{})
	if err == nil || !strings.Contains(err.Error(), "nothing to edit") {
		t.Fatalf("want 'nothing to edit', got %v", err)
	}
}

func TestEditEmptyingBothRejected(t *testing.T) {
	orig := mtt.Task{ID: "e1", Type: "epic", Title: "old", Status: "tbd", Created: fixed(), Updated: fixed()}
	_, err := NewEditor(&editStore{get: orig}, later).Edit("e1", EditParams{Title: strptr("")})
	if err == nil || !strings.Contains(err.Error(), "title or a description") {
		t.Fatalf("want content invariant error, got %v", err)
	}
}

func TestEditNotFoundPropagates(t *testing.T) {
	fs := &editStore{getErr: mtt.ErrNotFound}
	_, err := NewEditor(fs, later).Edit("ghost", EditParams{Title: strptr("x")})
	if !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
```

Note: `fixed()` is the fixed-clock helper already declared in `add_test.go` — do not redeclare it.

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/core/ -run TestEdit -v`
Expected: FAIL (undefined: `Editor`, `NewEditor`, `EditParams`).

- [ ] **Step 3: Create `internal/core/edit.go`**

```go
package core

import (
	"fmt"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Editor is the edit-a-task usecase (a mutation): it loads a task, applies the
// requested non-flow field changes, bumps Updated via the injected clock, and
// persists via TaskStore.Update.
type Editor struct {
	store mtt.TaskStore
	now   func() time.Time
}

// NewEditor wires the usecase. now is injected for deterministic timestamps.
func NewEditor(store mtt.TaskStore, now func() time.Time) *Editor {
	return &Editor{store: store, now: now}
}

// EditParams are the requested edits. A nil pointer means "leave unchanged"; a
// non-nil pointer (including to "") means "set to this value". Only title and
// description are editable: id/type are immutable, status moves through flow
// enforcement, and re-parenting is a separate operation.
type EditParams struct {
	Title       *string
	Description *string
}

// Edit applies p to task id, bumps Updated, persists, and returns the task.
func (e *Editor) Edit(id string, p EditParams) (mtt.Task, error) {
	if p.Title == nil && p.Description == nil {
		return mtt.Task{}, fmt.Errorf("nothing to edit: provide --title and/or --description")
	}
	t, err := e.store.Get(id)
	if err != nil {
		return mtt.Task{}, err
	}
	if p.Title != nil {
		t.Title = *p.Title
	}
	if p.Description != nil {
		t.Description = *p.Description
	}
	if t.Title == "" && t.Description == "" {
		return mtt.Task{}, fmt.Errorf("a task needs a title or a description")
	}
	t.Updated = e.now().UTC().Truncate(time.Second)
	return e.store.Update(t)
}
```

- [ ] **Step 4: Run — verify pass**

Run: `go test ./internal/core/ -v`
Expected: PASS (all core tests — add, list, edit).

- [ ] **Step 5: Update `internal/core/CLAUDE.md`** — under "## Responsibilities", append:

```markdown
- `Select` (pure read): filter tasks by status/type (AND across dimensions, OR within) and impose a
  deterministic order — `Created` desc by default (or `Updated`), tie-broken by ID as an **opaque string**
  (never parsing ID structure; provider-agnostic). No store injected — a pure function the CLI composes with
  `TaskStore.List` (a pure read needs no usecase; the only logic is the filter/sort). Reused later by
  `ready`/`tree`.
- `Editor` (the `edit` usecase, a mutation): load via `TaskStore.Get`, apply only the provided
  title/description (nil pointer = unchanged), enforce the title-or-description invariant, bump `updated`
  from the **injected clock**, persist via `TaskStore.Update`. id/type/status/parent are not editable here.
```

- [ ] **Step 6: Commit**

```bash
git add internal/core/edit.go internal/core/edit_test.go internal/core/CLAUDE.md
git commit -m "feat(core): Editor usecase (title/description; bump updated; injected clock)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: CLI — global flags + `projectRoot` helper + DRY refactor

**Files:**
- Create: `internal/cli/project.go`
- Modify: `internal/adapter/yaml/root.go` (export `HasProject`)
- Modify: `internal/cli/root.go`, `internal/cli/add.go`, `internal/cli/show.go`, `internal/cli/types.go`, `internal/cli/init.go`
- Test: `internal/cli/project_test.go`

**Interfaces:**
- Consumes: `yaml.FindRoot`, `yaml.HasProject`, `cobra` persistent flags, test helpers `runOut`/`chdir`/`runRoot`.
- Produces: `func projectRoot(cmd *cobra.Command) (string, error)`; `func baseDir(cmd *cobra.Command) (string, error)`; `func jsonFlag(cmd *cobra.Command) bool`; `func yaml.HasProject(dir string) bool`; root persistent flags `--dir`/`--json`; `root.Version`.

- [ ] **Step 1: Add `HasProject` to the adapter** — in `internal/adapter/yaml/root.go`, append:

```go
// HasProject reports whether dir directly contains an .mtt/ project directory
// (no upward walk, unlike FindRoot). Used by callers that resolve an explicit
// project root (e.g. a --dir flag).
func HasProject(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, dirName))
	return err == nil && info.IsDir()
}
```

- [ ] **Step 2: Write the failing test** — `internal/cli/project_test.go`:

```go
package cli

import (
	"strings"
	"testing"
)

func TestDirFlagAndEnvResolveProject(t *testing.T) {
	proj := t.TempDir()
	other := t.TempDir() // sibling temp dir, no .mtt ancestor

	// init the project via --dir, from an unrelated cwd
	chdir(t, other)
	if _, _, err := runOut(t, "--dir", proj, "init"); err != nil {
		t.Fatalf("init --dir: %v", err)
	}
	if _, _, err := runOut(t, "--dir", proj, "add", "--type", "epic", "build auth"); err != nil {
		t.Fatalf("add --dir: %v", err)
	}
	out, _, err := runOut(t, "--dir", proj, "show", "e1")
	if err != nil || !strings.Contains(out, "e1") {
		t.Fatalf("show --dir: out=%q err=%v", out, err)
	}

	// MTT_DIR env resolves the same project (cwd still `other`, has no .mtt)
	t.Setenv("MTT_DIR", proj)
	out, _, err = runOut(t, "show", "e1")
	if err != nil || !strings.Contains(out, "e1") {
		t.Fatalf("show via MTT_DIR: out=%q err=%v", out, err)
	}

	// --dir without .mtt errors (flag overrides env)
	if _, _, err := runOut(t, "--dir", other, "show", "e1"); err == nil {
		t.Fatal("--dir without .mtt should error")
	}
}
```

- [ ] **Step 3: Run — verify fail**

Run: `go test ./internal/cli/ -run TestDirFlagAndEnvResolveProject -v`
Expected: FAIL (unknown flag `--dir`).

- [ ] **Step 4: Create `internal/cli/project.go`**

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
)

// resolveDir returns the explicit project directory from --dir, else $MTT_DIR,
// else "" (meaning "discover from the cwd").
func resolveDir(cmd *cobra.Command) string {
	dir, _ := cmd.Flags().GetString("dir")
	if dir == "" {
		dir = os.Getenv("MTT_DIR")
	}
	return dir
}

// projectRoot resolves the project root for a command: --dir/MTT_DIR if set
// (which must itself contain .mtt/, no upward walk), else the nearest ancestor
// of the cwd containing .mtt/ (FindRoot).
func projectRoot(cmd *cobra.Command) (string, error) {
	if dir := resolveDir(cmd); dir != "" {
		if !yaml.HasProject(dir) {
			return "", fmt.Errorf("no .mtt/ in %q", dir)
		}
		return dir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return yaml.FindRoot(cwd)
}

// baseDir resolves the base directory for init: --dir/MTT_DIR if set, else the
// cwd. Unlike projectRoot it does not require an existing .mtt/ (init creates it).
func baseDir(cmd *cobra.Command) (string, error) {
	if dir := resolveDir(cmd); dir != "" {
		return dir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return cwd, nil
}

// jsonFlag reports whether the persistent --json flag was set.
func jsonFlag(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("json")
	return v
}
```

- [ ] **Step 5: Register persistent flags + `Version` in `internal/cli/root.go`** — set `Version` on the command literal and add the flags before `AddCommand`:

```go
	root := &cobra.Command{
		Use:           "mtt",
		Short:         "mtt — minimalist file-backed task tracker for agents and humans",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String("dir", "", "project root containing .mtt/ (overrides discovery; env MTT_DIR)")
	root.PersistentFlags().Bool("json", false, "emit machine-readable JSON output")
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd(), newShowCmd())
```

- [ ] **Step 6: Refactor `add`/`show`/`types` to `projectRoot`, and `init` to `baseDir`** — replace each command's `cwd, _ := os.Getwd(); root, _ := yaml.FindRoot(cwd)` block with `projectRoot(cmd)`.

In `internal/cli/add.go` `RunE`, replace the cwd/FindRoot block with:

```go
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
```

In `internal/cli/show.go` `RunE`, likewise replace:

```go
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
```

In `internal/cli/types.go` `RunE`, likewise replace:

```go
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
```

In `internal/cli/init.go` `RunE`, replace the cwd block with `baseDir` and base the name on it:

```go
			base, err := baseDir(cmd)
			if err != nil {
				return err
			}
			projectName := name
			if projectName == "" {
				projectName = filepath.Base(base)
			}
			if err := yaml.Init(base, tmpl, projectName, force); err != nil {
				return err
			}
```

Remove the now-unused `os` import from `add.go`/`show.go`/`types.go` if it becomes unused (each still may use `os`? — after this change they no longer call `os.Getwd`; drop `"os"` from their import blocks if nothing else uses it). `init.go` keeps `"path/filepath"`, drops `"os"` if unused.

- [ ] **Step 7: Run — verify pass (and no regressions)**

Run: `go test ./internal/cli/ -v 2>&1 | tail -30`
Expected: PASS — the new `TestDirFlagAndEnvResolveProject` plus all existing cli tests (add/show/types/init/version) still green. `--version` works: `go run ./cmd/mtt --version` prints the version string.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/project.go internal/cli/project_test.go internal/cli/root.go internal/cli/add.go internal/cli/show.go internal/cli/types.go internal/cli/init.go internal/adapter/yaml/root.go
git commit -m "feat(cli): global --dir/MTT_DIR + --version + --json flags; projectRoot DRY

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: CLI — JSON output view + `show --json`

**Files:**
- Create: `internal/cli/json.go`
- Modify: `internal/cli/show.go`
- Test: `internal/cli/show_json_test.go`

**Interfaces:**
- Consumes: `mtt.Task`, `jsonFlag` (Task 4).
- Produces: `type taskJSON`; `func toTaskJSON(t mtt.Task) taskJSON`; `func writeJSON(w io.Writer, v any) error`.

- [ ] **Step 1: Write the failing test** — `internal/cli/show_json_test.go`:

```go
package cli

import (
	"encoding/json"
	"testing"
)

func TestShowJSON(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runOut(t, "add", "--type", "epic", "build auth"); err != nil {
		t.Fatal(err)
	}
	out, _, err := runOut(t, "show", "--json", "e1")
	if err != nil {
		t.Fatalf("show --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	if got["id"] != "e1" || got["type"] != "epic" || got["title"] != "build auth" {
		t.Fatalf("json fields = %v", got)
	}
	if got["status"] != "tbd" {
		t.Fatalf("status = %v, want tbd", got["status"])
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/cli/ -run TestShowJSON -v`
Expected: FAIL (`--json` accepted but ignored → output is the human block, not JSON → unmarshal error).

- [ ] **Step 3: Create `internal/cli/json.go`**

```go
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// taskJSON is the CLI's machine-readable view of a task. JSON is a presentation
// concern, so the tags live here, not on the pure domain type (mirrors the YAML
// adapter's DTO). Reserved collections are omitted until later phases populate
// them; adding fields later is additive.
type taskJSON struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title,omitempty"`
	Status      string `json:"status"`
	Parent      string `json:"parent,omitempty"`
	Created     string `json:"created"`
	Updated     string `json:"updated"`
	Description string `json:"description,omitempty"`
}

// toTaskJSON maps a domain task to its JSON view (RFC3339 UTC timestamps).
func toTaskJSON(t mtt.Task) taskJSON {
	return taskJSON{
		ID: t.ID, Type: t.Type, Title: t.Title, Status: t.Status, Parent: t.Parent,
		Created:     t.Created.UTC().Format(time.RFC3339),
		Updated:     t.Updated.UTC().Format(time.RFC3339),
		Description: t.Description,
	}
}

// writeJSON marshals v as indented JSON with a trailing newline (stable diff).
func writeJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}
```

- [ ] **Step 4: Wire `--json` into `show`** — in `internal/cli/show.go` `RunE`, replace the final render block:

```go
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), formatTask(task))
			return err
```

- [ ] **Step 5: Run — verify pass**

Run: `go test ./internal/cli/ -run 'TestShowJSON|TestShow' -v`
Expected: PASS (JSON path and the existing human-output show tests).

- [ ] **Step 6: Commit**

```bash
git add internal/cli/json.go internal/cli/show.go internal/cli/show_json_test.go
git commit -m "feat(cli): taskJSON view + show --json

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: CLI — `mtt list`

**Files:**
- Create: `internal/cli/list.go`
- Modify: `internal/cli/root.go` (register)
- Test: `internal/cli/list_test.go`

**Interfaces:**
- Consumes: `projectRoot`, `jsonFlag`, `taskJSON`/`toTaskJSON`/`writeJSON`, `yaml.NewTaskStore`, `core.Select`/`core.ListFilter`/`core.SortKey`, test helpers `runOut`/`runRoot`/`chdir`.
- Produces: `func newListCmd() *cobra.Command`; `func writeList(w io.Writer, tasks []mtt.Task) error`; registered in `NewRootCmd`.

- [ ] **Step 1: Write the failing test** — `internal/cli/list_test.go`:

```go
package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustAdd(t *testing.T, args ...string) {
	t.Helper()
	if _, _, err := runOut(t, append([]string{"add"}, args...)...); err != nil {
		t.Fatalf("add %v: %v", args, err)
	}
}

func TestListCommand(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, "--type", "epic", "build auth")
	mustAdd(t, "--type", "epic", "build billing")
	mustAdd(t, "--no-parent", "fix login") // default type (task) -> t1

	// presence, not order (wall-clock e2e/unit split)
	out, _, err := runOut(t, "list")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"e1  epic  [tbd]", "e2  epic  [tbd]", "t1  task  [tbd]"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list missing %q in:\n%s", want, out)
		}
	}

	// --type task narrows to t1, drops epics
	out, _, err = runOut(t, "list", "--type", "task")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "t1  task") || strings.Contains(out, "e1  epic") {
		t.Fatalf("type=task filter wrong:\n%s", out)
	}

	// invalid --sort errors
	if _, _, err := runOut(t, "list", "--sort", "bogus"); err == nil {
		t.Fatal("invalid --sort should error")
	}

	// --json is a valid array of 3
	out, _, err = runOut(t, "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("invalid json array: %v\n%s", err, out)
	}
	if len(arr) != 3 {
		t.Fatalf("json array len = %d, want 3", len(arr))
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/cli/ -run TestListCommand -v`
Expected: FAIL (unknown command "list").

- [ ] **Step 3: Create `internal/cli/list.go`**

```go
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newListCmd builds `mtt list`: list tasks with filters and a stable order.
func newListCmd() *cobra.Command {
	var (
		statuses []string
		types    []string
		sortKey  string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch sortKey {
			case "", string(core.SortCreated), string(core.SortUpdated):
			default:
				return fmt.Errorf("invalid --sort %q: want created|updated", sortKey)
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			tasks, err := yaml.NewTaskStore(root).List()
			if err != nil {
				return err
			}
			selected := core.Select(tasks, core.ListFilter{
				Statuses: statuses, Types: types, Sort: core.SortKey(sortKey),
			})
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
	cmd.Flags().StringVar(&sortKey, "sort", "", "sort order: created|updated (default created)")
	return cmd
}

// writeList renders tasks one per line: "<id>  <type>  [<status>]  <title>"
// (the title is omitted when empty).
func writeList(w io.Writer, tasks []mtt.Task) error {
	var b strings.Builder
	for _, t := range tasks {
		fmt.Fprintf(&b, "%s  %s  [%s]", t.ID, t.Type, t.Status)
		if t.Title != "" {
			fmt.Fprintf(&b, "  %s", t.Title)
		}
		b.WriteString("\n")
	}
	_, err := fmt.Fprint(w, b.String())
	return err
}
```

- [ ] **Step 4: Register in `internal/cli/root.go`** — add `newListCmd()` to the `AddCommand` call:

```go
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd(), newShowCmd(), newListCmd())
```

- [ ] **Step 5: Run — verify pass**

Run: `go test ./internal/cli/ -run TestListCommand -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/list.go internal/cli/list_test.go internal/cli/root.go
git commit -m "feat(cli): mtt list — filters, stable order, --json

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: CLI — `mtt edit`

**Files:**
- Create: `internal/cli/edit.go`
- Modify: `internal/cli/root.go` (register)
- Test: `internal/cli/edit_test.go`

**Interfaces:**
- Consumes: `projectRoot`, `jsonFlag`, `toTaskJSON`/`writeJSON`, `yaml.NewTaskStore`, `core.NewEditor`/`core.EditParams`, `mtt.ErrNotFound`, `cmd.Flags().Changed`.
- Produces: `func newEditCmd() *cobra.Command`; registered in `NewRootCmd`.

- [ ] **Step 1: Write the failing test** — `internal/cli/edit_test.go`:

```go
package cli

import (
	"strings"
	"testing"
)

func TestEditCommand(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runOut(t, "add", "--type", "epic", "old title"); err != nil {
		t.Fatal(err)
	}

	if _, _, err := runOut(t, "edit", "e1", "--title", "new title"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	out, _, err := runOut(t, "show", "e1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "new title") {
		t.Fatalf("show after edit = %q", out)
	}

	// no editable flag -> error
	if _, _, err := runOut(t, "edit", "e1"); err == nil {
		t.Fatal("edit with no flag should error")
	}
	// missing id -> error
	if _, _, err := runOut(t, "edit", "nope", "--title", "x"); err == nil {
		t.Fatal("edit missing id should error")
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/cli/ -run TestEditCommand -v`
Expected: FAIL (unknown command "edit").

- [ ] **Step 3: Create `internal/cli/edit.go`**

```go
package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newEditCmd builds `mtt edit <id>`: edit a task's non-flow fields.
func newEditCmd() *cobra.Command {
	var title, desc string
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a task's title and/or description",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("provide exactly one task id (example: mtt edit e1 --title \"…\")")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var p core.EditParams
			if cmd.Flags().Changed("title") {
				p.Title = &title
			}
			if cmd.Flags().Changed("description") {
				p.Description = &desc
			}
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			editor := core.NewEditor(yaml.NewTaskStore(root), time.Now)
			task, err := editor.Edit(args[0], p)
			if err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return fmt.Errorf("task %q not found", args[0])
				}
				return err
			}
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), toTaskJSON(task))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", task.ID)
			return err
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&desc, "description", "", "new description")
	return cmd
}
```

- [ ] **Step 4: Register in `internal/cli/root.go`** — add `newEditCmd()` to `AddCommand`:

```go
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd(), newShowCmd(), newListCmd(), newEditCmd())
```

- [ ] **Step 5: Run — verify pass**

Run: `go test ./internal/cli/ -run TestEditCommand -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/edit.go internal/cli/edit_test.go internal/cli/root.go
git commit -m "feat(cli): mtt edit — title/description, bump updated

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: e2e testscript + doc reconciliation

**Files:**
- Create: `internal/cli/testdata/scripts/list_edit.txt`
- Modify: `internal/cli/CLAUDE.md`, `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md`, `DESIGN.md`, `DESIGN.ru.md`, `TASKS.md`, `sessions/003_list_and_edit.md`, `NEXT_SESSION.md`

**Interfaces:**
- Consumes: the full CLI (`init`/`add`/`list`/`edit`/`show`, global flags) via `testscript`.
- Produces: the acceptance e2e; synced docs.

- [ ] **Step 1: Write the e2e script** — `internal/cli/testdata/scripts/list_edit.txt`:

```txt
# init a project, add tasks, then list/filter/edit
mkdir proj
cd proj
exec mtt init
stdout 'initialized'

exec mtt add --type epic 'build auth'
stdout 'created e1'
exec mtt add --type epic 'build billing'
stdout 'created e2'
exec mtt add --no-parent 'fix login'
stdout 'created t1'

# list shows every task (presence asserts, not order)
exec mtt list
stdout 'e1  epic  \[tbd\]  build auth'
stdout 'e2  epic  \[tbd\]  build billing'
stdout 't1  task  \[tbd\]  fix login'

# --type task narrows to t1 and drops the epics
exec mtt list --type task
stdout 't1  task  \[tbd\]'
! stdout 'e1  epic'

# --status filters; unknown status -> empty (still exit 0)
exec mtt list --status tbd
stdout 'e1  epic'
exec mtt list --status ghost
! stdout 'e1'

# --sort updated is accepted
exec mtt list --sort updated
stdout 't1  task'

# invalid --sort errors
! exec mtt list --sort bogus
stderr 'invalid --sort'

# --json emits a valid array (anchored on a known id)
exec mtt list --json
stdout '"id": "e1"'
stdout '"type": "epic"'

# edit updates the title; show reflects it
exec mtt edit e1 --title 'build auth v2'
stdout 'updated e1'
exec mtt show e1
stdout 'build auth v2'

# edit with no editable flag errors
! exec mtt edit e1
stderr 'nothing to edit'

# edit a missing id errors
! exec mtt edit nope --title 'x'
stderr 'not found'

# global --dir operates from outside the project
cd $WORK
exec mtt --dir $WORK/proj list
stdout 'e1  epic'

# MTT_DIR env is equivalent
env MTT_DIR=$WORK/proj
exec mtt list
stdout 'e1  epic'
```

- [ ] **Step 2: Run the e2e**

Run: `go test ./internal/cli/ -run TestScripts -v`
Expected: PASS (`list_edit.txt` alongside the existing `add_show.txt`/`init.txt`).

- [ ] **Step 3: Full gate**

Run: `make check`
Expected: gofmt clean, vet clean, golangci-lint clean, `go test -race -cover ./...` PASS, build OK. (If `golangci-lint` flags an unused import in a refactored command file from Task 4, remove it and re-run.)

- [ ] **Step 4: Update `internal/cli/CLAUDE.md`** — update the "## Current state" paragraph to:

```markdown
`root` + `version` + `init` + `types` + `add` + `show` + `list` + `edit`, plus the root persistent flags
`--dir`/`MTT_DIR`, `--version`, and `--json`. `projectRoot(cmd)` resolves the root (--dir/MTT_DIR else
FindRoot) and DRYs the former `Getwd → FindRoot`; `baseDir` does the same for `init` (no .mtt required).
`list` composes `TaskStore.List` → `core.Select` (pure read: filter/order in core, no usecase) and renders
human text or, with `--json`, a `taskJSON` array; `edit` goes through `core.Editor` (a mutation) and prints
`updated <id>` or the JSON object. `show`/`list`/`edit` honor `--json` via the `taskJSON` view.
```

- [ ] **Step 5: Reconcile `CLI_REFERENCE.md` + `CLI_REFERENCE.ru.md`** — mark the now-implemented surface (keep both language files in sync):
  - In the **Global flags** table, annotate `--json`, `--dir`/`MTT_DIR`, and `--version` as implemented (session 003); leave `--role`/`--quiet`/`--no-color` as pending.
  - `### mtt list` — change the phase tag to note `--status`/`--type`/`--sort` and `--json` shipped in session 003; keep `--kind`/`--parent`/`--ready` marked as later.
  - `### mtt edit` — note it is implemented in session 003 (title/description; `-`/stdin still later).
  - Add a one-line note under the exit-codes table that the richer taxonomy (2/4/…) is still **proposed** — 003 keeps the single generic failure code.

- [ ] **Step 6: Reconcile `DESIGN.md` + `DESIGN.ru.md`** — keep the mirror in sync:
  - Where `list`/ordering is discussed, record: **`list` default order is `Created` desc (provider-agnostic — the domain timestamp, not ID structure), tie-broken by an opaque ID string**; `--sort created|updated`.
  - Note `edit` touches only title/description; **re-parenting is a separate operation** (already in the backlog), not an edit.
  - Add to the "Later (backlog)" list: **durable, git-independent audit of edits** (a change-log or field versioning — additive later) **plus the subject-identity (`By`) source** (likely `.mtt/config.local.yaml`, distinct from `--role`); `history` stays transition-only (phase 3).

- [ ] **Step 7: Reconcile `TASKS.md`** — in the cross-cutting global-flags note, tick that `--dir`/`MTT_DIR`, the `--version` flag, and `--json` landed in session 003; under "Later (coarse)" add the edit-audit + subject-identity backlog item.

- [ ] **Step 8: Fill `sessions/003_list_and_edit.md`** — set Status to done; in "## Done", summarize what shipped (list with filters/sort/--json; edit; global flags; port grew List/Update; core Select + Editor; DRY projectRoot), and note the deferred edit-audit/subject-identity slice and the e2e `list_edit.txt`.

- [ ] **Step 9: Update `NEXT_SESSION.md`** — move "Where we are" forward (003 done and merged pending PR); set the next task to **session 004 (hierarchy: `--parent`, `mtt tree`)**; carry forward the lessons plus the new one: **default list order is provider-agnostic (`Created` desc, opaque-ID tie-break); order determinism is unit-tested, e2e asserts presence not sequence**; and record the open **edit-audit + subject-identity** slice as a design item to schedule.

- [ ] **Step 10: Commit**

```bash
git add internal/cli/testdata/scripts/list_edit.txt internal/cli/CLAUDE.md CLI_REFERENCE.md CLI_REFERENCE.ru.md DESIGN.md DESIGN.ru.md TASKS.md sessions/003_list_and_edit.md NEXT_SESSION.md
git commit -m "test(cli): list_edit e2e; docs: reconcile list/edit + global flags (session 003)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Final verification

- [ ] `make check` green (the gate CI runs).
- [ ] `git log --oneline` shows the 8 task commits on `feat/s003-list-edit`.
- [ ] Manual smoke (the e2e scenarios to hand the user): `mtt init` → a few `mtt add` → `mtt list` /
  `--type` / `--status` / `--sort updated` / `--json` → `mtt edit e1 --title …` → `mtt show e1` →
  `mtt --dir <path> list` / `MTT_DIR=<path> mtt list` → `mtt --version`.
- [ ] Open the PR (branch → PR → CI green → squash into `main`), per AGENTS.md — only on the user's request.
