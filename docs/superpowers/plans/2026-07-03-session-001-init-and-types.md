# Session 001 — Init & inspect — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `mtt init [--template default|coding] [--force] [--name]` and `mtt types [<type>]` — the first vertical slice through the pure `pkg/mtt` contract, the YAML config adapter, and the CLI.

**Architecture:** Hexagonal, config-as-data. `pkg/mtt` holds pure domain types (no serialization tags, no `prefix`) + `Config.Validate()` + helpers. `internal/adapter/yaml` maps its own DTOs to/from the domain and owns file I/O, embedded templates, and ID-encoding fields. `internal/cli` is the composition root: it wires the adapter, calls `Validate`, formats output. No `internal/core` this session (deferred to 002).

**Tech Stack:** Go 1.23.1, cobra, `gopkg.in/yaml.v3` (config parsing), `github.com/rogpeppe/go-internal/testscript` (e2e). Authoritative design: [../specs/2026-07-03-session-001-init-and-types-design.md](../specs/2026-07-03-session-001-init-and-types-design.md).

## Global Constraints

- Module path `github.com/pashukhin/mtt`; Go `1.23.1`.
- **TDD**: every step order is test → red → implement → green → commit. `make check` (fmt-check + vet + lint + `go test -race -cover` + build) must be green before the branch is done.
- **Clean architecture**: `pkg/mtt` imports nothing of ours and has **no yaml/json tags, no `prefix`**. `internal/adapter/yaml` maps DTOs↔domain and carries **no business rules** beyond provider-specific checks (prefix, exactly-one-default). `internal/cli` may import both. No package imports `internal/core` (it does not exist yet).
- **DDD / name-agnostic**: no literals for type/status **names** in code; `kind` is the `StatusKind` value object. Names live only in the embedded templates.
- **Lint**: golangci-lint v2 (`standard` + gocritic, revive, misspell, unconvert). Every exported identifier has a doc comment ending in a period; wrap errors with `%w`; no ignored errors in non-test code. goimports local-prefix `github.com/pashukhin/mtt`.
- **Docs language**: agent-facing docs (CLAUDE.md files) English only.
- Every commit ends with the trailer `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.
- Branch: `feat/s001-init-and-types` (already created; doc reconciliation already committed as `20b8085`).

## File Structure

**`pkg/mtt/`** — pure domain contract:
- `kind.go` — package doc + `StatusKind` value object + constants + `Valid()`.
- `config.go` — `Config`, `Project`, `Type`, `Flow`, `Status`, `Transition` + `DefaultType()` + `ChildrenIn()`.
- `validate.go` — `Config.Validate()` + `Type.validateFlow()`.
- `CLAUDE.md`, plus `kind_test.go`, `validate_test.go`, `config_test.go`.

**`internal/adapter/yaml/`** — default driven adapter (config layer):
- `dto.go` — package doc + on-disk DTOs (with yaml tags + `prefix`) + `toDomain()` + `checkPrefixes()`.
- `root.go` — `FindRoot`, dir/file name constants, `ErrNotInitialized`.
- `templates.go` + `templates/default.yaml` + `templates/coding.yaml` — embedded templates + `renderTemplate()`.
- `init.go` — `Init`, `ErrAlreadyInitialized`, `atomicWrite`.
- `load.go` — `Load`, `decodeInto`.
- `CLAUDE.md`, plus `dto_test.go`, `root_test.go`, `init_test.go`, `load_test.go`, `testdata/golden/`.

**`internal/cli/`** — commands (composition root):
- `init.go` — `newInitCmd`. `types.go` — `newTypesCmd` + `formatTypes`/`writeTypeBlock`.
- `root.go` — register the two commands (modify).
- `CLAUDE.md` (modify), plus `types_test.go`, `script_test.go`, `testdata/scripts/init.txt`.

---

### Task 1: `pkg/mtt` — `StatusKind` value object

**Files:**
- Create: `pkg/mtt/kind.go`
- Test: `pkg/mtt/kind_test.go`

**Interfaces:**
- Produces: `type StatusKind string`; consts `KindInitial`, `KindActive`, `KindTerminal StatusKind`; `func (StatusKind) Valid() bool`.

- [ ] **Step 1: Write the failing test**

`pkg/mtt/kind_test.go`:
```go
package mtt

import "testing"

func TestStatusKindValid(t *testing.T) {
	for _, k := range []StatusKind{KindInitial, KindActive, KindTerminal} {
		if !k.Valid() {
			t.Errorf("Valid(%q) = false, want true", k)
		}
	}
	for _, k := range []StatusKind{"", "todo", "Initial", "done"} {
		if k.Valid() {
			t.Errorf("Valid(%q) = true, want false", k)
		}
	}
}

