# Session 002 — Create & view a task — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first full hexagon slice — create a task and view it — with `mtt add` and `mtt show`.

**Architecture:** `cli → core → mtt.TaskStore ← yaml.Store`. The pure `pkg/mtt` contract grows a `Task` model + reserved value objects + the `TaskStore` port. `internal/core` (new) owns the add *policy* and imports only `pkg/mtt`. `internal/adapter/yaml` implements the port with flat, per-prefix ID minting and deterministic serialization. The CLI is the composition root.

**Tech Stack:** Go 1.23, cobra, `gopkg.in/yaml.v3`, `rogpeppe/go-internal/testscript` (e2e).

## Global Constraints

- **Test before code** (TDD red → green → refactor). `make check` green before any commit.
- Fanatical **SOLID / DRY / KISS / DDD / clean architecture (hexagonal)**. Dependencies point inward: `core` imports **only** `pkg/mtt`, **never** `adapter/*`.
- `pkg/mtt` is **pure**: no yaml/json tags, no `prefix`, no I/O. Adapters own DTOs and map to/from the domain.
- The code contains **no literals** for type/status names or ID structure. `StatusKind`/`RefKind` are value objects (closed vocabularies), not names.
- CLI output → `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` (never `cmd.Print*`). Commands return errors via `RunE`.
- Wrap errors: `fmt.Errorf("...: %w", err)`. No `panic` in library code. Every exported symbol gets a doc comment.
- Deterministic serialization: field order = struct order. IDs are flat, per-prefix (`e1`, `t17`), independent of position; files are `.mtt/tasks/<id>.yaml`; writes are atomic (temp + rename) with `O_EXCL` reservation.
- Module path: `github.com/pashukhin/mtt`. Go version floor `1.23.1`.

## File Structure

**Create:**
- `pkg/mtt/task.go` — `Task`, `Ref`/`RefKind`, `Comment`, `HistoryEntry`, `Check` + `RefKind.Valid`.
- `pkg/mtt/store.go` — `TaskStore` port + `ErrNotFound`.
- `internal/adapter/yaml/mint.go` — flat per-prefix ID minting (scan + `O_EXCL`).
- `internal/adapter/yaml/task_dto.go` — `ymlTask` (+ sub-DTOs) + `fromDomainTask`/`toDomain` mapping.
- `internal/adapter/yaml/task.go` — `Store` (`NewTaskStore`, `Create`, `Get`).
- `internal/core/add.go` — `Adder`, `NewAdder`, `AddParams`, `Add`.
- `internal/core/CLAUDE.md` — package orientation.
- `internal/cli/add.go`, `internal/cli/show.go` — the two commands (+ `formatTask`).
- Test files alongside each; golden `internal/adapter/yaml/testdata/golden/task_min.yaml`; e2e `internal/cli/testdata/scripts/add_show.txt`.

**Modify:**
- `pkg/mtt/config.go` — add `Status.Default`; add `Type.IsRoot`, `Type.InitialStatus`, `Config.TypeByName`.
- `pkg/mtt/validate.go` — two new per-flow invariants (default-status).
- `internal/cli/root.go` — register `newAddCmd()`, `newShowCmd()`.
- `pkg/mtt/CLAUDE.md`, `internal/adapter/yaml/CLAUDE.md`, `internal/cli/CLAUDE.md` — package docs.
- Project docs: `DESIGN.md`, `DESIGN.ru.md`, `AGENTS.md`, `TASKS.md`, `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md`, `sessions/002_create_and_view.md`, `NEXT_SESSION.md`.

---

### Task 1: Domain — `Status.Default`, entry/type helpers, validation

**Files:**
- Modify: `pkg/mtt/config.go`
- Modify: `pkg/mtt/validate.go`
- Test: `pkg/mtt/config_test.go` (append), `pkg/mtt/validate_test.go` (append cases)

**Interfaces:**
- Produces: `Status.Default bool`; `func (t Type) IsRoot() bool`; `func (t Type) InitialStatus() (Status, bool)`; `func (c Config) TypeByName(name string) (Type, bool)`. Two new `Validate` invariants: ≤1 `Default` status per flow; a `Default` status must be `KindInitial`.

- [ ] **Step 1: Write failing helper tests** — append to `pkg/mtt/config_test.go`:

```go
func TestIsRoot(t *testing.T) {
	if !(Type{}).IsRoot() {
		t.Fatal("no parents => root")
	}
	if (Type{Parents: []string{"epic"}}).IsRoot() {
		t.Fatal("with parents => not root")
	}
}

func TestInitialStatus(t *testing.T) {
	// first initial wins when none is marked default
	ty := Type{Flow: Flow{Statuses: []Status{
		{Name: "a", Kind: KindInitial},
		{Name: "b", Kind: KindInitial},
		{Name: "c", Kind: KindActive},
	}}}
	if s, ok := ty.InitialStatus(); !ok || s.Name != "a" {
		t.Fatalf("first initial: got %q ok=%v", s.Name, ok)
	}
	// a marked default initial wins over order
	ty.Statuses[1].Default = true
	if s, ok := ty.InitialStatus(); !ok || s.Name != "b" {
		t.Fatalf("default initial: got %q ok=%v", s.Name, ok)
	}
	// no initial => false
	none := Type{Flow: Flow{Statuses: []Status{{Name: "x", Kind: KindTerminal}}}}
	if _, ok := none.InitialStatus(); ok {
		t.Fatal("no initial should return false")
	}
}

func TestTypeByName(t *testing.T) {
	c := Config{Types: []Type{{Name: "epic"}, {Name: "task"}}}
	if ty, ok := c.TypeByName("task"); !ok || ty.Name != "task" {
		t.Fatalf("lookup task: %q ok=%v", ty.Name, ok)
	}
	if _, ok := c.TypeByName("ghost"); ok {
		t.Fatal("ghost should not resolve")
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `go test ./pkg/mtt/ -run 'TestIsRoot|TestInitialStatus|TestTypeByName' -v`
Expected: FAIL (compile error: `Default`/`IsRoot`/`InitialStatus`/`TypeByName` undefined).

- [ ] **Step 3: Add the field and helpers** — in `pkg/mtt/config.go`, add `Default bool` to `Status` (with a doc comment) and append the helpers:

```go
// Status is one state in a flow. Kind is a value object; Description is optional.
// Default marks THE entry status when a flow has more than one initial (mirrors
// Type.Default); it is ignored unless the status is initial.
type Status struct {
	Name        string
	Kind        StatusKind
	Description string
	Default     bool
}
```

```go
// IsRoot reports whether the type sits at the root level (declares no parents).
func (t Type) IsRoot() bool { return len(t.Parents) == 0 }

// InitialStatus returns the flow's entry status: the initial status marked
// Default, else the first initial in config order. The bool is false when the
// flow has no initial status.
func (t Type) InitialStatus() (Status, bool) {
	var first Status
	found := false
	for _, s := range t.Statuses {
		if s.Kind != KindInitial {
			continue
		}
		if s.Default {
			return s, true
		}
		if !found {
			first, found = s, true
		}
	}
	return first, found
}

