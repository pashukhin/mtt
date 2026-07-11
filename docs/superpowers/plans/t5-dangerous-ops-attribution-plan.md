# Dangerous-Ops Attribution (t5) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Force `--who` + `--why` at dangerous points — bypassing a gate (`--no-run`), a critical transition (per-edge `require`), and a destructive `rm --force` — recording the destruction to an append-only `.mtt/audit.log`.

**Architecture:** Policy lives in `core` (hexagonal): the `Transitioner` unions the effective required-attribution from `{global, per-edge, --no-run}`; the `Remover` gains a pre-flight who/why precondition and writes an audit record **before** deleting. The audit log is a new driven port `mtt.AuditStore` with a JSONL YAML-adapter. No new domain storage for transitions — the reason rides the existing `HistoryEntry.Why`.

**Tech Stack:** Go 1.23, cobra CLI, `text/template` gates, `testscript` (txtar) e2e, YAML (`gopkg.in/yaml.v3`), `encoding/json` for the audit line.

## Global Constraints

- **TDD, red→green→refactor.** Write the failing test first; run it; implement minimally; run; commit.
- **`make check` green before EVERY commit** (gofmt + go vet + golangci-lint v2 + `go test -race -cover` + build over `./...`). No unused vars. **Signature changes to an exported symbol must be atomic with all their callers in one commit** — never commit a tree where `go build ./...` fails.
- **Hexagonal boundaries:** `cli → core → port ← adapter`. Policy in `core`; storage/audit only through a port; no business logic in `cli`; `pkg/mtt` holds domain types + ports.
- **`ErrMissingAttribution` → CLI exit 2** (existing `exitCode` map, `internal/cli/root.go:171-184`). Its text is `mtt: missing required attribution` (`internal/core/runner.go:38`); the missing fields are appended via `strings.Join(missing, ", ")` from ONE shared helper.
- **Anti-vacuity:** `who` is pre-satisfied in this repo (global `require.who: true` + `config.local` author). Any test asserting a path *forces who* MUST set `RequireWho=false` **and** empty `By` (unit) / no `MTT_BY` and no global `require.who` (e2e), else it passes vacuously.
- **Time:** UTC, truncated to seconds (`now().UTC().Truncate(time.Second)`), matching existing history stamps.
- **Test helpers already in package `core`** (reuse — do NOT invent new names): `newMemStore(tasks ...mtt.Task) *memStore` (`dependency_test.go:18`), `baseTask() mtt.Task` (id `t1`, status `tbd`; `transition_test.go:92`), `testClock` (`transition_test.go:90`), `fakeRunner` with **pointer** receivers → always `&fakeRunner{}` (`transition_test.go:14-29`), `flowCfg(cmdsA, cmdsB []string) mtt.Config` (type `task`; edges `tbd→in_progress` [cmdsA], `tbd→cancelled`, `in_progress→done` [cmdsB], `in_progress→cancelled`; `transition_test.go:66`).

---

## File Structure

- `pkg/mtt/config.go` — **modify**: add `type Require struct{Who,Why bool}`; add `Require` field to `Transition`.
- `pkg/mtt/audit.go` — **create**: `AuditEntry` + `AuditStore` port.
- `internal/core/transition.go` — **modify**: `missingAttribution` → shared `missingAttributionFields`; union `effWho/effWhy`.
- `internal/core/remove.go` — **modify**: `Remover` gains `audit`+`now`; new signatures; pre-flight; append-before-delete.
- `internal/adapter/yaml/audit.go` — **create**: `AuditStore` impl (JSONL append to `.mtt/audit.log`).
- `internal/adapter/yaml/dto.go` — **modify**: `ymlTransition.Require`; map it in `ymlConfig.toDomain`.
- `internal/cli/rm.go` — **modify**: load settings, resolve attribution, wire audit store, thread `by`/`why`, forward the pre-flight error raw. (Changed **atomically with** `remove.go` in Task 3.)
- `.gitattributes` — **create**: `/.mtt/audit.log merge=union`.
- `internal/cli/testdata/scripts/dangerous.txt` — **create**: e2e for `--no-run` and `rm --force`.
- `docs/architecture/model.go` — **modify**: add `AuditEntry`/`AuditStore` (Task 2); reshape `Remover`/`NewRemover` (Task 3). NOTE: this is `package architecture` — it declares its OWN port types (no `pkg/mtt` import); add architecture-local mirrors.
- Tests: `internal/core/transition_test.go`, `internal/core/remove_test.go`, `internal/adapter/yaml/audit_test.go`, `internal/adapter/yaml/dto_require_test.go`.
- Docs sync (Task 6): `DESIGN.md`/`.ru`, `CLI_REFERENCE.md`/`.ru`, `AGENTS.md`, three package `CLAUDE.md`.

