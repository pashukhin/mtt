# t45 — JSON output consistency (`types` / `version` / `init` / `rm` / `use`)

Status: spec (decision record). Type: task. Branch: `task/t45`.

## Context / problem

`internal/cli/root.go`'s `Long` asserts **"All commands support --json"**, but several surfaces ignore
`--json` and print human text:

- `mtt types` — `formatTypes` renders human blocks. **Worst offender**: an agent introspecting the flow via
  `mtt types --json` gets prose, not the graph.
- `mtt version` — prints `resolveVersion()` (t30) as a bare line.
- `mtt init` — prints `initialized .mtt/config.yaml (template "…")`.
- `mtt rm <id>` **single path** (`runRmSingle`, `rm.go`) — prints `removed <id>`. (The bulk `rm` path already
  emits a per-item JSON array via `reportBulk`.)
- `mtt use --clear` (`use.go:35` → `current cleared`) and `mtt use` with **no current** (`use.go:63` →
  `no current task`) — text only. (`use <id>` and `use` *with* a current already emit the task JSON.)
  *Surfaced by spec_review — a 5th gap the original spec missed; without it the promise stays false.*

Already-compliant (no change): `mtt do` delegates to `runTransition`, which honors `--json`; `add`/`edit`/
`show`/`list`/`tree`/`tags`/`roadmap`/`ready`/`status`/`dep`/`tag` all branch on `jsonFlag`. Cobra's
generated `completion` and `help` are framework built-ins, not mtt's own commands (see D7).

The shared machinery exists: `jsonFlag(cmd)`, `writeJSON(w, v)`, and the dedicated `*JSON`-struct pattern
(`taskJSON`/`showJSON` in `json.go`). This task closes the gaps so the promise is **literally true**,
prioritizing `types --json` (the agent-introspection surface).

Framing (maintainer-settled, then extended by spec_review): **implement** all gaps (not narrow); `types
--json` carries the **full flow graph**; fold the `rm`-single and `use` gaps into t45.

## Decisions

### D1 — Implement `--json` for `types`, `version`, `init`, `rm`(single), `use`(clear / no-current)

Each command, under `--json`, emits its machine-readable shape via `writeJSON` **instead of** the human text
(the pattern every other command follows). After this, every mtt command supports `--json` (D7).

### D2 — `types --json`: the full flow graph

An **always-non-null array** of type objects (`mtt types <type>` filters to a one-element array; an unknown
type errors, exit 1 — as the human path; config is `Validate`d first, so an invalid config errors
identically).

```json
[
  {
    "name": "task",
    "prefix": "t",                       // from settings.Prefixes (NOT a mtt.Type field)
    "parents": [],                       // always present; [] for a root
    "default": true,                     // omitted unless true
    "description": "…",                  // omitted when empty
    "statuses": [                        // always present
      { "name": "tbd", "kind": "initial", "default": true, "description": "…" }
      // status "default" omitted unless true (THE entry status when a flow has >1 initial);
      // status "description" omitted when empty
    ],
    "transitions": [                     // always present
      {
        "name": "start",                 // omitted for an unnamed edge
        "from": "tbd",
        "to": "speccing",
        "description": "…",              // omitted when empty
        "current": "set",                // "set" | "clear"; omitted when the edge sets no current action
        "require": { "who": true, "why": true },   // per-edge attribution; omitted when zero (no requirement)
        "commands": [                    // always present ([] when none)
          { "run": "make check", "timeout": "10m0s", "rollback": { "run": "…", "timeout": "…" } }
        ],
        "post": [ { "run": "git push …", "timeout": "…" } ]   // always present ([] when none)
      }
    ]
  }
]
```

Field sourcing and completeness (the spec_review fixes):

- **`prefix`** is **not** on `mtt.Type` (`pkg/mtt/CLAUDE.md`: no adapter fields on the domain type); it
  arrives via `settings.Prefixes` (as `formatTypes(cfg, settings.Prefixes, …)` already does). So the type
  mapper takes `(t mtt.Type, prefix string)` — it is **not** a pure `Config` mapper.
- **`require`** (`Transition.Require`) and **`current`** (`Transition.Current`) are included — an agent
  preflighting a move needs to know which edges demand `--who`/`--why` (a missing one is exit 2 **before** the
  gate) and which edges move the `current` pointer. Omitting them is why the original "full graph" claim was
  false.
- **status `default`** (`Status.Default`, the entry marker when >1 initial) and **status `description`** are
  included — otherwise an agent cannot tell which `kind:"initial"` is the entry, and loses the authored
  per-status guidance that `show --json` already exposes.

Conventions (matching the repo): structural arrays (`parents`, `statuses`, `transitions`, `commands`,
`post`) are **non-null** (`[]`, never `null`) — the roadmap `idStrings` precedent; scalar optionals
(`description`, edge `name`, `timeout`, type/status `default`), plus `require`, `current`, and the `rollback`
object, are `omitempty`. **Timeouts** are strings from `time.Duration.String()` (`"10m0s"`, omitted when 0) —
this matches the human `(timeout 10m0s)` and **establishes the string-duration convention** for future JSON
duration fields (a non-Go consumer parses it like `time.ParseDuration`).

### D3 — `version --json`: `{ "version": "<resolveVersion()>" }`

A single object wrapping the t30 resolver (a scalar fact, so an object, not an array).

### D4 — `init --json`: a summary object

```json
{ "path": "<abs>/.mtt/config.yaml", "template": "default", "name": "<project>", "created": true }
```

