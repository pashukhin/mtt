# t58 — Integrity upgrades Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `mtt check` the single integrity gate (dangling refs + dangling depends_on + dependency cycles, exit 7), rename the sentinel to `ErrIntegrity`, make `dep list --cycles` gate CI, and mark `(missing)` blockers in `roadmap`.

**Architecture:** One pure core entry point `CheckIntegrity` composes the existing `CheckRefs` with two new detectors (dangling-dep sweep + `DepGraph.Cycles()`); the CLI renders a `kind`-tagged union and keys exit on hard kinds. `dep`/`roadmap` are small CLI edits (no core change). Follows the approved spec ([t58-integrity-upgrades.md](../specs/t58-integrity-upgrades.md)).

**Tech Stack:** Go 1.23, cobra, testscript (txtar e2e).

## Global Constraints

- **TDD**: red → green → refactor; `make check` green before every commit; each task leaves the tree green.
- The spec is the decision record; on conflict, the spec wins. Do not relitigate the single-gate/exit-7 decision.
- Hard kinds = `dangling-ref` | `dangling-dep` | `cycle`; `unverified-ref` is soft (does not gate). Exit rule keys on **Kind**, never a nested status.
- `dangling` summary bucket = `dangling-ref` + `dangling-dep` (a dangling-dep-only repo must NOT read `0 dangling`).
- Duplicate-id is out of scope (closed by c15; a dup makes `List()` fail closed → exit 1, not 7).
- English docs are source of truth; every DESIGN/CLI_REFERENCE change lands in the RU mirror in the same task; sweep ALL parallel occurrences (`check` is described in DESIGN + CLI_REFERENCE).
- Commit trailer: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.

---

### Task 1: Core — `CheckIntegrity` + finding model + `ErrDanglingRefs → ErrIntegrity`

**Files:**
- Create: `internal/core/checkintegrity.go`
- Modify: `internal/core/backlinks.go` (rename the sentinel), `internal/cli/check.go:60` + `internal/cli/root.go:204` + `internal/cli/status_test.go` (compile-forced rename usages)
- Test: Create `internal/core/checkintegrity_test.go`

**Interfaces:**
- Produces: `IntegrityKind` (`IntegrityDanglingRef`/`IntegrityUnverifiedRef`/`IntegrityDanglingDep`/`IntegrityCycle`), `IntegrityFinding{Kind, Ref *CheckFinding, Dep *DepFinding, Cycle []mtt.TaskID}`, `DepFinding{Task, Missing mtt.TaskID}`, `func CheckIntegrity(tasks []mtt.Task, notes []mtt.Note, kbWired bool) []IntegrityFinding`, `var ErrIntegrity`. Consumed by Tasks 2–3.

- [ ] **Step 1: Write the failing test** (`checkintegrity_test.go`). Reuse the memStore-free style (CheckIntegrity is pure):

```go
package core

import (
	"reflect"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestCheckIntegrity(t *testing.T) {
	tasks := []mtt.Task{
		{ID: "t1", Type: "task", Status: "tbd", Refs: []mtt.Ref{
			{Kind: mtt.RefNote, ID: "gone"},                    // dangling-ref (no such note)
			{Kind: mtt.RefURL, ID: "https://example.com/x"},    // unverified-ref (soft)
		}},
		{ID: "t2", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t99"}}, // dangling-dep
		{ID: "t3", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t4"}},  // cycle t3<->t4
		{ID: "t4", Type: "task", Status: "tbd", DependsOn: []mtt.TaskID{"t3"}},
		{ID: "t5", Type: "task", Status: "tbd"}, // clean
	}
	got := CheckIntegrity(tasks, nil, true)

	var kinds []IntegrityKind
	for _, f := range got {
		kinds = append(kinds, f.Kind)
	}
	// deterministic order: refs (CheckRefs order), then deps (task recency), then cycles
	want := []IntegrityKind{IntegrityDanglingRef, IntegrityUnverifiedRef, IntegrityDanglingDep, IntegrityCycle}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	// payloads
	if got[0].Ref == nil || got[0].Ref.Status != RefDangling {
		t.Fatalf("dangling-ref payload = %+v", got[0].Ref)
	}
	if got[1].Ref == nil || got[1].Ref.Status != RefUnverified {
		t.Fatalf("unverified-ref payload = %+v", got[1].Ref)
	}
	if got[2].Dep == nil || got[2].Dep.Task != "t2" || got[2].Dep.Missing != "t99" {
		t.Fatalf("dangling-dep payload = %+v", got[2].Dep)
	}
	if len(got[3].Cycle) == 0 {
		t.Fatalf("cycle payload empty")
	}
}

func TestCheckIntegrityClean(t *testing.T) {
	tasks := []mtt.Task{{ID: "t1", Type: "task", Status: "tbd"}}
	if got := CheckIntegrity(tasks, nil, true); len(got) != 0 {
		t.Fatalf("clean repo = %v, want empty", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/core/ -run TestCheckIntegrity -v`
