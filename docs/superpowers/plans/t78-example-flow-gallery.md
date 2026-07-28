# t78 — Example-flow gallery + docs/no-code template — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a quick-start "pick a shape" gallery (a new FLOW_GUIDE §2b that indexes every shipped flow shape) plus one new runnable `templates/docs.yaml` (a gate-less `review ⇄ revision` no-code flow), keeping all example-set enumerations in sync.

**Architecture:** Documentation-first. One new hosted template file + two test additions (the only code), then doc edits: the canonical gallery in FLOW_GUIDE, a README pointer, and updates to every place that enumerates the hosted example set (CLI_REFERENCE, templates/README, templates/CLAUDE, two Go doc-comments, CHANGELOG). No Go logic changes; `init`'s built-in default is untouched.

**Tech Stack:** Go (test files, `gopkg.in/yaml.v3`), YAML (the template), Markdown (bilingual EN+RU docs). Spec: [t78-example-flow-gallery.md](../specs/t78-example-flow-gallery.md).

## Global Constraints

- **No Go core change:** nothing under `cmd/`, `pkg/`, or `internal/**` logic changes; only test files and two doc-comments are touched. `templates.go` still `//go:embed`s **only** `default.yaml`; `BuiltinNames()` stays `{"default"}`.
- **Verbatim-install contract:** `templates/docs.yaml` is a hosted (non-embedded) example → **literal** `project.name: my-project` (never `{{.Name}}`), installed byte-for-byte.
- **Bilingual sync:** `FLOW_GUIDE.md ↔ .ru.md`, `README.md ↔ .ru.md`, `CLI_REFERENCE.md ↔ .ru.md` change together. `templates/*` docs are EN-only.
- **Do NOT touch `DESIGN.md`/`.ru.md`** — its example-set mention is an archival design-record (c23 territory); it stays true-but-non-exhaustive (spec D5).
- **Gate:** `make check` (gofmt + go vet + golangci-lint v2 + `go test -race -cover` + build) green before the impl `mtt submit`.
- **§2b heading (exact, both langs, parenthesised to yield a clean anchor):** EN `## 2b. Example flow gallery (pick a shape)` → `#2b-example-flow-gallery-pick-a-shape`; RU `## 2b. Галерея примеров флоу (выбери форму)`.
- **The 5 gallery rows, fixed order:** minimal · coding · docs/no-code · hierarchy · git-flow (⚠ advanced) — plus a pointer to §7 for non-code domains with gates/posts.

---

### Task 1: `templates/docs.yaml` + its CI guards (TDD)

The one code/test task. Write the test changes first (RED — file absent), then create the template (GREEN).

**Files:**
- Create: `templates/docs.yaml`
- Modify/Test: `templates/templates_test.go` (add `"docs.yaml"` to `TestExamplesLoadAndValidate`; add `TestDocsIsNoCode`)

**Interfaces:**
- Consumes: `yaml.ValidateTemplateBytes([]byte) error` (existing, `internal/adapter/yaml`, imported as `yaml`); `gopkg.in/yaml.v3` `Unmarshal` (dep at `go.mod:9`, must be imported under an alias — see Step 1 note).
- Produces: `templates/docs.yaml` — a `doc` type (`prefix: d`, `default: true`), statuses `draft`(initial)/`review`(active)/`revision`(active)/`published`(terminal)/`cancelled`(terminal), edges `submit`/`publish`/`request_changes`/`resubmit` (+ unnamed cancels), zero `commands`/`post`/`post_defaults`/`events`.

- [ ] **Step 1: Add the test changes (both RED)**

In `templates/templates_test.go`, add `"docs.yaml"` to the existing slice so the line reads:

```go
	for _, name := range []string{"coding.yaml", "docs.yaml", "hierarchy.yaml", "git-flow.yaml"} {
```

Add a new import — the file already imports the internal adapter as `yaml`, so alias yaml.v3 to avoid the collision. The import block becomes:

```go
import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/internal/adapter/yaml"
	yamlv3 "gopkg.in/yaml.v3"
)
```

Append the structural no-code guard:

```go
func TestDocsIsNoCode(t *testing.T) {
	// The docs/no-code starter must EXECUTE NOTHING — no gate, no post, no
	// lifecycle-event pipeline. A structural check (not a token grep) proves it,
	// catching any side-effecting step, not just git/gh. `events` is captured as
	// a whole so a command hidden in any store×kind slot cannot slip past; a
	// `rollback:` only nests under a `command`, so empty `commands` ⇒ no rollback.
	data, err := os.ReadFile("docs.yaml")
	if err != nil {
		t.Fatalf("read docs.yaml: %v", err)
	}
	var doc struct {
		Types []struct {
			PostDefaults []any `yaml:"post_defaults"`
			Transitions  []struct {
				From     string `yaml:"from"`
				To       string `yaml:"to"`
				Commands []any  `yaml:"commands"`
				Post     []any  `yaml:"post"`
			} `yaml:"transitions"`
		} `yaml:"types"`
		Events map[string]any `yaml:"events"`
	}
	if err := yamlv3.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal docs.yaml: %v", err)
	}
	if len(doc.Events) != 0 {
		t.Errorf("docs.yaml must define no events, got: %v", doc.Events)
	}
	for _, ty := range doc.Types {
		if len(ty.PostDefaults) != 0 {
			t.Errorf("docs.yaml must carry no post_defaults")
		}
		for _, tr := range ty.Transitions {
			if len(tr.Commands) != 0 || len(tr.Post) != 0 {
				t.Errorf("docs.yaml %s→%s must have no commands/post (no-code)", tr.From, tr.To)
			}
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they FAIL**

Run: `go test ./templates/ -run 'TestExamplesLoadAndValidate|TestDocsIsNoCode' -v`
Expected: FAIL — both fatal on `read docs.yaml: open docs.yaml: no such file or directory`.

- [ ] **Step 3: Create `templates/docs.yaml`**

```yaml
version: 1
project:
  name: my-project
command_timeout: 5m
types:
  - name: doc
    description: A document or no-code deliverable (RFC, spec, article). DoD — reviewed and published; iterate via revision. No branch, no PR, no build gate.
    prefix: d
    parents: []
    default: true
    statuses:
      - {name: draft,     kind: initial}
      - {name: review,    kind: active}
      - {name: revision,  kind: active}
      - {name: published, kind: terminal}
      - {name: cancelled, kind: terminal}
    # A docs / no-code flow gates on human review, not on a build: no branch is
    # created, no PR is opened, nothing runs. The review<->revision loop is the
    # point — a reviewer can send a doc back any number of times. Want an
    # automated non-code check (a link checker, a markdown linter)? Hang a
    # `commands:` list on `publish` (see FLOW_GUIDE §4).
    transitions:
      - {from: draft,    to: review,    name: submit,          description: "hand the draft to a reviewer", current: set}
      - {from: draft,    to: cancelled, description: "drop the draft"}
      - {from: review,   to: published, name: publish,         description: "accepted — publish", current: clear}
      - {from: review,   to: revision,  name: request_changes, description: "send back for edits"}
      - {from: review,   to: cancelled, description: "reject the doc"}
      - {from: revision, to: review,    name: resubmit,        description: "edits done — re-review"}
      - {from: revision, to: cancelled, description: "abandon the doc"}