// TypeByName returns the type with the given name, or false when absent.
func (c Config) TypeByName(name string) (Type, bool) {
	for _, t := range c.Types {
		if t.Name == name {
			return t, true
		}
	}
	return Type{}, false
}
```

- [ ] **Step 4: Run — verify pass**

Run: `go test ./pkg/mtt/ -run 'TestIsRoot|TestInitialStatus|TestTypeByName' -v`
Expected: PASS.

- [ ] **Step 5: Write failing validation cases** — append two cases to the `cases` slice in `TestValidateErrors` in `pkg/mtt/validate_test.go`:

```go
		{"two default statuses", func(c *Config) {
			c.Types[0].Statuses[0].Default = true
			c.Types[0].Statuses = append(c.Types[0].Statuses, Status{Name: "tbd2", Kind: KindInitial, Default: true})
			c.Types[0].Transitions = append(c.Types[0].Transitions, Transition{From: "tbd2", To: "doing"})
		}, "default statuses"},
		{"default on non-initial", func(c *Config) { c.Types[0].Statuses[1].Default = true }, "default status must be initial"},
```

- [ ] **Step 6: Run — verify fail**

Run: `go test ./pkg/mtt/ -run TestValidateErrors -v`
Expected: FAIL on the two new subtests (no error produced).

- [ ] **Step 7: Add the invariants** — in `pkg/mtt/validate.go`, inside `validateFlow`, append before `return errs`:

```go
	defaults := 0
	for _, s := range t.Statuses {
		if !s.Default {
			continue
		}
		defaults++
		if s.Kind != KindInitial {
			errs = append(errs, fmt.Errorf("type %q status %q: default status must be initial", t.Name, s.Name))
		}
	}
	if defaults > 1 {
		errs = append(errs, fmt.Errorf("type %q: %d default statuses, want at most one", t.Name, defaults))
	}
```

- [ ] **Step 8: Run — verify pass (and no regressions)**

Run: `go test ./pkg/mtt/ -v`
Expected: PASS (including `TestValidateOK`, all `TestValidateErrors` subtests).

- [ ] **Step 9: Commit**

```bash
git add pkg/mtt/config.go pkg/mtt/validate.go pkg/mtt/config_test.go pkg/mtt/validate_test.go
git commit -m "feat(pkg/mtt): Status.Default + entry/type helpers + validation"
```

---

### Task 2: Domain — `Task` model, value objects, `TaskStore` port

**Files:**
- Create: `pkg/mtt/task.go`, `pkg/mtt/store.go`
- Test: `pkg/mtt/task_test.go`
- Modify: `pkg/mtt/CLAUDE.md`

**Interfaces:**
- Produces: `Task` (fields: `ID,Type,Title,Status,Parent string; Tags,DependsOn []string; Refs []Ref; Created,Updated time.Time; Description string; Comments []Comment; History []HistoryEntry`); `RefKind` (`RefNote/RefTask/RefComment/RefURL`) + `RefKind.Valid`; `Ref{Kind RefKind; ID, Label string}`; `Comment{ID int; Author string; Created time.Time; Body string; Refs []Ref; Replies []Comment}`; `HistoryEntry{At time.Time; By,Role,From,To string; Checks []Check}`; `Check{Cmd string; Exit int}`; `TaskStore` interface (`Create(Task) (Task, error)`, `Get(string) (Task, error)`); `ErrNotFound`.

- [ ] **Step 1: Write the failing test** — `pkg/mtt/task_test.go`:

```go
package mtt

import (
	"errors"
	"testing"
)

func TestRefKindValid(t *testing.T) {
	for _, k := range []RefKind{RefNote, RefTask, RefComment, RefURL} {
		if !k.Valid() {
			t.Fatalf("%q should be valid", k)
		}
	}
	if RefKind("bogus").Valid() {
		t.Fatal("bogus should be invalid")
	}
}

func TestErrNotFound(t *testing.T) {
	if ErrNotFound == nil || !errors.Is(ErrNotFound, ErrNotFound) {
		t.Fatal("ErrNotFound must be a usable sentinel")
	}
}

// compile-time: a Task carries the reserved collections.
var _ = Task{Refs: []Ref{{Kind: RefTask}}, Comments: []Comment{{Replies: nil}}, History: []HistoryEntry{{Checks: []Check{{Exit: 0}}}}}
```

- [ ] **Step 2: Run — verify fail**

Run: `go test ./pkg/mtt/ -run 'TestRefKindValid|TestErrNotFound' -v`
Expected: FAIL (undefined types).

- [ ] **Step 3: Create `pkg/mtt/task.go`**

```go
package mtt

import "time"

// Task is a single unit of work. Field order == on-disk order (deterministic
// diff). Title is optional when Description is set (core requires at least one).
// Tags/DependsOn/Refs/Comments/History are reserved; they are populated in later
// sessions and omitted from storage while empty.
type Task struct {
	ID          string
	Type        string
	Title       string
	Status      string
	Parent      string
	Tags        []string
	DependsOn   []string
	Refs        []Ref
	Created     time.Time
	Updated     time.Time
	Description string
	Comments    []Comment
	History     []HistoryEntry
}

// RefKind is the closed vocabulary of reference targets — a value object.
type RefKind string

// The four reference kinds.
const (
	RefNote    RefKind = "note"
	RefTask    RefKind = "task"
	RefComment RefKind = "comment"
	RefURL     RefKind = "url"
)

// Valid reports whether k is one of the four defined kinds.
func (k RefKind) Valid() bool {
	switch k {
	case RefNote, RefTask, RefComment, RefURL:
		return true
	default:
		return false
	}
}

// Ref is a structured, verifiable reference (informational; not a blocking edge).
type Ref struct {
	Kind  RefKind
	ID    string
	Label string
}

// Comment is a tree node via nested Replies; ID is sequential within the task.
type Comment struct {
	ID      int
	Author  string
	Created time.Time
	Body    string
	Refs    []Ref
	Replies []Comment
}

// HistoryEntry is one append-only transition record. Role is the roles seam.
type HistoryEntry struct {
	At     time.Time
	By     string
	Role   string
	From   string
	To     string
	Checks []Check
}

// Check is one gate command's result recorded on a transition.
type Check struct {
	Cmd  string
	Exit int
}
```

- [ ] **Step 4: Create `pkg/mtt/store.go`**

```go
package mtt

import "errors"

// TaskStore is the mandatory-minimum driven port for tasks: create (the adapter
// mints the ID) and get by ID. Implementations map their own DTOs to and from
// these pure domain types.
type TaskStore interface {
	// Create persists a logical task (empty ID); the adapter mints the ID and
	// returns the stored task.
	Create(t Task) (Task, error)
	// Get loads a task by ID, returning ErrNotFound when it does not resolve.
	Get(id string) (Task, error)
}