func TestStatusKindConstants(t *testing.T) {
	if KindInitial != "initial" || KindActive != "active" || KindTerminal != "terminal" {
		t.Fatalf("kind constants drifted: %q %q %q", KindInitial, KindActive, KindTerminal)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/mtt/`
Expected: FAIL — build error (`StatusKind`, `KindInitial`, … undefined).

- [ ] **Step 3: Write minimal implementation**

`pkg/mtt/kind.go`:
```go
// Package mtt is the public domain contract for mtt: pure domain types and ports,
// free of storage/serialization concerns. Adapters map their own DTOs to and from
// these types; nothing here knows about YAML, files, or the CLI.
package mtt

// StatusKind is the category of a flow status — a closed domain vocabulary the
// code reasons about (ready/terminal logic), unlike open, user-defined status
// names. It is a value object, not a name.
type StatusKind string

// The three status kinds. Every flow needs at least one status of each.
const (
	KindInitial  StatusKind = "initial"
	KindActive   StatusKind = "active"
	KindTerminal StatusKind = "terminal"
)

// Valid reports whether k is one of the three defined kinds.
func (k StatusKind) Valid() bool {
	switch k {
	case KindInitial, KindActive, KindTerminal:
		return true
	default:
		return false
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/mtt/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/mtt/kind.go pkg/mtt/kind_test.go
git commit -m "feat(pkg/mtt): StatusKind value object" \
  -m "Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: `pkg/mtt` — domain structs + `Config.Validate()`

**Files:**
- Create: `pkg/mtt/config.go` (structs only in this task), `pkg/mtt/validate.go`
- Test: `pkg/mtt/validate_test.go`

**Interfaces:**
- Consumes: `StatusKind` and its constants (Task 1).
- Produces: structs `Config{Version int; Project Project; Types []Type}`, `Project{Name string}`, `Type{Name, Description string; Parents []string; Default bool; Flow}`, `Flow{Statuses []Status; Transitions []Transition}`, `Status{Name string; Kind StatusKind; Description string}`, `Transition{From, To, Description string; Commands []string}`; `func (Config) Validate() error`.

- [ ] **Step 1: Write the failing test**

`pkg/mtt/validate_test.go`:
```go
package mtt

import (
	"strings"
	"testing"
)

// linearFlow is a minimal valid flow: initial -> active -> terminal (+ a second terminal).
func linearFlow() Flow {
	return Flow{
		Statuses: []Status{
			{Name: "tbd", Kind: KindInitial},
			{Name: "doing", Kind: KindActive},
			{Name: "done", Kind: KindTerminal},
			{Name: "cancelled", Kind: KindTerminal},
		},
		Transitions: []Transition{
			{From: "tbd", To: "doing"},
			{From: "tbd", To: "cancelled"},
			{From: "doing", To: "done"},
			{From: "doing", To: "cancelled"},
		},
	}
}

func validConfig() Config {
	return Config{Types: []Type{{Name: "task", Default: true, Flow: linearFlow()}}}
}

func TestValidateOK(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"no types", func(c *Config) { c.Types = nil }, "at least one type"},
		{"dup type", func(c *Config) { c.Types = append(c.Types, c.Types[0]) }, "duplicate type"},
		{"two defaults", func(c *Config) {
			t2 := c.Types[0]
			t2.Name = "bug"
			c.Types = append(c.Types, t2)
		}, "at most one"},
		{"unknown parent", func(c *Config) { c.Types[0].Parents = []string{"ghost"} }, "unknown parent"},
		{"self parent", func(c *Config) { c.Types[0].Parents = []string{"task"} }, "its own parent"},
		{"dup status", func(c *Config) {
			c.Types[0].Statuses = append(c.Types[0].Statuses, Status{Name: "tbd", Kind: KindInitial})
		}, "duplicate status"},
		{"bad kind", func(c *Config) { c.Types[0].Statuses[0].Kind = "weird" }, "invalid kind"},
		{"transition to unknown", func(c *Config) {
			c.Types[0].Transitions = append(c.Types[0].Transitions, Transition{From: "tbd", To: "ghost"})
		}, "unknown status"},
		{"no active (2-status flow)", func(c *Config) {
			c.Types[0].Flow = Flow{
				Statuses:    []Status{{Name: "a", Kind: KindInitial}, {Name: "b", Kind: KindTerminal}},
				Transitions: []Transition{{From: "a", To: "b"}},
			}
		}, "no active status"},
		{"initial with incoming", func(c *Config) {
			c.Types[0].Transitions = append(c.Types[0].Transitions, Transition{From: "doing", To: "tbd"})
		}, "initial needs 0 incoming"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.mut(&c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/mtt/`
Expected: FAIL — `Config`, `Type`, … and `Validate` undefined.

- [ ] **Step 3: Write minimal implementation**

`pkg/mtt/config.go` (structs only — helpers arrive in Task 3):
```go
package mtt

// Config is a whole mtt configuration: project metadata and task types. The only
// mandatory field for a provider is at least one Type; the rest is optional.
type Config struct {
	Version int
	Project Project
	Types   []Type
}

// Project holds project-level metadata.
type Project struct {
	Name string
}

// Type is a task type: its name, optional description, allowed parent types, the
// default marker, and its flow. Mandatory: Name and a Flow whose statuses have a
// name+kind and whose transitions have from/to.
type Type struct {
	Name        string
	Description string
	Parents     []string
	Default     bool
	Flow
}

// Flow is a per-type status graph: a closed set of statuses and the transitions
// between them. Status identity is scoped to the flow; there are no cross-flow
// transitions.
type Flow struct {
	Statuses    []Status
	Transitions []Transition
}

// Status is one state in a flow. Kind is a value object; Description is optional.
type Status struct {
	Name        string
	Kind        StatusKind
	Description string
}

// Transition is a directed edge between two statuses of the same flow. Description
// and Commands are optional; Commands run as gates in a later phase.
type Transition struct {
	From        string
	To          string
	Description string
	Commands    []string
}
```

`pkg/mtt/validate.go`:
```go
package mtt

import (
	"errors"
	"fmt"
)

// Validate checks the structural, name-agnostic domain invariants and returns a
// joined error listing every violation (nil when the config is valid).
func (c Config) Validate() error {
	var errs []error
	if len(c.Types) == 0 {
		errs = append(errs, errors.New("config: at least one type is required"))
	}
	seen := make(map[string]bool, len(c.Types))
	defaults := 0
	for _, t := range c.Types {
		if seen[t.Name] {
			errs = append(errs, fmt.Errorf("type %q: duplicate type name", t.Name))
		}
		seen[t.Name] = true
		if t.Default {
			defaults++
		}
		errs = append(errs, t.validateFlow()...)
	}
	if defaults > 1 {
		errs = append(errs, fmt.Errorf("config: %d types marked default, want at most one", defaults))
	}
	for _, t := range c.Types {
		for _, p := range t.Parents {
			switch {
			case p == t.Name:
				errs = append(errs, fmt.Errorf("type %q: cannot be its own parent", t.Name))
			case !seen[p]:
				errs = append(errs, fmt.Errorf("type %q: unknown parent type %q", t.Name, p))
			}
		}
	}
	return errors.Join(errs...)
}

// validateFlow checks one type's flow: status-name uniqueness, kind validity,
// transition reference resolution, kind<->topology consistency, and >=1 of each kind.
func (t Type) validateFlow() []error {
	var errs []error
	known := make(map[string]bool, len(t.Statuses))
	for _, s := range t.Statuses {
		if known[s.Name] {
			errs = append(errs, fmt.Errorf("type %q: duplicate status %q", t.Name, s.Name))
		}
		known[s.Name] = true
		if !s.Kind.Valid() {
			errs = append(errs, fmt.Errorf("type %q status %q: invalid kind %q", t.Name, s.Name, s.Kind))
		}
	}
	in := make(map[string]int)
	out := make(map[string]int)
	for _, tr := range t.Transitions {
		if !known[tr.From] {
			errs = append(errs, fmt.Errorf("type %q: transition from unknown status %q", t.Name, tr.From))
		}
		if !known[tr.To] {
			errs = append(errs, fmt.Errorf("type %q: transition to unknown status %q", t.Name, tr.To))
		}
		out[tr.From]++
		in[tr.To]++
	}
	var haveInitial, haveActive, haveTerminal bool
	for _, s := range t.Statuses {
		switch s.Kind {
		case KindInitial:
			haveInitial = true
			if in[s.Name] != 0 || out[s.Name] < 1 {
				errs = append(errs, fmt.Errorf("type %q status %q: initial needs 0 incoming and >=1 outgoing (in=%d out=%d)", t.Name, s.Name, in[s.Name], out[s.Name]))
			}
		case KindActive:
			haveActive = true
			if in[s.Name] < 1 || out[s.Name] < 1 {
				errs = append(errs, fmt.Errorf("type %q status %q: active needs >=1 incoming and >=1 outgoing (in=%d out=%d)", t.Name, s.Name, in[s.Name], out[s.Name]))
			}
		case KindTerminal:
			haveTerminal = true
			if out[s.Name] != 0 || in[s.Name] < 1 {
				errs = append(errs, fmt.Errorf("type %q status %q: terminal needs 0 outgoing and >=1 incoming (in=%d out=%d)", t.Name, s.Name, in[s.Name], out[s.Name]))
			}
		}
	}
	if !haveInitial {
		errs = append(errs, fmt.Errorf("type %q: no initial status", t.Name))
	}
	if !haveActive {
		errs = append(errs, fmt.Errorf("type %q: no active status", t.Name))
	}
	if !haveTerminal {
		errs = append(errs, fmt.Errorf("type %q: no terminal status", t.Name))
	}
	return errs
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/mtt/`
Expected: PASS (all subtests).

- [ ] **Step 5: Commit**

```bash
git add pkg/mtt/config.go pkg/mtt/validate.go pkg/mtt/validate_test.go
git commit -m "feat(pkg/mtt): domain types + structural Config.Validate" \
  -m "Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: `pkg/mtt` — `DefaultType` + `ChildrenIn` + package CLAUDE.md

**Files:**
- Modify: `pkg/mtt/config.go` (append helpers)
- Create: `pkg/mtt/CLAUDE.md`
- Test: `pkg/mtt/config_test.go`

**Interfaces:**
- Produces: `func (Config) DefaultType() (Type, bool)`; `func (Type) ChildrenIn(Config) []Type`.

- [ ] **Step 1: Write the failing test**

`pkg/mtt/config_test.go`:
```go
package mtt

import "testing"

func TestDefaultType(t *testing.T) {
	c := Config{Types: []Type{{Name: "epic"}, {Name: "task", Default: true}}}
	got, ok := c.DefaultType()
	if !ok || got.Name != "task" {
		t.Fatalf("DefaultType = (%q,%v), want (task,true)", got.Name, ok)
	}
	// no marked default -> first type
	c2 := Config{Types: []Type{{Name: "epic"}, {Name: "task"}}}
	got2, ok2 := c2.DefaultType()
	if !ok2 || got2.Name != "epic" {
		t.Fatalf("fallback DefaultType = (%q,%v), want (epic,true)", got2.Name, ok2)
	}
	// no types -> false
	if _, ok3 := (Config{}).DefaultType(); ok3 {
		t.Fatalf("empty config DefaultType ok = true, want false")
	}
}

func TestChildrenIn(t *testing.T) {
	c := Config{Types: []Type{
		{Name: "epic"},
		{Name: "task", Parents: []string{"epic"}},
		{Name: "subtask", Parents: []string{"task"}},
	}}
	kids := c.Types[0].ChildrenIn(c)
	if len(kids) != 1 || kids[0].Name != "task" {
		t.Fatalf("ChildrenIn(epic) = %v, want [task]", names(kids))
	}
	if k := c.Types[2].ChildrenIn(c); len(k) != 0 {
		t.Fatalf("ChildrenIn(subtask) = %v, want []", names(k))
	}
}

func names(ts []Type) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name
	}
	return out
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/mtt/`
Expected: FAIL — `DefaultType`, `ChildrenIn` undefined.

- [ ] **Step 3: Write minimal implementation**

Append to `pkg/mtt/config.go`:
```go
// DefaultType returns the type marked default, or the first type when none is
// marked. The bool is false only when there are no types.
func (c Config) DefaultType() (Type, bool) {
	if len(c.Types) == 0 {
		return Type{}, false
	}
	for _, t := range c.Types {
		if t.Default {
			return t, true
		}
	}
	return c.Types[0], true
}

// ChildrenIn returns the types that declare t as a parent — the computed inverse
// of Parents — in config order.
func (t Type) ChildrenIn(c Config) []Type {
	var kids []Type
	for _, other := range c.Types {
		for _, p := range other.Parents {
			if p == t.Name {
				kids = append(kids, other)
				break
			}
		}
	}
	return kids
}
```

Create `pkg/mtt/CLAUDE.md`:
```markdown
# pkg/mtt

The **public domain contract**: pure types + (later) ports. No CLI, no files, no YAML.

## Rules

- **No serialization concerns**: no yaml/json struct tags, no adapter fields (e.g. `prefix`). Adapters own DTOs and map to/from these types.
- **Value objects over primitives**: `StatusKind` is a closed vocabulary (initial/active/terminal) the code reasons about. Type/status **names** are open and user-defined — never literals in code.
- **References by identity**: transitions reference statuses by name; cross-aggregate links are names/IDs, never pointers. Back-references (e.g. `ChildrenIn`) are **computed**, not stored.
- **Provider-agnostic**: mandatory minimum (a Type needs a name + a flow with statuses(name+kind) and transitions(from/to)); the rest is optional.

## Invariants (Config.Validate — structural, name-agnostic)

kind↔topology (initial: 0 in/≥1 out; active: ≥1 in/≥1 out; terminal: ≥1 in/0 out); ≥1 of each kind per flow; unique type/status names; transition refs resolve; ≤1 default type; parents exist and are not self.

## Boundaries

Does NOT: read/write storage, mint IDs, or format output. Those live in adapters / CLI.
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/mtt/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/mtt/config.go pkg/mtt/config_test.go pkg/mtt/CLAUDE.md
git commit -m "feat(pkg/mtt): DefaultType/ChildrenIn helpers + CLAUDE.md" \
  -m "Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: `internal/adapter/yaml` — DTOs, `toDomain`, `checkPrefixes` (+ yaml.v3 dep)

**Files:**
- Create: `internal/adapter/yaml/dto.go`
- Test: `internal/adapter/yaml/dto_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: `pkg/mtt` types (Tasks 1–3).
- Produces: unexported DTO structs `ymlConfig/ymlProject/ymlType/ymlStatus/ymlTransition`; `func (ymlConfig) toDomain() (mtt.Config, map[string]string)`; `func checkPrefixes(mtt.Config, map[string]string) error`.

- [ ] **Step 1: Add the dependency**

Run:
```bash
go get gopkg.in/yaml.v3@latest
go mod tidy
```
Expected: `gopkg.in/yaml.v3` appears in `go.mod` require block.

- [ ] **Step 2: Write the failing test**

`internal/adapter/yaml/dto_test.go`:
```go
package yaml

import (
	"strings"
	"testing"

	goyaml "gopkg.in/yaml.v3"

	"github.com/pashukhin/mtt/pkg/mtt"
)

const sampleConfig = `
version: 1
project: {name: demo}
types:
  - name: task
    description: A unit of work.
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: tbd, kind: initial}
      - {name: doing, kind: active}
      - {name: done, kind: terminal}
    transitions:
      - {from: tbd, to: doing}
      - {from: doing, to: done}
`

func TestToDomain(t *testing.T) {
	var yc ymlConfig
	if err := goyaml.Unmarshal([]byte(sampleConfig), &yc); err != nil {
		t.Fatal(err)
	}
	cfg, prefixes := yc.toDomain()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("mapped config invalid: %v", err)
	}
	if prefixes["task"] != "t" {
		t.Fatalf("prefix = %q, want t", prefixes["task"])
	}
	if cfg.Project.Name != "demo" {
		t.Fatalf("project name = %q, want demo", cfg.Project.Name)
	}
	if cfg.Types[0].Statuses[1].Kind != mtt.KindActive {
		t.Fatalf("status kind not mapped: %q", cfg.Types[0].Statuses[1].Kind)
	}
	if err := checkPrefixes(cfg, prefixes); err != nil {
		t.Fatalf("checkPrefixes rejected a good config: %v", err)
	}
}

func TestCheckPrefixes(t *testing.T) {
	cfg := mtt.Config{Types: []mtt.Type{{Name: "a", Default: true}, {Name: "b"}}}
	if err := checkPrefixes(cfg, map[string]string{"a": "x", "b": "x"}); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("duplicate prefix not reported: %v", err)
	}
	if err := checkPrefixes(cfg, map[string]string{"a": "x", "b": ""}); err == nil || !strings.Contains(err.Error(), "missing prefix") {
		t.Fatalf("missing prefix not reported: %v", err)
	}
	noDefault := mtt.Config{Types: []mtt.Type{{Name: "a"}, {Name: "b"}}}
	if err := checkPrefixes(noDefault, map[string]string{"a": "x", "b": "y"}); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("missing default not reported: %v", err)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/adapter/yaml/`
Expected: FAIL — `ymlConfig`, `toDomain`, `checkPrefixes` undefined.

- [ ] **Step 4: Write minimal implementation**

`internal/adapter/yaml/dto.go`:
```go
// Package yaml is the default driven adapter: it stores mtt config (and later
// tasks) as YAML files under .mtt/, mints IDs, and maps its own DTOs to and from
// the pure pkg/mtt domain. It carries no business rules beyond provider-specific
// checks (prefixes, exactly-one-default).
package yaml

import (
	"errors"
	"fmt"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// ymlConfig and friends are the on-disk DTOs: they hold the yaml tags and the
// adapter-only prefix, and are mapped to the domain by toDomain.
type ymlConfig struct {
	Version int        `yaml:"version"`
	Project ymlProject `yaml:"project"`
	Types   []ymlType  `yaml:"types"`
}

type ymlProject struct {
	Name string `yaml:"name"`
}

type ymlType struct {
	Name        string          `yaml:"name"`
	Description string          `yaml:"description"`
	Prefix      string          `yaml:"prefix"`
	Parents     []string        `yaml:"parents"`
	Default     bool            `yaml:"default"`
	Statuses    []ymlStatus     `yaml:"statuses"`
	Transitions []ymlTransition `yaml:"transitions"`
}

type ymlStatus struct {
	Name        string `yaml:"name"`
	Kind        string `yaml:"kind"`
	Description string `yaml:"description"`
}

type ymlTransition struct {
	From        string   `yaml:"from"`
	To          string   `yaml:"to"`
	Description string   `yaml:"description"`
	Commands    []string `yaml:"commands"`
}

// toDomain maps the DTO to the pure domain Config and the adapter-owned
// type-name -> prefix map.
func (yc ymlConfig) toDomain() (mtt.Config, map[string]string) {
	cfg := mtt.Config{Version: yc.Version, Project: mtt.Project{Name: yc.Project.Name}}
	prefixes := make(map[string]string, len(yc.Types))
	for _, yt := range yc.Types {
		t := mtt.Type{Name: yt.Name, Description: yt.Description, Parents: yt.Parents, Default: yt.Default}
		for _, ys := range yt.Statuses {
			t.Statuses = append(t.Statuses, mtt.Status{Name: ys.Name, Kind: mtt.StatusKind(ys.Kind), Description: ys.Description})
		}
		for _, yr := range yt.Transitions {
			t.Transitions = append(t.Transitions, mtt.Transition{From: yr.From, To: yr.To, Description: yr.Description, Commands: yr.Commands})
		}
		cfg.Types = append(cfg.Types, t)
		prefixes[yt.Name] = yt.Prefix
	}
	return cfg, prefixes
}

// checkPrefixes enforces the YAML provider's stricter rules: exactly one default
// type, and a present + unique prefix per type.
func checkPrefixes(cfg mtt.Config, prefixes map[string]string) error {
	var errs []error
	defaults := 0
	for _, t := range cfg.Types {
		if t.Default {
			defaults++
		}
	}
	if defaults != 1 {
		errs = append(errs, fmt.Errorf("config: %d types marked default, want exactly one", defaults))
	}
	seen := make(map[string]string, len(prefixes))
	for _, t := range cfg.Types {
		p := prefixes[t.Name]
		if p == "" {
			errs = append(errs, fmt.Errorf("type %q: missing prefix", t.Name))
			continue
		}
		if other, dup := seen[p]; dup {
			errs = append(errs, fmt.Errorf("type %q: prefix %q already used by type %q", t.Name, p, other))
		}
		seen[p] = t.Name
	}
	return errors.Join(errs...)
}
```

- [ ] **Step 5: Run test to verify it passes, then commit**

Run: `go test ./internal/adapter/yaml/`
Expected: PASS.
```bash
git add go.mod go.sum internal/adapter/yaml/dto.go internal/adapter/yaml/dto_test.go
git commit -m "feat(yaml): config DTOs + domain mapping + provider checks" \
  -m "Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: `internal/adapter/yaml` — `FindRoot`

**Files:**
- Create: `internal/adapter/yaml/root.go`
- Test: `internal/adapter/yaml/root_test.go`

**Interfaces:**
- Produces: consts `dirName=".mtt"`, `configName="config.yaml"`, `localConfigName="config.local.yaml"`; `var ErrNotInitialized error`; `func FindRoot(start string) (string, error)`.

- [ ] **Step 1: Write the failing test**

`internal/adapter/yaml/root_test.go`:
```go
package yaml

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFindRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, dirName), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, start := range []string{root, nested} {
		got, err := FindRoot(start)
		if err != nil {
			t.Fatalf("FindRoot(%s): %v", start, err)
		}
		if got != root {
			t.Fatalf("FindRoot(%s) = %s, want %s", start, got, root)
		}
	}
	if _, err := FindRoot(t.TempDir()); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("FindRoot(uninit) err = %v, want ErrNotInitialized", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/yaml/ -run TestFindRoot`
Expected: FAIL — `FindRoot`, `dirName`, `ErrNotInitialized` undefined.

- [ ] **Step 3: Write minimal implementation**

`internal/adapter/yaml/root.go`:
```go
package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// dirName is the mtt data directory created inside a project root.
const dirName = ".mtt"

// configName is the committed config file within dirName.
const configName = "config.yaml"

// localConfigName is the optional gitignored overlay within dirName.
const localConfigName = "config.local.yaml"

// ErrNotInitialized is returned when no .mtt directory is found.
var ErrNotInitialized = errors.New("mtt: not initialized (no .mtt directory found)")

// FindRoot walks up from start until it finds a directory that contains .mtt/,
// returning that directory. It returns ErrNotInitialized when none is found.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve start dir: %w", err)
	}
	for {
		info, statErr := os.Stat(filepath.Join(dir, dirName))
		if statErr == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotInitialized
		}
		dir = parent
	}
}
```

- [ ] **Step 4: Run test to verify it passes, then commit**

Run: `go test ./internal/adapter/yaml/ -run TestFindRoot`
Expected: PASS.
```bash
git add internal/adapter/yaml/root.go internal/adapter/yaml/root_test.go
git commit -m "feat(yaml): FindRoot root discovery" \
  -m "Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: `internal/adapter/yaml` — embedded templates + `Init` + golden test

**Files:**
- Create: `internal/adapter/yaml/templates/default.yaml`, `internal/adapter/yaml/templates/coding.yaml`, `internal/adapter/yaml/templates.go`, `internal/adapter/yaml/init.go`
- Test: `internal/adapter/yaml/init_test.go`, `internal/adapter/yaml/testdata/golden/{default,coding}.yaml` (generated)

**Interfaces:**
- Consumes: `dirName`, `configName` (Task 5).
- Produces: `func renderTemplate(name, projectName string) ([]byte, error)`; `var ErrAlreadyInitialized error`; `func Init(root, tmplName, projectName string, force bool) error`.

- [ ] **Step 1: Create the template files**

`internal/adapter/yaml/templates/default.yaml`:
```yaml
version: 1
project:
  name: {{.Name}}
types:
  - name: epic
    description: A large body of work spanning multiple tasks.
    prefix: e
    parents: []
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, description: "all epic tasks closed"}
      - {from: in_progress, to: cancelled}
  - name: task
    description: A unit of work under an epic.
    prefix: t
    parents: [epic]
    default: true
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, description: "review the spec, create a branch"}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, description: "quality gate"}
      - {from: in_progress, to: cancelled}
  - name: subtask
    description: A small step within a task.
    prefix: s
    parents: [task]
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done}
      - {from: in_progress, to: cancelled}