---

## Task 1: Union required-attribution policy in the Transitioner

**Files:**
- Modify: `pkg/mtt/config.go` (add `Require` type + `Transition.Require` field)
- Modify: `internal/core/transition.go` (`missingAttribution` at ~115-126 → `missingAttributionFields`; the check at ~63 → union)
- Test: `internal/core/transition_test.go`

**Interfaces:**
- Produces: `mtt.Require{Who bool; Why bool}`; `mtt.Transition.Require Require`; `core.missingAttributionFields(reqWho, reqWhy bool, by, why string) []string`.
- Consumes: existing `TransitionOptions`, `ErrMissingAttribution`, `edge` (already looked up in `Transition`).

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

- [ ] **Step 2: Write the failing tests.** In `internal/core/transition_test.go`, add (uses the real helpers; ensure the file imports `errors` and `strings` — add them if absent):

```go
func TestTransition_NoRunForcesWhoAndWhy(t *testing.T) {
	store := newMemStore(baseTask())      // t1 @ tbd
	cfg := flowCfg(nil, nil)              // edge tbd→in_progress: no commands, no require
	tr := NewTransitioner(store, cfg, &fakeRunner{}, testClock)

	// (b) missing why (By present) → error mentions why
	_, err := tr.Transition("t1", "in_progress", TransitionOptions{By: "alice", NoRun: true})
	if !errors.Is(err, ErrMissingAttribution) || !strings.Contains(err.Error(), "why") {
		t.Fatalf("no-run without why: want ErrMissingAttribution mentioning why, got %v", err)
	}

	// (b′) non-vacuous who: RequireWho=false AND By="" → who forced solely by --no-run
	_, err = tr.Transition("t1", "in_progress", TransitionOptions{Why: "bypass ci", NoRun: true})
	if !errors.Is(err, ErrMissingAttribution) || !strings.Contains(err.Error(), "who") {
		t.Fatalf("no-run without who: want ErrMissingAttribution mentioning who, got %v", err)
	}

	// success: both present → moves, Why recorded
	got, err := tr.Transition("t1", "in_progress", TransitionOptions{By: "alice", Why: "bypass ci", NoRun: true})
	if err != nil {
		t.Fatalf("no-run with who+why: unexpected error %v", err)
	}
	if w := got.History[len(got.History)-1].Why; w != "bypass ci" {
		t.Fatalf("Why not recorded: got %q", w)
	}
}

func TestTransition_PerEdgeRequireUnionsWithGlobal(t *testing.T) {
	store := newMemStore(baseTask())
	cfg := flowCfg(nil, nil)
	cfg.Types[0].Transitions[0].Require = mtt.Require{Why: true} // tbd→in_progress requires why
	tr := NewTransitioner(store, cfg, &fakeRunner{}, testClock)

	// global who + edge why → both required; give only who → missing why
	_, err := tr.Transition("t1", "in_progress", TransitionOptions{By: "alice", RequireWho: true})
	if !errors.Is(err, ErrMissingAttribution) || !strings.Contains(err.Error(), "why") {
		t.Fatalf("union: want missing why, got %v", err)
	}
}
```

- [ ] **Step 3: Run the tests — verify they fail.**

Run: `go test ./internal/core/ -run 'TestTransition_(NoRunForces|PerEdgeRequire)' -v`
Expected: FAIL — the moves currently **succeed** (no-run bypasses attribution today; per-edge `require` is ignored), so the assertions fail with "unexpected error <nil>" / "want ErrMissingAttribution … got <nil>".

- [ ] **Step 4: Refactor the helper and add the union.** In `internal/core/transition.go`, replace `missingAttribution` (lines ~115-126) with the field-level helper:

```go
// missingAttributionFields reports which required attribution fields (who/why) are
// absent, aggregated so the caller fixes them all at once. The single home for the
// who/why check, shared by the transition path and the rm --force pre-flight.
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

Then in `Transition`, replace the check at line ~63 (`if missing := missingAttribution(opts); …`) with the union, using the already-found `edge`:

```go
	effWho := opts.RequireWho || edge.Require.Who || opts.NoRun
	effWhy := opts.RequireWhy || edge.Require.Why || opts.NoRun
	if missing := missingAttributionFields(effWho, effWhy, opts.By, opts.Why); len(missing) > 0 {
		return mtt.Task{}, fmt.Errorf("%w: %s", ErrMissingAttribution, strings.Join(missing, ", "))
	}
```

- [ ] **Step 5: Run the tests — verify they pass.**

Run: `go test ./internal/core/ -run 'TestTransition' -v`
Expected: PASS (new + all existing transition tests — the helper is behavior-equivalent when `NoRun=false` and `edge.Require` is zero).

- [ ] **Step 6: `make check`, then commit.**

```bash
make check
git add pkg/mtt/config.go internal/core/transition.go internal/core/transition_test.go
git commit -m "feat(t5): union required-attribution (global + per-edge + --no-run) in Transitioner"
```

---

## Task 2: AuditStore port + JSONL adapter

**Files:**
- Create: `pkg/mtt/audit.go`
- Create: `internal/adapter/yaml/audit.go`
- Modify: `docs/architecture/model.go` (add architecture-local `AuditEntry`/`AuditStore` near the other ports, ~line 631 by `Runner`)
- Test: `internal/adapter/yaml/audit_test.go`

**Interfaces:**
- Produces: `mtt.AuditEntry{At time.Time; Who, Why, Action string; TaskID TaskID}`; `mtt.AuditStore` (`Append(AuditEntry) error`); `yaml.NewAuditStore(root string) *AuditStore` (satisfies `mtt.AuditStore`).

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
	"strings"
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
	var lines []string
	for _, l := range strings.Split(string(raw), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), raw)
	}
	var got struct{ At, Who, Why, Action, ID string }
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
```

- [ ] **Step 3: Run the test — verify it fails.**

Run: `go test ./internal/adapter/yaml/ -run TestAuditStore -v`
Expected: FAIL (`undefined: NewAuditStore`).

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
	b, err := json.Marshal(auditLine{
		At:     e.At.UTC().Format(time.RFC3339),
		Who:    e.Who,
		Why:    e.Why,
		Action: e.Action,
		ID:     string(e.TaskID),
	})
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

- [ ] **Step 5: Mirror the port in the architecture reference.** In `docs/architecture/model.go`, add near the `Runner` port (this file is `package architecture` and defines its own types — do NOT import `pkg/mtt`):

```go
// AuditEntry records one out-of-flow dangerous action (a --force destruction with
// no task history to carry its attribution). [t5]
type AuditEntry struct {
	At     time.Time
	Who    string
	Why    string
	Action string
	TaskID TaskID
}

// AuditStore appends dangerous-action records; append-only (no read surface). [t5]
type AuditStore interface {
	Append(AuditEntry) error
}
```

- [ ] **Step 6: Run the tests — verify they pass.**

Run: `go test ./internal/adapter/yaml/ -run TestAuditStore -v`
Expected: PASS (both).

- [ ] **Step 7: `make check`, then commit.**

```bash
make check
git add pkg/mtt/audit.go internal/adapter/yaml/audit.go internal/adapter/yaml/audit_test.go docs/architecture/model.go
git commit -m "feat(t5): AuditStore port + JSONL .mtt/audit.log adapter"
```

---

## Task 3: Remover force-policy + audit (core + CLI + model, ATOMIC)

> This task changes the exported `NewRemover`/`Remove`/`RemoveMany` signatures. Their ONLY callers are `internal/cli/rm.go:37` and `:64` (verified repo-wide) plus the tests and `model.go`. All are fixed **in this one commit** so `go build ./...` never breaks (Global Constraints).

**Files:**
- Modify: `internal/core/remove.go`
- Modify: `internal/cli/rm.go`
- Modify: `docs/architecture/model.go` (`Remover` interface + `NewRemover` var, ~598-611)
- Test: `internal/core/remove_test.go`

**Interfaces:**
- Consumes: `mtt.AuditStore` (Task 2), `missingAttributionFields` (Task 1), `resolveAttribution(cmd, author) (role, by, why string, err error)` (`internal/cli/status.go:180`), `yaml.Load(root) (mtt.Config, Settings, error)`, `yaml.NewAuditStore(root)`.
- Produces: `NewRemover(store mtt.TaskStore, audit mtt.AuditStore, now func() time.Time) *Remover`; `RemoveMany(ids []mtt.TaskID, force bool, by, why string) ([]RemoveResult, error)`; `Remove(id mtt.TaskID, force bool, by, why string) error`.

