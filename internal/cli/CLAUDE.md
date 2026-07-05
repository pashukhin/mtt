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
file with `--log-file` (`gateOutputWriter` builds the `io.Discard`/stderr/file/`MultiWriter`). `resolveRoleBy`
resolves `role` (flag→`MTT_ROLE`) and `by` (flag→`MTT_BY`→`Settings.Author` from config.local). **`Execute()`
returns an `int` exit code** (`exitCode`: `core.ErrBlocked`→3, `core.ErrInvalidTransition`→6, else 1); `main`
and the testscript harness call `os.Exit(Execute())`. `mtt show` renders a `history:` audit section.
