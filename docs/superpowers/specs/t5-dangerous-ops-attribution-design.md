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

`Remover` gains an `AuditStore` dependency and threads who/why:

```go
func NewRemover(store mtt.TaskStore, audit mtt.AuditStore, now func() time.Time) *Remover
func (r *Remover) RemoveMany(ids []mtt.TaskID, force bool, by, why string) []RemoveResult
func (r *Remover) Remove(id mtt.TaskID, force bool, by, why string) error // thin wrapper, unchanged semantics
```

Behavior:
- **`force` and (`by == ""` or `why == ""`)** → return `ErrMissingAttribution` **before any deletion**
  (atomic: nothing removed). Applies once per call (bulk: a single up-front check).
- For **each id actually deleted** under `force` → `audit.Append(AuditEntry{now(), by, why, "rm --force", id})`.
- **`force == false`** → the existing reject-if-referenced path, **unchanged**: no who/why required, no audit
  write.
- A clock (`now func() time.Time`) is injected (deterministic tests), matching sibling usecases.

## 5. Adapter (`internal/adapter/yaml`)

### 5.1 Per-edge require DTO

`ymlTransition` ([`dto.go`](../../../internal/adapter/yaml/dto.go)) gains
`Require ymlRequire \`yaml:"require,omitempty"\`` — the `ymlRequire{Who,Why}` shape already exists (reused).
The config is **load-only** (decoded from `ymlConfig`; it is generated from embedded `text/template` starter
templates, never marshaled back from the struct), so only the **decode** direction is wired: `toConfig` maps
`yr.Require` into the domain `Transition.Require`. A per-edge `require:` is authored by hand in the committed
`.mtt/config.yaml` (or a test fixture); the `omitempty` keeps it absent by default.

### 5.2 Audit adapter

New `internal/adapter/yaml/audit.go` implementing `mtt.AuditStore`:
- `Append` opens `.mtt/audit.log` with `O_APPEND|O_CREATE|O_WRONLY` (0644) and writes **one JSON line**:
  `{"at":"<RFC3339 UTC>","who":"…","why":"…","action":"rm --force","id":"t7"}\n`.
- Time is formatted RFC3339 UTC (matching task timestamps); the entry is `json.Marshal`ed then `+ "\n"`.
- No read/parse surface in t5 (append-only). A reader can `grep`/`jq` the file; ordering is by append, with
  `at` available for chronological sort after a union-merge.

### 5.3 Git policy

- New `.gitattributes` at repo root: `/.mtt/audit.log merge=union` (append-only union-merge dissolves
  cross-branch append conflicts).
- `.mtt/audit.log` is **committed** (rides the task branch → PR → squash), like `.mtt/tasks/*.yaml`. It is
  **not** added to `.gitignore`.

## 6. CLI (`internal/cli`)

- `rm.go`: load `cfg, settings` (for `settings.Author` default), call `resolveAttribution(cmd, settings.Author)`
  → `by`/`why` (reuse the existing helper from [`status.go`](../../../internal/cli/status.go); same package),
  build `core.NewRemover(store, yaml.NewAuditStore(root), time.Now)`, and pass `force, by, why` through both
  `runRmSingle` and the bulk path.
- `--who`/`--why` are already **root-persistent** → `rm` inherits them; no new flags.
- Missing attribution on `--force` surfaces as `ErrMissingAttribution` → **exit 2** (existing `exitCode` map).
- No CLI wiring change for transitions: `runTransition` already passes `settings.Require.{Who,Why}` and
  `NoRun`; the per-edge `require` rides the domain `edge.Require` that `core` reads. (`mtt types` **may**
  render a per-edge `require` marker for discoverability — a small optional `writeTypeBlock` touch, not
  required for correctness.)

## 7. Repo config (dogfood)

