# GitHub-gated flow — Implementation Plan (t76)

> **For agentic workers:** REQUIRED SUB-SKILL: use superpowers:executing-plans (inline, LIGHT weight)
> to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a `github-gated` flagship init template that puts executable mtt gates on the close of a
GitHub issue — using only existing primitives (`mtt` self-calls + t40 env + `gh`), near-zero new Go —
proven by one out-of-tree dogfood pass, with the enforcement-honesty and t40-enabler story in the docs.

**Architecture:** Two linked entities, not a status mirror (spec §"The clean model"). A GitHub issue keeps
its native `open⇄closed` model; a rich mtt task (this flow, local YAML) *gates* it. The issue's **close is
the terminal action** of the task (`gh issue close` in the `approve→done` post, only reachable after the
flow's gates). Drift (an issue moved directly in the UI / raw `gh`) is **detected and complained about via a
polled `sync` self-loop**, never enforced. The template is a *hosted, verbatim-installed* example (like
`git-flow.yaml`): literal `project.name`, no `{{.Name}}` render.

**Tech Stack:** YAML config template; `gh` (authenticated) + `jq` on PATH; mtt's t40 task-context env
(`MTT_TASK_*`); shape-safe `{{.ID}}` interpolation only. No new Go unless the dogfood proves need (default:
none — Task 3 records the evidence-backed decision).

## Global Constraints (from the spec — every task inherits these)

- **LIGHT execution weight** (spec §"Execution weight"): config + one out-of-tree dogfood + docs, near-zero
  product Go. ONE adversarial impl_review + the human gate — not the full apparatus.
- **Do NOT build `mtt tt` up front** (spec §"What this task IS", item 3). Task 3 decides from evidence;
  default is *don't build* — record the finding either way.
- **Enforcement honesty in the docs** (spec §"Enforcement honesty"): mtt gates only `mtt`-mediated moves;
  the choke point is `mtt <edge>`; **any** actor (human *or* agent) closing/reopening the issue directly in
  the GitHub UI or via raw `gh` **bypasses** the gate. Drift detection is **polled** (no webhook; needs `gh`
  + network at run time; can lag) and **symmetric** (closed-while-unfinished *or* reopened-while-done).
  Never claim to *enforce*.
- **Two known interactions must be pre-noted in the template header AND the docs** (spec §"Dogfood plan"):
  (1) **attribution clunk** — under `require.who`, the inner `mtt ref add` self-call in `start`'s post needs
  an author too (exit 2 → `PostActionError` → exit 5); set `author:` in `.mtt/config.local.yaml` FIRST.
  (2) **mutate-in-`post:` re-entrancy** — that same `mtt ref add` fires the `task.update` lifecycle event,
  so combined with an auto-commit/push events block it triggers a nested commit+push inside the outer move.
- **Bilingual human docs** (CLAUDE.md): EN primary + RU mirror kept in sync — `README(.ru).md`,
  `DESIGN(.ru).md`, `CLI_REFERENCE(.ru).md`, `FLOW_GUIDE(.ru).md`. Agent docs (AGENTS/CLAUDE) EN-only.
  Grep **all** parallel occurrences (EN+RU + "Shipped" blocks) of any changed flow-fact.
- **Gate discipline:** `make check` green before **every** commit (write to a file / check `$?` separately —
  a piped `tail` masks the exit code). Run `make fmt` if gofmt struct-alignment trips the gate. Avoid `#` in
  mtt task/note descriptions (mints parasitic tags). Commit non-`.mtt` code/docs **before** `mtt submit`/
  `mtt approve` (clean-tree gate).
- **Attribution:** do NOT export `MTT_BY`; the canonical author is `.mtt/config.local.yaml`
  (`author: Grigorii Pashukhin`). `--why` without backticks.

---

## Preconditions (already done this session — do not repeat)

- `task/t76` was **behind** main and lacked t40's code (it branched before t40 merged). `origin/main` was
  merged into `task/t76` (clean, zero file overlap) so the branch now carries t40 (`internal/cli/taskenv.go`
  present) and `make check` is green on the merged base. This is required because the template's `MTT_TASK_*`
  env-form only works against a t40 binary, and the empirical plan/impl reviews rebuild from the branch.

## Verified facts (grounding — checked empirically this session)

- **`mtt ref list <id> --json`** shape: `{"refs":[{"kind","id","status"}],"backlinks":[]}` — the ref target
  is field **`.id`** (NOT `.target`). jq extractor (no inner double-quotes, YAML-clean):
  `jq -r --arg k url '.refs[] | select(.kind==$k) | .id'`.
- **`MTT_TASK_JSON` does NOT carry refs** (`toTaskJSON` omits them) — so reading the linked issue URL back
  needs an `mtt ref list --json` self-call (read-only, takes no lock, safe). This is a genuine t40 gap and a
  Task-3 friction finding.
- **Self-loop transitions (`from==to`) are valid** and the named-edge sugar works (`mtt sync <id>`).
- **t40 env** a gate/post sees: `MTT_TASK_JSON` (lean shape, no refs), `MTT_TASK_CHILDREN_JSON`
  (`[{"id","type","status"}]`, `[]` when none), scalars `MTT_TASK_ID/TYPE/STATUS/TITLE/PARENT/PRIORITY/TAGS`.
  `{{...}}` stays shape-safe (`ID/Type/From/To`); free text (title) rides env as **data**.
- `gh` v2.96 authenticated (repo scope), `jq` 1.7 present. `mtt ref add <id> url:<https-url>` works; a bare
  host without scheme is rejected.

---

## Task 1: Author `templates/github-gated.yaml` (TDD via the examples-validate test)

**Files:**
- Modify: `templates/templates_test.go` (add `github-gated.yaml` to the validated list; add an assumptions test)
- Create: `templates/github-gated.yaml`
- Modify: `templates/README.md`, `templates/CLAUDE.md` (register the new hosted example)

**Interfaces:**
- Consumes: `yaml.ValidateTemplateBytes` (existing) — must accept the new file as standalone-valid YAML.
- Produces: a `task` type named-edge flow `start / submit / approve / decline / sync / cancel` over statuses
  `tbd → in_progress → review → done (+ cancelled)`.

- [ ] **Step 1: Extend the examples list (red).** In `templates/templates_test.go`,
      `TestExamplesLoadAndValidate`, change the loop list to include the new file:

```go
	for _, name := range []string{"coding.yaml", "hierarchy.yaml", "git-flow.yaml", "github-gated.yaml"} {
```

- [ ] **Step 2: Add a stated-assumptions test (red).** Append to `templates/templates_test.go` — mirrors
      `TestGitFlowStatesItsAssumptions` so a future edit that drops a safety statement fails CI:

```go
func TestGitHubGatedStatesItsAssumptions(t *testing.T) {
	data, err := os.ReadFile("github-gated.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ToLower(string(data))
	// no silent traps: the header/edges must STATE every external assumption and honesty limit.
	for _, want := range []string{"gh", "jq", "--no-run", "drift", "author", "bypass"} {
		if !strings.Contains(s, want) {
			t.Fatalf("github-gated.yaml must state %q (no silent traps / enforcement honesty)", want)
		}
	}
}
```

- [ ] **Step 3: Run the tests to confirm they FAIL.**

Run: `go test ./templates/ -run 'TestExamplesLoadAndValidate|TestGitHubGatedStatesItsAssumptions' -v`
Expected: FAIL — `read github-gated.yaml: ... no such file`.

- [ ] **Step 4: Create `templates/github-gated.yaml`** with exactly this content (double-quoted YAML scalars
      so the jq `--arg` filters and the `\"$MTT_TASK_TITLE\"` quoting are unambiguous; `{{.ID}}` is the only
      interpolation; free text rides the env):

```yaml
version: 1
# github-gated — the GitHub-integration flagship (a generalized, adaptable SAMPLE, not a mandate).
#
# TWO LINKED ENTITIES, not a status mirror: a GitHub issue keeps its native open/closed model; a rich
# mtt task (this flow, local YAML) GATES it. The issue's CLOSE is the TERMINAL ACTION of the task — the
# `approve → done` post runs `gh issue close`, reachable only after the flow's gates pass. This is the
# release story: "put executable mtt gates on the close of your GitHub issues."
#
# ENFORCEMENT HONESTY — mtt gates only `mtt`-mediated moves. The choke point is `mtt <edge>`, the interface
# the agent uses. ANY actor, human OR agent, closing/reopening the issue directly in the GitHub UI or with a
# raw `gh` call BYPASSES the gate (no local hook intercepts it). Drift is DETECTED and COMPLAINED about, never
# enforced. The `sync` self-loop is a POLLED, SYMMETRIC drift check (needs `gh`+network at run time; can lag).
#
# REQUIRES on PATH: `gh` (authenticated — `gh auth status`) and `jq`; run inside a git repo whose GitHub
# remote `gh` can infer (or add `--repo owner/name` to the `gh` calls). ADAPT the commands to your project.
#
# TWO KNOWN INTERACTIONS you must handle:
#   1. ATTRIBUTION — the inner `mtt ref add` self-call in `start`'s post is an `mtt` MUTATION. If you enable
#      `require.who` (as mtt's own dogfood config does), that self-call needs an author too, or it exits 2
#      (→ PostActionError → exit 5). Set `author:` in `.mtt/config.local.yaml` FIRST (or pass `--who`).
#   2. MUTATE-IN-POST — that same `mtt ref add` fires the `task.update` lifecycle event. Combined with an
#      auto-commit/push `events:` block (e.g. alongside git-flow) it triggers a NESTED commit+push inside
#      `start`'s post phase. Not a deadlock (reads take no lock), but real churn — scope it, or capture the
#      URL without a mid-move mutation.
#
# state-moving `cancel` edges carry the `--no-run` caveat: a bypass skips ALL commands (including the issue
# close), so you own the GitHub side by hand then.
project:
  name: my-project
command_timeout: 5m
types:
  - name: task
    prefix: t
    parents: []
    default: true
    description: "A unit of work that GATES a GitHub issue's close: mtt runs the flow; `gh` moves the issue only at the gated edges."
    statuses:
      - {name: tbd, kind: initial, description: "queued; `mtt start` creates the linked GitHub issue and takes the task into work"}
      - {name: in_progress, kind: active, description: "the linked issue is open; do the work, `mtt sync` to poll for drift, then `mtt submit`"}
      - {name: review, kind: active, description: "agent gate: run an adversarial review; `mtt approve` closes the issue (the gated terminal action), `mtt decline` sends it back"}
      - {name: done, kind: terminal, description: "the gated close ran (`gh issue close --reason completed`): the issue is closed BECAUSE the task passed its flow"}
      - {name: cancelled, kind: terminal, description: "abandoned; any linked issue is closed as not planned"}
    transitions:
      - from: tbd
        to: in_progress
        name: start
        description: "create the GitHub issue for this task and capture its URL back as a url: ref (set author FIRST if require.who is on — see the header)"
        post:
          - "u=$(gh issue create --title \"$MTT_TASK_TITLE\" --body \"Gated by mtt task {{.ID}} — closed only when its mtt flow reaches done.\") && mtt ref add {{.ID}} \"url:$u\""
      - {from: tbd, to: cancelled, name: cancel, description: "abandon before any issue exists"}
      - from: in_progress
        to: in_progress
        name: sync
        description: "POLLED, SYMMETRIC drift check (read-only): compare the linked issue's open/closed state to this task and COMPLAIN on a mismatch. A direct UI/`gh` move is caught only here, never blocked."
        commands:
          - "u=$(mtt ref list {{.ID}} --json | jq -r --arg k url '.refs[] | select(.kind==$k) | .id' | head -n1); [ -n \"$u\" ] || { echo \"no linked issue url ref on {{.ID}}\" >&2; exit 1; }; s=$(gh issue view \"$u\" --json state --jq .state); [ \"$s\" = OPEN ] || { echo \"DRIFT: issue $u is $s but task {{.ID}} is not done — reconcile\" >&2; exit 1; }"
      - {from: in_progress, to: review, name: submit, description: "work done; run the agent review before the gated close"}
      - from: review
        to: done
        name: approve
        description: "agent review passed → TERMINAL ACTION: close the linked issue. Drift guard: refuses if the issue was already closed out of band."
        commands:
          - "u=$(mtt ref list {{.ID}} --json | jq -r --arg k url '.refs[] | select(.kind==$k) | .id' | head -n1); [ -n \"$u\" ] || { echo \"no linked issue url ref on {{.ID}}\" >&2; exit 1; }; s=$(gh issue view \"$u\" --json state --jq .state); [ \"$s\" = OPEN ] || { echo \"DRIFT: issue $u already $s (closed out of band?) — reconcile before mtt closes it\" >&2; exit 1; }"
        post:
          - "u=$(mtt ref list {{.ID}} --json | jq -r --arg k url '.refs[] | select(.kind==$k) | .id' | head -n1); gh issue close \"$u\" --reason completed"
      - {from: review, to: in_progress, name: decline, description: "review found issues; address them, then `mtt submit`"}
      - from: in_progress
        to: cancelled
        name: cancel
        description: "abandon; close the linked issue as not planned. --no-run skips this — you own the close by hand then."
        post:
          - "u=$(mtt ref list {{.ID}} --json | jq -r --arg k url '.refs[] | select(.kind==$k) | .id' | head -n1); [ -z \"$u\" ] || gh issue close \"$u\" --reason \"not planned\""
      - {from: review, to: cancelled, name: cancel, description: "abandon from review; close the issue as not planned by hand (or mirror the in_progress cancel post:)"}
```

- [ ] **Step 5: Run the template tests to confirm they PASS.**

Run: `go test ./templates/ -run 'TestExamplesLoadAndValidate|TestGitHubGatedStatesItsAssumptions' -v`
Expected: PASS (the file loads + validates; all stated-assumption tokens present).

- [ ] **Step 6: Register the example in `templates/README.md`** — add under the bullet list, after the
      `git-flow.yaml` line:

```markdown
- `github-gated.yaml` — the GitHub-integration flagship: puts executable mtt gates on a GitHub issue's
  close (two linked entities; the issue's close is the task's terminal action; drift is detect+complain,
  never enforced). Needs `gh`+`jq`; read its header before use.
```

- [ ] **Step 7: Register the example in `templates/CLAUDE.md`** — extend the "other files are hosted
      examples" sentence to name `github-gated.yaml`, and add to the Tests section that
      `TestGitHubGatedStatesItsAssumptions` guards its stated assumptions (`gh`/`jq`/`--no-run`/drift/author/
      bypass). Keep the note that `TestExamplesLoadAndValidate` now covers four files.

- [ ] **Step 8: Gate + commit.**

```bash
make check > /tmp/t76-check.log 2>&1; echo "exit=$?"; tail -3 /tmp/t76-check.log
git add templates/github-gated.yaml templates/templates_test.go templates/README.md templates/CLAUDE.md
git commit -m "t76: github-gated flagship init template"
```

Note: `make check` runs the mtt-flow event hooks are NOT involved here (this is a plain `git commit`, not an
`mtt` mutation). If gofmt trips on struct alignment in the test, run `make fmt` and re-stage.

---

## Task 2: Out-of-tree dogfood pass (validate the pattern end-to-end; decide on `mtt tt`)

Runs in a **dedicated out-of-tree store** (`MTT_DIR` under the scratchpad) pointed at a **scratch GitHub
repo** — NEVER the live `.mtt/` (its event hooks auto-commit/push `main`; spec §"Dogfood plan"). The artifact
is the validated template + a written friction log; no repo state is touched.

**Files:**
- Create (transient, out-of-tree): a scratch `MTT_DIR` + a scratch GitHub repo — deleted at the end.
- Produce: the friction log + `mtt tt` decision, folded into Task 3's docs (FLOW_GUIDE section) and the PR body.

- [ ] **Step 1: Stand up the scratch env.**

```bash
SCRATCH=/tmp/claude-1000/-home-gss-projects-mtt/e26272f8-a2db-4f66-a2d6-5e5b62ea6f4d/scratchpad/t76-dogfood
rm -rf "$SCRATCH"; mkdir -p "$SCRATCH"
REPO="pashukhin/mtt-t76-dogfood"            # scratch; delete at the end
gh repo create "$REPO" --private --clone --add-readme
cd "$SCRATCH" && git clone "https://github.com/$REPO" work && cd work
export MTT_DIR="$PWD/.mtt-store"            # out-of-tree store, NOT the live .mtt/
mkdir -p "$MTT_DIR"
/home/gss/projects/mtt/bin/mtt init --dir "$MTT_DIR" >/dev/null
cp /home/gss/projects/mtt/templates/github-gated.yaml "$MTT_DIR/config.yaml"
# author for require-free store is optional; set one to also exercise the attribution note:
printf 'author: dogfood-bot\n' >> "$MTT_DIR/config.local.yaml" 2>/dev/null || printf 'author: dogfood-bot\n' > "$MTT_DIR/config.local.yaml"
```

- [ ] **Step 2: Drive the happy path — create issue, work, review, gated close.**

```bash
M() { /home/gss/projects/mtt/bin/mtt --dir "$MTT_DIR" "$@"; }
M add 'Wire the widget'                       # t1
M start t1 -v                                 # post: gh issue create + ref add url:
M ref list t1                                  # EXPECT: one url: ref = the new issue
M sync t1 -v                                   # EXPECT: passes (issue OPEN, task not done)
M submit t1                                     # in_progress -> review
M approve t1 -v                                 # gate: issue OPEN -> post: gh issue close --reason completed
gh issue view "$(M ref list t1 --json | jq -r --arg k url '.refs[] | select(.kind==$k) | .id')" --json state --jq .state
# EXPECT: CLOSED
```

- [ ] **Step 3: Prove the gate BITES (the load-bearing claim) and drift is detected.**

```bash
M add 'Second widget'                          # t2
M start t2 -v
ISS=$(M ref list t2 --json | jq -r --arg k url '.refs[] | select(.kind==$k) | .id')
# (a) close the issue directly in GitHub (bypass) -> the sync poll must COMPLAIN:
gh issue close "$ISS" --reason "not planned"
M sync t2 -v; echo "sync exit=$?"             # EXPECT: nonzero + "DRIFT: ... is CLOSED"
# (b) the approve gate refuses to mtt-close an already-closed issue:
M submit t2; M approve t2 -v; echo "approve exit=$?"   # EXPECT: nonzero + "already CLOSED (closed out of band?)"
```

- [ ] **Step 4: Record the friction log** (the acceptance deliverable). At minimum, confirm/observe:
  - the **refs-not-in-`MTT_TASK_JSON`** gap → every issue-URL read is an `mtt ref list --json` self-call
    (t40 removed the *title/children* re-query but not the *ref* re-query; candidate follow-up: add refs to
    the env, or a `--arg` channel per t77);
  - the **attribution clunk** under `require.who` (the store above is require-free by default; note the
    exit-2 path an adopter hits and the author-first fix);
  - whether `gh issue create` emits only the URL on stdout (adjust the capture if it prints extra lines);
  - the **`mtt tt` decision:** the config-only pattern is ergonomic enough with t40 → **do NOT build
    `mtt tt`** (record this explicitly, with the one caveat above as the sole residual friction), OR if a
    step proved genuinely clunky, name the specific sugar it would justify. Default: don't build.

- [ ] **Step 5: Tear down the scratch repo/store.**

```bash
cd /home/gss/projects/mtt
gh repo delete "$REPO" --yes
rm -rf "$SCRATCH"
unset MTT_DIR
```

No commit in this task — the artifact is the friction log, consumed by Task 3.

---

## Task 3: Docs — enforcement honesty, t40 enabler, known interactions (bilingual) + CHANGELOG + t9 check

**Files (each with its RU mirror kept in sync):**
- Modify: `README.md` / `README.ru.md` — one bullet naming the `github-gated` flagship.
- Modify: `CLI_REFERENCE.md` / `CLI_REFERENCE.ru.md` — the `mtt init --template` list (currently
  "`coding`, `hierarchy`, and the `git-flow` flagship") → add `github-gated`.
- Modify: `DESIGN.md` / `DESIGN.ru.md` — a "Shipped (t76)" block under the templates section (§ around the
  t62 flagship block) stating the two-linked-entities model + enforcement honesty + t40 enabler.
- Modify: `FLOW_GUIDE.md` / `FLOW_GUIDE.ru.md` — a short subsection under §8 (samples) describing the
  github-gated pattern, the enforcement-honesty contract, the polled+symmetric drift limit, and the two known
  interactions with the author-first fix.
- Modify: `CHANGELOG.md` — `[Unreleased] → Added` entry.
- Verify (no edit expected): `.mtt/tasks/t9.yaml` still names "GitHub Issues as a true TaskStore
  (store-of-record)" + the composition-root selection-seam prerequisite (spec acceptance item 4).

- [ ] **Step 1: Grep every parallel occurrence** so no EN/RU/Shipped copy is missed:

Run: `grep -rn "git-flow\|template\|шаблон\|coding\.yaml\|hierarchy" README*.md DESIGN*.md CLI_REFERENCE*.md FLOW_GUIDE*.md`
Then edit each hit that enumerates the samples to include `github-gated`.

- [ ] **Step 2: CLI_REFERENCE (EN+RU)** — in the `--template` bullet, change the sample enumeration to
      "`coding`, `hierarchy`, the `git-flow` flagship, and the `github-gated` flagship (`gh`+`jq`; gates a
      GitHub issue's close on an mtt task's flow)" — EN in `CLI_REFERENCE.md:223`, RU in
      `CLI_REFERENCE.ru.md:224` (re-confirm line numbers at edit time).