// ErrNotFound is returned by TaskStore.Get when the ID does not resolve.
var ErrNotFound = errors.New("mtt: task not found")
```

- [ ] **Step 5: Run — verify pass**

Run: `go test ./pkg/mtt/ -v`
Expected: PASS.

- [ ] **Step 6: Update `pkg/mtt/CLAUDE.md`** — under "## Rules" or a new "## Model" note, add:

```markdown
- **Task model**: `Task` (id/type/title/status/parent + reserved `tags`/`depends_on`/`refs`/`comments`/`history`), value objects `RefKind` and (existing) `StatusKind`. `Status.Default` marks the entry status when a flow has >1 initial (mirrors `Type.Default`; resolved by `Type.InitialStatus`).
- **Port**: `TaskStore` (Create mints the ID in the adapter; Get returns `ErrNotFound`). Pure — no prefix/YAML leaks through it.
```

- [ ] **Step 7: Commit**

```bash
git add pkg/mtt/task.go pkg/mtt/store.go pkg/mtt/task_test.go pkg/mtt/CLAUDE.md
git commit -m "feat(pkg/mtt): Task model, value objects, TaskStore port"
```

---

### Task 3: YAML adapter — flat per-prefix ID minting

**Files:**
- Create: `internal/adapter/yaml/mint.go`
- Test: `internal/adapter/yaml/mint_test.go`

**Interfaces:**
- Consumes: `dirName` (`.mtt`, from `root.go`), `atomicWrite` (from `init.go`).
- Produces: `const tasksDirName = "tasks"`; `func mint(root, prefix string) (string, error)` — scans `.mtt/tasks/<prefix><N>.yaml`, returns `<prefix>(max+1)`, and reserves that file with `O_EXCL`.

- [ ] **Step 1: Write the failing test** — `internal/adapter/yaml/mint_test.go`:

```go
package yaml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMint(t *testing.T) {
	root := t.TempDir()

	id, err := mint(root, "e")
	if err != nil || id != "e1" {
		t.Fatalf("first mint = %q, %v; want e1", id, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".mtt", "tasks", "e1.yaml")); err != nil {
		t.Fatalf("reserved file missing: %v", err)
	}
	if id, _ := mint(root, "e"); id != "e2" {
		t.Fatalf("second mint = %q, want e2", id)
	}
	if id, _ := mint(root, "t"); id != "t1" {
		t.Fatalf("independent prefix = %q, want t1", id)
	}

	// respects an existing higher number
	if err := os.WriteFile(filepath.Join(root, ".mtt", "tasks", "e9.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if id, _ := mint(root, "e"); id != "e10" {
		t.Fatalf("after e9 = %q, want e10", id)
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/adapter/yaml/ -run TestMint -v`
Expected: FAIL (undefined: `mint`).

- [ ] **Step 3: Create `internal/adapter/yaml/mint.go`**

```go
package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// tasksDirName is the subdirectory of .mtt that holds one file per task.
const tasksDirName = "tasks"

// mint reserves and returns the next flat, per-prefix task ID under .mtt/tasks/.
// It scans <prefix><N>.yaml files, takes max(N)+1 (from 1), and creates the
// reserved file with O_EXCL so a concurrent mint cannot pick the same ID. IDs
// are flat (no parent chain), so identity is stable under re-parenting.
func mint(root, prefix string) (string, error) {
	dir := filepath.Join(root, dirName, tasksDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	re := regexp.MustCompile("^" + regexp.QuoteMeta(prefix) + `(\d+)\.yaml$`)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", dir, err)
	}
	max := 0
	for _, e := range entries {
		m := re.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if n, _ := strconv.Atoi(m[1]); n > max {
			max = n
		}
	}
	for n := max + 1; ; n++ {
		id := fmt.Sprintf("%s%d", prefix, n)
		path := filepath.Join(dir, id+".yaml")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = f.Close()
			return id, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("reserve %s: %w", path, err)
		}
	}
}
```

- [ ] **Step 4: Run — verify pass**

Run: `go test ./internal/adapter/yaml/ -run TestMint -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/yaml/mint.go internal/adapter/yaml/mint_test.go
git commit -m "feat(yaml): flat per-prefix task ID minting (O_EXCL)"
```

---

### Task 4: YAML adapter — task DTO + deterministic serialization

**Files:**
- Create: `internal/adapter/yaml/task_dto.go`
- Test: `internal/adapter/yaml/task_dto_test.go`
- Create: `internal/adapter/yaml/testdata/golden/task_min.yaml`

**Interfaces:**
- Consumes: `mtt.Task` and reserved value objects.
- Produces: `type ymlTask` (+ `ymlRef`/`ymlComment`/`ymlHistoryEntry`/`ymlCheck`); `func fromDomainTask(t mtt.Task) ymlTask`; `func (yt ymlTask) toDomain() (mtt.Task, error)`. Timestamps serialize as RFC3339 UTC (second precision) strings.

- [ ] **Step 1: Write the failing round-trip test** — `internal/adapter/yaml/task_dto_test.go`:

```go
package yaml

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	goyaml "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func fixedTime() time.Time { return time.Date(2026, 7, 4, 9, 20, 0, 0, time.UTC) }

func TestTaskRoundTrip(t *testing.T) {
	want := mtt.Task{
		ID: "t1", Type: "task", Title: "fix login", Status: "tbd",
		Parent: "e1", Tags: []string{"backend", "auth"}, DependsOn: []string{"t2"},
		Refs:    []mtt.Ref{{Kind: mtt.RefTask, ID: "t2", Label: "blocker"}},
		Created: fixedTime(), Updated: fixedTime(), Description: "multi\nline",
		Comments: []mtt.Comment{{ID: 1, Author: "agent", Created: fixedTime(), Body: "hi",
			Replies: []mtt.Comment{{ID: 2, Author: "human", Created: fixedTime(), Body: "yo"}}}},
		History: []mtt.HistoryEntry{{At: fixedTime(), By: "agent", Role: "impl", From: "tbd", To: "doing",
			Checks: []mtt.Check{{Cmd: "make test", Exit: 0}}}},
	}
	data, err := goyaml.Marshal(fromDomainTask(want))
	if err != nil {
		t.Fatal(err)
	}
	var yt ymlTask
	if err := goyaml.Unmarshal(data, &yt); err != nil {
		t.Fatal(err)
	}
	got, err := yt.toDomain()
	if err != nil {
		t.Fatal(err)
	}
	if !reflectDeepEqualTask(t, want, got) {
		t.Fatalf("round-trip mismatch:\nwant %+v\n got %+v", want, got)
	}
}

func TestTaskGoldenMinimal(t *testing.T) {
	task := mtt.Task{ID: "e1", Type: "epic", Title: "build auth", Status: "tbd",
		Created: fixedTime(), Updated: fixedTime()}
	got, err := goyaml.Marshal(fromDomainTask(task))
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "task_min.yaml")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run -update first): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("minimal task serialization != golden:\n%s", got)
	}
}

func reflectDeepEqualTask(t *testing.T, a, b mtt.Task) bool {
	t.Helper()
	da, _ := goyaml.Marshal(fromDomainTask(a))
	db, _ := goyaml.Marshal(fromDomainTask(b))
	return bytes.Equal(da, db)
}
```

Note: `*update` is the package-level flag already declared in `init_test.go` — do not redeclare it.

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/adapter/yaml/ -run 'TestTaskRoundTrip|TestTaskGoldenMinimal' -v`
Expected: FAIL (undefined: `fromDomainTask`, `ymlTask`).

- [ ] **Step 3: Create `internal/adapter/yaml/task_dto.go`**

