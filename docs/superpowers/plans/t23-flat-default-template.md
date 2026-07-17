# t23 — Flat, frictionless default template — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `mtt init`'s default template flat and frictionless (`task`+`chore`, both root) so `mtt add "x"` works out of the box, while preserving today's epic/task/subtask config as an opt-in `hierarchy` template.

**Architecture:** Config-only change. Three shipped templates (`default`, `coding`, `hierarchy`) are `text/template` YAML files embedded in `internal/adapter/yaml`. No engine/domain change — `core.Adder`, `Type.IsRoot`, `Config.Validate` are untouched. Tests are Go golden/validity tests in the adapter + `testscript` (txtar) e2e in the CLI.

**Tech Stack:** Go 1.23+, `text/template`, `embed`, `testscript` (txtar), `make check` gate (gofmt + vet + golangci-lint v2 + `go test -race -cover` + build).

## Global Constraints

- Spec of record: `docs/superpowers/specs/t23-flat-default-template.md`.
- **No engine/domain change.** Only templates, template plumbing, tests, and docs.
- **`make check` green before every commit** (TDD: red → green → refactor).
- **Storage only through the port**; templates are the only home of default type/status names.
- Deterministic YAML: field order = struct order; goldens are regenerated with `-update`, never hand-edited.
- **Docs are bilingual** (EN primary + RU mirror), update in lockstep: `README.md`↔`README.ru.md`, `DESIGN.md`↔`DESIGN.ru.md`, `CLI_REFERENCE.md`↔`CLI_REFERENCE.ru.md`.
- Commit trailer: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.
- Run a single testscript: `go test ./internal/cli -run 'TestScripts/<name>' -v` (name = file basename without `.txt`).
- Regenerate goldens: `go test ./internal/adapter/yaml -run TestRenderGolden -update`.

---

## Task 1: Ship the `hierarchy` template (additive; default unchanged)

Relocate today's default (epic/task/subtask) into a new `hierarchy` template, wire the plumbing, add a golden, add a per-template validity guard, and advertise the template in `mtt init` help. This task changes **no** default behavior, so every existing test stays green.

**Files:**
- Create: `internal/adapter/yaml/templates/hierarchy.yaml`
- Modify: `internal/adapter/yaml/templates.go:10` (embed directive) and `:14-17` (`templateFiles` map)
- Modify: `internal/adapter/yaml/init_test.go` (extend `TestRenderGolden`; add `TestTemplatesValidate`)
- Create (via `-update`): `internal/adapter/yaml/testdata/golden/hierarchy.yaml`
- Modify: `internal/cli/init.go:51` (help string)

**Interfaces:**
- Consumes: `renderTemplate(name, projectName string) ([]byte, error)`; `Init(root, tmplName, projectName string, force bool) error`; `Load(root string) (mtt.Config, Settings, error)`; `mtt.Config.Validate() error`.
- Produces: a third registered template name `"hierarchy"`, resolvable by `renderTemplate`/`Init`.

- [ ] **Step 1: Create the `hierarchy` template as a verbatim copy of today's default.**

Create `internal/adapter/yaml/templates/hierarchy.yaml` with EXACTLY the current content of `templates/default.yaml`:

```yaml
version: 1
project:
  name: {{.Name}}
command_timeout: 5m
types:
  - name: epic
    description: A large body of work spanning multiple tasks.
    prefix: e
    parents: []
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, current: set}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, description: "all epic tasks closed", current: clear}
      - {from: in_progress, to: cancelled}
  - name: task
    description: A unit of work under an epic.
    prefix: t
    parents: [epic]
    default: true
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, description: "review the spec, create a branch", current: set}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, description: "quality gate", current: clear}
      - {from: in_progress, to: cancelled}
  - name: subtask
    description: A small step within a task.
    prefix: s
    parents: [task]
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, current: set}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, current: clear}
      - {from: in_progress, to: cancelled}
```

Verify byte-identity to the current default: `diff internal/adapter/yaml/templates/default.yaml internal/adapter/yaml/templates/hierarchy.yaml` → expect **no output**.

- [ ] **Step 2: Wire the plumbing (embed directive + map).**

In `internal/adapter/yaml/templates.go`, update the embed directive (line 10):

```go
//go:embed templates/default.yaml templates/coding.yaml templates/hierarchy.yaml
var templatesFS embed.FS
```

and the `templateFiles` map (lines 14-17):