Optionally mark one or two genuinely critical edges in `.mtt/config.yaml` with `require: {who, why}`
(candidates: `deliver` `approved→done`; the `cancel` edges). If done, update `TestRepoDogfoodConfig` to keep
the committed-config guard green. Alternatively the repo config stays untouched and per-edge `require` is
exercised only in a test fixture config — both are valid; decided at implementation time.

## 8. Error handling / exit codes

- Missing required attribution (bypass edge / critical edge / `rm --force`) → `core.ErrMissingAttribution` →
  **exit 2**; message aggregates the missing fields (`missing required attribution: who, why`).
- `rm --force`: who/why validated **before** any deletion (fail-fast, nothing removed on failure).
- Audit is written **after** a successful delete (deletion is irreversible). If `Append` fails (I/O), it does
  **not** roll back the delete (nothing to roll back); the error surfaces in that id's `RemoveResult.Err`
  (stderr / bulk summary) — the audit failure is never swallowed. Operator sees: task deleted, log not
  written.
- `--no-run` forces who+why **before** the (skipped) gate — a clean fail-fast before the status changes.

## 9. Testing (TDD: red → green → refactor)

- **`core/transition_test.go`**: (a) critical edge (`Require.Why`) without `--why` → `ErrMissingAttribution`;
  (b) `NoRun=true` forces who+why even on an edge with no `require` and no commands; (c) union with the global
  policy; (d) with who+why the move succeeds and `HistoryEntry.Why` is recorded.
- **`core/remove_test.go`** (fake `AuditStore`): (a) `force` without who/why → `ErrMissingAttribution`,
  nothing deleted; (b) `force` with who+why → tasks deleted **and** one audit entry per id; (c) `force=false`
  unchanged, audit empty; (d) bulk: one up-front who/why check, N audit entries.
- **`adapter/yaml`**: (a) **decode** a config whose transition carries `require: {who, why}` → the domain
  `Transition.Require` is set (load-only; no marshal path); (b) `AuditStore.Append` produces valid JSONL that
  parses back with fields intact; append accumulates lines across calls.
- **e2e testscript** `internal/cli/testdata/scripts/dangerous.txt`: `mtt <edge> <id> --no-run` without
  `--why` → exit 2; with `--who/--why` → passes; `mtt rm <id> --force` without `--why` → exit 2, task still
  present; with `--who/--why` → deleted **and** `grep` finds the record in `.mtt/audit.log`.
- **Guards:** keep `TestRepoDogfoodConfig` green (if a repo edge is marked); `make check` green before commit.

## 10. Affected files

`pkg/mtt/{config.go,audit.go}` · `internal/core/{transition.go,remove.go}` ·
`internal/adapter/yaml/{dto.go,audit.go}` · `internal/cli/rm.go` · `.gitattributes` (new) · tests +
`internal/cli/testdata/scripts/dangerous.txt` · docs sync: `DESIGN.md`/`DESIGN.ru.md`,
`CLI_REFERENCE.md`/`CLI_REFERENCE.ru.md`, `AGENTS.md` ("dangerous ops"), the three touched-package `CLAUDE.md`
(cli, core, adapter/yaml).

## 11. Out of scope (explicit)

- **Roles / authorization** ("who *may* do X"): parked. t5 is attribution only.
- **`init --force`**: excluded (bootstrap; no `.mtt/` yet).
- **Audit read/query surface** (`mtt audit`, filtering): not in t5 — append-only now, read later if needed.
- **Refactoring the global `require`** out of `yaml.Settings` into the domain: not needed; no behavior change.
- **Attribution on non-`--force` `rm`** and other non-destructive commands: out — only the named dangerous
  actions.

## 12. Invariants (self-instructing)

1. `--no-run` (any edge) ⇒ who + why mandatory.
2. `rm --force` ⇒ who + why mandatory + exactly one `.mtt/audit.log` entry per deleted id.
3. Effective requirement = OR over {global, per-edge, --no-run, --force} — tighten-only, never loosened.
