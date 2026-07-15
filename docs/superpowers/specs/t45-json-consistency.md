# t45 — JSON output consistency (`types` / `version` / `init` / `rm`)

Status: spec (decision record). Type: task. Branch: `task/t45`.

## Context / problem

`internal/cli/root.go`'s `Long` asserts **"All commands support --json"**, but four surfaces ignore
`--json` and print human text (never call `jsonFlag(cmd)`):

- `mtt types` — `formatTypes` renders human blocks. **Worst offender**: an agent introspecting the flow via
  `mtt types --json` gets prose, not the graph.
- `mtt version` — prints `resolveVersion()` (t30) as a bare line.
- `mtt init` — prints `initialized .mtt/config.yaml (template "…")`.
- `mtt rm <id>` **single path** (`runRmSingle`) — prints `removed <id>`. (The bulk `rm` path already emits a
  per-item JSON array via `reportBulk`; `mtt do` delegates to `runTransition`, which already honors `--json`.)

The shared machinery already exists: `jsonFlag(cmd)`, `writeJSON(w, v)`, and the dedicated `*JSON`-struct
pattern (`taskJSON`/`showJSON` in `json.go`). This task closes the four gaps so the promise is **literally
true**, prioritizing `types --json` (the agent-introspection surface).

Framing (settled with the maintainer): **implement all four** (not narrow the promise); `types --json` carries
the **full flow graph** (incl. gates/post/rollback); the `rm` single-path gap is **folded into t45**.

## Decisions

### D1 — Implement `--json` for `types`, `version`, `init`, and `rm` (single path)

Each command, under `--json`, emits its machine-readable shape via `writeJSON` **instead of** the human text
(same pattern as every other command). After this, the root `Long` "All commands support --json" holds — no
narrowing.

### D2 — `types --json`: the full flow graph

An **always-non-null array** of type objects (even `mtt types <type>`, which filters to a one-element array;
an unknown type is an error, exit 1 — same as the human path). Config is validated first (as the human path
does); an invalid config errors identically.

```json
[
  {
    "name": "task",
    "prefix": "t",
    "parents": [],                       // always present; [] for a root
    "default": true,                     // omitted unless true
    "description": "…",                  // omitted when empty
    "statuses": [ { "name": "tbd", "kind": "initial" }, … ],   // always present
    "transitions": [                                            // always present
      {
        "name": "start",                 // omitted for an unnamed edge
        "from": "tbd",
        "to": "speccing",
        "description": "…",              // omitted when empty
        "commands": [                    // always present ([] when none)
          {
            "run": "make check",
            "timeout": "10m0s",          // Duration.String(); omitted when 0
            "rollback": { "run": "…", "timeout": "…" }   // omitted when nil
          }
        ],
        "post": [ { "run": "git push …", "timeout": "…" } ]    // always present ([] when none)
      }
    ]
  }
]
```

Conventions (consistent with the repo): structural arrays (`parents`, `statuses`, `transitions`, `commands`,
`post`) are **non-null** (`[]`, never `null`) — the roadmap `idStrings` precedent — so a consumer can always
iterate; scalar optionals (`description`, edge `name`, `timeout`, `default`) and the `rollback` object are
`omitempty`. Timeouts are strings from `time.Duration.String()` (matches the human `(timeout 10m0s)`).

### D3 — `version --json`: `{ "version": "<resolveVersion()>" }`

A single object wrapping the t30 resolver. Not an array (it is a scalar fact).

### D4 — `init --json`: a summary object

```json
{ "path": "<base>/.mtt/config.yaml", "template": "default", "name": "<project>", "created": true }
```

The CLI already holds `base`/`template`/`name`; `path = filepath.Join(base, ".mtt", "config.yaml")`. No
adapter change — `yaml.Init` is unchanged; the CLI builds the summary after a successful write. `created` is
`true` on success (including a `--force` overwrite — a file was written).

### D5 — `rm <id> --json` (single path): the removed task object

Mirrors `add --json` (which emits the *created* task): emit `toTaskJSON(removed)` — a **single object** (bulk
`rm --json` stays a per-item array). Because `rm` deletes, the task is **captured before removal** (load via
the store, then `Remover.Remove`, then emit the captured view on success). `--dry-run --json` and the
not-found/attribution errors keep their current exit codes (4 / 2); only the success output changes under
`--json`.

### D6 — Keep the root `Long` claim (now honest)

After D1+D5 the "All commands support --json" claim is true, so it stays. Acceptance is **behavioral** (each
command emits valid JSON under `--json`), not a naive "every constructor calls `jsonFlag`" grep — `mtt do`
legitimately delegates to `runTransition`.

## Schema location (impl sketch, detailed in the plan)

New view structs beside `taskJSON` (in `json.go` or a `types_json.go`): `typeJSON`, `statusJSON`,
`transitionJSON`, `commandJSON`, `rollbackJSON`, plus `versionJSON{Version}` and
`initJSON{Path, Template, Name, Created}`, built by pure mappers from `mtt.Config` (like `toTaskJSON`). The CLI
layer stays thin: build the view, `writeJSON`.

## Scope / cross-refs

- **In:** `--json` for `types` / `version` / `init` / `rm`-single, their view structs + mappers, tests, and
  the `CLI_REFERENCE` (EN+RU) doc for these shapes.
- **Out:** no new commands; the exhaustive docs audit is **t42** (t45's own doc edits shrink its load); the
  `version` resolution itself is t30 (done) — t45 only wraps it.

## Acceptance criteria

1. `mtt types --json` emits the type array with `statuses`, `transitions`, and per-transition `commands`
   (incl. a `timeout` and a `rollback` where configured) and `post`; `mtt types <type> --json` is a
   one-element array; `mtt types <unknown> --json` errors (exit 1); an invalid config errors as in the human
   path.
2. `mtt version --json` → `{"version": …}` (the resolved value); `mtt init --json` → the summary object;
   `mtt rm <id> --json` → the removed task object; `mtt rm <a> <b> --json` → the per-item array (unchanged).
3. Every command emits valid JSON under `--json` (the root `Long` claim is true) — verified by e2e over
   `types`/`version`/`init`/`rm`, and the pre-existing coverage for the rest.
4. `CLI_REFERENCE.md` ↔ `CLI_REFERENCE.ru.md` document the new shapes (EN source of truth, RU synced).
5. `make check` green.

## Testing approach

- e2e (testscript): extend `types.txt` (assert `--json` keys: `"statuses"`, `"kind"`, `"transitions"`,
  `"run"`, `"timeout"`, `"post"`; the single-type filter; the unknown-type error). Add `version --json` /
  `init --json` cases (own scripts or fold into existing). Extend `rm.txt`: single `--json` emits the removed
  task object; bulk `--json` unchanged.
- Unit: the pure mappers (`toTypeJSON` over a small `mtt.Config`) — assert non-null arrays, omitempty scalars,
  `timeout` string formatting, `rollback` presence/absence.
