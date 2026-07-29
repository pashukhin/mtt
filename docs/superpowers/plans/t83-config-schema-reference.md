# t83 — Config schema reference — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship an authoritative, exhaustive `.mtt/config.yaml` field reference (`CONFIG_REFERENCE.md` + `.ru`) plus a test that fails if any config DTO field goes undocumented — so a config is authorable from docs, not source.

**Architecture:** Docs-first. One new drift-guard test (the only Go), then the EN reference (drives the test green), then the RU mirror, then the sync surface (both agent rulebooks + README/FLOW_GUIDE/CLI_REFERENCE pointers + CHANGELOG). No Go logic changes. Spec: [t83-config-schema-reference.md](../specs/t83-config-schema-reference.md).

**Tech Stack:** Go test (`regexp`, `FindRoot`), Markdown (bilingual EN+RU).

## Global Constraints

- **No Go logic change:** nothing under `cmd/`/`pkg/`/`internal/**` logic changes; only a new test file. The test lives in `package yaml` (internal — like `dogfood_test.go`), calls `FindRoot(".")` directly, reads `dto.go` from CWD and `CONFIG_REFERENCE.md` via the repo root.
- **The 33 config fields** (distinct `yaml:"…"` tags in `internal/adapter/yaml/dto.go`, verified): `version, project, command_timeout, author, extract_hashtags, require, events, types, name, prefix, parents, default, description, post_defaults, statuses, transitions, kind, from, to, commands, current, post, inherit_post, who, why, task, note, create, update, delete, run, timeout, rollback`.
- **`omitempty` is inert** (DTO is decode-only) — required/optional comes from validation, never from a tag.
- **Bilingual sync:** `CONFIG_REFERENCE.md ↔ .ru.md`; and the new pair is added to **both** `CLAUDE.md:31–32` **and** `AGENTS.md:119–122`.
- **Gate:** `make check` green before the impl `mtt submit`.
- **§-cross-refs** point into FLOW_GUIDE: §1 mental model, §3 graph, §4 gates, §5 post/rollback, §5b events, §6 attribution, §11 validate.

---

### Task 1: The drift-guard test + `CONFIG_REFERENCE.md` (EN) — TDD

**Files:**
- Create: `internal/adapter/yaml/config_reference_test.go`
- Create: `CONFIG_REFERENCE.md`

**Interfaces:**
- Consumes: `FindRoot(string) (string, error)` (root.go:25, same package); `internal/adapter/yaml/dto.go` (read as text).
- Produces: `CONFIG_REFERENCE.md` documenting all 33 fields as backticked spans (optionally colon-suffixed).

- [ ] **Step 1: Write the test (RED)** — `internal/adapter/yaml/config_reference_test.go`:

```go
package yaml

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestConfigReferenceCoversSchema fails if a config field declared in dto.go is
// not documented (as a backticked span `field`, optionally `field:`) in
// CONFIG_REFERENCE.md. The drift guard so the reference can never silently go
// stale-incomplete — the sin that created t83. Guards the field ENUMERATION;
// the validity SEMANTICS are a review concern (see the doc + FLOW_GUIDE §11).
func TestConfigReferenceCoversSchema(t *testing.T) {
	dto, err := os.ReadFile("dto.go") // test CWD is the package dir
	if err != nil {
		t.Fatalf("read dto.go: %v", err)
	}
	root, err := FindRoot(".")
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	ref, err := os.ReadFile(filepath.Join(root, "CONFIG_REFERENCE.md"))
	if err != nil {
		t.Fatalf("read CONFIG_REFERENCE.md: %v", err)
	}
	doc := string(ref)

	tag := regexp.MustCompile(`yaml:"([^",]+)`) // name = token before the first quote/comma
	seen := map[string]bool{}
	for _, m := range tag.FindAllStringSubmatch(string(dto), -1) {
		name := m[1]
		if name == "" || name == "-" || seen[name] {
			continue
		}
		seen[name] = true
		// backtick-bounded so a common word (name/default/from/to/post/run/...) must be a
		// DELIBERATE documented field, and `post` never satisfies `post_defaults`.
		if !regexp.MustCompile("`" + regexp.QuoteMeta(name) + ":?`").MatchString(doc) {
			t.Errorf("config field %q is in dto.go but not documented (as `%s`) in CONFIG_REFERENCE.md", name, name)
		}
	}
	if len(seen) == 0 {
		t.Fatal("no yaml tags found in dto.go — the extractor is broken")
	}
}
```

- [ ] **Step 2: Run the test to verify it FAILS**

Run: `go test ./internal/adapter/yaml/ -run TestConfigReferenceCoversSchema -v`
Expected: FAIL — `read CONFIG_REFERENCE.md: open …/CONFIG_REFERENCE.md: no such file or directory`.

