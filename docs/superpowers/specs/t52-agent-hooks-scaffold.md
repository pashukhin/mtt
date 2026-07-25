# t52 — Scaffold agent settings + hooks (spec)

Status: revision 1 — decided in the 2026-07-25 brainstorm (four user decisions: harness breadth, merge
strategy, command surface, permissions allowlist; + one refinement: the command surface is the group
`mtt agent hooks`, not a flat `mtt scaffold-agents`).

## Problem

t51 shipped `mtt prime` (a curated KB session-start digest) but left **the hook that runs it as
config-not-code**: a documented `settings.json` snippet in `CLI_REFERENCE.md`, which every project must copy
by hand. This repo itself carries the snippet hand-authored in `.claude/settings.json`. Two gaps:

1. **Onboarding a project to the agent loop is manual.** To get the prime digest injected at session start,
   a user must know the exact Claude Code `settings.json` shape and hand-write it. Nothing in `mtt init`
   wires it. This is the "mechanize the process into mtt" rule (AGENTS.md TL;DR 0) unmet for the one hook we
   most want present.
2. **There is no idempotent, safe way to add our hooks to a project that already has a `settings.json`.**
   `mtt init` writes only `.mtt/config.yaml` and refuses to touch an already-initialized project
   (`ErrAlreadyInitialized`), so it cannot help an existing repo (like this one) adopt the hooks; and a naive
   writer would clobber a user's existing agent settings.

t52 is the **agent-config/hooks** facet of the post-t62 onboarding arc, and a sibling of **t46** (which
scaffolds agent **docs** — `AGENTS.md`/`CLAUDE.md`/`GEMINI.md`). The two are deliberately separate: t52 writes
per-harness **configuration** that wires mtt into the agent loop; t46 writes per-harness **documentation**.

## Decisions (brainstorm record, 2026-07-25)

