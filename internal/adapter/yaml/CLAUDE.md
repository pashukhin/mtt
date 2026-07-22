# internal/adapter/yaml

Default driven adapter — config **and** task storage this session. Stores config and tasks as YAML under
`.mtt/`, owns ID-encoding (`prefix`) and ID-minting, maps DTOs↔`pkg/mtt` domain. **No business rules**
beyond provider-specific checks.

## Responsibilities

- `FindRoot` — locate `.mtt/` walking up (like git).
- `HasProject(dir)` — reports whether `dir` **directly** contains `.mtt/` (no upward walk); used by the
  CLI's `--dir`/`MTT_DIR` resolution, which is explicit-root and must not silently fall back to discovery.
- `Init` — render an embedded template (`default`/`coding`/`hierarchy`, `text/template` `{{.Name}}`), **atomic** write (temp+rename), refuse overwrite without force.
- `Load` — read config + optional gitignored `config.local.yaml` overlay (later wins at top-level-field granularity: a scalar like `project.name` overrides, but a list such as `types` replaces wholesale — no element-level merge), map DTO→domain, run provider checks (exactly one `default`; prefix present+unique). Domain `Config.Validate()` is the caller's call. Returns `Settings{Prefixes, CommandTimeout, Author, Require}`: `Require` (`require:{who,why}`, s006.5) is the required-attribution policy — **tighten-only**, so the committed `require` is captured **before** the local overlay and OR-combined (`committed || local`): `config.local` can add a requirement but never relax a committed one.
- `NewTaskStore(root)` / `Store` — implements `mtt.TaskStore`. `Create` mints a **flat per-prefix** ID (`<prefix><N>` via `mint`, scan `max+1`, `O_EXCL` reserve), serializes the `ymlTask` DTO (RFC3339 UTC, `omitempty` on reserved fields), and writes atomically to `.mtt/tasks/<id>.yaml`. `Get` reads/maps a task, returning `mtt.ErrNotFound` when absent. IDs are flat (no parent chain) → stable under re-parenting; identity lives in the ID, hierarchy in the `parent` field. `List` reads `.mtt/tasks/*.yaml` → domain (order unspecified; `core` orders). Both read paths share `parseTaskFile(path, data)` (unmarshal → **id guard** → `toDomain`), which enforces the store's **load invariant** before mapping (**c15**): the in-file `id:` must equal the **filename stem** (fail-closed on the duplicate-id split-brain — a copied file that kept the other's inner id) **and** match the adapter's id encoding `^[a-zA-Z]+[0-9]+$` (`idPattern` — `<prefix><N>`). The charset guard is the **shell-safety** boundary: a loaded id is expanded into `{{.ID}}` inside gate/post `sh -c` commands, so an id carrying shell metacharacters/whitespace is refused at load, before any command runs — the domain `NewTaskID` stays opaque/permissive (this "prefix+digits" charset is **adapter** encoding, not a domain rule). A `toDomain` failure (a corrupt or zero-byte task file — e.g. the mint reserve window) is wrapped with the **offending file path** in both `List` and `Get` (`fmt.Errorf("%s: %w", path, err)`, A1) so it is a one-command fix at volume; the `ErrNotFound` (absent-file) path is untouched, so a genuinely missing task still maps to exit 4. `Update` overwrites an existing task by ID (`ErrNotFound` if absent); its existence check (`os.Stat` then write) is a check-then-act with a theoretical TOCTOU window, acceptable for the single-user local CLI (same filesystem assumptions as `Create`). `Create`/`Update` share one private `write` (marshal + atomic temp+rename) — serialization lives in exactly one place. `Delete` removes `.mtt/tasks/<id>.yaml` via `os.Remove` (`ErrNotFound` when absent); the unlink is atomic, same filesystem assumptions as the writes.
- `NewKnowledgeStore(root)` / `NoteStore` (t47) — implements `mtt.KnowledgeStore` over `.mtt/knowledge/<slug>.md`, one markdown file per note. **Serialization contract** (`note_dto.go`, `marshalNote`/`parseNote`): `"---\n"` + a **struct** frontmatter DTO (`ymlNote{title,tags,created,updated}` — deterministic order, `omitempty` title/tags; slug is the **file name**, never in frontmatter) + `"---\n"` + the body **verbatim** (trailing-newline preserved). Read splits on the **first** closing `---` and unmarshals **only** the frontmatter — **never** the whole file (`\n---\n` is yaml's doc separator), so `---` inside the body is safe; a file not starting with `---` is a load error (not `ErrNotFound`). `CreateNote` is **reserve-then-write** (`O_CREATE|O_EXCL` on the final path — mirroring `mint` — then `atomicWrite`), so an existing slug errors without clobbering. Every path-building method (`Create`/`Get`/`Update`/`Delete`) and `ListNotes` **re-validate** the slug (`NoteSlug.Valid()` / `NewNoteSlug` on the filename) — defense-in-depth against a raw `NoteSlug("../x")` cast. Golden `note_min.md`/`note_full.md` pin the layout. No versioning/search (t6). **(t1)** `ymlNote.Refs`
(`yaml:"refs,omitempty"`, after `tags`, before `created`) round-trips `Note.Refs` via the **existing**
`fromDomainRefs`/`toDomainRefs` (the task DTO's `ymlRef` — no second ref DTO); golden `note_refs.md`. A
refs-free note stays byte-identical (`omitempty`). **(t51)** `ymlNote.Priority` (`yaml:"priority,omitempty"`,
after `tags`, before `refs`) plain-copies `Note.Priority` (like `ymlTask.Priority`); a corrupt on-disk value
round-trips as-is and ranks medium (validity is a CLI concern); golden `note_priority.md`; priority-less notes
stay byte-identical.

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
  omitted, keeping existing files byte-unchanged. **Status `default`** (s008.97/A2): `ymlStatus.Default`
  (`yaml:"default,omitempty"`) maps to `mtt.Status.Default` in `toDomain`, so a `default: true` marker on an
  initial status is honored by `Type.InitialStatus()` via `Load` (previously silently dropped; the fallback
  first-initial always won). **Transition `name`** (s008.98): `ymlTransition.Name` (`yaml:"name,omitempty"`,
  after `to`) maps to `mtt.Transition.Name` — the edge verb for the `mtt <edge>` sugar / `mtt do`; a plain copy,
  validity (uniqueness/disjointness) is a `Config.Validate` concern, not a load-time one. **Per-edge `require`**
  (t5): `ymlTransition.Require` (`yaml:"require,omitempty"`, reusing the existing `ymlRequire{who,why}` shape)
  maps to `mtt.Transition.Require` in `toDomain` — decode-only (config is never marshaled), zero on edges that
  omit it; `core` unions it with the global policy. **Per-edge `post`** (t21): `ymlTransition.Post []ymlCommand`
  (`yaml:"post,omitempty"`, reusing `ymlCommand` scalar-or-map) maps to `mtt.Transition.Post` beside `Commands`
  — the finalization commands `core` runs after persist; decode-only, absent on edges that omit it.
- No flow/ready/traversal logic here (that is `core`, later). Templates are the **only** home of default type/status names.
- A project's `.mtt/config.yaml` is normally produced by `Init` templates; this repo's own committed config
  is hand-authored, reviewed like code, and guarded by `TestRepoDogfoodConfig` (`dogfood_test.go` — a
  deliberately non-hermetic test: it pins the repo root via `go.mod` and loads a temp COPY of the committed
  config, bypassing the `config.local` overlay). Task files are written only through the adapter.
- `NewAuditStore(root)` / `AuditStore` — implements `mtt.AuditStore` (t5), the append-only audit trail for
  out-of-flow dangerous actions (`rm --force`). `Append` `MkdirAll`s `.mtt`, then writes **one JSON line** per
  record (`O_APPEND|O_CREATE`) to `.mtt/audit.log`: `{at (RFC3339 UTC), who, why, action, id}` via a private
  `auditLine` DTO (keeps `pkg/mtt` free of json tags). Append-only — no read/parse surface. The log is
  committed with `.gitattributes` `merge=union` (append conflicts dissolve on branch merges).
- `NewCurrent(root)` / `Current` — implements `mtt.CurrentStore` (session 006.7), owning **only** the top-level
  `current:` key of `.mtt/config.local.yaml`. Read/modify/write go through a **`yaml.Node`** (not a struct
  decode) so `author`, comments, and any other local keys survive a rewrite (the file is human-edited); the
  write is atomic (temp+rename, shared `atomicWrite`). Independent of `Load` (which decodes non-strictly and
  ignores the unknown `current:` key). `Current()` returns `(_, false, nil)` when the file/key is absent (not an
  error); `SetCurrent` upserts; `ClearCurrent` deletes only that key. `ymlTransition.current` (`omitempty`)
  carries the `Transition.Current` (`set|clear`) rule; `toDomain` casts it (validated by `Config.Validate`).
