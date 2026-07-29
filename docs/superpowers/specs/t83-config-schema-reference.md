# t83 — Config schema reference (an authoritative `.mtt/config.yaml` doc, drift-guarded)

**Task:** t83 · **Type:** task · **Status:** speccing · **Tags:** pilot, docs
**Surfaced by:** the greenfield pilot (tessera) — to author a mature config (events/require/post), the
adopter agent could not rely on the docs and reverse-engineered the schema from Go struct tags
(`internal/adapter/yaml/dto.go`, `pkg/mtt/*.go`). This is the fix.

A **decision record**. It fixes scope, deliverable, and the anti-drift mechanism before the plan and code.

> **rev2 — response to adversarial spec review (rev1 DECLINE).** (MAJOR) sync surface now includes
> `AGENTS.md:119–122`, the second normative bilingual-pairs enumeration alongside `CLAUDE.md:31–32` (D4/§5).
> (MINOR) the DTO is **decode-only** → `omitempty` is inert; required/optional is reframed as a *validation*
> property, and D2 now **mandates** the validity rules an author needs (prefix charset, `kind` values,
> edge-name uniqueness, default-must-be-initial, duration format, `version`) — the semantic half the
> name-only test does not cover (Goal, D2). (MINOR) D3 root discovery corrected to `FindRoot(".")` + go.mod
> check.

## 1. Problem

The engine's config is code-as-config, authored by hand — but there is **no exhaustive field reference**.
FLOW_GUIDE is a **tutorial** (how to author a flow, worked examples, §1–§12); it teaches the *concepts*
(gates, `post:`, events, `require`) in prose but does not list **every field** with its type, its
required/optional status, and its validity constraints. In the pilot the adopter agent grepped FLOW_GUIDE *and* CLI_REFERENCE, found the
concepts but not an exhaustive schema, and went to `dto.go` to confirm the shape of `events:`,
`post_defaults:`, `inherit_post:`, and the `require:` semantics. An adopter who reads the source to write a
config is a documentation failure: the schema must be authorable **from docs alone**.

This also gates the next clean pilot, which asks an agent to author a flow **from scratch without leaning on
mtt as a flow guide** — impossible today without reading source.

## 2. Goal / non-goals

**Goal.** One authoritative, exhaustive, **reference-style** doc for `.mtt/config.yaml` (+ the
`config.local.yaml` overlay keys) — every key: its **type**, whether it is **required or optional** (a
*validation* property — the config DTO is **decode-only**, so `omitempty` is inert and does **not**
determine this), its **validity constraints** (charset / allowed values / formats — the very things the
pilot went to source for), a one-line **semantics**, and a **§-cross-ref** to the FLOW_GUIDE concept — plus
a **test that keeps it from silently going stale-incomplete** (the exact sin that caused this task).

**Non-goals (YAGNI):**
- **Not a tutorial.** It does not re-teach *how to author* a flow (that is FLOW_GUIDE) — it is a lookup
  reference (Diátaxis: reference ≠ tutorial). It cross-links to FLOW_GUIDE for the "why/how".
- **No Go logic change.** Only docs + one new test. No change to `cmd/`, `pkg/`, or `internal/**` logic.
- **Not a JSON-Schema / validator.** A human/agent-readable Markdown reference, not a machine schema file
  (that could be a later task; out of scope).
- **Not the CLI surface.** Commands stay in CLI_REFERENCE; this is the *config data* surface.

## 3. Decisions

### D1 — A new bilingual `CONFIG_REFERENCE.md` + `.ru.md`

- A dedicated reference manual, sibling to CLI_REFERENCE/FLOW_GUIDE — **EN canonical, RU mirror**, added to
  CLAUDE.md's bilingual-pairs list (CLAUDE.md:31–32). Reference and tutorial are different modes; keeping
  the exhaustive field table out of FLOW_GUIDE keeps the tutorial readable and gives the schema a clear home.
- Discoverability (the pilot agent looked in FLOW_GUIDE first): prominent pointers **into** it from
  FLOW_GUIDE (§1 Mental model and §11 Validate), CLI_REFERENCE (the `mtt init --template` area), and the
  README Docs list — so anyone in those docs is one hop away.

### D2 — Content: every field, grouped by nesting, reference-style

