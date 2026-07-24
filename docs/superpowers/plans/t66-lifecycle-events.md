# t66 — Lifecycle events + post_defaults Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Config-authored post-only command pipelines on create/update/delete of tasks and notes (events), plus per-type `post_defaults` prepended to every flow edge's `post:` with explicit per-edge opt-out — then rewrite the dogfood config on top of both.

**Architecture:** Two symmetric layers per the approved spec ([t66-lifecycle-events.md](../specs/t66-lifecycle-events.md)). Events live in `pkg/mtt.Config.Events` (domain), are fired per-entity by the mutating **usecases** (never the store, never `Transitioner`) through a new `core.EventEmitter` that reuses the t21/t28 pipeline machinery (`Command` VO → `expandCommands*` → `Runner` → `PostActionError`/exit 5). `post_defaults` is a pure domain computation (`Type.EffectivePost`) consumed by `core.Transitioner`.

**Tech Stack:** Go 1.23, gopkg.in/yaml.v3, cobra, testscript (txtar e2e).

## Global Constraints

- **TDD**: every task is red → green → refactor; `make check` green before every commit.
- The spec is the decision record — on any conflict between this plan and the spec, the spec wins; do not relitigate user-locked decisions (per-entity firing, post-only, two layers).
- Placeholder whitelists are **shape-safe structs**: task events expose exactly `{ID, Type, Event}`, note events `{Slug, Event}`; `{{.From}}`/`{{.To}}` in an event template must be a template error.
- `{{.Type}}` guard (spec §4): the emitter resolves the type via `cfg.TypeByName` **before** expansion and uses the config's name; a miss = finalization failure, mutation kept, exit 5.
- Exit taxonomy is unchanged: 2 attribution, 3 blocked, 4 not-found, 5 post/finalization (single-entity only), 6 invalid transition, 7 dangling refs; **bulk aggregate stays generic exit 1** (s008.9).
- Events fire **only on a real persist** (idempotent no-ops fire nothing).
- English docs are the source of truth; every DESIGN/CLI_REFERENCE/FLOW_GUIDE change lands in the RU mirror in the same task; grep for ALL parallel occurrences (the design-docs-parallel-occurrences lesson).
- Commit messages: imperative, `t66:` prefix, trailer `Co-Authored-By: <acting model> <noreply@anthropic.com>`.
- **Ordering warning (binary-vs-config skew):** Task 12 (the dogfood config rewrite) MUST be the last code-adjacent commit. From that commit on, the installed `mtt` binary (pre-t66) silently ignores `events:`/`post_defaults:`/`inherit_post:` (non-strict decode) — moves stop auto-committing `.mtt`. Run all subsequent flow moves with the branch-built binary: `make build && ./bin/mtt <move> …`; if a move was made with the old binary, commit `.mtt` by hand.

---

## Stage A — `post_defaults` (the t24 half; small, self-contained)

### Task 1: Domain — `Type.PostDefaults`, `Transition.SkipPostDefaults`, `Type.EffectivePost`, uniform post validation

**Files:**
- Modify: `pkg/mtt/config.go` (Type, Transition, new method)
- Modify: `pkg/mtt/validate.go` (post surfaces: valid + rollback-rejection)
- Test: `pkg/mtt/config_test.go`, `pkg/mtt/validate_test.go`

**Interfaces:**
- Produces: `Type.PostDefaults []Command`; `Transition.SkipPostDefaults bool`; `func (t Type) EffectivePost(tr Transition) []Command` (defaults first, edge appended; `SkipPostDefaults` → edge-only). Consumed by Task 3 (Transitioner) and Task 10 (adapter mapping — Task 2).

- [ ] **Step 1: Write the failing tests**

In `pkg/mtt/config_test.go`:

```go
func TestEffectivePost(t *testing.T) {
	d1 := Command{Run: "default-1"}
	d2 := Command{Run: "default-2"}
	own := Command{Run: "own"}
	typ := Type{Name: "task", PostDefaults: []Command{d1, d2}}
	cases := []struct {
		name string
		tr   Transition
		want []string
	}{
		{"defaults prepend to edge post", Transition{Post: []Command{own}}, []string{"default-1", "default-2", "own"}},
		{"defaults alone when edge has no post", Transition{}, []string{"default-1", "default-2"}},
		{"opt-out yields edge post only", Transition{Post: []Command{own}, SkipPostDefaults: true}, []string{"own"}},
		{"opt-out with empty post yields none", Transition{SkipPostDefaults: true}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := typ.EffectivePost(tc.tr)
			var runs []string
			for _, c := range got {
				runs = append(runs, c.Run)
			}
			if !reflect.DeepEqual(runs, tc.want) {
				t.Fatalf("EffectivePost runs = %v, want %v", runs, tc.want)
			}
		})
	}
	// no defaults: the edge's own slice must come back unchanged (also for a type
	// with nil PostDefaults — the common case must not allocate a copy).
	bare := Type{Name: "bare"}
	if got := bare.EffectivePost(Transition{Post: []Command{own}}); len(got) != 1 || got[0].Run != "own" {
		t.Fatalf("bare EffectivePost = %v", got)
	}
}
```

In `pkg/mtt/validate_test.go` (extend the existing valid-config fixture pattern used by the current tests — copy a minimal valid 3-status flow and mutate it):

```go
func TestValidatePostSurfacesRejectRollback(t *testing.T) {
	rb := &Command{Run: "undo"}
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string // substring of the joined error
	}{
		{"edge post rollback rejected", func(c *Config) {
			c.Types[0].Transitions[0].Post = []Command{{Run: "x", Rollback: rb}}
		}, "post command must not carry a rollback"},
		{"post_defaults rollback rejected", func(c *Config) {
			c.Types[0].PostDefaults = []Command{{Run: "x", Rollback: rb}}
		}, "post command must not carry a rollback"},
		{"post_defaults invalid command rejected", func(c *Config) {
			c.Types[0].PostDefaults = []Command{{Run: ""}}
		}, "invalid post command"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validTestConfig() // reuse/extract the existing valid fixture helper
			tc.mutate(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v, want substring %q", err, tc.want)
			}
		})
	}
}
```

