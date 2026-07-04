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

`root` + `version` + `init` + `types` + `add` + `show` + `list` + `edit`, plus the root persistent flags
`--dir`/`MTT_DIR`, `--version`, and `--json`. `projectRoot(cmd)` resolves the root (--dir/MTT_DIR else
FindRoot) and DRYs the former `Getwd → FindRoot`; `baseDir` does the same for `init` (no .mtt required).
`list` composes `TaskStore.List` → `core.Select` (pure read: filter/order in core, no usecase) and renders
human text or, with `--json`, a `taskJSON` array; `edit` goes through `core.Editor` (a mutation) and prints
`updated <id>` or the JSON object. `show`/`list`/`edit` honor `--json` via the `taskJSON` view.