Each field documented as a **backticked** entry with: **type** · **required / optional** (from *validation*,
not `omitempty` — the DTO is decode-only, so `omitempty` is inert) · its **validity constraint** where one
applies · a one-line **semantics** · a **§-cross-ref** to FLOW_GUIDE. Layout (a table or a definition list
per level):

- **Top level** (`ymlConfig`): `version`, `project` (→ `name`), `command_timeout`, `author`,
  `extract_hashtags`, `require`, `events`, `types`.
- **Type** (`ymlType`): `name`, `prefix`, `parents`, `default`, `description`, `post_defaults`, `statuses`,
  `transitions`.
- **Status** (`ymlStatus`): `name`, `kind`, `description`, `default`.
- **Transition** (`ymlTransition`): `from`, `to`, `name`, `description`, `commands`, `current`, `require`,
  `post`, `inherit_post`.
- **Command** (`ymlCommand`): scalar string **or** the map form `run` / `timeout` / `rollback` (recursive).
- **Events** (`ymlEvents`): stores `task` / `note` × kinds `create` / `update` / `delete` × `post`.
- **Require** (`ymlRequire`): `who` / `why` — tighten-only; the attribution resolution order
  (`--who`/`--by` > `MTT_BY` > `config.local` `author`).
- **`config.local.yaml` overlay**: `author`, `current` (the two keys the overlay owns; gitignored;
  later-wins overlay semantics).
- **Placeholders** documented explicitly and separately: **edge** commands expand `{{.ID}}` `{{.Type}}`
  `{{.From}}` `{{.To}}` (a whitelist — free text like `{{.Title}}` is a template error, never interpolated);
  **task events** expand `{{.ID}}` `{{.Type}}` `{{.Event}}`, **note events** `{{.Slug}}` `{{.Event}}`.

**Validity rules the reference MUST state** (this is the semantic half — what the pilot actually went to
source for; a doc that only backticks the 33 names passes D3 but is *not* "authorable from docs alone").
These are `Config.Validate` / `checkPrefixes` rules, restated at the field level (with a §11 cross-ref):

- `version`: currently `1`.
- `prefix`: **letters-only** `[a-zA-Z]+`, **unique** across types; **exactly one** type is `default: true`.
- `kind` ∈ {`initial`, `active`, `terminal`}; **≥1 of each**; `kind` must **match the graph topology**
  (`initial` = no incoming edge, `terminal` = no outgoing, `active` = both).
- transition `name`: **unique per source status** and **disjoint from status names** (it is the `mtt <verb>`
  edge sugar).
- status `default`: **at most one**, and it **must be `initial`**.
- `command_timeout` (adapter) and command `timeout`: **Go `time.ParseDuration` format** (e.g. `5m`,
  `10m0s`); a bad value fails at `Load`.
- `current` ∈ {`set`, `clear`}.
- `commands` is **optional** despite no `omitempty` (decode-only); `require` is **tighten-only**
  (`config.local`/per-edge may add, never relax).