- [ ] **Step 3: Create `CONFIG_REFERENCE.md`** (the full reference — every field is a backticked entry; validity rules restated at field level):

````markdown
# Config reference — `.mtt/config.yaml`

> Русская версия: [CONFIG_REFERENCE.ru.md](CONFIG_REFERENCE.ru.md). English is the source of truth; keep both in sync.

The authoritative, field-by-field schema of a project's flow config. This is a **reference** (look-ups); to
learn **how to author** a flow, read [FLOW_GUIDE.md](FLOW_GUIDE.md) — the section numbers below cross-link to it.

A flow config is **trusted code**: its gate/`post`/event commands run via `sh -c` with your privileges — only
run or commit a `.mtt/config.yaml` you trust, exactly as you would a `Makefile`. The live config is a two-file
overlay: the committed `.mtt/config.yaml` plus an optional gitignored `.mtt/config.local.yaml` (later wins at
top-level-field granularity). **`omitempty` in the DTO is inert** — the config is decode-only, so a field's
required/optional status is a *validation* property (below), never a serialization tag.

## Top-level keys

| Field | Type | Required? | Meaning / validity | See |
|---|---|---|---|---|
| `version` | int | required | schema version; currently `1`. | §11 |
| `project` | map | required | project block; holds `name`. | §1 |
| `project.name` | string | required | the project's display name. | §1 |
| `command_timeout` | duration | optional (default `5m`) | per-command budget; Go `time.ParseDuration` (`5m`, `10m0s`); a bad value fails at load. | §4 |
| `author` | string | optional | acting-subject default; normally set in the gitignored `config.local.yaml`, not the committed file (see Overlay). | §6 |
| `extract_hashtags` | bool | optional (default `false`) | opt-in extraction of `#hashtags` from a task's title/description into tags. | — |
| `require` | map | optional | required-attribution policy `{who, why}` (see Require); **tighten-only**. | §6 |
| `events` | map | optional | lifecycle-event pipelines fired on non-flow mutations (see Events). | §5b |
| `types` | list | required (≥1) | the entity types; each defines its own flow (see Type). | §1, §3 |

## `types[]` — a Type

| Field | Type | Required? | Meaning / validity | See |
|---|---|---|---|---|
| `name` | string | required | type name, unique across types. | §1 |
| `prefix` | string | required | id prefix (the YAML adapter mints `<prefix><N>`); **letters-only** `[a-zA-Z]+`, **unique** across types. | §3 |
| `parents` | list of string | required (may be `[]`) | parent type names; `[]` = a root type; `[epic]` = placeable only under an `epic`. | §3 |
| `default` | bool | optional | mark **exactly one** type `default: true` — used by `mtt add` without `--type`. | §3 |
| `description` | string | optional | human description, shown by `mtt types`. | — |
| `post_defaults` | list of command | optional | finalization commands prepended to every edge's `post:` for this type (opt out per-edge with `inherit_post: false`). | §5 |
| `statuses` | list | required | the states (see Status); needs ≥1 `initial`, ≥1 `active`, ≥1 `terminal`. | §1 |
| `transitions` | list | required | the directed edges (see Transition). | §1 |

## `statuses[]` — a Status

| Field | Type | Required? | Meaning / validity | See |
|---|---|---|---|---|
| `name` | string | required | status name, unique within the type. | §1 |
| `kind` | enum | required | one of `initial` \| `active` \| `terminal`; must **match topology** (`initial` = no incoming, `terminal` = no outgoing, `active` = both). | §3, §11 |
| `description` | string | optional | shown by `mtt show`/`mtt types` (self-documents the next step). | — |
| `default` | bool | optional | **at most one** status may be `default`, and it **must be `initial`** (the entry point when a type has >1 initial). | §3 |

## `transitions[]` — a Transition (edge)

| Field | Type | Required? | Meaning / validity | See |
|---|---|---|---|---|
| `from` | string | required | source status name (must resolve). | §1 |
| `to` | string | required | target status name (must resolve). | §1 |
| `name` | string | optional | edge verb for `mtt <name> <id>` / `mtt do`; **unique per source status** and **disjoint from status names**. | §3 |
| `description` | string | optional | shown at the edge by `mtt show`/`mtt types`. | — |
| `commands` | list of command | optional | **gates** — run in order; all must exit 0 or the move is blocked (exit 3). | §4 |
| `current` | enum | optional | `set` \| `clear` — moves the personal "current task" pointer (empty = no change). | §6 |
| `require` | map | optional | per-edge `{who, why}`; unioned with the global policy (tighten-only). | §6 |
| `post` | list of command | optional | **finalization** commands run *after* the move persists; a failure keeps the move (exit 5). | §5 |
| `inherit_post` | bool | optional | pointer semantics — absent ≠ `false`; only an explicit `inherit_post: false` opts the edge out of the type's `post_defaults`. | §5 |