```

- [ ] **Step 4: Run the tests to verify they PASS**

Run: `go test ./templates/ -run 'TestExamplesLoadAndValidate|TestDocsIsNoCode' -v`
Expected: PASS (both). `ValidateTemplateBytes` accepts the graph (one initial, two active — the `review⇄revision` cycle supplies both endpoints' in+out edges — two terminal; single default type; edge names unique per source and disjoint from status names; prefix `d` letters-only).

- [ ] **Step 5: Sanity-drive the template (optional but recommended)**

Run: `go build -o bin/mtt ./cmd/mtt && (cd "$(mktemp -d)" && /home/gss/projects/mtt/bin/mtt init --template /home/gss/projects/mtt/templates/docs.yaml --no-agent-hooks --no-agent-docs && /home/gss/projects/mtt/bin/mtt types)`
Expected: init succeeds; `mtt types` renders the `doc` graph with the `submit`/`publish`/`request_changes`/`resubmit` verbs and no gate commands.

- [ ] **Step 6: Commit**

```bash
git add templates/docs.yaml templates/templates_test.go
git commit -m "t78: add docs.yaml no-code template (review<->revision loop) + CI guards"
```

---

### Task 2: FLOW_GUIDE gallery §2b + install-line + Neighbours (EN + RU)

The canonical gallery lives here; keep EN and RU byte-parallel.

**Files:**
- Modify: `FLOW_GUIDE.md` (insert §2b before `## 3. The graph` at line 60; update §1 install line at :32; update §12 Neighbours at :287)
- Modify: `FLOW_GUIDE.ru.md` (mirror: before `## 3. Граф` at line 59; §1 at :31; §12 at :289)

**Interfaces:**
- Consumes: `templates/docs.yaml` (Task 1) — the gallery's docs row installs it.
- Produces: anchor `#2b-example-flow-gallery-pick-a-shape` (EN), referenced by Task 3's README pointer.

- [ ] **Step 1: Insert §2b into `FLOW_GUIDE.md`** (between the §2 closing paragraph and `## 3. The graph`)

```markdown
## 2b. Example flow gallery (pick a shape)

Don't start from a blank config — start from the nearest shape and adapt it. Only `default` is built into
the binary; the rest install from the repo's `templates/` dir by path or URL (e.g.
`mtt init --template templates/coding.yaml`, or the
`https://raw.githubusercontent.com/pashukhin/mtt/main/templates/coding.yaml` form). Each is a **sample you
adapt**, never a mandate — inspect any with `mtt types` after install.

| Shape | Use when | Try |
|---|---|---|
| **minimal** | solo, zero-config; one linear flow you add your own gate to | `mtt init` (built-in `default`, gate-less — §2 shows adding a gate) |
| **coding** | a software project; per-type DoD (feature / bugfix / refactor) | `mtt init --template templates/coding.yaml` (walkthrough: `demo/`, §4) |
| **docs / no-code** | an iterative **review ⇄ revision** cycle with no code gate — docs, RFCs, specs, articles | `mtt init --template templates/docs.yaml` |
| **hierarchy** | epic → task → subtask nesting | `mtt init --template templates/hierarchy.yaml` |
| **git-flow** ⚠ **advanced** | branch → agent review → PR → deliver (this repo's own shape); needs `gh`+`jq` — review before use | `mtt init --template templates/git-flow.yaml` (§8) |

For **non-code domains with a gate or post** — a blog post gated by a review script, a purchase request
validated at the approve edge — see the content-review and approval samples in §7.
```

- [ ] **Step 2: Update the §1 install line in `FLOW_GUIDE.md`** (line 32)

Replace:

```
templates/coding.yaml`, `templates/hierarchy.yaml`, or the `git-flow` flagship — see §8), then inspect any
```

with:

```
templates/coding.yaml`, `templates/docs.yaml`, `templates/hierarchy.yaml`, or the `git-flow` flagship — see the gallery in §2b), then inspect any
```

- [ ] **Step 3: Update §12 Neighbours in `FLOW_GUIDE.md`** (line 287)

Replace the sentence:

```
Shipped (t62): the **git-flow flagship** and the `coding`/`hierarchy` samples live in the repo's `templates/`
dir, installable by path or `--template <url>` (only `default` is built into the binary). Still tracked
separately:
```

with (present-tense live catalog — resolves the "Shipped(t62)" label inconsistency; the templates mechanism note keeps the t62 provenance):

```
The repo's `templates/` dir ships installable samples — the `coding`, `docs`, and `hierarchy` flows and the
**git-flow flagship** (by path or `--template <url>`; only `default` is built into the binary; the templates
mechanism landed in t62) — browsable in the gallery (§2b). Still tracked separately:
```

- [ ] **Step 4: Mirror all three edits into `FLOW_GUIDE.ru.md`**

Insert §2b (before `## 3. Граф`, line 59):

