# t45 — JSON output consistency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `mtt types` / `version` / `init` / `rm`(single) / `use`(clear+no-current) honor `--json`, so the root `Long`'s "All commands support --json" is true, with `types --json` carrying the full flow graph.

**Architecture:** Add dedicated `*JSON` view structs + pure mappers (the repo's `taskJSON`/`toTaskJSON` pattern), then each command's `RunE` branches on `jsonFlag(cmd)` and emits its view via `writeJSON` instead of the human text. No `core`/adapter changes — the flow-graph mapper reads `mtt.Config` (+ `settings.Prefixes` for the CLI-only prefix).

**Tech Stack:** Go (cobra CLI), `encoding/json` via the existing `writeJSON`, testscript (txtar) e2e + Go unit tests.

## Global Constraints

- Spec (authority): `docs/superpowers/specs/t45-json-consistency.md` — decisions D1–D7 are binding.
- TDD: red → green → refactor. `make check` green before every commit.
- JSON conventions: **structural arrays** (`parents`, `statuses`, `transitions`, `commands`, `post`) are **non-null** (build with `make`, never leave nil); **scalar optionals** + `current` + `rollback` are `omitempty`; **`require` is a pointer** `*requireJSON` (Go ignores `omitempty` on a non-pointer struct — a value field would always serialize `{"who":false,"why":false}`).
- Timeouts are `time.Duration.String()` strings (`"10m0s"`, omitted when 0).
- `prefix` is **not** a `mtt.Type` field — pass it from `settings.Prefixes` (`toTypeJSON(t, prefix)`).
- `rm`-single captures the task **before** `Remove` (which returns only an error); emit only when `--json`.
- `use --json` emits the current task **or `null`**; `init --json` `path` is **absolutized** (`filepath.Abs`).
- Acceptance is **behavioral** (each command emits valid JSON under `--json`); cobra `completion`/`help` are carved out.
- Docs: EN is source of truth, `CLI_REFERENCE.ru.md` kept in sync. Non-`.mtt` changes committed by hand on `task/t45`.

---

### Task 1: `types --json` — flow-graph views, mappers, wiring

**Files:**
- Create: `internal/cli/types_json.go` (view structs + mappers + `toTypesJSON`)
- Create: `internal/cli/types_json_test.go` (unit tests for the mappers)
- Modify: `internal/cli/types.go` (RunE: branch on `jsonFlag`)
- Create: `internal/cli/testdata/scripts/types_json.txt` (e2e)

**Interfaces:**
- Produces: `typeJSON`, `statusJSON`, `transitionJSON`, `commandJSON`, `rollbackJSON`, `requireJSON`; `toTypeJSON(t mtt.Type, prefix string) typeJSON`; `toTypesJSON(cfg mtt.Config, prefixes map[string]string, filter string) ([]typeJSON, error)`.
- Consumes: `writeJSON`, `jsonFlag` (existing).

- [ ] **Step 1: Write the failing mapper unit test.** Create `internal/cli/types_json_test.go`:

```go
package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestToTypeJSON(t *testing.T) {
	cfg := mtt.Type{
		Name:        "task",
		Description: "a unit of work",
		Parents:     nil,
		Default:     true,
		Flow: mtt.Flow{
			Statuses: []mtt.Status{
				{Name: "tbd", Kind: mtt.KindInitial, Default: true, Description: "queued"},
				{Name: "done", Kind: mtt.KindTerminal},
			},
			Transitions: []mtt.Transition{
				{
					Name: "start", From: "tbd", To: "done", Description: "go",
					Current: mtt.CurrentSet,
					Require: mtt.Require{Who: true},
					Commands: []mtt.Command{
						{Run: "make check", Timeout: 10 * time.Minute,
							Rollback: &mtt.Command{Run: "git reset"}},
					},
					Post: []mtt.Command{{Run: "git push"}},
				},
			},
		},
	}
	v := toTypeJSON(cfg, "t")
	if v.Name != "task" || v.Prefix != "t" || !v.Default {
		t.Fatalf("head: %+v", v)
	}
	if v.Parents == nil || v.Statuses == nil || v.Transitions == nil {
		t.Fatalf("structural arrays must be non-nil: %+v", v)
	}
	if v.Statuses[0].Default != true || v.Statuses[0].Description != "queued" {
		t.Fatalf("status default/description dropped: %+v", v.Statuses[0])
	}
	tr := v.Transitions[0]
	if tr.Name != "start" || tr.From != "tbd" || tr.To != "done" || tr.Current != "set" {
		t.Fatalf("transition head: %+v", tr)
	}
	if tr.Require == nil || tr.Require.Who != true || tr.Require.Why != false {
		t.Fatalf("require must be a non-nil pointer with who=true: %+v", tr.Require)
	}
	if tr.Commands == nil || tr.Post == nil {
		t.Fatalf("commands/post must be non-nil: %+v", tr)
	}
	c := tr.Commands[0]
	if c.Run != "make check" || c.Timeout != "10m0s" || c.Rollback == nil || c.Rollback.Run != "git reset" {
		t.Fatalf("command: %+v", c)
	}
	if c.Rollback.Timeout != "" {
		t.Fatalf("zero rollback timeout must omit: %q", c.Rollback.Timeout)
	}
}

func TestToTypeJSONZeroValuesOmit(t *testing.T) {
	// a bare transition: no require, no current, no command timeout/rollback
	cfg := mtt.Type{Name: "x", Flow: mtt.Flow{
		Statuses:    []mtt.Status{{Name: "a", Kind: mtt.KindInitial}},
		Transitions: []mtt.Transition{{From: "a", To: "a", Commands: []mtt.Command{{Run: "true"}}}},
	}}
	blob := mustMarshal(t, []typeJSON{toTypeJSON(cfg, "x")})
	for _, absent := range []string{`"require"`, `"current"`, `"rollback"`, `"timeout"`, `"default"`, `"description"`} {
		if strings.Contains(blob, absent) {
			t.Fatalf("expected %s omitted, got:\n%s", absent, blob)
		}
	}
	for _, present := range []string{`"parents": []`, `"post": []`, `"commands": [`} {
		if !strings.Contains(blob, present) {
			t.Fatalf("expected %s present (non-null), got:\n%s", present, blob)
		}
	}
}
```

- [ ] **Step 2: Add the `mustMarshal` helper to the test file** (indented JSON string, matching `writeJSON`):

```go
func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
```

Add `"encoding/json"` to the test's imports.

- [ ] **Step 3: Run the tests, verify they fail to compile.**

Run: `go test ./internal/cli/ -run TestToTypeJSON`
Expected: FAIL — `undefined: toTypeJSON` / `typeJSON`.

- [ ] **Step 4: Create `internal/cli/types_json.go`** with the views and mappers:

```go
package cli

import (
	"fmt"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// typeJSON is the machine-readable view of a configured type and its full flow
// graph (statuses + transitions incl. gates/post/rollback/require/current).
type typeJSON struct {
	Name        string           `json:"name"`
	Prefix      string           `json:"prefix"`
	Parents     []string         `json:"parents"`
	Default     bool             `json:"default,omitempty"`
	Description string           `json:"description,omitempty"`
	Statuses    []statusJSON     `json:"statuses"`
	Transitions []transitionJSON `json:"transitions"`
}

type statusJSON struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Default     bool   `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}

type transitionJSON struct {
	Name        string        `json:"name,omitempty"`
	From        string        `json:"from"`
	To          string        `json:"to"`
	Description string        `json:"description,omitempty"`
	Current     string        `json:"current,omitempty"`
	Require     *requireJSON  `json:"require,omitempty"` // pointer: omitempty is ignored on a struct value
	Commands    []commandJSON `json:"commands"`
	Post        []commandJSON `json:"post"`
}

type commandJSON struct {
	Run      string        `json:"run"`
	Timeout  string        `json:"timeout,omitempty"`
	Rollback *rollbackJSON `json:"rollback,omitempty"`
}

type rollbackJSON struct {
	Run     string `json:"run"`
	Timeout string `json:"timeout,omitempty"`
}

type requireJSON struct {
	Who bool `json:"who,omitempty"`
	Why bool `json:"why,omitempty"`
}

// toTypesJSON maps the configured types (optionally filtered to one) to their
// JSON views. Mirrors formatTypes: an unknown filter is an error.
func toTypesJSON(cfg mtt.Config, prefixes map[string]string, filter string) ([]typeJSON, error) {
	out := make([]typeJSON, 0, len(cfg.Types))
	for _, t := range cfg.Types {
		if filter != "" && string(t.Name) != filter {
			continue
		}
		out = append(out, toTypeJSON(t, prefixes[string(t.Name)]))
	}
	if filter != "" && len(out) == 0 {
		return nil, fmt.Errorf("unknown type %q", filter)
	}
	return out, nil
}

// toTypeJSON maps one type (prefix comes from settings.Prefixes, not the domain type).
func toTypeJSON(t mtt.Type, prefix string) typeJSON {
	parents := make([]string, len(t.Parents))
	for i, p := range t.Parents {
		parents[i] = string(p)
	}
	statuses := make([]statusJSON, len(t.Statuses))
	for i, s := range t.Statuses {
		statuses[i] = statusJSON{Name: string(s.Name), Kind: string(s.Kind), Default: s.Default, Description: s.Description}
	}
	transitions := make([]transitionJSON, len(t.Transitions))
	for i, tr := range t.Transitions {
		transitions[i] = toTransitionJSON(tr)
	}
	return typeJSON{
		Name: string(t.Name), Prefix: prefix, Parents: parents,
		Default: t.Default, Description: t.Description,
		Statuses: statuses, Transitions: transitions,
	}
}

func toTransitionJSON(tr mtt.Transition) transitionJSON {
	commands := make([]commandJSON, len(tr.Commands))
	for i, c := range tr.Commands {
		commands[i] = toCommandJSON(c)
	}
	post := make([]commandJSON, len(tr.Post))
	for i, c := range tr.Post {
		post[i] = toCommandJSON(c)
	}
	var req *requireJSON
	if tr.Require.Who || tr.Require.Why {
		req = &requireJSON{Who: tr.Require.Who, Why: tr.Require.Why}
	}
	return transitionJSON{
		Name: tr.Name, From: string(tr.From), To: string(tr.To),
		Description: tr.Description, Current: string(tr.Current),
		Require: req, Commands: commands, Post: post,
	}
}

func toCommandJSON(c mtt.Command) commandJSON {
	cj := commandJSON{Run: c.Run}
	if c.Timeout > 0 {
		cj.Timeout = c.Timeout.String()
	}
	if c.Rollback != nil {
		rb := &rollbackJSON{Run: c.Rollback.Run}
		if c.Rollback.Timeout > 0 {
			rb.Timeout = c.Rollback.Timeout.String()
		}
		cj.Rollback = rb
	}
	return cj
}
```

- [ ] **Step 5: Run the unit tests, verify green.**

Run: `go test ./internal/cli/ -run TestToTypeJSON -v`
Expected: PASS (both cases).

- [ ] **Step 6: Wire `types.go`.** In `internal/cli/types.go`, in the `RunE` after `cfg.Validate()` and computing `filter`, replace the human render block:

```go
			filter := ""
			if len(args) == 1 {
				filter = args[0]
			}
			out, err := formatTypes(cfg, settings.Prefixes, filter)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprint(cmd.OutOrStdout(), out); err != nil {
				return err
			}
			return nil
```

with:

```go
			filter := ""
			if len(args) == 1 {
				filter = args[0]
			}
			if jsonFlag(cmd) {
				views, err := toTypesJSON(cfg, settings.Prefixes, filter)
				if err != nil {
					return err
				}
				return writeJSON(cmd.OutOrStdout(), views)
			}
			out, err := formatTypes(cfg, settings.Prefixes, filter)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprint(cmd.OutOrStdout(), out); err != nil {
				return err
			}
			return nil
```

- [ ] **Step 7: Create the e2e `internal/cli/testdata/scripts/types_json.txt`:**

```
# types --json: emits the flow graph; --json wiring + structure + filter + error
mkdir proj
cd proj
exec mtt init
stdout 'initialized'

# the graph carries statuses (with kind) and transitions (from/to)
exec mtt types --json
stdout '"statuses"'
stdout '"kind"'
stdout '"transitions"'
stdout '"from"'
stdout '"to"'
stdout '"commands"'
stdout '"post"'

# filtering to one type yields a one-element array (the other type's name absent)
exec mtt types task --json
stdout '"name": "task"'
! stdout '"name": "epic"'

# an unknown type errors (exit 1), same as the human path
! exec mtt types nope --json
stderr 'unknown type "nope"'
```

- [ ] **Step 8: Run the script + gate.**

Run: `go test ./internal/cli/ -run 'TestScript/types_json' && make check`
Expected: script PASS; `OK: make check passed`. (If the `default` template lacks a `task`/`epic` type, adjust the filter/`! stdout` names in Step 7 to two types the template actually defines — check `internal/adapter/yaml` templates.)

- [ ] **Step 9: Commit.**

```bash
git add internal/cli/types_json.go internal/cli/types_json_test.go internal/cli/types.go internal/cli/testdata/scripts/types_json.txt
git commit -m "t45: types --json — full flow graph (statuses/transitions/gates/post/require/current)"
```

---

### Task 2: `version --json` + `init --json`

**Files:**
- Modify: `internal/cli/json.go` (add `versionJSON`, `initJSON`)
- Modify: `internal/cli/version.go` (RunE: branch on `jsonFlag`)
- Modify: `internal/cli/init.go` (RunE: branch on `jsonFlag`, abs path)
- Modify: `internal/cli/version_test.go` (unit: `version --json`)
- Modify: `internal/cli/testdata/scripts/init.txt` (e2e: `init --json`)

**Interfaces:**
- Produces: `versionJSON{Version string}`, `initJSON{Path, Template, Name string; Created bool}`.
- Consumes: `resolveVersion` (t30), `writeJSON`, `jsonFlag`.

- [ ] **Step 1: Write the failing `version --json` unit test.** Append to `internal/cli/version_test.go`:

```go
func TestVersionJSON(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"version", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if got.Version != resolveVersion() {
		t.Fatalf("version = %q, want %q", got.Version, resolveVersion())
	}
}
```

Add `"bytes"` and `"encoding/json"` to `version_test.go`'s imports (it currently imports only `"testing"`).

- [ ] **Step 2: Run it, verify it fails.**

Run: `go test ./internal/cli/ -run TestVersionJSON`
Expected: FAIL — output is the bare version line, `json.Unmarshal` errors (or `undefined: versionJSON` once referenced).

- [ ] **Step 3: Add the view structs to `internal/cli/json.go`** (after `taskJSON`/before `writeJSON`, anywhere top-level):

```go
// versionJSON is `mtt version --json`.
type versionJSON struct {
	Version string `json:"version"`
}

// initJSON is `mtt init --json`: the created-config summary (absolute path).
type initJSON struct {
	Path     string `json:"path"`
	Template string `json:"template"`
	Name     string `json:"name"`
	Created  bool   `json:"created"`
}
```

- [ ] **Step 4: Wire `version.go`.** Replace the `RunE` body:

```go
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), resolveVersion())
			return err
		},