- [ ] **Step 1: Write the failing tests.** In `internal/core/remove_test.go`, add the fake audit store, a `deleted` probe on `memStore`, and the new cases:

```go
// deleted reports whether id is absent from the store (memStore.Delete removes it).
func (m *memStore) deleted(id mtt.TaskID) bool { _, ok := m.byID[id]; return !ok }

type fakeAudit struct {
	entries  []mtt.AuditEntry
	failOnID mtt.TaskID // if set, Append errors for that id
}

func (f *fakeAudit) Append(e mtt.AuditEntry) error {
	if e.TaskID == f.failOnID {
		return fmt.Errorf("disk full")
	}
	f.entries = append(f.entries, e)
	return nil
}

func tbdTask(id mtt.TaskID) mtt.Task {
	return mtt.Task{ID: id, Type: "task", Status: "tbd", Created: testClock(), Updated: testClock()}
}

func TestRemoveMany_ForceRequiresWhoAndWhy(t *testing.T) {
	store := newMemStore(tbdTask("t1"))
	audit := &fakeAudit{}
	res, err := NewRemover(store, audit, testClock).RemoveMany([]mtt.TaskID{"t1"}, true, "", "")
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
	store := newMemStore(tbdTask("t1"))
	audit := &fakeAudit{}
	res, err := NewRemover(store, audit, testClock).RemoveMany([]mtt.TaskID{"t1"}, true, "alice", "cleanup")
	if err != nil {
		t.Fatalf("pre-flight error: %v", err)
	}
	if res[0].Err != nil {
		t.Fatalf("delete error: %v", res[0].Err)
	}
	if len(audit.entries) != 1 || audit.entries[0].TaskID != "t1" ||
		audit.entries[0].Who != "alice" || audit.entries[0].Why != "cleanup" || audit.entries[0].Action != "rm --force" {
		t.Fatalf("audit entry wrong: %+v", audit.entries)
	}
	if !store.deleted("t1") {
		t.Fatal("task should be deleted after successful append")
	}
}

func TestRemoveMany_AppendFailureSkipsDelete(t *testing.T) {
	store := newMemStore(tbdTask("t1"), tbdTask("t2"))
	audit := &fakeAudit{failOnID: "t1"}
	res, err := NewRemover(store, audit, testClock).RemoveMany([]mtt.TaskID{"t1", "t2"}, true, "alice", "cleanup")
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
	store := newMemStore(tbdTask("t1"))
	audit := &fakeAudit{}
	res, err := NewRemover(store, audit, testClock).RemoveMany([]mtt.TaskID{"t1"}, false, "", "")
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

> Ensure `remove_test.go` imports `errors` and `fmt` (add if absent). If a `tbdTask`-like builder already exists, reuse it and drop the local one.

- [ ] **Step 2: Update the existing `NewRemover`/`Remove`/`RemoveMany` call sites in `remove_test.go`.** Run `grep -nE 'NewRemover|\.Remove\(|RemoveMany\(' internal/core/remove_test.go` and mechanically update each: `NewRemover(store)` → `NewRemover(store, &fakeAudit{}, testClock)`; `.Remove(id, force)` → `.Remove(id, force, "who", "why")` (or `"",""` where a non-force path is under test); `res := ….RemoveMany(ids, force)` → `res, _ := ….RemoveMany(ids, force, "who", "why")` (discard the pre-flight error where it isn't the subject; assert it where it is).

- [ ] **Step 3: Run the tests — verify they fail to COMPILE (expected at this stage).**

Run: `go test ./internal/core/ -run TestRemoveMany -v`
Expected: FAIL — compile error ("not enough arguments in call to NewRemover" / "assignment mismatch: 2 variables but RemoveMany returns 1"). This is the intended red: the signatures don't exist yet.

- [ ] **Step 4: Rewrite `remove.go`.** In `internal/core/remove.go`: add `"time"` to imports; **keep `RemoveResult`, `externalReferencingIDs`, and `dedupIDSlice` exactly as they are** (only the `Remover` struct, `NewRemover`, `Remove`, `RemoveMany`, `removeOne` change). Replace those five:

```go
// Remover is the delete-a-task usecase. By default it refuses to delete a task
// referenced by others; --force overrides. Under --force it FORCES who+why
// (pre-flight) and writes an audit record BEFORE deleting (no destruction without a
// preceding record). now is injected for deterministic audit timestamps.
type Remover struct {
	store mtt.TaskStore
	audit mtt.AuditStore
	now   func() time.Time
}

