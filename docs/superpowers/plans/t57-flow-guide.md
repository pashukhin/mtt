# FLOW_GUIDE (flow-authoring guide) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a top-level bilingual `FLOW_GUIDE.md`/`FLOW_GUIDE.ru.md` teaching how to author your **own**
flow on mtt's executable-transition engine, with our dogfood flow as a labelled adaptable sample.

**Architecture:** Docs-only. The engine is already shipped and unchanged; this task produces prose anchored on
live, tested artifacts (the committed `.mtt/config.yaml`, the `demo/` walkthrough) and validates every
hand-authored config snippet through `mtt types` (which runs `Config.Validate`) so no invalid flow ships.
Runnable universal `mtt init` templates are split to `t62`; the git-flagship template is deferred to `t24`.

**Tech Stack:** Markdown (EN source of truth + RU mirror per AGENTS.md); the `mtt` CLI + a scratch `.mtt/` for
snippet validation; `make check` for the gate.

**Spec:** `docs/superpowers/specs/t57-flow-guide.md`.

## Global Constraints

Every task inherits these. Values are copied verbatim from the spec / verified against the code + committed
`.mtt/config.yaml` — do **not** paraphrase a mechanism; if a claim isn't here, look it up in DESIGN.md's
"Shipped" blocks or CLI_REFERENCE before writing it.

- **Docs-only diff.** New: `FLOW_GUIDE.md`, `FLOW_GUIDE.ru.md`. Modified (docs): `README.md`, `README.ru.md`,
  `AGENTS.md`, `CLAUDE.md`, and optionally `CLI_REFERENCE.md`/`.ru`. **No** Go code, **no** new `init`
  template, **no** engine change.
- **Bilingual rule.** EN is the source of truth; the RU file mirrors it section-for-section; **config snippets
  are byte-identical across EN/RU** (only prose is translated).
- **Every config snippet must pass `Config.Validate`.** A flow needs **≥1 `initial`, ≥1 `active`, ≥1
  `terminal`** status (a 2-status flow is rejected); `kind` is derived from topology (initial = no incoming;
  terminal = no outgoing); type/status names unique; every transition `from`/`to` resolves; **≤1** `default`
  type and **≤1** `default` status (a default status must be `initial`).
- **Exit-code taxonomy** (state these correctly): gate blocked = **3**; `post:` failure keeps the move = **5**;
  missing attribution = **2**; not found = **4**; invalid transition = **6**; dangling refs = **7**.
- **Placeholders:** only `{{.ID}}` `{{.Type}}` `{{.From}}` `{{.To}}` expand (in `commands`, `post`, `rollback`,
  and shown descriptions). Free text like `{{.Title}}` is a template error, never interpolated.
- **Timeouts:** the global `command_timeout` is an adapter setting in `config.yaml`, **default `5m`**,
  overridable in `config.local.yaml`; a per-command `timeout:` overrides it. The dogfood `make check` gate uses
  `{run: 'make check', timeout: 10m}` because the 5m default would SIGKILL it.
- **Attribution:** `by` resolves `--who`/`--by` > `MTT_BY` > `author:` in the gitignored `config.local.yaml`.
  `require: {who, why}` is tighten-only (global + per-edge). The committed config sets `require: {who: true}`,
  so a copy-adopter who sets none of the three hits **exit 2** on the first move.
- **The real dogfood `start` edge** (quote it exactly, it is idempotent — **no rollback**):
  `git switch task/{{.ID}} || (git switch main && git switch -c task/{{.ID}})`.
- **`rollback:` is real but the dogfood config does not use it.** Teach it from the DESIGN illustration:
  `commands: [{run: 'git checkout -b task/{{.ID}}', rollback: 'git branch -D task/{{.ID}}'}]`, labelled as an
  illustration.
- **Dogfood `post:` duplication** (use these numbers): the auto-commit block appears **24×**, the main-push
  block **14×**, across **12** `cancel` edges — the reason the flagship template waits for `t24`.
- **Trust boundary:** a flow config runs **arbitrary shell with the user's privileges** — commands are trusted
  project config (like a Makefile), never network input; ids are charset-validated at load. Only run/commit a
  config you trust.

## File structure

- `FLOW_GUIDE.md` — the guide, EN, source of truth. One document, 12 sections (see Task 1).
- `FLOW_GUIDE.ru.md` — the RU mirror (same structure + byte-identical snippets).
- `README.md` / `README.ru.md` — add one discoverability pointer line each.
- `AGENTS.md` — register the new bilingual pair in "Documentation language".
- `CLAUDE.md` — register the new bilingual pair in "Docs language".
- `CLI_REFERENCE.md` / `.ru` — optional one-line "see FLOW_GUIDE" note near the `types`/flow description.