```

`internal/adapter/yaml/templates/coding.yaml`:
```yaml
version: 1
project:
  name: {{.Name}}
types:
  - name: feature
    description: A new capability. DoD — a branch and a green gate.
    prefix: f
    parents: []
    default: true
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, description: "create a feature branch"}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, description: "quality gate", commands: ["make lint", "make test"]}
      - {from: in_progress, to: cancelled}
  - name: bugfix
    description: A fix. DoD — a failing test first, then green.
    prefix: b
    parents: []
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, description: "write a failing test that reproduces the bug", commands: ["! make test"]}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, description: "quality gate", commands: ["make lint", "make test"]}
      - {from: in_progress, to: cancelled}
  - name: refactor
    description: A behavior-preserving change. DoD — no public-API diff, then green.
    prefix: r
    parents: []
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, description: "create a branch"}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, description: "no public-API change + quality gate", commands: ["git diff --exit-code -- pkg/", "make lint", "make test"]}
      - {from: in_progress, to: cancelled}
```

- [ ] **Step 2: Write the failing test**

`internal/adapter/yaml/init_test.go`:
```go
package yaml

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestRenderGolden(t *testing.T) {
	for _, name := range []string{"default", "coding"} {
		got, err := renderTemplate(name, "demo")
		if err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		golden := filepath.Join("testdata", "golden", name+".yaml")
		if *update {
			if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("read golden %s (run -update first): %v", golden, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s render != golden", name)
		}
	}
}

func TestRenderUnknownTemplate(t *testing.T) {
	if _, err := renderTemplate("nope", "demo"); err == nil {
		t.Fatal("want error for unknown template")
	}
}

func TestInit(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatalf("init: %v", err)
	}
	dst := filepath.Join(root, ".mtt", "config.yaml")
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if bytes.Contains(data, []byte("{{.Name}}")) || !bytes.Contains(data, []byte("name: demo")) {
		t.Fatalf("project name not substituted:\n%s", data)
	}
	if err := Init(root, "default", "demo", false); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("re-init err = %v, want ErrAlreadyInitialized", err)
	}
	if err := Init(root, "coding", "demo", true); err != nil {
		t.Fatalf("force re-init: %v", err)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/adapter/yaml/ -run 'TestRender|TestInit'`
Expected: FAIL — `renderTemplate`, `Init`, `ErrAlreadyInitialized` undefined.

- [ ] **Step 4: Write minimal implementation**

`internal/adapter/yaml/templates.go`:
```go
package yaml

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/default.yaml templates/coding.yaml
var templatesFS embed.FS

// templateFiles maps init template names to their embedded paths.
var templateFiles = map[string]string{
	"default": "templates/default.yaml",
	"coding":  "templates/coding.yaml",
}

// renderTemplate renders the named init template, substituting the project name.
func renderTemplate(name, projectName string) ([]byte, error) {
	path, ok := templateFiles[name]
	if !ok {
		return nil, fmt.Errorf("unknown template %q", name)
	}
	raw, err := templatesFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template %q: %w", name, err)
	}
	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Name string }{Name: projectName}); err != nil {
		return nil, fmt.Errorf("render template %q: %w", name, err)
	}
	return buf.Bytes(), nil
}
```

`internal/adapter/yaml/init.go`:
```go
package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrAlreadyInitialized is returned by Init when config exists and force is false.
var ErrAlreadyInitialized = errors.New("mtt: already initialized (.mtt/config.yaml exists; use --force)")