// NewRemover wires the usecase with the audit port and an injected clock.
func NewRemover(store mtt.TaskStore, audit mtt.AuditStore, now func() time.Time) *Remover {
	return &Remover{store: store, audit: audit, now: now}
}

// Remove deletes a single id. Thin wrapper over RemoveMany([id]); forwards the
// pre-flight error and, absent that, the per-id result error. The empty-slice check
// guards the [0] index on the pre-flight path.
func (r *Remover) Remove(id mtt.TaskID, force bool, by, why string) error {
	res, err := r.RemoveMany([]mtt.TaskID{id}, force, by, why)
	if err != nil {
		return err
	}
	return res[0].Err
}

// RemoveMany deletes each id best-effort. The error return is the PRE-FLIGHT
// precondition failure (missing attribution under --force), returned before any
// deletion with a nil results slice; the CLI forwards it raw (exit 2). Per-id
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

// removeOne deletes one id. Under --force it appends the audit record FIRST; only on
// a successful append does it delete (a failed append leaves the task — and the
// current pointer — intact). Without --force the subgraph referenced-check runs and
// no audit is written.
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
	entry := mtt.AuditEntry{At: r.now().UTC().Truncate(time.Second), Who: by, Why: why, Action: "rm --force", TaskID: id}
	if err := r.audit.Append(entry); err != nil {
		return fmt.Errorf("audit append for %q: %w", id, err)
	}
	return r.store.Delete(id)
}
```

- [ ] **Step 5: Wire `internal/cli/rm.go`.** Add `"time"` to imports. In the `RunE` bulk path, replace the `results := core.NewRemover(yaml.NewTaskStore(root)).RemoveMany(ids, force)` block:

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

and rewrite `runRmSingle`'s delete section:

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

- [ ] **Step 6: Reshape the architecture reference.** In `docs/architecture/model.go` (~598-611):

```go
type Remover interface {
	Remove(id TaskID, force bool, by, why string) error
	RemoveMany(ids []TaskID, force bool, by, why string) ([]RemoveResult, error)
}
```

```go
// NewRemover wires the delete usecase — TaskStore + AuditStore + injected clock
// (audit records --force deletes; who/why forced pre-flight). [s008.5; t5]
var NewRemover func(store TaskStore, audit AuditStore, now func() time.Time) Remover
```

- [ ] **Step 7: Run the tests — verify they pass.**

Run: `go test ./internal/core/ ./internal/cli/ -run 'TestRemove|TestScripts/rm' -v`
Expected: PASS. If `internal/cli/testdata/scripts/rm.txt` deletes a *referenced* task under `--force` without attribution, it now exits 2 — add `--who x --why y` to that line in `rm.txt` (it inherits the persistent flags).

- [ ] **Step 8: `make check`, then commit (atomic).**

```bash
make check
git add internal/core/remove.go internal/core/remove_test.go internal/cli/rm.go docs/architecture/model.go internal/cli/testdata/scripts/rm.txt
git commit -m "feat(t5): rm --force forces who+why (pre-flight, exit 2) + append-before-delete audit"
```

---

## Task 4: Decode per-edge `require` in the YAML config

**Files:**
- Modify: `internal/adapter/yaml/dto.go` (`ymlTransition` field + `ymlConfig.toDomain` transition literal at ~line 132)
- Test: `internal/adapter/yaml/dto_require_test.go`

**Interfaces:**
- Consumes: `mtt.Require` (Task 1). Produces: `Transition.Require` populated from `transitions[].require.{who,why}` on load.

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
	dir := filepath.Join(root, ".mtt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `version: 1
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
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	edge := cfg.Types[0].Transitions[0]
	if !edge.Require.Who || !edge.Require.Why {
		t.Fatalf("per-edge require not decoded: %+v", edge.Require)
	}
}
```

- [ ] **Step 2: Run the test — verify it fails.**

Run: `go test ./internal/adapter/yaml/ -run TestLoad_DecodesPerEdgeRequire -v`
Expected: FAIL (`edge.Require` zero — the field isn't decoded/mapped).

- [ ] **Step 3: Add the DTO field and mapping.** In `internal/adapter/yaml/dto.go`, add to `ymlTransition` (after `Current`):

```go
	Require ymlRequire `yaml:"require,omitempty"`