## Validated config snippets (author these verbatim in the guide; DO NOT invent variants)

These four are pre-validated shapes. Task 1 embeds them and re-validates each via `mtt types`.

**S1 — minimal valid flow (§2, and the "generic script-gate" of §7):**
```yaml
version: 1
project: {name: demo}
types:
  - name: item
    prefix: i
    parents: []
    default: true
    statuses:
      - {name: todo,  kind: initial}
      - {name: doing, kind: active}
      - {name: done,  kind: terminal}
    transitions:
      - {from: todo,  to: doing}
      - {from: doing, to: done, commands: ["./gate.sh"]}
```

**S2 — content-review flow (§7):**
```yaml
version: 1
project: {name: blog}
types:
  - name: post
    prefix: p
    parents: []
    default: true
    statuses:
      - {name: draft,     kind: initial}
      - {name: review,    kind: active}
      - {name: published, kind: terminal}
      - {name: rejected,  kind: terminal}
    transitions:
      - {from: draft,  to: review,    name: submit,  commands: ["./review-checks.sh"]}
      - {from: review, to: published, name: publish, post: ["./publish.sh"]}
      - {from: review, to: rejected,  name: reject}
```

**S3 — approval / sign-off flow (§7):**
```yaml
version: 1
project: {name: procurement}
types:
  - name: request
    prefix: r
    parents: []
    default: true
    statuses:
      - {name: submitted, kind: initial}
      - {name: review,    kind: active}
      - {name: approved,  kind: terminal}
      - {name: denied,    kind: terminal}
    transitions:
      - {from: submitted, to: review}
      - {from: review, to: approved, name: approve, commands: ["./validate-request.sh"]}
      - {from: review, to: denied,   name: deny}
```

**S4 — the `rollback:` illustration (§5; NOT the dogfood config):**
```yaml
      - from: tbd
        to: in_progress
        commands:
          - {run: 'git checkout -b task/{{.ID}}', rollback: 'git branch -D task/{{.ID}}'}
```

---

### Task 1: Write `FLOW_GUIDE.md` (EN, source of truth)

**Files:**
- Create: `FLOW_GUIDE.md`
- Reference (read, quote): `.mtt/config.yaml`, `DESIGN.md`, `CLI_REFERENCE.md`, `demo/`

**Interfaces:**
- Consumes: the four validated snippets S1–S4 (above) and the Global Constraints facts.
- Produces: `FLOW_GUIDE.md` with 12 sections (the section list Task 2 mirrors and Task 3 links to).

- [ ] **Step 1: Create the skeleton with the 12 section headers.**