// Init writes .mtt/config.yaml under root from the named template, substituting
// the project name. It refuses to overwrite an existing config unless force is set.
// The write is atomic (temp file + rename).
func Init(root, tmplName, projectName string, force bool) error {
	content, err := renderTemplate(tmplName, projectName)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	dst := filepath.Join(dir, configName)
	if !force {
		if _, statErr := os.Stat(dst); statErr == nil {
			return ErrAlreadyInitialized
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("stat %s: %w", dst, statErr)
		}
	}
	return atomicWrite(dst, content)
}

// atomicWrite writes data to path via a temp file in the same directory + rename.
func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Generate goldens, verify, commit**

Run:
```bash
go test ./internal/adapter/yaml/ -run TestRenderGolden -update
go test ./internal/adapter/yaml/ -run 'TestRender|TestInit'
```
Expected: second run PASS. Inspect `testdata/golden/default.yaml` — it must be the template with `{{.Name}}` replaced by `demo`.
```bash
git add internal/adapter/yaml/templates internal/adapter/yaml/templates.go internal/adapter/yaml/init.go internal/adapter/yaml/init_test.go internal/adapter/yaml/testdata/golden
git commit -m "feat(yaml): embedded templates + atomic Init + golden test" \
  -m "Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: `internal/adapter/yaml` — `Load` (+ overlay) + integration test + CLAUDE.md

**Files:**
- Create: `internal/adapter/yaml/load.go`, `internal/adapter/yaml/CLAUDE.md`
- Test: `internal/adapter/yaml/load_test.go`

**Interfaces:**
- Consumes: `ymlConfig.toDomain`, `checkPrefixes` (Task 4); `dirName`, `configName`, `localConfigName` (Task 5); `Init`, `renderTemplate` (Task 6).
- Produces: `func Load(root string) (mtt.Config, map[string]string, error)`.

- [ ] **Step 1: Write the failing test**

`internal/adapter/yaml/load_test.go`:
```go
package yaml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitLoadValidate(t *testing.T) {
	for _, name := range []string{"default", "coding"} {
		root := t.TempDir()
		if err := Init(root, name, "demo", false); err != nil {
			t.Fatalf("init %s: %v", name, err)
		}
		cfg, prefixes, err := Load(root)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("%s: domain invalid: %v", name, err)
		}
		if len(prefixes) != len(cfg.Types) {
			t.Fatalf("%s: %d prefixes for %d types", name, len(prefixes), len(cfg.Types))
		}
		if got, ok := cfg.DefaultType(); !ok || got.Name == "" {
			t.Fatalf("%s: no default type", name)
		}
	}
}

