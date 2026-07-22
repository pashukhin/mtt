# internal/cli

The CLI layer on cobra. **Only** flag/argument parsing, wiring adapters from config, calling `core`
(usecases), and formatting output. Thin by definition.

## Boundaries

- NO business logic; storage only through a port — logic in `core`; a pure read (e.g. `show`) may call a
  `TaskStore` method directly, without a `core` usecase.
- One command = one file `<cmd>.go` with a constructor `new<Cmd>Cmd() *cobra.Command`.
- Commands return errors via `RunE` (they don't print errors themselves or call `os.Exit`).
- Output only through `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` (testability).

## Tests

e2e via `testscript` (txtar) in temp dirs; one script per command. `TestMain`
(`script_test.go`) **scrubs `MTT_DIR`/`MTT_BY`/`MTT_ROLE`** (`mttEnvVars`) in the
harness process only — a value exported in the caller's shell would otherwise
override the in-process tests' cwd-discovery/attribution and (via `init`
resolving to `$MTT_DIR`) scatter a stray `.mtt` that poisons other packages'
discovery; the scrub is gated by `inMttCommandSubprocess` so the re-invoked `mtt`
command keeps the per-script `env MTT_DIR=…` (c4).

## Current state

`root` + `version` + `init` + `types` + `add` + `show` + `list` + `edit` + `tree` + `dep` + `ready` +
`status`, plus the root persistent flags `--dir`/`MTT_DIR`, `--version`, `--json`, and (session 006)
`--role`/`MTT_ROLE` + `--by`/`MTT_BY` (the history seams, resolved by `resolveRoleBy`). `projectRoot(cmd)` resolves the root (--dir/MTT_DIR else
FindRoot) and DRYs the former `Getwd → FindRoot`; `baseDir` does the same for `init` (no .mtt required).
`list` composes `TaskStore.List` → `core.Select` (pure read: filter/order in core, no usecase; loads `cfg`
for the `--kind`/`--parent` filters) and renders human text or, with `--json`, a `taskJSON` array; `edit`
goes through `core.Editor` (a mutation) and prints `updated <id>` or the JSON object. `show`/`list`/`edit`
honor `--json` via the `taskJSON` view.

Versioning (t30): `mtt version`, the `--version` flag, and `version --json` all call `resolveVersion()` (`version.go`) —
the pure, tested `resolve(ldflags, buildVersionFn)` prefers the ldflags-injected `version` (defaults to
`"dev"`; **no committed version number**) → the `runtime/debug` build-info module version (set by
`go install …@vX.Y.Z`) → `"dev"`. The Makefile stamps a `git describe` string for dev builds and the
explicit `VERSION` for `release` (SemVer / tag-as-SoT; see [RELEASING.md](../../RELEASING.md)).

Hierarchy (session 004): `add --parent <id>` (mutually exclusive with `--no-parent`) routes placement
validation through `core.Adder`; `tree [<id>]` builds `core.Index` from `TaskStore.List` and renders an
ASCII tree (`renderTree`) with **keep-ancestors** filtering (`--status`/`--kind`), `--depth`, and a nested
`--json` (`buildTreeJSON`); `show` prints the lineage breadcrumb from `Index.Ancestors`. `taskLine` is the
shared one-row formatter (list + tree); `parseKinds` validates `--kind` against the `StatusKind` vocabulary
(shared by `list` + `tree`). Pure reads (`tree`/`show`) call the store directly — no usecase.