Write `FLOW_GUIDE.md` starting with a one-paragraph intro (the domain-neutral spine: "a flow binds the
transitions of your config-defined entities to external actions/artifacts; running a test suite is one
instance; a flow is per-project data — this guide teaches the engine, our own flow is a labelled sample") and
these headers in order:
`## 1. Mental model` · `## 2. A minimal flow from scratch` · `## 3. The graph` · `## 4. Gates: executable
transitions` · `## 5. Finalization: post: actions` · `## 6. Attribution & guards` · `## 7. Flows for any
domain` · `## 8. The git-integration pattern (a sample)` · `## 9. Adaptation checklist` · `## 10. Exercise your
flow locally` · `## 11. Validate your flow` · `## 12. Neighbours`.

- [ ] **Step 2: Draft §1–§3 (mental model, minimal flow, the graph).**

§1: the spine + the "product headline (agent gate) is the go-to-market emphasis, but the engine is
domain-neutral" nuance + point at `mtt init` templates and `mtt types`. §2: embed **S1 verbatim** and explain
the ≥1-of-each-kind rule (a 2-status flow is rejected). §3: types (root vs `parents:` hierarchy), statuses +
`kind` (topology-derived), transitions, named edges → `mtt <edge>`/`mtt do`, `default` type/status.

- [ ] **Step 3: Draft §4–§6 (gates, post/rollback, attribution).**

§4: `commands:` (all exit 0 or blocked → exit 3), the **global `command_timeout` (5m default, override in
`config.local.yaml`)** + per-command `timeout:` (mention the dogfood `make check` `timeout: 10m`), placeholders
(the 4 only), and the **trust caution** (arbitrary shell, your privileges). Show a gate as `./gate.sh` first,
then `make lint`/`make test` as *one* domain's instance. §5: `post:` runs after persist; two-phase failure
(gate → exit 3 nothing persisted; post → exit 5 move kept + remaining commands printed); teach `rollback:` from
**S4**, labelled "illustration — our dogfood config does not use rollback (see §8)." §6: `require:{who,why}`
(global + per-edge, tighten-only) and the **`by` resolution chain** (`--who`/`--by` > `MTT_BY` >
`config.local.yaml author`); state that the shipped config sets `require:{who}` so an adopter must set one or
hit exit 2; the `current` pointer (`Transition.Current: set|clear`).

- [ ] **Step 4: Draft §7 (flows for any domain) with S2 and S3.**

Embed **S2 verbatim** (content-review: draft→review→published, gate = a review script, `post:` = publish) and
**S3 verbatim** (approval: submitted→review→approved/denied, gate = validate a request file). Note the generic
script-gate is just S1 ("the gate can be any command"). End with: "runnable versions of these ship in `t62`
(`mtt init --template …`)."

- [ ] **Step 5: Draft §8–§9 (the git worked example + adaptation checklist).**

§8: walk this repo's **real** committed config. Quote the idempotent start edge exactly
(`git switch task/{{.ID}} || (git switch main && git switch -c task/{{.ID}})` — **no rollback**), the
auto-commit `.mtt` post, `approve` → `git push` + `gh pr create` (idempotent), `deliver`/`cancel` → push `main`
with a narrowed pathspec. Caveats: assumes GitHub + `gh` + `jq` + the `task/<id>` model + direct push to
`main`; how to swap for GitLab MR / no-PR / trunk-based / a non-dev lifecycle. Note the `post:` duplication
(24×/14×/12) and that `t24` will DRY it, `t62` will ship the clean template. §9: the copy-then-change checklist
— gate command, artifact paths (`docs/superpowers/pr/<id>.md`, spec/plan paths), `gh`+`jq` deps, branch name,
`.gitignore` for `config.local.yaml`, and attribution (`author:` in `config.local.yaml`).

- [ ] **Step 6: Draft §10–§12 (run loop, validate, neighbours).**

§10: `mtt init` a scratch project → `mtt add` → `mtt <status>`/`mtt do` a move, watch the gate (`▶`/`✓`/`✗`);
`--no-run` to iterate past a side-effecting gate; `-v`/`--log-file` to debug; `mtt types` to inspect. §11:
`Config.Validate` runs on `mtt add`/`mtt types`; list the structural invariants (from Global Constraints) so
the reader knows what is rejected; note that reference integrity (`mtt check`, `t58`) is a **separate** concern.
§12: cross-links — runnable templates (`t62`), agent-usage docs (`t46`), settings/hooks scaffold (`t52`).

- [ ] **Step 7: Validate every embedded config snippet (the "test").**

For each of S1, S2, S3 (S4 is a fragment — skip): write it to a scratch config and run `mtt types`.

Run (example for S1, repeat for S2/S3 with their own file):
```bash
SC=/tmp/claude-1000/-home-gss-projects-mtt/b8e7f29a-4972-498f-a2f2-8f7a0a17a682/scratchpad/flowtest
mkdir -p "$SC/.mtt" && cp <snippet>.yaml "$SC/.mtt/config.yaml"
./bin/mtt --dir "$SC" types
```
Expected: the flow renders (types/statuses/transitions/edge verbs printed) with **no** validation error. If any
snippet errors, fix the snippet in the guide until `mtt types` is clean, then re-run.

- [ ] **Step 8: Cross-check the §8 git-example against the live config.**

Run: `grep -n 'git switch task\|checkout -b\|rollback\|git push origin main\|gh pr create' .mtt/config.yaml`
Expected: the start edge matches the idempotent `git switch …` quoted in §8; **zero** `rollback`/`checkout -b`
in the config (confirming §8's "no rollback" claim). Fix §8 if any quoted line drifts from the config.

- [ ] **Step 9: Commit.**

```bash
git add FLOW_GUIDE.md
git commit -m "t57: FLOW_GUIDE.md — bilingual flow-authoring guide (EN)"
```

---

### Task 2: Write `FLOW_GUIDE.ru.md` (RU mirror)

**Files:**
- Create: `FLOW_GUIDE.ru.md`
- Reference: `FLOW_GUIDE.md` (the just-written EN source)

**Interfaces:**
- Consumes: the finished `FLOW_GUIDE.md` (same 12 sections, snippets S1–S4).
- Produces: `FLOW_GUIDE.ru.md` in structural + snippet parity with EN.

- [ ] **Step 1: Mirror the structure.** Create `FLOW_GUIDE.ru.md` with the **same 12 section headers** (Russian
  prose, same order), a top note "Русская версия FLOW_GUIDE.md; English is the source of truth, keep in sync"
  (mirror the DESIGN.ru.md header convention).

- [ ] **Step 2: Translate the prose** section-by-section, keeping every **config snippet byte-identical** to
  EN (S1–S4 are code, not translated). Keep command names, exit codes, placeholders, and file paths identical.

- [ ] **Step 3: Verify structural parity (the "test").**

Run: `grep -c '^## ' FLOW_GUIDE.md FLOW_GUIDE.ru.md`
Expected: identical header counts (12 each). Then eyeball that S1/S2/S3 blocks are byte-identical (e.g.
`grep -A15 'name: post' FLOW_GUIDE.md` vs `FLOW_GUIDE.ru.md` match).

- [ ] **Step 4: Commit.**

```bash
git add FLOW_GUIDE.ru.md
git commit -m "t57: FLOW_GUIDE.ru.md — RU mirror"
```

---

### Task 3: Discoverability + register the bilingual pair

**Files:**
- Modify: `README.md`, `README.ru.md` (one pointer line each)
- Modify: `AGENTS.md` (Documentation language list), `CLAUDE.md` (Docs language line)
- Modify (optional): `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md` (a "see FLOW_GUIDE" note)

**Interfaces:**
- Consumes: `FLOW_GUIDE.md`/`.ru.md` (must exist).
- Produces: the guide is discoverable and the language rule lists the new pair.

- [ ] **Step 1: Add the README pointer.** In `README.md` (and translate in `README.ru.md`), near the
  flow/feature description, add one line, e.g.: `Authoring your own flow? See [FLOW_GUIDE.md](FLOW_GUIDE.md).`
  No content duplication.

- [ ] **Step 2: Register the pair in AGENTS.md.** In the "Documentation language" section, the "Bilingual docs"
  line, append the new pair so it reads:
  `… \`DESIGN.md\` ↔ \`DESIGN.ru.md\`, \`CLI_REFERENCE.md\` ↔ \`CLI_REFERENCE.ru.md\`, \`FLOW_GUIDE.md\` ↔ \`FLOW_GUIDE.ru.md\`.`

- [ ] **Step 3: Register the pair in CLAUDE.md.** In the "Docs language" section, the bilingual list, append
  `\`FLOW_GUIDE.md\` ↔ \`FLOW_GUIDE.ru.md\`` in the same style (keep AGENTS.md and CLAUDE.md consistent — the
  rule is stated in both).

- [ ] **Step 4 (optional): CLI_REFERENCE note.** Where `mtt types` / the flow is described, add
  `See [FLOW_GUIDE.md](FLOW_GUIDE.md) to author a flow.` in `CLI_REFERENCE.md` and its RU mirror. Skip if it
  doesn't fit cleanly.

- [ ] **Step 5: Run the gate (the "test").**

Run: `make check`
Expected: `OK: make check passed` (EXIT 0). Docs changes don't touch Go, but this confirms nothing regressed
(e.g. `TestRepoDogfoodConfig` still green, no broken build).

- [ ] **Step 6: Commit.**

```bash
git add README.md README.ru.md AGENTS.md CLAUDE.md CLI_REFERENCE.md CLI_REFERENCE.ru.md
git commit -m "t57: link + register FLOW_GUIDE in the docs-language rule"
```

---

## Self-Review (author's checklist, run after writing the plan)

**Spec coverage:** every spec section maps to a task — framing/spine → Task 1 §1; the 12-section outline → Task
1 §2–§6 (steps 2–6); the 4 decisions (guide-first/t24, tight scope, bilingual, t62 split) → Task 1 (content) +
Task 2 (RU) + §12 links + §7/§8 forward-links; deliverable (guide + pointers + language-rule registration) →
Tasks 2–3; verification (snippet validation + git cross-check) → Task 1 steps 7–8 + Task 2 step 3; out-of-scope
(no engine change, no template) honored (docs-only). No gap.

**Placeholder scan:** no "TBD/TODO/handle edge cases"; the config snippets are provided verbatim (S1–S4); the
language-rule edits are shown verbatim; validation commands are exact with expected output.

**Consistency:** section numbering 1–12 is identical in Task 1 (write), Task 2 (mirror), and Task 3 (link);
snippet ids S1–S4 are referenced consistently; exit codes and timeout values match the Global Constraints
throughout.
