# t46 — Scaffold how-to-use-mtt agent docs (spec)

Status: revision 1 — decided in the 2026-07-26 brainstorm (three user decisions: doc set, merge strategy,
command surface; + one approval: the pre-release API consolidation of the `internal/scaffold` seam). The
2026-07-26 adversarial spec review returned **no blocker/major** (the API-consolidation count grep-verified
complete, the runbook generic + accurate, the merge convergent); its five polish minors are folded here —
a concrete rename change-inventory, the `claudeHarness`→`claudeTarget` rename + intentional `"claude"`
name-reuse note, the init two-scaffold output ordering, the CHANGELOG prose seam-name, and naming `GEMINI.md`
in the FLOW_GUIDE flip.

## Problem

A fresh project adopting mtt has no on-ramp for **how an agent works under the mtt flow**. mtt's own
`AGENTS.md` "Working under mtt" runbook is project-specific (it references this repo's dogfood setup and our
specialized statuses). Nothing scaffolds a **generic, project-agnostic** runbook into a new project. This is
the docs half of the post-t62 onboarding arc: t52 shipped the **config/hooks** facet (`mtt agent hooks`);
t46 is the **docs** facet (`mtt agent docs`) — a sibling in the same `mtt agent` group.

Split rationale (from t23, 2026-07-17, Part A): the scaffolded docs must describe mtt **mechanics**
(types/statuses/transitions, gates, attribution, dangerous-ops, storage, closure-via-edges) in a
**project-agnostic** way and point agents at `mtt types` / `mtt roadmap` / `mtt show` to discover **this**
project's actual flow. It must **not** hardcode our dogfood statuses (`speccing`/`spec_review`/…) — a
continuation of t19's self-instructing runbook and the prime directive "mechanize the process into mtt".

## Decisions (brainstorm record, 2026-07-26)

1. **Doc set = AGENTS.md (runbook) + CLAUDE.md + GEMINI.md (pointers)** (user decision, Option 1 of 3). The
   full generic runbook lives in **`AGENTS.md`** (the cross-tool standard — Codex and others read it); thin
   **marked-block pointers** go in `CLAUDE.md` and `GEMINI.md` (each auto-loaded by its harness → redirect to
   AGENTS.md + `mtt roadmap`/`mtt types`). Content lives in one place (DRY). Unlike t52's config (whose *hook
   format* was unverified), markdown has **no format risk**, so writing all three is safe and honors the
   task's three-file scope.
2. **Merge = idempotent marked-block injection** (user decision, Option 1 of 2). A
   `<!-- mtt:begin -->…<!-- mtt:end -->` block: create-if-absent, insert-if-missing, **replace-if-present**
   (a re-run regenerates the block as mtt evolves — the answer to the "stays honest" design-open); all
   surrounding user content is preserved; a malformed marker pair is a **loud refusal**, never a clobber.