Dependencies & ready (session 005): `dep add/rm <id> <dep-id>` route through `core.DependencyEditor`
(self/cycle rejected; add and rm both idempotent — duplicate/absent-edge are no-ops); `dep list <id>` builds `core.DepGraph` from
`TaskStore.List` and renders `depends on:` (dangling → `(missing)`) + computed `required by:`, with `--tree`
(transitive, cycle-safe), `--cycles` (project-wide, defensive), and a non-null `--json`. `mtt ready` and
`list --ready` share one primitive — `core.Select(core.Ready(tasks, cfg), filter, cfg)` — so readiness and
the list filters compose (AND). `toStatusNames`/`toTypeNames` are the shared string→identity converters for
`list`/`ready`. Pure reads (`dep list`/`ready`) call the store directly; mutations (`dep add/rm`) go through
`core`. **(c12)** `depends_on` is part of the agent contract: `taskJSON.DependsOn` (`json:"depends_on,omitempty"`,
via `idStrings`) rides every task-object emission (`show`/`list`/`add`/`edit`/`rm`/`status`/`tag` `--json`),
and human `show` prints a `depends:` line — `dependsEntries(cfg, tasks, task)` (in `show.go`) renders each
blocker as `<id> [<status>]` with `✓` on a terminal (satisfied) one and `(missing)` on a dangling one
(unresolvable kind → no mark, mirroring conservative readiness); `formatTask` takes the pre-rendered slice.