func TestLoadOverlayOverridesName(t *testing.T) {
	root := t.TempDir()
	if err := Init(root, "default", "demo", false); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(root, dirName, localConfigName)
	if err := os.WriteFile(overlay, []byte("project:\n  name: overridden\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Name != "overridden" {
		t.Fatalf("name = %q, want overridden", cfg.Project.Name)
	}
}

func TestLoadMissing(t *testing.T) {
	if _, _, err := Load(t.TempDir()); err == nil {
		t.Fatal("want error loading a dir with no config")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/yaml/ -run TestInitLoadValidate`
Expected: FAIL — `Load` undefined.

- [ ] **Step 3: Write minimal implementation**

`internal/adapter/yaml/load.go`:
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

// Load reads .mtt/config.yaml under root, merges the optional gitignored
// .mtt/config.local.yaml overlay (later layer wins, per field/section), maps to
// the domain Config, and runs the YAML provider's checks (exactly one default;
// prefixes present+unique). Domain invariants (Config.Validate) are the caller's.
func Load(root string) (mtt.Config, map[string]string, error) {
	var yc ymlConfig
	if err := decodeInto(filepath.Join(root, dirName, configName), &yc, true); err != nil {
		return mtt.Config{}, nil, err
	}
	if err := decodeInto(filepath.Join(root, dirName, localConfigName), &yc, false); err != nil {
		return mtt.Config{}, nil, err
	}
	cfg, prefixes := yc.toDomain()
	if err := checkPrefixes(cfg, prefixes); err != nil {
		return mtt.Config{}, nil, err
	}
	return cfg, prefixes, nil
}

// decodeInto decodes a YAML file onto dst, overlaying whatever dst already holds.
// A missing file is not an error when required is false.
func decodeInto(path string, dst *ymlConfig, required bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if !required && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := goyaml.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
```

Create `internal/adapter/yaml/CLAUDE.md`:
```markdown
# internal/adapter/yaml

Default driven adapter — the **config layer** this session (tasks come later). Stores config as YAML under `.mtt/`, owns ID-encoding (`prefix`), maps DTOs↔`pkg/mtt` domain. **No business rules** beyond provider-specific checks.

## Responsibilities

- `FindRoot` — locate `.mtt/` walking up (like git).
- `Init` — render an embedded template (`default`/`coding`, `text/template` `{{.Name}}`), **atomic** write (temp+rename), refuse overwrite without force.
- `Load` — read config + optional gitignored `config.local.yaml` overlay (later wins, per field), map DTO→domain, run provider checks (exactly one `default`; prefix present+unique). Domain `Config.Validate()` is the caller's call.

## Boundaries

- The domain never sees YAML: DTOs carry the yaml tags + `prefix`; `toDomain` maps to pure types.
- No flow/ready/traversal logic here (that is `core`, later). Templates are the **only** home of default type/status names.
- `.mtt/config.yaml` is edited only through this adapter (determinism + validation).
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adapter/yaml/`
Expected: PASS (all adapter tests).

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/yaml/load.go internal/adapter/yaml/load_test.go internal/adapter/yaml/CLAUDE.md
git commit -m "feat(yaml): Load with overlay merge + CLAUDE.md" \
  -m "Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: `internal/cli` — `init` command

**Files:**
- Create: `internal/cli/init.go`
- Modify: `internal/cli/root.go` (register)
- Test: `internal/cli/init_test.go`

**Interfaces:**
- Consumes: `yaml.Init` (Task 6).
- Produces: `func newInitCmd() *cobra.Command`.

- [ ] **Step 1: Write the failing test**

`internal/cli/init_test.go`:
```go
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func runRoot(t *testing.T, args ...string) error {
	t.Helper()
	root := NewRootCmd()
	root.SetOut(new(strings.Builder))
	root.SetErr(new(strings.Builder))
	root.SetArgs(args)
	return root.Execute()
}

func TestInitCommand(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".mtt", "config.yaml")); err != nil {
		t.Fatalf("config not created: %v", err)
	}
	if err := runRoot(t, "init"); err == nil {
		t.Fatal("re-init without --force should fail")
	}
	if err := runRoot(t, "init", "--force", "--template", "coding"); err != nil {
		t.Fatalf("force init: %v", err)
	}
}

// chdir switches to dir for the duration of the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}
```
(Add `"strings"` to the import block.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestInitCommand`
Expected: FAIL — unknown command "init".

- [ ] **Step 3: Write minimal implementation**

`internal/cli/init.go`:
```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
)

// newInitCmd builds `mtt init`: write the starter .mtt/config.yaml.
func newInitCmd() *cobra.Command {
	var (
		tmpl  string
		force bool
		name  string
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a project (.mtt/config.yaml)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			projectName := name
			if projectName == "" {
				projectName = filepath.Base(cwd)
			}
			if err := yaml.Init(cwd, tmpl, projectName, force); err != nil {
				return err
			}
			cmd.Printf("initialized .mtt/config.yaml (template %q)\n", tmpl)
			return nil
		},
	}
	cmd.Flags().StringVar(&tmpl, "template", "default", "starter template: default|coding")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config")
	cmd.Flags().StringVar(&name, "name", "", "project name (default: current directory name)")
	return cmd
}
```

Modify `internal/cli/root.go` — replace the single AddCommand line:
```go
	root.AddCommand(newVersionCmd())
```
with:
```go
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd())
```
(`newTypesCmd` lands in Task 9; if executing strictly in order, add only `newInitCmd()` here and add `newTypesCmd()` in Task 9. To keep the package compiling now, add both only after Task 9 — for this task add just `newInitCmd()`.)

So for Task 8, use:
```go
	root.AddCommand(newVersionCmd(), newInitCmd())
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestInitCommand`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/init.go internal/cli/root.go internal/cli/init_test.go
git commit -m "feat(cli): mtt init command" \
  -m "Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: `internal/cli` — `types` command + formatter + CLAUDE.md

**Files:**
- Create: `internal/cli/types.go`
- Modify: `internal/cli/root.go` (register `newTypesCmd`), `internal/cli/CLAUDE.md`
- Test: `internal/cli/types_test.go`

**Interfaces:**
- Consumes: `yaml.FindRoot`, `yaml.Load` (Tasks 5,7); `mtt.Config` (Task 2).
- Produces: `func newTypesCmd() *cobra.Command`; `func formatTypes(cfg mtt.Config, prefixes map[string]string, filter string) (string, error)`.

- [ ] **Step 1: Write the failing test**

`internal/cli/types_test.go`:
```go
package cli

import (
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestFormatTypes(t *testing.T) {
	cfg := mtt.Config{Types: []mtt.Type{{
		Name: "task", Description: "A unit of work.", Default: true,
		Flow: mtt.Flow{
			Statuses: []mtt.Status{
				{Name: "tbd", Kind: mtt.KindInitial},
				{Name: "doing", Kind: mtt.KindActive},
				{Name: "done", Kind: mtt.KindTerminal},
			},
			Transitions: []mtt.Transition{
				{From: "tbd", To: "doing"},
				{From: "doing", To: "done", Description: "gate", Commands: []string{"make test"}},
			},
		},
	}}}
	out, err := formatTypes(cfg, map[string]string{"task": "t"}, "")
	if err != nil {
		t.Fatal(err)
	}
	want := "task  (prefix t · root · default)\n" +
		"  A unit of work.\n" +
		"  statuses: tbd[initial] doing[active] done[terminal]\n" +
		"  transitions:\n" +
		"    tbd -> doing\n" +
		"    doing -> done  # gate\n" +
		"        $ make test\n" +
		"\n"
	if out != want {
		t.Fatalf("formatTypes mismatch:\n got: %q\nwant: %q", out, want)
	}
}

func TestFormatTypesFilter(t *testing.T) {
	cfg := mtt.Config{Types: []mtt.Type{
		{Name: "epic", Parents: nil, Flow: mtt.Flow{Statuses: []mtt.Status{{Name: "a", Kind: mtt.KindInitial}}}},
		{Name: "task", Parents: []string{"epic"}},
	}}
	out, err := formatTypes(cfg, map[string]string{"epic": "e", "task": "t"}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if want := "task  (prefix t · parents: epic)\n  transitions:\n\n"; out != want {
		t.Fatalf("filtered output = %q, want %q", out, want)
	}
	if _, err := formatTypes(cfg, nil, "ghost"); err == nil {
		t.Fatal("want error for unknown type filter")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestFormatTypes`
Expected: FAIL — `formatTypes` undefined.

- [ ] **Step 3: Write minimal implementation**

`internal/cli/types.go`:
```go
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	"github.com/pashukhin/mtt/pkg/mtt"
)

// newTypesCmd builds `mtt types [type]`: show the configured types and their flow.
func newTypesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "types [type]",
		Short: "Show configured task types and their flow",
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
			cfg, prefixes, err := yaml.Load(root)
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}
			filter := ""
			if len(args) == 1 {
				filter = args[0]
			}
			out, err := formatTypes(cfg, prefixes, filter)
			if err != nil {
				return err
			}
			cmd.Print(out)
			return nil
		},
	}
}