- [ ] **Step 3: FLOW_GUIDE (EN+RU) §8** — add a `github-gated` paragraph. Required content (no placeholders):
      the two-linked-entities model; the terminal-close-as-gated-action; the **enforcement-honesty** sentence
      (gates `mtt`-mediated moves; a direct UI/`gh` move by anyone bypasses; drift is detect+complain, polled
      + symmetric, never enforced); **t40 as the ergonomic enabler** (title/children ride the env, no
      re-query) with the honest residual (refs are NOT in `MTT_TASK_JSON`, so the issue-URL read stays an
      `mtt ref list --json` self-call); and the **two known interactions** (attribution: set `author` first;
      mutate-in-post re-entrancy with auto-commit events).

- [ ] **Step 4: DESIGN (EN+RU)** — add a `> **Shipped (t76): github-gated flagship template.**` block near the
      t62/t40 shipped blocks, summarizing the decision trail in 3–4 sentences (validate-then-build; two linked
      entities; store-of-record deferred to t9; near-zero Go).

- [ ] **Step 5: README (EN+RU)** — one bullet: the `github-gated` flagship exists and what it does
      (mirror the existing `hierarchy`/git-flow phrasing style).

- [ ] **Step 6: CHANGELOG `[Unreleased] → Added`:**

```markdown
- **`github-gated` flagship init template (t76).** Puts executable mtt gates on a GitHub issue's close using
  only existing primitives (`mtt` self-calls + t40 task-context env + `gh`) — no new Go. Two linked entities
  (a native GitHub issue + a rich gating mtt task); the issue's close is the task's terminal action; drift
  (a direct UI/`gh` move) is detect+complain (polled, symmetric), never enforced. Header + docs pre-note the
  two known interactions (require.who attribution; mutate-in-`post:` re-entrancy). True GitHub-as-store stays
  deferred (t9).
```