```

and in `ymlConfig.toDomain` (the `mtt.Transition{…}` literal at line ~132), add:

```go
			Require: mtt.Require{Who: yr.Require.Who, Why: yr.Require.Why},
```

- [ ] **Step 4: Run the test — verify it passes.**

Run: `go test ./internal/adapter/yaml/ -run TestLoad_DecodesPerEdgeRequire -v`
Expected: PASS. Then `go test ./internal/adapter/yaml/` → PASS (existing config-load tests unaffected — zero-valued for edges without `require`).

- [ ] **Step 5: `make check`, then commit.**

```bash
make check
git add internal/adapter/yaml/dto.go internal/adapter/yaml/dto_require_test.go
git commit -m "feat(t5): decode per-edge require:{who,why} in the YAML config"
```

---

## Task 5: e2e for `--no-run` and `rm --force`; `.gitattributes`

**Files:**
- Create: `internal/cli/testdata/scripts/dangerous.txt`
- Create: `.gitattributes`

**Interfaces:** none new — exercises the wired behavior end-to-end.

- [ ] **Step 1: Write the failing e2e.** Create `internal/cli/testdata/scripts/dangerous.txt` (idiom mirrors `dogfood.txt`: `mtt init` + `cp` a fixture; cwd is `$WORK`; the fixture config has NO global `require.who` and NO commands on edges, so no git is needed and exit-2 is caused by the new code, not the global policy; the env has no `MTT_BY`):

```
# t5 — dangerous-ops attribution: --no-run and rm --force force who+why.

exec mtt init
cp danger.yaml .mtt/config.yaml
exec mtt types
stdout 'task'

exec mtt add 'first task'
stdout 'created t1'
exec mtt add 'second task'
stdout 'created t2'

# --- rm --force without --why is rejected (exit 2); task still present ---
! exec mtt rm t1 --force --who alice
stderr 'missing required attribution'
exec mtt show t1
stdout 'first task'

# --- bulk rm --force without --why is ALSO exit 2; nothing deleted (B1 guard) ---
! exec mtt rm t1 t2 --force --who alice
stderr 'missing required attribution'
exec mtt show t1
exec mtt show t2

# --- rm --force with who+why deletes AND records to audit.log ---
exec mtt rm t1 --force --who alice --why 'stale duplicate'
stdout 'removed t1'
grep '"id":"t1"' .mtt/audit.log
grep '"action":"rm --force"' .mtt/audit.log
grep '"why":"stale duplicate"' .mtt/audit.log

# --- a --no-run transition without --why is rejected (exit 2) ---
# Use `mtt status` (NOT the `mtt start` sugar): --no-run is local to status/do;
# the edge-verb sugar hardcodes noRun=false and would reject the flag.
! exec mtt status t2 speccing --no-run --who alice
stderr 'missing required attribution'

# --- --no-run with who+why passes ---
exec mtt status t2 speccing --no-run --who alice --why 'ci down, forcing'
stdout 't2: tbd . speccing'

-- danger.yaml --
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

> `grep` is the testscript builtin (regexp match against a file); `stdout 't2: tbd . speccing'` uses `.` to match the `→` rune (output is `t2: tbd → speccing`, `status.go:114`). `encoding/json` emits spaceless `"id":"t1"`, so the greps match.

- [ ] **Step 2: Run the e2e — verify it fails.**

Run: `go test ./internal/cli/ -run 'TestScripts/dangerous' -v`
Expected: PASS already if Tasks 1+3 are done (the behavior is wired). To see it as a genuine red, run this task's file BEFORE Task 3 lands; in normal order it is a regression lock. If it FAILS, read the runner output — most likely a fixture-seeding mismatch (adjust `cp`/archive to match `dogfood.txt` exactly).

> NOTE: because the behavior is implemented in Tasks 1+3, this e2e is a **characterization/lock** test, not a classic red→green. That is acceptable for an integration guard. If executing strictly TDD, author `dangerous.txt` immediately after Task 1 (the `--no-run` half) and extend it in Task 3 (the `rm` half); this plan keeps it whole here for cohesion.

