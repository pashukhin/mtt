# internal/adapter/yaml

Default driven adapter — config **and** task storage this session. Stores config and tasks as YAML under
`.mtt/`, owns ID-encoding (`prefix`) and ID-minting, maps DTOs↔`pkg/mtt` domain. **No business rules**
beyond provider-specific checks.

## Responsibilities

- `FindRoot` — locate `.mtt/` walking up (like git).
- `Init` — render an embedded template (`default`/`coding`, `text/template` `{{.Name}}`), **atomic** write (temp+rename), refuse overwrite without force.
- `Load` — read config + optional gitignored `config.local.yaml` overlay (later wins at top-level-field granularity: a scalar like `project.name` overrides, but a list such as `types` replaces wholesale — no element-level merge), map DTO→domain, run provider checks (exactly one `default`; prefix present+unique). Domain `Config.Validate()` is the caller's call.
- `NewTaskStore(root)` / `Store` — implements `mtt.TaskStore`. `Create` mints a **flat per-prefix** ID (`<prefix><N>` via `mint`, scan `max+1`, `O_EXCL` reserve), serializes the `ymlTask` DTO (RFC3339 UTC, `omitempty` on reserved fields), and writes atomically to `.mtt/tasks/<id>.yaml`. `Get` reads/maps a task, returning `mtt.ErrNotFound` when absent. IDs are flat (no parent chain) → stable under re-parenting; identity lives in the ID, hierarchy in the `parent` field.

## Boundaries

- The domain never sees YAML: DTOs carry the yaml tags + `prefix`; `toDomain` maps to pure types.
- No flow/ready/traversal logic here (that is `core`, later). Templates are the **only** home of default type/status names.
- `.mtt/config.yaml` is edited only through this adapter (determinism + validation).