```markdown
## 2b. Галерея примеров флоу (выбери форму)

Не начинай с пустого конфига — возьми ближайшую форму и адаптируй. В бинарник встроен только `default`;
остальные ставятся из каталога `templates/` по пути или URL (например,
`mtt init --template templates/coding.yaml` или форма
`https://raw.githubusercontent.com/pashukhin/mtt/main/templates/coding.yaml`). Каждая — **образец для
адаптации**, а не мандат; после установки смотри любую через `mtt types`.

| Форма | Когда брать | Попробовать |
|---|---|---|
| **minimal** | solo, zero-config; один линейный флоу, гейт добавляешь сам | `mtt init` (встроенный `default`, без гейта — §2 показывает, как добавить) |
| **coding** | софт-проект; per-type DoD (feature / bugfix / refactor) | `mtt init --template templates/coding.yaml` (разбор: `demo/`, §4) |
| **docs / no-code** | итеративный цикл **review ⇄ revision** без кодового гейта — доки, RFC, спеки, статьи | `mtt init --template templates/docs.yaml` |
| **hierarchy** | вложенность epic → task → subtask | `mtt init --template templates/hierarchy.yaml` |
| **git-flow** ⚠ **advanced** | branch → agent review → PR → deliver (форма этого репо); нужны `gh`+`jq` — читай перед use | `mtt init --template templates/git-flow.yaml` (§8) |

Для **не-кодовых доменов с гейтом или post** — пост в блоге, проверяемый скриптом-ревью, или заявка,
валидируемая на ребре approve — см. образцы content-review и approval в §7.
```

§1 install line (line 31) — replace:

```
(`mtt init --template templates/coding.yaml`, `templates/hierarchy.yaml` или флагман `git-flow` — см. §8),
```

with:

```
(`mtt init --template templates/coding.yaml`, `templates/docs.yaml`, `templates/hierarchy.yaml` или флагман `git-flow` — см. галерею в §2b),
```

§12 Соседи — the RU block spans **three** lines (289-291); quote all three as the before-text (mirroring the EN 3-line quote), replace:

```
Выпущено (t62): **флагман git-flow** и образцы `coding`/`hierarchy` лежат в каталоге репо `templates/`,
ставятся по пути или `--template <url>` (в бинарь вкомпилирован только `default`). Всё ещё трекается
отдельно:
```

with (the last clause stays split across a line break exactly as in the file — "трекается" then "отдельно:"):

```
Каталог репо `templates/` поставляет устанавливаемые образцы — флоу `coding`, `docs`, `hierarchy` и
**флагман git-flow** (по пути или `--template <url>`; в бинарь вкомпилирован только `default`; механизм
шаблонов появился в t62) — их можно листать в галерее (§2b). Всё ещё трекается
отдельно:
```

(EN Step 3 already quotes its full 3-line block `FLOW_GUIDE.md:287-289`; this makes RU mirror it, so neither edit leaves a dangling continuation.)

- [ ] **Step 5: Verify build/render, then commit**

Run: `make check` — Expected: PASS (docs-only; no Go change). Eyeball that both §2b tables render (5 rows, git-flow flagged advanced) and the EN anchor is `#2b-example-flow-gallery-pick-a-shape`.