Expected: FAIL — `CheckIntegrity`/`IntegrityKind`/`DepFinding` undefined.

- [ ] **Step 3: Implement** — `internal/core/checkintegrity.go`:

```go
package core

import "github.com/pashukhin/mtt/pkg/mtt"

// IntegrityKind is the closed vocabulary of integrity finding classes. The kind
// is self-describing of hardness: dangling-ref / dangling-dep / cycle gate
// (exit 7); unverified-ref is informational.
type IntegrityKind string

const (
	IntegrityDanglingRef   IntegrityKind = "dangling-ref"
	IntegrityUnverifiedRef IntegrityKind = "unverified-ref"
	IntegrityDanglingDep   IntegrityKind = "dangling-dep"
	IntegrityCycle         IntegrityKind = "cycle"
)

// DepFinding is a dangling blocking edge: Task depends on Missing, absent from
// the task set. References by identity (both mtt.TaskID), never the task value.
type DepFinding struct {
	Task    mtt.TaskID
	Missing mtt.TaskID
}

// IntegrityFinding is one problem found by the sweep; exactly one of Ref/Dep/Cycle
// is populated per Kind (a discriminated union).
type IntegrityFinding struct {
	Kind  IntegrityKind
	Ref   *CheckFinding
	Dep   *DepFinding
	Cycle []mtt.TaskID
}

// CheckIntegrity sweeps refs, dangling depends_on, and dependency cycles from one
// task/note snapshot in a deterministic order (refs in CheckRefs order, then deps
// by the task slice order, then cycles). Pure — no store, no clock.
func CheckIntegrity(tasks []mtt.Task, notes []mtt.Note, kbWired bool) []IntegrityFinding {
	var out []IntegrityFinding
	// 1. refs (reuse the existing sweep; map RefStatus -> kind)
	for _, rf := range CheckRefs(tasks, notes, kbWired) {
		rf := rf
		kind := IntegrityDanglingRef
		if rf.Status == RefUnverified {
			kind = IntegrityUnverifiedRef
		}
		out = append(out, IntegrityFinding{Kind: kind, Ref: &rf})
	}
	// 2. dangling depends_on (an id absent from the task set)
	exists := make(map[mtt.TaskID]bool, len(tasks))
	for _, t := range tasks {
		exists[t.ID] = true
	}
	for _, t := range tasks {
		for _, dep := range t.DependsOn {
			if !exists[dep] {
				out = append(out, IntegrityFinding{Kind: IntegrityDanglingDep, Dep: &DepFinding{Task: t.ID, Missing: dep}})
			}
		}
	}
	// 3. dependency cycles (DepGraph.Cycles is deterministic + cycle-safe)
	for _, cyc := range NewDepGraph(tasks).Cycles() {
		out = append(out, IntegrityFinding{Kind: IntegrityCycle, Cycle: cyc})
	}
	return out
}
```

`internal/core/backlinks.go` — rename the sentinel:

```go
// ErrIntegrity is returned by the check command when the integrity sweep finds
// >=1 hard finding (a dangling ref, a dangling depends_on, or a dependency
// cycle). The CLI maps it to exit 7.
var ErrIntegrity = errors.New("mtt: integrity check failed")
```

Then `s/core.ErrDanglingRefs/core.ErrIntegrity/` in `cli/check.go:60`, `cli/root.go:204`; `s/ErrDanglingRefs/ErrIntegrity/` in `cli/status_test.go` (the `TestExitCode` case + any comment).

