# t52 — Scaffold agent settings + hooks (spec)

Status: revision 2 — rev1 decided in the 2026-07-25 brainstorm (four user decisions: harness breadth, merge
strategy, command surface, permissions allowlist; + one refinement: the group `mtt agent hooks`, not a flat
`mtt scaffold-agents`). rev2 addresses the 2026-07-25 adversarial spec review (0 blocker, 3 major, 3 minor):
**M1** a single serializer for the create+merge paths (Go sorts map keys, so the on-disk order is
alphabetical — the idempotency proof requires one serializer, and the illustrative shapes/tests now assert the
real bytes); **M2** the docs list gains `FLOW_GUIDE`(+ru) and the stale t51 "hook is config, not code" claims;
**M3** extract a shared `internal/fsutil.AtomicWrite` (mode-preserving) instead of a second hand-rolled
durability writer; **m4** an empty/whitespace-only existing file is treated as absent (Created), not a
refusal; **m5** the merge is nil-/type-safe over arbitrary user JSON (unknown shape ⇒ not-ours; an
incompatible existing type at a path we must extend ⇒ loud refusal, never clobber); **m6** `mtt agent hooks`
resolves the root via `projectRoot` (requires `.mtt/`, honors `--dir`/`MTT_DIR`).

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
- **Preserving arbitrary formatting / comments / key order** in an existing settings file — Claude
  `settings.json` is strict JSON (no comments); the merge re-serializes deterministically (2-space indent,
  trailing newline, **alphabetical object keys** — Go's `encoding/json`). We preserve **data** (every
  key/value), not byte layout, so a first merge on a hand-ordered file re-sorts its keys once (idempotent
  after). JSON numbers round-trip through `float64`; integers beyond 2^53 (irrelevant to a settings file)
  are not bit-preserved. Preserving original key order would need an ordered-map/token parser — YAGNI here.
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
- **Root resolution (m6).** `mtt agent hooks` resolves the project root via **`projectRoot(cmd)`** (the shared
  `--dir`/`MTT_DIR`-then-`FindRoot` resolver used by the read commands) — it is an onboarding command **for an
  already-initialized mtt project** (the scaffolded hook runs `mtt prime`, which needs `.mtt/`), so an absent
  `.mtt/` is an actionable error (`run 'mtt init' first`), not a silent scaffold into a non-project dir. `mtt
  init` instead resolves via its existing **`baseDir(cmd)`** (no `.mtt/` required — init creates it) and passes
  that root into the shared `Run`. Both honor `--dir`/`MTT_DIR`.
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

> **On-disk byte order (M1).** The block above is shown in **reading order**. On disk the single map-based
> serializer (see Merge semantics) emits object keys **alphabetically** (`command` before `type`; `PreCompact`
> before `SessionStart`; `hooks` before `permissions`) with 2-space indent + a trailing newline. **Array**
> element order (the `allow` list) is preserved as authored. Unit test 1 and the e2e assert those exact bytes —
> not the reading-order shape.

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
  `Merge`, and — iff `action != Unchanged` — `MkdirAll` the parent dir + **`fsutil.AtomicWrite(path, data,
  0o644)`**; collect `Result{Harness, Path, Action}`. A `Merge` error (malformed existing JSON, or an
  incompatible existing shape per m5) aborts that harness with a wrapped error naming the path; with one
  harness this fails the command.

The **pure `Merge`** is unit-tested exhaustively; the tiny IO wrapper is covered by CLI e2e.

### Merge semantics (Claude Code)

**One serializer for both paths (M1).** The create path and the merge path build the **same**
`map[string]any` value and run it through **one** serializer — `json.MarshalIndent(m, "", "  ")` + a trailing
`\n`. This is load-bearing for idempotency: Go's `encoding/json` **sorts object keys alphabetically**, so a
value that first landed via a struct/literal (`type` before `command`) would *not* byte-match the map-marshal
output on the next (merge) run, and the `Unchanged` check below would loop forever reporting `Merged`. Because
**both** paths marshal the same map, the on-disk bytes are the fixed point of the serializer (alphabetical
keys) and convergence holds. (Numbers round-trip through `float64` — see Non-goals; irrelevant in this domain.)

Given existing bytes:

1. **Absent, or an existing file whose content is empty/whitespace-only (m4)** ⇒ build the canonical
   `map[string]any` from scratch, serialize, action `Created`. (The `exists bool` distinguishes truly-absent
   from empty; both take this path — an empty stray `settings.json` is not "malformed".)
