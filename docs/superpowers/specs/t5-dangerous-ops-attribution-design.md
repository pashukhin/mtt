# t5 — dangerous-ops attribution (design spec)

- **Task:** `t5` (core, high) — *dangerous-ops attribution: force `--who`/`--why` on flow-bypassing / risk
  actions (`--no-run` over a real gate, `rm --force`); the parked roles seam, first concrete trigger.*
- **Date:** 2026-07-11
- **Status:** spec (speccing → spec_review)
- **Author:** Grigorii Pashukhin

## 1. Motivation

`mtt` already records attribution (`--who`/`--by`/`MTT_BY`/`config.local author` and `--why`) and enforces a
**project-global** required-attribution policy (`require: {who, why}` in `config.yaml`, currently `who: true`),
checked on every gated transition ([`internal/core/transition.go`](../../../internal/core/transition.go)).

What it does **not** do: force a **stronger, explicit signature** at the moments where an agent is *bypassing
or overriding a safety mechanism*. Two such moments exist today and are unattributed or under-attributed:

1. **Bypassing a gate** — `mtt status … --no-run` / `mtt do … --no-run` skips the edge's gate commands. Today
   only the global policy applies (so `why` is not forced), yet skipping a real gate is exactly the decision
   that should be justified in the record.
2. **Destroying data** — `mtt rm … --force` hard-deletes a task despite inbound references. This path does not
   go through attribution at all (it is not a transition, writes no history), so *who* deleted *what* and
   *why* is lost.

This is also the **first concrete trigger** for the parked "roles" seam. t5 deliberately stays at
**attribution** (who + why): it does **not** introduce roles, authorization, or "who may do X". It only forces
*who did it and why* to be recorded at critical points.

**Goal:** a single principle — **a dangerous action ⇒ who + why are mandatory** — applied to three sources of
"danger": the config (a per-edge critical transition), a bypass (`--no-run`), and a destruction (`rm --force`).

## 2. Decisions (from brainstorming)

| # | Decision |
|---|----------|
| D1 | **Scope:** `--no-run` bypass (transitions) **+** `rm --force` (destruction). `init --force` is **out** (bootstrap, no `.mtt/` history yet). |
| D2 | **Per-edge criticality** is expressed as **`require: {who, why}` on the transition** (reuse the existing `require` shape, at edge level). |
| D3 | **`rm --force` attribution home:** a new append-only **`.mtt/audit.log`** (a deleted task has no history to carry it). |
| D4 | **`--no-run` forces who + why ALWAYS** (unconditional — even on an edge with no commands). Simpler; no "has-commands" probe. |
| D5 | **Architecture (approach A):** policy lives in `core`; the audit log is a first-class **driven port** (`mtt.AuditStore`) with a YAML/JSONL adapter. |
| D6 | **audit.log format:** **JSON Lines** — one JSON object per line (append-safe, greppable, `.log` ⇒ line-oriented). |
| D7 | **audit.log in git:** **committed** + `.gitattributes` `merge=union` (real, durable, team-visible audit; union-merge dissolves append conflicts in the branch-per-task flow). |

**Unifying rule (the invariant):** the *effective* required attribution for an action is the **OR** of all
sources — global policy ∨ per-edge `require` ∨ bypass (`--no-run`) ∨ destruction (`rm --force`). Sources can
only **tighten**, never loosen.

## 3. Domain model (`pkg/mtt`)

### 3.1 Per-edge require

New value object (in [`config.go`](../../../pkg/mtt/config.go), beside `Transition`):

```go
// Require is a required-attribution policy: who/why must be supplied. Used both
// as the project-global default and as a per-edge (Transition) override; the two
// are unioned (tighten-only).
type Require struct {
    Who bool
    Why bool
}
```

`Transition` gains a field:

```go
type Transition struct {
    From        StatusName
    To          StatusName
    Description string
    Name        string
    Commands    []Command
    Current     CurrentAction
    Require     Require // per-edge required-attribution (empty = none); unioned with global + --no-run
}
```

