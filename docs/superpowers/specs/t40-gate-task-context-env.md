# t40 — task-context environment for gate/post/event commands (decision record)

**Type:** task · **Tags:** flow, dx · **Blocks:** t60 (release, ergonomic enabler) · **Sibling:** t77 (facet a, deferred)

Decision record from the 2026-07-27 brainstorm. Resolves the pilot's **#1** friction: a gate/post
command can see only the 4-field shape-safe placeholder whitelist (`{{.ID}} {{.Type}} {{.From}} {{.To}}`)
and **cannot see its own task's fields or children**, forcing re-query (`mtt show --json {{.ID}}`) and
shell-outs. This gates a clean `github-gated` template (t76) and every non-dev gate whose payload *is* the
entity's fields.

## Problem

Executable gates/posts run as `sh -c` (exec adapter) with placeholders expanded from `cmdContext{ID,Type,
From,To}` (`internal/core/expand.go`) — a deliberately narrow, **shape-safe** whitelist: free text
(title/tags/description) is **never** interpolated into the command string (the injection boundary kept
since t5/t21/s007; a `{{.Title}}` is a template error). The cost: a gate that needs the task's title,
tags, parent, priority, or its **children's statuses** (a roll-up "all children done" gate) has no way to
see them except re-querying mtt mid-move. The pilot hit exactly this.

## Decision

Expose the task's read-only context to gate/post/event commands **as environment variables**, not as new
template placeholders. This is the crux that keeps safety intact:

- **`{{...}}` stays shape-safe and UNCHANGED** (`cmdContext{ID,Type,From,To}` only). Free text is still
  never interpolated into the command *string*.
- **The environment carries the rest.** A command referencing `"$MTT_TASK_TITLE"` gets the value as
  **opaque shell data** — the shell does not re-parse an env value as code. So free text rides env safely;
  the only way to make it code is for the gate author to `eval` it, which is author-owned (same as any
  data). The t5/t21/s007 boundary — *mtt never interpolates free text into the command string* — holds.

### Env surface

Injected into every gate `commands:`, edge `post:`, and lifecycle-`event` post process:

- **`MTT_TASK_JSON`** — the whole task as JSON, the **lean `toTaskJSON` DTO** (`internal/cli/json.go` —
  the shape `list`/`add --json` emit), **not** the rich `show --json`/`showJSON` (which drags in the flow
  `cfg` for `next` and cross-store `backlinks` the env has no need for). The general/extensible channel
  (`jq` for anything not covered by a scalar).
- **`MTT_TASK_CHILDREN_JSON`** — the task's children, each with (at least) `{id, type, status}`, as a JSON
  array. This is the **one relational field** the pilot genuinely needed (the roll-up gate) and what an
  HTTP/deploy "is-this-aggregate-complete" gate needs. Requires listing the task set (a `List` on the
  move/event path — acceptable cost).
- **Convenience scalars** (avoid `jq` for the hot fields): `MTT_TASK_ID`, `MTT_TASK_TYPE`,
  `MTT_TASK_STATUS`, `MTT_TASK_TITLE`, `MTT_TASK_PARENT` (`""` if none), `MTT_TASK_PRIORITY` (`""` if
  unset), `MTT_TASK_TAGS` (comma-separated, matching `--tag` input).

`.From`/`.To` of a move stay in `{{...}}` (already there; events have no from/to). **ancestors** and
**ref-backlinks** are **YAGNI** — explicitly not built (pilot Q3). **VCS context** (diff/merge-base) is a
**separate axis**, not this task.

## Scope

- **In:** facet (b) — the ambient read-only task-context env above, for gate/post **and** event pipelines
  (consistent), plus the docs stating the env surface + the safety contract.
- **Out / deferred:** facet (a) — the per-invocation caller `--arg key=value` → `{{.Args.key}}` /
  `--pr-body-file` value channel — is a **distinct mechanism** (caller values into the template surface),
  split to **t77**, not release-critical. **by/why** (move attribution) in env — deferred (add on demand;
  the t76 template does not need it, and a name must avoid colliding with the input `MTT_BY`). **VCS
  context** — separate axis, possible sibling.

## Wiring / design considerations (plan to resolve)