## A command (`commands` / `post` / `post_defaults` items)

A command is a **scalar string** (a shell command, run via `sh -c`) **or** a map:

| Field | Type | Required? | Meaning / validity | See |
|---|---|---|---|---|
| `run` | string | required (map form) | the shell command. | §4 |
| `timeout` | duration | optional | per-command budget overriding `command_timeout`; Go `time.ParseDuration`. | §4 |
| `rollback` | command | optional | compensator (scalar or map) run in reverse if a **later** gate command fails; rejected in any `post`/event surface. | §5 |

## `events` — lifecycle pipelines (post-only)

`events:` hangs **post-only** command lists on non-flow mutations, per store × kind, fired per entity, only
when something persisted:

| Field | Type | Meaning | See |
|---|---|---|---|
| `task` / `note` | map | the entity store the hooks apply to. | §5b |
| `create` / `update` / `delete` | map | the mutation kind; holds `post`. | §5b |
| `post` | list of command | commands to run after that mutation persists. | §5b |

## `require` — required attribution

| Field | Type | Meaning | See |
|---|---|---|---|
| `who` | bool | require an acting subject before the gate runs. | §6 |
| `why` | bool | require a free-text reason. | §6 |

**Tighten-only:** `config.local` and per-edge `require` may *add* a requirement, never relax a committed one.
Attribution resolves in order: `--who`/`--by` > `MTT_BY` (env) > `author:` in `config.local.yaml`.

## `.mtt/config.local.yaml` — the gitignored overlay

| Field | Type | Meaning |
|---|---|---|
| `author` | string | your durable personal acting-subject default (source of truth for `by`). |
| `current` | string | the personal "current task" id pointer (managed by `current: set`/`clear`; not hand-edited). |

## Placeholders

Only a fixed whitelist expands; any other `{{…}}` (e.g. `{{.Title}}`) is a template error, never interpolated
(a safety boundary — free text rides the environment, not the command string).

- **Edge `commands`/`post`/`post_defaults`:** `{{.ID}}`, `{{.Type}}`, `{{.From}}`, `{{.To}}`.
- **`events.task.*`:** `{{.ID}}`, `{{.Type}}`, `{{.Event}}`.
- **`events.note.*`:** `{{.Slug}}`, `{{.Event}}`.

## Validating your config

`mtt add` and `mtt types` run structural validation and reject a broken flow before runtime (see
[FLOW_GUIDE.md §11](FLOW_GUIDE.md)). *Reference* integrity (dangling `depends_on`, cycles via `mtt check`) is
a separate concern.
````

- [ ] **Step 4: Run the test to verify it PASSES**

Run: `go test ./internal/adapter/yaml/ -run TestConfigReferenceCoversSchema -v`
Expected: PASS (all 33 fields appear as backticked spans; `seen` = 33, non-empty).

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/yaml/config_reference_test.go CONFIG_REFERENCE.md
git commit -m "t83: config schema reference (EN) + drift-guard test"
```

---

### Task 2: `CONFIG_REFERENCE.ru.md` (the RU mirror)

**Files:** Create `CONFIG_REFERENCE.ru.md`.

- [ ] **Step 1: Create the RU mirror** — translate `CONFIG_REFERENCE.md` **row-for-row** (same tables, same field order, same §-cross-refs). Keep **field names, `kind`/`current` enum values, durations, and placeholders in English/verbatim** (they are literal YAML); translate only the prose cells (Meaning/validity) and headings. RU headings: `# Справочник конфига — .mtt/config.yaml`, `## Ключи верхнего уровня`, `## types[] — тип`, `## statuses[] — статус`, `## transitions[] — переход (ребро)`, `## Команда`, `## events — lifecycle-пайплайны`, `## require — обязательная атрибуция`, `## .mtt/config.local.yaml — overlay`, `## Плейсхолдеры`, `## Валидация конфига`. Add the reciprocal top link to `CONFIG_REFERENCE.md`. (The drift test reads only the EN file — RU parity is a review concern.)

- [ ] **Step 2: Commit**

```bash
git add CONFIG_REFERENCE.ru.md
git commit -m "t83: config schema reference (RU mirror)"
```

---

### Task 3: Sync surface — rulebooks, pointers, CHANGELOG

**Files:** `CLAUDE.md`, `AGENTS.md`, `README.md`, `README.ru.md`, `FLOW_GUIDE.md`, `FLOW_GUIDE.ru.md`, `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md`, `CHANGELOG.md`.