```go
var templateFiles = map[string]string{
	"default":   "templates/default.yaml",
	"coding":    "templates/coding.yaml",
	"hierarchy": "templates/hierarchy.yaml",
}
```

- [ ] **Step 3: Extend `TestRenderGolden` to cover `hierarchy` (write the failing expectation).**

In `internal/adapter/yaml/init_test.go`, change the render loop list:

```go
for _, name := range []string{"default", "coding", "hierarchy"} {
```

- [ ] **Step 4: Run the render test to see it fail (golden missing).**

Run: `go test ./internal/adapter/yaml -run TestRenderGolden -v`
Expected: FAIL — `read golden .../testdata/golden/hierarchy.yaml (run -update first)`.

- [ ] **Step 5: Generate the `hierarchy` golden.**

Run: `go test ./internal/adapter/yaml -run TestRenderGolden -update`
This creates `testdata/golden/hierarchy.yaml` (rendered with name `demo`) and leaves `default.yaml`/`coding.yaml` byte-unchanged. Verify: `diff internal/adapter/yaml/testdata/golden/default.yaml internal/adapter/yaml/testdata/golden/hierarchy.yaml` → **no output**.

- [ ] **Step 6: Run the render test to see it pass.**

Run: `go test ./internal/adapter/yaml -run TestRenderGolden -v`
Expected: PASS.

- [ ] **Step 7: Add `TestTemplatesValidate` (guard: every shipped template Loads + Validates).**

Append to `internal/adapter/yaml/init_test.go`:

```go
// TestTemplatesValidate guards that every shipped starter template renders into a
// structurally valid config — Load (provider checks) then Config.Validate. Load does
// not call Validate, so without this a broken template could ship unnoticed.
func TestTemplatesValidate(t *testing.T) {
	for _, name := range []string{"default", "coding", "hierarchy"} {
		t.Run(name, func(t *testing.T) {
			tmp := t.TempDir()
			if err := Init(tmp, name, "demo", false); err != nil {
				t.Fatalf("init %s: %v", name, err)
			}
			cfg, _, err := Load(tmp)
			if err != nil {
				t.Fatalf("load %s: %v", name, err)
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("validate %s: %v", name, err)
			}
		})
	}
}
```

- [ ] **Step 8: Run the validity test.**

Run: `go test ./internal/adapter/yaml -run TestTemplatesValidate -v`
Expected: PASS (all three subtests) — today's `default`/`coding` are valid, and `hierarchy` is a copy of the valid `default`.

- [ ] **Step 9: Advertise `hierarchy` in `mtt init` help.**

In `internal/cli/init.go:51`, change:

```go
	cmd.Flags().StringVar(&tmpl, "template", "default", "starter template: default|coding|hierarchy")
```

- [ ] **Step 10: Run the adapter + cli packages.**

Run: `go test ./internal/adapter/yaml ./internal/cli`
Expected: PASS (default behavior unchanged; hierarchy is additive).

- [ ] **Step 11: Commit.**

```bash
git add internal/adapter/yaml/templates/hierarchy.yaml internal/adapter/yaml/templates.go \
        internal/adapter/yaml/init_test.go internal/adapter/yaml/testdata/golden/hierarchy.yaml \
        internal/cli/init.go
git commit -m "t23: add hierarchy template (relocate today's default) + validity guard"
```

---

## Task 2: Flatten the `default` template + migrate e2e

Replace `default.yaml` with a flat `task`+`chore` config, prove the ergonomics regression is fixed, regenerate the default golden, and migrate every testscript that depended on the old hierarchical default. Flattening necessarily reddens those scripts, so their migration lands in the **same** task to keep the suite green.

**Files:**
- Modify: `internal/adapter/yaml/templates/default.yaml` (flatten)
- Modify (via `-update`): `internal/adapter/yaml/testdata/golden/default.yaml`
- Rewrite: `internal/cli/testdata/scripts/init.txt`
- Modify: `internal/cli/testdata/scripts/add_json.txt`
- Modify (one-line init switch): `add_depends_on.txt`, `add_show.txt`, `batch.txt`, `dep.txt`, `list_edit.txt`, `ready.txt`, `rm.txt`, `roadmap.txt`, `tags.txt`, `tree.txt` (all under `internal/cli/testdata/scripts/`)

**Interfaces:**
- Consumes: the flat `default` template; `mtt init`, `mtt add`, `mtt types` CLI.
- Produces: default `mtt init` yields root types `task`(prefix `t`, default) + `chore`(prefix `c`); `mtt add "x"` → `t1` with no `--parent`.

