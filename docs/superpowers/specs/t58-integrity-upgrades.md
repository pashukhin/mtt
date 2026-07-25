# t58 — Integrity upgrades (spec)

Status: revision 2 — addresses the 2026-07-25 adversarial spec review (2 majors, 3 minors).
Decided in the 2026-07-25 brainstorm (audit findings + 1 scoping decision with the user: single
integrity gate under exit 7).

## Problem

The 2026-07-22 integrity audit found that mtt's health checks are incomplete and inconsistent —
a broken repo can pass silently or fail to gate CI:

1. **`mtt check` inspects `refs` only.** A dangling `depends_on` — the *one* edge type that blocks
   work — passes `check` silently, even though `dep list` / `show` / `roadmap` all surface it. The
   integrity gate misses the most consequential dangling link.
2. **`roadmap` marks a nonexistent blocker without `(missing)`.** It prints `↳ blocked by: t99`
   as a bare id, while `dep list` (`dep.go:280`) and `show` (`show.go:91`) both render `(missing)`
   for a dangling blocker — an inconsistency that hides a broken dependency in the primary
   "what next?" view.
3. **`dep list --cycles` exits 0 even with a cycle.** It prints the cycle but returns nil, so —
   unlike `check` (exit 7) — it cannot gate CI. A hand-edited dependency cycle ships green.

A fourth audit finding (a duplicated id across task files — a hand-copied YAML keeping the other's
inner id) is **already closed by c15**: the load path refuses a file whose in-file `id:` ≠ its
filename stem (`internal/adapter/yaml/task.go:34`, pinned by `task_security_test.go:34-53`), and
filenames are unique on disk, so two files cannot share an id. It is out of scope here (§Non-goals).

Root cause: `check` was built for `refs` (t1) and never grew to cover the blocking-edge graph;
`--cycles` and `roadmap` each evolved their own exit/marker conventions. This task makes integrity
**one gate with one exit code and consistent markers**.

## Goals

1. **`check` becomes the single integrity gate** — one core entry point sweeping three finding
   classes: dangling `refs` (existing), dangling `depends_on` (new), and dependency `cycles` (new).
   Any finding of a *hard* class → exit 7.
2. **Rename the sentinel** `core.ErrDanglingRefs` → `core.ErrIntegrity` (exit 7 unchanged) — the
   code now covers more than refs; the name should not lie.
3. **`dep list --cycles` gates** — returns `ErrIntegrity` (exit 7) when cycles exist, for scripting
   parity with `check`; its printed output is unchanged.
4. **`roadmap` marks dangling blockers `(missing)`** — text and JSON — consistent with `dep`/`show`.
5. Docs + tests: CLI_REFERENCE (EN+RU), CLAUDE.md (core + cli), CHANGELOG; unit + e2e coverage.

## Non-goals

- **Duplicate-id detection** — closed by c15's load guard (above); no new work, only a spec note.
- **New exit codes per finding class** — rejected in the brainstorm; one integrity code (7) is what
  a CI gate wants ("integrity failed"), not a per-class taxonomy.
- **Auto-repair** — `check` reports; it never mutates. Fixing a dangling edge is the user's job
  (`dep rm`, `edit`, `rm --force`).
- **Config-drift / unknown-status detection** — `check` stays about the reference + dependency
  graph; status-vocabulary drift is a separate concern (surfaced already by the move path, c14).
- **Folding `--cycles` into `check`'s output** — `dep list --cycles` stays a *focused view*
  (dep-scoped, with the `dep list` ergonomics); it shares the exit semantics, not the rendering.

## Decisions (brainstorm record, 2026-07-25)

1. **Single integrity gate, exit 7** (user decision): `check` sweeps refs + dangling depends_on +
   cycles, all under one exit code; the sentinel is renamed edge-neutral. Alternatives rejected:
   `check += depends_on` but cycles stay separate (CI must call two commands); distinct codes per
   class (taxonomy bloat).
2. **Cycle detection reuses `DepGraph.Cycles()`** (already shipped, defensive) — no new algorithm.
3. **The `(missing)` marker is a CLI concern** — `roadmap` re-derives existence from the task set it
   already lists (DRY with `show`/`dep`, which mark CLI-side); `core.Roadmap` is untouched.

## Design

### 1. Core — one integrity entry point (`internal/core`)

A new finding model + one composing function. The existing ref sweep (`CheckRefs`,
`CheckFinding`) is **reused unchanged**; the new function composes it with two more detectors.

