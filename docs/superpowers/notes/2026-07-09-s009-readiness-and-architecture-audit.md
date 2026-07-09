# s009 readiness & architecture audit — findings and recommendations

Status: **analysis findings + recommendations**, 2026-07-09. Companion to
[2026-07-09-positioning-and-agent-ux-analysis.md](2026-07-09-positioning-and-agent-ux-analysis.md)
(vectors 1–2: positioning + agent UX; its `U*` items are cross-referenced here). This note covers:

- **Part A — s009 dogfood readiness** (`S*` items): a review of the s009 spec
  ([specs/2026-07-09-session-009-dogfood-design.md](../specs/2026-07-09-session-009-dogfood-design.md)),
  [sessions/009_dogfood.md](../../../sessions/009_dogfood.md), and the flow-granularity note
  ([2026-07-09-flow-granularity-for-dogfood.md](2026-07-09-flow-granularity-for-dogfood.md)) against each
  other, the git history, and live behavior of the built `0.8.9-dev` binary.
- **Part B — architecture-vs-declarations audit** (`A*` items): three independent code audits (import
  graph / layer purity; the name-agnostic invariant; ports & GAPs vs
  [docs/architecture/model.go](../../architecture/model.go)), each verified with file:line evidence.

**How to use this note.** Each item is self-contained: evidence (file:line) → why it matters → fix
sketch → acceptance. Items are independently actionable; do not bundle unrelated ones into one branch.
Priorities are in §Priorities at the end. Part B also lists **what is verified CLEAN — do not "fix"
those**; the clean verdicts are load-bearing context for any refactor.

---

## Part A — s009 dogfood readiness

Overall: the spec is disciplined (forward-only migration, honest gates, proven e2e strategy, a
committed-config guard test) — but it contradicts a later-recorded decision, and three concrete gaps
need closing **before** writing-plans.

### S1 — BLOCKER: the spec contradicts recorded decision A (session flow shape)

- **Evidence:** the spec (commit `84e469c`, 2026-07-09 20:15) and `sessions/009_dogfood.md` define
  `session` as `tbd → in_progress → done` (+`cancelled`), branch on `→ in_progress`. A **later** commit
  `dd2d903` (20:32, touches only the flow-granularity note) records **decision A**: the full flow
  `tbd → speccing → planning → in_progress → review → done` (+`cancelled`), with branch + `current: set`
  on `→ speccing`. Both documents claim authority ("Authoritative design for s009" vs "Decisions carried
  into s009 … made").
- **Why it matters:** writing-plans would build from a stale Q3; the guard test and config would encode
  the wrong flow.
- **Fix sketch:** reconcile explicitly, in one docs commit. Recommended direction: **follow decision A**
  (it is later, deliberate, and its rationale — history shows *what* was done per stage and *how many
  times* a session looped back — is recorded). That means rewriting spec **Q3** and the session file:
  full status list; a `description` on every edge (they are the agent's runbook — s008.95 guidance
  prints them); gate placement per the note's §8 (`make check` on `→ review` and `→ done` only; early
  edges instruction-only); branch + `current: set` moves to `tbd → speccing`;
  `TestRepoDogfoodConfig` assertions updated to check the branch command on the `→ speccing` edge (not
  `→ in_progress`). If instead decision A is revoked, edit the note's §8 to say so and why.
- **Acceptance:** spec, session file, and note agree on one flow; the guard-test description in the spec
  matches that flow's edges.

### S2 — the branch command: broken rollback pattern + misplaced idempotency rationale

