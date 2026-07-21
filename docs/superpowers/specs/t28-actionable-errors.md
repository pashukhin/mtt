# Actionable attribution + post-failure errors (`t28`)

Status: spec (decision record). Type: task (`t28`). Branch: `task/t28`. Tags: `dx`, `release`, `think`.

## Context / problem

mtt's error messages state *what* failed but not *how to fix it*. Two exit paths are the worst offenders for
an agent (or human) driving the flow:

- **exit 2 — missing required attribution.** `core.Transitioner`
  ([transition.go:69](../../../internal/core/transition.go#L69)), `core.Remover`
  ([remove.go:57](../../../internal/core/remove.go#L57)) and `core.NoteRemover`
  ([noteremove.go:31](../../../internal/core/noteremove.go#L31)) all emit
  `mtt: missing required attribution: who, why`. It never says *how* to supply who/why (the
  `config.local.yaml author:` / `MTT_BY` / `--who` / `--why` seams), so a first-move agent is stuck.
- **exit 5 — post-action failed after a persisted move.** `Transitioner`'s POST phase
  ([transition.go:113–120](../../../internal/core/transition.go#L113)) returns `ErrPostAction`; the CLI
  ([status.go:132](../../../internal/cli/status.go#L132)) prints `move applied, but a post-action failed: …`.
  It does **not** say the move is **already saved** (so a naive retry re-does the move) nor **which commands
  still need running by hand** (the `.mtt` commit / branch push the post was supposed to finalize).

There is prior art for the shape of the fix: the **blocked-gate hint**
([status.go:106](../../../internal/cli/status.go#L106)) already wraps a core sentinel (`ErrBlocked`) with an
actionable CLI hint (`re-run with -v or --log-file`). `t28` generalizes that discipline to the
attribution and post-failure paths, and audits the rest of the taxonomy.

**Finding (audited during brainstorm): exit 6 is already actionable.** `core.Transitioner`
([transition.go:63](../../../internal/core/transition.go#L63)) already emits
`… cannot move <from> → <to> (allowed from <from>: <targets>)` via the pure `allowedTargets`, and `mtt do`'s
`doMissError` lists `availableActions`. So exit 6 needs verification/alignment, **not** a new mechanism.

Non-negotiable constraints:

- **Hexagon.** `core` stays policy ("what is wrong"); the CLI owns presentation ("how to fix it, in terms of
  this project's env/config"). `core` must **not** learn about `config.local.yaml`, `MTT_BY`, or flag names —
  those hints live in `internal/cli`. The one `core` change (D3) carries *data* (the unfinished post commands),
  not CLI phrasing.
- **DRY.** A hint that applies to many commands (attribution, not-found) lives in **one** place, not copied per
  command.
- **No exit-code change.** exit 2/4/5/6 stay exactly as mapped by `exitCode`
  ([root.go:180](../../../internal/cli/root.go#L180)); `t28` only enriches the *message*.
- **TDD, KISS, YAGNI.** No new config surface, no new flags.

## User stories

Primary user = the coding **agent** (and human) hitting a wall on a flow move.

- **US1** — When a move is refused for missing attribution, tell me exactly how to set who/why so my next
  attempt works. (`mtt <status>` / `mtt do` / `rm --force` / `note rm --force`)
- **US2** — When a post-action fails after my move persisted, tell me the move **is saved** (don't retry it)
  and print the exact commands I must finish by hand.
- **US3** — When I typo a task id, point me at where to find the real ids.
- **US4** — When I request an impossible transition, show me the moves that *are* allowed. (already true —
  verify.)

## Decisions

### D1 — Generic, context-free hints live in `Execute` (one hook, all commands)

`Execute` ([root.go:55](../../../internal/cli/root.go#L55)) is the single point every command's error flows
through (`error: <err>` on stderr). Add one call there:

```go
_, _ = fmt.Fprintln(root.ErrOrStderr(), "error:", err)
if h := exitHint(err); h != "" {
    _, _ = fmt.Fprint(root.ErrOrStderr(), h)
}
return exitCode(err)
```

`exitHint(err) string` (new, `errors.go`) is a pure sentinel switch returning a trailing hint block (each line
`"  …\n"`) or `""`:

```go
func exitHint(err error) string {
    switch {
    case errors.Is(err, core.ErrMissingAttribution):
        return attributionHint // D2
    case errors.Is(err, mtt.ErrNotFound):
        return notFoundHint     // D4
    default:
        return "" // ErrPostAction (D3) and ErrInvalidTransition (D5) are handled with context, not here
    }
}
```

- It matches **only** the context-free sentinels. `ErrPostAction` / `ErrInvalidTransition` fall to `""` so
  there is **no duplication** with the contextual handling (D3/D5). This is safe because a `*PostActionError`
  (D3) `Is(ErrPostAction)` only — it is neither `ErrMissingAttribution` nor `ErrNotFound`.
- The hint is a **suffix** to the existing `error:` line; the error text itself is unchanged (so existing
  golden/e2e assertions on the message body still hold).

### D2 — exit 2: attribution setup hint

`attributionHint` (a package const, `errors.go`) — a fixed, generic block covering **both** who and why (the
core message already names which fields are missing; the hint shows how to set each):

```
  set 'who': add `author: <name>` to .mtt/config.local.yaml, or `export MTT_BY=<name>`, or pass `--who <name>`
  set 'why': pass `--why "<reason>"`
```

- **Generic on purpose.** It fires for the `require:{who,why}` policy path **and** the dangerous-ops path
  (`--no-run`, `rm --force`, `note rm --force`) — all three wrap `ErrMissingAttribution`, so one hint serves
  all (DRY). It does not try to parse which field is missing (the error body already says `who`/`why`/`who,
  why`); showing both setups is cheap and always correct.
- `author:` is the **durable** default (survives across invocations), so it leads; `MTT_BY` is the
  env/session default; `--who`/`--why` are per-command.

### D3 — exit 5: post-failure recovery with the exact remaining commands

**Core change (the only one): a typed `*PostActionError`** replacing the plain-wrapped `ErrPostAction`, so the
CLI can render the unfinished commands without re-expanding templates (expansion is a `core` concern).

```go
// core (runner.go): typed, carries the recovery data; Is() preserves the exit-5 sentinel mapping.
type PostActionError struct {
    ID        mtt.TaskID
    From, To  mtt.StatusName
    Remaining []string // the post commands that did NOT complete: the failed one + those after it
    Cause     string   // "command %q exited %d" | the operational error | "expand post: …"
}
func (e *PostActionError) Error() string      { return fmt.Sprintf("%s: %s", ErrPostAction, e.Cause) }
func (e *PostActionError) Is(t error) bool     { return t == ErrPostAction }
```

`Transitioner`'s POST phase builds it at each of its three failure points
([transition.go:113/117/120](../../../internal/core/transition.go#L113)):

- **non-zero check** — `firstFailure` gives index `i`; `Remaining = expanded[i:]` (the failed command + those
  after, which the runner never reached — `Runner.Run` stops at the first non-zero).
- **operational error** (`rerr`) — the failing command is the last recorded check (Runner CONTRACT), so
  `Remaining = expanded[len(pchecks)-1:]`; `Cause = rerr.Error()`.
- **expand error** — no `expanded` exists; `Remaining = rawRuns(edge.Post)` (the unexpanded `Run` templates,
  best-effort — expansion is what failed); `Cause = "expand post: …"`.

`Remaining` commands are **expanded** (placeholders resolved), so they are copy-paste-ready.

**CLI change (`status.go` `runTransition`, the sole exit-5 site):** replace the terse post-fail line with a
recovery block on stderr; `errors.As(txErr, &pe)` extracts the typed error:

```
move applied — the status change IS saved; do NOT re-run the move.
finish the finalization by hand:
  <remaining cmd 1>
  <remaining cmd 2>
```

- The **cause** is *not* repeated here — `Execute` already prints `error: mtt: post-action failed after the
  move: <cause>`. The block adds the two things missing today: the **idempotence warning** and the **exact
  remaining commands**. (This removes the current redundant `%v` echo of the whole error.)
- The stdout move-render (the `<id>: from → to` line + guidance) is **unchanged** — the move happened.

### D4 — exit 4: not-found hint (generic, task + note)

`notFoundHint` (const) fired by `exitHint` for any `mtt.ErrNotFound` wrap (task ids: `show`/`edit`/`tree`/
`use`/`dep`/`status`/`add --parent`/`--depends-on`/`rm`; note slugs: `note …`):

```
  check the id — 'mtt roadmap' or 'mtt list' show existing task ids ('mtt note list' for notes)
```

- One generic line covers both carriers (task and note both wrap the same `ErrNotFound`; distinguishing them
  in `Execute` would need a second sentinel for marginal benefit — YAGNI). It points at the discovery
  commands, which is the actionable next step after a typo.

### D5 — exit 6: verify already-actionable, align only

No new mechanism. `t28` **verifies** both invalid-transition messages list the valid moves and aligns wording
if they drift:

- **core path** (`status`/sugar, [transition.go:63](../../../internal/core/transition.go#L63)):
  `… cannot move <from> → <to> (allowed from <from>: <targets>)` — already lists targets via `allowedTargets`.
- **`mtt do` path**: `doMissError` already lists `availableActions`.

If both already read clearly and consistently, D5 ships **zero code** (an acceptance check, not an edit).
Recorded so the "exit 6 in scope" decision is honored honestly rather than padded with a redundant hint.

### D6 — Where the hint text lives & format

- **Text home:** `internal/cli/errors.go` (the existing not-found-wrapper home) holds `exitHint`,
  `attributionHint`, `notFoundHint` (consts). The contextual exit-5 block lives in `status.go` (the exit-5
  site), reusing the typed `*PostActionError`.
- **Format:** hints are a trailing block under the `error:` line (or the post-fail block on stderr), each line
  indented two spaces, phrased as an imperative the reader can act on. No color, no new flag, stderr only.

## Scope

**In:**
- `exitHint`/`attributionHint`/`notFoundHint` in `internal/cli/errors.go` + the `Execute` hook (exit 2, exit 4).
- The typed `core.PostActionError` (`runner.go`) built at `Transitioner`'s three POST failure points
  (`transition.go`); the CLI recovery block in `status.go` (exit 5).
- exit 6 verification + wording alignment (likely no code).
- Unit tests (`core.PostActionError` `Remaining`/`Is`), CLI e2e (testscript) for each case; docs sync.

**Out:**
- **exit 3** (blocked) — already has the `-v`/`--log-file` hint (unchanged).
- **exit 7** (dangling refs) — `mtt check` already prints the findings (unchanged).
- **exit 1** (generic) — no taxonomy, no hint.
- **Per-field attribution parsing** (showing only the missing one) — YAGNI; the generic who+why block is
  correct for every path.
- **Any new flag / config / color** — deferred (`t55` owns verbosity/color).

## Acceptance criteria

1. **exit 2 hint (e2e).** A transition/`rm --force`/`note rm --force` missing who/why exits **2**, and stderr
   contains the `error: … missing required attribution: …` line **plus** the `set 'who': …` / `set 'why': …`
   hint block. Verified for at least the transition path and one dangerous-ops path (shared `exitHint`).
2. **exit 4 hint (e2e).** `mtt show <bogus-id>` (and one note path, `mtt note show <bogus-slug>`) exits **4**
   with the not-found line **plus** the `check the id — 'mtt roadmap' …` hint.
3. **exit 5 recovery (unit + e2e).**
   - *Unit:* a `Transitioner` whose post gate fails on the 2nd of 3 post commands returns a `*PostActionError`
     with `Remaining == expanded[1:]` (the failed + the untried), `errors.Is(err, ErrPostAction) == true`, and
     `Cause` naming the failed command. The operational-error and expand-error branches populate `Remaining`
     (last-check-onward / raw) and `Cause`.
   - *e2e:* a move whose `post:` fails prints, on stderr, `the status change IS saved; do NOT re-run the move`
     and the exact remaining command(s); the task file shows the **new** status (move persisted); exit **5**.
4. **exit 6 unchanged-but-verified (e2e).** `mtt done <tbd-task>` (no direct edge) exits **6** and lists
   `allowed from <status>: …`; `mtt do <task> <bogus-edge>` exits **6** and lists the available actions. If
   wording was aligned, the e2e asserts the aligned text.
5. **No hint bleed (e2e/unit).** exit-5 output does **not** carry the not-found/attribution hints (its error is
   `*PostActionError`, matched by neither branch); a blocked gate (exit 3) still shows only its own `-v` hint.
6. **Exit codes unchanged.** `TestExitCode` still maps 2/3/4/5/6/7 exactly (the typed `*PostActionError` still
   maps to 5 via `Is`).
7. `make check` green. Docs synced (below).

## Testing approach

- **Unit (`internal/core`):** `PostActionError` construction at all three POST failure points (fake `Runner`
  returning: a non-zero 2nd check; an operational error; a malformed post template) — assert `Remaining`,
  `Cause`, and `errors.Is(…, ErrPostAction)`. `internal/cli` `TestExitCode` gains a `*PostActionError` case
  (→ 5).
- **Unit (`internal/cli`):** `exitHint` table — `ErrMissingAttribution` → attribution block; `ErrNotFound`
  wrap → not-found block; `*PostActionError` and `ErrInvalidTransition` → `""`; unrelated error → `""`.
- **e2e (testscript, hermetic):** extend/add scripts for exit-2 (transition + a dangerous-ops path), exit-4
  (task + note), exit-5 (a config with a **failing `post:`** command — e.g. `post: ['false']` — assert the
  recovery block + persisted status + exit 5), exit-6 (both paths). No network.

## Docs to sync (docs-sync judgment, `impl_review`)

Grep **all** parallel occurrences (EN + RU) before editing.

- **`CLI_REFERENCE.md ↔ .ru.md`:** the exit-code table / attribution section — note that exit 2 prints the
  who/why setup hint and exit 5 prints the recovery commands + "move is saved". Grep for the exit-code list.
- **`DESIGN.md ↔ .ru.md`:** the `ErrPostAction` / post-persist note (the t21 "Shipped" block) — mention the
  typed `PostActionError` carrying the unfinished commands for recovery; the attribution note — mention the
  actionable setup hint. One parallel clause each (EN + RU).
- **`CHANGELOG.md`** `[Unreleased]` → **Changed** (or Added): actionable errors — exit 2 hints how to set
  who/why; exit 5 prints the exact recovery commands and that the move is already saved.
- **CLAUDE.md:** `internal/cli` (the `exitHint`/`Execute` hook + the exit-5 recovery block) and `internal/core`
  (the typed `PostActionError`). Keep thin.
- **`AGENTS.md`:** no new rule expected (the flow is unchanged); touch only if a convention changes.

## Sequencing & tracking (process, not code)

`t28` is `speccing` on `task/t28`. This document is the `speccing` deliverable. Next: commit it, run an
adversarial subagent **spec review**, address findings, then `spec_human_review` → `planning` → `plan_review`
→ `plan_human_review` → TDD `implementing` → `impl_review` → `approved` (auto PR) → merge → `deliver`. Part of
the `v0.10.0` batch (with `t44`, `t14`); with `t14` delivered, `t28` is the last blocker of `t42` (user-docs
audit) — delivering it unblocks `t42`.