```go
// IntegrityKind is the closed vocabulary of integrity finding classes. The kind
// is SELF-DESCRIBING of hardness — a consumer switching on Kind knows whether a
// finding gates (dangling-ref / dangling-dep / cycle) or is informational
// (unverified-ref), without inspecting a nested status. (Review r1 F1: a single
// "dangling-ref" kind would mislabel an unverified url/no-KB ref as dangling.)
type IntegrityKind string
const (
	IntegrityDanglingRef   IntegrityKind = "dangling-ref"    // a refs entry to a MISSING target (HARD)
	IntegrityUnverifiedRef IntegrityKind = "unverified-ref"  // a url / no-KB ref (SOFT, informational)
	IntegrityDanglingDep   IntegrityKind = "dangling-dep"    // a depends_on id not in the task set (HARD)
	IntegrityCycle         IntegrityKind = "cycle"           // a depends_on cycle (HARD)
)

// IntegrityFinding is one problem the sweep found. Exactly one of Ref/Dep/Cycle
// is populated per Kind (a discriminated union, DDD value object).
type IntegrityFinding struct {
	Kind  IntegrityKind
	Ref   *CheckFinding // dangling-ref | unverified-ref: wraps the existing ref finding (carries CarrierKind/CarrierID/Ref/Status)
	Dep   *DepFinding   // dangling-dep
	Cycle []mtt.TaskID  // cycle: the id chain
}

// DepFinding is a dangling blocking edge: task Task depends on Missing, which is
// absent from the task set. References by IDENTITY (DDD) — both mtt.TaskID, never
// the full task value.
type DepFinding struct {
	Task    mtt.TaskID
	Missing mtt.TaskID
}

// CheckIntegrity sweeps refs, dangling depends_on, and dependency cycles from a
// single task/note snapshot, in a deterministic order (refs, then deps, then
// cycles; each internally ordered). kbWired controls note verifiability (as
// today). Pure — no store, no clock (mirrors CheckRefs / Roadmap).
func CheckIntegrity(tasks []mtt.Task, notes []mtt.Note, kbWired bool) []IntegrityFinding
```

- **Ref** findings wrap the existing `CheckRefs` output; the finding's `Kind` is derived from the
  ref's `Status` — `RefDangling → dangling-ref` (hard), `RefUnverified → unverified-ref` (soft). The
  embedded `CheckFinding` keeps its `Status` too, but the top-level `Kind` is the single source of
  hardness (no consumer has to read a nested status to know whether it gates).
- **Dangling-dep** findings: for each task, each `depends_on` id absent from the task-id set →
  one `IntegrityFinding{Kind: dangling-dep, Dep: {Task, Missing}}`. Deterministic (task order by
  the shared recency comparator, then dep order as stored). A new tiny `DepFinding` value type.
- **Cycle** findings: `NewDepGraph(tasks).Cycles()` → one finding per cycle (the id chain as
  returned; `Cycles()` is already deterministic + cycle-safe).
- `ErrDanglingRefs` → **`ErrIntegrity`** (`var ErrIntegrity = errors.New("mtt: integrity check failed")`).
  Blast radius (5 sites): `backlinks.go` (def), `cli/check.go`, `cli/root.go` (exitCode),
  `cli/status_test.go` (TestExitCode), CLI_REFERENCE (EN+RU).

### 2. `mtt check` (`internal/cli/check.go`)

`check` calls `CheckIntegrity` and renders all four kinds; **exit 7** iff any *hard-kind* finding
exists. **Hard kinds = `dangling-ref` | `dangling-dep` | `cycle`**; `unverified-ref` (url / no-KB)
stays non-failing, exactly as today — the exit rule keys on `Kind`, not a nested status. Human
output extends the current lines:

```
task:t2 → note:missing-note   [dangling-ref]
task:t7 → depends_on:t99      [dangling-dep]
cycle: t3 -> t4 -> t3
3 dangling, 1 unverified, 1 cycle across N entities
```

**Summary buckets** (review r2 F1 — the summary must not zero-out a dangling-dep-only failure): the
`dangling` count folds **both** hard dangling-link kinds (`dangling-ref` + `dangling-dep`), so a
repo whose only fault is `t7 depends_on:t99` reads `1 dangling, 0 unverified, 0 cycle` (exit 7,
not a zeroed line). Full form: `<dangling> dangling, <unverified> unverified, <cycle> cycle across
<N> entities`. The golden e2e asserts this exact string for the dangling-dep-only case.

`N entities` is the count of **distinct carriers/subjects** across all findings: a ref carrier
(its `CarrierID`, as `countCarriers` does today), a dep finding's `Task`, and each id appearing in
any `cycle` chain — deduplicated into one set. (Stated so the golden e2e is reproducible.)