The existing global policy stays where it is (`yaml.Settings.Require`, adapter-level, `tighten-only` overlay
of committed + local) — **not** refactored into the domain (out of scope; no behavior change there).

### 3.2 Audit port

New file `pkg/mtt/audit.go` — a **driven port** (the second beyond storage, after `Runner`):

```go
// AuditEntry records one out-of-flow dangerous action (a destruction that has no
// task history to carry its attribution).
type AuditEntry struct {
    At     time.Time
    Who    string // acting subject (--who/--by/MTT_BY/config.local author)
    Why    string // --why
    Action string // e.g. "rm --force"
    TaskID TaskID
}

// AuditStore appends dangerous-action records. Append-only; no read surface in t5.
type AuditStore interface {
    Append(AuditEntry) error
}
```

## 4. Core (`internal/core`)

### 4.1 Transitioner — union policy

In `Transition(id, to, opts)`, **after** the edge is found and **before** `missingAttribution`, compute the
effective requirement from all sources and check *that*:

```go
effWho := opts.RequireWho || edge.Require.Who || opts.NoRun
effWhy := opts.RequireWhy || edge.Require.Why || opts.NoRun
```

`missingAttribution` is refactored to take the **effective** booleans (not read `opts.RequireWho/Why`
directly), returning the aggregated missing list (`who`, `why`) → one `ErrMissingAttribution` (unchanged
sentinel, unchanged CLI exit code 2). Fail-fast: the check precedes the gate, so `--no-run` (which skips the
gate) still cannot skip the signature. This single site covers **both** `mtt status --no-run` and
`mtt do --no-run` (both route through `Transitioner.Transition`).

On success the reason rides the existing `HistoryEntry.Why` — no new storage.

### 4.2 Remover — force ⇒ who + why + audit

`Remover` gains an `AuditStore` dependency and threads who/why. **`RemoveMany` grows an `error` return** for a
**pre-flight** failure (the missing-attribution precondition), distinct from the per-id best-effort
`[]RemoveResult`:

```go
func NewRemover(store mtt.TaskStore, audit mtt.AuditStore, now func() time.Time) *Remover
func (r *Remover) RemoveMany(ids []mtt.TaskID, force bool, by, why string) ([]RemoveResult, error)
func (r *Remover) Remove(id mtt.TaskID, force bool, by, why string) error // thin wrapper (see note)
```

