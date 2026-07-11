# Dangerous-Ops Attribution (t5) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Force `--who` + `--why` at dangerous points — bypassing a gate (`--no-run`), a critical transition (per-edge `require`), and a destructive `rm --force` — recording the destruction to an append-only `.mtt/audit.log`.

**Architecture:** Policy lives in `core` (hexagonal): the `Transitioner` unions the effective required-attribution from `{global, per-edge, --no-run}`; the `Remover` gains a pre-flight who/why precondition and writes an audit record **before** deleting. The audit log is a new driven port `mtt.AuditStore` with a JSONL YAML-adapter. No new domain storage for transitions — the reason rides the existing `HistoryEntry.Why`.

**Tech Stack:** Go 1.23, cobra CLI, `text/template` gates, `testscript` (txtar) e2e, YAML (`gopkg.in/yaml.v3`), `encoding/json` for the audit line.

## Global Constraints

- **TDD, red→green→refactor.** Write the failing test first; run it; implement minimally; run; commit.
- **`make check` green before every commit** (gofmt + go vet + golangci-lint v2 + `go test -race -cover` + build). No unused vars (lint fails the build).
- **Hexagonal boundaries:** `cli → core → port ← adapter`. Policy in `core`; storage/audit only through a port; no business logic in `cli`; `pkg/mtt` holds domain types + ports.
- **Sentinels are core policy:** `ErrMissingAttribution` → CLI **exit 2** (existing `exitCode` map in `internal/cli/root.go:171-184`).
- **Attribution wording:** the missing-fields message is `mtt: missing required attribution: <fields>` (existing `ErrMissingAttribution` text + `strings.Join(missing, ", ")`), produced from ONE shared helper.
- **Anti-vacuity:** `who` is pre-satisfied in this repo (global `require.who: true` + `config.local` author). Any test asserting a path *forces who* MUST set `RequireWho=false` **and** empty `By`, else it passes vacuously.
- **Time:** UTC, truncated to seconds (`now().UTC().Truncate(time.Second)`), matching existing history stamps.

---

## File Structure

- `pkg/mtt/config.go` — **modify**: add `type Require struct{Who,Why bool}`; add `Require` field to `Transition`.
- `pkg/mtt/audit.go` — **create**: `AuditEntry` + `AuditStore` port.
- `internal/core/transition.go` — **modify**: refactor `missingAttribution` → shared field-level helper; union `effWho/effWhy`.
- `internal/core/remove.go` — **modify**: `Remover` gains `audit` + `now`; new `NewRemover`/`Remove`/`RemoveMany` signatures; pre-flight check; append-before-delete.
- `internal/adapter/yaml/audit.go` — **create**: `AuditStore` impl (JSONL append to `.mtt/audit.log`).
- `internal/adapter/yaml/dto.go` — **modify**: `ymlTransition.Require`; map it in `toConfig`.
- `internal/cli/rm.go` — **modify**: load settings, resolve attribution, wire audit store, thread `by`/`why`, forward the pre-flight error raw.
- `.gitattributes` — **create**: `/.mtt/audit.log merge=union`.
- `internal/cli/testdata/scripts/dangerous.txt` — **create**: e2e for `--no-run` and `rm --force`.
- `docs/architecture/model.go` — **modify**: `Remover`/`NewRemover` declarations.
- Tests: `internal/core/transition_test.go`, `internal/core/remove_test.go`, `internal/adapter/yaml/audit_test.go`, plus a decode test in `internal/adapter/yaml`.
- Docs sync (final task): `DESIGN.md`/`DESIGN.ru.md`, `CLI_REFERENCE.md`/`CLI_REFERENCE.ru.md`, `AGENTS.md`, three package `CLAUDE.md` (cli/core/adapter-yaml).

---

## Task 1: Union required-attribution policy in the Transitioner

**Files:**
- Modify: `pkg/mtt/config.go` (add `Require` type + `Transition.Require` field)
- Modify: `internal/core/transition.go:63` (call site) and `:115-126` (`missingAttribution` → `missingAttributionFields`)
- Test: `internal/core/transition_test.go`

**Interfaces:**
- Produces: `mtt.Require{Who bool; Why bool}`; `mtt.Transition.Require Require`; `core.missingAttributionFields(reqWho, reqWhy bool, by, why string) []string` (unexported, package `core`).
- Consumes: existing `TransitionOptions{Role,By,Why,NoRun,RequireWho,RequireWhy}`, `ErrMissingAttribution`.

- [ ] **Step 1: Add the domain types.** In `pkg/mtt/config.go`, add after the `Transition` struct:

```go
// Require is a required-attribution policy: who/why must be supplied. Used as the
// project-global default and as a per-edge (Transition) override; the two are
// unioned (tighten-only) — see core.Transitioner.
type Require struct {
	Who bool
	Why bool
}
```