- [ ] **Step 4: Run** — `go test ./internal/core/ ./internal/cli/ -run 'TestCheckIntegrity|TestExitCode' -v` → PASS; then `go build ./...` → clean.

- [ ] **Step 5: Commit**

```bash
git add internal/core/ internal/cli/check.go internal/cli/root.go internal/cli/status_test.go
git commit -m "t58: core — CheckIntegrity (refs + dangling-dep + cycles); rename ErrDanglingRefs -> ErrIntegrity"
```

### Task 2: `mtt check` renders the four kinds + union JSON + gates on hard

**Files:**
- Modify: `internal/cli/check.go`
- Test: `internal/cli/testdata/scripts/check.txt`

**Interfaces:**
- Consumes: `CheckIntegrity`, `ErrIntegrity` (Task 1).
- Produces: `check` prints per-kind lines + the `<dangling> dangling, <unverified> unverified, <cycle> cycle across <N> entities` summary; `--json` a `kind`-tagged array; returns `ErrIntegrity` iff a hard finding exists.

- [ ] **Step 1: Write the failing e2e** — extend `check.txt`:

```
# a dangling depends_on is now an integrity failure (exit 7).
# CLI can't mint a dangling edge directly (dep add validates existence), so add
# a target, depend on it, then rm --force it to leave the edge dangling.
exec mtt add 'C'
exec mtt dep add t2 t3
exec mtt rm t3 --force --who tester --why 'leave a dangling dep'
! exec mtt check
stdout 'dangling-dep'
stdout '1 dangling'
stderr 'integrity check failed'

# --json is a kind-tagged union
exec mtt check --json
stdout '"kind": "dangling-dep"'
stdout '"missing": "t3"'
```

(Runs in a fresh `mtt init` project — the default template configures no events, so `rm --force` fires no pipeline; it needs `--who`/`--why` as a dangerous op.) The **cycle path cannot be reached through the CLI** (`dep add` rejects a cycle), so it is covered by the Task 1 core test and Task 3's `writeDepCycles` unit test; note this in the script comment rather than attempting a hand-written-yaml e2e.

- [ ] **Step 2: Run** → FAIL (check still exits 0 on a dangling dep).

- [ ] **Step 3: Implement** — rewrite `check.go`'s `RunE` body to call `CheckIntegrity` and render by kind:

```go
findings := core.CheckIntegrity(tasks, notes, true)
danglingRef, danglingDep, unverified, cycles := 0, 0, 0, 0
if jsonFlag(cmd) {
	rows := make([]integrityJSON, 0, len(findings))
	for _, f := range findings {
		rows = append(rows, toIntegrityJSON(f))
	}
	if err := writeJSON(cmd.OutOrStdout(), rows); err != nil {
		return err
	}
} else {
	var b strings.Builder
	for _, f := range findings {
		switch f.Kind {
		case core.IntegrityDanglingRef, core.IntegrityUnverifiedRef:
			fmt.Fprintf(&b, "%s:%s → %s:%s   [%s]\n", f.Ref.CarrierKind, f.Ref.CarrierID, f.Ref.Ref.Kind, f.Ref.Ref.ID, f.Kind)
		case core.IntegrityDanglingDep:
			fmt.Fprintf(&b, "task:%s → depends_on:%s   [%s]\n", f.Dep.Task, f.Dep.Missing, f.Kind)
		case core.IntegrityCycle:
			fmt.Fprintf(&b, "cycle: %s\n", joinIDs(f.Cycle, " -> "))
		}
	}
	for _, f := range findings {
		switch f.Kind {
		case core.IntegrityDanglingRef:
			danglingRef++
		case core.IntegrityDanglingDep:
			danglingDep++
		case core.IntegrityUnverifiedRef:
			unverified++
		case core.IntegrityCycle:
			cycles++
		}
	}
	fmt.Fprintf(&b, "%d dangling, %d unverified, %d cycle across %d entities\n",
		danglingRef+danglingDep, unverified, cycles, countIntegrityCarriers(findings))
	if _, err := fmt.Fprint(cmd.OutOrStdout(), b.String()); err != nil {
		return err
	}
}
if integrityHasHard(findings) {
	return core.ErrIntegrity
}
return nil
```

