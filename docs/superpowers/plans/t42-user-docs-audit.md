# t42 — user-docs audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reconcile the human-facing docs with the actually-shipped surface (kill unshipped-as-present claims, internal contradictions, EN↔RU drift, stale versions) and add a lightweight machine guard so the docs cannot silently drift again.

**Architecture:** A docs-only sweep driven by evidence (claim → proof via the live command registry + running `./bin/mtt` + reading shipped-capability from code). One new Go guard test in `internal/cli` (part of `make check`) provides a genuine red→green for the machine-catchable findings (stale version badge, phantom command); the non-machine findings ride a per-file claim→verify→fix checklist. Every prose fix is applied to both language mirrors (EN primary, RU mirror) in the same commit.

**Tech Stack:** Go 1.23 (`go test`), `make check` gate, Markdown docs. No new dependencies.

## Global Constraints

- **Green gate per commit.** `make check` (gofmt + vet + golangci-lint v2 + `go test -race -cover` + build) must be green before every commit. A red test is never committed — the guard's "red" is *observed in a run step*, then made green in the same commit as its doc fixes.
- **Bilingual sync.** Every human-facing prose change lands in both mirrors in the same commit: `README.md`↔`README.ru.md`, `DESIGN.md`↔`DESIGN.ru.md`, `CLI_REFERENCE.md`↔`CLI_REFERENCE.ru.md`, `FLOW_GUIDE.md`↔`FLOW_GUIDE.ru.md`. English is the source of truth.
- **Agent-doc language rule.** `AGENTS.md` and the `CLAUDE.md` files are English-only and fact-checked on intersection only (never restructured to the bilingual rule).
- **Grep all parallel occurrences.** A changed flow-fact is restated in several places (EN+RU + repeated "Shipped" blocks in DESIGN). Grep every occurrence of a fixed string, not just the first (memory: `design-docs-parallel-occurrences`).
- **Anchor on strings, not line numbers.** Line numbers in the spec/plan drift; locate every edit by its grep-able text.
- **Attribution.** Do not export `MTT_BY`; the canonical author is `.mtt/config.local.yaml` (`author: Grigorii Pashukhin`). Flow moves auto-commit `.mtt`; commit non-`.mtt` files (docs, the test) yourself BEFORE any `mtt submit`/`approve` (clean-tree gate). Commit trailer: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.

---

## File structure

**New:**
- `internal/cli/docs_guard_test.go` — the drift-guard test (3 assertions). Reuses `repoRoot()` (`internal/cli/repo_test.go:13`) and `NewRootCmd()` (`internal/cli/root.go:16`).

**Modified (docs):**
- `README.md` / `README.ru.md` — status line (F1), vision sections → planned/roadmap (F2–F5), `mtt start` → neutral (F6).
- `CHANGELOG.md` — remove phantom `mtt guide` (F8), fix stale template-names (F10), `--json` wording (F7, if in-scope section).
- `CLI_REFERENCE.md` / `CLI_REFERENCE.ru.md` — verify `--json` bullet already accurate (F7, likely no change); optional `mtt comment` inline tag (M4); residual drift.
- `DESIGN.md` / `DESIGN.ru.md` — fix glued YAML (F9, EN only), `mtt start t17` → representative (F6), residual.
- `FLOW_GUIDE.md` / `FLOW_GUIDE.ru.md` — residual drift only.

**Modified (code):**
- `internal/cli/root.go` — long-help "All commands support --json." → precise (F7).
- `internal/cli/CLAUDE.md` — add one line noting the docs-guard test, if the package's doc warrants it.

---

## Task 1: Drift-guard test + machine-catchable fixes (TDD red→green)

**Files:**
- Create: `internal/cli/docs_guard_test.go`
- Modify: `README.md` / `README.ru.md` (status blockquote — F1), `CHANGELOG.md` (remove `mtt guide` — F8)
- Reuse: `internal/cli/repo_test.go:13` (`repoRoot()`), `internal/cli/root.go:16` (`NewRootCmd()`)

**Interfaces:**
- Consumes: `repoRoot() string`, `NewRootCmd() *cobra.Command` (already exported).
- Produces: three tests — `TestDocsCommandsDocumented`, `TestDocsNoPhantomCommands`, `TestDocsNoHardcodedVersion` — and a `registryCommands()` helper local to the test file.

- [ ] **Step 1: Write the guard test**