The **complete field-name set** the reference must cover (the test's target, from `dto.go`), 33 distinct
names: `version, project, command_timeout, author, extract_hashtags, require, events, types, name, prefix,
parents, default, description, post_defaults, statuses, transitions, kind, from, to, commands, current,
post, inherit_post, who, why, task, note, create, update, delete, run, timeout, rollback`.

### D3 — Anti-drift guard: `TestConfigReferenceCoversSchema`

A **non-hermetic** test (in `internal/adapter/yaml/`, locating the repo root via the package's exported
`FindRoot(".")` then a `go.mod` sanity check — the pattern `dogfood_test.go:60,64` already uses) that:

1. reads `internal/adapter/yaml/dto.go`, extracts **every distinct `yaml:"…"` tag name** (strip
   `,omitempty` and any options — take the token before the first comma);
2. reads `CONFIG_REFERENCE.md` (the **EN canonical**);
3. asserts each field name appears as a **backticked span** `` `<name>` `` in the doc.

A new DTO field with no doc entry → **red CI**. This is the mechanism that stops this reference from
repeating the drift that created the task. (Mirrors the existing `TestGitFlowStatesItsAssumptions` /
`TestExamplesLoadAndValidate` "docs-must-say-X" pattern.)

- **Why `dto.go` only:** `pkg/mtt/config.go` carries **zero** yaml tags (verified — the domain is tag-free;
  the adapter DTO is the sole serialization contract). `dto.go` holds the config DTOs exclusively (task/note
  DTOs live in `task_dto.go`/`note_dto.go`), so its tag set == the config schema.
- **Backticked form, not bare word:** field names include common English words (`name`, `default`, `from`,
  `to`, `post`, `run`, `create`, `note`, …); a bare-substring check would pass trivially. Requiring the
  backticked span makes each match a **deliberate** documented field, matching the doc's own convention
  (every field is authored as `` `field` ``). Residual risk (a field left undocumented while its name is
  incidentally backticked elsewhere) is low and acceptable for a tripwire; the reviewer may tighten to a
  table-cell/heading anchor if desired.
- **EN-only assertion:** the test guards the canonical EN doc; RU parity stays a review concern (as with the
  other bilingual pairs — the test does not read `.ru.md`).
- **Scope of the guard — names, not semantics:** the test asserts the field *enumeration* stays complete
  (no silently-added DTO field goes undocumented). It does **not** assert the D2 *validity rules* are
  present — that semantic completeness is guarded by review + the FLOW_GUIDE §11 cross-ref. Stated so the
  test's green is not mistaken for "authorable-from-docs" on its own.

### D4 — Sync surface

- New: `CONFIG_REFERENCE.md`, `CONFIG_REFERENCE.ru.md`.
- `CLAUDE.md:31–32` **and** `AGENTS.md:119–122` — the bilingual-pairs list is enumerated in **both**
  agent rulebooks (identical four pairs); add `` `CONFIG_REFERENCE.md` ↔ `CONFIG_REFERENCE.ru.md` `` to
  **both**, or the two authoritative rulebooks disagree on what needs bilingual sync. **(rev1 MAJOR.)**
- `README.md` + `README.ru.md` Docs list (README.md:144–146) — add a `CONFIG_REFERENCE` line.
- `FLOW_GUIDE.md` + `.ru.md` — a pointer in §1 (Mental model) and §11 (Validate) to the full field
  reference.
- `CLI_REFERENCE.md` + `.ru.md` — a pointer near the `mtt init --template` config-as-code note.
- `CHANGELOG.md` — one line under `[Unreleased] → ### Added`.
- `internal/adapter/yaml/templates_test.go` **not** touched; the new test is its own file
  (e.g. `config_reference_test.go`).

## 4. Test plan (red → green)

1. Add `config_reference_test.go` with `TestConfigReferenceCoversSchema` (RED — `CONFIG_REFERENCE.md`
   absent → read error / all fields missing).
2. Write `CONFIG_REFERENCE.md` covering all 33 fields → GREEN.
3. Write the RU mirror + the pointer/CLAUDE/README/CHANGELOG edits. `make check` green.

## 5. Files touched

New: `CONFIG_REFERENCE.md`, `CONFIG_REFERENCE.ru.md`, `internal/adapter/yaml/config_reference_test.go`,
`docs/superpowers/specs/t83-config-schema-reference.md` (this), later the plan.
Edited: `CLAUDE.md`, `AGENTS.md`, `README.md`, `README.ru.md`, `FLOW_GUIDE.md`, `FLOW_GUIDE.ru.md`,
`CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md`, `CHANGELOG.md`.

## 6. Risks & mitigations

- **Reference drifts from DTO** (the original sin) → D3's test makes it a red CI, not a silent gap.
- **Common-word false pass in the test** → backticked-span requirement + doc convention; residual risk
  acknowledged, tightenable in review.
- **`dto.go` gains a non-config struct** → the test would then demand documenting its fields. Low risk
  (`dto.go` is config-DTO-exclusive by the package's own CLAUDE.md); if it happens, scope the grep to the
  config struct names. Flag for the reviewer.
- **New bilingual pair = permanent upkeep** (against c23's slim-docs direction) → justified: it is a
  *reference* (stable, additive), the test bounds its completeness, and the alternative (agents reading
  source) is worse. RU parity is review-guarded, EN is the tested canonical.
- **CHANGELOG gate:** the new test lives under `internal/` → the impl-submit "code changed" check fires and
  requires a CHANGELOG entry (D4 includes one).

## 7. Out of scope

A machine-readable JSON-Schema / `mtt config validate` command; documenting task/note file schemas (a
separate reference); auto-generating the doc from the DTOs (the test enforces coverage without generation);
the `mtt config` command surface (t82).