and add the field to `Transition` (after `Current`):

```go
	Require Require // per-edge required attribution (zero = none); unioned with global + --no-run
```

- [ ] **Step 2: Write the failing test.** In `internal/core/transition_test.go`, add (use the file's existing fake `TaskStore`/`Runner` helpers and `cfg` builders — mirror a neighboring test for setup):

```go
func TestTransition_NoRunForcesWhyAndWho(t *testing.T) {
	// Edge with no require and no commands; global policy OFF → only --no-run forces.
	store := newFakeStore(taskAt("t1", "speccing")) // helper: task in status "speccing"
	cfg := cfgWithEdge("speccing", "planning")      // helper: one type, one edge, no commands/require
	tr := NewTransitioner(store, cfg, fakeRunner{}, fixedNow)

	// (b) missing why → error, mentioning why (who is present via By)
	_, err := tr.Transition("t1", "planning", TransitionOptions{By: "alice", NoRun: true})
	if !errors.Is(err, ErrMissingAttribution) || !strings.Contains(err.Error(), "why") {
		t.Fatalf("no-run without why: want ErrMissingAttribution mentioning why, got %v", err)
	}

	// (b′) non-vacuous who: RequireWho=false AND By="" → who forced by --no-run
	_, err = tr.Transition("t1", "planning", TransitionOptions{Why: "bypass ci", NoRun: true})
	if !errors.Is(err, ErrMissingAttribution) || !strings.Contains(err.Error(), "who") {
		t.Fatalf("no-run without who: want ErrMissingAttribution mentioning who, got %v", err)
	}

	// success: both present → moves, Why recorded
	task, err := tr.Transition("t1", "planning", TransitionOptions{By: "alice", Why: "bypass ci", NoRun: true})
	if err != nil {
		t.Fatalf("no-run with who+why: unexpected error %v", err)
	}
	if got := task.History[len(task.History)-1].Why; got != "bypass ci" {
		t.Fatalf("Why not recorded: got %q", got)
	}
}

func TestTransition_PerEdgeRequireUnionsWithGlobal(t *testing.T) {
	store := newFakeStore(taskAt("t1", "speccing"))
	cfg := cfgWithRequireEdge("speccing", "planning", mtt.Require{Why: true}) // edge require: why
	tr := NewTransitioner(store, cfg, fakeRunner{}, fixedNow)

	// global who + edge why → both required; give only who → missing why
	_, err := tr.Transition("t1", "planning", TransitionOptions{By: "alice", RequireWho: true})
	if !errors.Is(err, ErrMissingAttribution) || !strings.Contains(err.Error(), "why") {
		t.Fatalf("union: want missing why, got %v", err)
	}
}
```

> If helpers like `cfgWithRequireEdge`/`taskAt` don't exist, add small local builders in the test file (do not export). Keep them beside the test.

- [ ] **Step 3: Run the test — verify it fails.**

Run: `go test ./internal/core/ -run TestTransition_NoRunForces -v`
Expected: FAIL (compile error `Require` unknown, or wrong behavior — `missingAttribution` ignores `NoRun`/`edge`).

- [ ] **Step 4: Refactor the helper and add the union.** In `internal/core/transition.go`, replace `missingAttribution` (lines ~115-126):

```go
// missingAttributionFields reports which required attribution fields (who/why) are
// absent, aggregated so the caller can fix them all in one shot. The single home
// for the who/why field check, shared by the transition path and the rm pre-flight.
func missingAttributionFields(reqWho, reqWhy bool, by, why string) []string {
	var missing []string
	if reqWho && by == "" {
		missing = append(missing, "who")
	}
	if reqWhy && why == "" {
		missing = append(missing, "why")
	}
	return missing
}
```

Then in `Transition`, replace the check at line ~63 (the `if missing := missingAttribution(opts); …` block) with the union, using the already-found `edge`:

```go
	effWho := opts.RequireWho || edge.Require.Who || opts.NoRun
	effWhy := opts.RequireWhy || edge.Require.Why || opts.NoRun
	if missing := missingAttributionFields(effWho, effWhy, opts.By, opts.Why); len(missing) > 0 {
		return mtt.Task{}, fmt.Errorf("%w: %s", ErrMissingAttribution, strings.Join(missing, ", "))
	}
```

- [ ] **Step 5: Run the tests — verify they pass.**

Run: `go test ./internal/core/ -run 'TestTransition_(NoRunForces|PerEdgeRequire)' -v`
Expected: PASS. Also run the whole core package to catch regressions: `go test ./internal/core/`
Expected: PASS (existing attribution tests still green — the helper is behavior-equivalent when `NoRun=false` and `edge.Require` is zero).

- [ ] **Step 6: Commit.**

```bash
git add pkg/mtt/config.go internal/core/transition.go internal/core/transition_test.go
git commit -m "feat(t5): union required-attribution (global + per-edge + --no-run) in Transitioner"
```

---

## Task 2: AuditStore port + JSONL adapter

**Files:**
- Create: `pkg/mtt/audit.go`
- Create: `internal/adapter/yaml/audit.go`
- Test: `internal/adapter/yaml/audit_test.go`

**Interfaces:**
- Produces: `mtt.AuditEntry{At time.Time; Who, Why, Action string; TaskID TaskID}`; `mtt.AuditStore` interface (`Append(AuditEntry) error`); `yaml.NewAuditStore(root string) *AuditStore` returning a value that satisfies `mtt.AuditStore`.
- Consumes: nothing new.

- [ ] **Step 1: Define the port.** Create `pkg/mtt/audit.go`:

```go
package mtt

import "time"

// AuditEntry records one out-of-flow dangerous action — a destruction that has no
// task history to carry its attribution.
type AuditEntry struct {
	At     time.Time
	Who    string // acting subject (--who/--by/MTT_BY/config.local author)
	Why    string // --why
	Action string // e.g. "rm --force"
	TaskID TaskID
}

// AuditStore appends dangerous-action records. Append-only; no read surface (t5).
type AuditStore interface {
	Append(AuditEntry) error
}
```

- [ ] **Step 2: Write the failing test.** Create `internal/adapter/yaml/audit_test.go`:

```go
package yaml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestAuditStore_AppendWritesJSONL(t *testing.T) {
	root := t.TempDir()
	s := NewAuditStore(root)
	at := time.Date(2026, 7, 11, 9, 20, 0, 0, time.UTC)

	if err := s.Append(mtt.AuditEntry{At: at, Who: "alice", Why: "stale dup", Action: "rm --force", TaskID: "t7"}); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if err := s.Append(mtt.AuditEntry{At: at, Who: "bob", Why: "bad import", Action: "rm --force", TaskID: "t9"}); err != nil {
		t.Fatalf("append 2: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".mtt", "audit.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := splitNonEmpty(string(raw)) // local helper below
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), raw)
	}
	var got struct {
		At, Who, Why, Action, ID string
	}
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("line 0 not JSON: %v", err)
	}
	if got.Who != "alice" || got.Why != "stale dup" || got.Action != "rm --force" || got.ID != "t7" || got.At != "2026-07-11T09:20:00Z" {
		t.Fatalf("line 0 fields wrong: %+v", got)
	}
}

func TestAuditStore_AppendCreatesMttDir(t *testing.T) {
	root := t.TempDir() // no .mtt yet
	if err := NewAuditStore(root).Append(mtt.AuditEntry{At: time.Unix(0, 0).UTC(), Action: "rm --force", TaskID: "t1"}); err != nil {
		t.Fatalf("append into fresh root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".mtt", "audit.log")); err != nil {
		t.Fatalf("log not created: %v", err)
	}
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}
```

> Add `"strings"` to the test imports.

- [ ] **Step 3: Run the test — verify it fails.**

Run: `go test ./internal/adapter/yaml/ -run TestAuditStore -v`
Expected: FAIL (`NewAuditStore` undefined).

- [ ] **Step 4: Implement the adapter.** Create `internal/adapter/yaml/audit.go`:

```go
package yaml

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// AuditStore is the append-only JSONL audit log at <root>/.mtt/audit.log.
type AuditStore struct{ root string }

// NewAuditStore wires the audit adapter for a project root.
func NewAuditStore(root string) *AuditStore { return &AuditStore{root: root} }

// auditLine is the on-disk JSON shape (keeps pkg/mtt free of json tags).
type auditLine struct {
	At     string `json:"at"`
	Who    string `json:"who,omitempty"`
	Why    string `json:"why,omitempty"`
	Action string `json:"action"`
	ID     string `json:"id"`
}

// Append writes one JSON line, creating .mtt if absent. Append-only (O_APPEND).
func (s *AuditStore) Append(e mtt.AuditEntry) error {
	dir := filepath.Join(s.root, ".mtt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("audit: mkdir %s: %w", dir, err)
	}
	line := auditLine{
		At:     e.At.UTC().Format(time.RFC3339),
		Who:    e.Who,
		Why:    e.Why,
		Action: e.Action,
		ID:     string(e.TaskID),
	}
	b, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("audit: marshal: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("audit: open log: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("audit: write: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run the tests — verify they pass.**

Run: `go test ./internal/adapter/yaml/ -run TestAuditStore -v`
Expected: PASS (both).

- [ ] **Step 6: Commit.**

```bash
git add pkg/mtt/audit.go internal/adapter/yaml/audit.go internal/adapter/yaml/audit_test.go
git commit -m "feat(t5): AuditStore port + JSONL .mtt/audit.log adapter"
```

---

## Task 3: Remover — force ⇒ who+why (pre-flight) + append-before-delete

**Files:**
- Modify: `internal/core/remove.go`
- Modify: `docs/architecture/model.go:598-611`
- Test: `internal/core/remove_test.go` (update ~11 call sites + new cases)

**Interfaces:**
- Consumes: `mtt.AuditStore` (Task 2), `missingAttributionFields` (Task 1).
- Produces: `NewRemover(store mtt.TaskStore, audit mtt.AuditStore, now func() time.Time) *Remover`; `RemoveMany(ids []mtt.TaskID, force bool, by, why string) ([]RemoveResult, error)`; `Remove(id mtt.TaskID, force bool, by, why string) error`.

- [ ] **Step 1: Write the failing tests.** In `internal/core/remove_test.go`, add a fake audit store and new cases (and update existing `NewRemover(store)` call sites — see Step 4):

```go
type fakeAudit struct {
	entries  []mtt.AuditEntry
	failOnID mtt.TaskID // if set, Append returns an error for that id
	order    []string   // "append:<id>" / used with fakeStore to assert ordering
}

func (f *fakeAudit) Append(e mtt.AuditEntry) error {
	if e.TaskID == f.failOnID {
		return fmt.Errorf("disk full")
	}
	f.entries = append(f.entries, e)
	return nil
}

func TestRemoveMany_ForceRequiresWhoAndWhy(t *testing.T) {
	store := newFakeStore(taskAt("t1", "tbd"))
	audit := &fakeAudit{}
	r := NewRemover(store, audit, fixedNow)

	res, err := r.RemoveMany([]mtt.TaskID{"t1"}, true, "", "") // force, no who/why
	if !errors.Is(err, ErrMissingAttribution) {
		t.Fatalf("want pre-flight ErrMissingAttribution, got %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("want empty results on pre-flight fail, got %d", len(res))
	}
	if store.deleted("t1") {
		t.Fatal("nothing must be deleted on pre-flight failure")
	}
	if len(audit.entries) != 0 {
		t.Fatal("no audit entry on pre-flight failure")
	}
}

func TestRemoveMany_ForceAppendsBeforeDelete(t *testing.T) {
	store := newFakeStore(taskAt("t1", "tbd"))
	audit := &fakeAudit{}
	r := NewRemover(store, audit, fixedNow)

	res, err := r.RemoveMany([]mtt.TaskID{"t1"}, true, "alice", "cleanup")
	if err != nil {
		t.Fatalf("pre-flight error: %v", err)
	}
	if res[0].Err != nil {
		t.Fatalf("delete error: %v", res[0].Err)
	}
	if len(audit.entries) != 1 || audit.entries[0].TaskID != "t1" ||
		audit.entries[0].Who != "alice" || audit.entries[0].Why != "cleanup" ||
		audit.entries[0].Action != "rm --force" {
		t.Fatalf("audit entry wrong: %+v", audit.entries)
	}
	if !store.deleted("t1") {
		t.Fatal("task should be deleted after successful append")
	}
}

func TestRemoveMany_AppendFailureSkipsDelete(t *testing.T) {
	store := newFakeStore(taskAt("t1", "tbd"), taskAt("t2", "tbd"))
	audit := &fakeAudit{failOnID: "t1"}
	r := NewRemover(store, audit, fixedNow)

	res, err := r.RemoveMany([]mtt.TaskID{"t1", "t2"}, true, "alice", "cleanup")
	if err != nil {
		t.Fatalf("pre-flight error: %v", err)
	}
	if res[0].Err == nil {
		t.Fatal("t1 append failed → its RemoveResult.Err must be set")
	}
	if store.deleted("t1") {
		t.Fatal("t1 must NOT be deleted when its audit append failed")
	}
	if res[1].Err != nil || !store.deleted("t2") {
		t.Fatalf("t2 should proceed independently: err=%v deleted=%v", res[1].Err, store.deleted("t2"))
	}
}

func TestRemoveMany_NoForceUnchanged(t *testing.T) {
	store := newFakeStore(taskAt("t1", "tbd"))
	audit := &fakeAudit{}
	res, err := NewRemover(store, audit, fixedNow).RemoveMany([]mtt.TaskID{"t1"}, false, "", "")
	if err != nil {
		t.Fatalf("no-force must not pre-flight error: %v", err)
	}
	if res[0].Err != nil || !store.deleted("t1") {
		t.Fatalf("no-force delete: err=%v deleted=%v", res[0].Err, store.deleted("t1"))
	}
	if len(audit.entries) != 0 {
		t.Fatal("no audit on non-force delete")
	}
}
```

> The fake `TaskStore` in this package needs a `deleted(id)` probe. If it doesn't record deletes, extend it (add a `map[mtt.TaskID]bool` set in `Delete`). If `newFakeStore`/`taskAt`/`fixedNow` don't exist, add minimal local versions mirroring `transition_test.go`.

- [ ] **Step 2: Run the tests — verify they fail.**

Run: `go test ./internal/core/ -run TestRemoveMany -v`
Expected: FAIL (signature mismatch: `NewRemover` wants 1 arg; `RemoveMany` returns 1 value).

- [ ] **Step 3: Rewrite `remove.go`.** Replace the top of `internal/core/remove.go` (struct + constructor + Remove + RemoveMany + removeOne) with:

```go
// Remover is the delete-a-task usecase. By default it refuses to delete a task
// referenced by others; --force overrides. Under --force it FORCES who+why
// (pre-flight) and writes an audit record BEFORE deleting (no destruction without
// a preceding record). now is injected for deterministic audit timestamps.
type Remover struct {
	store mtt.TaskStore
	audit mtt.AuditStore
	now   func() time.Time
}

// NewRemover wires the usecase with the audit port and an injected clock.
func NewRemover(store mtt.TaskStore, audit mtt.AuditStore, now func() time.Time) *Remover {
	return &Remover{store: store, audit: audit, now: now}
}

// Remove deletes a single id. Thin wrapper over RemoveMany([id]); it forwards the
// pre-flight error and, absent that, the per-id result error. (The empty-slice
// check guards the [0] index on the pre-flight path.)
func (r *Remover) Remove(id mtt.TaskID, force bool, by, why string) error {
	res, err := r.RemoveMany([]mtt.TaskID{id}, force, by, why)
	if err != nil {
		return err
	}
	return res[0].Err
}

// RemoveMany deletes each id best-effort. The error return is the PRE-FLIGHT
// precondition failure (missing attribution under --force) — returned before any
// deletion, with a nil results slice; the CLI forwards it raw (exit 2). Per-id
// outcomes ride []RemoveResult. Under --force each id is audited BEFORE delete.
func (r *Remover) RemoveMany(ids []mtt.TaskID, force bool, by, why string) ([]RemoveResult, error) {
	if force {
		if missing := missingAttributionFields(true, true, by, why); len(missing) > 0 {
			return nil, fmt.Errorf("%w: %s", ErrMissingAttribution, strings.Join(missing, ", "))
		}
	}

	ordered := dedupIDSlice(ids)
	set := make(map[mtt.TaskID]bool, len(ordered))
	for _, id := range ordered {
		set[id] = true
	}

	var idx Index
	var g DepGraph
	var snapErr error
	if !force {
		tasks, err := r.store.List()
		if err != nil {
			snapErr = fmt.Errorf("list tasks: %w", err)
		} else {
			idx = NewIndex(tasks)
			g = NewDepGraph(tasks)
		}
	}

	results := make([]RemoveResult, 0, len(ordered))
	for _, id := range ordered {
		results = append(results, RemoveResult{ID: id, Err: r.removeOne(id, force, by, why, set, idx, g, snapErr)})
	}
	return results, nil
}

// removeOne deletes one id. Under --force it appends the audit record FIRST; only
// on a successful append does it delete (so a failed append leaves the task — and
// the current pointer — intact). Without --force the subgraph referenced-check runs
// and no audit is written.
func (r *Remover) removeOne(id mtt.TaskID, force bool, by, why string, set map[mtt.TaskID]bool, idx Index, g DepGraph, snapErr error) error {
	if _, err := r.store.Get(id); err != nil {
		if errors.Is(err, mtt.ErrNotFound) {
			return fmt.Errorf("task %q: %w", id, mtt.ErrNotFound)
		}
		return fmt.Errorf("load task %q: %w", id, err)
	}
	if !force {
		if snapErr != nil {
			return snapErr
		}
		if refs := externalReferencingIDs(idx, g, id, set); len(refs) > 0 {
			return fmt.Errorf("task %q is referenced by %s; use --force to delete anyway",
				id, strings.Join(refs, ", "))
		}
		return r.store.Delete(id)
	}
	// force: record BEFORE destroying.
	entry := mtt.AuditEntry{At: r.now().UTC().Truncate(time.Second), Who: by, Why: why, Action: "rm --force", TaskID: id}
	if err := r.audit.Append(entry); err != nil {
		return fmt.Errorf("audit append for %q: %w", id, err)
	}
	return r.store.Delete(id)
}
```

Add `"time"` to the `remove.go` imports.

- [ ] **Step 4: Update all `NewRemover`/`Remove`/`RemoveMany` call sites in the test file.** In `internal/core/remove_test.go`, every existing `NewRemover(store)` becomes `NewRemover(store, &fakeAudit{}, fixedNow)`; every `.Remove(id, force)` becomes `.Remove(id, force, "who", "why")` (or `"",""` where testing non-force); every `res := …RemoveMany(ids, force)` becomes `res, _ := …RemoveMany(ids, force, "who", "why")` (discard the pre-flight error where not under test, or assert it where relevant). Run `grep -n 'NewRemover\|\.Remove(\|RemoveMany(' internal/core/remove_test.go` to enumerate them.

- [ ] **Step 5: Update the architecture reference.** In `docs/architecture/model.go`, change the `Remover` interface and `NewRemover` var (lines ~598-611):

```go
type Remover interface {
	Remove(id TaskID, force bool, by, why string) error
	RemoveMany(ids []TaskID, force bool, by, why string) ([]RemoveResult, error)
}
```

```go
// NewRemover wires the delete usecase — TaskStore + AuditStore + injected clock
// (audit records dangerous --force deletes; who/why forced pre-flight). [s008.5; t5]
var NewRemover func(store TaskStore, audit AuditStore, now func() time.Time) Remover
```

- [ ] **Step 6: Run the tests — verify they pass.**

Run: `go test ./internal/core/ -run TestRemove -v`
Expected: PASS (new + updated existing). Then `go test ./internal/core/` → PASS.

- [ ] **Step 7: Commit.**

```bash
git add internal/core/remove.go internal/core/remove_test.go docs/architecture/model.go
git commit -m "feat(t5): rm --force forces who+why (pre-flight) + append-before-delete audit"
```

---

## Task 4: Decode per-edge `require` in the YAML config

**Files:**
- Modify: `internal/adapter/yaml/dto.go` (`ymlTransition` field + `toConfig` map at ~line 132)
- Test: `internal/adapter/yaml/` (a decode test — add to an existing `*_test.go` for config load, e.g. `load_test.go`, or create `dto_test.go`)

**Interfaces:**
- Consumes: `mtt.Require` (Task 1).
- Produces: `Transition.Require` populated from `transitions[].require.{who,why}` on load.

- [ ] **Step 1: Write the failing test.** Create `internal/adapter/yaml/dto_require_test.go`:

```go
package yaml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DecodesPerEdgeRequire(t *testing.T) {
	root := t.TempDir()
	mustWriteConfig(t, root, `version: 1
project: {name: demo}
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: a, kind: initial, default: true}
      - {name: b, kind: terminal}
    transitions:
      - from: a
        to: b
        name: go
        require: {who: true, why: true}
`)
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	typ := cfg.Types[0]
	edge := typ.Transitions[0]
	if !edge.Require.Who || !edge.Require.Why {
		t.Fatalf("per-edge require not decoded: %+v", edge.Require)
	}
}

// mustWriteConfig writes .mtt/config.yaml under root.
func mustWriteConfig(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, ".mtt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
```

> If a `mustWriteConfig`-equivalent already exists in the package's tests, reuse it and drop the local copy to avoid a redeclaration.

- [ ] **Step 2: Run the test — verify it fails.**

Run: `go test ./internal/adapter/yaml/ -run TestLoad_DecodesPerEdgeRequire -v`
Expected: FAIL (`edge.Require` zero — the field isn't mapped).

- [ ] **Step 3: Add the DTO field and mapping.** In `internal/adapter/yaml/dto.go`, add to `ymlTransition` (after `Current`):

```go
	Require ymlRequire `yaml:"require,omitempty"`
```

and in `toConfig` where the transition is built (line ~132), add `Require` to the `mtt.Transition{…}` literal:

```go
			Require: mtt.Require{Who: yr.Require.Who, Why: yr.Require.Why},
```

- [ ] **Step 4: Run the test — verify it passes.**

Run: `go test ./internal/adapter/yaml/ -run TestLoad_DecodesPerEdgeRequire -v`
Expected: PASS. Then `go test ./internal/adapter/yaml/` → PASS (existing config-load tests unaffected — new field is `omitempty`/zero by default).

- [ ] **Step 5: Commit.**

```bash
git add internal/adapter/yaml/dto.go internal/adapter/yaml/dto_require_test.go
git commit -m "feat(t5): decode per-edge require:{who,why} in the YAML config"
```

---

## Task 5: Wire `rm` to attribution + audit; add `.gitattributes`; e2e

**Files:**
- Modify: `internal/cli/rm.go`
- Create: `.gitattributes`
- Create: `internal/cli/testdata/scripts/dangerous.txt`

**Interfaces:**
- Consumes: `yaml.Load` → `(cfg, Settings, err)`; `resolveAttribution(cmd, settings.Author) (role, by, why, err)` (`internal/cli/status.go:180`); `yaml.NewAuditStore(root)` (Task 2); `core.NewRemover(store, audit, now)` / `RemoveMany(…, by, why)` (Task 3).
- Produces: `mtt rm … --force` requires `--who`+`--why` (exit 2 when missing, single AND bulk) and appends to `.mtt/audit.log`.

- [ ] **Step 1: Write the failing e2e test.** Create `internal/cli/testdata/scripts/dangerous.txt`:

```
# t5 — dangerous-ops attribution: --no-run and rm --force force who+why.
# Fixture config has NO global require.who, so exit-2 is caused by the new code,
# not the global policy. No MTT_BY in the env.
env MTT_DIR=$WORK/.mtt
unquote .mtt/config.yaml

exec mtt add 'first task'
exec mtt add 'second task'

# --- rm --force without --why is rejected (exit 2), task still present ---
! exec mtt rm t1 --force --who alice
stderr 'missing required attribution'
exec mtt show t1
stdout 'first task'

# --- bulk rm --force without --why is ALSO exit 2, nothing deleted (B1 guard) ---
! exec mtt rm t1 t2 --force --who alice
stderr 'missing required attribution'
exec mtt show t1
exec mtt show t2

# --- rm --force with who+why deletes AND records to audit.log ---
exec mtt rm t1 --force --who alice --why 'stale duplicate'
stdout 'removed t1'
exec grep '"id":"t1"' .mtt/audit.log
exec grep '"action":"rm --force"' .mtt/audit.log
exec grep '"why":"stale duplicate"' .mtt/audit.log

# --- a --no-run transition without --why is rejected (exit 2) ---
# Use `mtt status` (NOT the `mtt start` sugar): --no-run is local to status/do;
# the edge-verb sugar hardcodes noRun=false and would reject the flag.
! exec mtt status t2 speccing --no-run --who alice
stderr 'missing required attribution'

# --- --no-run with who+why passes ---
exec mtt status t2 speccing --no-run --who alice --why 'ci down, forcing'
stdout 't2: tbd . speccing'

-- .mtt/config.yaml --
version: 1
project: {name: dangertest}
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: tbd, kind: initial, default: true}
      - {name: speccing, kind: active}
      - {name: done, kind: terminal}
    transitions:
      - {from: tbd, to: speccing, name: start}
      - {from: speccing, to: done, name: finish}
```

> Match the existing `testdata/scripts/*.txt` conventions (this repo uses `testscript`). `unquote` + the trailing `-- file --` archive mirror how `dogfood.txt` seeds a config. Adjust the `stdout` matcher for the exact arrow rendering if needed (`.` stands in for the non-ASCII `→` in the regex).

- [ ] **Step 2: Run the e2e — verify it fails.**

Run: `go test ./internal/cli/ -run 'TestScripts/dangerous' -v`
Expected: FAIL (rm doesn't yet require who/why; no audit.log written).

- [ ] **Step 3: Wire `rm.go`.** Edit `internal/cli/rm.go`. Add imports `"time"` and keep `yaml`/`core`/`mtt`. Replace the bulk delete wiring (lines ~37) and the `runRmSingle` body.

Bulk path (inside `RunE`, replacing the `results := core.NewRemover(…).RemoveMany(ids, force)` block):

```go
			_, settings, err := yaml.Load(root)
			if err != nil {
				return err
			}
			_, by, why, err := resolveAttribution(cmd, settings.Author)
			if err != nil {
				return err
			}
			remover := core.NewRemover(yaml.NewTaskStore(root), yaml.NewAuditStore(root), time.Now)
			results, err := remover.RemoveMany(ids, force, by, why)
			if err != nil {
				return err // pre-flight ErrMissingAttribution → exit 2 (raw, not via reportBulk)
			}
			items := make([]bulkItem, 0, len(results))
			for _, r := range results {
				items = append(items, bulkItem{id: r.ID, err: r.Err})
				if r.Err == nil {
					_ = clearCurrentIfMatches(root, r.ID)
				}
			}
			return reportBulk(cmd, items, "removed")
```

`runRmSingle` — change its signature is not needed (it already has `cmd, root`); rewrite its delete section:

```go
func runRmSingle(cmd *cobra.Command, root, idArg string, force, dryRun bool) error {
	id, err := mtt.NewTaskID(idArg)
	if err != nil {
		return err
	}
	if dryRun {
		return previewBulk(cmd, []mtt.TaskID{id})
	}
	_, settings, err := yaml.Load(root)
	if err != nil {
		return err
	}
	_, by, why, err := resolveAttribution(cmd, settings.Author)
	if err != nil {
		return err
	}
	remover := core.NewRemover(yaml.NewTaskStore(root), yaml.NewAuditStore(root), time.Now)
	if err := remover.Remove(id, force, by, why); err != nil {
		return err
	}
	if err := clearCurrentIfMatches(root, id); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", id)
	return err
}
```

- [ ] **Step 4: Add `.gitattributes`.** Create `.gitattributes` at the repo root:

```
# Append-only audit log: union-merge takes both sides' lines instead of conflicting.
/.mtt/audit.log merge=union
```

- [ ] **Step 5: Run the e2e — verify it passes.**

Run: `go test ./internal/cli/ -run 'TestScripts/dangerous' -v`
Expected: PASS. Then `go test ./internal/cli/` → PASS (existing `rm.txt` still green — a single-id `rm` with an author in scope keeps working; note `rm.txt` may need `--who`/`--why` added to its `--force` line if it deletes a referenced task under force — update it if it reddens).

- [ ] **Step 6: Commit.**

```bash
git add internal/cli/rm.go .gitattributes internal/cli/testdata/scripts/dangerous.txt internal/cli/testdata/scripts/rm.txt
git commit -m "feat(t5): rm --force requires who+why + writes audit.log; --no-run e2e; .gitattributes"
```

---

## Task 6: Documentation sync

**Files:**
- Modify: `DESIGN.md`, `DESIGN.ru.md`, `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md`, `AGENTS.md`
- Modify: `internal/cli/CLAUDE.md`, `internal/core/CLAUDE.md`, `internal/adapter/yaml/CLAUDE.md`

**Interfaces:** none (docs only). No code — this task exists because the repo's Definition of Done requires docs in sync (AGENTS.md), and the three package `CLAUDE.md` files must stay current per the project rules.

- [ ] **Step 1: DESIGN.md + DESIGN.ru.md.** Add a short "Dangerous-ops attribution (t5)" subsection under the flow/gate design: the union rule (global ∨ per-edge ∨ `--no-run`), the `mtt.AuditStore` port + `.mtt/audit.log` (JSONL, committed, `merge=union`), and append-before-delete for `rm --force`. Keep EN/RU in sync (English primary).

- [ ] **Step 2: CLI_REFERENCE.md + CLI_REFERENCE.ru.md.** Document: `rm --force` now requires `--who`/`--why` (exit 2 otherwise) and records to `.mtt/audit.log`; `--no-run` requires `--who`/`--why`; per-edge `require: {who, why}` config knob. Keep EN/RU in sync.

- [ ] **Step 3: AGENTS.md.** Under "Working under mtt", add a "dangerous ops" bullet: bypassing a gate (`--no-run`) or a destructive `rm --force` forces `--who`+`--why`; `rm --force` leaves an audit record; per-edge `require:` marks critical transitions.

- [ ] **Step 4: Package CLAUDE.md files.** `internal/core/CLAUDE.md` (Transitioner union policy + `missingAttributionFields` seam; Remover pre-flight + append-before-delete + `AuditStore` dep); `internal/adapter/yaml/CLAUDE.md` (`NewAuditStore` JSONL append; per-edge `require` decode); `internal/cli/CLAUDE.md` (`rm` wires attribution + audit; pre-flight error forwarded raw → exit 2).

- [ ] **Step 5: Verify the gate is green.**

Run: `make check`
Expected: all green (gofmt, vet, lint, `go test -race -cover`, build).

- [ ] **Step 6: Commit.**

```bash
git add DESIGN.md DESIGN.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md AGENTS.md internal/cli/CLAUDE.md internal/core/CLAUDE.md internal/adapter/yaml/CLAUDE.md
git commit -m "docs(t5): dangerous-ops attribution — DESIGN/CLI_REFERENCE/AGENTS/CLAUDE sync"
```

---

## Notes for the implementer

- **Order matters:** Tasks 1→2 have no dependency between them but Task 3 needs both (the shared helper + the port); Task 5 needs Tasks 2, 3, 4. Do them in order.
- **`--no-run` needs no CLI change** — Task 1's core union already covers `mtt status --no-run` and `mtt do --no-run` (both call `Transitioner.Transition`). The sugar `mtt <status>` hardcodes `noRun=false`, so it cannot bypass; do not add `--no-run` to it.
- **Do not** add global `require.why` or touch `.mtt/config.yaml`'s existing edges — per-edge `require` is exercised by the adapter decode test, and marking a repo edge is an optional, separate call (spec §7).
- **Anti-vacuity discipline:** in every who-forcing assertion, keep `RequireWho=false` and `By=""` (Task 1 tests) and no `MTT_BY`/author in the e2e (Task 5), or the test proves nothing.