Create `internal/cli/docs_guard_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docFile reads a repo-root doc by name (repoRoot anchors via runtime.Caller,
// so cwd does not matter), failing the test if absent.
func docFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// registryCommands lists the live top-level command names. Cobra's lazily
// injected help/completion are absent because we never call Execute — so the
// test asserts only over our explicitly registered commands.
func registryCommands() []string {
	var names []string
	for _, c := range NewRootCmd().Commands() {
		names = append(names, c.Name())
	}
	return names
}

// TestDocsCommandsDocumented: every shipped command has a CLI_REFERENCE heading
// whose backtick usage line begins with `mtt <name>` (first whitespace token).
// Group commands (dep/ref/note/agent/tag) have only subcommand headings
// (`mtt dep add …`) — their first token is still the group name. Catches a
// shipped-but-undocumented command.
func TestDocsCommandsDocumented(t *testing.T) {
	ref := docFile(t, "CLI_REFERENCE.md")
	headingRe := regexp.MustCompile("(?m)^#{2,4} `mtt ([^`]+)`")
	documented := map[string]bool{}
	for _, m := range headingRe.FindAllStringSubmatch(ref, -1) {
		if f := strings.Fields(m[1]); len(f) > 0 {
			documented[f[0]] = true // first token = (group) command name; "tag" != "tags"
		}
	}
	for _, name := range registryCommands() {
		if !documented[name] {
			t.Errorf("command %q is not documented in CLI_REFERENCE.md (no `### `mtt %s …`` heading)", name, name)
		}
	}
}

// knownVerbs = registry commands + cobra built-ins (real but lazily injected,
// so absent from NewRootCmd().Commands()) + shipped default-template statuses
// (verb sugar) + an explicit allowlist of documented non-registry verbs
// (parked/planned/edge-sugar). MAINTENANCE POINT, not zero-cost: a new
// parked/planned/edge-sugar verb mentioned in README/CLI_REFERENCE/CHANGELOG
// must be added here (a deliberate forcing function that also blocks phantoms).
// NOTE: the scan covers released CHANGELOG sections too, so a future *rename*
// of a real command would re-trip on historical entries — resolve by adding the
// old name here (an intended, visible signal), never by rewriting history.
func knownVerbs() map[string]bool {
	k := map[string]bool{}
	for _, n := range registryCommands() {
		k[n] = true
	}
	for _, s := range []string{"help", "completion"} { // cobra built-ins: real commands, documented, lazily added
		k[s] = true
	}
	for _, s := range []string{"done", "in_progress", "cancelled"} { // default-template statuses (verb sugar)
		k[s] = true
	}
	for _, s := range []string{ // parked / planned / edge-verb sugar, documented honestly
		"advance", "start", "cancel", "caps", "comment", "search", "gantt", "decline", "reparent",
	} {
		k[s] = true
	}
	return k
}

// TestDocsNoPhantomCommands: every `mtt <verb>` token in the tool-describing
// docs names a known verb. FLOW_GUIDE/.ru are EXCLUDED by design — that guide
// teaches authoring custom flows and legitimately contains arbitrary
// illustrative verbs (`mtt doing`, …) indistinguishable from a phantom by
// shape. This documented cap keeps the test honest. Catches `mtt guide`.
func TestDocsNoPhantomCommands(t *testing.T) {
	known := knownVerbs()
	// Capture the first verb-word after "`mtt " (a backtick, "mtt", a space).
	// Underscore admits in_progress; hyphen admits self-update. A leading
	// [a-z] excludes "<status>"/"--flag" placeholders.
	tokenRe := regexp.MustCompile("`mtt ([a-z][a-z0-9_-]*)")
	for _, f := range []string{"README.md", "README.ru.md", "CLI_REFERENCE.md", "CLI_REFERENCE.ru.md", "CHANGELOG.md"} {
		body := docFile(t, f)
		for _, m := range tokenRe.FindAllStringSubmatch(body, -1) {
			if !known[m[1]] {
				t.Errorf("%s: `mtt %s` is not a known command/status/allowlisted verb (phantom?)", f, m[1])
			}
		}
	}
}

// TestDocsNoHardcodedVersion: the READMEs' status blockquote carries no pinned
// x.y.z (the version is ldflags-injected, so a pinned badge is stale by
// construction). Scoped to `>`-blockquote lines (the only place a badge lives).
func TestDocsNoHardcodedVersion(t *testing.T) {
	semver := regexp.MustCompile(`\d+\.\d+\.\d+`)
	for _, f := range []string{"README.md", "README.ru.md"} {
		for _, line := range strings.Split(docFile(t, f), "\n") {
			if !strings.HasPrefix(line, ">") {
				continue
			}
			if v := semver.FindString(line); v != "" {
				t.Errorf("%s: hardcoded version %q in a status blockquote — use version-agnostic wording", f, v)
			}
		}
	}
}
```

- [ ] **Step 2: Run the guard test — observe RED, confirm exactly the expected findings**

Run: `go test ./internal/cli/ -run 'TestDocs' -v`
Expected FAIL (verified empirically against current docs with this exact `knownVerbs()`):
- `TestDocsNoHardcodedVersion` — `README.md` **and** `README.ru.md` flag `0.8.98` (F1).
- `TestDocsNoPhantomCommands` — **exactly one** token: `CHANGELOG.md: `mtt guide`` (F8). (The regex flags `mtt help`/`mtt completion` from CLI_REFERENCE too, but they are now allowlisted as cobra built-ins → not reported. If they still appear, the allowlist edit above was dropped.)
- `TestDocsCommandsDocumented` — expected **PASS** (all 23 registry commands have a `### `mtt …`` heading; verified in review).

**Empirically confirm the flagged set.** Read every `TestDocsNoPhantomCommands` failure line. The ONLY expected failure is `guide`. If any *other* token is flagged, decide per token: a legitimate command/parked/planned/edge-sugar verb → add it to `knownVerbs()` (with a comment); a real phantom → fix the doc. Record the final flagged set in the commit message.

- [ ] **Step 3: Fix F1 — rewrite the README status blockquote (EN)**

In `README.md`, replace the status blockquote (grep-anchor `> **Status:** working alpha`):

```markdown
> **Status:** working alpha — the flow engine ships (per-type flows + executable, gated status
> transitions with history, structured commands + rollback), plus tasks/hierarchy/dependencies, the
> knowledge base (`mtt note`/`mtt prime`), verifiable references (`mtt ref`/`mtt check`), and agent-config
> scaffolding (`mtt agent`). Run `mtt version` for the build and `mtt --help` for the live surface.
> The `advance`/`start`/`done` meta-walk is **parked** (single-edge `status` is the norm); comment threads,
> text search, `mtt-ui`, and external adapters are later phases. Full plan in [DESIGN.md](DESIGN.md).
```

(Drops the pinned `0.8.98-dev` and the stale "Phases 1–3" enumeration; keeps the honest parked/later-phase caveat. Exact wording may be tightened — the invariants are: no pinned semver, no stale phase list, mtt-ui/search/comments still marked later.)

- [ ] **Step 4: Fix F1 — mirror the status blockquote (RU)**

In `README.ru.md`, replace the parallel blockquote (grep-anchor `> **Статус:** рабочая альфа`) with the Russian mirror of Step 3 — same facts, no pinned semver, no stale phase list, `mtt-ui`/поиск/комментарии marked as поздние фазы.

- [ ] **Step 5: Fix F8 — remove the phantom `mtt guide` from CHANGELOG**

In `CHANGELOG.md` (the `0.9.0` `Project & flow:` bullet), the phantom clause **wraps across two lines** — the Edit `old_string` must include the newline + 2-space continuation indent. Replace exactly:

old_string (spans CHANGELOG.md:173–174):
```
`mtt types` (type + edge map), `mtt guide` (pre-flow orientation: queue navigation,
  first-move setup, mid-flight resumption).
```
new_string:
```
`mtt types` (type + edge map).
```
This keeps the surrounding `mtt init`/`mtt types` items and the historically-true `coding` mention intact.

- [ ] **Step 6: Run the guard test — observe GREEN**

Run: `go test ./internal/cli/ -run 'TestDocs' -v`
Expected: all three PASS.

- [ ] **Step 7: Full gate**

Run: `make check` (write output to a file or check `$?` separately — a piped `| tail` in an `&&` chain masks the exit code).
Expected: green.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/docs_guard_test.go README.md README.ru.md CHANGELOG.md
git commit -m "t42: docs-guard test + F1 badge + F8 phantom fix

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: README vision sections + pitch (F2–F6, EN+RU)

**Files:**
- Modify: `README.md`, `README.ru.md`

**Interfaces:**
- Consumes: the guard from Task 1 stays green (these edits touch no semver/command tokens beyond neutral swaps).
- Produces: honest README body — unshipped features marked planned; `mtt start` removed from prose.

- [ ] **Step 1: F2/F3 — Key ideas: mark unshipped capabilities as planned (EN, + the wrapped F2 RU)**

F2 — the "Optional features" clause **wraps across two lines** in both READMEs; the Edit `old_string` must include the newline + 2-space indent.

In `README.md` (spans :53–54) replace exactly:
```
Optional features (history, dependencies, comment trees,
  search) are per-adapter capabilities.
```
with:
```
Optional per-adapter capabilities: history and dependencies today; comment trees and text search are planned.
```

In `README.ru.md` (spans :53–54) replace exactly:
```
Опциональные возможности (история, зависимости, дерево комментариев, поиск) —
  capability уровня адаптера.
```
with:
```
Опциональные capability уровня адаптера: история и зависимости — сейчас; дерево комментариев и текстовый поиск — планируются.
```

F3 (EN, single line) — grep-anchor the `Verifiable references & append-only history.` bullet (`Tasks/comments carry checkable`) → refs shipped, comments planned:
`**Verifiable references & append-only history.** Tasks carry checkable `refs` to notes/tasks; every status transition is recorded for audit. (Threaded comments are planned.)`
(The F3 RU mirror is handled in Step 5 — its anchor does not wrap.)

- [ ] **Step 2: F4 — Key ideas: mark `mtt-ui` planned (EN)**

Grep-anchor `Optional human UI (`mtt-ui`).` bullet → prefix the feature as planned, consistent with the status caveat:
`**Optional human UI (`mtt-ui`, planned).** A small local web server (task management, Gantt, KB browser) — only needed on the YAML default; with an external backend, humans use its native UI.`

- [ ] **Step 3: F5 — "For agents" / "For humans": planned markers (EN)**

Grep-anchor the `## For agents` paragraph → distinguish shipped from planned:
`Task and dependency management from the command line and knowledge storage (`mtt note`) today; threaded comments, text search, and an optional external indexer hook are planned.`

Grep-anchor `## For humans (optional)` → mark the whole section planned:
`*(Planned.)* `mtt-ui` (a small local web server): minimal task management, a Gantt chart, a KB browser. Plus CLI text output for task info, KB search, and an ASCII Gantt.`

- [ ] **Step 4: F6 — pitch: remove `mtt start` (EN, + the wrapped "task terms" RU)**

- Pitch (single line, `README.md:23`) — grep-anchor `a small Go CLI (`mtt add`, `mtt start`, `mtt done`, …)` → `a small Go CLI (`mtt add`, `mtt status`, `mtt done`, …)`. (RU `:23` mirror in Step 5, single line.)
- "Key ideas / killer feature" (**wraps across `README.md:44–45`**) — replace exactly:
  ```
  task terms
    (`mtt start`, `mtt done`) while the tool enforces the discipline.
  ```
  with:
  ```
  task terms (a gated status move like `mtt done`) while the tool enforces the discipline.
  ```
- Same clause **wraps in `README.ru.md:44–45`** — replace exactly:
  ```
  в терминах задач
    (`mtt start`, `mtt done`), а дисциплину держит инструмент.
  ```
  with:
  ```
  в терминах задач (переход-гейт вроде `mtt done`), а дисциплину держит инструмент.
  ```

- [ ] **Step 5: Mirror the remaining Step 1–4 edits in `README.ru.md` (RU)**

The two wrapped RU edits (F2 capabilities clause, F6 "task terms") were already applied in Steps 1 and 4. Here apply the RU mirror of the **single-line** edits at their parallel anchors:
- F3 — grep-anchor `**Проверяемые ссылки и append-only история.**` (`Задачи/комментарии несут проверяемые`) → refs shipped, comments planned (mirror of Step 1 F3).
- F4 — grep-anchor `**Опциональный человеческий UI (`mtt-ui`).**` → mark `(планируется)`.
- F5 — grep-anchor `## Для агентов` paragraph and `## Для людей (опционально)` → mark comment trees / text search / indexer / mtt-ui / Gantt as planned.
- F6 pitch — grep-anchor `небольшая Go CLI (`mtt add`, `mtt start`, `mtt done`, …)` → replace `mtt start` with `mtt status`.
Same facts as EN; keep the mirrors consistent.

- [ ] **Step 6: Full README/.ru residual pass**

Read `README.md` and `README.ru.md` end-to-end. Grep both for any remaining `mtt start`, `comment`, `search`, `mtt-ui`, `indexer`, `Gantt` mentions presented as present; fix any missed occurrence in both mirrors. Confirm EN↔RU paragraph parity.

- [ ] **Step 7: Guard + gate**

Run: `go test ./internal/cli/ -run 'TestDocs'` (expect PASS) then `make check` (expect green).

- [ ] **Step 8: Commit**

```bash
git add README.md README.ru.md
git commit -m "t42: README vision sections → planned markers; drop mtt start from pitch (F2-F6)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: `--json` precision + CHANGELOG stale claims (F7, F10)

**Files:**
- Modify: `internal/cli/root.go` (long-help), `CHANGELOG.md`

**Interfaces:**
- Consumes: registry-honesty from Task 1.
- Produces: `mtt --help` states the `--json` scope precisely; CHANGELOG carries no stale template-name / phantom claim.

- [ ] **Step 1: Check for a snapshot of the root long-help**

Run: `grep -rn "All commands support --json" internal/ ; grep -rln "mtt --help" internal/cli/testdata 2>/dev/null`
Expected: only `internal/cli/root.go` matches (no `_test.go`/txtar snapshot locks the string). If a snapshot exists, update it in this task.

- [ ] **Step 2: F7 — fix root.go long-help wording**

In `internal/cli/root.go`, grep-anchor `All commands support --json.` (end of the `Long:` string). Replace with a precise claim that matches reality (`version`/`init`/etc. emit JSON; cobra's `completion`/`help` do not):
`Every command that emits output supports --json (the generated completion/help aside).`

- [ ] **Step 3: Verify the binary reflects it**

Run: `make build && ./bin/mtt --help | grep -i json`
Expected: the new precise sentence; `./bin/mtt completion bash --json` still emits a script (the exemption is now honest).

- [ ] **Step 4: F10 — fix CHANGELOG stale template-names**

In `CHANGELOG.md`, grep-anchor `lists the valid names (`coding, default, hierarchy`)`. Since t62 leaves only `default` embedded and the same [Unreleased] block (grep-anchor `--template coding` / `--template hierarchy` … `removed`) already records the removal, change the c14 line to `lists the valid names (`default`)` — removing the internal contradiction.

- [ ] **Step 5: F7 — CHANGELOG `--json` claim (decide by section)**

Run: `grep -n "^## \[" CHANGELOG.md` to locate the section boundaries, then find the `JSON everywhere: every command honors --json` line. If it sits under **[Unreleased]**, soften to `every command that emits output honors --json`. If under a released version (e.g. `[0.9.0]`), LEAVE it (Keep-a-Changelog: released entries are immutable historical records; it is an imprecise summary, not a phantom) and note the decision in the commit message.

- [ ] **Step 6: Guard + gate**

Run: `go test ./internal/cli/ -run 'TestDocs'` (PASS) then `make check` (green). The build in Step 3 already refreshed `./bin/mtt`.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/root.go CHANGELOG.md
git commit -m "t42: precise --json help scope; CHANGELOG stale template-names (F7, F10)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: CLI_REFERENCE sweep (F7 verify, M4, residual — EN+RU)

**Files:**
- Modify: `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md` (only if drift found)

**Interfaces:**
- Consumes: nothing new.
- Produces: CLI_REFERENCE verified accurate; optional `mtt comment` inline honesty tag.

- [ ] **Step 1: F7 — verify the `--json everywhere` bullet is already accurate (no change expected)**

Run: `grep -n "json.*everywhere\|completion.*help.*exempt\|Только.*completion\|Исключены только" CLI_REFERENCE.md CLI_REFERENCE.ru.md`
Confirm both files already state "Only cobra's generated `completion`/`help` are exempt" / "Исключены только … `completion`/`help`". **This is the claim→verify outcome: the bullet is honest as written → NO edit.** Record "F7 CLI_REFERENCE: verified already accurate, no change" in the commit message (or skip the commit if this task makes no edit — see Step 4).

- [ ] **Step 2: M4 (optional) — inline honesty tag on `mtt comment` headings**

Grep-anchor `### `mtt comment add` and `### `mtt comment list` in `CLI_REFERENCE.md`. The section-level `## Comments *(phase 4; capability CommentStore)*` already marks them unshipped; for parity with every other unshipped command's inline tag, optionally append `*(phase 4)*` to each `mtt comment` heading. Mirror in `CLI_REFERENCE.ru.md`. (Nice-to-have; skip if it adds noise.)

- [ ] **Step 3: Residual EN↔RU parity pass**

Run: `grep -cE "^### \`mtt" CLI_REFERENCE.md CLI_REFERENCE.ru.md` (expect equal counts, verified 43=43 in review). Read both files section-by-section for any command whose documented flags/behavior drifted from `./bin/mtt <cmd> --help`; spot-check 3–4 commands (`add`, `list`, `rm`, `agent hooks`). Fix any drift in both mirrors.

- [ ] **Step 4: Guard + gate + commit (only if edited)**

Run: `go test ./internal/cli/ -run 'TestDocs'` (PASS) then `make check` (green). If Steps 2–3 produced edits:

```bash
git add CLI_REFERENCE.md CLI_REFERENCE.ru.md
git commit -m "t42: CLI_REFERENCE — comment-heading tags + residual EN/RU parity (M4)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

If no edits were needed, note "F7 CLI_REFERENCE verified accurate; no CLI_REFERENCE drift" in the Task 5 or final commit body and skip this commit.

---

## Task 5: DESIGN sweep (F9 YAML, F6 DESIGN, residual — EN+RU)

**Files:**
- Modify: `DESIGN.md` (F9 EN-only glue + F6), `DESIGN.ru.md` (F6 mirror)

**Interfaces:**
- Consumes: nothing new.
- Produces: valid hierarchy-template YAML in DESIGN.md; no `mtt start` in DESIGN prose.

- [ ] **Step 1: F9 — fix the glued YAML list items (EN only)**

In `DESIGN.md`, grep-anchor `to: cancelled}  - name: task` and `to: cancelled}  - name: subtask` (the hierarchy-template example). Split each glued line so the new `- name: task` / `- name: subtask` type entry starts on its own line at the correct indentation, matching the correct shape in `DESIGN.ru.md` (grep-anchor `- name: task` / `- name: subtask` there for the reference indentation). Verify no other `}  - name` glue remains: `grep -nE "}\s{2,}- name" DESIGN.md` → empty.

- [ ] **Step 2: F6 — `mtt start t17` → representative (EN)**

In `DESIGN.md`, grep-anchor `the agent works in task terms** (`mtt start t17`, `mtt done t17`)` → replace `mtt start t17` with a shipped-default-representative move, e.g. `mtt status t17 in_progress` (keeping `mtt done t17`, which works as sugar): `the agent works in task terms** (`mtt status t17 in_progress`, `mtt done t17`)`.

- [ ] **Step 3: F6 — mirror in DESIGN.ru.md (RU)**

In `DESIGN.ru.md`, grep-anchor `агент работает в терминах задач** (`mtt start t17`, `mtt done t17`)` → apply the same replacement.

- [ ] **Step 4: Residual DESIGN pass (parallel occurrences)**

Grep both DESIGN files for any other `mtt start` and for unshipped-as-present phrasing that reads as current rather than as a Shipped(tX)/parked note: `grep -n "mtt start\b" DESIGN.md DESIGN.ru.md`. DESIGN legitimately documents parked/future design in Shipped/PARKED blocks — only fix prose that states an unshipped feature as *available now*. Fix any finding in both mirrors.

- [ ] **Step 5: Guard + gate**

Run: `go test ./internal/cli/ -run 'TestDocs'` (PASS) then `make check` (green).

- [ ] **Step 6: Commit**

```bash
git add DESIGN.md DESIGN.ru.md
git commit -m "t42: DESIGN — fix glued hierarchy YAML (F9); mtt start -> representative (F6)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: FLOW_GUIDE sweep + final full-pass + CHANGELOG audit entry