```

with:

```go
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jsonFlag(cmd) {
				return writeJSON(cmd.OutOrStdout(), versionJSON{Version: resolveVersion()})
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), resolveVersion())
			return err
		},
```

- [ ] **Step 5: Wire `init.go`.** Replace the success-print block:

```go
			if err := yaml.Init(base, tmpl, projectName, force); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "initialized .mtt/config.yaml (template %q)\n", tmpl); err != nil {
				return err
			}
			return nil
```

with:

```go
			if err := yaml.Init(base, tmpl, projectName, force); err != nil {
				return err
			}
			if jsonFlag(cmd) {
				absBase, err := filepath.Abs(base)
				if err != nil {
					return err
				}
				return writeJSON(cmd.OutOrStdout(), initJSON{
					Path:     filepath.Join(absBase, ".mtt", "config.yaml"),
					Template: tmpl, Name: projectName, Created: true,
				})
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "initialized .mtt/config.yaml (template %q)\n", tmpl); err != nil {
				return err
			}
			return nil
```

(`init.go` already imports `path/filepath`.)

- [ ] **Step 6: Run unit test, verify green.**

Run: `go test ./internal/cli/ -run TestVersionJSON -v`
Expected: PASS.

- [ ] **Step 7: Add `init --json` e2e.** Append to `internal/cli/testdata/scripts/init.txt`:

```
# init --json: the created-config summary (absolute path, template, name)
mkdir jsonproj
cd jsonproj
exec mtt init --json
stdout '"path"'
stdout '"config.yaml"'
stdout '"template": "default"'
stdout '"name": "jsonproj"'
stdout '"created": true'
```

- [ ] **Step 8: Run scripts + gate.**

Run: `go test ./internal/cli/ -run 'TestScript/init' && make check`
Expected: PASS; `OK: make check passed`.

- [ ] **Step 9: Commit.**

```bash
git add internal/cli/json.go internal/cli/version.go internal/cli/init.go internal/cli/version_test.go internal/cli/testdata/scripts/init.txt
git commit -m "t45: version --json ({version}) + init --json (summary, abs path)"
```

---

### Task 3: `rm`(single) + `use`(clear / no-current) `--json`

**Files:**
- Modify: `internal/cli/rm.go` (`runRmSingle`: capture-before-delete, emit under `--json`; add `errors` import)
- Modify: `internal/cli/use.go` (clear + no-current branches: emit `null` under `--json`)
- Modify: `internal/cli/testdata/scripts/rm.txt` (e2e)
- Modify: `internal/cli/testdata/scripts/current_task.txt` (e2e)

**Interfaces:**
- Consumes: `toTaskJSON`, `writeJSON`, `jsonFlag`, `taskNotFound` (existing).

- [ ] **Step 1: Extend `rm.txt` (red for single `--json`).** Find the single-delete section of `internal/cli/testdata/scripts/rm.txt` and add, after a task is created (adapt the id to the script's existing setup — create a fresh task if needed):

```
# rm <id> --json emits the removed task object (mirrors add --json)
exec mtt add 'to delete'
stdout 'created (t\d+)'
exec mtt rm t1 --json
stdout '"id": "t1"'
stdout '"status"'
! stdout 'removed t1'
```

(Use whatever id `mtt add` prints in this script's context; if `t1` is taken, create and target a fresh one.)

- [ ] **Step 2: Run it, verify it fails.**

Run: `go test ./internal/cli/ -run 'TestScript/rm'`
Expected: FAIL — `rm --json` still prints `removed t1`, the `"id"` assertion fails.

- [ ] **Step 3: Rewrite `runRmSingle`** in `internal/cli/rm.go`:

```go
func runRmSingle(cmd *cobra.Command, root, idArg string, force, dryRun bool) error {
	id, err := mtt.NewTaskID(idArg)
	if err != nil {
		return err
	}
	if dryRun {
		return previewBulk(cmd, []mtt.TaskID{id})
	}
	store := yaml.NewTaskStore(root)
	_, settings, err := yaml.Load(root)
	if err != nil {
		return err
	}
	_, by, why, err := resolveAttribution(cmd, settings.Author)
	if err != nil {
		return err
	}
	// Remove returns only an error, so capture the task before deleting when --json.
	var removed mtt.Task
	if jsonFlag(cmd) {
		removed, err = store.Get(id)
		if err != nil {
			if errors.Is(err, mtt.ErrNotFound) {
				return taskNotFound(id)
			}
			return err
		}
	}
	remover := core.NewRemover(store, yaml.NewAuditStore(root), time.Now)
	if err := remover.Remove(id, force, by, why); err != nil {
		return err
	}
	if err := clearCurrentIfMatches(root, id); err != nil {
		return err
	}
	if jsonFlag(cmd) {
		return writeJSON(cmd.OutOrStdout(), toTaskJSON(removed))
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", id)
	return err
}
```

Add `"errors"` to `rm.go`'s imports.

- [ ] **Step 4: Run rm tests, verify green.**

Run: `go test ./internal/cli/ -run 'TestScript/rm' -v`
Expected: PASS (single `--json` object; the existing `removed <id>` non-json and bulk cases unchanged).

- [ ] **Step 5: Extend `current_task.txt` (red for `use --json`).** Append to `internal/cli/testdata/scripts/current_task.txt`:

```
# use --json: the current task or null (c.f. t45)
exec mtt add 'work'
stdout 'created (t\d+)'
# no current yet -> null
exec mtt use --clear
exec mtt use --json
stdout '^null'
# set a current -> the task object
exec mtt use t1 --json
stdout '"id": "t1"'
# clear --json -> null
exec mtt use --clear --json
stdout '^null'
```

(Adapt `t1` to the id created here.)

- [ ] **Step 6: Run it, verify it fails.**

Run: `go test ./internal/cli/ -run 'TestScript/current_task'`
Expected: FAIL — `use --json` (no current) prints `no current task`, and `use --clear --json` prints `current cleared`, not `null`.

- [ ] **Step 7: Wire `use.go`.** In the `clearFlag` branch, replace:

```go
				if err := current.ClearCurrent(); err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "current cleared")
				return err
