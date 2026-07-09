# internal/adapter/yaml

Default driven adapter — config **and** task storage this session. Stores config and tasks as YAML under
`.mtt/`, owns ID-encoding (`prefix`) and ID-minting, maps DTOs↔`pkg/mtt` domain. **No business rules**
beyond provider-specific checks.

## Responsibilities

- `FindRoot` — locate `.mtt/` walking up (like git).
- `HasProject(dir)` — reports whether `dir` **directly** contains `.mtt/` (no upward walk); used by the
  CLI's `--dir`/`MTT_DIR` resolution, which is explicit-root and must not silently fall back to discovery.
- `Init` — render an embedded template (`default`/`coding`, `text/template` `{{.Name}}`), **atomic** write (temp+rename), refuse overwrite without force.
- `Load` — read config + optional gitignored `config.local.yaml` overlay (later wins at top-level-field granularity: a scalar like `project.name` overrides, but a list such as `types` replaces wholesale — no element-level merge), map DTO→domain, run provider checks (exactly one `default`; prefix present+unique). Domain `Config.Validate()` is the caller's call. Returns `Settings{Prefixes, CommandTimeout, Author, Require}`: `Require` (`require:{who,why}`, s006.5) is the required-attribution policy — **tighten-only**, so the committed `require` is captured **before** the local overlay and OR-combined (`committed || local`): `config.local` can add a requirement but never relax a committed one.
- `NewTaskStore(root)` / `Store` — implements `mtt.TaskStore`. `Create` mints a **flat per-prefix** ID (`<prefix><N>` via `mint`, scan `max+1`, `O_EXCL` reserve), serializes the `ymlTask` DTO (RFC3339 UTC, `omitempty` on reserved fields), and writes atomically to `.mtt/tasks/<id>.yaml`. `Get` reads/maps a task, returning `mtt.ErrNotFound` when absent. IDs are flat (no parent chain) → stable under re-parenting; identity lives in the ID, hierarchy in the `parent` field. `List` reads `.mtt/tasks/*.yaml` → domain (order unspecified; `core` orders). `Update` overwrites an existing task by ID (`ErrNotFound` if absent); its existence check (`os.Stat` then write) is a check-then-act with a theoretical TOCTOU window, acceptable for the single-user local CLI (same filesystem assumptions as `Create`). `Create`/`Update` share one private `write` (marshal + atomic temp+rename) — serialization lives in exactly one place. `Delete` removes `.mtt/tasks/<id>.yaml` via `os.Remove` (`ErrNotFound` when absent); the unlink is atomic, same filesystem assumptions as the writes.

## Boundaries

- The domain never sees YAML: DTOs carry the yaml tags + `prefix`; `toDomain` maps to pure types.
- **`ymlCommand` (s007; `rollback` s008)** — a transition command on disk is a bare **scalar** (a command string, back-compat) **or** a `{run, timeout, rollback}` **map**; `ymlCommand.UnmarshalYAML` dispatches on the node `Kind` and maps both to one `mtt.Command`. The map branch decodes into a local string-`Timeout` alias (never back into `ymlCommand` — infinite recursion; yaml.v3 can't decode `30s` into `time.Duration`) then `time.ParseDuration`s it, so a bad duration surfaces at `Load` (like the global `command_timeout`) and `toDomain` stays error-free. The **`rollback`** field is itself a `*ymlCommand` (scalar or map), so yaml.v3 recurses into the same `UnmarshalYAML`; `ymlCommand.toDomain()` maps a command **recursively**, deep-copying the rollback (a fresh `*mtt.Command`, not the DTO pointer). Config is never marshaled (read-only + template text), so there is no `MarshalYAML`.
- **Identity mapping**: on-disk DTOs keep plain `string` fields; `fromDomain*` casts the named identities
  (`TaskID`/`TypeName`/`StatusName`) to `string`, and `toDomain` maps back — **guarding** the required
  `id`/`type`/`status` via `mtt.NewTaskID`/`NewTypeName`/`NewStatusName` (a corrupt file with an empty one
  fails to load). Optional fields (`parent`, `depends_on`, **`priority`** — s008.6, `yaml:"priority,omitempty"`
  after `status`) use plain conversion (`mtt.Priority(yt.Priority)`; empty is legitimate, an unknown on-disk
  value round-trips as-is and ranks medium — validity is a CLI-boundary concern, not a load-time one). Unset
  priority is omitted on disk, so existing task files & goldens are byte-unchanged. **Tags** (s008.7)
  round-trip the same way via `ymlTask.Tags` (`yaml:"tags,omitempty"`, after `parent`) — a plain `[]string`
  copy, **no adapter change** and **no sort** (the adapter copies `Tags` verbatim; the normalized+sorted-set
  invariant lives in `core.canonicalTags`). Golden `task_tags.yaml` locks the field position; unset tags are
  omitted, keeping existing files byte-unchanged.
- No flow/ready/traversal logic here (that is `core`, later). Templates are the **only** home of default type/status names.
- `.mtt/config.yaml` is edited only through this adapter (determinism + validation).
- `NewCurrent(root)` / `Current` — implements `mtt.CurrentStore` (session 006.7), owning **only** the top-level
  `current:` key of `.mtt/config.local.yaml`. Read/modify/write go through a **`yaml.Node`** (not a struct
  decode) so `author`, comments, and any other local keys survive a rewrite (the file is human-edited); the
  write is atomic (temp+rename, shared `atomicWrite`). Independent of `Load` (which decodes non-strictly and
  ignores the unknown `current:` key). `Current()` returns `(_, false, nil)` when the file/key is absent (not an
  error); `SetCurrent` upserts; `ClearCurrent` deletes only that key. `ymlTransition.current` (`omitempty`)
  carries the `Transition.Current` (`set|clear`) rule; `toDomain` casts it (validated by `Config.Validate`).