Add helpers in `check.go`: `integrityHasHard(fs)` (any Kind != unverified-ref), `countIntegrityCarriers(fs)` (dedup set of ref `CarrierID` + dep `Task` + each id in any cycle chain), `joinIDs`, and the `integrityJSON`/`toIntegrityJSON` union view:

```go
type integrityJSON struct {
	Kind    string   `json:"kind"`
	Carrier *struct{ Kind, ID string } `json:"carrier,omitempty"`
	Ref     *refJSON `json:"ref,omitempty"`
	Status  string   `json:"status,omitempty"`
	Task    string   `json:"task,omitempty"`
	Missing string   `json:"missing,omitempty"`
	Cycle   []string `json:"cycle,omitempty"`
}
```

`toIntegrityJSON` fills the arm matching `f.Kind` (ref arm reuses the old `carrier`/`ref`/`status` fields so `check.txt`'s existing `"status": "dangling"` assertion still passes). Delete the now-unused `refCheckJSON`/`toRefCheckJSON`/`countCarriers` (or keep `countCarriers`'s logic folded into `countIntegrityCarriers`).

- [ ] **Step 4: Run** — `go test ./internal/cli/ -run 'TestScript/check' -v` → PASS; `make check` → green.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/check.go internal/cli/testdata/scripts/check.txt
git commit -m "t58: check — render dangling-dep + cycles, kind-tagged --json, gate on hard findings"
```

### Task 3: `dep list --cycles` gates (exit 7)

**Files:**
- Modify: `internal/cli/dep.go` (`writeDepCycles`)
- Test: `internal/cli/testdata/scripts/dep.txt`

**Interfaces:**
- Consumes: `ErrIntegrity`.
- Produces: `writeDepCycles` returns `ErrIntegrity` when `len(cycles) > 0` (after printing); `no cycles` → nil (exit 0).

- [ ] **Step 1: Write the failing e2e.** A CLI-minted cycle is impossible (`dep add` rejects it), so cover this by the same leave-a-dangling technique is not a cycle. Instead assert the **exit-0 path is preserved** (acyclic `--cycles` still exits 0 with `no cycles`) AND add a `TestWriteDepCyclesGates` **unit test** in `dep_test.go` that builds a `core.DepGraph` from a cyclic task slice and asserts `writeDepCycles` returns `core.ErrIntegrity`:

```go
func TestWriteDepCyclesGates(t *testing.T) {
	g := core.NewDepGraph([]mtt.Task{
		{ID: "t1", DependsOn: []mtt.TaskID{"t2"}},
		{ID: "t2", DependsOn: []mtt.TaskID{"t1"}},
	})
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := writeDepCycles(cmd, g)
	if !errors.Is(err, core.ErrIntegrity) {
		t.Fatalf("cyclic --cycles = %v, want ErrIntegrity", err)
	}
	if !strings.Contains(out.String(), "cycle:") {
		t.Fatalf("cycle still printed? %q", out.String())
	}
	// acyclic -> nil
	g2 := core.NewDepGraph([]mtt.Task{{ID: "t1"}})
	if err := writeDepCycles(cmd, g2); err != nil {
		t.Fatalf("acyclic --cycles = %v, want nil", err)
	}
}
```

- [ ] **Step 2: Run** → FAIL (returns nil today). **Step 3: Implement** — in `writeDepCycles`, after the print block, when `len(cycles) > 0` return `core.ErrIntegrity` instead of the write's nil (both text and `--json` paths; the `--json` path returns `ErrIntegrity` when the emitted array is non-empty). Keep printing first.

- [ ] **Step 4: Run** — `go test ./internal/cli/ -run 'TestWriteDepCyclesGates|TestScript/dep' -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/dep.go internal/cli/dep_test.go
git commit -m "t58: dep list --cycles gates CI (exit 7 via ErrIntegrity) on a cycle"
```

### Task 4: `roadmap` marks `(missing)` blockers

**Files:**
- Modify: `internal/cli/roadmap.go`
- Test: `internal/cli/testdata/scripts/roadmap.txt`

**Interfaces:**
- Produces: `writeRoadmap(w, entries, tasks)` (new `tasks` param for the existence set); text appends ` (missing)` to a dangling blocker; `roadmapJSON` gains `BlockedByMissing []string \`json:"blocked_by_missing,omitempty"\``.