// formatTypes renders the configured types as human-readable blocks. When filter
// is non-empty, only that type is shown (error if unknown).
func formatTypes(cfg mtt.Config, prefixes map[string]string, filter string) (string, error) {
	var b strings.Builder
	shown := 0
	for _, t := range cfg.Types {
		if filter != "" && t.Name != filter {
			continue
		}
		shown++
		writeTypeBlock(&b, t, prefixes[t.Name])
	}
	if filter != "" && shown == 0 {
		return "", fmt.Errorf("unknown type %q", filter)
	}
	return b.String(), nil
}

// writeTypeBlock appends one type's block to b.
func writeTypeBlock(b *strings.Builder, t mtt.Type, prefix string) {
	rel := "root"
	if len(t.Parents) > 0 {
		rel = "parents: " + strings.Join(t.Parents, ", ")
	}
	fmt.Fprintf(b, "%s  (prefix %s · %s", t.Name, prefix, rel)
	if t.Default {
		b.WriteString(" · default")
	}
	b.WriteString(")\n")
	if t.Description != "" {
		fmt.Fprintf(b, "  %s\n", t.Description)
	}
	if len(t.Statuses) > 0 {
		b.WriteString("  statuses:")
		for _, s := range t.Statuses {
			fmt.Fprintf(b, " %s[%s]", s.Name, s.Kind)
		}
		b.WriteString("\n")
	}
	b.WriteString("  transitions:\n")
	for _, tr := range t.Transitions {
		fmt.Fprintf(b, "    %s -> %s", tr.From, tr.To)
		if tr.Description != "" {
			fmt.Fprintf(b, "  # %s", tr.Description)
		}
		b.WriteString("\n")
		for _, c := range tr.Commands {
			fmt.Fprintf(b, "        $ %s\n", c)
		}
	}
	b.WriteString("\n")
}
```

Modify `internal/cli/root.go` to register both new commands:
```go
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd())
```

Update `internal/cli/CLAUDE.md` "Current state" section:
```markdown
## Current state