- [ ] **Step 1: Rewrite `init.txt` to assert the flat default + the ergonomics fix (the failing test).**

Replace the whole of `internal/cli/testdata/scripts/init.txt` with:

```
# init creates config; types lists the configured (flat) types
mkdir proj
cd proj
exec mtt init
stdout 'initialized'
exists .mtt/config.yaml
exec mtt types
stdout 'task'
stdout 'chore'
stdout 'active'
stdout '-> done'
stdout 'default'
! stdout 'epic'
! stdout 'subtask'

# the default type (task) is a root: the first add just works, no --parent
exec mtt add 'first task'
stdout 'created t1'
exec mtt add 'a chore' --type chore
stdout 'created c1'

# a single type can be filtered
exec mtt types task
stdout 'prefix t'
! stdout 'chore'

# re-init without --force fails
! exec mtt init
stderr 'already initialized'

# --force overwrites; coding template shows gated per-type DoD
exec mtt init --force --template coding
exec mtt types
stdout 'feature'
stdout 'bugfix'
stdout 'refactor'
stdout 'make test'

# --force + hierarchy template restores the epic > task > subtask model
exec mtt init --force --template hierarchy
exec mtt types
stdout 'epic'
stdout 'task'
stdout 'subtask'

# types outside a project errors — and the error points the way out (U4).
cd $WORK/empty
! exec mtt types
stderr 'not initialized'
stderr 'mtt init'

# init --json: the created-config summary (absolute path, template, name)
mkdir jsonproj
cd jsonproj
exec mtt init --json
stdout '"path"'
stdout 'config\.yaml"'
stdout '"template": "default"'
stdout '"name": "jsonproj"'
stdout '"created": true'

-- empty/.keep --
```

- [ ] **Step 2: Run `init.txt` to see it fail (default still hierarchical).**

Run: `go test ./internal/cli -run 'TestScripts/init' -v`
Expected: FAIL — `mtt types` still prints `epic`/`subtask`, and `mtt add 'first task'` errors with `type "task" requires a parent`.

- [ ] **Step 3: Flatten `default.yaml`.**

Replace the whole of `internal/adapter/yaml/templates/default.yaml` with:

```yaml
version: 1
project:
  name: {{.Name}}
command_timeout: 5m
types:
  - name: task
    description: A unit of work.
    prefix: t
    parents: []
    default: true
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, current: set}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, current: clear}
      - {from: in_progress, to: cancelled}
  - name: chore
    description: A small maintenance change.
    prefix: c
    parents: []
    statuses:
      - {name: tbd,         kind: initial}
      - {name: in_progress, kind: active}
      - {name: done,        kind: terminal}
      - {name: cancelled,   kind: terminal}
    transitions:
      - {from: tbd,         to: in_progress, current: set}
      - {from: tbd,         to: cancelled}
      - {from: in_progress, to: done, current: clear}
      - {from: in_progress, to: cancelled}
```

- [ ] **Step 4: Run `init.txt` to see it pass.**

Run: `go test ./internal/cli -run 'TestScripts/init' -v`
Expected: PASS — `mtt types` shows `task`/`chore`, `mtt add 'first task'` → `created t1`.

- [ ] **Step 5: Regenerate the default golden.**

Run: `go test ./internal/adapter/yaml -run TestRenderGolden -update`
This rewrites `testdata/golden/default.yaml` (now flat) and leaves `coding.yaml`/`hierarchy.yaml` unchanged.

- [ ] **Step 6: Confirm the validity guard still holds.**

Run: `go test ./internal/adapter/yaml -run 'TestRenderGolden|TestTemplatesValidate' -v`
Expected: PASS — the flat `default` Loads + Validates (one default type `task`; unique prefixes `t`/`c`; each flow has initial/active/terminal with valid in/out degrees).

- [ ] **Step 7: Migrate the 10 hierarchy-dependent scripts (one-line init switch).**

In EACH of these files, append `--template hierarchy` to the **initial bootstrap** `mtt init` line (usually `exec mtt init` → `exec mtt init --template hierarchy`; if it already carries flags like `--name X`, append to them). Leave everything else untouched — they build epic/task/subtask via `--type`/`--parent` and keep prefixes e/t/s, and `--template hierarchy` renders byte-identically to the config they saw before:

```
internal/cli/testdata/scripts/add_depends_on.txt
internal/cli/testdata/scripts/add_show.txt
internal/cli/testdata/scripts/batch.txt
internal/cli/testdata/scripts/dep.txt
internal/cli/testdata/scripts/list_edit.txt
internal/cli/testdata/scripts/ready.txt
internal/cli/testdata/scripts/rm.txt
internal/cli/testdata/scripts/roadmap.txt
internal/cli/testdata/scripts/tags.txt
internal/cli/testdata/scripts/tree.txt
```

Note for `add_show.txt`: its `! exec mtt add 'bare task'` / `stderr 'requires a parent'` block stays valid under `--template hierarchy` (the hierarchy `task` still requires a parent) — do NOT delete it.

Caution: some scripts contain `exec mtt init --force …` or `cp … .mtt/config.yaml` lines elsewhere. Change ONLY the initial `exec mtt init` bootstrap line; leave any `--force`/`cp` lines as-is. Grep each file for `mtt init` first: `grep -n 'mtt init' internal/cli/testdata/scripts/<file>`.

- [ ] **Step 8: Fix `add_json.txt` (comment + redundant `--no-parent`).**

In `internal/cli/testdata/scripts/add_json.txt`, update the block (lines ~6-14). The default `task` is now root, so `--no-parent` is a no-op:

```
# The default type (task) is a root, so the first add just works.
exec mtt --json add 'first task'
stdout '"id": "t1"'
stdout '"status": "tbd"'
! stdout 'created t1'

# Without --json, the plain ack is unchanged.
exec mtt add 'second task'
stdout 'created t2'
```

- [ ] **Step 9: Run the full cli + adapter suites.**

Run: `go test ./internal/cli ./internal/adapter/yaml`
Expected: PASS. If any migrated script fails, `grep -n 'mtt init\|--type\|--parent\|epic\|subtask' <script>` and confirm the init line is the only default-dependent line; fix per the failure.

- [ ] **Step 10: Full gate.**