- [ ] **Step 7: Confirm the t9 amendment** (acceptance item 4):

Run: `git show origin/main:.mtt/tasks/t9.yaml | grep -i "store-of-record\|TaskStore\|selection\|composition"`
Expected: matches — no edit needed. If it does not, fix t9's description (via `mtt edit`) before delivery.

- [ ] **Step 8: Gate + commit the docs.**

```bash
make check > /tmp/t76-check.log 2>&1; echo "exit=$?"; tail -3 /tmp/t76-check.log
git add README*.md DESIGN*.md CLI_REFERENCE*.md FLOW_GUIDE*.md CHANGELOG.md
git commit -m "t76: docs — github-gated template, enforcement honesty, t40 enabler (EN+RU)"
```

---

## Task 4: Review gate → PR body → approve

- [ ] **Step 1: Final `make check` (whole tree).**

Run: `make check > /tmp/t76-check.log 2>&1; echo "exit=$?"; tail -5 /tmp/t76-check.log`
Expected: `exit=0`.

- [ ] **Step 2: Write the PR body** to `docs/superpowers/pr/t76.md` (title will be prefixed `t76: ` by
      approve). Summarize: the template, the dogfood evidence (gate bites; drift detected), the `mtt tt`
      decision, the enforcement-honesty contract, and the t9 deferral. Commit it with the docs or in its own
      commit BEFORE `mtt submit`.