**B1 — attribution is a pre-flight precondition, not a per-id outcome (fixes bulk exit-code hole).** The
review found that folding a missing-attribution error into the per-id `RemoveResult.Err` would, on the bulk
path, be aggregated by `reportBulk` into a plain `fmt.Errorf` with **no `%w`** ([`bulk.go:81`](../../../internal/cli/bulk.go#L81)),
so `errors.Is(err, ErrMissingAttribution)` is false and the CLI falls to the **default exit 1**
([`root.go:171-184`](../../../internal/cli/root.go#L171-L184)) — contradicting §6/§8/invariant #2 (which require
exit 2). Fix: `RemoveMany` checks the precondition **first** and returns `ErrMissingAttribution` as its
**second return value** (the `[]RemoveResult` is nil/empty) **before touching any task**. Both CLI paths
(`runRmSingle` and bulk) return that error **directly** (raw sentinel, not through `reportBulk`), so it maps to
**exit 2** uniformly. Single vs bulk no longer diverge.

**B2 — append BEFORE delete (makes invariant #2 achievable; also fixes M1).** For **each** id under `force`,
in order: (i) `audit.Append(AuditEntry{now(), by, why, "rm --force", id})`; (ii) **only if Append
succeeds**, `store.Delete(id)`. Rationale: for an audit feature the guarantee that matters is *no destruction
without a record* — so the record is written first. Consequences:
- **Append fails** → the id is **not** deleted; its `RemoveResult.Err` carries the append error; `current` is
  untouched (nothing was destroyed) — this closes M1 (no dangling `current` pointer). Best-effort continues
  with the remaining ids.
- **Append ok, Delete fails** (rare — `force` bypasses reject-if-referenced, so only an I/O error) → a record
  exists for a task still present; treated as a **recorded attempt** (acceptable: a spurious log line is
  strictly safer than a silent destruction). Surfaced as that id's `RemoveResult.Err`.
- **Both ok** → `current` is cleared by the CLI iff it named the deleted id (existing `clearCurrentIfMatches`),
  which runs only on `RemoveResult.Err == nil`.

Other behavior:
- **`force == false`** → the existing reject-if-referenced path, **unchanged**: no who/why required, no audit
  write, `RemoveMany`'s error return is `nil`.
- **`Remove`** is still a thin wrapper over `RemoveMany([id])` but its semantics are **no longer "unchanged"**:
  it now returns `ErrMissingAttribution` under `force` without who/why, and triggers an audit write on success.
- A clock (`now func() time.Time`) is injected (deterministic tests), matching sibling usecases.
- `store.Delete` is the existing `TaskStore` method the current `RemoveMany` already calls (see [`remove.go`](../../../internal/core/remove.go)); only its **ordering** relative to `Append` is new.

## 5. Adapter (`internal/adapter/yaml`)

### 5.1 Per-edge require DTO

`ymlTransition` ([`dto.go`](../../../internal/adapter/yaml/dto.go)) gains
`Require ymlRequire \`yaml:"require,omitempty"\`` — the `ymlRequire{Who,Why}` shape already exists (reused).
The config is **load-only** (decoded from `ymlConfig`; it is generated from embedded `text/template` starter
templates, never marshaled back from the struct), so only the **decode** direction is wired: `toConfig` maps
`yr.Require` into the domain `Transition.Require`. A per-edge `require:` is authored by hand in the committed
`.mtt/config.yaml` (or a test fixture) and simply absent on edges that don't declare it (an unmarshaled zero
`ymlRequire{}` → zero `Require{}`). The `yaml:",omitempty"` tag mirrors the existing top-level
[`dto.go:24`](../../../internal/adapter/yaml/dto.go#L24) for consistency; it has no effect on a struct field in
`yaml.v3` and there is no marshal path — noted only so no one relies on it.

### 5.2 Audit adapter

New `internal/adapter/yaml/audit.go` implementing `mtt.AuditStore`:
- `Append` `MkdirAll(.mtt)` defensively (cheap; the root always exists in the `rm` path but the adapter stays
  self-sufficient), then opens `.mtt/audit.log` with `O_APPEND|O_CREATE|O_WRONLY` (0644) and writes **one JSON
  line**: `{"at":"<RFC3339 UTC>","who":"…","why":"…","action":"rm --force","id":"t7"}\n`.
- Time is formatted RFC3339 UTC (matching task timestamps); the entry is `json.Marshal`ed then `+ "\n"`.
- No read/parse surface in t5 (append-only). A reader can `grep`/`jq` the file; ordering is by append, with
  `at` available for chronological sort after a union-merge.

### 5.3 Git policy

- New `.gitattributes` at repo root: `/.mtt/audit.log merge=union` — the built-in union driver (no gitconfig
  needed) takes **both** sides' lines on a conflicting hunk instead of raising a conflict, which is the right
  behavior for an append-only line log. Caveats (honest): union does **not** sort or dedup — cross-branch
  entries may interleave (harmless; each carries `at`) and, in the rare case two branches log the identical
  action, the line can appear twice. This is about **branch merges**; the PR squash itself doesn't append.
- `.mtt/audit.log` is **committed** (rides the task branch → PR → squash), like `.mtt/tasks/*.yaml`. It is
  **not** added to `.gitignore`.

## 6. CLI (`internal/cli`)

- `rm.go`: `yaml.Load` returns `(cfg, settings, err)` — `rm` needs only `settings.Author`, so discard `cfg`
  with `_` (an unused var reddens golangci-lint). Call `resolveAttribution(cmd, settings.Author)` which returns
  `(role, by, why, err)` ([`status.go:180`](../../../internal/cli/status.go#L180)) — **propagate its `err`**
  (it errors when `--who` and `--by` are both set), and ignore `role` (not used by `rm`). Build
  `core.NewRemover(store, yaml.NewAuditStore(root), time.Now)` and pass `force, by, why` through both
  `runRmSingle` and the bulk path.
- Both paths return `RemoveMany`'s **pre-flight `error` directly** (raw `ErrMissingAttribution`), never through
  `reportBulk` — so it maps to **exit 2** on single *and* bulk (the B1 fix).
- `--who`/`--why` are already **root-persistent** → `rm` inherits them; no new flags.
- No CLI wiring change for transitions: `runTransition` already passes `settings.Require.{Who,Why}` and
  `NoRun`; the per-edge `require` rides the domain `edge.Require` that `core` reads. (`mtt types` **may**
  render a per-edge `require` marker for discoverability — a small optional `writeTypeBlock` touch, not
  required for correctness.)

## 7. Repo config (dogfood)

Optionally mark one or two genuinely critical edges in `.mtt/config.yaml` with `require: {who, why}`
(candidates: `deliver` `approved→done`; the `cancel` edges, which run `git switch main` and can strand
task-branch context). **Guard note (corrected):** `TestRepoDogfoodConfig`
([`dogfood_test.go`](../../../internal/adapter/yaml/dogfood_test.go)) asserts settings-level `require.who` and
**fixed transition counts** — it does **not** read per-edge `.Require`. Adding `require:` to an existing edge
changes no transition count, so the guard neither reddens nor protects the new field; a dedicated assertion
would have to be added if we want the marked edge guarded. Either way, per-edge `require` is exercised by the
adapter decode test (§9) independently of the repo config, so **the repo config may stay untouched** and the
feature is still fully tested. Decided at implementation time.

## 8. Error handling / exit codes

- Missing required attribution (bypass edge / critical edge / `rm --force`) → `core.ErrMissingAttribution` →
  **exit 2**; message aggregates the missing fields (`missing required attribution: who, why`). On `rm`, this
  is `RemoveMany`'s **pre-flight** error return, forwarded raw by the CLI on **both** single and bulk paths
  (never through `reportBulk`), so exit 2 is uniform (B1).
- `rm --force`: who/why validated **before** any deletion (fail-fast, nothing removed on failure).
- **Audit is written BEFORE delete (B2), per id.** `Append` fails → the id is **not** deleted, `current`
  untouched, error in that id's `RemoveResult.Err`; best-effort continues. `Append` ok but `Delete` fails
  (I/O; `force` already bypassed reject-if-referenced) → a record exists for a still-present task, treated as a
  recorded attempt (a spurious line is safer than a silent destruction) and surfaced as `RemoveResult.Err`. The
  audit failure is never swallowed. Net guarantee: **no `--force` destruction without a preceding record.**
- `--no-run` forces who+why **before** the (skipped) gate — a clean fail-fast before the status changes.

## 9. Testing (TDD: red → green → refactor)

> **Anti-vacuity note (M2).** In this repo `who` is *pre-satisfied* — the global `require.who: true` plus a
> `config.local` author mean `effWho` is already true and `by` is already non-empty. So the genuinely new
> forcing is **`why`**. Every test that asserts a path *forces who* MUST set `RequireWho=false` **and** empty
> `by`/author, or it passes vacuously (the global policy, not the new code, would be doing the work). Tests
> below are written against the **core** options directly (not the repo config), so they control this.

- **`core/transition_test.go`**: (a) critical edge (`Require.Why`) without `--why` → `ErrMissingAttribution`;
  (b) `NoRun=true` forces **why** on an edge with no `require`/no commands (global `RequireWhy=false`); (b′)
  `NoRun=true` with `RequireWho=false` **and** `By=""` forces **who** (non-vacuous who check); (c) union — e.g.
  global `who` + edge `why` requires both; (d) with who+why the move succeeds and `HistoryEntry.Why` is
  recorded.
- **`core/remove_test.go`** (fake `AuditStore`): (a) `force` with empty `by`/`why` → pre-flight
  `ErrMissingAttribution` as the **error return**, `[]RemoveResult` empty, `store.Delete` never called, audit
  empty; (b) `force` with who+why → tasks deleted **and** one audit entry per id, **append recorded before
  delete** (fake orders calls); (c) `force=false` unchanged, error return `nil`, audit empty; (d) bulk: one
  pre-flight check (missing → whole call errors, nothing deleted); (e) **append-fails fake** → that id not
  deleted, its `RemoveResult.Err` set, other ids proceed.
- **`adapter/yaml`**: (a) **decode** a config whose transition carries `require: {who, why}` → the domain
  `Transition.Require` is set (load-only; no marshal path); (b) `AuditStore.Append` produces valid JSONL that
  parses back with fields intact; append accumulates lines across calls; (c) `Append` creates `.mtt` if absent.
- **e2e testscript** `internal/cli/testdata/scripts/dangerous.txt` (env has no `MTT_BY`; a fixture config
  without global `require.who` so exit-2 is caused by the new code, not the global policy): `mtt <edge> <id>
  --no-run` without `--why` → exit 2; with `--who/--why` → passes; `mtt rm <id> --force` without `--why` →
  exit 2, task still present; with `--who/--why` → deleted **and** `grep` finds the record in `.mtt/audit.log`;
  **bulk** `mtt rm a b --force` without `--why` → exit 2, both present (the B1 regression guard).
- **Guards:** `make check` green before commit; if a repo edge is later marked `require:`, add a matching
  `TestRepoDogfoodConfig` assertion (the current guard would not otherwise cover it — see §7).

## 10. Affected files

`pkg/mtt/{config.go,audit.go}` · `internal/core/{transition.go,remove.go}` ·
`internal/adapter/yaml/{dto.go,audit.go}` · `internal/cli/rm.go` · `.gitattributes` (new) · tests +
`internal/cli/testdata/scripts/dangerous.txt` · docs sync: `DESIGN.md`/`DESIGN.ru.md`,
`CLI_REFERENCE.md`/`CLI_REFERENCE.ru.md`, `AGENTS.md` ("dangerous ops"), the three touched-package `CLAUDE.md`
(cli, core, adapter/yaml).

**Signature-churn (M3) — do not understate.** Changing `NewRemover`/`Remove`/`RemoveMany` ripples to:
- the architecture reference [`docs/architecture/model.go:599-610`](../../../docs/architecture/model.go#L599-L610)
  — `Remove`/`RemoveMany`/`NewRemover` declarations must be updated to the new shapes (kept in sync by design);
- **~11 call sites** of `NewRemover`/`Remove`/`RemoveMany` across `internal/core/remove_test.go` and
  `internal/cli/rm.go` — all must thread the new `audit`, `now`, `by`, `why` args and the `RemoveMany` error
  return.

## 11. Out of scope (explicit)

- **Roles / authorization** ("who *may* do X"): parked. t5 is attribution only.
- **`init --force`**: excluded (bootstrap; no `.mtt/` yet).
- **Audit read/query surface** (`mtt audit`, filtering): not in t5 — append-only now, read later if needed.
- **Refactoring the global `require`** out of `yaml.Settings` into the domain: not needed; no behavior change.
- **Attribution on non-`--force` `rm`** and other non-destructive commands: out — only the named dangerous
  actions.
- **Mandatory who+why on `cancel` edges** (context-destroying `git switch main`): out of *hard-coded* scope —
  but this is **already expressible** with the new per-edge `require: {who, why}` mechanism (a config choice,
  §7), not a code gap. t5 ships the mechanism; whether the repo config marks `cancel` is a separate call.

## 12. Invariants (self-instructing)

1. `--no-run` (any edge) ⇒ who + why mandatory (fail-fast before the status changes).
2. `rm --force` ⇒ who + why mandatory (pre-flight, exit 2 on single **and** bulk) **and** the audit record is
   written **before** the delete, so **no `--force` destruction occurs without a preceding `.mtt/audit.log`
   record**. (A record may exist without a delete only if `Delete` itself fails after a successful append — a
   recorded attempt, never a silent destruction.)
3. Effective requirement = OR over {global, per-edge, --no-run, --force} — tighten-only, never loosened.