`root` + `version` + `init` + `types`. `init`/`types` bootstrap config directly via the YAML adapter
(the composition root wiring adapters from config); `types` calls `Config.Validate()` and formats output.
Next (phase 1): `add / show / list / edit / close`.
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestFormatTypes`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/types.go internal/cli/root.go internal/cli/CLAUDE.md internal/cli/types_test.go
git commit -m "feat(cli): mtt types command + block formatter" \
  -m "Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: e2e `testscript` acceptance + final gate

**Files:**
- Create: `internal/cli/script_test.go`, `internal/cli/testdata/scripts/init.txt`
- Modify: `go.mod`, `go.sum`, `sessions/001_init_and_types.md` (Done section)

**Interfaces:**
- Consumes: `cli.Execute` (existing), `mtt init`/`mtt types` (Tasks 8,9).

- [ ] **Step 1: Add the testscript dependency**

Run:
```bash
go get github.com/rogpeppe/go-internal/testscript@latest
go mod tidy
```

- [ ] **Step 2: Write the failing e2e (script + harness)**

`internal/cli/script_test.go`:
```go
package cli

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"mtt": func() int {
			if err := Execute(); err != nil {
				return 1
			}
			return 0
		},
	}))
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{Dir: "testdata/scripts"})
}
```

`internal/cli/testdata/scripts/init.txt`:
```
# init creates config; types lists the configured types
exec mtt init
stdout 'initialized'
exists .mtt/config.yaml
exec mtt types
stdout 'epic'
stdout 'task'
stdout 'subtask'
stdout 'active'
stdout '-> done'
stdout 'default'