- **`core` stays serialization-free** (AGENTS.md: `pkg/mtt` types carry no json tags), so the JSON blobs
  serialize through the **CLI's task-JSON DTO** (`toTaskJSON`), never a duplicate marshaler in core. Because
  lifecycle events fire **per-entity inside core** (e.g. `Remover`'s bulk loop), the CLI cannot pre-build
  every env — so inject an **env-builder `func(mtt.Task) map[string]string`** into `Transitioner` **and**
  `EventEmitter` (mirroring the injected clock/runner). Core passes only the **task**; the injected fn — a
  **CLI closure that captures the store** — does BOTH the children-`List` AND the serialization, and returns
  the ready `MTT_TASK_*` map. Core decides *when* to call it and forwards the map to the runner; it does no
  `List` and no marshaling. **Correction (spec review): `EventEmitter` does NOT currently hold a `TaskStore`
  — this seam is exactly why we do not add one** (only the builder fn joins its constructor). The
  **env-var key whitelist** (which `MTT_TASK_*` keys) is presentation and lives in the CLI builder (like
  `taskJSON`'s json tags); the **security whitelist** (`cmdContext{ID,Type,From,To}` for `{{...}}`) stays in
  core, unchanged.
- **`Runner` port gains env.** `Run(commands, env)` (and `Compensate(commands, env)` for **parity only** —
  no concrete env-dependent-compensator need) — the exec adapter sets `env` on the `sh -c` process
  (`cmd.Env = append(os.Environ(), …)`), faked in tests. It lives in `internal/core` (Runner is a
  core-defined port) + `internal/adapter/exec` + the test fakes (`transition_test.go`/`remove_test.go`/
  `exec_test.go`/`exec_pgid_test.go`) **and the doc-model `Runner` in `docs/architecture/model.go`** (a
  compiled snapshot — keep it in sync).
- **Note events get NO `MTT_TASK_*`.** `EventEmitter.NoteEvent` fires with a `mtt.Note`
  (`noteEventContext{Slug,Event}`), not a task — the task env is meaningless there. Task events + gate/post
  get `MTT_TASK_*`; note events get none (an `MTT_NOTE_*` surface is out of scope / demand-driven).

## Safety (the load-bearing argument)

- `{{...}}` whitelist unchanged → no new free-text-into-command-string surface.
- Env values are **data**: `sh -c` does not re-evaluate them (verified in review — a title of
  `` $(touch /tmp/PWNED) `` referenced as `"$MTT_TASK_TITLE"` echoes literally; nothing runs); a gate that
  `eval`s an env value owns that (documented). The JSON blobs are **marshaled** (never string-concatenated),
  so a `"` in the title is escaped; `MTT_TASK_TAGS` is comma-joined safely (tags forbid commas).
- **The real env edge is a NUL byte** (not a newline — newlines in env are fine): Go's `exec` refuses an
  env value containing NUL (`environment variable contains NUL`), which would fail the gate **operationally**
  (not a security hole). NUL is unreachable from argv but reachable from a hand-edited YAML title; the
  builder should strip/guard NUL (or accept the loud operational failure). `validateTitle` today rejects
  only `\n\r`.
- Same env is offered to events — no new privilege (events already run `sh -c` post pipelines).

## TDD / acceptance

- **Test-first.** Unit: the env-builder maps a task (+children) to the exact `MTT_TASK_*` set (names,
  scalar formats, tags-csv, empty parent/priority). Adapter: exec sets env on `sh -c` (a fake command
  echoes `$MTT_TASK_TITLE` / `$MTT_TASK_CHILDREN_JSON` and the test asserts it). Core: `Transitioner` and
  `EventEmitter` pass the built env through `Runner.Run`/event post (fake runner captures env). e2e
  (`testscript`): a gate reading `$MTT_TASK_TITLE` and a roll-up gate over `$MTT_TASK_CHILDREN_JSON`
  (blocks until all children done) work end-to-end.
- **Invariant preserved:** the existing `{{...}}` whitelist tests stay green (a `{{.Title}}` is still a
  template error); no free text enters the command string.
- `make check` green; CHANGELOG updated; `internal/core`/`internal/adapter/exec` CLAUDE.md + the doc-model
  `docs/architecture/model.go` updated (Runner gains env; the env surface documented). t40 re-titled to
  facet (b) (done). The deferred facet-(a) sibling **t77 was created on `main` (2026-07-27)** — real, though
  it is not on this feature branch (branched earlier), so it resolves at delivery; not a silent drop. The
  env surface is documented so the t76 `github-gated` template can drop its re-query workarounds.