**Files:**
- Modify: `FLOW_GUIDE.md`, `FLOW_GUIDE.ru.md` (only if drift found), `CHANGELOG.md` (audit entry), optionally `internal/cli/CLAUDE.md`

**Interfaces:**
- Consumes: all prior tasks.
- Produces: a clean final sweep + a CHANGELOG entry documenting the audit and the guard.

- [ ] **Step 1: FLOW_GUIDE residual pass (EN+RU)**

Read `FLOW_GUIDE.md` / `FLOW_GUIDE.ru.md`. They teach authoring custom flows (so `mtt doing`/custom verbs are legitimate illustration, not drift). Check only for: stale command names, wrong flags vs `./bin/mtt <cmd> --help`, and EN↔RU divergence. Fix any real drift in both mirrors. (Grep-anchor `mtt start` / `start` at FLOW_GUIDE.md:221 — confirm it describes a *custom flow's* `start` edge, not the shipped default; leave if illustrative.)

- [ ] **Step 2: Final cross-doc parallel-occurrence sweep**

Run a consolidated grep across ALL in-scope docs for the fixed facts, to catch any missed parallel occurrence:
```bash
grep -rn "0\.8\.98\|mtt guide\|coding, default, hierarchy" README.md README.ru.md DESIGN.md DESIGN.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md CHANGELOG.md FLOW_GUIDE.md FLOW_GUIDE.ru.md
grep -rn "\`mtt start\`\|mtt start t" README.md README.ru.md DESIGN.md DESIGN.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md FLOW_GUIDE.md FLOW_GUIDE.ru.md
```
Expected: no stale badge, no phantom `guide`, no stale template-name triplet, no `mtt start` presented as a shipped default verb (parked `### `mtt start`` headings in CLI_REFERENCE are fine — they are explicitly PARKED). Fix anything that remains, in both mirrors.

- [ ] **Step 3: AGENTS.md / CLAUDE.md intersection fact-check**

Run: `grep -rn "mtt guide\|0\.8\.98\|coding, default, hierarchy\|All commands support --json" AGENTS.md CLAUDE.md internal/*/CLAUDE.md`
Expected: no hits (agent docs were not asserting these). Fix only a direct factual intersection if found; do not restructure.

- [ ] **Step 4: Optionally note the guard in `internal/cli/CLAUDE.md`**

If `internal/cli/CLAUDE.md` enumerates the package's tests/invariants, add one line: the docs-guard test (`docs_guard_test.go`) asserts registry↔CLI_REFERENCE consistency, no phantom `mtt <verb>` tokens (FLOW_GUIDE excluded), and no hardcoded README version badge. Skip if the file doesn't track tests at that granularity.

- [ ] **Step 5: Add the CHANGELOG [Unreleased] audit entry**

In `CHANGELOG.md`, under `## [Unreleased]` → `### Changed` (or a `### Fixed`), add:
```markdown
- **Docs audit (t42).** Reconciled the human-facing docs (README/.ru, DESIGN/.ru, CLI_REFERENCE/.ru,
  CHANGELOG, FLOW_GUIDE/.ru) with the shipped surface: removed unshipped-as-present claims (comment
  threads, text search, `mtt-ui`, external indexer as present), a stale version badge and phase list, the
  never-existent `mtt guide`, an over-broad "all commands support `--json`", a stale template-name list, and
  a broken DESIGN hierarchy-template YAML example. Added a `make check` **docs-guard test** (registry ↔
  CLI_REFERENCE consistency, no phantom `mtt <verb>` tokens, no hardcoded README badge) so the docs cannot
  silently drift again.
```

- [ ] **Step 6: Guard + gate**

Run: `go test ./internal/cli/ -run 'TestDocs'` (PASS) then `make check` (green).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "t42: FLOW_GUIDE + final sweep; CHANGELOG audit entry + docs-guard note

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-review (run before `mtt submit`)

- **Spec coverage:** F1 (Task 1 §3–4 + Task 6 §2), F2/F3 (Task 2 §1), F4 (Task 2 §2), F5 (Task 2 §3), F6 (Task 2 §4 + Task 5 §2–3), F7 (Task 3 §2, §5 + Task 4 §1), F8 (Task 1 §5), F9 (Task 5 §1), F10 (Task 3 §4), M4 (Task 4 §2), guard-test (Task 1). Every spec finding maps to a task.
- **Placeholder scan:** no "TBD"/"handle edge cases"/"similar to" — every code step carries real Go; every doc step carries a grep anchor + the exact reword.
- **Type consistency:** `repoRoot()`, `NewRootCmd()`, `registryCommands()`, `docFile()`, `knownVerbs()`, and the three `TestDocs*` names are used consistently across Task 1 and the later "run the guard" steps.
- **Green-gate honesty:** Task 1 observes the guard RED in a run step (§2) and commits it GREEN together with F1/F8 fixes (§6–8) — never a red commit. All other tasks keep the guard green.