```bash
git add FLOW_GUIDE.md FLOW_GUIDE.ru.md
git commit -m "t78: FLOW_GUIDE §2b example-flow gallery (EN+RU) + install-line/Neighbours refresh"
```

---

### Task 3: Sync surface — README pointer, CLI_REFERENCE, templates docs, comments, CHANGELOG

Every remaining place that points to the gallery or enumerates the hosted set.

**Files:**
- Modify: `README.md` (pointer after Quickstart block), `README.ru.md` (mirror)
- Modify: `CLI_REFERENCE.md:228`, `CLI_REFERENCE.ru.md:229` (add `docs` to the enumeration)
- Modify: `templates/README.md` (add `docs.yaml` row + gallery pointer), `templates/CLAUDE.md` (add `docs.yaml`)
- Modify: `templates/templates.go:4` (comment), `internal/adapter/yaml/load_test.go:12-13` (comment) — reword generically
- Modify: `CHANGELOG.md` (`[Unreleased]` → `### Added`)

**Interfaces:**
- Consumes: `templates/docs.yaml` (Task 1); the §2b anchor (Task 2).

- [ ] **Step 1: README Quickstart pointer**

In `README.md`, immediately after the Quickstart code block's closing ``` (line 130) and before `See [CLI_REFERENCE.md]...`, insert:

```markdown
> **Which flow?** `default` is minimal (task + chore). For a per-type coding DoD, a docs/no-code loop, an
> epic→task→subtask hierarchy, or the full git branch→PR→deliver flow, browse the **example-flow gallery**
> in [FLOW_GUIDE.md §2b](FLOW_GUIDE.md#2b-example-flow-gallery-pick-a-shape).
```

In `README.ru.md`, at the mirror location — after the Quickstart code block's closing ``` (line 132) and before `Полный список команд…` (line 134):

```markdown
> **Какой флоу?** `default` минимален (task + chore). Для per-type coding-DoD, docs/no-code-петли, иерархии
> epic→task→subtask или полного git-флоу branch→PR→deliver — смотри **галерею примеров** в
> [FLOW_GUIDE.ru.md §2b](FLOW_GUIDE.ru.md#2b-галерея-примеров-флоу-выбери-форму).
```

- [ ] **Step 2: CLI_REFERENCE enumeration (EN + RU)**

`CLI_REFERENCE.md:228` — replace `(`coding`, `hierarchy`, and the `git-flow` flagship)` with `(`coding`, `docs`, `hierarchy`, and the `git-flow` flagship)`.
`CLI_REFERENCE.ru.md:229` — replace `(`coding`, `hierarchy` и флагман `git-flow`)` with `(`coding`, `docs`, `hierarchy` и флагман `git-flow`)`.

- [ ] **Step 3: `templates/README.md` — add the row + demote to a file list pointing at the gallery**

Add the `docs.yaml` bullet after the `coding.yaml` line (keep the fixed order default/coding/docs/hierarchy/git-flow):

```markdown
- `docs.yaml`     — a docs / no-code flow: a gate-less `review ⇄ revision` loop (no branch, no PR).
```

Append a chooser pointer at the end of the file (so this stays a terse file list, not a second "when to pick" index):

```markdown

For **which to pick**, see the example-flow gallery in
[../FLOW_GUIDE.md §2b](../FLOW_GUIDE.md#2b-example-flow-gallery-pick-a-shape).
```

- [ ] **Step 4: `templates/CLAUDE.md` — add `docs.yaml` to the enumerations + note its invariant**

Lines 11-12: the enumeration straddles a line break (`` `coding.yaml`, `` ends line 11, `` `hierarchy.yaml`, `` starts line 12), so match across the newline. Replace the two-line fragment:

```
built-in names: `coding.yaml`,
  `hierarchy.yaml`, and the **`git-flow.yaml` flagship**
```

with:

```
built-in names: `coding.yaml`,
  `docs.yaml`, `hierarchy.yaml`, and the **`git-flow.yaml` flagship**
```
Line 19: change `the file examples (`coding`/`hierarchy`/`git-flow`) are installed **verbatim**` to `the file examples (`coding`/`docs`/`hierarchy`/`git-flow`) are installed **verbatim**`.
In the Tests section, add one line after the `TestExamplesLoadAndValidate` description:

```markdown
`TestDocsIsNoCode` asserts `docs.yaml` executes nothing (no `commands`/`post`/`post_defaults`/`events`) — the
structural form of its "no branch, no PR, no gate" promise.
```

- [ ] **Step 5: Reword the two Go doc-comments generically (no enumeration to drift)**

`templates/templates.go` line 4 — change:

```go
// examples (coding, hierarchy, git-flow) — de-embedded per the t62 minimal-
```

to:

```go
// examples (the hosted templates/*.yaml) — de-embedded per the t62 minimal-
```

`internal/adapter/yaml/load_test.go` lines 12-13 — change:

```go
	// t62: only `default` is a built-in; the hosted examples (coding/hierarchy/
	// git-flow) are load+validated by templates/templates_test.go.
```

to:

```go
	// t62: only `default` is a built-in; the hosted templates/*.yaml examples
	// are load+validated by templates/templates_test.go.
```

- [ ] **Step 6: CHANGELOG `[Unreleased] → ### Added`**

Add as the first bullet under `### Added` (line 9):

```markdown
- **Example-flow gallery + a docs/no-code starter (t78).** A new `docs.yaml` hosted template — a gate-less
  `review ⇄ revision` no-code flow (no branch, no PR) — and an **example-flow gallery** in FLOW_GUIDE (§2b)
  that indexes every shipped shape (`default`/`coding`/`docs`/`hierarchy`/`git-flow`) with a one-line
  "use when" and a copy-paste install; README and CLI_REFERENCE point to it. Docs-only; `mtt init`'s
  minimal built-in default is unchanged.
```

- [ ] **Step 7: Full gate + commit**

Run: `make check` — Expected: PASS (the two comment rewords compile; everything else is Markdown).

```bash
git add README.md README.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md templates/README.md templates/CLAUDE.md templates/templates.go internal/adapter/yaml/load_test.go CHANGELOG.md
git commit -m "t78: sync surface — README/CLI_REFERENCE pointers, templates docs, comments, CHANGELOG"
```

---

## Self-Review

**1. Spec coverage.** D1 gallery §2b + README pointer → Task 2 Step 1 + Task 3 Step 1. D2 five rows + §7 pointer → Task 2 Step 1. D3 `docs.yaml` review⇄revision → Task 1 Step 3. D4 `TestExamplesLoadAndValidate` + structural `TestDocsIsNoCode` → Task 1 Steps 1–4. D5 full sync surface: CLI_REFERENCE (T3 S2), templates/README (T3 S3), templates/CLAUDE (T3 S4), templates.go + load_test.go comments (T3 S5), FLOW_GUIDE §1/§12 (T2 S2–S4), CHANGELOG (T3 S6), DESIGN deliberately excluded (Global Constraints). All covered.

**2. Placeholder scan.** No TBD/TODO; every doc edit gives the literal before/after text; the template and tests are complete code.

**3. Type/name consistency.** `docs.yaml` statuses/edges are identical across Task 1 (YAML), the gallery prose, and CHANGELOG. The yaml.v3 import is aliased `yamlv3` to avoid colliding with the internal `yaml` import — used consistently in `TestDocsIsNoCode`. The §2b anchor `#2b-example-flow-gallery-pick-a-shape` is used identically in the README pointer (T3 S1) and templates/README pointer (T3 S3).

## Execution note

Tasks are ordered so each ends green: Task 1 (`go test ./templates/`), Tasks 2–3 (`make check`). Task 1 must land first — the gallery/README/CLI_REFERENCE reference `templates/docs.yaml` by path, so the file must exist for those references to be honest.
