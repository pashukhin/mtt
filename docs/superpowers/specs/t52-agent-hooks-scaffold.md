# t52 — Scaffold agent settings + hooks (spec)

Status: revision 4 — rev1 decided in the 2026-07-25 brainstorm (four user decisions: harness breadth, merge
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

rev3 addresses the 2026-07-25 **second** adversarial spec review (0 blocker, 3 major, 3 minor — all small, all
in rev2's own fixes or a gap it surfaced): **R1** the one serializer must be a `json.Encoder` with
`SetEscapeHTML(false)` — `json.MarshalIndent` HTML-escapes `>` and would write our own `mtt prime 2>/dev/null`
(verified empirically); **R2** the `fsutil` extraction must also export `SyncDir` and repoint `audit.go`'s
direct `syncDir` caller (else the adapter won't compile — "tests stay green" was false); **R3** define
`mtt init`'s config-then-scaffold **failure contract** (config is written first; a scaffold error is exit 1
with config-success reported first) + `Run`'s multi-harness/read-error semantics; **r4** an explicit JSON
`null` at a container path ⇒ treated as absent (create), like m4; **r5** the presence predicate is a
leading-`mtt prime` **prefix** match (after trimming), not a loose substring; **r6** `Run` treats only
`os.IsNotExist` as absent, other read errors propagate.

rev4 addresses the 2026-07-25 **third** spec review (all six round-2 findings verified FIXED; 1 major + 3
minor remained): **D1** the docs checklist must also create `internal/fsutil/CLAUDE.md` (the non-negotiable
"every `internal/` package keeps its own CLAUDE.md" rule for the newly-extracted package); **d2** the stale
`Status: revision 2` header (now correct); **d3** a test for r6's non-`IsNotExist` read-error propagation
(a `.claude/settings.json` that is a **directory**); **d4** r5 tightened to a **word-boundary** prefix (equal
to `mtt prime` or followed by a space) so `mtt primer` does not match.

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
- **init failure contract (R3).** The config write is init's **primary job** and runs **first**; the scaffold
  runs **after**, so a scaffold failure can only occur once config is already on disk. If `Run` errors (a
  pre-existing malformed / m5-incompatible `.claude/settings.json`), init **reports the config success first**
  (human: the existing `initialized …` line; then the scaffold error on stderr) and returns the **wrapped
  scaffold error → exit 1** — `.mtt/config.yaml` is **kept** (it was written and is valid; no rollback of a
  completed primary job, mirroring the t21/t28 "the change IS saved" posture). In `--json` mode, init prints
  **no** partial JSON object on a scaffold failure (a half-object would misread as success); the error goes to
  stderr and the non-zero exit is the signal. (A config-write failure — e.g. `ErrAlreadyInitialized` without
  `--force` — aborts **before** the scaffold, unchanged from today.)
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

> **On-disk byte order (M1/R1).** The block above is shown in **reading order**. On disk the single serializer
> (see Merge semantics) emits object keys **alphabetically** (`command` before `type`; `PreCompact` before
> `SessionStart`; `hooks` before `permissions`) with 2-space indent + a trailing newline. `>` is **not**
> escaped (`SetEscapeHTML(false)`), so `2>/dev/null` is written literally. **Array** element order (the `allow`
> list) is preserved as authored. Unit test 1 and the e2e assert those exact bytes — not the reading-order shape.

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
  hooks` and `mtt init` (DRY): for each harness, read `<root>/<RelPath>`, call `Merge`, and — iff `action !=
  Unchanged` — `MkdirAll` the parent dir + **`fsutil.AtomicWrite(path, data, 0o644)`**; collect
  `Result{Harness, Path, Action}`. Semantics pinned for the seam (R3/r6):
  - **Read errors (r6):** only `os.IsNotExist` ⇒ `exists=false` (⇒ `Merge` may `Create`); **any other** read
    error (EACCES, …) is **propagated**, never mistaken for absent.
  - **Failure = stop, return partial results (R3):** a `Merge` error (malformed existing JSON, or an
    incompatible existing shape per m5) or a write error aborts the loop and returns `(resultsSoFar, wrapped
    error naming the harness+path)` — **write-as-you-go**, so already-processed harnesses' `Result`s come back
    alongside the error (callers can report what did land). With t52's single harness this is simply "the
    command fails"; the contract is fixed **now** so the deferred second harness has defined behavior.

The **pure `Merge`** is unit-tested exhaustively; the tiny IO wrapper is covered by CLI e2e.

### Merge semantics (Claude Code)

**One serializer for both paths (M1/R1).** The create path and the merge path build the **same**
`map[string]any` value and run it through **one** serializer: a `json.Encoder` with **`SetEscapeHTML(false)`**
and `SetIndent("", "  ")` (`enc.Encode(m)` already appends the trailing `\n` — do **not** double-add). This is
load-bearing for idempotency on two counts:
- Go's `encoding/json` **sorts object keys alphabetically**, so a value that first landed via a struct/literal
  (`type` before `command`) would *not* byte-match the map-marshal output on the next (merge) run, and the
  `Unchanged` check below would loop forever reporting `Merged`. Because **both** paths marshal the same map,
  the on-disk bytes are the serializer's fixed point (alphabetical keys) and convergence holds.
- **`SetEscapeHTML(false)` is mandatory (R1):** the default `json.MarshalIndent` HTML-escapes `<`, `>`, `&` —
  our own hook command contains `2>/dev/null`, which the default emits as the escaped `2>/dev/null`. That
  both contradicts the field-tested literal shape and would rewrite this repo's readable `.claude/settings.json`
  into the `>` form on first merge. Verified empirically (`json.MarshalIndent` → `2>/dev/null`;
  `json.Encoder`+`SetEscapeHTML(false)` → `2>/dev/null`, keys still alphabetical). Tests assert the literal
  byte `2>/dev/null` — the exact byte the naive `MarshalIndent` gets wrong.

(Numbers round-trip through `float64` — see Non-goals; irrelevant in this domain.)

Given existing bytes:

1. **Absent, or an existing file whose content is empty/whitespace-only (m4)** ⇒ build the canonical
   `map[string]any` from scratch, serialize, action `Created`. (The `exists bool` distinguishes truly-absent
   from empty; both take this path — an empty stray `settings.json` is not "malformed".)
2. **Present and non-empty** ⇒ `json.Unmarshal` into `map[string]any` (preserves unknown keys). A parse
   failure ⇒ **error, no write** (never clobber). Then, working on the map — every access **nil-/type-safe**
   (m5): a value of an unexpected type is handled explicitly, never a panic:
   - `hooks.SessionStart` / `hooks.PreCompact` (each independently): the hook is considered **already present**
     iff **any** command string reachable under that event, **after trimming leading whitespace, equals
     `mtt prime` or begins with `mtt prime ` (a trailing space)** — a **word-boundary** prefix (r5/d4), so
     `mtt prime --limit 5` matches but `mtt primer …` does not, and a command that merely *mentions* the phrase
     (e.g. `git commit -m "wire mtt prime"`) is correctly **not** present (a loose substring would wrongly treat
     it as present and silently suppress our hook). The presence walk
     tolerates arbitrary shapes (event not an array, an entry missing `hooks`, `hooks` not an array, `command`
     not a string) — any such shape simply reads as "not ours". If absent, **append** our matcher-group
     `{hooks:[{type:command,command:...}]}`, preserving existing entries; a missing `hooks` map / event array
     is created. **But** if `hooks` exists and is a **non-null, non-object** value, or an event key is a
     **non-null, non-array** value (a structure we cannot safely extend), ⇒ **loud refusal naming the path**
     (never overwrite user data).
   - `permissions.allow`: **union** our read-only entries into the existing array (existing entries first, our
     new ones appended in canonical order), **dedup** by exact string. Missing `permissions`/`allow` created;
     an existing `permissions` that is a non-null, non-object value, or an `allow` that is a non-null,
     non-array value ⇒ **loud refusal** (same no-clobber rule).
   - **Explicit JSON `null` at a container path (r4):** a `null` at `hooks`, an event key, `permissions`, or
     `allow` is treated as **absent** (create the container) — mirroring m4's empty-file rule (a `null`
     placeholder is not a structure to preserve, and not "malformed"), so it never trips the refusal above.
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
  best-effort parent-dir fsync via `SyncDir`. `MkdirAll` of the parent is the caller's job (both callers do it).
- **`internal/fsutil.SyncDir(dir string) error`** — the parent-dir fsync with the won
  `errors.ErrUnsupported`/`syscall.EINVAL`/Windows handling, **exported (R2)** because `syncDir` has a **second,
  direct** consumer beyond the writer chain: `internal/adapter/yaml/audit.go:60` fsyncs the `.mtt` dirent after
  the audit-log append. That caller is repointed to `fsutil.SyncDir` (or via a one-line yaml shim). Verified
  the full consumer set before extracting: `atomicWrite` (init.go:54, note.go:69, task.go:84), `atomicWriteMode`
  (current.go:115, +via atomicWrite), `syncDir` (init.go:144 in the writer, **audit.go:60 direct**).
- The yaml adapter's `atomicWrite`/`atomicWriteMode` become thin **re-delegations** to `fsutil.AtomicWrite`
  (`atomicWrite` = `fsutil.AtomicWrite(path, data, 0o644)`, `Current`'s fresh `config.local.yaml` = `…, 0o600`);
  `syncDir` becomes a shim to `fsutil.SyncDir` (or is deleted and callers repointed). Observable behavior is
  **byte-identical** — the adapter's existing tests and goldens (perm-policy, reserve-artifact, audit fsync)
  are the safety net and stay green **only because `audit.go` is repointed** (missing it fails the build).
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
  1. absent ⇒ `Created` with the canonical shape (assert both events + the allow set, byte-exact — **including
     the literal `mtt prime 2>/dev/null || true`**, the byte a naive `MarshalIndent` would `>`-escape, R1);
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
  13. **(m5 refuse)** an incompatible **non-null** type at a path we must extend (`hooks` a string, an event
      key a string, `permissions.allow` a string/object) ⇒ **loud refusal**, `Run` performs no write;
  14. **(r4 null)** `null` at `hooks` / an event / `permissions` / `allow` ⇒ treated as absent, `Created`/`Merged`
      (not a refusal);
  15. **(r5 word-boundary)** commands that must **not** count as present — a mention not-at-start
      (`git commit -m "wire mtt prime"`) and a longer word (`mtt primer`) ⇒ our hook appended (neither a loose
      substring nor a bare prefix should report `unchanged`); and `mtt prime --limit 5` **does** count.
- **CLI e2e** (`testscript`): `mtt agent hooks` in a temp dir ⇒ asserts `.claude/settings.json` content
  (literal `2>/dev/null`); a second run ⇒ idempotent (`unchanged`); `mtt init` (default) ⇒ scaffolds; `mtt
  init --no-agent-hooks` ⇒ no `.claude/`; `--json` surfaces for both the subcommand and init;
  **`mtt agent hooks` outside an mtt project ⇒ actionable error (m6)**; **(R3)** `mtt init` with a
  **pre-existing malformed `.claude/settings.json`** ⇒ exit 1, `.mtt/config.yaml` **written** (config-success
  reported first), scaffold error on stderr, and **no** partial JSON object under `--json`;
  **(r6/d3)** `.claude/settings.json` that is a **directory** ⇒ the non-`IsNotExist` read error **propagates**
  (the command fails; not mistaken for absent).

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
  ("`sessionStart` hook is config … **not code**").
- **New `internal/scaffold/CLAUDE.md`** (package responsibility, the `Harness` seam, "no `pkg/mtt` domain, no
  `.mtt/` storage").
- **New `internal/fsutil/CLAUDE.md` (D1)** — the required per-package doc for the newly-extracted package: the
  shared atomic-write + dir-fsync durability primitive lifted from the yaml adapter; no domain, no storage
  semantics (the non-negotiable `CLAUDE.md:22` "every `internal/` package keeps its own CLAUDE.md" rule).
- `internal/adapter/yaml/CLAUDE.md`: a one-line note that `atomicWrite`/`syncDir` now delegate to
  `internal/fsutil`.
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