Run: `make check`
Expected: green (fmt + vet + lint + `go test -race -cover` + build). `TestRepoDogfoodConfig` stays green (the repo's own hand-authored `task`+`chore` config is template-independent).

- [ ] **Step 11: Commit.**

```bash
git add internal/adapter/yaml/templates/default.yaml internal/adapter/yaml/testdata/golden/default.yaml \
        internal/cli/testdata/scripts/
git commit -m "t23: flatten the default template (task+chore, root) + migrate e2e to --template hierarchy"
```

---

## Task 3: Docs sync (EN + RU + adapter CLAUDE.md)

Reconcile every documentation occurrence that presents epic/task/subtask as the default, in lockstep across languages. No engine code changes; `make check` stays green (build only touches the already-updated `init.go` from Task 1).

**Files:**
- Modify: `README.md`, `README.ru.md`
- Modify: `DESIGN.md`, `DESIGN.ru.md`
- Modify: `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md`
- Modify: `internal/adapter/yaml/CLAUDE.md`

Line numbers below are anchors from the spec — re-grep before editing (`grep -nE 'epic|subtask|template|default config' <file>`).

- [ ] **Step 1: README feature bullet (`:46`) — EN + RU.**

`README.md:46`: change `Epic → task → subtask is just the default config;` →
`Epic → task → subtask is just the `hierarchy` template (`mtt init --template hierarchy`);`

`README.ru.md:46`: change `Эпик → задача → подзадача — лишь дефолтный конфиг;` →
`Эпик → задача → подзадача — лишь шаблон `hierarchy` (`mtt init --template hierarchy`);`

- [ ] **Step 2: README Quickstart (`:111-117`) — EN + RU, real rewrite.**

Replace the `README.md` Quickstart code block with the flat default flow (+ a hierarchy pointer):

```sh
mtt init                    # create .mtt/ (default template: flat task + chore)
mtt add "Ship auth"         # -> t1 (a root task; no --parent needed)
mtt add "Login endpoint"    # -> t2
mtt status t1 in_progress   # move through the flow (a transition's commands run + gate here)
mtt done t1                 # -> the done terminal (default flow has no gate commands)
mtt list                    # all tasks
mtt tree                    # the task tree
mtt roadmap                 # dependency + priority execution order
# For an epic > task > subtask hierarchy: mtt init --template hierarchy
```

Mirror in `README.ru.md` with the same commands and translated comments (e.g. `# создать .mtt/ (шаблон default: плоские task + chore)`, `# -> t1 (корневая задача; --parent не нужен)`, … `# Для иерархии epic > task > subtask: mtt init --template hierarchy`).

- [ ] **Step 3: DESIGN hierarchy-source sentence (`:226-227`) — EN + RU.**

`DESIGN.md:226-227`: change
`The epic → task → subtask hierarchy is **not hardcoded** — it follows from the default config:` →
`The epic → task → subtask hierarchy is **not hardcoded** — it follows from the `hierarchy` template (the flat `default` template ships `task`+`chore`):`
(keep the following `epic (root) ← task ← subtask` line).

Mirror the same reword in `DESIGN.ru.md` (parallel line ~228).

- [ ] **Step 4: DESIGN init-writes paragraph + example block (`:614` and `:618-655`) — EN + RU.**

`DESIGN.md:614`: change
`` `mtt init` writes `.mtt/config.yaml` with example types `epic`/`task`/`subtask` and a linear flow `` →
`` `mtt init` writes `.mtt/config.yaml` from the `default` template — flat root types `task`+`chore` with a linear flow ``
… and add one sentence after the paragraph: `The epic/task/subtask example below is the `hierarchy` template (`mtt init --template hierarchy`).` Leave the illustrative YAML block (`:618-655`) as the hierarchy example (it is now correctly labeled).

Mirror in `DESIGN.ru.md`.

- [ ] **Step 5: CLI_REFERENCE `init` intro + `--template` (`:146`, `:153`) — EN + RU.**

`CLI_REFERENCE.md:146`: change `(types `epic`/`task`/`subtask`, flow ...)` → `(types `task`/`chore`, flow `tbd → in_progress → done` plus the terminal `cancelled`, no commands)`.

`CLI_REFERENCE.md:153`: change
`` - `--template <name>` — starter config: `default` (epic/task/subtask, no commands) or `coding` `` →
`` - `--template <name>` — starter config: `default` (flat `task`+`chore`, no commands), `coding` `` … and extend the sentence to add `, or `hierarchy` (epic/task/subtask, no commands)`. Keep `Default: `default`.`

Mirror in `CLI_REFERENCE.ru.md`.

- [ ] **Step 6: CLI_REFERENCE `mtt tree` example (`:303`) — EN + RU.**

`CLI_REFERENCE.md:303`: change `Prints the epic → task → subtask tree as an ASCII tree` → `Prints the task hierarchy (parent → child) as an ASCII tree`. Mirror in `CLI_REFERENCE.ru.md`.

- [ ] **Step 7: Confirm the no-change occurrence.**

`CLI_REFERENCE.md:430` (+ru) — *"The default/`coding` templates `set` on → `in_progress`"* — stays TRUE (flat `task`+`chore` keep `current: set/clear`). Read it, confirm no edit needed. Do NOT change.

- [ ] **Step 8: Adapter `CLAUDE.md` template list.**

In `internal/adapter/yaml/CLAUDE.md`, find the `Init` responsibility line (`render an embedded template (`default`/`coding`, …)`) and change the enumerated set to `default`/`coding`/`hierarchy`.

- [ ] **Step 9: Grep sweep — no stale "default = epic/subtask" claim remains.**

Run: `grep -rniE 'default (config|template).*(epic|subtask)|epic.*task.*subtask.*default' README.md README.ru.md DESIGN.md DESIGN.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md`
Expected: no hit that still calls epic/task/subtask the *default*. (Capability/example mentions of epic/task/subtask are fine.)

- [ ] **Step 10: Gate + commit.**

Run: `make check`
Expected: green.

```bash
git add README.md README.ru.md DESIGN.md DESIGN.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md \
        internal/adapter/yaml/CLAUDE.md
git commit -m "t23: docs — reconcile default template (flat task+chore) across EN+RU + adapter CLAUDE.md"
```

---

## Final verification (before `mtt submit` → impl_review)

- [ ] `make check` green from a clean tree.
- [ ] Manual smoke: in a temp dir, `mtt init && mtt add "hello"` → `created t1` (no `--parent`); `mtt init --force --template hierarchy && mtt add --type epic E && mtt add --type task --parent e1 T` works.
- [ ] Principles self-check (AGENTS.md): clean architecture (config-only, engine untouched), DRY (no duplicated config across tests — hierarchy fixture is one shipped template), KISS (flat starter), TDD (red ergonomics test written before the flatten).
- [ ] Docs-sync judgment: EN+RU parallel occurrences all updated; DESIGN/README/CLI_REFERENCE consistent with the shipped templates.