3. **Surface = `mtt agent docs` + init by default** (user decision, Option 1 of 2). A dedicated
   `mtt agent docs` subcommand (sibling of `mtt agent hooks`), and **`mtt init` scaffolds docs by default**
   (`--no-agent-docs` opts out — symmetric with t52's `--no-agent-hooks`). The marked-block injection makes a
   default-on injection into an existing `AGENTS.md` non-destructive.
4. **API consolidation of `internal/scaffold` (pre-release)** (user approval). t52 named the seam `Harness`
   assuming every target is an agent-tool config file; t46 adds `AGENTS.md` (a **cross-tool doc, not a
   harness**), which reveals the honest general name. Both t52 and t46 are `[Unreleased]`, so consolidate now:
   `Harness` → **`Target`**, `Registry()` → **`HookTargets()`**, add **`DocTargets()`**;
   reporting `Result.Harness` → **`Result.Name`** and the CLI's `{harness,…}` JSON → **`{name,…}`**;
   `initJSON.Scaffold` → **`Hooks`** plus a new **`Docs`** array.

## Goals

1. **`mtt agent docs`** — a new, idempotent subcommand scaffolding a project-agnostic "Working under mtt"
   runbook (`AGENTS.md`) + pointer blocks (`CLAUDE.md`, `GEMINI.md`) into the project. Re-runnable; root via
   `projectRoot` (needs `.mtt/`).
2. **`mtt init` scaffolds docs by default** — after `.mtt/config.yaml` (and the t52 hooks), init injects the
   doc blocks; `--no-agent-docs` opts out. The failure contract mirrors t52 (config is the primary job; a doc
   scaffold error is reported after config-success and exits 1, config kept).
3. **Generic, honest content** — describes mtt mechanics project-agnostically and directs the agent to
   `mtt types` / `mtt roadmap` / `mtt show` for **this** project's flow; **no** hardcoded statuses; **no**
   assumption that a specific edge verb (`start`) exists (it points at `mtt show <id>`'s `next:` line instead).
4. **Idempotent marked-block merge** — a shared pure `blockMerge`: create / insert / **regenerate** / refuse
   malformed; surrounding content preserved; a re-run is `unchanged`.
5. **API consolidation** — the `internal/scaffold` seam is renamed to the honest general terms (Decision 4),
   reused by both `agent hooks` and `agent docs`, with t52's tests + docs updated to match (pre-release).
6. **No silent traps** — opt-in/opt-out, never clobber, everything written/merged is reported (human + JSON).
7. **Docs (EN+RU) + tests** reflect the new command, the init default, and the consolidated API.

## Non-goals

- **Config-derived content** — the runbook is **static** generic text. It does NOT snapshot the project's
  actual types/statuses (that would rot as the flow evolves); it tells the agent to run `mtt types`. This is
  the whole point of the t23 split constraint.
- **Per-package `CLAUDE.md` stubs** — the task floats these as *optional*; deferred (YAGNI). Per-package docs
  are project-specific; the generic root runbook + pointers are the on-ramp. (Revisit only on real need.)
- **Localizing the runbook** — the scaffolded docs are **English only** (agent-facing; mirrors the AGENTS.md /
  CLAUDE.md language rule). This spec's *repo* docs still follow the bilingual rule below.
- **Touching t52's hook content or the hook merge** — t46 reuses the shared `Run`/`Target` seam and the
  `blockMerge` is new; the JSON/field renames are mechanical (Decision 4), not a behavior change to hooks.
- **Un-scaffolding / removing blocks** — the merge only creates / inserts / regenerates. Removing the block is
  a manual delete (between and including the markers).
- **A `--agents`/target selector flag** — as in t52, YAGNI; `agent docs` scaffolds its fixed doc set.

## Design

### Surface

- `mtt agent docs [--json]` — scaffolds the doc set for the project. Root via **`projectRoot(cmd)`** (an
  initialized mtt project; `--dir`/`MTT_DIR` honored). Human output: one line per file
  (`<name>: <action> <path>`, `name` ∈ `agents`/`claude`/`gemini`, action ∈ `created|merged|unchanged`) plus a
  one-line note when anything changed. `--json` emits a non-null `[{name, path, action}]`.
- `mtt init` gains **`--no-agent-docs`** (bool, default false). After the config write **and** the t52 hook
  scaffold, unless the flag is set, init runs the doc scaffold and folds the results into output. `initJSON`
  carries **`hooks`** (t52, renamed from `scaffold`) and **`docs`** arrays (both non-null; `[]` when the
  respective opt-out is set). **Output ordering:** config-success line first, then the hook lines, then the
  doc lines; a doc-scaffold error (hooks already succeeded) prints config-success + hook results first, then
  the error → exit 1 (no partial JSON) — the t52 contract extended to the second scaffold.
- Targets (relative to project root): `AGENTS.md` (the runbook), `CLAUDE.md`, `GEMINI.md` (pointers). Parent
  is the project root itself, so no directory creation.

### Doc content (static, project-agnostic)

**`AGENTS.md` block** (the runbook — the block *content* between the markers):

```markdown
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
```

**`CLAUDE.md` / `GEMINI.md` block** (identical pointer content):

```markdown
## Working under mtt

This project uses **mtt** for its tasks and workflow. The full runbook is in **AGENTS.md** (the "Working under
mtt" section). Start with `mtt roadmap` (what's next), `mtt types` (this project's flow + gates), and
`mtt show <id>` (a task and its next moves). This block is scaffolded by `mtt agent docs`; edit OUTSIDE the
mtt:begin/mtt:end markers.
```

Every referenced command is universal (`roadmap`/`ready`/`types`/`show`/`add`/`edit`/`note add`/`prime`, the
`mtt <status|edge> <id>` sugar); nothing assumes a named edge or a specific status. The prose mentions
`mtt:begin/mtt:end` as *text* — never the literal `<!-- mtt:begin -->` marker — so it does not perturb marker
detection.

### Merge semantics (`blockMerge`, pure)

Markers (exact): `<!-- mtt:begin -->` and `<!-- mtt:end -->` (HTML comments — invisible in rendered markdown).
The canonical block is `B = "<!-- mtt:begin -->\n" + content + "<!-- mtt:end -->\n"`, where `content` is
normalized to end with exactly one `\n`.

Given `existing` bytes and `exists`:

1. **Absent, or empty/whitespace-only** ⇒ result = `B`; action `Created`.
2. **Present, `countBegin == 0 && countEnd == 0`** (no block yet) ⇒ **append**: `ensureTrailingNewline(existing)
   + "\n" + B`; action `Merged`.
3. **Present, `countBegin == 1 && countEnd == 1` with begin before end** ⇒ **regenerate**: replace the span
   from the begin marker through the end-of-line of the end marker with `B`, preserving the exact prefix and
   suffix; then if the result equals `existing` → `Unchanged`, else `Merged`.
4. **Any other marker shape** — `countBegin != countEnd`, either `> 1`, or an `end` before its `begin` ⇒
   **loud refusal** naming the file (a shape we cannot safely regenerate; never clobber user content).

Idempotency: after a `Created`/`Merged` write, the file has exactly one well-formed block; a re-run hits case 1
(no — it exists) → case 3, which reproduces `B` between the same prefix/suffix → byte-identical → `Unchanged`.
Convergence in one step: a first `append` (case 2) leaves a well-formed block, so the second run is case 3 →
`Unchanged`. This is simpler than t52's JSON merge (no key-ordering concern — the block bytes are fixed text).

### Architecture (reuse t52; consolidate the seam)

The seam is `internal/scaffold`, reused verbatim for the IO and generalized in name (Decision 4):

- **Rename** `Harness` → **`Target`** (interface `{ Name() string; RelPath() string; Merge(existing []byte,
  exists bool) ([]byte, Action, error) }`); the concrete `claudeHarness` → **`claudeTarget`** (for
  consistency); `Registry()` → **`HookTargets()`**; `Result.Harness` → **`Result.Name`**. `Action`/`Result`/
  `Run` behavior is otherwise unchanged. `Run(root string, targets []Target)` stays the write-as-you-go IO
  edge. **Name reuse is intentional:** the hook target and the CLAUDE.md doc target both report `Name() =
  "claude"` — disambiguated by their distinct `Path` (human output) and by living in separate `hooks`/`docs`
  arrays (`init --json`), so there is no functional collision.
- **New `scaffold/docs.go`** — `DocTargets() []Target` returns three `docTarget{name, relPath, content}`
  values (agents/claude/gemini), each `Merge` delegating to the shared pure **`blockMerge(existing, exists,
  content)`**; the runbook + pointer content are package constants.
- **CLI** — `newAgentDocsCmd()` (in `agent.go`) calls `scaffold.Run(root, scaffold.DocTargets())` and reuses
  the shared `reportScaffold`; `scaffoldJSON` becomes `{name, path, action}`; `mtt init` calls both
  `HookTargets()` (unless `--no-agent-hooks`) and `DocTargets()` (unless `--no-agent-docs`), folding `hooks`
  and `docs` into `initJSON` + output.

**Rename change-inventory (so the plan drops nothing):** the consolidation touches, beyond the docs list
below — `internal/scaffold/scaffold.go` (interface, `Registry`→`HookTargets`, `Result.Name`, `Run` sig),
`internal/scaffold/claude.go` + `claude_test.go` (`claudeHarness`→`claudeTarget`), `internal/scaffold/run_test.go`
(`Registry()` → `HookTargets()`), `internal/cli/agent.go` (`scaffoldJSON`/`r.Harness`→`r.Name`),
`internal/cli/init.go` (`Registry()`→`HookTargets()`, `Scaffold`→`Hooks`), `internal/cli/json.go`
(`initJSON.Scaffold`→`Hooks`+add `Docs`), and the two testscripts
`internal/cli/testdata/scripts/{agent_hooks.txt,init.txt}` (the `"harness"`/`"scaffold"` JSON assertions).
`go build`/`go test` enforce the code+test sites; the docs are the only silently-missable class (listed below).

`internal/scaffold` stays onboarding infra — **not** `pkg/mtt` domain, **not** `.mtt/` storage, writing via
`internal/fsutil.AtomicWrite` (t52).

### No silent traps

- **Opt-in / documented opt-out** — `mtt agent docs` is explicit; init's default has `--no-agent-docs`.
- **No clobber** — marked-block injection preserves all surrounding content; a malformed marker pair refuses.
- **Reported** — every create/merge/unchanged prints its file + action (human and `--json`); init folds both
  hooks and docs. The regeneration note in the block itself tells a human the section is managed.
- **Failure contract** — init: config first & kept; a doc-scaffold error is reported after config-success and
  exits 1 (no partial JSON), same as t52.

### Testing

- **Unit** (`internal/scaffold`, table-driven on `blockMerge` and the doc targets):
  1. absent ⇒ `Created` = the canonical block (byte-exact);
  2. empty/whitespace-only existing ⇒ `Created`;
  3. existing file, no markers ⇒ `Merged`, block **appended**, original content preserved verbatim;
  4. existing file already carrying our exact block ⇒ `Unchanged`, no write;
  5. **convergence:** `blockMerge(appendResult, …)` ⇒ `Unchanged` (one-step convergence);
  6. existing block with **stale content** ⇒ `Merged`, block **regenerated**, prefix/suffix preserved;
  7. malformed markers (begin without end / end before begin / duplicate begin) ⇒ **loud refusal**, no write;
  8. `DocTargets()` returns three targets with the right `RelPath`s and non-empty content; each is idempotent.
- **CLI e2e** (`testscript`): `mtt agent docs` in a temp project ⇒ `AGENTS.md`/`CLAUDE.md`/`GEMINI.md` created
  with the marked block + a `Working under mtt` heading; a second run ⇒ idempotent (`unchanged`); a merge into
  a **pre-existing** `AGENTS.md` (with unrelated content) ⇒ appended, original preserved; `mtt init` (default)
  ⇒ scaffolds docs; `mtt init --no-agent-docs` ⇒ no doc blocks; `--json` (`agent docs` + init `docs`/`hooks`
  arrays, incl. the non-null `[]` under each opt-out); `mtt agent docs` outside a project ⇒ actionable error.

## Docs to update (behavior change)

- **`DESIGN.md`(+ru)** — extend the Shipped(t52) onboarding note (or add a Shipped(t46) note): `mtt agent docs`,
  init default + `--no-agent-docs`, the marked-block merge, the doc set, and the seam consolidation
  (`Target`/`HookTargets`/`DocTargets`). Update any t52 wording that named the JSON key `harness`/`scaffold`.
- **`FLOW_GUIDE.md`(+ru)** — the "Agent-usage docs" bullet in the "still tracked separately" list is **shipped
  by t46**; flip it (mirrors the t52 "Settings & hooks" flip) and name all three files (**AGENTS.md / CLAUDE.md
  / GEMINI.md**), since the bullet currently says only "AGENTS.md/CLAUDE.md".
- **`CLI_REFERENCE.md`(+ru)** — a `mtt agent docs` entry + `mtt init --no-agent-docs`; update the t52
  `mtt agent hooks` `--json` shape `{harness,…}` → `{name,…}` and `init --json` `scaffold` → `hooks`+`docs`.
- **`internal/cli/CLAUDE.md`** — the `agent docs` wiring + the renamed reporting; **`internal/scaffold/CLAUDE.md`**
  — the `Target`/`HookTargets`/`DocTargets` seam + `blockMerge`.
- **`CHANGELOG.md`** — an `[Unreleased]` entry for `mtt agent docs`; amend the t52 entry's `--json` wording to
  the consolidated `hooks`/`docs`/`name` shape **and its prose "extensible `Harness` seam" → `Target`** (both
  are `[Unreleased]`).
- **`README.md`(+ru)** — the Quickstart `mtt init` line already notes hook scaffolding; extend to docs (or note
  `--no-agent-docs`).

## Open questions / risks

- **Marker-string collision** — the block content mentions `mtt:begin/mtt:end` as prose but never the literal
  `<!-- mtt:begin -->`; marker detection matches the full comment, so prose is inert. Pinned by a unit test.
- **A user who mangles the markers** (e.g. deletes only `end`) lands in case 4 (loud refusal) — intentional
  (don't guess); the message names the file and the fix (restore or remove the stray marker).
- **Consolidation churn on t52** — mechanical renames across `scaffold`/CLI/tests/docs; behavior-preserving
  (t52's e2e + unit assertions updated to the new key names; the hook merge itself is untouched).