```go
package yaml

import (
	"fmt"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// timeLayout is the on-disk timestamp format: RFC3339 UTC, second precision.
const timeLayout = time.RFC3339

// ymlTask is the on-disk DTO for a task: yaml tags + omitempty for optional and
// reserved fields; field order matches mtt.Task for a deterministic diff.
type ymlTask struct {
	ID          string            `yaml:"id"`
	Type        string            `yaml:"type"`
	Title       string            `yaml:"title,omitempty"`
	Status      string            `yaml:"status"`
	Parent      string            `yaml:"parent,omitempty"`
	Tags        []string          `yaml:"tags,omitempty"`
	DependsOn   []string          `yaml:"depends_on,omitempty"`
	Refs        []ymlRef          `yaml:"refs,omitempty"`
	Created     string            `yaml:"created"`
	Updated     string            `yaml:"updated"`
	Description string            `yaml:"description,omitempty"`
	Comments    []ymlComment      `yaml:"comments,omitempty"`
	History     []ymlHistoryEntry `yaml:"history,omitempty"`
}

type ymlRef struct {
	Kind  string `yaml:"kind"`
	ID    string `yaml:"id"`
	Label string `yaml:"label,omitempty"`
}

type ymlComment struct {
	ID      int          `yaml:"id"`
	Author  string       `yaml:"author,omitempty"`
	Created string       `yaml:"created"`
	Body    string       `yaml:"body,omitempty"`
	Refs    []ymlRef     `yaml:"refs,omitempty"`
	Replies []ymlComment `yaml:"replies,omitempty"`
}

type ymlHistoryEntry struct {
	At     string     `yaml:"at"`
	By     string     `yaml:"by,omitempty"`
	Role   string     `yaml:"role,omitempty"`
	From   string     `yaml:"from"`
	To     string     `yaml:"to"`
	Checks []ymlCheck `yaml:"checks,omitempty"`
}

type ymlCheck struct {
	Cmd  string `yaml:"cmd"`
	Exit int    `yaml:"exit"`
}

func fmtTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func fromDomainRefs(rs []mtt.Ref) []ymlRef {
	if len(rs) == 0 {
		return nil
	}
	out := make([]ymlRef, len(rs))
	for i, r := range rs {
		out[i] = ymlRef{Kind: string(r.Kind), ID: r.ID, Label: r.Label}
	}
	return out
}

func fromDomainComments(cs []mtt.Comment) []ymlComment {
	if len(cs) == 0 {
		return nil
	}
	out := make([]ymlComment, len(cs))
	for i, c := range cs {
		out[i] = ymlComment{ID: c.ID, Author: c.Author, Created: fmtTime(c.Created), Body: c.Body,
			Refs: fromDomainRefs(c.Refs), Replies: fromDomainComments(c.Replies)}
	}
	return out
}

func fromDomainHistory(hs []mtt.HistoryEntry) []ymlHistoryEntry {
	if len(hs) == 0 {
		return nil
	}
	out := make([]ymlHistoryEntry, len(hs))
	for i, h := range hs {
		var checks []ymlCheck
		if len(h.Checks) > 0 {
			checks = make([]ymlCheck, len(h.Checks))
			for j, ch := range h.Checks {
				checks[j] = ymlCheck{Cmd: ch.Cmd, Exit: ch.Exit}
			}
		}
		out[i] = ymlHistoryEntry{At: fmtTime(h.At), By: h.By, Role: h.Role, From: h.From, To: h.To, Checks: checks}
	}
	return out
}

// fromDomainTask maps the pure domain task to its on-disk DTO.
func fromDomainTask(t mtt.Task) ymlTask {
	return ymlTask{
		ID: t.ID, Type: t.Type, Title: t.Title, Status: t.Status, Parent: t.Parent,
		Tags: t.Tags, DependsOn: t.DependsOn, Refs: fromDomainRefs(t.Refs),
		Created: fmtTime(t.Created), Updated: fmtTime(t.Updated), Description: t.Description,
		Comments: fromDomainComments(t.Comments), History: fromDomainHistory(t.History),
	}
}

func toDomainRefs(rs []ymlRef) []mtt.Ref {
	if len(rs) == 0 {
		return nil
	}
	out := make([]mtt.Ref, len(rs))
	for i, r := range rs {
		out[i] = mtt.Ref{Kind: mtt.RefKind(r.Kind), ID: r.ID, Label: r.Label}
	}
	return out
}

func toDomainComments(cs []ymlComment) ([]mtt.Comment, error) {
	if len(cs) == 0 {
		return nil, nil
	}
	out := make([]mtt.Comment, len(cs))
	for i, c := range cs {
		created, err := parseTime(c.Created)
		if err != nil {
			return nil, err
		}
		replies, err := toDomainComments(c.Replies)
		if err != nil {
			return nil, err
		}
		out[i] = mtt.Comment{ID: c.ID, Author: c.Author, Created: created, Body: c.Body,
			Refs: toDomainRefs(c.Refs), Replies: replies}
	}
	return out, nil
}

func toDomainHistory(hs []ymlHistoryEntry) ([]mtt.HistoryEntry, error) {
	if len(hs) == 0 {
		return nil, nil
	}
	out := make([]mtt.HistoryEntry, len(hs))
	for i, h := range hs {
		at, err := parseTime(h.At)
		if err != nil {
			return nil, err
		}
		var checks []mtt.Check
		if len(h.Checks) > 0 {
			checks = make([]mtt.Check, len(h.Checks))
			for j, ch := range h.Checks {
				checks[j] = mtt.Check{Cmd: ch.Cmd, Exit: ch.Exit}
			}
		}
		out[i] = mtt.HistoryEntry{At: at, By: h.By, Role: h.Role, From: h.From, To: h.To, Checks: checks}
	}
	return out, nil
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", s, err)
	}
	return t.UTC(), nil
}

// toDomain maps the on-disk DTO back to the pure domain task.
func (yt ymlTask) toDomain() (mtt.Task, error) {
	created, err := parseTime(yt.Created)
	if err != nil {
		return mtt.Task{}, err
	}
	updated, err := parseTime(yt.Updated)
	if err != nil {
		return mtt.Task{}, err
	}
	comments, err := toDomainComments(yt.Comments)
	if err != nil {
		return mtt.Task{}, err
	}
	history, err := toDomainHistory(yt.History)
	if err != nil {
		return mtt.Task{}, err
	}
	return mtt.Task{
		ID: yt.ID, Type: yt.Type, Title: yt.Title, Status: yt.Status, Parent: yt.Parent,
		Tags: yt.Tags, DependsOn: yt.DependsOn, Refs: toDomainRefs(yt.Refs),
		Created: created, Updated: updated, Description: yt.Description,
		Comments: comments, History: history,
	}, nil
}
```

- [ ] **Step 4: Generate the golden and verify**

Run: `go test ./internal/adapter/yaml/ -run TestTaskGoldenMinimal -update`
Then inspect `internal/adapter/yaml/testdata/golden/task_min.yaml` — it must be exactly:

```yaml
id: e1
type: epic
title: build auth
status: tbd
created: "2026-07-04T09:20:00Z"
updated: "2026-07-04T09:20:00Z"
```

- [ ] **Step 5: Run — verify pass**

Run: `go test ./internal/adapter/yaml/ -run 'TestTaskRoundTrip|TestTaskGoldenMinimal' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/yaml/task_dto.go internal/adapter/yaml/task_dto_test.go internal/adapter/yaml/testdata/golden/task_min.yaml
git commit -m "feat(yaml): task DTO + deterministic RFC3339 serialization"
```

---

### Task 5: YAML adapter — `Store` (Create / Get)

**Files:**
- Create: `internal/adapter/yaml/task.go`
- Test: `internal/adapter/yaml/task_test.go`
- Modify: `internal/adapter/yaml/CLAUDE.md`

**Interfaces:**
- Consumes: `mint` (Task 3), `fromDomainTask`/`ymlTask.toDomain` (Task 4), `Load` (session 001), `atomicWrite`/`dirName`/`tasksDirName`.
- Produces: `type Store struct{ root string }`; `func NewTaskStore(root string) *Store`; `Create`/`Get` implementing `mtt.TaskStore`.

