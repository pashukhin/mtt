# internal/core

Usecase logic. Depends **only** on the `pkg/mtt` domain contract and its ports — **never** on `adapter/*`.

## Responsibilities

- `Adder` (the `add` usecase): resolve the type (`--type` or the config default), enforce placement
  (`--parent <id>`: the parent must exist — `TaskStore.Get` — and its type must satisfy `AcceptsParent`;
  else a non-root type needs `--no-parent`), pick the entry status (`Type.InitialStatus` — default-marked
  initial, else first initial), stamp `created`/`updated` from an **injected clock**, and persist via
  `TaskStore.Create` (which mints the ID in the adapter). `--depends-on` targets (`AddParams.DependsOn`) are
  validated before `Create` (`resolveDeps`: each `Get`, a missing target wraps `ErrNotFound`) and **deduped**;
  no cycle check is needed (the new task's id is unminted, so it cannot be a target).
- `Match` (pure predicate): reports whether a task satisfies a `ListFilter` — status/type/kind/**priority**/**tags**/**exclude-tags**/parent (AND
  across dimensions, OR within). The **Tags** dimension (s008.7) is slice-valued — a task matches if it carries
  **any** filter tag (`anyOrEmptyIntersect`, the slice analogue of scalar `anyOrEmpty`). **ExcludeTags** (c8) is the
  negative dimension — a task carrying **any** exclude tag is rejected (`intersects`; empty → rejects nothing),
  so it composes with Tags as AND and overlap means exclude wins. (`anyOrEmptyIntersect` now delegates to `intersects`.) `cfg` is consulted only for the kind dimension (resolve the task's status
  category via its type's flow). The `Priorities` dimension matches the **stored** label (an unset task
  matches only when no `--priority` filter is given — filtering is on the authored label, not the ordering
  default). Shared by `Select` **and** the CLI's `tree` walk (one predicate, two consumers — DRY).
- `TagCounts` (pure read, `tags.go`, c9): tallies tags across a task set → `[]TagCount{Tag, Count}` sorted by
  Count desc then Tag asc (each task's `Tags` is a normalized set, so it contributes at most once per tag).
  Backs `mtt tags`; the CLI applies the scope/filters via `Select` first, then counts.
- `Select` (pure read): `Match`-filter tasks, then impose a deterministic order — `Created` desc by default
  (or `Updated`), tie-broken by ID as an **opaque string** (never parsing ID structure; provider-agnostic).
  `SortPriority` (s008.6) orders by `Priority.Rank` asc (high first), tie-broken by the shared
  `lessByRecency` (`lessByPriority` falls back to it — DRY). No store injected — a pure function
  (`Select(tasks, ListFilter, cfg)`) the CLI composes with `TaskStore.List`. The sibling comparator
  (`lessByRecency`) is shared with `Index`.
- `Index` (pure derived hierarchy): built from a task slice (`NewIndex`) — no store, no clock; **not** part
  of the `pkg/mtt` contract (the resolved graph is derived). Exposes `Roots`/`Children`/`Ancestors`/`Get`;
  children are **computed** (inverse of `parent`), never stored; orphans (dangling parent) surface as roots;
  `Ancestors` is cycle-safe (visited-set). Sibling order matches `Select`. Consumed by `tree` and `show`
  lineage (pure reads — no usecase).
- `Editor` (the `edit` usecase, a mutation): load via `TaskStore.Get`, apply only the provided
  title/description/**priority** (nil pointer = unchanged; a non-nil `Priority` to `""` clears it), enforce the
  title-or-description invariant **and the single-line-title rule** (c13, shared `validateTitle` — a newline in
  a title is a usage error, also enforced on `add`; the description stays free-form), bump `updated` from the
  **injected clock**, persist via `TaskStore.Update`.
  The "nothing to edit" guard is `Title==nil && Description==nil && Priority==nil`. id/type/status/parent are
  not editable here.
- `Ready` (pure read): actionable tasks — status non-terminal AND every `depends_on` terminal, resolved by
  category (`kindOf` → `Type.StatusKind`). **Conservative**: a dangling blocker or a config-drift status
  (unresolvable) leaves a task not-ready. No store/clock; ordered by the shared `lessByRecency`. One
  primitive behind both `mtt ready` and `list --ready` (`Select(Ready(...), filter, cfg)`). `kindOf` DRYs
  the type→`StatusKind` lookup shared with `matchesKind`; **`terminalSatisfied`** (s008.6, factored out of
  `isReady`) is the single home for "is this blocker satisfied (present + terminal)?", shared with `Roadmap`.
- `Roadmap` (pure read, s008.6): `Roadmap(tasks, cfg) []RoadmapEntry` — the non-terminal tasks in an execution
  order over **two "comes-after" axes** (a **priority-guided Kahn** over the union graph): `depends_on` (a
  non-terminal blocker precedes its dependent) **and `parent`** (a non-terminal child precedes its parent — a
  parent completes only once its children do). Both axes are **hard**; priority is the **soft** tiebreak and it
  **propagates** — `effectivePriority` gives each node `min(own, min over everything it transitively unblocks
  across both axes)`, so a high task pulls its prerequisites forward (a low blocker of a high task outranks an
  independent medium one). Each `RoadmapEntry` carries `Ready` (core.Ready membership — single source of truth,
  **depends_on-only**) + `BlockedBy` (depends_on not `terminalSatisfied`) + `Contains` (a parent's non-terminal
  children — the parent axis is ordering + annotation, **not** readiness, so a parent with open children can be
  `Ready` yet ordered last). No store/clock, **not** in the `pkg/mtt` contract. It builds its **own**
  non-terminal-restricted graph (**not** `DepGraph`, whose `Dependents` are unfiltered — GAP #6 stays
  unextracted). **Cycle-safe** across both axes (memoized `effectivePriority` DFS; a node in — or downstream of
  — a cycle, including a cross-axis one, is appended best-effort so the function always terminates and returns
  every node). **Not** `list --sort priority` (that sorts by own priority; roadmap propagates).
- `Adder`/`Editor` **tags** (s008.7): `Adder` unions explicit `--tag` (`AddParams.Tags`) with
  `ExtractTags(title/description)` into a canonical set; `Editor` reconciles tags on a text change via a
  **text-delta** (`reconcileTags` — drop tags whose `#hashtag` left the text, add new ones, keep manual tags;
  no provenance, so a text+manual collision drops with the text). `canonicalTags` (normalize+dedup+**sort**,
  `tags.go`) is the single home for the canonical form. The `#hashtag` text is never rewritten.
- `TagEditor` (mutation, s008.7): `AddTags`/`RemoveTags` edit `Task.Tags` and persist via `TaskStore.Update`
  (**no new port** — GAP #1, like `depends_on`). Both idempotent and **both return the tags actually changed**
  (`(mtt.Task, []string, error)`, canonical order, nil on a no-op — c14) so the CLI reports the real effect, not
  the requested set (`subtractTags` computes the added set; `removed` is collected during the filter). `RemoveTags`
  is **guarded** — it refuses a
  tag whose `#hashtag` is still in the title/description (all targets validated before any write — atomic), and
  its `load` wraps `ErrNotFound` (`%w`) so a missing id maps to exit 4. `#hashtags` in title/description are the
  **primary** authoring path; `tag add/rm` is secondary/pointed.
- `DependencyEditor` (mutation): `AddDependency`/`RemoveDependency` edit `Task.DependsOn` and persist via
  `TaskStore.Update` (**no new port** — the edge rides the field, like `parent` in s004). Rejects a
  self-edge and, via `DepGraph.Reaches`, any edge that would create a **cycle**; a duplicate add is an
  idempotent no-op; removing an absent edge is likewise an idempotent no-op (the task must exist). Bumps
  `updated` from the injected clock on a real change.
- `Remover` (the `rm` usecase, a mutation): `RemoveMany(ids, force) []RemoveResult` is the primary method —
  best-effort per id (a missing/rejected id doesn't stop the rest); existence is checked per id via `store.Get`
  (keeping the not-found / load-error wordings), while `Index`+`DepGraph` are built **once** from a single
  `List` snapshot for the referenced-check. **Subgraph-aware (s008.9):** `externalReferencingIDs` counts only
  referents **OUTSIDE** the id set, so deleting an epic + its children in one call needs no `--force` (an
  in-set child/dependent does not block). `Remove(id, force, by, why)` is a thin **wrapper** over
  `RemoveMany([id])` (set={id} ⇒ every referent external ⇒ identical single-id reject/exit-4 semantics).
  `--force` leaves dangling refs, which the system tolerates (`Ready` conservative; `Index`
  orphans→roots). The task-set **selector** and the bulk report are CLI concerns (no core surface).
  **Dangerous-ops (t5):** `NewRemover(store, audit, now)` takes an `mtt.AuditStore` + injected clock;
  `RemoveMany(ids, force, by, why) ([]RemoveResult, error)` — under `force` it FORCES who+why as a **pre-flight**
  precondition (missing → `ErrMissingAttribution` as the **error return**, nothing deleted, the CLI forwards it
  raw → exit 2, never via `reportBulk`), and per id it **appends the audit record BEFORE `store.Delete`** (a
  failed append leaves the task intact — no destruction without a trail). `force=false` is the unchanged
  reject-if-referenced path (no who/why, no audit). **(t1)** `Remove`/`RemoveMany` take a `Backlinks`;
  `externalReferencingIDs` adds incoming **refs** (`bl.To(RefTask, id)`) to the children+dependents referents —
  a **note** carrier is labelled `note:<slug>` and never in the deletion set (so it always blocks); task
  carriers keep the subgraph-ignore. `core.Remover` gains **no** KB port — the CLI builds the cross-store value.
- **Refs (t1, `ref.go`/`backlinks.go`/`refedit.go`/`noteremove.go`):** pure ref-set algebra (`canonicalRefs`
  dedup-by-`(kind,id)`-last-wins + sort, `upsertRef`, `removeRef`) + `RefStatus`/`VerifyRef` (capability-aware:
  `task` via a task existence fn, `note` via a note fn or `unverified` when nil, `url` always `unverified`).
  `NewBacklinks(tasks, notes)` is the **computed cross-store** inverse index (`RefKey → []Referent`, sorted;
  never stored) backing `check`/`show`/the delete guard; `CheckRefs(tasks, notes, kbWired)` returns the non-ok
  findings (dangling+unverified) deterministically (`ErrDanglingRefs` → CLI exit 7). `RefEditor`/`NoteRefEditor`
  (upsert/remove on `Task.Refs`/`Note.Refs`, bump `Updated`, idempotent absent-remove) and `NoteRemover`
  (refuse-by-default on caller-supplied referents; `--force` forces who/why + audit before `DeleteNote`) mirror
  the task mutations. `Adder`/`NoteAdder` accept creation-time `Refs` (canonicalized, guarded nil-when-empty).
- **KB prime (t51, `prime.go`):** `Note.Priority` (reuse the task `Priority` VO) rides `NoteParams`/
  `NoteEditParams` (the `NoteEditor` guard gains `p.Priority == nil`) and `NoteFilter{Priorities, Sort}` — the
  priority filter (stored-label, `anyOrEmpty`) + `--sort priority` fold **into `SelectNotes`** via the shared
  `SortKey`/`lessNotesByRecency` (mirror `Select`/`lessByPriority`; no standalone `SortNotes*`). **`Prime(notes,
  bl, opts) ([]PrimeEntry, int)`** is a pure derived read (like `Roadmap`, not in the contract): eligible ⇔
  an **explicit** priority whose `Rank()` ≤ `MinPriority` (**unset never primed** — the opt-in safety model);
  order = priority band, then **backlink-count desc** (`len(bl.To(RefNote, slug))`), then recency; `Limit`
  caps; the second return is `total` (eligible before cap). No `Body` on `PrimeEntry` — bodies never leave core.
- **`EventEmitter` (t66, `event.go`):** runs the config-authored lifecycle-event pipelines (post-only) after
  a successful mutation. Fired by the mutating USECASES — `Adder` (create), `Editor`/`TagEditor`/
  `DependencyEditor`/`RefEditor` (update, only on a real persist — idempotent no-ops fire nothing),
  `Remover` (delete, per id INSIDE the RemoveMany loop — batch-then-fire would let the first auto-commit
  sweep the whole batch), and the four note usecases — never the store (a flow move must not double-fire)
  and never `Transitioner` (an edge has its own post). Nil emitter = inert (tests pass nil). Holds the FULL
  `Config`: `TaskEvent` resolves `task.Type` via `TypeByName` BEFORE expansion and renders the config's name
  (the c15-class `{{.Type}}` guard — a poisoned on-disk `type:` never reaches `sh -c`; a miss = finalization
  failure). Contexts are per-store whitelists (`taskEventContext{ID,Type,Event}` /
  `noteEventContext{Slug,Event}` in `expand.go`; `expandTemplate`/`expandCommands` take `any` data now).
  Event post expands AFTER persist (a create's id is unminted earlier); every failure — template, command,
  drift, audit append — is a `*PostActionError` (mutation kept, exit 5; empty `Remaining` on the
  no-recovery audit-append case). **`EventOptions{NoRun,By,Why}`** rides every mutating method;
  `Preflight()` (called FIRST, before any load/persist) forces who+why under a bypass (exit 2); under NoRun
  the emitter writes the audit skip-record (`action: "<command> --no-run"`) instead of running — whether or
  not a hook is configured; `rm --force --no-run`/`note rm --force --no-run` write ONE combined record at
  the pre-delete moment and skip the emitter call (pin iii); a no-op under bypass writes nothing.
- `Runner` (driven **port**, defined here — the first beyond storage): `Run(commands []mtt.Command)
  ([]mtt.Check, error)` **+ `Compensate(commands []mtt.Command) []mtt.Check`** (s008). Implemented by
  `internal/adapter/exec`, **faked** in tests. A non-zero exit is **data** (a `Check`), not a Go error; the
  error is only an operational failure. `Run` CONTRACT: on an operational failure the failing command's `Check`
  is the **last** element (`Exit -1`) — compensation uses it to locate the failure. `Compensate` runs
  already-expanded rollbacks **best-effort** (in order, never stopping, never erroring; operational failure →
  `Exit -1`). No `dir` param — the exec adapter holds `cwd=root`, so `core` stays free of filesystem paths. Each
  `Command.Run`/`Rollback.Run` is **already expanded** at this boundary (see `expandCommands`); the
  per-command-vs-global timeout resolution lives in the adapter.
- `expandCommands` (`expand.go`, s007, pure; `expandOne`/`expandTemplate` since s008): renders each
  `Command.Run` **and, recursively, `Rollback.Run`** (`text/template`) against `cmdContext{ID, Type, From, To}`
  — the **only** exposed fields, a self-enforcing shape-safe whitelist (`{{.Title}}` or any free-text/typo is a
  template error). Expansion is **eager** (both run + rollback, up front), so a malformed rollback template is
  exit 1 **before** any side effect. Copies `Timeout` through. `pkg/mtt` stays template-agnostic (stores the
  raw template); expansion — and its injection policy — live here.
- `ExpandText` (`expand.go`, t16, pure): the **lenient sibling** of `expandCommands` for **shown guidance**
  (descriptions) — `ExpandText(raw, id, typ, from, to) string` reuses `expandTemplate`/`cmdContext` but returns
  the **raw** string (whole, not per-placeholder) on **any** parse/execute error, because guidance is
  informational and must never break a command (gate commands stay strict). Same four-field whitelist. The CLI
  guidance helpers call it; node (status) descriptions pass `From=To=status`.
- `Transitioner` (the flow-gate usecase, a mutation):
  `Transition(id, to, TransitionOptions{Role,By,Why,NoRun,RequireWho,RequireWhy})` applies a **single** edge —
  a **linear lookup** in `Type.Transitions` (no `ResolvedFlow` yet; it earns its keep in s007's multi-edge
  walk). A miss with no outgoing edges distinguishes a genuine **terminal** (`StatusKind` known) from a status
  **not in the flow** at all (config drift → "not in the … flow", never "it is terminal" — c14). Then (s006.5) it enforces **required-attribution** — the shared `missingAttributionFields(reqWho, reqWhy,
  by, why)` (t5, also used by the `rm --force` pre-flight) aggregates the absent `who`/`why` into one
  `ErrMissingAttribution` **before the gate** (fail fast; `NoRun` does **not** bypass it). The **effective**
  requirement is the union `opts.Require{Who,Why} || edge.Require || NoRun` (t5) — global policy, per-edge
  `Transition.Require`, and a `--no-run` bypass each tighten it; `--no-run` alone forces **both** who+why —
  then **expands** the edge's command placeholders (`expandCommands`, using the **pre-move** status for
  `.From`; an expansion error aborts as a plain error — exit 1, not `ErrBlocked`) and gates via `Runner` (any
  non-zero check → `ErrBlocked`, task unchanged, no history), appends a
  `HistoryEntry` (`from/to/at/by/role/why/checks`), persists via `TaskStore.Update` (**no new port** — history
  rides `Task.History`, GAP #1 rule). **On a block (s008)** it computes the compensation plan from a **single**
  failure index (`firstFailure` for a non-zero check; `len(checks)-1` for an operational error) and runs the
  **succeeded-prefix** rollbacks **in reverse** via `Runner.Compensate` (`rollbacksBefore`); the failed command's
  own rollback is never run. Compensation is **best-effort** and never changes the outcome — still `ErrBlocked`
  (exit 3), task unchanged, no history; the block error carries a `compSummary` (`compensated N …`). **Post-persist
  phase (t21):** after `store.Update`, gated by `!NoRun`, the edge's `Post []Command` run through the **same**
  `expandCommands`/`Runner`/`firstFailure` and the same `{ID,Type,From,To}` context — the finalization phase. A
  post failure returns the **persisted** task with the typed **`*PostActionError`** (t28; `Is()`→`ErrPostAction`,
  so it still maps to exit 5) — the **single** case a non-nil error carries a valid task; move kept, no rollback,
  no history for post checks. The error carries `Remaining` (the unfinished post commands: the failed one + those
  never reached — `runsOf(expanded[i:])`, guarded against an empty `pchecks`) + `Cause`, so the CLI prints exact
  recovery steps. The CLI keeps the move and maps it to exit 5.
  `ErrBlocked`/`ErrInvalidTransition`/`ErrMissingAttribution`/`ErrPostAction` are core
  sentinels (flow/attribution is core policy); the CLI maps them to exit codes 3/6/2/5. Since s006.7,
  `findTransition` **delegates** to the pure `mtt.Type.FindTransition` primitive (single source of truth for
  edge lookup). The **current-task** pointer (s006.7) is *not* core's concern: the RULE is the domain field
  `Transition.Current`, but the CLI applies set/clear after a move — `Transitioner` is untouched.
- `DepGraph` (pure derived graph over `depends_on`, parallel to `Index` over `parent`): built from a task
  slice (`NewDepGraph`) — no store/clock, not in the `pkg/mtt` contract. Exposes `Get`/`DependsOn` (stored
  order, dangling kept)/`Dependents` (**computed** reverse edges)/`Reaches` (cycle-check)/`Cycles`
  (defensive). Cycle-safe (visited-set); sibling order matches `Select`. Kept **separate** from `Index`
  (GAP #6 not extracted — `parent` is a single-parent tree, `depends_on` a multi-edge DAG).

## Identities

- `ListFilter`, `Index`, `AddParams`, and `Editor` use the named `pkg/mtt` identities (`TaskID`/`TypeName`/
  `StatusName`), not bare `string`. `anyOrEmpty[T comparable]` is generic so it serves both status and type
  filters. Conversion to/from `string` happens at the CLI (arg parsing) and the adapter (DTO), never here.
- `Ready`, `DependencyEditor`, and `DepGraph` use `mtt.TaskID` throughout; string conversion stays at the
  cli/adapter boundary.

## Self-update (t44)

`selfupdate.go` — `SelfUpdater.Prepare` (resolve latest via `mtt.ReleaseSource` → a determinate `Plan`
`{State: UpdateAvailable|NoUpdate|Undetermined, Via: asset|go-install|none}`; errors **only** on a
`Latest()` failure, so `--check-only` never fails on a resolvable release) and `SelfUpdater.Apply`
(fetch asset + `SHA256SUMS` → `verifyChecksum` **before** `BinaryReplacer.Replace`, or the `GoInstaller`
fallback). Pure helpers `assetName`/`verifyChecksum`/`isNewer`/`Orderable` (SemVer via `x/mod/semver`,
reusing the CLI-resolved current version). `goAvailable`/`goos`/`goarch` are injected — core never reads
`PATH`/`runtime`.

## Boundaries

- No storage access, no ID minting, no output formatting, no YAML — those live in the adapter / CLI.
- The clock is injected (`now func() time.Time`) for deterministic tests.
- Policy lives here; the pure primitives it composes (`IsRoot`, `InitialStatus`, `TypeByName`, `DefaultType`)
  live in `pkg/mtt`.
