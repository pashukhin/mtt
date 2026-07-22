# Placeholders in shown descriptions (t16) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Shown flow descriptions/guidance expand the same `{{.ID}}`/`{{.Type}}`/`{{.From}}`/`{{.To}}` placeholders as `commands`, best-effort (raw on error), in human **and** `--json` output — so guidance names the concrete task (`task/t17`, not `task/<id>`).

**Architecture:** A lenient `core.ExpandText` (reuses the private `cmdContext`/`expandTemplate`) returns raw on any template error. The CLI's guidance helpers (`moveGuidance`/`formatNextMoves`/`statusGuidance`) and `toShowJSON` call it; edge descriptions use the edge's `from`/`to`, status (node) descriptions use `From=To=status`. A few repo `.mtt/config.yaml` descriptions switch to `{{.ID}}` as a live dogfood demo.

**Tech Stack:** Go 1.23+, `text/template` (via existing `expandTemplate`), cobra, `testscript` e2e.

## Global Constraints

- **Spec of record:** `docs/superpowers/specs/t16-description-placeholders.md` (D1–D3, AC1–6).
- **TDD:** unit-first for `ExpandText`, e2e-first for the wiring (red on literal `{{.ID}}` → green). `make check` green before every commit.
- **Best-effort:** guidance NEVER errors on a bad description template — `ExpandText` returns raw. (Gate commands stay strict via `expandCommands`.)
- **Whitelist:** only `{ID,Type,From,To}` (reuse `cmdContext`); `{{.Title}}` → raw (best-effort).
- **Node rule:** a status (node) description expands with `From = To = the status's own name`.
- **`--json` must be fully expanded** (`status_description` + `next[].description`) — the primary agent surface.
- **`TestRepoDogfoodConfig` stays green** after the D3 config edits.

---

## File structure

**Modify:**
- `internal/core/expand.go` — add `ExpandText`.
- `internal/core/expand_test.go` — `TestExpandText`.
- `internal/cli/guidance.go` — expand in `moveGuidance` (+`id` param), `formatNextMoves` (+`id,typeName`), `statusGuidance`; add `core` import.
- `internal/cli/status.go:126` — pass `task.ID` to `moveGuidance`.
- `internal/cli/show.go:87` — pass `t.ID`/`t.Type` to `formatNextMoves`.
- `internal/cli/json.go` — `toShowJSON` expands `next[].description`; add `core` import.
- `internal/cli/guidance_test.go` — update the `formatNextMoves` callers (new arity).
- `internal/cli/testdata/scripts/desc_placeholders.txt` (new) — e2e.
- `.mtt/config.yaml` — 4 descriptions `<id>`/`<this-task-id>` → `{{.ID}}`.
- Docs: `DESIGN.md`↔`.ru.md`, `CLI_REFERENCE.md`↔`.ru.md`, `CHANGELOG.md`, `internal/core/CLAUDE.md`, `internal/cli/CLAUDE.md`.

---

## Task 1: `core.ExpandText` (unit red→green)

**Files:** `internal/core/expand.go`, `internal/core/expand_test.go`

**Interfaces:**
- Produces: `func ExpandText(raw, id, typ, from, to string) string` — expands the 4 placeholders, raw on error.

- [ ] **Step 1: Write the failing test** — `internal/core/expand_test.go` (append; the file may not exist — create with `package core`):

```go
package core

import "testing"

func TestExpandText(t *testing.T) {
	cases := []struct{ raw, id, typ, from, to, want string }{
		{"task/{{.ID}}", "t17", "task", "tbd", "in_progress", "task/t17"},
		{"{{.From}}→{{.To}} ({{.Type}})", "t1", "task", "tbd", "done", "tbd→done (task)"},
		{"no placeholders", "t1", "task", "a", "b", "no placeholders"},
		{"", "t1", "task", "a", "b", ""},
		{"{{.Title}}", "t1", "task", "a", "b", "{{.Title}}"}, // unknown field -> raw (best-effort)
		{"{{.ID", "t1", "task", "a", "b", "{{.ID"},          // malformed -> raw
	}
	for _, c := range cases {
		if got := ExpandText(c.raw, c.id, c.typ, c.from, c.to); got != c.want {
			t.Fatalf("ExpandText(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/core/ -run TestExpandText -count=1`
Expected: FAIL — `ExpandText` undefined.

- [ ] **Step 3: Implement** — append to `internal/core/expand.go`:

```go
// ExpandText expands {{.ID}}/{{.Type}}/{{.From}}/{{.To}} in raw for SHOWN guidance
// (descriptions), returning the raw text UNCHANGED on any parse/execute error —
// guidance is informational and must never break a command. (Gate commands use the
// strict expandCommands, which aborts on error; this is the lenient sibling.)
func ExpandText(raw, id, typ, from, to string) string {
	out, err := expandTemplate(raw, cmdContext{ID: id, Type: typ, From: from, To: to})
	if err != nil {
		return raw
	}
	return out
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/core/ -run TestExpandText -race -count=1`
Expected: PASS.

- [ ] **Step 5: `make check` + commit**

Run: `make check`
```bash
git add internal/core/expand.go internal/core/expand_test.go
git commit -m "t16: core.ExpandText — lenient (raw-on-error) placeholder expander for shown guidance"
```

---

## Task 2: wire expansion into guidance + show (human + --json), e2e red→green

**Files:** `internal/cli/guidance.go`, `internal/cli/status.go`, `internal/cli/show.go`, `internal/cli/json.go`, `internal/cli/guidance_test.go`, `internal/cli/testdata/scripts/desc_placeholders.txt`

- [ ] **Step 1: Write the failing e2e** — `internal/cli/testdata/scripts/desc_placeholders.txt`:

```
# t16 — shown descriptions/guidance expand {{.ID}}/{{.From}}/{{.To}}/{{.Type}} (best-effort).
exec mtt init
cp flow.yaml .mtt/config.yaml
exec mtt add 'a task'
stdout 'created t1'

# On-move guidance (text): the edge description and the destination-status description expand.
# (the target status is named `active`, so the sugar is `mtt active t1`.)
exec mtt active t1
stdout 'branch task/t1'
stdout 'you are at active'
# best-effort: a {{.Title}} in the onward description renders raw, and the move still succeeds.
stdout '\{\{.Title\}\}'

# show (text): current-status description (node rule: {{.To}} => the status name `active`, {{.ID}} => t1).
exec mtt show t1
stdout 'you are at active \(id t1\)'

# show --json: BOTH status_description AND next[].description expanded (no raw {{.ID}}).
exec mtt show t1 --json
stdout '"status_description": *"[^"]*active'
! stdout '\{\{\.ID\}\}'

-- flow.yaml --
version: 1
project: {name: descph}
types:
  - name: task
    prefix: t
    default: true
    statuses:
      - {name: tbd, kind: initial, default: true}
      - {name: active, kind: active, description: "you are at {{.To}} (id {{.ID}})"}
      - {name: done, kind: terminal}
    transitions:
      - {from: tbd, to: active, description: "create branch task/{{.ID}}"}
      - {from: active, to: done, description: "close (from {{.From}}) — {{.Title}}"}
```

**CRITICAL note (why the onward desc has NO `{{.ID}}`):** `ExpandText` returns the **whole** string raw on any
template error — it is not per-placeholder. The onward `active→done` desc deliberately contains a `{{.Title}}`
(unknown field) to exercise best-effort, so the **entire** string renders raw. If it also contained `{{.ID}}`,
that `{{.ID}}` would leak raw into `next[].description` and break the `! stdout '\{\{\.ID\}\}'` guard on
`show --json`. So the `{{.Title}}` demo description carries **no** `{{.ID}}` — only the always-expandable edge
desc (`task/{{.ID}}`) and status desc (`id {{.ID}}`) carry `{{.ID}}`.

NOTE on the assertions: after `mtt active t1`, `moveGuidance` prints the edge desc `create branch task/t1`
and the destination `active` status desc `you are at active (id t1)` (node rule From=To=active). The onward
`active→done` description renders **raw** as `close (from {{.From}}) — {{.Title}}` (best-effort: `{{.Title}}`
errors, so the WHOLE string — incl. the `{{.From}}` — stays raw).
`mtt show t1` shows the current `active` status desc `you are at active (id t1)` (node rule `{{.To}}`→`active`)
+ the onward `done` move. `show --json` expands both `status_description` (→ `…active (id t1)`) and the
`next[].description` (raw `close (from {{.From}}) — {{.Title}}`, which carries **no** `{{.ID}}`), so the
`! stdout '\{\{\.ID\}\}'` guard holds.

- [ ] **Step 2: Run to verify it fails (RED)**

Run: `go test ./internal/cli/ -run 'TestScripts/desc_placeholders' -count=1`
Expected: FAIL — today the descriptions print verbatim, so `stdout 'branch task/t1'` fails (it shows `task/{{.ID}}`).

- [ ] **Step 3: Wire `guidance.go`** — add `"github.com/pashukhin/mtt/internal/core"` to the imports, then:

`moveGuidance` (gains `id mtt.TaskID`):
```go
func moveGuidance(cfg mtt.Config, id mtt.TaskID, typeName mtt.TypeName, from, to mtt.StatusName) string {
	typ, ok := cfg.TypeByName(typeName)
	if !ok {
		return ""
	}
	var b strings.Builder
	if edge, ok := typ.FindTransition(from, to); ok && edge.Description != "" {
		fmt.Fprintf(&b, "  ▸ %s\n", core.ExpandText(edge.Description, string(id), string(typeName), string(from), string(to)))
	}
	if st, ok := typ.StatusByName(to); ok && st.Description != "" {
		// node rule: a status description sees From=To=its own name (the destination).
		fmt.Fprintf(&b, "  ▸ %s\n", core.ExpandText(st.Description, string(id), string(typeName), string(to), string(to)))
	}
	if onward := typ.TransitionsFrom(to); len(onward) > 0 {
		fmt.Fprintf(&b, "  next: %s\n", formatNextMoves(onward, id, typeName))
	}
	return b.String()
}
```

`formatNextMoves` (gains `id mtt.TaskID, typeName mtt.TypeName`):
```go
func formatNextMoves(onward []mtt.Transition, id mtt.TaskID, typeName mtt.TypeName) string {
	parts := make([]string, 0, len(onward))
	for _, e := range onward {
		s := string(e.To)
		if e.Name != "" {
			s = e.Name + " → " + string(e.To)
		}
		if e.Description != "" {
			s += " (" + core.ExpandText(e.Description, string(id), string(typeName), string(e.From), string(e.To)) + ")"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " · ")
}
```

`statusGuidance` (expand the node description; From=To=current status):
```go
func statusGuidance(cfg mtt.Config, t mtt.Task) (statusDesc string, onward []mtt.Transition) {
	typ, ok := cfg.TypeByName(t.Type)
	if !ok {
		return "", nil
	}
	if st, ok := typ.StatusByName(t.Status); ok {
		statusDesc = core.ExpandText(st.Description, string(t.ID), string(t.Type), string(t.Status), string(t.Status))
	}
	return statusDesc, typ.TransitionsFrom(t.Status)
}
```

- [ ] **Step 4: Update the callers**

`internal/cli/status.go:126` — pass `task.ID`:
```go
		if g := moveGuidance(cfg, task.ID, task.Type, last.From, last.To); g != "" {
```
`internal/cli/show.go:87` (inside `formatTask`, which has `t`) — pass `t.ID`, `t.Type`:
```go
		fmt.Fprintf(&b, "  next:     %s\n", formatNextMoves(onward, t.ID, t.Type))
```

- [ ] **Step 5: Expand `--json` onward descriptions** — `internal/cli/json.go` `toShowJSON` (add the `core`
import), the `for _, e := range onward` loop:
```go
	for _, e := range onward {
		sj.Next = append(sj.Next, nextMoveJSON{
			Name: e.Name, To: string(e.To),
			Description: core.ExpandText(e.Description, string(t.ID), string(t.Type), string(e.From), string(e.To)),
		})
	}
```
(`status_description` is already the expanded value from `statusGuidance` — no change there.)

- [ ] **Step 6: Fix the `guidance_test.go` callers (new arity)** — `internal/cli/guidance_test.go:10` and `:18`:
```go
	got := formatNextMoves([]mtt.Transition{ /* …unchanged… */ }, "t1", "task")
	...
	if got := formatNextMoves(nil, "t1", "task"); got != "" {
```

- [ ] **Step 7: Run e2e + unit to verify GREEN**

Run: `go test ./internal/cli/ -run 'TestScripts/desc_placeholders|TestFormatNextMoves' -race -count=1`
Expected: PASS.

- [ ] **Step 8: No-regression (existing guidance/show/status tests) + `make check`**

Run: `go test ./internal/cli/ -run 'TestScripts|TestShow|TestStatus|TestFormatNextMoves' -race -count=1`
Run: `make check`
Expected: green.

- [ ] **Step 9: Commit**

```bash
git add internal/cli/guidance.go internal/cli/status.go internal/cli/show.go internal/cli/json.go \
        internal/cli/guidance_test.go internal/cli/testdata/scripts/desc_placeholders.txt
git commit -m "t16: expand placeholders in shown descriptions/guidance (human + --json), best-effort"
```

---

## Task 3: dogfood — `{{.ID}}` in repo config descriptions

**Files:** `.mtt/config.yaml`