- [ ] **Step 1: Write the failing e2e** — extend `roadmap.txt`: a task with a dangling blocker (create t1, dep t1→t2, rm t2 --force) → `roadmap` prints `↳ blocked by: t2 (missing)`:

```
exec mtt add 'A'
exec mtt add 'B'
exec mtt dep add t1 t2
exec mtt rm t2 --force --who tester --why 'leave a dangling blocker'
exec mtt roadmap
stdout 'blocked by: t2 \(missing\)'
exec mtt roadmap --json
stdout '"blocked_by_missing"'
```

- [ ] **Step 2: Run** → FAIL (no marker). **Step 3: Implement:**
  - `newRoadmapCmd` passes `tasks` into `writeRoadmap` and `toRoadmapJSON`.
  - A shared helper `existsSet(tasks) map[mtt.TaskID]bool`, and in `writeRoadmap` build the set once; when rendering `blocked by`, map each id: `id` if present, `id + " (missing)"` if absent (a small `markMissing(ids, set) []string`).
  - `toRoadmapJSON(e, set)` fills `BlockedByMissing` with the subset of `e.BlockedBy` not in `set` (nil when none → omitempty; `BlockedBy` stays the full list for back-compat).

- [ ] **Step 4: Run** — `go test ./internal/cli/ -run 'TestScript/roadmap' -v` → PASS; `make check` → green.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/roadmap.go internal/cli/testdata/scripts/roadmap.txt
git commit -m "t58: roadmap marks a dangling blocker (missing) (text + blocked_by_missing json)"
```

### Task 5: Docs — DESIGN/CLI_REFERENCE (+RU), CLAUDE.md, CHANGELOG

**Files:**
- Modify: `DESIGN.md` + `DESIGN.ru.md`, `CLI_REFERENCE.md` + `CLI_REFERENCE.ru.md`, `internal/core/CLAUDE.md`, `internal/cli/CLAUDE.md`, `CHANGELOG.md`

- [ ] **Step 1: DESIGN.md (+ru)** — rewrite the `check`/refs narrative (`DESIGN.md:888-889,906`; RU `905,921`): `check` is an **integrity** sweep over dangling refs + dangling `depends_on` + dependency cycles, exit 7 on any hard finding (0 on clean/unverified); note `dep list --cycles` shares the exit. Mirror RU.
- [ ] **Step 2: CLI_REFERENCE (+ru)** — `mtt check`: dangling deps + cycles now covered (exit 7), the `unverified-ref` soft class, the `kind`-tagged `--json`; `dep list --cycles` gates; `roadmap` `(missing)` marker + `blocked_by_missing`; the dup-id → exit-1 (fail-closed load) note so the two failure modes aren't conflated. Rename `ErrDanglingRefs`/"dangling references" wording.
- [ ] **Step 3: CLAUDE.md** — `internal/core` (`CheckIntegrity` + `IntegrityFinding`/`Kind`/`DepFinding`; `ErrDanglingRefs`→`ErrIntegrity`), `internal/cli` (check renders 4 kinds + gates hard; `--cycles` gates; roadmap missing marker).
- [ ] **Step 4: CHANGELOG** — one `[Unreleased]` entry (integrity gate + rename + `--cycles` gate + roadmap marker).
- [ ] **Step 5:** `make check` → green. **Commit:**

```bash
git add DESIGN*.md CLI_REFERENCE*.md internal/core/CLAUDE.md internal/cli/CLAUDE.md CHANGELOG.md
git commit -m "t58: docs — integrity gate framing (DESIGN/CLI_REFERENCE EN+RU, CLAUDE.md, CHANGELOG)"
```

### Task 6: Close the loop

- [ ] `make check` green; run the AGENTS.md Principles self-check.
- [ ] `git grep -n "ErrDanglingRefs"` → no hits remain (rename complete).
- [ ] `mtt submit` (impl gate: clean tree + CHANGELOG + make check) → adversarial code review.