- [ ] **Step 3: Add `.gitattributes`.** Create `.gitattributes` at the repo root:

```
# Append-only audit log: union-merge takes both sides' lines instead of conflicting.
/.mtt/audit.log merge=union
```

- [ ] **Step 4: Run the e2e + full CLI suite — verify green.**

Run: `go test ./internal/cli/`
Expected: PASS (dangerous.txt + existing scripts).

- [ ] **Step 5: `make check`, then commit.**

```bash
make check
git add internal/cli/testdata/scripts/dangerous.txt .gitattributes
git commit -m "test(t5): e2e for --no-run + rm --force attribution; .gitattributes merge=union"
```

---

## Task 6: Documentation sync

**Files:** `DESIGN.md`, `DESIGN.ru.md`, `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md`, `AGENTS.md`, `internal/cli/CLAUDE.md`, `internal/core/CLAUDE.md`, `internal/adapter/yaml/CLAUDE.md`.

**Interfaces:** none (docs). Required by the Definition of Done (AGENTS.md) + the per-package `CLAUDE.md` rule.

- [ ] **Step 1: DESIGN.md + DESIGN.ru.md.** Add a "Dangerous-ops attribution (t5)" subsection under the flow/gate design: the union rule (global ∨ per-edge ∨ `--no-run`), the `mtt.AuditStore` port + `.mtt/audit.log` (JSONL, committed, `merge=union`), append-before-delete for `rm --force`. EN primary, keep RU in sync.

- [ ] **Step 2: CLI_REFERENCE.md + CLI_REFERENCE.ru.md.** Document: `rm --force` requires `--who`/`--why` (exit 2 otherwise) and records to `.mtt/audit.log`; `--no-run` requires `--who`/`--why`; the per-edge `require: {who, why}` config knob. Keep EN/RU in sync.

- [ ] **Step 3: AGENTS.md.** Under "Working under mtt", add a "dangerous ops" bullet: `--no-run` and `rm --force` force `--who`+`--why`; `rm --force` leaves an audit record; per-edge `require:` marks critical transitions.

- [ ] **Step 4: Package `CLAUDE.md`.** `internal/core/CLAUDE.md` (Transitioner union + `missingAttributionFields` seam; Remover pre-flight + append-before-delete + `AuditStore` dep); `internal/adapter/yaml/CLAUDE.md` (`NewAuditStore` JSONL append; per-edge `require` decode); `internal/cli/CLAUDE.md` (`rm` wires attribution + audit; pre-flight error forwarded raw → exit 2).

- [ ] **Step 5: `make check`.**

Run: `make check`
Expected: all green.

- [ ] **Step 6: Commit.**

```bash
git add DESIGN.md DESIGN.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md AGENTS.md internal/cli/CLAUDE.md internal/core/CLAUDE.md internal/adapter/yaml/CLAUDE.md
git commit -m "docs(t5): dangerous-ops attribution — DESIGN/CLI_REFERENCE/AGENTS/CLAUDE sync"
```

---

## Notes for the implementer

- **Order & atomicity:** Task 1 and Task 2 are independent (Task 1 touches transition; Task 2 adds the port). Task 3 needs both and is **atomic** (core + cli + model in one commit) so the signature churn never breaks `go build ./...`. Task 4 is independent of 1-3. Task 5 needs 1+3. Task 6 last.
- **`--no-run` needs no CLI change** — Task 1's core union covers `mtt status --no-run` and `mtt do --no-run` (both call `Transitioner.Transition`). The sugar `mtt <status>` hardcodes `noRun=false` and cannot bypass; do NOT add `--no-run` to it (and do NOT test `--no-run` via `mtt start`).
- **Reuse the real test helpers** (`newMemStore`, `baseTask`, `testClock`, `&fakeRunner{}`, `flowCfg`) — do not invent `newFakeStore`/`fixedNow`/`cfgWithEdge`. `memStore` lives in `dependency_test.go`; adding a `deleted` method there or in `remove_test.go` is fine (same package).
- **Anti-vacuity discipline:** keep `RequireWho=false` + `By=""` in who-forcing unit assertions (Task 1) and no `MTT_BY`/no global `require.who` in the e2e (Task 5), or the test proves nothing.
- **Do not** add global `require.why` or edit `.mtt/config.yaml`'s existing edges — per-edge `require` is covered by the adapter decode test; marking a repo edge is optional/separate (spec §7).
