# internal/adapter/exec

The default **driven adapter for `core.Runner`** — the first driven port beyond storage. Runs a
transition's `commands` as gates.

## Responsibilities

- `NewRunner(dir, timeout, progress, cmdOut, tailLines)` / `Run(commands []mtt.Command)` — run each command with
  `cwd=dir`, in order, **stopping at the first non-zero exit**. The effective timeout per command is
  `cmd.Timeout` when set, else the constructor `timeout` (the adapter global `command_timeout`) as a
  **fallback** (`context.WithTimeout`) — so a tight per-command timeout fails fast independent of the global
  (s007). Each `mtt.Command.Run` is **already expanded** by `core` (this adapter does not template); records a
  `mtt.Check{Cmd: cmd.Run, Exit}` per executed command (the expanded command — truthful audit). On an
  operational failure the failing command's `Check` is the **last** element (`Exit -1`) — a port CONTRACT
  `core` compensation relies on.
- `Compensate(commands []mtt.Command) []mtt.Check` (s008) — runs already-expanded rollbacks **best-effort**:
  in order, **never stopping**, **never returning an error** (operational failure → `Exit -1`); prints a
  labeled `↩ compensating (N command[s])` header then the same `▶`/`✓`/`✗` per-command lines. `runReport` (the
  per-command run+report+timing) is shared by `Run` and `Compensate` (DRY).
- **Two output streams, separate concerns.** `progress` (always) gets the live pipeline lines
  `▶ <cmd>` / `✓|✗ <cmd> (exit N, <elapsed>)` — per-command wall-clock timing, display-only (not persisted).
  `cmdOut` gets each command's own stdout/stderr (the CLI passes `io.Discard` by default, stderr with `-v`,
  and/or a file with `--log-file`). Nil writers default to `io.Discard`. **`tailLines > 0` (s008.97/U2):** `Run`
  tees a command's output into a bounded ring buffer (`tailBuffer`) and, on a **failure**, echoes the last
  `tailLines` lines to `progress` under the `✗` line — so a blocked gate shows *why* even when output is hidden.
  Hidden-by-default holds for **succeeding** commands (nothing echoed). `Compensate` never echoes a tail. The
  CLI passes `tailLines>0` only when output is otherwise hidden (`!-v && no --log-file`), else `0`.
- A **non-zero exit is data** (a `Check`), not a Go error; the returned `error` signals only an
  **operational** failure (the command could not launch, or timed out — exit recorded as `-1`).
- Cross-platform shell seam `shell(cmd)`: `sh -c` on Unix, `cmd /c` on Windows. Commands are trusted
  project config (like a Makefile), never network input.
- **Process-group kill on timeout (t14/SEC1).** Each command runs as the **leader of its own process group**
  (Unix `Setpgid`); on a timeout the runner `SIGKILL`s the **whole group** (`kill(-pgid)`), so a gate that
  backgrounds a child (`daemon &`, `nohup`) cannot outlive its deadline. `Cmd.WaitDelay` (2s) bounds `Wait`
  against a child that inherited the stdout/stderr pipe (and, as a bonus, closes the former infinite hang of a
  gate that exits 0 but leaves such a child). The group-kill is build-tagged (`procattr_unix.go`); Windows
  (`procattr_windows.go`) is a documented **best-effort no-op** (no POSIX groups; CI-unverified — no runner).

## Boundaries

- No flow logic, no history, no gating decision — `core.Transitioner` decides blocked-vs-applied from the
  returned checks/error. This package only *runs* and *reports*.
- The project root (`dir`) and timeout are injected by the CLI (from `.mtt/` config); this package holds no
  config knowledge.
