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

e2e via `testscript` (txtar) in temp dirs; one script per command.

## Current state

`root` + `version` + `init` + `types` + `add` + `show` + `list` + `edit` + `tree` + `dep` + `ready` +
`status`, plus the root persistent flags `--dir`/`MTT_DIR`, `--version`, `--json`, and (session 006)
`--role`/`MTT_ROLE` + `--by`/`MTT_BY` (the history seams, resolved by `resolveRoleBy`). `projectRoot(cmd)` resolves the root (--dir/MTT_DIR else
FindRoot) and DRYs the former `Getwd → FindRoot`; `baseDir` does the same for `init` (no .mtt required).
`list` composes `TaskStore.List` → `core.Select` (pure read: filter/order in core, no usecase; loads `cfg`
for the `--kind`/`--parent` filters) and renders human text or, with `--json`, a `taskJSON` array; `edit`
goes through `core.Editor` (a mutation) and prints `updated <id>` or the JSON object. `show`/`list`/`edit`
honor `--json` via the `taskJSON` view.

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
`core`.

Flow gate (session 006): `mtt status <id> <new>` wires `yaml.Load` (→ `Settings`) +
`exec.NewRunner(root, timeout, progress, cmdOut)` + `core.Transitioner`; `--no-run` bypasses the gate.
Gate execution reports **live pipeline progress** (`▶`/`✓`/`✗` + timing) to **stderr** always; the
commands' own output is hidden by default, streamed to stderr with `-v`/`--verbose`, and/or written to a
file with `--log-file` (`gateOutputWriter` builds the `io.Discard`/stderr/file/`MultiWriter`). **`Execute()`
returns an `int` exit code** (`exitCode`: `core.ErrBlocked`→3, `core.ErrInvalidTransition`→6,
`core.ErrMissingAttribution`→2, else 1); `main` and the testscript harness call `os.Exit(Execute())`.
`mtt show` renders a `history:` audit section.

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

Dogfood enablers (session 008.5): `mtt rm <id>` (`ExactArgs(1)`, `--force`) routes through `core.Remover`
(reject-if-referenced; `--force` deletes despite refs); requires an **explicit id** (no current resolution —
destructive); after a successful delete it clears the `current:` pointer if it named the deleted task
(`yaml.NewCurrent`). `exitCode` now maps `mtt.ErrNotFound → 4`, applied **uniformly**: `taskNotFound(id)`
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