- [ ] **Step 1: Write the failing test** — `internal/adapter/yaml/task_test.go`:

```go
package yaml

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

var _ mtt.TaskStore = (*Store)(nil)

func initDefault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	return root
}

func TestStoreCreateAndGet(t *testing.T) {
	root := initDefault(t)
	s := NewTaskStore(root)

	e1, err := s.Create(mtt.Task{Type: "epic", Title: "build auth", Status: "tbd", Created: fixedTime(), Updated: fixedTime()})
	if err != nil || e1.ID != "e1" {
		t.Fatalf("create epic = %q, %v; want e1", e1.ID, err)
	}
	if _, err := s.Create(mtt.Task{Type: "epic", Title: "second", Status: "tbd", Created: fixedTime(), Updated: fixedTime()}); err != nil {
		t.Fatal(err)
	}
	// independent per-prefix counter
	t1, err := s.Create(mtt.Task{Type: "task", Title: "orphan", Status: "tbd", Created: fixedTime(), Updated: fixedTime()})
	if err != nil || t1.ID != "t1" {
		t.Fatalf("create task = %q, %v; want t1", t1.ID, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".mtt", "tasks", "e1.yaml")); err != nil {
		t.Fatalf("e1 file missing: %v", err)
	}

	got, err := s.Get("e1")
	if err != nil {
		t.Fatalf("get e1: %v", err)
	}
	if got.Title != "build auth" || got.Type != "epic" || got.Status != "tbd" {
		t.Fatalf("round-trip lost fields: %+v", got)
	}

	if _, err := s.Get("nope"); !errors.Is(err, mtt.ErrNotFound) {
		t.Fatalf("get missing err = %v, want ErrNotFound", err)
	}
	if _, err := s.Create(mtt.Task{Type: "ghost", Status: "tbd", Created: fixedTime(), Updated: fixedTime()}); err == nil {
		t.Fatal("create with unknown type should error")
	}
}
```

Add `"os"` to the import block of this file.

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/adapter/yaml/ -run TestStoreCreateAndGet -v`
Expected: FAIL (undefined: `Store`, `NewTaskStore`).

- [ ] **Step 3: Create `internal/adapter/yaml/task.go`**

```go
package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	goyaml "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Store is the YAML implementation of mtt.TaskStore: one file per task under
// .mtt/tasks/, with flat per-prefix ID minting. It loads config lazily (for the
// type->prefix map) so Get stays independent of config.
type Store struct {
	root string
}

// NewTaskStore returns a task store rooted at the given project directory.
func NewTaskStore(root string) *Store { return &Store{root: root} }

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
	data, err := goyaml.Marshal(fromDomainTask(t))
	if err != nil {
		return mtt.Task{}, fmt.Errorf("marshal task %s: %w", id, err)
	}
	path := filepath.Join(s.root, dirName, tasksDirName, id+".yaml")
	if err := atomicWrite(path, data); err != nil {
		return mtt.Task{}, err
	}
	return t, nil
}

