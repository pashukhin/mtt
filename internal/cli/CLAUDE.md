# internal/cli

The CLI layer on cobra. **Only** flag/argument parsing, wiring adapters from config, calling `core`
(usecases), and formatting output. Thin by definition.

## Boundaries

- NO business logic and NO direct storage access — logic in `core`, data behind a port.
- One command = one file `<cmd>.go` with a constructor `new<Cmd>Cmd() *cobra.Command`.
- Commands return errors via `RunE` (they don't print errors themselves or call `os.Exit`).
- Output only through `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` (testability).

## Tests

e2e via `testscript` (txtar) in temp dirs; one script per command.

## Current state

`root` + `version` + `init` + `types` + `add` + `show`. `add`/`show` wire the YAML `TaskStore` into the
`core` add usecase (composition root); `show` formats a task via `formatTask`. Next (session 003): `list`/`edit`.
