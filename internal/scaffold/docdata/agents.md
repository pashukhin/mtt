## Working under mtt

This project tracks its tasks and workflow in **mtt** — a local, file-backed task **state machine**. You do
work by moving a task through its type's flow; each transition can run **gate commands that BLOCK the move on
failure**, so "done" means the checks passed.

**Discover THIS project's flow (don't assume it):**
- `mtt roadmap` — what to work on next (dependency + priority order).
- `mtt ready` — tasks that are unblocked and can be started.
- `mtt types` — the task types with their statuses, transitions, and gate commands (the Definition of Done).
- `mtt show <id>` — a task's details, current status, and the exact next moves available from here.

**The work loop:**
1. Pick a task from `mtt roadmap`.
2. Take it into work using the flow's first move (see the `next:` line in `mtt show <id>` for the exact verb).
3. Do the work, then advance with `mtt <status> <id>` or `mtt <edge> <id>` — one of the moves `mtt show`
   lists. A transition's gate commands run first and BLOCK the move if any fails; read the guidance printed on
   each move.
4. Close a task only by reaching a terminal status through a flow edge. Never delete a task to "finish" it.

**Attribution:** every move records who made it. Set `author:` once in `.mtt/config.local.yaml` (personal,
gitignored), or pass `--who <you>`. Record a reason with `--why "<why>"`; the project may **require** who/why
on some moves and always on dangerous operations (a forced delete, a gate bypass) — mtt tells you when.

**Storage:** `.mtt/` is committed project data. Task files are written **only** by mtt — never hand-edit them;
use `mtt add` / `mtt edit` / the flow moves, and mtt keeps history and determinism.

**Knowledge:** record durable decisions and lessons with `mtt note add`; `mtt prime` prints the important
notes — wire it into your agent's session start.

Run `mtt --help` or `mtt <command> --help` for the full surface. This section is scaffolded by
`mtt agent docs` and is regenerated on re-run — put your own notes OUTSIDE the mtt:begin/mtt:end markers.