2. **Present and non-empty** ⇒ `json.Unmarshal` into `map[string]any` (preserves unknown keys). A parse
   failure ⇒ **error, no write** (never clobber). Then, working on the map — every access **nil-/type-safe**
   (m5): a value of an unexpected type is handled explicitly, never a panic:
   - `hooks.SessionStart` / `hooks.PreCompact` (each independently): the hook is considered **already present**
     iff **any** command string reachable under that event contains the token `mtt prime` (substring) — robust
     to a user who customized the args (e.g. `mtt prime --limit 5`), so we never append a duplicate. The
     presence walk tolerates arbitrary shapes (event not an array, an entry missing `hooks`, `hooks` not an
     array, `command` not a string) — any such shape simply reads as "not ours". If absent, **append** our
     matcher-group `{hooks:[{type:command,command:...}]}`, preserving existing entries; a missing `hooks`
     map / event array is created. **But** if `hooks` exists and is **not** a JSON object, or an event key
     exists and is **not** an array (a structure we cannot safely extend), ⇒ **loud refusal naming the path**
     (never overwrite user data).
   - `permissions.allow`: **union** our read-only entries into the existing array (existing entries first, our
     new ones appended in canonical order), **dedup** by exact string. Missing `permissions`/`allow` created;
     an existing `permissions` that is not an object, or an `allow` that is not an array ⇒ **loud refusal**
     (same no-clobber rule).
   - Every other key is untouched.
3. Serialize the (possibly modified) map with the **same** serializer. If the bytes **equal** the existing
   bytes, action `Unchanged` (no write); else `Merged`.

Idempotency proof: after a `Created`/`Merged` write, the file **is** the serializer's output; on the next run
step 2 finds `mtt prime` in both events and every allow-entry present (adds nothing), so step 3 re-serializes
the same map to the same bytes ⇒ `Unchanged`. Because both paths share the serializer, even a formatting-only
first pass (e.g. a hand-authored `type`-before-`command` file already carrying our hooks) converges in exactly
one `Merged` write, then stays `Unchanged` — asserted by a dedicated unit case
`Merge(CreatedOutput, exists=true) ⇒ Unchanged`.

### Atomic write — extract a shared `internal/fsutil` (M3)

Rather than hand-roll a **second** durability writer (the rule-of-two DRY trigger, and durability code with
platform-specific `syncDir` edge cases is exactly the wrong thing to duplicate "minimally"), **extract** the
yaml adapter's discipline into a new tiny package:

- **`internal/fsutil.AtomicWrite(path string, data []byte, fallbackMode os.FileMode) error`** — the existing
  `internal/adapter/yaml` `atomicWriteMode` behavior verbatim: `CreateTemp` in the target dir → write → `Chmod`
  (**preserve** an existing non-empty target's mode, else `fallbackMode`) → `Sync` → close → `Rename` →
  best-effort parent-dir fsync (`syncDir` — the won `errors.ErrUnsupported`/`syscall.EINVAL`/Windows handling
  moves with it). `MkdirAll` of the parent is the caller's job (both callers already do it).
- The yaml adapter's `atomicWrite`/`atomicWriteMode`/`syncDir` become thin **re-delegations** to
  `fsutil.AtomicWrite` (its `atomicWrite` = `fsutil.AtomicWrite(path, data, 0o644)`, `Current`'s fresh
  `config.local.yaml` = `…, 0o600`). Observable behavior is **byte-identical** — the adapter's existing tests
  and goldens (incl. the perm-policy and reserve-artifact cases) are the safety net and stay green.
- **`scaffold`** calls `fsutil.AtomicWrite(path, data, 0o644)` — new settings files land `0644`, and a user who
  chmod'd their `settings.json` to `0600` keeps it (mode preservation is the *better* policy, and now shared).

This is in-scope precisely because t52 is the second consumer; the change to the guarded adapter is a
mechanical extract-and-delegate, not a behavior change.

### No silent traps

- **Opt-in**: the standalone command is explicit; init's default is documented with an opt-out flag.
- **No clobber**: additive merge; malformed existing file ⇒ loud refusal.
- **Reported**: every write/merge prints its action + path (human and `--json`); init folds the same into its
  report. The user always sees that a `SessionStart`/`PreCompact` hook now runs `mtt prime`.
- **Low risk by construction**: the scaffolded hook runs our own read-only `mtt prime`, not an untrusted
  payload — so unlike a t62 external template (`sh -c` of foreign config), no confirm prompt is needed.

### Testing

- **Unit** (`internal/scaffold`, table-driven on `claude.Merge`) — assert exact **alphabetical-key** bytes:
  1. absent ⇒ `Created` with the canonical shape (assert both events + the allow set, byte-exact);
  2. existing file with unrelated keys (`env`, a custom hook) ⇒ `Merged`, unrelated keys preserved, our hooks
     appended;
  3. existing file already carrying our exact hooks + allow (in alphabetical bytes) ⇒ `Unchanged`, no write;
  4. **(M1 convergence)** `Merge(Created-output, exists=true) ⇒ Unchanged` — the create output is the
     serializer's fixed point;
  5. **(M1 one-step)** a hand-authored file already carrying our hooks but with `type`-before-`command`/other
     key order ⇒ exactly one `Merged` (re-sort), then `Unchanged`;
  6. existing `SessionStart` with a **different** command ⇒ our hook appended, theirs preserved;
  7. existing `SessionStart` with a **customized** `mtt prime --limit 5` ⇒ recognized, **no duplicate**;
  8. `permissions.allow` with some overlap ⇒ union + dedup, order (existing first);
  9. malformed JSON ⇒ error, and `Run` performs no write;
  10. **(m4)** an empty / whitespace-only existing file ⇒ `Created` (not a refusal);
  11. one event present, the other absent ⇒ only the absent one gains the hook;
  12. **(m5 tolerate)** valid-JSON-but-odd shapes the presence walk must survive without panic and read as
      "not ours" (`SessionStart: "oops"`, `SessionStart: [{}]`, an entry whose `command` is a number) ⇒ our
      hook appended where safe;
  13. **(m5 refuse)** an incompatible type at a path we must extend (`hooks` not an object, an event key not an
      array, `permissions.allow` not an array) ⇒ **loud refusal**, `Run` performs no write.
- **CLI e2e** (`testscript`): `mtt agent hooks` in a temp dir ⇒ asserts `.claude/settings.json` content; a
  second run ⇒ idempotent (`unchanged`); `mtt init` (default) ⇒ scaffolds; `mtt init --no-agent-hooks` ⇒ no
  `.claude/`; `--json` surfaces for both the subcommand and init.

## Docs to update (behavior change)

Grep the whole tree for the changed flow-fact — the "sessionStart hook is config, not code" claim has
**parallel restated occurrences** (the project's own logged lesson); each must flip.

- `DESIGN.md` (+ `DESIGN.ru.md`): a **Shipped (t52)** note under the init/onboarding section — the `mtt agent`
  group, the init default + `--no-agent-hooks`, the Claude flagship + `Harness` seam, the additive-merge
  semantics (one serializer / alphabetical bytes), and the read-only allowlist rationale. **Amend the stale
  t51 claim** at `DESIGN.md:957-958` (+ `DESIGN.ru.md:972`): "the `sessionStart` hook is **config, not
  code**" → t52 makes it *also* scaffolded (config **and** code — mtt now emits it, still overridable).
  (Leave `DESIGN.md:508`/`.ru:514` — that "config, not code" is the t26/lifecycle-events auto-commit, a
  different fact.)
- `FLOW_GUIDE.md:292` (+ `FLOW_GUIDE.ru.md:295`): the **"Settings & hooks — scaffolding editor/agent settings
  and hooks (`sessionStart → mtt prime`)"** bullet currently reads as a still-future neighbour — **flip it to
  Shipped/t52** (or move it out of the future list), since t52 ships exactly this.
- `CLI_REFERENCE.md` (+ `.ru`): document `mtt agent hooks` and `mtt init --no-agent-hooks`; update the t51
  "wire it into session start" section to point at the command (the hook is **now** scaffolded, no longer only
  a manual snippet).
- `internal/cli/CLAUDE.md`: the new command wiring; **amend the stale claim** at `internal/cli/CLAUDE.md:294`
  ("`sessionStart` hook is config … **not code**"). **New** `internal/scaffold/CLAUDE.md` (package
  responsibility, the `Harness` seam, "no `pkg/mtt` domain, no `.mtt/` storage") and a one-line note in
  `internal/adapter/yaml/CLAUDE.md` that `atomicWrite` now delegates to `internal/fsutil`.
- `CHANGELOG.md`: an `[Unreleased]` entry (the `task` flow gates on it).
- `README.md`/`.ru` if the getting-started section enumerates init behavior.

## Resolved by the spec review (2026-07-25)

- **Claude permission match syntax** — confirmed valid: `Bash(mtt <sub>:*)` matches both the bare command and
  args, multi-word prefixes work, and Claude Code splits on `&&`/`;`/`|` so no allow-entry leaks into a
  chained mutating command. The read-only set was re-checked against the real command surface (`root.go`) —
  every mutating verb is excluded. No change needed.
- **Atomic write** — resolved to **extract `internal/fsutil.AtomicWrite`** (M3), not a second hand-rolled
  writer; mode preservation is the shared, better policy.
- **Map-key ordering** — resolved to **one serializer** for create+merge (M1); on-disk keys are alphabetical.