**`--json` is now a heterogeneous, `kind`-tagged union** — a consumer must **switch on `kind`**, not
assume every element has `carrier`. The `dangling-ref`/`unverified-ref` arms keep the existing
`carrier`/`ref`/`status` fields and add `kind`; the `dangling-dep` arm is `{kind, task, missing}`;
the `cycle` arm is `{kind, cycle: [...]}`. (Additive for the ref arm — a tolerant consumer reading
named fields still works; a clean repo still emits `[]`.) The `check.txt` e2e's `"status":
"dangling"` assertion still holds (the ref arm is unchanged bar the added `kind`).

### 3. `dep list --cycles` (`internal/cli/dep.go`)

`writeDepCycles` returns **`core.ErrIntegrity`** (exit 7) when `len(cycles) > 0`, after printing the
cycles (so the report is still shown). `no cycles` → exit 0 (unchanged). This is the only behavior
change to `dep`; `--json --cycles` also exits 7 when the emitted array is non-empty.

### 4. `roadmap` missing marker (`internal/cli/roadmap.go`)

The `↳ blocked by:` line marks a blocker absent from the listed task set as `(missing)`:

```
  ↳ blocked by: t4, t99 (missing)
```

The CLI builds an existence set from the **`tasks`** slice it already lists (not the filtered
`entries` — `tasks` is the full snapshot, robust and DRY with `show.go`'s `dependsEntries`) and
appends ` (missing)` to any `BlockedBy` id not in it. Because `BlockedBy` holds only non-terminal
*or* dangling ids (`roadmap.go`), and a confirmed-terminal task is never in it, no existing-task id
is ever wrongly marked. JSON: the
roadmap entry's `blocked_by` stays a string array (back-compat), and a sibling
`blocked_by_missing` array (omitempty, non-null when present) lists the dangling subset — so a
machine consumer sees the distinction without breaking the existing field. `core.Roadmap` is
untouched (the marker is a pure CLI read, DRY with `show`/`dep`).

## Testing (TDD)

- **core** (`checkintegrity_test.go`): a fixture with a dangling ref, an **unverified** ref (a
  `url:`), a dangling depends_on, a clean task, and a 2-cycle → assert the finding set (kinds
  incl. the `dangling-ref` vs `unverified-ref` split, payloads, deterministic order); a clean repo
  → empty; unverified-ref-only → one `unverified-ref` finding (soft — the CLI exit rule keys on
  kind). Reuse the existing `CheckRefs` fixtures where possible.
- **cli** e2e (`integrity.txt` or extend `check.txt`): a repo with a dangling `depends_on` → `check`
  exits non-zero + prints `[dangling-dep]`; a cycle → `check` non-zero + `cycle:` line;
  `dep list --cycles` on the same → non-zero (was 0); a clean repo → `check` exit 0. `roadmap.txt`:
  a task blocked by a nonexistent id → `blocked by: … (missing)`.
- **exit code**: extend `TestExitCode` — `core.ErrIntegrity` → 7 (renamed from `ErrDanglingRefs`).
- A note (or reuse) that the duplicate-id guard is `task_security_test.go` (c15) — no new test, just
  the spec cross-reference.

## Docs

Sweep ALL parallel occurrences (EN+RU) per the design-docs-parallel-occurrences lesson — `check` is
described in three places, not one.

- **DESIGN.md + DESIGN.ru.md** (review r1 F2 — the real omission): the `check`/refs narrative
  currently says `check` is a sweep for **dangling references** that "exits 7 on any dangling ref
  (0 on clean/unverified)". Rewrite to: an **integrity** sweep over dangling refs + dangling
  `depends_on` + dependency cycles, exit 7 on any hard finding; note `dep list --cycles` shares the
  exit; update the KB/refs Decisions-table row if it names the ref-only scope. EN is source of truth;
  mirror RU.
- **CLI_REFERENCE (EN+RU):** `mtt check` now covers dangling `depends_on` and dependency cycles
  (exit 7); `dep list --cycles` exits 7 on a cycle (gates CI); `roadmap` marks a `(missing)` blocker.
  Rename any `ErrDanglingRefs`/"dangling references" wording to the integrity framing.
- **CLAUDE.md:** `internal/core` (the `CheckIntegrity` entry point + `IntegrityFinding`/`Kind`;
  `ErrDanglingRefs`→`ErrIntegrity`), `internal/cli` (check renders the four kinds; `--cycles` gates;
  roadmap missing marker).
- **CHANGELOG:** one `[Unreleased]` entry.

> **Exit-code note (for docs + a possible test):** a duplicate-id / stem-mismatch file makes the
> YAML `List()` **fail closed** (c15 load guard) → `mtt check` exits **1** (a load error surfaced
> before the sweep), *not* 7. So duplicate-id is correctly outside the exit-7 integrity umbrella —
> consistent with the Non-goal, and worth one line in CLI_REFERENCE so the two failure modes aren't
> conflated.

## Open seams (recorded, not built)

- **`check` as a flow gate** — a `commands: ["mtt check"]` on an edge could hard-gate integrity; the
  read-only-`mtt`-in-gates rule (SEC2) already permits it. Not wired here; it is a config choice.
- **Cross-store cycle / richer graph findings** (e.g. a `refs` cycle) — out of scope; `refs` are
  informational and non-blocking, so a refs cycle is not an integrity failure.