- [ ] **Step 1: Both agent rulebooks (the bilingual-pairs enumeration)**

`CLAUDE.md:31–32` — append the new pair to the list, e.g. after `` `FLOW_GUIDE.md` ↔ `FLOW_GUIDE.ru.md` `` add `` , `CONFIG_REFERENCE.md` ↔ `CONFIG_REFERENCE.ru.md` ``.
`AGENTS.md:119–122` — the identical four-pair list; append `` `CONFIG_REFERENCE.md` ↔ `CONFIG_REFERENCE.ru.md` `` there too (verify the exact surrounding text first; it is the "Bilingual docs (English primary + Russian mirror)" line).

- [ ] **Step 2: README Docs list (EN + RU)**

`README.md` — after the `CLI_REFERENCE.md` Docs-list line (README.md:146), add:

```markdown
- [CONFIG_REFERENCE.md](CONFIG_REFERENCE.md) — the full `.mtt/config.yaml` field reference
```

`README.ru.md` — the mirror location (after its `CLI_REFERENCE.ru.md` Docs line), add:

```markdown
- [CONFIG_REFERENCE.ru.md](CONFIG_REFERENCE.ru.md) — полный справочник полей `.mtt/config.yaml`
```

- [ ] **Step 3: FLOW_GUIDE pointers (EN + RU), §1 and §11**

In `FLOW_GUIDE.md` §1 (near the `mtt types` line ~:32) and §11 (structural validation), add a one-line pointer, e.g.: `Every field is catalogued in [CONFIG_REFERENCE.md](CONFIG_REFERENCE.md).` Mirror into `FLOW_GUIDE.ru.md` §1/§11 with `[CONFIG_REFERENCE.ru.md](CONFIG_REFERENCE.ru.md)` and RU prose (`Каждое поле — в справочнике …`).

- [ ] **Step 4: CLI_REFERENCE pointer (EN + RU)**

`CLI_REFERENCE.md` — near the `mtt init --template` "Untrusted config-as-code" note (~:227), add: `The full config schema is in [CONFIG_REFERENCE.md](CONFIG_REFERENCE.md).` Mirror into `CLI_REFERENCE.ru.md` (the «Недоверенный config-as-code» note, ~:226) with the `.ru` target.

- [ ] **Step 5: CHANGELOG**

`CHANGELOG.md` — first bullet under `[Unreleased] → ### Added`:

```markdown
- **Config schema reference (t83).** A new `CONFIG_REFERENCE.md` (+ RU mirror) documents every
  `.mtt/config.yaml` field — type, required/optional, validity rules, semantics, and a FLOW_GUIDE cross-ref —
  so a config is authorable from docs, not source. A drift-guard test fails if a config DTO field goes
  undocumented. Surfaced by the greenfield pilot.
```

- [ ] **Step 6: Full gate + commit**

Run: `make check` — Expected: PASS (the new test is green; docs are Markdown; no Go logic changed).

```bash
git add CLAUDE.md AGENTS.md README.md README.ru.md FLOW_GUIDE.md FLOW_GUIDE.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md CHANGELOG.md
git commit -m "t83: sync surface — rulebook pairs, README/FLOW_GUIDE/CLI_REFERENCE pointers, CHANGELOG"
```

---

## Self-Review

**1. Spec coverage.** D1 (bilingual dedicated doc) → Tasks 1–2. D2 (every field + validity rules + placeholders + overlay) → Task 1 Step 3 (all 33 fields as backticked rows; the validity facts restated per field; edge-vs-event placeholders; overlay `author`/`current`). D3 (drift test, `FindRoot`, backtick, names-not-semantics) → Task 1 Steps 1–4. D4 (sync surface incl. **both** CLAUDE.md and AGENTS.md) → Task 3. All covered.

**2. Placeholder scan.** No TBD/TODO; the EN doc and test are complete verbatim; RU is a row-for-row mirror instruction (the norm for a full bilingual doc); each edit gives literal target text.

**3. Consistency.** The 33 fields in Task 1's doc == the Global-Constraints list == the test's extraction target. The test's backtick regex (`` `name:?` ``) matches the doc's backticked field cells and keeps `post` distinct from `post_defaults` / `timeout` from `command_timeout`. `FindRoot(".")` matches root.go:25 + dogfood_test.go usage; the test is `package yaml` (internal), so `FindRoot` is unqualified.

## Execution note

Task 1 lands first (test + EN doc, green together). The CHANGELOG entry (Task 3 Step 5) is required because a new file under `internal/` (the test) trips the impl-submit "code changed → CHANGELOG" gate.