- [ ] **Step 1: Switch the 4 placeholder-bearing descriptions** to `{{.ID}}`:
  - `.mtt/config.yaml:14` (task `speccing` status): `docs/superpowers/specs/<this-task-id>-<slug>.md` → `docs/superpowers/specs/{{.ID}}-<slug>.md`
  - `.mtt/config.yaml:18` (task `planning` status): `docs/superpowers/plans/<this-task-id>-<slug>.md` → `docs/superpowers/plans/{{.ID}}-<slug>.md`
  - `.mtt/config.yaml:34` (task `start` edge): `task/<id>` → `task/{{.ID}}`
  - `.mtt/config.yaml:227` (chore `start` edge): `task/<id>` → `task/{{.ID}}`

  (Leave `<slug>` literal — there is no slug placeholder. Do NOT touch `post:` commands — they already use
  `{{.ID}}`. These are the ONLY descriptions carrying `<id>`/`<this-task-id>`.)

- [ ] **Step 2: Verify dogfood config still valid + green**

Run: `go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig -count=1`
Expected: PASS (the test asserts descriptions via non-empty + `strings.Contains` on stable substrings like
`pull main first` / `superpowers:brainstorming`, which the edits preserve — never literal equality).

Run: `make check`
Expected: green.

- [ ] **Step 3: Commit**

```bash
git add .mtt/config.yaml
git commit -m "t16: dogfood — repo flow descriptions use {{.ID}} (guidance now names the concrete task)"
```

---

## Task 4: docs sync

**Files:** `DESIGN.md`↔`.ru.md`, `CLI_REFERENCE.md`↔`.ru.md`, `CHANGELOG.md`, `internal/core/CLAUDE.md`, `internal/cli/CLAUDE.md`

- [ ] **Step 1: `DESIGN.md` + `.ru.md`** — grep the placeholder material: `grep -n 'placeholder\|{{.ID}}\|expandCommands' DESIGN.md DESIGN.ru.md`. Add a clause: the same `{{.ID}}/{{.Type}}/{{.From}}/{{.To}}` placeholders now expand in **shown descriptions/guidance** (best-effort — raw on error; node descriptions see `From=To=status`), not just `commands`. Parallel EN + RU.

- [ ] **Step 2: `CLI_REFERENCE.md` + `.ru.md`** — grep `description`/`guidance`. Note the placeholder expansion (human **and** `--json`; a bad template degrades to raw) and the node rule (`{{.From}}`/`{{.To}}` in a status description both render that status's own name). Parallel EN + RU.

- [ ] **Step 3: `CHANGELOG.md`** — under `[Unreleased] → ### Changed`, append:
```markdown
- **Shown flow descriptions/guidance expand placeholders** — `{{.ID}}`/`{{.Type}}`/`{{.From}}`/`{{.To}}` now
  expand in on-move guidance and `mtt show` (human and `--json`), best-effort (a bad template shows raw), so
  guidance names the concrete task (`task/t17`, not `task/<id>`).
```

- [ ] **Step 4: CLAUDE.md** — `internal/core/CLAUDE.md`: `ExpandText` (the lenient, raw-on-error sibling of `expandCommands`, for shown guidance). `internal/cli/CLAUDE.md`: the guidance helpers expand descriptions (signatures thread `id`/`type`; `--json` via `toShowJSON`; node rule From=To=status). Keep thin.

- [ ] **Step 5: `make check` + commit**

Run: `make check`
```bash
git add DESIGN.md DESIGN.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md CHANGELOG.md \
        internal/core/CLAUDE.md internal/cli/CLAUDE.md
git commit -m "t16: docs — placeholders in shown descriptions (DESIGN EN/RU, CLI_REFERENCE EN/RU, CHANGELOG, CLAUDE)"
```

---

## Acceptance criteria mapping (spec → tasks)

- **AC1** (`ExpandText` unit) → Task 1.
- **AC2** (move guidance expands) + **AC3** (show human + `--json` expand) + **AC4** (best-effort `{{.Title}}`) → Task 2.
- **AC5** (dogfood config valid; live expansion) → Task 3 + the `impl_review` spot-check.
- **AC6** (`make check` + docs) → each commit + Task 4.

## impl_review checklist

- Principles self-check (DRY — reuse `expandTemplate`/`cmdContext`; KISS; hexagon — expansion policy in core, wiring in cli); docs-sync (DESIGN EN+RU, CLI_REFERENCE EN+RU, CHANGELOG, both CLAUDE); `make check` green.
- **AC5 live spot-check:** on this repo, `mtt show <a tbd task>` shows the `start` edge's `task/{{.ID}}` expanded to the real id (e.g. `task/t16`); `mtt show --json` carries no raw `{{.ID}}`.