```

with:

```go
				if err := current.ClearCurrent(); err != nil {
					return err
				}
				if jsonFlag(cmd) {
					return writeJSON(cmd.OutOrStdout(), nil)
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "current cleared")
				return err
```

In the `default` branch's no-current case, replace:

```go
				if !ok {
					_, err = fmt.Fprintln(cmd.OutOrStdout(), "no current task")
					return err
				}
```

with:

```go
				if !ok {
					if jsonFlag(cmd) {
						return writeJSON(cmd.OutOrStdout(), nil)
					}
					_, err = fmt.Fprintln(cmd.OutOrStdout(), "no current task")
					return err
				}
```

(The set and with-current branches already emit `toTaskJSON`; `writeJSON(w, nil)` marshals to `null`.)

- [ ] **Step 8: Run use tests + gate.**

Run: `go test ./internal/cli/ -run 'TestScript/current_task' && make check`
Expected: PASS; `OK: make check passed`.

- [ ] **Step 9: Commit.**

```bash
git add internal/cli/rm.go internal/cli/use.go internal/cli/testdata/scripts/rm.txt internal/cli/testdata/scripts/current_task.txt
git commit -m "t45: rm-single --json (removed task) + use --json (current task or null)"
```

---

### Task 4: Docs — `CLI_REFERENCE` (EN+RU) + verify the `Long` claim

**Files:**
- Modify: `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md` (document the new `--json` shapes)
- Modify: `internal/cli/CLAUDE.md` (note the JSON surfaces)

- [ ] **Step 1: Document the shapes in `CLI_REFERENCE.md`.** In each command's section (`types`, `version`, `init`, `rm`, `use`), add a `--json` sentence. For `types`: "`--json` emits the flow graph — an array of `{name, prefix, parents, default?, description?, statuses:[{name,kind,default?,description?}], transitions:[{name?, from, to, description?, current?, require?, commands:[{run, timeout?, rollback?}], post:[…]}]}` (structural arrays non-null; `types <type> --json` → one-element array; unknown type → exit 1)." For `version`: "`--json` → `{\"version\": …}`." For `init`: "`--json` → `{path, template, name, created}` (absolute path)." For `rm`: "`rm <id> --json` → the removed task object; bulk `--json` → the per-item outcome array." For `use`: "`--json` → the current task object, or `null` when there is none (incl. `--clear`)."

- [ ] **Step 2: Mirror the same sentences into `CLI_REFERENCE.ru.md`** (RU, kept in sync — translate the prose, keep the JSON literals identical).

- [ ] **Step 3: Note it in `internal/cli/CLAUDE.md`.** Add one line to the JSON-surfaces paragraph: "JSON consistency (t45): `types`/`version`/`init`/`rm`-single/`use` now honor `--json` (`types_json.go` holds the flow-graph views; `require` is a `*requireJSON` so `omitempty` works). Every mtt command emits JSON under `--json`; cobra `completion`/`help` excepted."

- [ ] **Step 4: Verify the `Long` claim is now behaviorally true.** For each mtt command, confirm `--json` produces JSON:

Run:
```bash
cd $(mktemp -d) && mtt init >/dev/null
for c in "types" "version" "init --force" "roadmap" "ready" "list" "tree" "tags" "use"; do
  printf '%-12s ' "$c"; mtt $c --json 2>&1 | head -c 40; echo
done
```
Expected: each line emits JSON (`[`, `{`, or `null`) — not prose. (`completion`/`help` are cobra built-ins, out of scope.)

- [ ] **Step 5: Gate + commit.**

Run: `make check`
Expected: `OK: make check passed`.

```bash
git add CLI_REFERENCE.md CLI_REFERENCE.ru.md internal/cli/CLAUDE.md
git commit -m "t45: docs — --json shapes for types/version/init/rm/use (EN+RU)"
```

---

## Final acceptance (after all tasks)

- [ ] **AC-1:** `mtt types --json` emits the graph with `statuses` (incl. `kind`; `default`/`description` where set) and `transitions` (incl. `name`, `current`, `require`, per-transition `commands` with `timeout`+`rollback` where configured, and `post`); `mtt types <type> --json` is a one-element array; `mtt types <unknown> --json` exits 1; an invalid config errors as the human path.
- [ ] **AC-2:** `mtt version --json` → `{"version": …}`; `mtt init --json` → summary (absolute `path`); `mtt rm <id> --json` → the removed task object; `mtt rm <a> <b> --json` → the per-item array (unchanged); `mtt use --json` → the current task or `null` (incl. `--clear` and no-current).
- [ ] **AC-3:** Every mtt command emits valid JSON under `--json` (Step 4 of Task 4; `completion`/`help` excepted).
- [ ] **AC-4:** `CLI_REFERENCE.md` ↔ `.ru.md` document the shapes; `internal/cli/CLAUDE.md` current.
- [ ] **AC-5:** `make check` green.

## Self-review notes

- **Spec coverage:** D1 → Tasks 1-3 (5 surfaces). D2 → Task 1 (schema incl. require/current/status default+description; prefix from Prefixes; timeout string; require pointer). D3/D4 → Task 2. D5 → Task 3 (capture-before-delete; single vs bulk). D6 → Task 3 (use null). D7 → Task 4 Step 4 (behavioral verify; carve-out). Testing → unit (Task 1) + e2e (Tasks 1-3).
- **Type consistency:** `toTypeJSON(t, prefix)`, `toTypesJSON(cfg, prefixes, filter)`, `toTransitionJSON`, `toCommandJSON`, `versionJSON`, `initJSON`, `requireJSON` (pointer) — names identical across tasks. `writeJSON(w, nil)` → `null` (verified: `json.go` `MarshalIndent`).
- **No new version literal / no core change.** `rm`-single adds one `store.Get` only under `--json`.