# a single type can be filtered
exec mtt types epic
stdout 'prefix e'
! stdout 'subtask'

# re-init without --force fails
! exec mtt init
stderr 'already initialized'

# --force overwrites; coding template shows gated per-type DoD
exec mtt init --force --template coding
exec mtt types
stdout 'feature'
stdout 'bugfix'
stdout 'refactor'
stdout 'make test'

# types outside a project errors
cd $WORK/empty
! exec mtt types
stderr 'not initialized'

-- empty/.keep --
```

- [ ] **Step 3: Run test to verify it fails (before harness) / passes (after)**

Run: `go test ./internal/cli/ -run TestScripts`
Expected: PASS once `script_test.go` + `init.txt` exist and Tasks 8–9 are in. If it fails on a specific `stdout` match, reconcile the assertion with the actual formatter output (`go test -run TestScripts -v` prints the script log).

- [ ] **Step 4: Run the full gate**

Run: `make check`
Expected: `OK: make check passed` (fmt-check + vet + lint + `go test -race -cover ./...` + build all green).
If lint flags anything, fix it (doc comments on exported identifiers, wrapped errors) and re-run.

- [ ] **Step 5: Fill the session Done section + commit**

Edit `sessions/001_init_and_types.md` — replace the trailing `—` under `## Done` with a short summary: commands shipped (`init`, `types`), packages created (`pkg/mtt`, `internal/adapter/yaml`), the config-as-data + DTO-mapping decisions, and any deviations (no `internal/core` yet). Then:
```bash
git add internal/cli/script_test.go internal/cli/testdata go.mod go.sum sessions/001_init_and_types.md
git commit -m "test(cli): e2e init/types acceptance + session 001 done" \
  -m "Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage** (spec §§ mapped to tasks):
- §5 pure contract (StatusKind, structs, Validate, DefaultType/ChildrenIn) → Tasks 1–3.
- §4 invariants: domain (Task 2), adapter prefix/one-default (Task 4).
- §6 adapter (DTOs+mapping, FindRoot, Init+templates, Load+overlay) → Tasks 4–7.
- §7 CLI init/types + block formatter → Tasks 8–9.
- §8 templates default/coding → Task 6.
- §11 tests: unit (1–9), golden (6), testscript e2e (10).
- §12 acceptance (init creates config, types prints types+kinds+transitions, coding shows gated DoD, testscript, golden, `make check`) → Task 10.
- §3.11 provider-agnostic / §9 layering / core-deferred → encoded in file structure + CLAUDE.md (Tasks 3,7,9). Doc reconciliation (spec §10) already committed (`20b8085`).

**Placeholder scan:** none — every step carries full code/commands.

**Type consistency:** `Init(root, tmplName, projectName string, force bool)`, `Load(root) (mtt.Config, map[string]string, error)`, `FindRoot(start) (string, error)`, `formatTypes(mtt.Config, map[string]string, string) (string, error)`, `renderTemplate(name, projectName) ([]byte, error)` are used identically across producer and consumer tasks. `StatusKind` constants (`KindInitial/Active/Terminal`) consistent. `ymlConfig.toDomain` returns `(mtt.Config, map[string]string)` matching `Load`/`checkPrefixes` usage.

**Note for the executor:** if any `testscript` `stdout` assertion in Task 10 disagrees with the exact formatter text, trust the formatter unit test (Task 9) as the source of truth and adjust the script's regex — the acceptance is that init/types run and the coding gates are visible, not a byte-exact CLI layout.
```
