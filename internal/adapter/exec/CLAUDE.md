# internal/adapter/exec

The default **driven adapter for `core.Runner`** — the first driven port beyond storage. Runs a
transition's `commands` as gates.

## Responsibilities

- `NewRunner(dir, timeout)` / `Run(commands)` — run each command with `cwd=dir` and a **per-command**
  timeout (`context.WithTimeout`), in order, **stopping at the first non-zero exit**. Records a
  `mtt.Check{Cmd, Exit}` per executed command.
- A **non-zero exit is data** (a `Check`), not a Go error; the returned `error` signals only an
  **operational** failure (the command could not launch, or timed out — exit recorded as `-1`).
- Cross-platform shell seam `shell(cmd)`: `sh -c` on Unix, `cmd /c` on Windows. Commands are trusted
  project config (like a Makefile), never network input.

## Boundaries

- No flow logic, no history, no gating decision — `core.Transitioner` decides blocked-vs-applied from the
  returned checks/error. This package only *runs* and *reports*.
- The project root (`dir`) and timeout are injected by the CLI (from `.mtt/` config); this package holds no
  config knowledge.