// Get loads a task by ID, returning mtt.ErrNotFound when the file is absent.
func (s *Store) Get(id string) (mtt.Task, error) {
	path := filepath.Join(s.root, dirName, tasksDirName, id+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mtt.Task{}, mtt.ErrNotFound
		}
		return mtt.Task{}, fmt.Errorf("read %s: %w", path, err)
	}
	var yt ymlTask
	if err := goyaml.Unmarshal(data, &yt); err != nil {
		return mtt.Task{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return yt.toDomain()
}
```

- [ ] **Step 4: Run — verify pass**

Run: `go test ./internal/adapter/yaml/ -v`
Expected: PASS (all yaml tests).

- [ ] **Step 5: Update `internal/adapter/yaml/CLAUDE.md`** — under "## Responsibilities", add:

```markdown
- `NewTaskStore(root)` / `Store` — implements `mtt.TaskStore`. `Create` mints a **flat per-prefix** ID (`<prefix><N>` via `mint`, scan `max+1`, `O_EXCL` reserve), serializes the `ymlTask` DTO (RFC3339 UTC, `omitempty` on reserved fields), and writes atomically to `.mtt/tasks/<id>.yaml`. `Get` reads/maps a task, returning `mtt.ErrNotFound` when absent. IDs are flat (no parent chain) → stable under re-parenting; identity lives in the ID, hierarchy in the `parent` field.
```

Also update the "Current state"/scope line to mention tasks are now handled (not just config).

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/yaml/task.go internal/adapter/yaml/task_test.go internal/adapter/yaml/CLAUDE.md
git commit -m "feat(yaml): Store implements TaskStore (Create mints, Get loads)"
```

---

### Task 6: Core — the `add` usecase

**Files:**
- Create: `internal/core/add.go`
- Test: `internal/core/add_test.go`
- Create: `internal/core/CLAUDE.md`

**Interfaces:**
- Consumes: `mtt.TaskStore`, `mtt.Config`, `mtt.Type` helpers (`DefaultType`, `TypeByName`, `IsRoot`, `InitialStatus`).
- Produces: `type Adder`; `func NewAdder(store mtt.TaskStore, cfg mtt.Config, now func() time.Time) *Adder`; `type AddParams struct{ Title, TypeName, Description string; NoParent bool }`; `func (a *Adder) Add(p AddParams) (mtt.Task, error)`.

- [ ] **Step 1: Write the failing test** — `internal/core/add_test.go`:

```go
package core

import (
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

type fakeStore struct {
	got   mtt.Task
	retID string
}

func (f *fakeStore) Create(t mtt.Task) (mtt.Task, error) {
	f.got = t
	t.ID = f.retID
	return t, nil
}
func (f *fakeStore) Get(string) (mtt.Task, error) { return mtt.Task{}, mtt.ErrNotFound }

func flow() mtt.Flow {
	return mtt.Flow{
		Statuses: []mtt.Status{
			{Name: "tbd", Kind: mtt.KindInitial},
			{Name: "doing", Kind: mtt.KindActive},
			{Name: "done", Kind: mtt.KindTerminal},
		},
		Transitions: []mtt.Transition{{From: "tbd", To: "doing"}, {From: "doing", To: "done"}},
	}
}

func cfg() mtt.Config {
	return mtt.Config{Types: []mtt.Type{
		{Name: "epic", Flow: flow()},
		{Name: "task", Parents: []string{"epic"}, Default: true, Flow: flow()},
	}}
}

func fixed() time.Time { return time.Date(2026, 7, 4, 9, 20, 30, 500, time.UTC) }

func TestAddRootExplicitType(t *testing.T) {
	fs := &fakeStore{retID: "e1"}
	got, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "build auth", TypeName: "epic"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "e1" {
		t.Fatalf("id = %q, want e1", got.ID)
	}
	if fs.got.Type != "epic" || fs.got.Status != "tbd" || fs.got.Title != "build auth" {
		t.Fatalf("logical task wrong: %+v", fs.got)
	}
	if !fs.got.Created.Equal(fixed().Truncate(time.Second)) || !fs.got.Updated.Equal(fs.got.Created) {
		t.Fatalf("timestamps: created=%v updated=%v", fs.got.Created, fs.got.Updated)
	}
	if fs.got.Parent != "" {
		t.Fatalf("root task must have empty parent, got %q", fs.got.Parent)
	}
}

func TestAddDefaultTypeNeedsParent(t *testing.T) {
	_, err := NewAdder(&fakeStore{retID: "t1"}, cfg(), fixed).Add(AddParams{Title: "x"})
	if err == nil || !strings.Contains(err.Error(), "requires a parent") {
		t.Fatalf("want 'requires a parent', got %v", err)
	}
}

func TestAddNoParentException(t *testing.T) {
	fs := &fakeStore{retID: "t1"}
	got, err := NewAdder(fs, cfg(), fixed).Add(AddParams{Title: "orphan", NoParent: true})
	if err != nil || got.ID != "t1" || fs.got.Type != "task" {
		t.Fatalf("no-parent create failed: id=%q type=%q err=%v", got.ID, fs.got.Type, err)
	}
}

func TestAddUnknownType(t *testing.T) {
	_, err := NewAdder(&fakeStore{}, cfg(), fixed).Add(AddParams{Title: "x", TypeName: "ghost"})
	if err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("want 'unknown type', got %v", err)
	}
}

func TestAddNeedsTitleOrDescription(t *testing.T) {
	_, err := NewAdder(&fakeStore{}, cfg(), fixed).Add(AddParams{TypeName: "epic"})
	if err == nil || !strings.Contains(err.Error(), "title or a description") {
		t.Fatalf("want title-or-description error, got %v", err)
	}
	// description-only is allowed
	if _, err := NewAdder(&fakeStore{retID: "e1"}, cfg(), fixed).Add(AddParams{TypeName: "epic", Description: "figure it out"}); err != nil {
		t.Fatalf("description-only should be allowed: %v", err)
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/core/ -v`
Expected: FAIL (package/symbols undefined).

- [ ] **Step 3: Create `internal/core/add.go`**

```go
// Package core holds mtt's usecase logic. It depends only on the pkg/mtt domain
// contract and its ports — never on a concrete adapter.
package core

import (
	"fmt"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// Adder is the create-a-task usecase. It resolves the type, enforces placement,
// picks the entry status, stamps timestamps, and persists via the TaskStore.
type Adder struct {
	store mtt.TaskStore
	cfg   mtt.Config
	now   func() time.Time
}

// NewAdder wires the usecase. now is injected so timestamps are deterministic in
// tests (pass time.Now in production).
func NewAdder(store mtt.TaskStore, cfg mtt.Config, now func() time.Time) *Adder {
	return &Adder{store: store, cfg: cfg, now: now}
}

// AddParams are the inputs to Add. TypeName empty selects the default type.
// NoParent creates a parent-requiring type at top level (a conscious exception).
type AddParams struct {
	Title       string
	TypeName    string
	NoParent    bool
	Description string
}

// Add creates one task and returns it with the adapter-minted ID.
func (a *Adder) Add(p AddParams) (mtt.Task, error) {
	if p.Title == "" && p.Description == "" {
		return mtt.Task{}, fmt.Errorf("provide a title or a description")
	}
	var (
		typ mtt.Type
		ok  bool
	)
	if p.TypeName != "" {
		if typ, ok = a.cfg.TypeByName(p.TypeName); !ok {
			return mtt.Task{}, fmt.Errorf("unknown type %q", p.TypeName)
		}
	} else if typ, ok = a.cfg.DefaultType(); !ok {
		return mtt.Task{}, fmt.Errorf("no types configured")
	}
	if !typ.IsRoot() && !p.NoParent {
		return mtt.Task{}, fmt.Errorf("type %q requires a parent; use --parent (session 004) or --no-parent to create it top-level", typ.Name)
	}
	initial, ok := typ.InitialStatus()
	if !ok {
		return mtt.Task{}, fmt.Errorf("type %q has no initial status", typ.Name)
	}
	now := a.now().UTC().Truncate(time.Second)
	return a.store.Create(mtt.Task{
		Type:        typ.Name,
		Title:       p.Title,
		Status:      initial.Name,
		Description: p.Description,
		Created:     now,
		Updated:     now,
	})
}
```

- [ ] **Step 4: Run — verify pass**

Run: `go test ./internal/core/ -v`
Expected: PASS (all five tests).

- [ ] **Step 5: Create `internal/core/CLAUDE.md`**

```markdown
# internal/core

Usecase logic. Depends **only** on the `pkg/mtt` domain contract and its ports — **never** on `adapter/*`.

## Responsibilities

- `Adder` (the `add` usecase): resolve the type (`--type` or the config default), enforce placement
  (a non-root type needs `--no-parent` here since `--parent` is session 004), pick the entry status
  (`Type.InitialStatus` — default-marked initial, else first initial), stamp `created`/`updated` from an
  **injected clock**, and persist via `TaskStore.Create` (which mints the ID in the adapter).

## Boundaries

- No storage access, no ID minting, no output formatting, no YAML — those live in the adapter / CLI.
- The clock is injected (`now func() time.Time`) for deterministic tests.
- Policy lives here; the pure primitives it composes (`IsRoot`, `InitialStatus`, `TypeByName`, `DefaultType`)
  live in `pkg/mtt`.
```

- [ ] **Step 6: Commit**

```bash
git add internal/core/add.go internal/core/add_test.go internal/core/CLAUDE.md
git commit -m "feat(core): add usecase (type/placement/entry-status policy)"
```

---

### Task 7: CLI — `mtt add`

**Files:**
- Create: `internal/cli/add.go`
- Modify: `internal/cli/root.go`
- Test: `internal/cli/add_test.go`

**Interfaces:**
- Consumes: `yaml.FindRoot`/`yaml.Load`/`yaml.NewTaskStore`, `core.NewAdder`/`core.AddParams`, `NewRootCmd`, the existing `chdir` test helper (in `init_test.go`).
- Produces: `func newAddCmd() *cobra.Command`; registered in `NewRootCmd`.

- [ ] **Step 1: Write the failing test** — `internal/cli/add_test.go`:

```go
package cli

import (
	"strings"
	"testing"
)

func runOut(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out, errb strings.Builder
	root := NewRootCmd()
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errb.String(), err
}

func TestAddCommand(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	out, _, err := runOut(t, "add", "--type", "epic", "fix login")
	if err != nil {
		t.Fatalf("add epic: %v", err)
	}
	if !strings.Contains(out, "e1") {
		t.Fatalf("add output = %q, want it to mention e1", out)
	}

	// default type (task) requires a parent
	if _, _, err := runOut(t, "add", "just a task"); err == nil {
		t.Fatal("bare add of default type should error (needs parent)")
	}

	// --no-parent creates the default type at top level
	out, _, err = runOut(t, "add", "--no-parent", "orphan")
	if err != nil {
		t.Fatalf("add --no-parent: %v", err)
	}
	if !strings.Contains(out, "t1") {
		t.Fatalf("no-parent output = %q, want t1", out)
	}

	if _, _, err := runOut(t, "add", "--type", "ghost", "x"); err == nil {
		t.Fatal("unknown type should error")
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/cli/ -run TestAddCommand -v`
Expected: FAIL (unknown command "add").

- [ ] **Step 3: Create `internal/cli/add.go`**

```go
package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/internal/core"
)

// newAddCmd builds `mtt add [title]`: create a task.
func newAddCmd() *cobra.Command {
	var (
		typeName string
		noParent bool
		desc     string
	)
	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Create a task",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			root, err := yaml.FindRoot(cwd)
			if err != nil {
				return err
			}
			cfg, _, err := yaml.Load(root)
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}
			title := ""
			if len(args) == 1 {
				title = args[0]
			}
			adder := core.NewAdder(yaml.NewTaskStore(root), cfg, time.Now)
			task, err := adder.Add(core.AddParams{Title: title, TypeName: typeName, NoParent: noParent, Description: desc})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", task.ID)
			return err
		},
	}
	cmd.Flags().StringVar(&typeName, "type", "", "task type (default: the config's default type)")
	cmd.Flags().BoolVar(&noParent, "no-parent", false, "create a parent-requiring type at top level (conscious exception)")
	cmd.Flags().StringVar(&desc, "description", "", "task description")
	return cmd
}
```

- [ ] **Step 4: Register in `internal/cli/root.go`** — change the `AddCommand` line to:

```go
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd())
```

- [ ] **Step 5: Run — verify pass**

Run: `go test ./internal/cli/ -run TestAddCommand -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/add.go internal/cli/root.go internal/cli/add_test.go
git commit -m "feat(cli): mtt add"
```

---

### Task 8: CLI — `mtt show`

**Files:**
- Create: `internal/cli/show.go`
- Modify: `internal/cli/root.go`
- Test: `internal/cli/show_test.go`
- Modify: `internal/cli/CLAUDE.md`

**Interfaces:**
- Consumes: `yaml.FindRoot`/`yaml.NewTaskStore`, `mtt.ErrNotFound`, `mtt.Task`.
- Produces: `func newShowCmd() *cobra.Command`; `func formatTask(t mtt.Task) string`; registered in `NewRootCmd`.

- [ ] **Step 1: Write the failing test** — `internal/cli/show_test.go`:

```go
package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestFormatTask(t *testing.T) {
	ts := time.Date(2026, 7, 4, 9, 20, 0, 0, time.UTC)
	got := formatTask(mtt.Task{ID: "t1", Type: "task", Title: "fix login", Status: "tbd",
		Parent: "e1", Created: ts, Updated: ts, Description: "do the thing"})
	want := "t1  task  [tbd]\n" +
		"  title:    fix login\n" +
		"  parent:   e1\n" +
		"  created:  2026-07-04T09:20:00Z\n" +
		"  updated:  2026-07-04T09:20:00Z\n" +
		"\n  do the thing\n"
	if got != want {
		t.Fatalf("formatTask mismatch:\n got: %q\nwant: %q", got, want)
	}
	// no title, no parent, no description => those lines are omitted
	bare := formatTask(mtt.Task{ID: "e1", Type: "epic", Status: "tbd", Created: ts, Updated: ts})
	if strings.Contains(bare, "title:") || strings.Contains(bare, "parent:") {
		t.Fatalf("bare task should omit title/parent lines: %q", bare)
	}
}

func TestShowCommand(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, _, err := runOut(t, "add", "--type", "epic", "fix login"); err != nil {
		t.Fatalf("add: %v", err)
	}
	out, _, err := runOut(t, "show", "e1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	for _, want := range []string{"e1", "epic", "tbd", "fix login"} {
		if !strings.Contains(out, want) {
			t.Fatalf("show output %q missing %q", out, want)
		}
	}
	if _, _, err := runOut(t, "show", "missing"); err == nil {
		t.Fatal("show of a missing id should error")
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/cli/ -run 'TestFormatTask|TestShowCommand' -v`
Expected: FAIL (undefined `formatTask`; unknown command "show").

- [ ] **Step 3: Create `internal/cli/show.go`**

```go
package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newShowCmd builds `mtt show <id>`: display a task.
func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			root, err := yaml.FindRoot(cwd)
			if err != nil {
				return err
			}
			task, err := yaml.NewTaskStore(root).Get(args[0])
			if err != nil {
				if errors.Is(err, mtt.ErrNotFound) {
					return fmt.Errorf("task %q not found", args[0])
				}
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), formatTask(task))
			return err
		},
	}
}

// formatTask renders a task as a human-readable block. The parent line shows the
// raw parent ID; the computed lineage ("you are here") arrives in session 004.
func formatTask(t mtt.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s  [%s]\n", t.ID, t.Type, t.Status)
	if t.Title != "" {
		fmt.Fprintf(&b, "  title:    %s\n", t.Title)
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

- [ ] **Step 4: Register in `internal/cli/root.go`** — change the `AddCommand` line to:

```go
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd(), newShowCmd())
```

- [ ] **Step 5: Run — verify pass**

Run: `go test ./internal/cli/ -v`
Expected: PASS (all cli tests).

- [ ] **Step 6: Update `internal/cli/CLAUDE.md`** — change the "Current state" section's command list to include `add` + `show`, e.g.:

```markdown
`root` + `version` + `init` + `types` + `add` + `show`. `add`/`show` wire the YAML `TaskStore` into the
`core` add usecase (composition root); `show` formats a task via `formatTask`. Next (session 003): `list`/`edit`.
```

- [ ] **Step 7: Commit**

```bash
git add internal/cli/show.go internal/cli/root.go internal/cli/show_test.go internal/cli/CLAUDE.md
git commit -m "feat(cli): mtt show"
```

---

### Task 9: CLI — end-to-end `testscript`

**Files:**
- Create: `internal/cli/testdata/scripts/add_show.txt`

**Interfaces:**
- Consumes: the `mtt` testscript command (registered in `script_test.go`), the `default` template.

- [ ] **Step 1: Write the e2e script** — `internal/cli/testdata/scripts/add_show.txt`:

```
# init, then create and view tasks
mkdir proj
cd proj
exec mtt init
stdout 'initialized'

# explicit root type -> e1, then e2
exec mtt add --type epic 'fix login'
stdout 'created e1'
exec mtt add --type epic 'second epic'
stdout 'created e2'

# default type (task) requires a parent
! exec mtt add 'bare task'
stderr 'requires a parent'

# --no-parent creates the default type (task) at top level -> t1
exec mtt add --no-parent 'orphan task'
stdout 'created t1'

# show renders id, type, status, title
exec mtt show e1
stdout 'e1  epic  \[tbd\]'
stdout 'fix login'

exec mtt show t1
stdout 'task'
stdout 'orphan task'

# unknown type and missing id error
! exec mtt add --type nope 'x'
stderr 'unknown type'
! exec mtt show missing
stderr 'not found'

# outside a project: not initialized
cd $WORK/empty
! exec mtt add --no-parent 'x'
stderr 'not initialized'
! exec mtt show e1
stderr 'not initialized'

-- empty/.keep --
```

- [ ] **Step 2: Run — verify pass**

Run: `go test ./internal/cli/ -run TestScripts -v`
Expected: PASS (`add_show.txt` and the existing `init.txt`).

- [ ] **Step 3: Full gate**

Run: `make check`
Expected: PASS (fmt + vet + lint + `go test -race -cover` + build all green).

- [ ] **Step 4: Commit**

```bash
git add internal/cli/testdata/scripts/add_show.txt
git commit -m "test(cli): e2e add -> show acceptance scenario"
```

---

### Task 10: Docs — reconcile DESIGN/AGENTS/TASKS/CLI_REFERENCE + session close

**Files:**
- Modify: `DESIGN.md`, `DESIGN.ru.md`, `AGENTS.md`, `TASKS.md`, `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md`, `sessions/002_create_and_view.md`, `NEXT_SESSION.md`

No code; this task records the two decisions (flat IDs, `--no-parent`/`Status.Default`) in the source-of-truth docs and closes the session. English is the source of truth; mirror each change in the `.ru` file.

- [ ] **Step 1: AGENTS.md — storage invariant.** In the "## Storage invariants" section, replace the line `- IDs are stable (\`e1_t3_s2\`) and independent of \`title\`.` with:

```markdown
- IDs are **flat, per-prefix** (`e1`, `t17`) and independent of `title` **and of position** — re-parenting
  changes only the `parent` field, never the ID. (Hierarchy is stored in `parent`, computed for display.)
```

- [ ] **Step 2: DESIGN.md — ID scheme.** In "## Types and hierarchy (domain) vs ID/slug (adapter)", replace the paragraph beginning "In the YAML adapter the ID is built by walking the parent chain" with:

```markdown
In the YAML adapter the ID is **flat and per-prefix**: `<prefix><N>`, where `N` is sequential per prefix
(`max+1`, `O_EXCL`). The ID does **not** encode the parent chain (`epic` #1 → `e1`; `task` #17 → `t17`;
`subtask` #3 → `s3`), so identity is decoupled from position: **re-parenting** a task changes only its
`parent` field — the ID stays stable and the file is not renamed. The ID is **stable** and independent of
text; the name lives in `title`; hierarchy lives in `parent` and is **computed** for display (e.g. `mtt show`
renders the lineage). The file name = `<id>.yaml`.
```

Update the Task-model example's `id: e1_t3_s2` / `parent: e1_t3` to the flat scheme (`id: s3`, `parent: t17`),
and in the "Decisions" table change the `ID/slug` row's `stable e1_t3_s2` to `stable flat per-prefix e1/t17/s3`.

- [ ] **Step 3: DESIGN.md — record `--no-parent` and `Status.Default`.** In "### Model invariants (checked on config load)", after the "Default type" bullet, add:

```markdown
- **Entry status.** A task starts at its type's **initial** status. When a flow has more than one `initial`,
  the entry is the one marked `default: true` on the status (mirrors the default **type** marker), else the
  first `initial` in config order (`Type.InitialStatus`). Validation: at most one `default` status per flow,
  and a `default` status must be `initial`.
- **`add` placement / `--no-parent`.** A type whose `parents` is non-empty requires a parent (`--parent`);
  as a **conscious exception**, `--no-parent` creates it at top level (a flat root ID). Root types need
  neither. (This keeps the user from ever being blocked from creating a top-level task.)
```

- [ ] **Step 4: DESIGN.md — backlog.** In the "## Implementation order" area / the "Later (coarse)" backlog (mirror the list that lives in TASKS.md), add three items:

```markdown
- later — **re-parenting** (`mtt reparent`/`move`): change a task's `parent`; enabled by flat, position-free IDs.
- later — **tags**: a cross-cutting `[]string` label on tasks (reserved in the model now); filtering lands with `list`.
- later — **boards / views**: a query/view over tags/status/type (relates to `list` and `mtt-ui`); the backlog is such a view.
```

- [ ] **Step 5: Mirror in DESIGN.ru.md** — apply the same four changes (Steps 2–4) to the Russian mirror, matching its existing wording/structure.

- [ ] **Step 6: TASKS.md** — under the e2 section note flat IDs (replace any `e1_t3_s2` phrasing with the flat scheme), and under "## Later (coarse)" add the reparent / tags / boards bullets from Step 4.

- [ ] **Step 7: CLI_REFERENCE.md** — add entries for the two commands (place them near `init`/`types`, matching the file's existing format):

```markdown
### `mtt add [title] [flags]`

Create a task. Provide a `title` (positional) and/or `--description`; at least one is required.

- `--type <name>` — the task type (default: the config's default type).
- `--no-parent` — create a parent-requiring type at top level (a conscious exception; `--parent` arrives in a
  later session).
- `--description <text>` — the task description.

Prints `created <id>` (a flat, per-prefix ID such as `e1` or `t17`). A non-root default type without
`--no-parent` errors and tells you how to proceed.

### `mtt show <id>`

Show a task: id, type, status, title, timestamps, and description.
```

- [ ] **Step 8: Mirror in CLI_REFERENCE.ru.md** — add the Russian equivalents of both command entries.

- [ ] **Step 9: sessions/002_create_and_view.md** — fill the "## Done" section:

```markdown
## Done

Shipped `mtt add [title] [--type] [--no-parent] [--description]` and `mtt show <id>` over the full hexagon:
the `Task` model + `TaskStore` port (`pkg/mtt`), the `add` usecase (`internal/core`, injected clock), and the
YAML task store with **flat per-prefix ID minting** + deterministic RFC3339 serialization.

Decisions taken in the brainstorm (see the spec): flat, position-free IDs (re-parenting keeps IDs stable);
`--no-parent` as a conscious top-level exception; `Status.Default` as the multi-initial entry marker; title
**or** description required. Reserved (model only): `tags`/`refs`/`comments`/`history`.

Acceptance: `add_show.txt` e2e + a golden task file + `make check` green. Deferred: `--parent`/hierarchy and
the `show` lineage line → 004; `list`/`edit` → 003.
```

- [ ] **Step 10: NEXT_SESSION.md** — update "## Where we are" and "## Next task" to point at session 003 (list & edit): note that 002 is done/merged, that the `Task` model + `TaskStore` + `core` + flat ID minting are now in place, and that 003 adds `mtt list` (filters) + `mtt edit`.

- [ ] **Step 11: Gate + commit**

Run: `make check`
Expected: PASS.

```bash
git add DESIGN.md DESIGN.ru.md AGENTS.md TASKS.md CLI_REFERENCE.md CLI_REFERENCE.ru.md sessions/002_create_and_view.md NEXT_SESSION.md
git commit -m "docs(session-002): flat IDs, --no-parent, Status.Default; close session"
```

---

## Self-Review

**Spec coverage** (spec §→ task):
- §4 Task model + value objects + `Status.Default` → Tasks 1, 2. §5 Validate invariants → Task 1. §6 `TaskStore` + `ErrNotFound` → Task 2. §7 flat minting + DTO/serialization + Store → Tasks 3, 4, 5. §8 core add policy (title-or-desc, default/explicit type, `--no-parent`, entry status, clock) → Task 6. §9 CLI add/show + `formatTask` → Tasks 7, 8. §10 doc reconciliation → Tasks 2/5/6/8 (CLAUDE.md) + Task 10 (project docs). §12 tests (pkg/core/adapter/cli + golden + round-trip) → Tasks 1–9. §13 acceptance (`add_show.txt`, golden, `make check`) → Task 9. §14 deferred seams → not built (correct).
- All spec sections map to a task. No gaps.

**Placeholder scan:** No TBD/TODO/"handle edge cases"; every code step shows complete code; every doc step gives the literal text to write.

**Type consistency:** `mtt.TaskStore` (Create/Get) is defined in Task 2 and consumed identically in Tasks 5 (impl), 6 (fake), 7/8 (wiring). `fromDomainTask`/`ymlTask.toDomain` defined in Task 4, used in Task 5. `mint(root, prefix)` defined Task 3, used Task 5. `core.NewAdder(store, cfg, now)` / `core.AddParams{Title,TypeName,NoParent,Description}` defined Task 6, used Task 7. `formatTask(mtt.Task) string` defined Task 8, tested Task 8. `Status.Default`, `Type.IsRoot/InitialStatus`, `Config.TypeByName` defined Task 1, used Task 6. `fixedTime()` (yaml tests) defined in Task 4's test file and reused in Task 5's test (same package). `*update` reused from `init_test.go` (not redeclared). Consistent throughout.