(If `validate_test.go` has no reusable valid-fixture helper, extract one — `validTestConfig()` — from its existing table setup as part of this step.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/mtt/ -run 'TestEffectivePost|TestValidatePostSurfaces' -v`
Expected: FAIL — `SkipPostDefaults`/`PostDefaults`/`EffectivePost` undefined; rollback-in-post currently accepted.

- [ ] **Step 3: Implement**

`pkg/mtt/config.go` — add fields + method:

```go
type Type struct {
	Name        TypeName
	Description string
	Parents     []TypeName
	Default     bool
	PostDefaults []Command // prepended to every edge's Post unless the edge opts out (t66/t24)
	Flow
}
```

```go
type Transition struct {
	// ... existing fields ...
	Post             []Command // commands run AFTER persist (finalization, e.g. git commit); non-transactional (t21)
	SkipPostDefaults bool      // opt out of the type's PostDefaults (YAML: inherit_post: false); zero value = inherit
}
```

```go
// EffectivePost returns the post pipeline that actually runs for edge tr: the
// type's PostDefaults followed by the edge's own Post — unless the edge opts
// out (SkipPostDefaults), then only its own Post. The t24 precedence rule:
// defaults first, specifics appended, opt-out only explicit.
func (t Type) EffectivePost(tr Transition) []Command {
	if tr.SkipPostDefaults || len(t.PostDefaults) == 0 {
		return tr.Post
	}
	out := make([]Command, 0, len(t.PostDefaults)+len(tr.Post))
	out = append(out, t.PostDefaults...)
	return append(out, tr.Post...)
}
```

`pkg/mtt/validate.go` — replace the existing `for _, cmd := range tr.Post` loop with a shared helper and apply it to both post surfaces (events join in Task 4):

```go
// validatePostCommands checks a post-phase pipeline: each command well-formed
// AND rollback-free — post pipelines have no compensation phase (uniform rule
// across edge post, post_defaults, and event post; t66).
func validatePostCommands(cmds []Command, where string) []error {
	var errs []error
	for _, cmd := range cmds {
		if !cmd.Valid() {
			errs = append(errs, fmt.Errorf("%s: invalid post command (empty run or negative timeout)", where))
		}
		if cmd.Rollback != nil {
			errs = append(errs, fmt.Errorf("%s: post command must not carry a rollback (post has no compensation phase)", where))
		}
	}
	return errs
}
```

In `validateFlow`, replace the `tr.Post` loop body with:

```go
		errs = append(errs, validatePostCommands(tr.Post, fmt.Sprintf("type %q transition %q->%q", t.Name, tr.From, tr.To))...)
```

and after the transitions loop add:

```go
	errs = append(errs, validatePostCommands(t.PostDefaults, fmt.Sprintf("type %q post_defaults", t.Name))...)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/mtt/ -v`
Expected: PASS (including all pre-existing tests — the rollback-rejection is a deliberate behavior change; if an existing validate test asserts a rollback-carrying post is accepted, update that test and note it in the commit message).

- [ ] **Step 5: Commit**

```bash
git add pkg/mtt/ && git commit -m "t66: domain — PostDefaults + EffectivePost + uniform post rollback-rejection"
```

### Task 2: Adapter — `post_defaults:` / `inherit_post:` decode

**Files:**
- Modify: `internal/adapter/yaml/dto.go` (`ymlType`, `ymlTransition`, `toDomain`)
- Test: `internal/adapter/yaml/dto_post_test.go` (extend — it already covers `post:` decode)

**Interfaces:**
- Consumes: Task 1's domain fields.
- Produces: `Load` returns configs with `PostDefaults`/`SkipPostDefaults` populated; `inherit_post:` omitted or `true` → `SkipPostDefaults=false`, `inherit_post: false` → `true`.

- [ ] **Step 1: Write the failing test** (extend `dto_post_test.go`, mirroring its existing decode-assert style):

```go
func TestConfigDecodePostDefaultsAndInheritPost(t *testing.T) {
	src := `
version: 1
project: {name: p}
types:
  - name: task
    prefix: t
    post_defaults:
      - 'echo default'
      - {run: 'echo timed', timeout: 30s}
    statuses:
      - {name: a, kind: initial}
      - {name: b, kind: active}
      - {name: c, kind: terminal}
    transitions:
      - {from: a, to: b}
      - {from: b, to: c, inherit_post: false, post: ['echo own']}
`
	var yc ymlConfig
	if err := goyaml.Unmarshal([]byte(src), &yc); err != nil {
		t.Fatal(err)
	}
	cfg, _ := yc.toDomain()
	typ := cfg.Types[0]
	if len(typ.PostDefaults) != 2 || typ.PostDefaults[0].Run != "echo default" || typ.PostDefaults[1].Timeout != 30*time.Second {
		t.Fatalf("PostDefaults = %+v", typ.PostDefaults)
	}
	if typ.Transitions[0].SkipPostDefaults {
		t.Fatal("edge without inherit_post must inherit (SkipPostDefaults=false)")
	}
	e := typ.Transitions[1]
	if !e.SkipPostDefaults || len(e.Post) != 1 || e.Post[0].Run != "echo own" {
		t.Fatalf("inherit_post:false edge = %+v", e)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/yaml/ -run TestConfigDecodePostDefaultsAndInheritPost -v`
Expected: FAIL (unknown fields are silently ignored → zero values → assertions fail).

- [ ] **Step 3: Implement** — in `dto.go`:

```go
type ymlType struct {
	Name         string          `yaml:"name"`
	Description  string          `yaml:"description"`
	Prefix       string          `yaml:"prefix"`
	Parents      []string        `yaml:"parents"`
	Default      bool            `yaml:"default"`
	PostDefaults []ymlCommand    `yaml:"post_defaults,omitempty"`
	Statuses     []ymlStatus     `yaml:"statuses"`
	Transitions  []ymlTransition `yaml:"transitions"`
}
```

`ymlTransition` gains `InheritPost *bool \`yaml:"inherit_post,omitempty"\`` (pointer: absent ≠ false). In `toDomain`, inside the type loop before statuses:

```go
		for _, c := range yt.PostDefaults {
			t.PostDefaults = append(t.PostDefaults, c.toDomain())
		}
```

and in the transition mapping add:

```go
Skip := yr.InheritPost != nil && !*yr.InheritPost
```

wired as `SkipPostDefaults: Skip` in the `mtt.Transition{...}` literal.

- [ ] **Step 4: Run tests** — `go test ./internal/adapter/yaml/ -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/yaml/ && git commit -m "t66: adapter — decode post_defaults + inherit_post"
```

### Task 3: Core — `Transitioner` runs `EffectivePost`

**Files:**
- Modify: `internal/core/transition.go` (the three `edge.Post` sites, lines ~117–127)
- Test: `internal/core/transition_test.go`

**Interfaces:**
- Consumes: `Type.EffectivePost` (Task 1).
- Produces: transitions run defaults-then-edge post; `SkipPostDefaults` edges run edge-only. No signature change.

- [ ] **Step 1: Write the failing test** (reuse the existing `fakeRunner` + store fake from `transition_test.go`; follow its existing post-phase test as the template):

```go
func TestTransitionRunsEffectivePost(t *testing.T) {
	// type with post_defaults [D]; edge a->b with post [E]; edge b->c opted out with post [F]
	// assert: a->b runs D then E (runner records order); b->c runs only F.
}
```

Write it concretely against the file's existing fixture helpers (the type-construction helper used by the current post tests), asserting `fakeRunner`'s recorded `Run` sequence — post commands arrive in one `Run` call in order `["D","E"]` for the inheriting edge and `["F"]` for the opted-out edge.

- [ ] **Step 2: Run** `go test ./internal/core/ -run TestTransitionRunsEffectivePost -v` → FAIL (only `E` runs).

- [ ] **Step 3: Implement** — in `Transition()`, replace the post block's uses of `edge.Post` with one local:

```go
	post := typ.EffectivePost(edge)
	if opts.NoRun || len(post) == 0 {
		return updated, nil
	}
	expanded, eerr := expandCommands(post, cmdContext{
		ID: string(t.ID), Type: string(t.Type), From: string(from), To: string(to),
	})
	if eerr != nil {
		return updated, &PostActionError{
			Remaining: runsOf(post), // raw templates — expansion is what failed
			Cause:     fmt.Sprintf("expand post for %s (%s->%s): %v", id, from, to, eerr),
		}
	}
```

(the rest of the block is unchanged — it already operates on `expanded`).

- [ ] **Step 4: Run** `go test ./internal/core/ -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/ && git commit -m "t66: core — Transitioner runs Type.EffectivePost (t24 precedence)"
```

---

## Stage B — Lifecycle events

### Task 4: Domain — `EventKind` VO, `EventHook(s)`, `Config.Events`, validation

**Files:**
- Create: `pkg/mtt/event.go`
- Modify: `pkg/mtt/config.go` (Config.Events field), `pkg/mtt/validate.go`
- Test: Create `pkg/mtt/event_test.go`; extend `pkg/mtt/validate_test.go`

**Interfaces:**
- Produces: `EventKind` (`EventCreate`/`EventUpdate`/`EventDelete`, `Valid()`), `EventHook{Post []Command}`, `EventHooks{Create,Update,Delete}` with `Hook(EventKind) EventHook`, `Events{Task, Note EventHooks}`, `Config.Events Events`. Consumed by Tasks 5–11.

- [ ] **Step 1: Write the failing tests**

`pkg/mtt/event_test.go`:

```go
package mtt

import "testing"

func TestEventKindValid(t *testing.T) {
	for _, k := range []EventKind{EventCreate, EventUpdate, EventDelete} {
		if !k.Valid() {
			t.Fatalf("%q must be valid", k)
		}
	}
	for _, k := range []EventKind{"", "created", "CREATE"} {
		if k.Valid() {
			t.Fatalf("%q must be invalid", k)
		}
	}
}

func TestEventHooksHook(t *testing.T) {
	h := EventHooks{
		Create: EventHook{Post: []Command{{Run: "c"}}},
		Update: EventHook{Post: []Command{{Run: "u"}}},
		Delete: EventHook{Post: []Command{{Run: "d"}}},
	}
	for kind, want := range map[EventKind]string{EventCreate: "c", EventUpdate: "u", EventDelete: "d"} {
		if got := h.Hook(kind); len(got.Post) != 1 || got.Post[0].Run != want {
			t.Fatalf("Hook(%q) = %+v, want run %q", kind, got, want)
		}
	}
	if got := h.Hook("bogus"); len(got.Post) != 0 {
		t.Fatalf("Hook(bogus) = %+v, want zero", got)
	}
}
```

`validate_test.go` — extend `TestValidatePostSurfacesRejectRollback`'s table with:

```go
		{"event post rollback rejected", func(c *Config) {
			c.Events.Task.Create.Post = []Command{{Run: "x", Rollback: rb}}
		}, "post command must not carry a rollback"},
		{"event post invalid command rejected", func(c *Config) {
			c.Events.Note.Delete.Post = []Command{{Run: ""}}
		}, "invalid post command"},
```

- [ ] **Step 2: Run** `go test ./pkg/mtt/ -run 'TestEvent|TestValidatePostSurfaces' -v` → FAIL (types undefined).

- [ ] **Step 3: Implement** — `pkg/mtt/event.go`:

```go
package mtt

// EventKind is the closed vocabulary of lifecycle events a store mutation
// fires: an entity was created, updated, or deleted. A value object like
// StatusKind — code reasons about the kind, never a bare string. It is also
// the {{.Event}} placeholder value, so it stays shape-safe by construction.
type EventKind string

// The three lifecycle events.
const (
	EventCreate EventKind = "create"
	EventUpdate EventKind = "update"
	EventDelete EventKind = "delete"
)

// Valid reports whether k is one of the three lifecycle events.
func (k EventKind) Valid() bool {
	return k == EventCreate || k == EventUpdate || k == EventDelete
}

// EventHook is the pipeline configured for one lifecycle event of one store.
// Post-only by design (t66): an event finalizes a persisted mutation, it can
// never block one; a blocking commands: phase would be an additive extension.
type EventHook struct {
	Post []Command
}

// EventHooks groups the three per-event hooks of one store.
type EventHooks struct {
	Create EventHook
	Update EventHook
	Delete EventHook
}

// Hook returns the hook for kind (the zero EventHook for an unknown kind).
func (h EventHooks) Hook(kind EventKind) EventHook {
	switch kind {
	case EventCreate:
		return h.Create
	case EventUpdate:
		return h.Update
	case EventDelete:
		return h.Delete
	}
	return EventHook{}
}

// Events is the config's lifecycle-event section: per-store hooks for tasks
// and notes. Optional — the zero value configures no events.
type Events struct {
	Task EventHooks
	Note EventHooks
}
```

`config.go`: `Config` gains `Events Events` after `Types`.

`validate.go` — in `Config.Validate`, after the types loop:

```go
	for _, ev := range []struct {
		where string
		hooks EventHooks
	}{{"events.task", c.Events.Task}, {"events.note", c.Events.Note}} {
		for _, h := range []struct {
			kind string
			hook EventHook
		}{{"create", ev.hooks.Create}, {"update", ev.hooks.Update}, {"delete", ev.hooks.Delete}} {
			errs = append(errs, validatePostCommands(h.hook.Post, fmt.Sprintf("%s.%s", ev.where, h.kind))...)
		}
	}
```

- [ ] **Step 4: Run** `go test ./pkg/mtt/ -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/mtt/ && git commit -m "t66: domain — EventKind VO + Events config section + event post validation"
```

### Task 5: Adapter — `events:` decode

**Files:**
- Modify: `internal/adapter/yaml/dto.go`
- Test: Create `internal/adapter/yaml/dto_events_test.go`

**Interfaces:**
- Consumes: Task 4's domain types.
- Produces: `Load` populates `Config.Events`; scalar and `{run, timeout}` command forms both work (reuse `ymlCommand`).

- [ ] **Step 1: Write the failing test** (`dto_events_test.go`, same style as Task 2's):

```go
func TestConfigDecodeEvents(t *testing.T) {
	src := `
version: 1
project: {name: p}
events:
  task:
    create:
      post: ['echo task-created {{.ID}} {{.Event}}']
    delete:
      post: [{run: 'echo bye', timeout: 10s}]
  note:
    update:
      post: ['echo note {{.Slug}}']
types:
  - name: task
    prefix: t
    statuses:
      - {name: a, kind: initial}
      - {name: b, kind: active}
      - {name: c, kind: terminal}
    transitions:
      - {from: a, to: b}
      - {from: b, to: c}
`
	var yc ymlConfig
	if err := goyaml.Unmarshal([]byte(src), &yc); err != nil {
		t.Fatal(err)
	}
	cfg, _ := yc.toDomain()
	if got := cfg.Events.Task.Create.Post; len(got) != 1 || got[0].Run != "echo task-created {{.ID}} {{.Event}}" {
		t.Fatalf("task.create = %+v", got)
	}
	if got := cfg.Events.Task.Delete.Post; len(got) != 1 || got[0].Timeout != 10*time.Second {
		t.Fatalf("task.delete = %+v", got)
	}
	if got := cfg.Events.Task.Update.Post; len(got) != 0 {
		t.Fatalf("unconfigured task.update = %+v, want empty", got)
	}
	if got := cfg.Events.Note.Update.Post; len(got) != 1 || got[0].Run != "echo note {{.Slug}}" {
		t.Fatalf("note.update = %+v", got)
	}
}
```

- [ ] **Step 2: Run** `go test ./internal/adapter/yaml/ -run TestConfigDecodeEvents -v` → FAIL.

- [ ] **Step 3: Implement** — `dto.go`:

```go
// ymlEvents / ymlEventHooks / ymlEventHook mirror the domain Events section on
// disk (decode-only — config is never marshaled). Command lists reuse
// ymlCommand (scalar or {run, timeout} map).
type ymlEvents struct {
	Task ymlEventHooks `yaml:"task,omitempty"`
	Note ymlEventHooks `yaml:"note,omitempty"`
}

type ymlEventHooks struct {
	Create ymlEventHook `yaml:"create,omitempty"`
	Update ymlEventHook `yaml:"update,omitempty"`
	Delete ymlEventHook `yaml:"delete,omitempty"`
}

type ymlEventHook struct {
	Post []ymlCommand `yaml:"post,omitempty"`
}

func (h ymlEventHook) toDomain() mtt.EventHook {
	out := mtt.EventHook{}
	for _, c := range h.Post {
		out.Post = append(out.Post, c.toDomain())
	}
	return out
}

func (e ymlEvents) toDomain() mtt.Events {
	return mtt.Events{
		Task: mtt.EventHooks{Create: e.Task.Create.toDomain(), Update: e.Task.Update.toDomain(), Delete: e.Task.Delete.toDomain()},
		Note: mtt.EventHooks{Create: e.Note.Create.toDomain(), Update: e.Note.Update.toDomain(), Delete: e.Note.Delete.toDomain()},
	}
}
```

`ymlConfig` gains `Events ymlEvents \`yaml:"events,omitempty"\`` (after `Require`); `ymlConfig.toDomain` sets `cfg.Events = yc.Events.toDomain()`.

- [ ] **Step 4: Run** `go test ./internal/adapter/yaml/ -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/yaml/ && git commit -m "t66: adapter — decode the events: config section"
```

### Task 6: Core — `EventEmitter` + event contexts + generic expansion

**Files:**
- Modify: `internal/core/expand.go` (`expandTemplate`/`expandOne`/`expandCommands` take `any` data; add the two event contexts)
- Create: `internal/core/event.go`
- Test: Create `internal/core/event_test.go`

**Interfaces:**
- Consumes: `Config.Events` (Task 4), `Runner` + `fakeRunner`, `mtt.AuditStore` + `fakeAudit` (from `remove_test.go`), `PostActionError`, `missingAttributionFields`, `runsOf`, `firstFailure`.
- Produces (consumed by Tasks 7–11 and the CLI):

```go
type EventOptions struct{ NoRun bool; By, Why string }
func (o EventOptions) Preflight() error
type EventEmitter struct{ /* cfg, runner, audit, now */ }
func NewEventEmitter(cfg mtt.Config, runner Runner, audit mtt.AuditStore, now func() time.Time) *EventEmitter
func (e *EventEmitter) TaskEvent(kind mtt.EventKind, t mtt.Task, action string, opts EventOptions) error // nil-receiver-safe
func (e *EventEmitter) NoteEvent(kind mtt.EventKind, n mtt.Note, action string, opts EventOptions) error // nil-receiver-safe
```

- [ ] **Step 1: Write the failing tests** (`internal/core/event_test.go`) covering, each as its own test func against `fakeRunner`/`fakeAudit`:

1. `TestTaskEventRunsExpandedPipeline` — cfg with `Events.Task.Update.Post: [{Run: "echo {{.ID}} {{.Type}} {{.Event}}"}]`; `TaskEvent(EventUpdate, task{ID:"t1",Type:"task"}, "edit", EventOptions{})` → runner received `echo t1 task update`; nil error.
2. `TestTaskEventNoHookNoRun` — empty config → runner not called, nil error.
3. `TestNilEmitterIsInert` — `var e *EventEmitter; e.TaskEvent(...)` → nil, no panic.
4. `TestTaskEventTypeDriftIsFinalizationFailure` — task with `Type: "ghost"` not in cfg → `*PostActionError` with `Remaining == runs of the raw hook`, `Cause` containing `task type "ghost" not in config`; runner NOT called.
5. `TestTaskEventFromToIsTemplateError` — hook `{{.From}}` → `*PostActionError` (expansion is post-persist: finalization failure, not a plain error), runner NOT called.
6. `TestTaskEventCommandFailure` — hook `[ok, fail, never]`, fakeRunner exits 1 on "fail" → `*PostActionError{Remaining: ["fail","never"]}`.
7. `TestNoRunSkipsPipelineAndWritesAuditRecord` — opts `{NoRun: true, By: "me", Why: "because"}` → runner NOT called; fakeAudit got one entry `{Who: "me", Why: "because", Action: "edit --no-run", TaskID: "t1"}`. Also with an EMPTY config (no hooks) → record still written (pin i).
8. `TestNoRunAuditAppendFailure` — fakeAudit erroring → `*PostActionError` with empty `Remaining` and a Cause containing `the audit record was NOT written` (pin ii).
9. `TestPreflight` — `EventOptions{NoRun: true}.Preflight()` → `ErrMissingAttribution` listing `who, why`; `{NoRun: true, By: "a", Why: "b"}` → nil; `{NoRun: false}` → nil.
10. `TestNoteEventContext` — `Events.Note.Create.Post: [{Run: "echo {{.Slug}} {{.Event}}"}]` → runner received `echo my-note create`; `{{.ID}}` in a note hook → `*PostActionError`.

- [ ] **Step 2: Run** `go test ./internal/core/ -run 'TestTaskEvent|TestNoteEvent|TestNilEmitter|TestNoRun|TestPreflight' -v` → FAIL.

- [ ] **Step 3: Implement**

`expand.go` — change signatures (mechanical; `cmdContext` callers unchanged):

```go
func expandCommands(cmds []mtt.Command, data any) ([]mtt.Command, error)
func expandOne(c mtt.Command, data any) (mtt.Command, error)
func expandTemplate(raw string, data any) (string, error)
```

and add below `cmdContext`:

```go
// taskEventContext is the placeholder whitelist for task lifecycle events:
// {{.ID}}, {{.Type}}, {{.Event}}. No From/To — an event is not an edge; a
// stray {{.From}} is a template error (the struct shape self-enforces it).
// Type carries the CONFIG's type name (membership-checked by the emitter),
// never the raw on-disk value (spec §4, the c15-class guard).
type taskEventContext struct {
	ID    string
	Type  string
	Event string
}

// noteEventContext is the placeholder whitelist for note lifecycle events:
// {{.Slug}} (structurally validated kebab ASCII) and {{.Event}}.
type noteEventContext struct {
	Slug  string
	Event string
}
```

`internal/core/event.go`:

```go
package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// EventOptions carry one mutation's bypass + attribution into its lifecycle
// event: NoRun skips the event pipeline (t5 discipline: forces By+Why — the
// usecase calls Preflight BEFORE persisting), By/Why sign the skip record.
type EventOptions struct {
	NoRun bool
	By    string
	Why   string
}

// Preflight validates the bypass attribution BEFORE anything persists: a
// --no-run bypass without who+why is ErrMissingAttribution (exit 2) and the
// mutation must not happen. Every mutating usecase calls it first.
func (o EventOptions) Preflight() error {
	if !o.NoRun {
		return nil
	}
	if missing := missingAttributionFields(true, true, o.By, o.Why); len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrMissingAttribution, strings.Join(missing, ", "))
	}
	return nil
}

// EventEmitter runs the config-authored lifecycle-event pipelines (post-only,
// t66) after a successful mutation. Fired by the mutating USECASES — never the
// store (a flow move must not double-fire) and never Transitioner (an edge has
// its own post). A nil emitter is inert, so tests and event-less wiring pass nil.
type EventEmitter struct {
	cfg    mtt.Config
	runner Runner
	audit  mtt.AuditStore
	now    func() time.Time
}

// NewEventEmitter wires the emitter; audit receives the --no-run skip records.
func NewEventEmitter(cfg mtt.Config, runner Runner, audit mtt.AuditStore, now func() time.Time) *EventEmitter {
	return &EventEmitter{cfg: cfg, runner: runner, audit: audit, now: now}
}

// TaskEvent fires the task hook for kind after t was persisted (or deleted).
// The mutation is already durable, so every failure here — template, command,
// type drift, audit append — is a FINALIZATION failure: a *PostActionError the
// caller returns beside the persisted result (CLI: exit 5, "the change is
// already saved"). action is the invoked command's name ("add", "tag rm", …)
// for the skip record. Under opts.NoRun the pipeline is skipped and the audit
// record is written instead — whether or not a hook is configured (one rule).
func (e *EventEmitter) TaskEvent(kind mtt.EventKind, t mtt.Task, action string, opts EventOptions) error {
	if e == nil {
		return nil
	}
	if opts.NoRun {
		return e.skipRecord(action, string(t.ID), opts)
	}
	hook := e.cfg.Events.Task.Hook(kind)
	if len(hook.Post) == 0 {
		return nil
	}
	typ, ok := e.cfg.TypeByName(t.Type)
	if !ok {
		// c15-class guard (spec §4): the on-disk type: is validated only as
		// non-empty; never expand it. Membership in the trusted config
		// vocabulary is the shape-safety test.
		return &PostActionError{
			Remaining: runsOf(hook.Post),
			Cause:     fmt.Sprintf("task type %q not in config — event pipeline not run", t.Type),
		}
	}
	return e.run(hook.Post, taskEventContext{ID: string(t.ID), Type: string(typ.Name), Event: string(kind)})
}

// NoteEvent is TaskEvent's note-store sibling ({{.Slug}}/{{.Event}} context;
// the slug is structurally validated at every adapter boundary, so it is
// shape-safe by construction).
func (e *EventEmitter) NoteEvent(kind mtt.EventKind, n mtt.Note, action string, opts EventOptions) error {
	if e == nil {
		return nil
	}
	if opts.NoRun {
		return e.skipRecord(action, string(n.Slug), opts)
	}
	hook := e.cfg.Events.Note.Hook(kind)
	if len(hook.Post) == 0 {
		return nil
	}
	return e.run(hook.Post, noteEventContext{Slug: string(n.Slug), Event: string(kind)})
}

// run expands and executes one event pipeline. Expansion happens AFTER the
// persist (for create the ID does not exist earlier — spec §4), so a template
// error is a finalization failure too, never a lost mutation.
func (e *EventEmitter) run(post []mtt.Command, data any) error {
	expanded, err := expandCommands(post, data)
	if err != nil {
		return &PostActionError{Remaining: runsOf(post), Cause: fmt.Sprintf("expand event post: %v", err)}
	}
	checks, rerr := e.runner.Run(expanded)
	if rerr != nil {
		i := len(checks) - 1 // failing command is last (Runner CONTRACT)
		if i < 0 {
			i = 0
		}
		return &PostActionError{Remaining: runsOf(expanded[i:]), Cause: rerr.Error()}
	}
	if i, c, failed := firstFailure(checks); failed {
		return &PostActionError{
			Remaining: runsOf(expanded[i:]),
			Cause:     fmt.Sprintf("command %q exited %d", c.Cmd, c.Exit),
		}
	}
	return nil
}

// skipRecord signs a --no-run bypass into the audit log (t5: no bypass without
// a trail), written at the moment the skipped pipeline would have fired. A
// failed append is a finalization failure with NO recovery commands (there is
// no user-runnable re-append) — the CLI renders the message only.
func (e *EventEmitter) skipRecord(action, id string, opts EventOptions) error {
	entry := mtt.AuditEntry{
		At: e.now().UTC().Truncate(time.Second), Who: opts.By, Why: opts.Why,
		Action: action + " --no-run", TaskID: mtt.TaskID(id),
	}
	if err := e.audit.Append(entry); err != nil {
		return &PostActionError{
			Cause: fmt.Sprintf("audit append for %q: %v — the change is already saved; the audit record was NOT written", id, err),
		}
	}
	return nil
}
```

- [ ] **Step 4: Run** `go test ./internal/core/ ./pkg/mtt/ -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/ && git commit -m "t66: core — EventEmitter (post-only lifecycle pipelines, type-membership guard, --no-run audit sink)"
```

### Task 7: Wire task create/update usecases — `Adder`, `Editor`

**Files:**
- Modify: `internal/core/add.go`, `internal/core/edit.go`
- Test: `internal/core/add_test.go`, `internal/core/edit_test.go`

**Interfaces:**
- Consumes: Task 6.
- Produces: `NewAdder(store, cfg, now, ev *EventEmitter)`; `AddParams` gains `Events EventOptions`; `NewEditor(store, now, ev *EventEmitter)`; `EditParams` gains `Events EventOptions`. Both: `Preflight()` first; fire after successful persist; a `*PostActionError` is returned WITH the persisted task (mirror `Transitioner`'s contract comment).

- [ ] **Step 1: Write the failing tests** — in each usecase's test file (reuse its existing store fake):
  - `TestAddFiresCreateEvent`: emitter wired with a recording `fakeRunner` + hook on `Task.Create` → after `Add`, runner saw the pipeline with the MINTED id expanded; returned task valid, err nil.
  - `TestAddEventFailureKeepsTask`: hook exits 1 → `Add` returns the persisted task AND a `*PostActionError` (assert `errors.As`); store contains the task.
  - `TestAddNoRunPreflight`: `AddParams{Events: EventOptions{NoRun: true}}` (no By/Why) → `ErrMissingAttribution`, store EMPTY (nothing persisted).
  - `TestEditFiresUpdateEvent` / `TestEditNoRunPreflight`: same shape over `Editor` (fire on a real edit; preflight before persist).

- [ ] **Step 2: Run** → FAIL (signature/param changes).

- [ ] **Step 3: Implement**
  - `Adder` struct gains `ev *EventEmitter`; `NewAdder(store, cfg, now, ev)`; `AddParams` gains `Events EventOptions`.
  - In `Add`, first line: `if err := p.Events.Preflight(); err != nil { return mtt.Task{}, err }`.
  - After the successful `store.Create` (returned `created` task), before `return`:

```go
	if err := a.ev.TaskEvent(mtt.EventCreate, created, "add", p.Events); err != nil {
		return created, err // finalization failure: the task IS persisted (exit 5)
	}
```

  - `Editor` symmetric: preflight first; after `store.Update` fire `mtt.EventUpdate` with action `"edit"`.
  - Update all compile-broken call sites (CLI passes `nil` emitter for now — Task 10 wires it; tests pass `nil` where events are irrelevant).

- [ ] **Step 4: Run** `make test` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/ internal/cli/ && git commit -m "t66: core — Adder/Editor fire create/update events (preflight before persist)"
```

### Task 8: Wire task update/delete usecases — `TagEditor`, `DependencyEditor`, `RefEditor`, `Remover`

**Files:**
- Modify: `internal/core/tageditor.go`, `internal/core/dependency.go`, `internal/core/refedit.go`, `internal/core/remove.go`
- Test: their `_test.go` files

**Interfaces:**
- Consumes: Task 6.
- Produces:
  - `NewTagEditor(store, now, ev)`, `AddTags/RemoveTags(id, tags, opts EventOptions)` — fire `EventUpdate` (action `"tag add"`/`"tag rm"`) **only when the returned changed-set is non-empty** (idempotent no-op ⇒ no persist ⇒ no fire).
  - `NewDependencyEditor(store, now, ev)`, `AddDependency/RemoveDependency(id, dep, opts EventOptions)` — fire `EventUpdate` (action `"dep add"`/`"dep rm"`) only on a real change.
  - `NewRefEditor(store, now, ev)` — fire `EventUpdate` (action `"ref add"`/`"ref rm"`) only when it persisted.
  - `NewRemover(store, audit, now, ev)`; `Remove/RemoveMany(ids, force, by, why, bl, noRun bool)` — per id, INSIDE the loop, after a successful `store.Delete`: fire `EventDelete` (action `"rm"`) with `EventOptions{NoRun: noRun, By: by, Why: why}`; the event error rides that id's `RemoveResult.Err`. **Pin iii:** under `force && noRun` the pre-delete audit record's action becomes `"rm --force --no-run"`, NO second (skip) record is written, and the `TaskEvent` call is skipped entirely for that id (the one record already signed the bypass; no pipeline runs). Under `noRun && !force`: `Preflight` at the top of `RemoveMany` (before any deletion), then per id `TaskEvent(EventDelete, task, "rm", opts)` — which writes the skip record. Under `!noRun`: normal fire.

- [ ] **Step 1: Write the failing tests** (per usecase, in its `_test.go`; the load-bearing ones):
  - `TestTagAddNoOpFiresNoEvent`: add an already-present tag → runner not called.
  - `TestTagAddFiresUpdateEvent`: real add → one pipeline run.
  - `TestRemoveManyFiresDeletePerEntityInsideLoop`: 2 ids, hook records `{{.ID}}`; assert runner saw TWO runs, in id order, and (key assertion) the FIRST run happened before the SECOND delete — with the fakes, assert order via a shared recording slice: store.Delete and runner.Run append markers `delete:tN` / `run:tN`; expect `[delete:t1 run:t1 delete:t2 run:t2]`.
  - `TestRemoveManyEventFailureRidesResult`: hook fails for one id → that `RemoveResult.Err` is a `*PostActionError`, the other id deleted cleanly.
  - `TestRemoveForceNoRunWritesOneRecord`: `force=true, noRun=true, by/why set` → fakeAudit has exactly ONE entry per id with `Action: "rm --force --no-run"`; runner not called.
  - `TestRemoveNoRunPreflight`: `noRun=true` without by/why → pre-flight `ErrMissingAttribution`, nothing deleted.

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Implement.** Signature changes as in Interfaces. In `removeOne` the force branch becomes:

```go
	action := "rm --force"
	if noRun {
		action = "rm --force --no-run"
	}
	entry := mtt.AuditEntry{At: r.now().UTC().Truncate(time.Second), Who: by, Why: why, Action: action, TaskID: id}
	if err := r.audit.Append(entry); err != nil {
		return fmt.Errorf("audit append for %q: %w", id, err)
	}
	if err := r.store.Delete(id); err != nil {
		return err
	}
	if noRun {
		return nil // the force record above already signed the bypass (pin iii)
	}
	return r.ev.TaskEvent(mtt.EventDelete, task, "rm", EventOptions{})
```

(`task` is the value from the existing `store.Get` at the top of `removeOne` — change `if _, err :=` to capture it.) The non-force tail becomes:

```go
		if err := r.store.Delete(id); err != nil {
			return err
		}
		return r.ev.TaskEvent(mtt.EventDelete, task, "rm", EventOptions{NoRun: noRun, By: by, Why: why})
```

and `RemoveMany` starts with the bypass preflight (before the force preflight, same aggregation):

```go
	if noRun && !force {
		if err := (EventOptions{NoRun: true, By: by, Why: why}).Preflight(); err != nil {
			return nil, err
		}
	}
```

- [ ] **Step 4: Run** `make test` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/ internal/cli/ && git commit -m "t66: core — tag/dep/ref/rm fire update/delete events; rm --force --no-run single record"
```

### Task 9: Wire note usecases — `NoteAdder`, `NoteEditor`, `NoteRefEditor`, `NoteRemover`

**Files:**
- Modify: `internal/core/note.go`, `internal/core/refedit.go` (NoteRefEditor), `internal/core/noteremove.go`
- Test: `internal/core/note_test.go`, `internal/core/refedit_test.go`, `internal/core/noteremove_test.go`

**Interfaces:**
- Consumes: Task 6.
- Produces: constructors gain `ev *EventEmitter`; params/methods gain `EventOptions` (same pattern as Tasks 7–8); actions: `"note add"`, `"note edit"`, `"note ref add"`, `"note ref rm"`, `"note rm"`. `NoteRemover` mirrors `Remover` exactly: its existing forced-delete audit action becomes `"note rm --force --no-run"` under bypass (one record); the delete event fires after `DeleteNote`.

- [ ] **Step 1: Write the failing tests** — mirror Task 7/8's shapes over notes: `TestNoteAddFiresCreateEvent` (slug expanded via `{{.Slug}}`), `TestNoteEditNoRunPreflight`, `TestNoteRemoveForceNoRunOneRecord`, `TestNoteEventFailureKeepsNote`.

- [ ] **Step 2: Run** → FAIL. **Step 3: Implement** (same pattern; `NoteEvent` instead of `TaskEvent`). **Step 4:** `make test` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/ internal/cli/ && git commit -m "t66: core — note usecases fire note lifecycle events"
```

### Task 10: CLI wiring — emitter construction, `--no-run` flags, rendering contract

**Files:**
- Create: `internal/cli/events.go`
- Modify: `internal/cli/status.go` (extract the runner-construction block into a shared helper), `internal/cli/add.go`, `edit.go`, `tag.go`, `dep.go`, `ref.go`, `noteref.go`, `rm.go`, `note.go`, `bulk.go`
- Test: `internal/cli/events_test.go` (unit: recovery rendering, empty-Remaining), e2e in Task 11

**Interfaces:**
- Consumes: everything above.
- Produces:
  - `newGateRunner(cmd, root string, settings yaml.Settings) (core.Runner, func(), error)` — extracted verbatim from `runTransition`'s runner-construction block (progress writer, `gateOutputWriter`, `gateTailLines`, log-file close func); `runTransition` now calls it (DRY — behavior byte-identical, existing e2e pins it).
  - `newEventEmitter(cmd *cobra.Command, root string, cfg mtt.Config, settings yaml.Settings) (*core.EventEmitter, func(), error)` — `newGateRunner` + `yaml.NewAuditStore(root)` + `time.Now`.
  - `eventOptions(cmd *cobra.Command, noRun bool, author string) core.EventOptions` — `resolveAttribution` → `{NoRun, By, Why}`.
  - `renderPostRecovery(cmd *cobra.Command, err error) `— `errors.As` a `*core.PostActionError`: stderr block `the change IS saved; do NOT re-run the mutation` + `Remaining` lines each indented, **omitting the "run these to finish:" list entirely when `Remaining` is empty** (spec §7). Reuse/extract the t28 rendering from `status.go` so move-recovery and event-recovery render through ONE function (adjust the move wording only if the extraction forces it — otherwise keep both texts).
  - Every mutating command: local `--no-run` flag (`"skip the configured lifecycle-event pipeline (forces --who and --why)"`), wires the emitter into its usecase, and on error: render primary output first when the mutation persisted (the `*PostActionError` path), then `renderPostRecovery`, then return the error (exit 5). Single-entity `--json`: object on stdout, recovery on stderr (t28 order) — e.g. in `add.go`, the `RunE` tail becomes:

```go
			task, err := adder.Add(core.AddParams{ /* … existing fields … */, Events: eventOptions(cmd, noRun, settings.Author)})
			if err != nil && !errors.As(err, new(*core.PostActionError)) {
				return err
			}
			evErr := err // nil or the finalization failure — render the task either way
			for _, r := range refs {
				warnIfNotOK(cmd, r, verifyOne(root, r))
			}
			if jsonFlag(cmd) {
				if werr := writeJSON(cmd.OutOrStdout(), toTaskJSON(task)); werr != nil {
					return werr
				}
			} else if _, werr := fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", task.ID); werr != nil {
				return werr
			}
			renderPostRecovery(cmd, evErr)
			return evErr
```

    (note: `add.go` currently discards `settings` — change `cfg, _, err := yaml.Load(root)` to keep them.)
  - Bulk (`bulk.go`): `bulkJSON` gains `Remaining []string \`json:"remaining,omitempty"\``; `reportBulk` fills it via `errors.As` per item, and the human per-item stderr line for a `*PostActionError` appends the saved-marker + remaining commands.

- [ ] **Step 1: Write the failing unit test** (`internal/cli/events_test.go`): `TestRenderPostRecoveryEmptyRemaining` — a `*core.PostActionError` with no `Remaining` renders the saved-marker line and NOT the "run these" header; with `Remaining` renders each command indented. (Buffer-backed cobra command, assert stderr contents.)

- [ ] **Step 2: Run** → FAIL. **Step 3: Implement** per Interfaces (compiler-driven sweep over the 9 command files; `dep.go`/`tag.go`/`ref.go`/`noteref.go`/`note.go` pass `eventOptions(...)` through to their usecase calls; `rm.go` passes `noRun` into `Remove`/`RemoveMany` and keeps the exit-2 pre-flight forwarding raw).

- [ ] **Step 4: Run** `make check` → green (this is the first task where the full CLI compiles again with all wiring).

- [ ] **Step 5: Commit**

```bash
git add internal/cli/ && git commit -m "t66: cli — event emitter wiring, --no-run on mutations, shared post-recovery rendering"
```

### Task 11: e2e testscripts

**Files:**
- Create: `internal/cli/testdata/scripts/events.txt`, `events_norun.txt`, `events_bulk.txt`
- Modify (only if a pinned output changed): existing scripts — expect NONE to change (events are opt-in config; `mtt init` templates ship none).

**Interfaces:** consumes the full stack. Mirror the structure of `post_actions.txt` (t21's e2e) — same env bootstrap, same exit-code assert idiom.

- [ ] **Step 1: Write `events.txt`** — a config (files section of the txtar) with marker-file pipelines, no git needed:

```
# lifecycle events fire per entity on non-flow mutations; exit 5 keeps the mutation
env MTT_BY=tester
exec mtt add 'first'
exists created-t1-create
exec mtt tag add t1 x
exists updated-t1-update
exec mtt rm t1
exists deleted-t1-delete
# failing update pipeline: mutation kept, exit 5
exec mtt add 'second'
! exec mtt edit t2 --title renamed
stderr 'the change IS saved'
exec mtt show t2
stdout renamed

-- .mtt/config.yaml --
version: 1
project: {name: p}
events:
  task:
    create: {post: ['touch created-{{.ID}}-{{.Event}}']}
    update: {post: ['test {{.ID}} = t1 && touch updated-{{.ID}}-{{.Event}} || false']}
    delete: {post: ['touch deleted-{{.ID}}-{{.Event}}']}
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: tbd, kind: initial}
      - {name: doing, kind: active}
      - {name: done, kind: terminal}
    transitions:
      - {from: tbd, to: doing}
      - {from: doing, to: done}
```

(The `update` hook is engineered to succeed for `t1` and fail for `t2` — one config exercises both paths. Adjust the exact assert lines to the harness idioms found in `post_actions.txt`, including the exit-code check used there for exit 5.)

- [ ] **Step 2: Write `events_norun.txt`** — `--no-run` forcing + audit sink:

```
env MTT_BY=
! exec mtt add 'x' --no-run
stderr 'who, why'
exec mtt add 'x' --no-run --who me --why testing
exists .mtt/audit.log
grep '"action":"add --no-run"' .mtt/audit.log
! exists created-t1-create
```

(plus the same config files section with a create hook, proving the pipeline was skipped).

- [ ] **Step 3: Write `events_bulk.txt`** — the mixed matrix (s008.9): `mtt rm t1 ghost -` style bulk with a not-found id + a failing delete hook → aggregate exit **1** (assert via the harness's exit-code idiom), per-item stderr lines include the remaining-commands block; `--json` rows carry `remaining`.

- [ ] **Step 4: Run** `go test ./internal/cli/ -run TestScript -v` (the testscript harness) → PASS; then `make check` → green.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/testdata/ && git commit -m "t66: e2e — event pipelines, --no-run audit sink, bulk mixed matrix"
```

---

## Stage C — Dogfood config + docs

### Task 12: Dogfood config rewrite + `TestRepoDogfoodConfig`

**Files:**
- Modify: `.mtt/config.yaml`, `internal/adapter/yaml/dogfood_test.go`

**⚠ From this commit on, run flow moves with `make build && ./bin/mtt …` (see Global Constraints).**

- [ ] **Step 1: Extend `TestRepoDogfoodConfig` (failing first)** with pins for: (a) `events.task`/`events.note` hooks exist for all three kinds; (b) every event post uses the narrowed pathspec (assert the command strings contain `.mtt/tasks/{{.ID}}.yaml` / `.mtt/knowledge/{{.Slug}}.md` and the `audit.log` conditional, and do NOT contain `git add .mtt `-broad); (c) `git add` is `&&`-chained (assert substring `git add -- $a && {`); (d) event commit subjects are namespaced (`-m "mtt: ` substring; no `-m "{{.ID}}: ` in any event post); (e) both types carry `post_defaults` with the `{{.From}} → {{.To}}` commit line; (f) `deliver`/`cancel` edges have `SkipPostDefaults=true` and their own narrowed commit + `git push origin main`; (g) every OTHER edge has an empty `Post` except `approve` (push + PR only) and (h) the deliver gate contains `grep -v "^mtt: "`.

- [ ] **Step 2: Rewrite `.mtt/config.yaml`.** Add at top level (after `require:`):

```yaml
events:
  task:
    create: &task_event
      post:
        - 'a=.mtt/tasks/{{.ID}}.yaml; test -f .mtt/audit.log && a="$a .mtt/audit.log"; git add -- $a && { git diff --cached --quiet -- $a || git commit -m "mtt: {{.ID}} {{.Event}}" -- $a; }'
        - '[ "$(git branch --show-current)" != main ] || git push origin main || { echo "push failed — git pull first, then git push origin main" >&2; false; }'
    update: *task_event
    delete: *task_event
  note:
    create: &note_event
      post:
        - 'a=.mtt/knowledge/{{.Slug}}.md; test -f .mtt/audit.log && a="$a .mtt/audit.log"; git add -- $a && { git diff --cached --quiet -- $a || git commit -m "mtt: note {{.Slug}} {{.Event}}" -- $a; }'
        - '[ "$(git branch --show-current)" != main ] || git push origin main || { echo "push failed — git pull first, then git push origin main" >&2; false; }'
    update: *note_event
    delete: *note_event
```

(YAML anchors keep the three kinds byte-identical; if the decoder or the guard test prefers explicitness, inline the block three times — the pins in Step 1 are on content, not on anchors.) Then, per type:

- add `post_defaults:` right after `default:`/`description:`:

```yaml
    post_defaults:
      - 'git add .mtt && git commit -m "{{.ID}}: {{.From}} → {{.To}}" -- .mtt'
```

- DELETE the per-edge `post:` block from every edge whose post is exactly that line (start, submit ×N, approve→spec_human_review, decline ×N, plan edges, impl submit edges — ~20 edges over both types);
- `approve` (impl_review→approved, both types): keep only the push + PR lines in `post:` (the commit now comes from `post_defaults`);
- `deliver` and every `cancel` edge (both types): add `inherit_post: false` above their existing `post:` (which stays byte-identical — narrowed pathspec + push main);
- `deliver` gate (both types): change the squash-grep line to:

```yaml
          - 'git log -n 200 --format=%s | grep -v "^mtt: " | grep "^{{.ID}}: " || { echo "no squash commit \"{{.ID}}: …\" on local main: git pull first, and check the PR/merge title started with \"{{.ID}}: \"" >&2; false; }'
```

- [ ] **Step 3: Run** `go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig -v` → PASS; `make check` → green.

- [ ] **Step 4: Sanity-run the live flow with the new binary**: `make build`, then `./bin/mtt show t66` (loads the rewritten config; no validation errors).

- [ ] **Step 5: Commit**

```bash
git add .mtt/config.yaml internal/adapter/yaml/dogfood_test.go && git commit -m "t66: dogfood — events auto-commit, post_defaults collapse (~28 post blocks -> 2), deliver grep filters event subjects"
```

### Task 13: Docs — DESIGN/CLI_REFERENCE/FLOW_GUIDE (+RU), CLAUDE.md ×4, CHANGELOG

**Files:**
- Modify: `DESIGN.md` + `DESIGN.ru.md`, `CLI_REFERENCE.md` + `CLI_REFERENCE.ru.md`, `FLOW_GUIDE.md` + `FLOW_GUIDE.ru.md`, `pkg/mtt/CLAUDE.md`, `internal/core/CLAUDE.md`, `internal/cli/CLAUDE.md`, `internal/adapter/yaml/CLAUDE.md`, `CHANGELOG.md`

- [ ] **Step 1: DESIGN.md** — add a **“Lifecycle events (t66)”** subsection after the t21 post-actions block covering: the two-layer decision + rejected alternatives (approach 2/3, pseudo-edges — condensed from the spec §Decisions); firing model (usecase layer, per-entity, real-persist-only, never Transitioner); contexts + the `{{.Type}}` membership guard; failure semantics (single-entity 5, bulk stays s008.9 exit 1); `--no-run` audit sink incl. the three pins; the event-subject/deliver-grep collision rule. Add `post_defaults`/`inherit_post` + the precedence rule to the flow section, and a Decisions-table row. **Sweep parallel occurrences**: `grep -n "post:" DESIGN.md` and update the t21 narrative ("per-edge only for now — t24" is now resolved), the dogfood section's move-commit description, and the "Why not a global default post?" caveat. Mirror every change in `DESIGN.ru.md`.
- [ ] **Step 2: CLI_REFERENCE (+ru)** — `events:` config reference (shape, placeholders per store, post-only), `post_defaults:`/`inherit_post:`, `--no-run` + forced `--who/--why` + audit record on the 14 mutating verbs, exit-5-on-mutations semantics + empty-Remaining rendering, bulk `--json` `remaining` field.
- [ ] **Step 3: FLOW_GUIDE (+ru)** — an “events” authoring section with the SAFE sample block (narrowed pathspec, `&&`-chain, namespaced subjects) and the no-silent-traps bar; forward-link t62.
- [ ] **Step 4: CLAUDE.md ×4** — `pkg/mtt` (EventKind/Events/EffectivePost/validate rules), `internal/core` (EventEmitter + EventOptions + firing rules), `internal/cli` (events.go helpers, --no-run, rendering), `internal/adapter/yaml` (events/post_defaults DTOs).
- [ ] **Step 5: CHANGELOG** — one `[Unreleased]` entry: events + post_defaults + dogfood auto-commit + `--no-run` on mutations.
- [ ] **Step 6:** `make check` → green.
- [ ] **Step 7: Commit**

```bash
git add DESIGN*.md CLI_REFERENCE*.md FLOW_GUIDE*.md CHANGELOG.md pkg/mtt/CLAUDE.md internal/*/CLAUDE.md && git commit -m "t66: docs — lifecycle events + post_defaults (EN+RU, CLAUDE.md sweep)"
```

### Task 14: Close the loop

- [ ] `make check` green; run the Principles self-check (AGENTS.md).
- [ ] `./bin/mtt submit` (impl gate: clean tree + CHANGELOG + make check) → adversarial code review per the flow.
- [ ] After t66 delivers (`mtt deliver`): `mtt cancel t26 --why "subsumed by t66 (events auto-commit shipped as dogfood config)"` and `mtt cancel t24 --why "resolved by t66 (post_defaults + inherit_post + defaults-first rule)"` — closure via flow edges, run with the NEW binary.