`path = filepath.Join(absBase, ".mtt", "config.yaml")` where `absBase = filepath.Abs(baseDir(cmd))` — the
`Abs` matters because `baseDir` returns the raw `--dir`/`MTT_DIR` value, which may be relative; the human
output never exposed a path, so JSON is the sole consumer and should be usable. `yaml.Init` is unchanged (no
adapter change — the CLI builds the summary from `base`/`template`/`name` after a successful write).
`created` is **always `true`** on the emitted payload (Init errors before the JSON on any non-success,
including already-initialized without `--force`); it is kept as an explicit success marker.

### D5 — `rm <id> --json` (single path): the removed task object

Emit `toTaskJSON(removed)` — mirroring `add --json` (which emits the *created* task). `Remover.Remove` returns
**only an error** (`internal/core/remove.go:39`), so the CLI **captures the task via the store before**
removing (`Get` → `Remove` → emit the captured view on success).

Single and bulk `rm --json` are **deliberately different shapes**, documented rather than reconciled:

- **single** → the task object; its `status` field is the task's **workflow state** (e.g. `"tbd"`).
- **bulk** → the per-item outcome array `[{ "id": …, "status": "removed"|"error", … }]` (`bulk.go`), where
  `status` is the **operation outcome**.

A consumer distinguishes them by object-vs-array (single has no array form, exactly like `add`). The `status`
key's dual meaning across the two forms is intentional — "show me what I deleted" (single) and "per-item
outcome" (bulk) are different questions. `--dry-run` and the not-found/attribution errors keep their exit
codes (4 / 2); only the success payload changes under `--json`.

### D6 — `use --json`: the current task, or `null`

One uniform model: `use --json` emits the current task (`toTaskJSON`) or JSON **`null`** when there is none.
So `use <id>` and `use` *with* a current → the task; `use` with **no** current **and** `use --clear` → `null`
(the pointer's value after the operation). This folds the two text-only branches (`current cleared`,
`no current task`).

### D7 — the root `Long` claim (now honest)

After D1/D5/D6, **every mtt command emits JSON under `--json`.** Cobra's generated `completion` and `help`
do not — they are framework scaffolding, not mtt commands, and `--json` is meaningless for them; the spec
records this carve-out. Keep the `Long` claim. Acceptance is **behavioral** (each command emits valid JSON
under `--json`), not a naive "every constructor calls `jsonFlag`" grep — `mtt do` legitimately delegates.

## Schema location (impl sketch, detailed in the plan)

View structs beside `taskJSON` (in `json.go` or a new `types_json.go`): `typeJSON`, `statusJSON`,
`transitionJSON`, `commandJSON`, `rollbackJSON`, `requireJSON`, plus `versionJSON{Version}` and
`initJSON{Path, Template, Name, Created}`. The type mapper is `toTypeJSON(t mtt.Type, prefix string)` (needs
the prefix from `settings.Prefixes`); the rest are pure `mtt.Config`/value mappers (like `toTaskJSON`). The
CLI layer stays thin: build the view, `writeJSON`.

**Impl caveat:** `require` must be modeled as a **pointer** `*requireJSON` (nil when `Require{}` is zero) —
Go's `encoding/json` does **not** honor `omitempty` on a non-pointer struct value, so a value field would
always serialize `"require":{"who":false,"why":false}`. (`current` is a string and `rollback` is already a
pointer, so those omit correctly as-is.)

## Scope / cross-refs

- **In:** `--json` for `types` / `version` / `init` / `rm`-single / `use`-clear+empty, their view structs +
  mappers, tests, and the `CLI_REFERENCE` (EN+RU) doc for these shapes.
- **Out:** no new commands; cobra `completion`/`help` (built-ins); the exhaustive docs audit is **t42** (t45's
  own doc edits shrink its load); the `version` resolution itself is t30 (done) — t45 only wraps it.

## Acceptance criteria

1. `mtt types --json` emits the type array with `statuses` (incl. `kind`, and `default`/`description` where
   set) and `transitions` (incl. `name`, `current`, `require`, per-transition `commands` with a `timeout` and
   a `rollback` where configured, and `post`); `mtt types <type> --json` is a one-element array; `mtt types
   <unknown> --json` errors (exit 1); an invalid config errors as in the human path.
2. `mtt version --json` → `{"version": …}`; `mtt init --json` → the summary object (absolute `path`); `mtt rm
   <id> --json` → the removed task object; `mtt rm <a> <b> --json` → the per-item array (unchanged); `mtt use
   --json` → the current task or `null` (incl. `--clear` and no-current → `null`).
3. Every mtt command emits valid JSON under `--json` (cobra `completion`/`help` excepted) — verified by e2e
   over the changed surfaces plus the pre-existing coverage for the rest.
4. `CLI_REFERENCE.md` ↔ `CLI_REFERENCE.ru.md` document the new shapes (EN source of truth, RU synced).
5. `make check` green.

## Testing approach

- e2e (testscript): extend `types.txt` (assert `--json` keys: `"statuses"`, `"kind"`, `"default"`,
  `"transitions"`, `"current"`, `"require"`, `"run"`, `"timeout"`, `"post"`; the single-type filter; the
  unknown-type error). Extend `rm.txt` (single `--json` → removed task object; bulk unchanged). Extend
  `use.txt` (`use --clear --json` and no-current `--json` → `null`; with-current → the task). Add `version
  --json` / `init --json` cases.
- Unit: the pure mappers over a small `mtt.Config` (non-null structural arrays; omitempty scalars; `require`
  and `current` presence/absence; status `default`/`description`; `timeout` string formatting; `rollback`
  presence/absence).