- [ ] **Step 3: ONE adversarial impl_review** — dispatch a fresh empirical subagent: materialize nothing new,
      re-run `mtt init --template templates/github-gated.yaml` in a throwaway `MTT_DIR`, drive the gate, and
      attack the command strings (quoting, drift edges, the two known interactions, YAML validity, the
      stated-assumptions test). **Verify every finding against the binary/code; fix the real ones; ignore the
      spurious.**

- [ ] **Step 4:** `mtt submit` → agent review → `mtt approve` (impl) → human merges the PR → `mtt deliver`
      (after `git switch main && git pull --ff-only`). These flow edges are driven with mtt, not by hand.

---

## Self-Review (run against the spec — done at plan-write time)

- **Acceptance item 1** (working template committed, demonstrably gating): Task 1 (template + tests) + Task 2
  Steps 2–3 (happy path + gate-bites/drift). ✅
- **Acceptance item 2** (≥1 e2e dogfood + friction + `mtt tt` decision): Task 2 Steps 2–4. ✅
- **Acceptance item 3** (docs: enforcement honesty + t40 enabler + two interactions): Task 3 Steps 3–5 +
  template header (Task 1 Step 4). ✅
- **Acceptance item 4** (`make check` green; CHANGELOG; t9 amendment confirmed): Task 3 Steps 6–8 + Task 4
  Step 1. ✅
- **Spec §"Enforcement honesty"** (polled + symmetric + bypass): FLOW_GUIDE (Task 3 Step 3) + template header.
  Residual honesty limit recorded: reopened-while-done drift has no flow edge on the terminal `done` state
  (caught only by a manual `gh` compare) — state this in FLOW_GUIDE.
- **No placeholders:** template YAML, test code, dogfood commands, and CHANGELOG entry are literal above.
- **Type consistency:** the jq extractor `--arg k url '.refs[] | select(.kind==$k) | .id'` and the edge names
  (`start/submit/approve/decline/sync/cancel`) are used identically in the template, the dogfood, and the
  assumptions test.
