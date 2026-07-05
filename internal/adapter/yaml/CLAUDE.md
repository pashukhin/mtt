# internal/adapter/yaml

Default driven adapter — config **and** task storage this session. Stores config and tasks as YAML under
`.mtt/`, owns ID-encoding (`prefix`) and ID-minting, maps DTOs↔`pkg/mtt` domain. **No business rules**
beyond provider-specific checks.

## Responsibilities

- `FindRoot` — locate `.mtt/` walking up (like git).
- `HasProject(dir)` — reports whether `dir` **directly** contains `.mtt/` (no upward walk); used by the
  CLI's `--dir`/`MTT_DIR` resolution, which is explicit-root and must not silently fall back to discovery.
- `Init` — render an embedded template (`default`/`coding`, `text/template` `{{.Name}}`), **atomic** write (temp+rename), refuse overwrite without force.
- `Load` — read config + optional gitignored `config.local.yaml` overlay (later wins at top-level-field granularity: a scalar like `project.name` overrides, but a list such as `types` replaces wholesale — no element-level merge), map DTO→domain, run provider checks (exactly one `default`; prefix present+unique). Domain `Config.Validate()` is the caller's call.
- `NewTaskStore(root)` / `Store` — implements `mtt.TaskStore`. `Create` mints a **flat per-prefix** ID (`<prefix><N>` via `mint`, scan `max+1`, `O_EXCL` reserve), serializes the `ymlTask` DTO (RFC3339 UTC, `omitempty` on reserved fields), and writes atomically to `.mtt/tasks/<id>.yaml`. `Get` reads/maps a task, returning `mtt.ErrNotFound` when absent. IDs are flat (no parent chain) → stable under re-parenting; identity lives in the ID, hierarchy in the `parent` field. `List` reads `.mtt/tasks/*.yaml` → domain (order unspecified; `core` orders). `Update` overwrites an existing task by ID (`ErrNotFound` if absent); its existence check (`os.Stat` then write) is a check-then-act with a theoretical TOCTOU window, acceptable for the single-user local CLI (same filesystem assumptions as `Create`). `Create`/`Update` share one private `write` (marshal + atomic temp+rename) — serialization lives in exactly one place.

## Boundaries

- The domain never sees YAML: DTOs carry the yaml tags + `prefix`; `toDomain` maps to pure types.
- **Identity mapping**: on-disk DTOs keep plain `string` fields; `fromDomain*` casts the named identities
  (`TaskID`/`TypeName`/`StatusName`) to `string`, and `toDomain` maps back — **guarding** the required
  `id`/`type`/`status` via `mtt.NewTaskID`/`NewTypeName`/`NewStatusName` (a corrupt file with an empty one
  fails to load). Optional fields (`parent`, `depends_on`) use plain conversion (empty is legitimate).
- No flow/ready/traversal logic here (that is `core`, later). Templates are the **only** home of default type/status names.
- `.mtt/config.yaml` is edited only through this adapter (determinism + validation).