Flow gate (session 006): `mtt status <id> <new>` wires `yaml.Load` (→ `Settings`) +
`exec.NewRunner(root, timeout, progress, cmdOut)` + `core.Transitioner`; `--no-run` bypasses the gate.
Gate execution reports **live pipeline progress** (`▶`/`✓`/`✗` + timing) to **stderr** always; the
commands' own output is hidden by default, streamed to stderr with `-v`/`--verbose`, and/or written to a
file with `--log-file` (`gateOutputWriter` builds the `io.Discard`/stderr/file/`MultiWriter`). **Blocked-gate
visibility (s008.97/U2):** when output is otherwise hidden (`hidden = !verbose && logFile==""`), `runTransition`
passes `gateTailLines` (10) to `exec.NewRunner` so the runner echoes the failing command's output tail, and
wraps an `ErrBlocked` error with a `hint: re-run with -v or --log-file …` (the `%w` wrap preserves exit 3);
with `-v`/`--log-file` neither fires (output already visible). **Post-persist (t21):** `runTransition` holds
the transition error in `txErr`; on `core.ErrPostAction` (the move IS persisted — only the post phase failed) it
**falls through** to render the move (a **local `e`** for the `Fprintf` writes, never `txErr`, or a successful
write would clobber the sentinel and lose exit 5). **Actionable recovery (t28):** it renders the move **first**
(`--json` task object on stdout, else the move line + guidance), then `errors.As`-extracts the typed
`*core.PostActionError` and prints a recovery block on stderr in **both** modes — `the status change IS saved;
do NOT re-run the move` + the exact remaining `post:` commands (`pe.Remaining`), one per indented line. Printing
it **after** the render keeps the order `move-line → recovery → Execute's error:` line (not an inverted "move
applied" before the move); the cause is left to `Execute`'s `error:` line (no dup). The old terse
`move applied, but a post-action failed: …` echo is gone. **`Execute()`
returns an `int` exit code** (`exitCode`: `core.ErrBlocked`→3, `core.ErrInvalidTransition`→6,
`core.ErrMissingAttribution`→2, `core.ErrPostAction`→5, else 1) and, before returning, prints the context-free
**`exitHint(err)`** block (`errors.go`) under the `error:` line — exit 2 → how to set who/why, exit 4 → point at
`mtt roadmap`/`mtt list` (`""` for the context-carrying sentinels, no bleed); `main` and the testscript harness
call `os.Exit(Execute())`. `mtt types` renders a `⇢` post line per edge. `mtt show` renders a `history:` audit section.

Attribution + verb sugar (session 006.5): `runTransition(cmd, root, cfg, settings, id, to, noRun)` is the
shared gated-edge path used by **both** `mtt status` and the sugar; `resolveAttribution(cmd, author)` returns
`role/by/why` — `by` is `--who`/`--by` (mutually exclusive, else error) → `MTT_BY` → `Settings.Author`; `why`
is `--why`; both ride into `core.TransitionOptions` along with `settings.Require.{Who,Why}`. **Verb sugar**
`mtt <status> <id>` is `root.RunE` (`runSugar`/`trySugar`): with exactly 2 args where arg0 is not a registered
command (cobra dispatches real commands first), it routes to `runTransition` iff arg1 is an existing task and
arg0 is a status in that task's type flow (`Type.StatusKind`); any classification miss → `unknown command`
(exit 1); `mtt` with no args → help. `--who`/`--why`/`-v`/`--verbose`/`--log-file` are **root-persistent** (the
sugar inherits output control); `--no-run` stays **local to `mtt status`** (the sugar cannot bypass the gate).
`mtt show` renders the reason as `why "…"` in the history line.

Current task / working context (session 006.7): `mtt use [<id>] [--clear]` sets (`use <id>`, validates existence),
shows (`use` → one `taskLine`, else `no current task`), or clears (`use --clear`) the personal current pointer
via `yaml.NewCurrent(root)` (the `mtt.CurrentStore` port). `resolveTaskID(root, explicit)` (in `resolve.go`)
resolves an **omitted id** to the current task for single-task verbs only — `status` (now 1-or-2 args), the
`mtt <status>` sugar (1-arg `trySugarCurrent` on the current task; falls through to `unknown command`, or a
helpful "no current task" when arg0 is a plausible status), `show`, and `edit` (all `MaximumNArgs(1)`); **never**
for `list`/`tree`/`dep`/`ready`. Order: explicit id > current; a stale/absent current gives an actionable
error (validated at the point of use). `applyCurrent(root, cfg, task, id)` (in `status.go`) moves the pointer
after a successful `runTransition` by reading the traversed edge's `Current` via `Type.FindTransition` —
`core.Transitioner` is untouched (the CLI applies the flow-declared set/clear).

Structured commands (session 007): no CLI wiring change — the runner is still `exec.NewRunner(root,
settings.CommandTimeout, …)` (the global is now the **per-command fallback**), and `core.Transitioner`
expands placeholders before the gate. The one CLI touch is `mtt types` (`formatTypes`): a command renders as
`$ <run>` plus `  (timeout <d>)` when the command carries a per-command timeout.

Dogfood enablers (session 008.5): `mtt rm` routes through `core.Remover`
(reject-if-referenced; `--force` deletes despite refs); a **single explicit id** takes no current resolution
(destructive); after a successful delete it clears the `current:` pointer if it named the deleted task
(`yaml.NewCurrent`, now `clearCurrentIfMatches`). **(Since s008.9 `rm` is `ArbitraryArgs` — single vs bulk;
see the Batch paragraph.)** **Dangerous-ops (t5):** both `rm` paths now `yaml.Load` (for `settings.Author`),
`resolveAttribution(cmd, author)` (→ `by`/`why`; `--who`/`--why` are root-persistent so `rm` inherits them),
and wire `core.NewRemover(store, yaml.NewAuditStore(root), time.Now)`. A `--force` without who+why is a
**pre-flight** `ErrMissingAttribution` — `RemoveMany`'s **error return**, forwarded **raw** by both paths
(never through `reportBulk`, whose plain `fmt.Errorf` would flatten to exit 1), so it maps to **exit 2** on
single *and* bulk. `exitCode` now maps `mtt.ErrNotFound → 4`, applied **uniformly**: `taskNotFound(id)`
(`errors.go`) wraps `ErrNotFound` and is used by `show`/`edit`/`tree`/`use`/`dep` (core wraps it in
`transition`/`dependency`/`add`), so every single-task not-found exits 4. `mtt add --depends-on <id>…`
(StringSlice, repeatable/csv) → `AddParams.DependsOn` (validation in `core.Adder`).

Rollback / compensation (session 008): still no wiring change — `core.Transitioner` (via `Runner.Compensate`,
implemented by the same `exec.Runner`) runs a blocked gate's compensators; the `↩ compensating (N)` phase and
per-compensator `▶`/`✓`/`✗` lines come from the runner on the existing stderr progress writer, and the block
error already carries the `compensated N …` summary (surfaced by `Execute` → stderr, exit 3). The one CLI touch
is `mtt types` (`writeTypeBlock`): under a command, a `↩ <rollback.Run>` line (+ `  (timeout <d>)`) when the
command declares a compensator.

Priorities + roadmap (session 008.6): `--priority high|medium|low` on `add` (→ `AddParams.Priority`) and `edit`
(→ `EditParams.Priority`; `--priority ""` clears — `Changed("priority")` is true), and repeatable `--priority`
+ `--sort priority` on `list`. The shared `parsePriority`/`toPriorities` (`priority.go`) validate at the CLI
boundary (`!Valid()` → usage error; never leak a bare string into `core`). `mtt show` prints a `priority:` line
(omitted when unset); `taskJSON` gains `priority` (`omitempty`), so it is readable via `show`/`list --json`.
**`mtt roadmap [--json]`** (`roadmap.go`) is a pure read — `TaskStore.List` → `core.Roadmap` → render:
`writeRoadmap` numbers entries (`N. <id>  [<priority>]  (<status>)  <title>`, `[..]` omitted when unset, `  ↳
blocked by: …` under a depends_on-blocked one and `  ↳ contains: …` under a parent), and
`roadmapJSON`/`toRoadmapJSON` emit `{id,title,status,priority,ready,blocked_by,contains}` with `priority` the
**stored** value (`""` when unset, not omitempty — honest) and `blocked_by`/`contains` always non-null arrays
(`[]` when empty, via the shared `idStrings` helper). Display echoes the stored priority — the *ordering* treats
unset as medium (and propagates it up the blocker chain), the *label* is never fabricated. Ordering is
`core`'s concern (two axes — depends_on + parent — with priority propagation); the CLI only renders.

Tags (session 008.7): `mtt tag add/rm <id> <tag>…` (variadic; `tag.go`) route through `core.TagEditor`
(`runTagEdit` shared path); a not-found id maps to exit 4 (the editor wraps `ErrNotFound`), the `rm` guard
surfaces as a plain error (exit 1). `--tag`/`--exclude-tag` are **`StringSliceVar`** (t50: comma-split **and**
repeatable — `--tag a,b` or `--tag a --tag b`, tool-wide incl. the `selector.go` `--filter` tag and its
`GetStringSlice` reader; the non-tag filter flags stay `StringArray`) on `add` (→ `AddParams.Tags`),
`list`, `tree`, and `ready` (→ `ListFilter.Tags`; `ready` in c10); the shared `toTags` normalizes/validates each value at the boundary
(`mtt.NormalizeTag`; invalid → usage error) so no bare string leaks into `core`. Text `#hashtags` are handled
in `core` (Adder/Editor), not parsed in the CLI. `mtt show` prints a `tags:` line (`formatTask`, after
`priority`); `taskJSON` gains `tags` (`omitempty`), readable via `show`/`list`/`edit`/`tag …` `--json`. Tags
are NOT shown in the `taskLine` row (list/tree) — visible via `show`/`--json`/the `--tag` filter. **`--exclude-tag`**
(c8, repeatable) on `list`, `ready`, **and** `tree` (`tree` in c10; → `ListFilter.ExcludeTags`, same `toTags`
boundary) is the negative filter: reject any task carrying one of the tags; composes with `--tag` as AND
(overlap → exclude wins). c10 closes the tag-filter symmetry — `list`/`ready`/`tree` all take **both** `--tag`
and `--exclude-tag`. Enables `mtt ready --exclude-tag backlog`. **`mtt tags`** (c9, `tags.go` `newTagsCmd`) is a pure read (like
`roadmap`/`tree`): `TaskStore.List` → `core.Select` (same `ListFilter`; default scope = open
`initial`+`active` kinds, suppressed by a status-scoping flag — `--all`/`--kind`/`--status`) → `core.TagCounts` → `count  tag` rows
(most-used first) or a `[{tag,count}]` array (`tagCountJSON`) under `--json`. Registered as `tags` (distinct
from the `tag` mutation command).

Batch & pipeline (session 008.9): a reusable **task-set selector** (`selector.go`) — `selectTaskIDs(cmd,
positional, allowExplicitIDs)` resolves ONE of three mutually-exclusive sources: explicit ids | stdin `-`
(`readIDsFromStdin`) | `--filter` (the shared `addSelectorFilterFlags`/`readSelectorFilter`/`filterActive`
over `core.Select`/`Ready`). >1 or 0 active sources is a usage error; a present-but-empty source is a no-op
(exit 0); dedup + first-occurrence order; **never** resolves `current`. `writeIDs`/`idsOf` back `--ids` on
`list`/`ready` (one id per line, `⊕ --json`). Bulk mutations share `bulk.go`: `runBulk(cmd, ids, verbPast,
apply)` (best-effort per item, `reportBulk` summary on stdout + per-item errors on stderr, `--json` per-item
array, a plain aggregate `fmt.Errorf` — no `%w`, so exit 1 not a per-item sentinel) and `previewBulk`
(`--dry-run`: ids to stdout + stderr summary, no mutation). **`rm` is `ArbitraryArgs`**: a single explicit id
keeps `runRmSingle` (verbatim `removed <id>`, exit 4); multi/`-`/`--filter` → `core.Remover.RemoveMany`
(subgraph-aware), then `reportBulk`, clearing `current` per deleted id. **`tag add/rm` is marker-driven**
(`tagArgs`): no marker → the single `applyTagSingle` path (`<id> <tag>…`, back-compat); a `-` or a filter flag
→ bulk (positionals are the tags, tasks from the selector, per-item `TagEditor` via `runBulk`). On `tag`, the
`--tag` flag is the tag **filter** (distinct from the positional tags being added/removed).

Flow guidance (session 008.95): `guidance.go` turns the flow's authored `description`s into inline agent
instructions. `moveGuidance(cfg, type, from, to)` builds the block printed on **stdout** after a successful
`runTransition` (status + sugar, text mode only — the `--json` move stays the task object): the traversed
edge's `Description`, the destination status's `Description` (each `▸ …`), and `next: …` (onward moves via
`formatNextMoves`). A blocked move prints nothing (no entry into the status). `mtt show` calls
`statusGuidance(cfg, task)` (→ current status `Description` + `TransitionsFrom`) and renders it in the human
block (a `▸` line + `next:` under the header via the extended `formatTask`) and in `--json` via `showJSON`
(`toShowJSON` — **anonymously embeds `taskJSON`** so `list`/`edit`/`status --json` are byte-unchanged, adding
`status_description`/`next` as `omitempty`). The pure `pkg/mtt` helpers `Type.StatusByName` /
`Type.TransitionsFrom` back both paths (mirroring `StatusKind`/`FindTransition`). **Placeholder expansion
(t16):** every shown description runs through `core.ExpandText` (best-effort — raw on error) so `{{.ID}}`/
`{{.Type}}`/`{{.From}}`/`{{.To}}` become concrete; the helpers thread `id`/`type` (`moveGuidance(cfg, id,
type, …)`, `formatNextMoves(onward, id, type)`), edge descriptions use the edge's `from`/`to`, and status
(node) descriptions use `From=To=status`. **`--json` too:** `toShowJSON` expands `next[].description`
(`status_description` comes already-expanded from `statusGuidance`), so an agent reading `--json` gets no raw
`{{.ID}}`. `mtt types`/`writeTypeBlock` renders the flow **schema** (no task in scope) and stays raw.

JSON surfaces (session 008.97/U3): `mtt add --json` emits the created task via the shared `toTaskJSON`
(instead of the plain `created <id>`), so an agent reads the fresh id from JSON. `mtt show --json` gains a
`history` array (`historyJSON`/`checkJSON` in `json.go`, `omitempty`) built by `toShowJSON` from `Task.History`
— entries `{at, by?, role?, why?, from, to, checks?:[{cmd, exit}]}`, surfacing the checks + attribution the
human view renders. History rides `showJSON` only (embedded `taskJSON` stays lean, so `list`/`edit`/`status
--json` are unchanged).

Named transitions / edge-verb (session 008.98): a transition's optional `Name` gives a semantic verb for the
edge out of the current status. **`mtt do [<id>] <edge>`** (`do.go`, `newDoCmd`) resolves the named edge via
`Type.FindTransitionByName(task.Status, edge)` → its `To` → the shared `runTransition` (gate/attribution/`--json`
inherited); edge-name-**only** (no status fallback); a miss is `doMissError` (wraps `core.ErrInvalidTransition`
→ exit 6, lists `availableActions`). The **sugar `mtt <edge> [<id>]`** rides `classifyStatusMove`, which now
tries `FindTransitionByName(task.Status, arg0)` **before** the target-status classification (disjoint namespaces
make it safe). `edgeNameInAnyFlow` (`resolve.go`, beside `statusInAnyFlow`) lets the "no current / missing task"
branches treat a bare edge verb as plausible (claim with an actionable error vs "unknown command"). Discoverability:
`writeTypeBlock` prints `[name] from -> to`, `formatNextMoves` prints `name → to`, and `nextMoveJSON.Name`
(omitempty) carries the verb in `show --json`. `core.Transitioner` is untouched (route-by-`to`).

JSON consistency (t45): `types`/`version`/`init`/`rm`-single/`use` now honor `--json` — `types_json.go` holds
the flow-graph views (`typeJSON`/`statusJSON`/`transitionJSON`/`commandJSON`/`rollbackJSON`/`requireJSON`;
`require` is a `*requireJSON` so `omitempty` works — Go ignores it on a struct value; timeouts are
`Duration.String()`; the type mapper takes the prefix from `settings.Prefixes`, not `mtt.Type`). `version`/`init`
views (`versionJSON`/`initJSON`) live in `json.go`; `rm`-single captures the task via the store **before**
`Remove` (which returns only an error) and emits `toTaskJSON`; `use --json` emits the current task or `null`
(`writeJSON(w, nil)`). The root **`--version` flag** is a manual `root.Flags().Bool("version",…)` handled in
`runSugar` (JSON-aware) — **not** cobra's built-in `Version` field, which short-circuits before RunE and would
ignore `--json`; so `mtt --version --json` and `mtt version --json` agree. Every mtt command now emits JSON
under `--json`; cobra `completion`/`help` are exempt.

Discoverability + tagline (session 008.97/U4/U5): the root `Short:` names the gate/state-machine (the empty
niche, not "file-backed tracker") and a root `Long:` documents the `mtt <status> [<id>]` sugar + current
resolution + the `roadmap`/`ready`/`types` entry points; `status`'s `Use:` is `status [<id>] <new-status>`
(the id is optional) with a `Long:` covering the sugar. `projectRoot` appends `(run 'mtt init' to create one)`
to **both** no-project errors — the explicit `--dir` case (inline) and the discovery case (wrapping
`yaml.ErrNotInitialized` with `%w`, so `errors.Is` still matches — the CLI, not the adapter, owns the hint).

Knowledge base (t47): `newNoteCmd` (`note.go`) is the `mtt note` group (the `dep` parent+subcommands pattern) —
`add <slug>`/`list`/`show <slug>`/`edit <slug>`/`rm <slug>`, each wiring `yaml.NewKnowledgeStore(root)` +
`core.NewNoteAdder`/`NewNoteEditor`/`SelectNotes`. Slugs go through `mtt.NewNoteSlug` at the boundary (never a
raw cast); tags reuse the shared `toTags`. Body input is `--body`/`--file` (`--file -` = stdin) via
`readNoteBody` (mutually exclusive). `edit` uses `Changed()` (touch only provided fields; `--tag` replaces the
set). `noteJSON`/`toNoteJSON` is the `--json` view (`slug` always, `tags` non-null `[]`); read+`add`+`edit`
honor `--json`, `rm` captures-before-delete for its `--json`. `noteNotFound(slug)` (`errors.go`) wraps
`mtt.ErrNotFound` so a missing slug maps to exit 4 (like `taskNotFound`). Search/versioning are out (t6).

References (t1): `newRefCmd` (`ref.go`) is `mtt ref add/rm/list` over `core.RefEditor` (task carriers);
`newNoteRefCmd` (`noteref.go`) the same over `core.NoteRefEditor` (note carriers, wired under `mtt note`).
`parseRefArg` (`ref.go`) splits `<kind>:<target>` on the **first** `:`, rejects `comment:` (t2) and validates
each target per kind (`NewTaskID`/`NewNoteSlug`/`url.Parse` scheme+host) → usage error (exit 1). On write,
`warnIfNotOK` prints a stderr warning for a **dangling** ref (or note-no-KB) but not a well-formed url;
`verifyOne` resolves against both stores (warn-not-block, exit 0; carrier-not-found → exit 4). `--ref`
(repeatable) on `add`/`note add` is creation-time (`parseRefFlags` → `core.AddParams/NoteParams.Refs`).
`ref list`/`note ref list` render outgoing refs (verified) + computed **backlinks** via the shared
`writeRefsAndBacklinks`; `show`/`note show` append `formatRefsBacklinks` (human) and set `showJSON.Refs/
Backlinks` / `noteShowJSON` (JSON — the lean `taskJSON`/`noteJSON` used by list/edit stay untouched);
`refJSON`/`backlinkJSON` + `verifiedRefsJSON`/`toBacklinkJSON` are the shared views. **`mtt check`**
(`check.go`) sweeps via `core.CheckRefs`, prints findings, returns `core.ErrDanglingRefs` → **exit 7** on any
dangling (0 on clean/unverified); `--json` emits `refCheckJSON` (renamed to avoid the `json.go` `checkJSON`
clash). Deletion guards: `rm` builds the real `Backlinks` (`loadBacklinks`) and passes it to `core.Remover`;
`note rm` gains `--force` + the guard via `core.NoteRemover` (referents from `referentIDs`, which drops the
note's own self-ref), reusing `resolveAttribution` + `yaml.NewAuditStore` (missing who/why → exit 2).
`exitCode` maps `core.ErrDanglingRefs → 7` (unit-tested in `TestExitCode`).

KB prime (t51): `--priority` on `note add`/`note edit` (`parsePriority`; `Changed("priority")` → `*mtt.Priority`
clear on empty, the task-`edit` idiom) and `note list --priority`/`--sort` (`toPriorities` + the validated
`--sort` switch → `core.NoteFilter{Priorities, Sort}`); `noteJSON.priority` (omitempty) + a `note show`
`priority:` line. **`mtt prime`** (`prime.go`, `newPrimeCmd`) is a pure read: `ListNotes` + `List` →
`core.NewBacklinks` → `core.Prime` → `writePrime` (markdown pointer digest, `N of M` footer, empty→actionable
line) or `primeJSON`/`toPrimeJSON` (`tags` non-null). `--min-priority` (default `high`) is validated **inline**
via `mtt.Priority.Valid()` (not `parsePriority`, which treats `""` as valid); `--limit` default 20. The
`sessionStart` hook is config (documented in CLI_REFERENCE), not code.

Self-update (t44): **`mtt self-update`** (`selfupdate.go`, `newSelfUpdateCmd`) wires the current version
(`resolveVersion()`), target (`EvalSymlinks(os.Executable())`), `runtime.GOOS/GOARCH`, `goAvailable`
(`exec.LookPath("go")`), the `github` + `installer` adapters, and `core.SelfUpdater`. `--check-only` /
`--force` / `--json` (`selfUpdateJSON`/`toSelfUpdateJSON` — the pinned `{current,latest,update_available,
updated,via,asset,path,reason,error}`). A **hermetic short-circuit** refuses an unorderable current
(`!core.Orderable`) before any network call when neither `--force` nor `--check-only` is set (so the e2e
`selfupdate.txt` dev-refusal needs no network). `renderSelfUpdate` prints the go-install "different location"
note only when the installed path ≠ the running binary. All failures/refusals map to exit 1 (no new taxonomy
code). Registered in `root.go`.