- **Evidence:** the note (§6, §8) prescribes `git checkout -b feat/{{.ID}}` paired with
  `rollback: git branch -D feat/{{.ID}}` "for idempotency". Two problems:
  1. The compensator **fails when run** (verified live): after `checkout -b` the process is *on* the new
     branch, and git refuses to delete the checked-out branch. See companion note **U1** (full repro).
  2. The rationale is confused: a `rollback:` runs only when a **later command in the same pipeline**
     fails (s008 semantics; the failing command's own rollback is never run). It does nothing for
     *retry* idempotency. In both candidate flows the branch command is the **only** command on its
     edge, so a rollback there is dead weight.
- **The real retry hazard:** a stale `feat/sN` branch already exists (from a previous manual attempt or
  an aborted session) → `checkout -b` exits non-zero → the edge blocks forever until someone deletes the
  branch by hand.
- **Fix sketch:** in the s009 config use an idempotent form —
  `git switch -c feat/{{.ID}} || git switch feat/{{.ID}}`
  (re-entering an existing session branch is the correct semantics for re-taking a session into work;
  the shell seam is already relied on by `! make test` in the coding template). No `rollback:` on that
  edge. Fix the documented pattern in DESIGN.md and the note per companion **U1**.
- **Acceptance:** `mtt speccing s1` (or `in_progress` under the simple flow) succeeds on a fresh repo,
  **and again** after the task is reset and the branch already exists; no failed-compensation output.

### S3 — "ordering comes from priorities" over-promises; migration order is timing-dependent

- **Evidence:** equal-priority roadmap/list ordering is `Created` **desc** (freshest first), tie-broken
  by ID as an opaque string **ascending** ([internal/core/list.go:89](../../../internal/core/list.go#L89),
  [internal/core/index.go:13](../../../internal/core/index.go#L13)). Consequences for the migration
  script (`p1`–`p5` phases then `s1`–`s4` sessions, priorities high/medium/medium/low):
  - if the equal-priority `add`s land in **different** wall-clock seconds, the later-created sorts
    first — the medium pair (comments/profiles) renders in *reverse* creation order;
  - if they land in the **same** second, the ID tiebreak is lexicographic — and `"p2" < "s2"`, so
    **bare future phases (medium by default) sort before medium sessions**.
  Either way the rendered order differs from the intended reading order, non-deterministically.
- **Fix sketch:** (a) give the bare phases `p2`–`p5` an explicit `--priority low` so they sink;
  (b) accept that the comments-vs-profiles order is cosmetic OR encode it with distinct priorities;
  (c) per the s008.6 carry-over lesson ("run the primitive by hand"), run `mtt roadmap` on the migrated
  set and eyeball it **before** committing `.mtt/tasks/*.yaml`; (d) soften the spec's "ordering in
  `mtt roadmap` comes from priorities" to "references-first and demo-last come from priorities; ties are
  recency-ordered".
- **Acceptance:** committed migration renders a `mtt roadmap` where no empty far-future phase precedes
  an actionable Phase-4 session.

### S4 — unclosed workflow seam: who commits the task-file mutations, and when

- **Evidence:** every `mtt add`/`status` move mutates `.mtt/tasks/*.yaml`. In the self-hosted workflow:
  a new session is `add`-ed while on `main` (uncommitted change); the take-into-work transition creates
  the branch and the uncommitted YAML rides along to it; every subsequent status move mutates the file
  on the branch. Two frictions: (1) the `→ done` move writes the file **after** the gate passes — if the
  agent runs `mtt done` as the last act before the PR merge, the `status: done` change is easy to leave
  uncommitted; (2) between `add` and the squash-merge, the task is invisible on `main`.
- **Fix sketch:** one process paragraph in the spec's "Docs sync" / DESIGN.md dogfood note:
  ".mtt mutations are committed with the session PR; after `mtt done`, `git add .mtt && git commit
  --amend` (or a final commit) before merging." Optionally note the visibility gap as accepted (solo).
- **Acceptance:** the dogfood docs state the rule; the first self-hosted session PR contains the task
  file in its final status.

### S5 — record the gate-cost decision explicitly

Under decision A with `make check` on `→ review` + `→ done` and per-step `make check` on step
`→ done`, a 5-step session runs the full gate ~7 times (minutes each with `-race`). The note's §6
warns "gate cost compounds"; the spec should own the number and state it is accepted (the
`command_timeout: 10m` headroom already exists). No behavior change — a one-paragraph decision record
so the cost is chosen, not discovered.

### S6 — the committed-config guard is the ONLY validation of the committed config

`Config.Validate` runs on `add`/`types`, **not** on `Load` (carry-over lesson, s008). So
`TestRepoDogfoodConfig` is not a nicety — it is the sole CI guard against a broken committed
`.mtt/config.yaml`. Keep it in the spec's acceptance as-is; under decision A update its assertions
(S1). Consider asserting *every* type's flow validates, not only the three types' existence.

### S7 — hygiene: stray `bin/.mtt` in the working tree

A leftover smoke-test artifact (`bin/.mtt/config.yaml` + `tasks/t1.yaml`) sits under the gitignored
`bin/`. It will hijack `FindRoot` for any mtt invocation with cwd under `bin/`, shadowing the repo's
real `.mtt/` once s009 commits one. `rm -rf bin/.mtt`; optionally have `make smoke` clean up after
itself.

### S8 — U2 (blocked-gate output visibility) is the first candidate "in-scope enabler"

The spec allows "if a real gap surfaces during migration, it is TDD'd as a separate, in-scope enabler".
Companion **U2** (a blocked gate hides *why* the command failed; the agent must re-run the whole slow
gate with `-v`) will be felt on the **first** failed `make check` of dogfooding, and its cost repeats
for every red gate thereafter. Recommend treating it as that enabler at the start of s009 rather than
mid-migration.

### What is already sound (verified, keep)

Q1 forward-only migration (live queue, not an archive) — coherent and matches the product story;
`p`/`s`/`t` prefixes are pairwise non-prefix-free — the flat mint is unambiguous; the bootstrap caveats
(mtt ids ≠ historical `sNNN`; no slug in branch names — structural placeholder whitelist; s009 itself
runs on the manual branch) are honestly recorded; the e2e strategy (scratch config, fake commands,
`[!exec:git] skip`) is the proven s006/s007/s008 pattern; `command_timeout: 10m` is sensible.

---

## Part B — architecture-vs-declarations audit

Three independent audits, all with file:line evidence. **Headline: zero violations of the declared
architecture.** The findings below are operational risks and contract debt the layer discipline cannot
see — ranked by when they bite.

### Verified CLEAN (do not "fix" these; they are the baseline)

- **Import graph** exactly `cli → core → port ← adapter` (via `go list`): `pkg/mtt` → stdlib only;
  `internal/core` → `pkg/mtt` only; both adapters → `pkg/mtt` only (+`yaml.v3`); `internal/cli` is the
  composition root. No back-edges, including in tests. `core.Runner` deliberately lives in
  [internal/core/runner.go](../../../internal/core/runner.go); `CurrentStore` in `pkg/mtt/current.go`.
- **`pkg/mtt` purity:** zero `yaml:`/`json:` tags; imports `errors fmt regexp strings time` only; no
  `prefix`/path concepts (prefixes only in the adapter DTO/Settings). Value objects all present with
  `Valid()`: `StatusKind`, `CurrentAction`, `Priority`(+`Rank`), `Command` (leaf-rollback rule),
  `RefKind`.
- **`core` purity:** no I/O imports (`text/template` is pure computation for placeholder expansion);
  **zero direct `time.Now()`** — every mutating usecase takes an injected `now`; no serialization
  awareness.
- **Adapters carry no business rules:** yaml = CRUD + DTO mapping + atomic writes + minting (minting is
  declared adapter-correct); exec only launches/reports — compensation *policy* (what, order) is
  computed in core.
- **Thin CLI:** no `TaskStore` mutations from cli (all via core usecases); the three documented
  exceptions (selector; `applyCurrent` at the composition root; sugar fallback-routing) hold exactly as
  documented.
- **Name-agnostic invariant fully holds in logic:** zero comparisons against type/status name literals
  anywhere in decisions; kind-by-topology everywhere (`kindOf`/`StatusKind`, `terminalSatisfied`,
  `Type.InitialStatus`, `Config.DefaultType`); ID structure (`regexp`+`strconv` over `<prefix><N>`)
  confined to [internal/adapter/yaml/mint.go](../../../internal/adapter/yaml/mint.go); sugar routing
  classifies arg0 against the task's **configured** flow (no hardcoded verb list).
- **Hygiene:** zero `panic(` in `pkg/`+`internal/`; `%w` wrapping + sentinel taxonomy
  (`ErrNotFound`/`ErrBlocked`/`ErrInvalidTransition`/`ErrMissingAttribution`) consistently mapped to
  exit codes in `root.go exitCode`; errcheck active.
- **Contract fidelity:** shipped `pkg/mtt` + core usecase surface is signature-identical to
  `docs/architecture/model.go` T1; deliberately-absent capability surface (`Capabilities()`,
  `ErrUnsupported`, `HistoryStore`/`DependencyStore`/`CommentStore`/`SearchStore`/`KnowledgeStore`,
  `ResolvedFlow`, `Advancer`) is **truly absent** and — verified — no shipped code quietly assumes any
  of it (zero type assertions in `internal/`). GAP #1 (embedded field rides `Update`) holds at every
  call site; GAP #2 (typed identities, DTO maps strings at boundary) holds; GAP #5 (By resolution
  order) holds.

### A1 — zero-byte mint window can take down every list-reading command — RISK: high for dogfood

- **Evidence:** `mint` reserves an ID by creating a **zero-byte** file with `O_EXCL`
  ([internal/adapter/yaml/mint.go:177](../../../internal/adapter/yaml/mint.go#L177)); the real content
  arrives only at `Create`'s later write ([task.go:41-55](../../../internal/adapter/yaml/task.go#L41)).
  A concurrent `List()` in that window — or **any crash between reserve and write** — hits the empty
  file: `Unmarshal` yields a zero DTO, `NewTaskID("")` fails, and the whole `List` errors
  ([task.go:111-114](../../../internal/adapter/yaml/task.go#L111)) — killing `list`/`tree`/`ready`/
  `roadmap`/`rm`/selector until the stub is removed by hand. The error (`mtt: empty task id`) carries
  **no file path** (`toDomain` errors returned unwrapped in `List` and `Get`), so at volume the corrupt
  file is expensive to locate.
- **Fix sketch (two independent cheap steps):** (1) wrap `toDomain` errors in `List`/`Get` with the
  file path (`fmt.Errorf("%s: %w", path, err)`) — turns a mystery into a one-command fix; (2) make
  `List` tolerate (skip + collect/warn) or make the reserve non-observable (e.g. reserve as
  `<id>.yaml.tmp` + rename at Create — but that changes mint's collision semantics; think before
  building). Step (1) alone removes most of the pain.
- **Acceptance:** with a hand-planted empty `tasks/t99.yaml`, `mtt list` either names the file in its
  error or skips it with a warning; a unit test locks it.

### A2 — `Status.Default` is a dead domain feature through the YAML adapter — BUG (silent drop)

- **Evidence:** the domain defines a `default:` marker on a **status** (entry point when a flow has
  multiple initials): field in `pkg/mtt/config.go`, validated (≤1 per flow, must be initial) in
  `validate.go`, resolved by `Type.InitialStatus`. The YAML DTO `ymlStatus`
  ([internal/adapter/yaml/dto.go:48-52](../../../internal/adapter/yaml/dto.go#L48)) has **no `default`
  field** and `toDomain` ([dto.go:122-124](../../../internal/adapter/yaml/dto.go#L122)) never maps it —
  a user's `default: true` on a status is silently dropped; the fallback (first initial in config
  order) always wins.
- **Why now:** any richer flow (s009 decision A territory) with multiple initials will reach for this
  documented knob and silently not get it.
- **Fix sketch:** add `Default bool \`yaml:"default,omitempty"\`` to `ymlStatus`, map it in `toDomain`,
  golden-test a two-initial flow with a marked default.
- **Acceptance:** a config with two initials + `default: true` on the second yields
  `Type.InitialStatus() ==` the marked one via `Load` (not only via a hand-built `mtt.Config`).

### A3 — no write-concurrency contract: last-writer-wins loses updates silently — RISK: the sharpest multi-agent edge

- **Evidence:** every mutation is Get → mutate → `Update`, whole-file write; `Update`'s stat-then-write
  TOCTOU is documented-accepted ([task.go:60-69](../../../internal/adapter/yaml/task.go#L60)). Two
  parallel agents transitioning/tagging the same task: both read, both write — one `history` entry (or
  the status itself) vanishes, **no error**. The port offers no version/CAS token; `ErrConflict` exists
  only in model.go (unshipped).
- **Assessment:** fine for solo / one-agent-per-checkout (the declared s009 mode). It is **not** in
  TASKS.md "Later (think)" yet — unlike its sibling (id reuse). It should be, so the multi-agent
  trigger is on record.
- **Fix sketch (when triggered, not now):** an optimistic-concurrency token on `Update` (e.g.
  compare-and-swap on `Updated` or a content hash) + `ErrConflict`; or per-file `O_EXCL` lock files.
  Decide together with monotonic minting (both are "concurrent agents on one store" items).
- **Acceptance (for the backlog entry):** TASKS.md Later (think) contains a lost-update item naming the
  trigger ("before two writing agents share a checkout/store").

### A4 — ID recycling after `rm` of the max id — confirmed in code (known think-item)

Mint is `max(N)+1` over **current** files ([mint.go](../../../internal/adapter/yaml/mint.go)); deleting
the highest-numbered task frees its id; the next `add` re-mints it, and any dangling refs left by
`rm --force` ([remove.go:280-284](../../../internal/core/remove.go#L280) leaves them deliberately)
silently rebind to the unrelated new task. Already documented (DESIGN.md `rm` caveat + TASKS Later
think-item) — this audit just confirms the mechanics. Keep deferred; the trigger ("before dogfooding at
volume / multi-agent") is approaching with s009 — reassess after living with it.

### A5 — stale `current` exits 1, explicit missing id exits 4 — inconsistent not-found signal

- **Evidence:** `staleCurrentErr` ([internal/cli/resolve.go:83-85](../../../internal/cli/resolve.go#L83))
  does not wrap `mtt.ErrNotFound`, so `mtt show`/`edit`/`<status>` via a dangling `current` pointer
  exits **1**, while the same command with an explicit missing id exits **4**. An agent branching on
  exit 4 ("target missing") gets two different signals for the same condition.
- **Fix sketch:** either wrap `ErrNotFound` in `staleCurrentErr` (keeping its actionable message), or
  document the distinction in CLI_REFERENCE's exit-code table as deliberate. Decide; today it is
  neither consistent nor documented.
- **Acceptance:** CLI_REFERENCE exit-code table and behavior agree for the stale-current case.

### A6 — no composition root: 37 inline `yaml.*` constructions across 15 CLI files — bet-#2 debt

- **Evidence:** `yaml.NewTaskStore`/`yaml.Load`/`yaml.NewCurrent` are constructed inline at ~37 call
  sites (edit.go, show.go, root.go, resolve.go, use.go, rm.go, selector.go, …). model.go §9 promises
  "on startup assemble adapters from config and inject into core" — no such seam exists.
- **Why it matters:** the second adapter (bet #2's entire premise) requires touching every command — or
  first building the factory that should already exist. Cheap now, expensive after.
- **Fix sketch:** one `openStores(root) (mtt.TaskStore, mtt.CurrentStore, mtt.Config, error)` helper in
  cli (even still hardcoded to yaml) — a pure refactor collapsing the 37 sites; backend *selection*
  logic comes later with `mtt connect`.
- **Acceptance:** `grep -rn 'yaml\.New\|yaml\.Load' internal/cli | wc -l` drops to ~1–3 sites
  (the helper + init/root special cases); behavior unchanged (`make check` green).

### A7 — the contract references `ErrUnsupported`, which does not exist — dangling godoc promise

- **Evidence:** `TaskStore.Delete`'s godoc says an adapter that cannot hard-delete returns
  `ErrUnsupported` ([pkg/mtt/store.go:22](../../../pkg/mtt/store.go#L22)) — but no such sentinel is
  shipped in `pkg/mtt`, and no `errors.Is` branch exists for it. The first external (archive-only)
  adapter author follows the godoc into a wall. GAP #3's "reserve early" advice is now overdue — the
  contract already depends on the name.
- **Fix sketch:** ship `var ErrUnsupported = errors.New("mtt: operation unsupported by this backend")`
  in `pkg/mtt` + an exit-code mapping decision (likely a distinct code; CLI_REFERENCE table update).
  Tiny, additive.
- **Acceptance:** godoc reference resolves; `exitCode` handles it deliberately (even if to generic 1,
  as a recorded decision).

### A8 — undocumented port invariants that will burn the first external adapter

Three invariants live in core/adapter internals but are **not stated on the `pkg/mtt` port surface**:

1. **`Task.Tags` must round-trip as a normalized + deduped + sorted set** — the invariant lives in
   core's `canonicalTags` and the yaml CLAUDE.md; a Jira-labels adapter returning raw labels breaks
   tag filtering/reconciliation silently.
2. **Timestamps are truncated to whole seconds UTC** — `.UTC().Truncate(time.Second)` is repeated in
   6 core usecases; this is YAML's disk-format policy leaked into core. A millisecond-native backend
   gets silent precision loss and `Updated`-comparison surprises. (Also a DRY nit: 6 copies — a
   `mtt.NowFunc`-adjacent helper would centralize it.)
3. **`Create` silently overwrites a non-empty incoming `Task.ID`**
   ([task.go:40](../../../internal/adapter/yaml/task.go#L40)) — the "ID must be empty in" precondition
   is unenforced and unstated.

**Fix sketch:** state all three in the port godocs (`TaskStore.Create/Update`, `Task.Tags` field doc);
optionally reject a non-empty incoming ID in `Create`. Doc-only is enough for now.
**Acceptance:** `go doc mtt.TaskStore` states the tags-set, timestamp-precision, and empty-ID-in
contracts.

### A9 — GAP #6's revisit trigger silently passed (derived-graph construction ×4)

The three traversals (`Index` upward walk, `DepGraph` DFS/cycles, `Roadmap` two-axis Kahn) genuinely
differ — the reuse-refusal is defensible. But **construction** is creeping duplication: `NewIndex`
([index.go:181-205](../../../internal/core/index.go#L181)) and `NewDepGraph`
([depgraph.go:22-42](../../../internal/core/depgraph.go#L22)) are ~20 near-identical lines, and the
byID map is rebuilt again in `Roadmap` and `Ready`. The documented revisit trigger ("a third graph")
never fired formally because `ResolvedFlow` was never built — yet Roadmap (s008.6) *was* the third
graph. No urgent action; log the revisit at the **next** graph consumer (extract a shared
`byID`/reverse-bucket builder then).

### A10 — sugar classification swallows operational errors

`trySugar`/`trySugarCurrent` ([internal/cli/root.go](../../../internal/cli/root.go)) convert
`projectRoot`/`yaml.Load`/`Current()` failures into `false, nil` → "unknown command". A corrupt config
thus masquerades as a typo. Deliberate decline semantics per comments — but consider distinguishing
"could not even load the project" (surface the real error) from "arg0 is not a status" (decline).
Low severity; an agent debugging a broken config gets a misleading signal.

### A11–A13 — recorded smells (low, mostly cosmetic)

- **A11:** `clearCurrentIfMatches` in [internal/cli/rm.go:71-80](../../../internal/cli/rm.go#L71) — a
  domain-ish consistency rule ("a delete must not leave a dangling current pointer") at the composition
  root; consistent with the documented CurrentStore decision, but it is a nontrivial invariant outside
  core. Revisit if a second such rule appears (same revisit-at-second-caller policy as `advance`).
- **A12:** the adapter tightens "≤1 default type (domain)" to "exactly one (YAML provider)"
  ([dto.go:150-160](../../../internal/adapter/yaml/dto.go#L150)) — declared provider policy, fine; noted
  because it changes `mtt add` behavior per provider.
- **A13:** name-agnostic cosmetics: help example `mtt tag add urgent --status tbd`
  ([internal/cli/tag.go:29](../../../internal/cli/tag.go#L29)) bakes a template status name into an
  example (behavior is config-driven — doc-only); internal comment "epic → task → subtask" in
  [tree.go:15](../../../internal/cli/tree.go#L15). Also trivial: `ErrNotFound` message is
  `"mtt: task not found"` vs model.go's anticipated KB-reusable `"mtt: not found"` — reconcile whenever
  the KB lands; model.go §7 renders `Index` as an interface while §8 says Layer B is concrete structs
  (snapshot self-inconsistency, one-line fix in model.go).

---

## Priorities

| when | items | rationale |
|---|---|---|
| **before writing-plans for s009** | S1 (reconcile flow decision), S2 (idempotent branch cmd, no dead rollback), S3 (migration priorities + hand-run roadmap), S5 (gate-cost paragraph) | they change what the plan builds |
| **start of s009 (enablers)** | S8/U2 (blocked-gate output), A1 step 1 (wrap List/Get errors with file path), A2 (`Status.Default` DTO mapping), S7 (`rm -rf bin/.mtt`) | first-week dogfood pain; all tiny |
| **with s009 docs sync** | S4 (who-commits-.mtt paragraph), S6 (guard-test scope note), A3-acceptance (add the lost-update think-item to TASKS Later) | doc-only |
| **pre-`v0.9.0`** | A5 (stale-current exit code decision), A7 (`ErrUnsupported` sentinel) | contract surface a release freezes |
| **before the first external adapter (bet #2)** | A6 (composition root), A8 (port-invariant godocs) | cheap now, expensive after a second adapter exists |
| **log only / revisit later** | A4 (id recycling — trigger approaching), A9 (graph-builder extraction at next consumer), A10–A13 | recorded, no action now |