1. **Harness breadth = Claude Code flagship + a code seam** (user decision, Option 1 of 3). The `SessionStart`
   hook format is verified only for Claude Code (field-tested in this repo). `codex` (TOML) and `gemini`
   config-hook formats are **unverified**; writing guessed config would be a silent trap against the release
   bar (t62's "no silent traps"). So t52 **fully implements Claude Code** and lays a clean **`Harness`
   interface + registry** so codex/gemini drop in when their formats are verified — **without** shipping any
   guessed config for them. The seam is **code** (interface + registry), not a CLI flag.
2. **Merge strategy = additive, idempotent merge** (user decision, Option 1 of 2). When an agent settings file
   already exists, parse it, add **only** our entries if missing, and **preserve every other key/hook**. A
   re-run is a no-op (`unchanged`). A malformed/unparseable existing file is a **loud refusal** — never a
   clobber. The merge is a **pure, unit-tested function**; IO lives at the edge.
3. **Command surface = a dedicated group + init by default** (user decision + refinement). A dedicated,
   re-runnable, idempotent subcommand **`mtt agent hooks`** (the `mtt agent` group is the onboarding umbrella;
   t46 later adds `mtt agent docs`). **`mtt init` runs `agent hooks` by default** after writing
   `.mtt/config.yaml`; opt-out is **`mtt init --no-agent-hooks`**. Rationale: minimize user steps at project
   start, while still supporting adoption on an already-initialized project via the standalone command.
4. **Permissions allowlist = included** (user decision, Option 2 of 2). Beyond the hooks, scaffold a
   **read-only mtt `permissions.allow`** list into the same Claude settings file, to cut permission prompts
   for the safe, non-mutating mtt commands.

Refinement: the command is the **group form `mtt agent hooks`** (chosen over a flat `mtt scaffold-agents`),
so t46's docs facet is a sibling subcommand of the same `mtt agent` group.

## Goals

1. **`mtt agent hooks`** — a new, idempotent subcommand that scaffolds agent configuration for every
   **registered** harness (t52 registers exactly `claude`). Re-runnable on an already-initialized project.
2. **`mtt init` scaffolds by default** — after `.mtt/config.yaml`, init runs the same scaffold; `--no-agent-hooks`
   opts out. init still refuses to overwrite an existing `.mtt/config.yaml`; the scaffold is orthogonal and
   idempotent, so it runs even when config already existed under `--force`.
3. **Claude Code target, field-tested shape** — write/merge into `.claude/settings.json` (the **committed**
   project settings, shared with the team/agents):
   - `hooks.SessionStart` **and** `hooks.PreCompact`, each running `mtt prime 2>/dev/null || true`;
   - `permissions.allow` — a union of read-only mtt subcommands.
4. **Additive, idempotent, no-clobber merge** — a pure `Merge` per harness: create-if-absent, union-if-present,
   no-op-if-already-there, refuse-if-malformed. Never touch a key/hook we did not author.
5. **No silent traps (release bar)** — the scaffold is opt-in (an explicit command, or init's default with a
   documented opt-out). What was written/merged is reported explicitly (human + `--json`). Because the hook
   runs our own **read-only** `mtt prime` (not an untrusted `sh -c` payload like a t62 external template), no
   confirmation prompt is warranted; a clear post-write report is the boundary.
6. **Docs (EN+RU) + tests** reflect the new command, the init default/opt-out, and the scaffolded shape.

## Non-goals

- **Writing codex/gemini config** — deferred until their hook formats are verified (Decision 1). t52 registers
  only `claude`; the `Harness` interface + registry are the extension seam. No guessed TOML/JSON is written,
  and no "manual snippet" is printed for them either (that is the deferred harnesses' own task).
- **Agent docs** — `AGENTS.md`/`CLAUDE.md`/`GEMINI.md` scaffolding is **t46** (`mtt agent docs`), a sibling.
- **A `--agents <list>` selector flag** — YAGNI with a single registered harness. The subcommand scaffolds all
  registered harnesses; selection UX is decided by the task that adds the second harness.
- **`settings.local.json` (gitignored) targeting / a `--local` flag** — t52 writes the **committed**
  `.claude/settings.json` (the whole point is a shared, team-visible hook). A local-overlay target is a future
  option, not shipped.
- **Preserving arbitrary formatting / comments** in an existing settings file — Claude `settings.json` is
  strict JSON (no comments); the merge re-serializes deterministically (2-space indent, trailing newline). We
  preserve **data** (every key/value), not byte layout.
- **Removing / editing hooks** — the scaffold only **adds** (union). It never deletes a user's hooks or ours;
  "un-scaffold" is out of scope (delete by hand).
- **Executing anything at scaffold time** — like `init`, the command only writes files; it runs no gate/hook.

## Design

### Surface

- `mtt agent` — a cobra **group** (mirrors the existing `mtt note` / `mtt dep` group pattern), no-op by itself
  (prints help). Its subcommands are the onboarding facets.
- `mtt agent hooks [--json]` — scaffolds agent configuration for all registered harnesses. Human output: one
  line per harness (`<harness>: <action> <path>`, action ∈ `created|merged|unchanged`) plus, on a
  create/merge, a one-line note of what the hook does. `--json` emits a non-null array of
  `{harness, path, action}` (t45 JSON-everywhere).
- `mtt init` gains **`--no-agent-hooks`** (bool, default false). After the config write, unless the flag is
  set, init runs the shared scaffold and folds the results into its output. `initJSON` gains a `scaffold`
  array of the same `{harness, path, action}` objects (non-null; empty `[]` when opted out).
- Target file: `<root>/.claude/settings.json`. The `.claude/` directory is created (0755) if absent.

### What is written (Claude Code)

A single JSON file, two concerns. Newly-created shape (existing files are **merged**, see below):

```json
{
  "hooks": {
    "SessionStart": [ { "hooks": [ { "type": "command", "command": "mtt prime 2>/dev/null || true" } ] } ],
    "PreCompact":   [ { "hooks": [ { "type": "command", "command": "mtt prime 2>/dev/null || true" } ] } ]
  },
  "permissions": {
    "allow": [
      "Bash(mtt roadmap:*)", "Bash(mtt list:*)", "Bash(mtt show:*)", "Bash(mtt tree:*)",
      "Bash(mtt ready:*)", "Bash(mtt types:*)", "Bash(mtt prime:*)", "Bash(mtt check:*)",
      "Bash(mtt tags:*)", "Bash(mtt version:*)", "Bash(mtt dep list:*)",
      "Bash(mtt note show:*)", "Bash(mtt note list:*)", "Bash(mtt ref list:*)", "Bash(mtt note ref list:*)"
    ]
  }
}
```

- **Hooks** — the field-tested reference shape (both `SessionStart` and `PreCompact`; the latter re-injects
  the digest around compaction, and `SessionStart` also re-fires with `source=compact`). The command is
  guarded (`2>/dev/null || true`) so a missing `mtt` binary never nags a contributor.
- **Permissions** — **only read-only, non-mutating** mtt subcommands, granular per subcommand (never a broad
  `Bash(mtt note:*)` that would also allow `note add`). The `Bash(mtt <sub>:*)` pattern is Claude Code's
  prefix match (covers the bare command and any args). **Deliberately excluded** (mutating): `add`, `edit`,
  `status`/verb-sugar/`do`, `start`/`submit`/`approve`/`decline`/`deliver`/`cancel`, `rm`, `tag add|rm`,
  `dep add|rm`, `ref add|rm`, `note add|edit|rm`, `note ref add|rm`, `use`/`use --clear` (writes
  `config.local`), `init`, `self-update`. The allowlist is a convenience for the read path, not an authority
  grant for state changes.

### Architecture (hexagonal / DDD)

This is **onboarding infrastructure**, not a tasks/knowledge domain concern: it writes files **outside**
`.mtt/` and does **not** go through `TaskStore`/`KnowledgeStore`. So it stays **out of `pkg/mtt`** (domain
purity preserved) and out of the yaml storage adapter.

New package **`internal/scaffold`** (the umbrella for agent-artifact scaffolding; t46's docs facet can share
it):

- `Harness` — the **seam interface**:
  - `Name() string` — e.g. `"claude"`.
  - `RelPath() string` — the target path relative to project root, e.g. `.claude/settings.json`.
  - `Merge(existing []byte, exists bool) (content []byte, action Action, err error)` — **pure**: given the
    current file bytes (and whether it exists), return the desired bytes and the action. `action` ∈
    `{Created, Merged, Unchanged}`; on `Unchanged`, `content` is ignored (no write).
- `claude.go` — the Claude Code `Harness` (JSON merge, below).
- `registry.go` — the registry `Registry() []Harness` (t52: exactly `claude`); the **single** extension point.
- `Run(root string, harnesses []Harness) ([]Result, error)` — the thin **IO edge**, shared by `mtt agent
  hooks` and `mtt init` (DRY): for each harness, read `<root>/<RelPath>` (absent ⇒ `exists=false`), call
  `Merge`, and **atomically write** iff `action != Unchanged`; collect `Result{Harness, Path, Action}`. A
  `Merge` error (e.g. malformed existing JSON) aborts that harness with a wrapped error naming the path; with
  one harness this fails the command.

The **pure `Merge`** is unit-tested exhaustively; the tiny IO wrapper is covered by CLI e2e.

### Merge semantics (Claude Code)

Given existing bytes:

1. **Absent** ⇒ emit the full canonical shape above; action `Created`.
2. **Present** ⇒ `json.Unmarshal` into a generic `map[string]any` (preserves unknown keys). A parse failure
   ⇒ **error, no write** (never clobber). Then, working on the map:
   - `hooks.SessionStart` / `hooks.PreCompact` (each independently): the hook is considered **already present**
     iff **any** command string under that event contains the token `mtt prime` (substring match) — robust to
     a user who customized the args (e.g. `mtt prime --limit 5`), so we never append a duplicate mtt-prime
     hook. If absent, **append** our matcher-group `{hooks:[{type:command,command:...}]}`, preserving existing
     entries. Missing `hooks` / event arrays are created.
   - `permissions.allow`: **union** our read-only entries into the existing array (existing entries first, our
     new ones appended in canonical order), **dedup** by exact string. Missing `permissions`/`allow` created.
   - Every other key is untouched.
3. Re-serialize deterministically (2-space indent + trailing newline). If the re-serialized bytes **equal** the
   existing bytes, action `Unchanged` (no write); else `Merged`.

Idempotency proof sketch: after a `Created`/`Merged` write, the token `mtt prime` is present in both events and
every allow-entry is present, so step 2 makes no additions and the re-serialized bytes equal the file ⇒ next
run is `Unchanged`. (The equality check in step 3 also makes a formatting-only first run converge in one step.)

### Atomic write

The scaffold writes via a **local minimal atomic write** (`os.CreateTemp` in the target dir → write → `Sync`
→ `Rename`, best-effort parent-dir fsync), new files at `0644`. It does **not** reuse the yaml adapter's
`atomicWrite` (which is `internal/adapter/yaml`-private and carries `.mtt/`-specific perm-**preservation**
policy — an existing file keeps its own mode). Rationale: a settings file is **not** the `.mtt/`
source-of-truth, its policy is a plain `0644` (no preservation), and an interrupted write is healed by
re-running the idempotent command. Extracting a shared `internal/fsutil.AtomicWrite` is a reasonable future
DRY move but is **out of t52's scope** (a tangential refactor of a guarded adapter); the plan may revisit if
the duplication is deemed load-bearing.

### No silent traps

- **Opt-in**: the standalone command is explicit; init's default is documented with an opt-out flag.
- **No clobber**: additive merge; malformed existing file ⇒ loud refusal.
- **Reported**: every write/merge prints its action + path (human and `--json`); init folds the same into its
  report. The user always sees that a `SessionStart`/`PreCompact` hook now runs `mtt prime`.
- **Low risk by construction**: the scaffolded hook runs our own read-only `mtt prime`, not an untrusted
  payload — so unlike a t62 external template (`sh -c` of foreign config), no confirm prompt is needed.

### Testing

- **Unit** (`internal/scaffold`, table-driven on `claude.Merge`):
  1. absent ⇒ `Created` with the canonical shape (assert both events + the allow set);
  2. existing file with unrelated keys (`env`, a custom hook) ⇒ `Merged`, unrelated keys preserved, our hooks
     appended;
  3. existing file already carrying our exact hooks + allow ⇒ `Unchanged`, no write;
  4. existing `SessionStart` with a **different** command ⇒ our hook appended, theirs preserved;
  5. existing `SessionStart` with a **customized** `mtt prime --limit 5` ⇒ recognized, **no duplicate**;
  6. `permissions.allow` with some overlap ⇒ union + dedup, order (existing first);
  7. malformed JSON ⇒ error, and `Run` performs no write;
  8. one event present, the other absent ⇒ only the absent one gains the hook.
- **CLI e2e** (`testscript`): `mtt agent hooks` in a temp dir ⇒ asserts `.claude/settings.json` content; a
  second run ⇒ idempotent (`unchanged`); `mtt init` (default) ⇒ scaffolds; `mtt init --no-agent-hooks` ⇒ no
  `.claude/`; `--json` surfaces for both the subcommand and init.

## Docs to update (behavior change)

- `DESIGN.md` (+ `DESIGN.ru.md`): a **Shipped (t52)** note under the init/onboarding section — the `mtt agent`
  group, the init default + `--no-agent-hooks`, the Claude flagship + `Harness` seam, the additive-merge
  semantics, and the read-only allowlist rationale.
- `CLI_REFERENCE.md` (+ `.ru`): document `mtt agent hooks` and `mtt init --no-agent-hooks`; update the t51
  "wire it into session start" section to point at the command (the hook is **now** scaffolded, no longer only
  a manual snippet).
- `internal/cli/CLAUDE.md`: the new command wiring; **new** `internal/scaffold/CLAUDE.md` (package
  responsibility, the `Harness` seam, "no `pkg/mtt` domain, no `.mtt/` storage").
- `CHANGELOG.md`: an `[Unreleased]` entry (the `task` flow gates on it).
- `README.md`/`.ru` if the getting-started section enumerates init behavior.

## Open questions / risks

- **Claude permission match syntax**: the `Bash(mtt <sub>:*)` prefix form is assumed to match both the bare
  command and args; validated against Claude Code's documented prefix-matching. If a bare-command match needs
  a separate `Bash(mtt <sub>)` entry, the plan adds both forms — a mechanical widening, no design change.
- **Second atomic-write implementation** (vs. the yaml adapter's) is an accepted, justified duplication
  (different perm policy); flagged for a possible future `internal/fsutil` extraction.
