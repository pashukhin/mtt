# t27 — Mechanize `gh pr create` on approve — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The `approve → approved` edge (both `task` and `chore`) idempotently opens/updates the branch's PR, so the only remaining human step is merging.

**Architecture:** Config-only. A third `post:` action on each approve edge runs after the existing commit + branch-push: it opens the PR iff no open PR exists, reading the title via read-only `mtt show --json | jq` and the body from an optional `docs/superpowers/pr/{{.ID}}.md` artifact (`--body-file`) with a generated fallback. No `core`/`pkg/mtt`/adapter/CLI Go changes; the only `.go` change is the config-shape guard `TestRepoDogfoodConfig`.

**Tech Stack:** Go 1.x; YAML flow config (`.mtt/config.yaml`); `gh` CLI + `jq` (new runtime deps of the post); `testscript`/std `testing`.

## Global Constraints

- **No production Go.** Only `.mtt/config.yaml`, `internal/adapter/yaml/dogfood_test.go` (test), docs, and a PR-body artifact change.
- **Byte-exact guard.** The new `cmdPrCreate` Go constant MUST byte-match the config `post:` scalar exactly — including the em-dash `—` and every double-quoted jq expression — or the guard reddens. Author both from the one canonical string below.
- **Single-quoted YAML scalar.** The post command contains **no** `'` (all jq/strings double-quoted) and **no** backticks, so it fits a single-quoted YAML scalar like the existing posts.
- **`make check` green before every commit.** TDD: red → green → refactor.
- **The `approved` status description MUST keep the literal substring `gh pr create`** (guarded by `TestRepoDogfoodConfig`, D7).

### THE CANONICAL COMMAND (copy verbatim into both the config and the Go constant)

```
[ -n "$(gh pr list --head task/{{.ID}} --state open --json number --jq ".[].number")" ] || { t="{{.ID}}: $(mtt show {{.ID}} --json | jq -r ".title // empty")"; if test -f docs/superpowers/pr/{{.ID}}.md; then gh pr create --base main --head task/{{.ID}} --title "$t" --body-file docs/superpowers/pr/{{.ID}}.md; else gh pr create --base main --head task/{{.ID}} --title "$t" --body "Automated PR for {{.ID}} — see: mtt show {{.ID}}"; fi; }
```

(Note the em-dash `—` in the fallback `--body`. Validated: `sh -n` syntax-OK; `.[].number` gives empty on `[]` → create, the number on an open PR → skip; injection-safe via `--title "$t"`.)

---

### Task 1: Mechanized PR-create post on both approve edges + reworded descriptions (guarded)

**Files:**
- Modify: `internal/adapter/yaml/dogfood_test.go` (add `cmdPrCreate`; approve case expects `post` len 3)
- Modify: `.mtt/config.yaml` (both approve edges → block style with 3rd post entry; reword approve-edge + `approved`-status descriptions, both types)

**Interfaces:**
- Consumes: existing `cmdPostCommit`, `cmdPushBranch` constants and the post-shape loop in `TestRepoDogfoodConfig`.
- Produces: a new package-level test constant `cmdPrCreate` (byte-matches the canonical command).

- [ ] **Step 1: Write the failing guard (RED)**

In `internal/adapter/yaml/dogfood_test.go`, add the constant after `cmdPushMain` (in the `const (...)` block):

```go
	// approve also opens/updates the PR (idempotent, config-only) — c1 pushed the
	// branch, t27 opens the PR. Byte-matches the .mtt/config.yaml approve post[2].
	cmdPrCreate = `[ -n "$(gh pr list --head task/{{.ID}} --state open --json number --jq ".[].number")" ] || { t="{{.ID}}: $(mtt show {{.ID}} --json | jq -r ".title // empty")"; if test -f docs/superpowers/pr/{{.ID}}.md; then gh pr create --base main --head task/{{.ID}} --title "$t" --body-file docs/superpowers/pr/{{.ID}}.md; else gh pr create --base main --head task/{{.ID}} --title "$t" --body "Automated PR for {{.ID}} — see: mtt show {{.ID}}"; fi; }`
```

Then change the approve case in the post-shape loop (currently expects len 2):

```go
			case tr.To == "approved": // approve: push the branch for the PR, then open the PR (t27)
				if len(tr.Post) != 3 || tr.Post[1].Run != cmdPushBranch || tr.Post[2].Run != cmdPrCreate {
					t.Fatalf("%s %s->approved post = %+v, want [commit, %q, %q]", ty.Name, tr.From, tr.Post, cmdPushBranch, cmdPrCreate)
				}
```

- [ ] **Step 2: Run the guard to verify it fails**

Run: `go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig`
Expected: FAIL — `task impl_review->approved post = [...len 2...], want [commit, "git push -u origin task/{{.ID}}", "[ -n ...]"]`.

- [ ] **Step 3: Add the 3rd post entry + reword descriptions in the config (GREEN)**

In `.mtt/config.yaml`, replace the **task** approve edge (the single flow-style line `- {from: impl_review, to: approved, name: approve, …}`) with block style:

```yaml
      - from: impl_review
        to: approved
        name: approve
        description: "code review passed; the PR is opened/updated automatically — hand to the human to merge"
        post:
          - 'git add .mtt && git commit -m "{{.ID}}: {{.From}} → {{.To}}" -- .mtt'
          - 'git push -u origin task/{{.ID}}'
          - '[ -n "$(gh pr list --head task/{{.ID}} --state open --json number --jq ".[].number")" ] || { t="{{.ID}}: $(mtt show {{.ID}} --json | jq -r ".title // empty")"; if test -f docs/superpowers/pr/{{.ID}}.md; then gh pr create --base main --head task/{{.ID}} --title "$t" --body-file docs/superpowers/pr/{{.ID}}.md; else gh pr create --base main --head task/{{.ID}} --title "$t" --body "Automated PR for {{.ID}} — see: mtt show {{.ID}}"; fi; }'
```

Make the **identical** replacement for the **chore** approve edge (the second occurrence).

Then reword both `approved` **status** descriptions (the `{name: approved, kind: active, description: "…"}` lines, task and chore) to — keeping the `gh pr create` substring:

```yaml
      - {name: approved, kind: active, description: "the PR is opened/updated automatically (mtt runs gh pr create for you; the branch was auto-pushed) — ask the human to merge; after the squash-merge run `mtt deliver`; human-requested changes -> `mtt decline`"}
```

- [ ] **Step 4: Run the guard to verify it passes (GREEN)**

Run: `go test ./internal/adapter/yaml/ -run TestRepoDogfoodConfig`
Expected: PASS.

- [ ] **Step 5: Full gate**

Run: `make check`
Expected: `OK: make check passed`. (If gofmt/vet/lint flag the test edit, fix and re-run.)

- [ ] **Step 6: Eyeball the rendered flow**

Run: `mtt types | grep -A1 "approve.*approved"` (or `mtt show t27`)
Expected: the approve edge now shows three `⇢` post lines ending in the `gh pr create` command, and the `approved` status description reads "opened/updated automatically".

- [ ] **Step 7: Commit**

```bash
git add .mtt/config.yaml internal/adapter/yaml/dogfood_test.go
git commit -m "t27: mechanize gh pr create on approve (config post + guard)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Docs sync (EN/RU) — approve mechanizes the PR

**Files:**
- Modify: `AGENTS.md` (the "Moves auto-commit … and auto-push (c1)" bullet — TWO sentences)
- Modify: `DESIGN.md` + `DESIGN.ru.md` (**TWO** live occurrences each: the t21/c1 "Shipped" block **and** the s009 dogfood "Shipped" block, plus the artifact-dir convention line)
- Modify: `CLI_REFERENCE.md` + `CLI_REFERENCE.ru.md` (the `post:` paragraph — reflow, no double "and")

**Interfaces:** none (prose only). EN↔RU must stay in sync.

> **Why two DESIGN occurrences:** `gh pr create` is described as a remaining-manual step in BOTH the t21/c1
> block *and* the s009 dogfood block. These "Shipped (sNNN)" blocks are **living text** (they already carry
> retroactive "(Since t21 … c1 …)" parentheticals), so both must move or DESIGN self-contradicts (this is the
> exact miss the plan review caught; c5 had the same one). Verify with the grep in Step 7.

- [ ] **Step 1: AGENTS.md** (two edits in the auto-commit/auto-push bullet).
  (a) Reword the tail "So the only manual git step left is opening the PR (`gh pr create` — a judgement call)." →
  "**`approve` also opens/updates the PR** (`gh pr create`, idempotent — skipped if an open PR exists; title from
  `mtt show --json`, body from `docs/superpowers/pr/<id>.md` when present else generated; needs `gh`+`jq`). So the
  only manual step left is **merging** the PR."
  (b) Broaden the next sentence's failure enumeration "If a post action fails (**commit *or* push**), the move is
  **kept** …" → "If a post action fails (**commit, push, or PR-open**), the move is **kept** …".

- [ ] **Step 2: DESIGN.md — occurrence A (t21/c1 block, ~L454).** Change "the remaining manual steps are `gh pr
  create` (title/body are a judgement call) and pulling main before `deliver`" → "`approve` also opens/updates the
  PR (`gh pr create`, idempotent; body from `docs/superpowers/pr/<id>.md` or a fallback — t27), so the remaining
  manual steps are **merging** and pulling main before `deliver`."

- [ ] **Step 3: DESIGN.md — occurrence B (s009 dogfood block, ~L861-867).** Three edits:
  (a) L863-864 "the remaining manual steps are opening the PR (`gh pr create`) and pulling main before `deliver`" →
  "the remaining manual steps are **merging** and pulling main before `deliver`".
  (b) L861-862 artifact-dir line "id-keyed names `docs/superpowers/specs|plans/<id>-<slug>.md`" → append the PR-body
  convention: "id-keyed names `docs/superpowers/specs|plans/<id>-<slug>.md` (+ an optional
  `docs/superpowers/pr/<id>.md` PR body — t27)".
  (c) L865-867 parenthetical "(Since t21 … and since c1 `approve` auto-pushes the task branch and `deliver`/`cancel`
  auto-push main (c5) — …)" → add t27: "…and since **t27** `approve` opens/updates the PR — …".

- [ ] **Step 4: DESIGN.ru.md — mirror Steps 2 & 3 (both occurrences + the artifact-dir line, ~L461 and ~L874-880).**
  A: «оставшиеся ручные шаги — `gh pr create` … и подтянуть main перед `deliver`» → "`approve` также открывает/обновляет
  PR (`gh pr create`, идемпотентно; тело из `docs/superpowers/pr/<id>.md` или fallback — t27), так что остаются
  ручными **мерж** и подтянуть main перед `deliver`». B: «оставшиеся ручные шаги — открыть PR (`gh pr create`) и
  подтянуть main перед `deliver`» → «оставшиеся ручные шаги — **мерж** и подтянуть main перед `deliver`»; artifact-dir
  line → «…`docs/superpowers/specs|plans/<id>-<slug>.md` (+ опциональное `docs/superpowers/pr/<id>.md` тело PR — t27)»;
  parenthetical → добавить «…и с **t27** `approve` открывает/обновляет PR — …».

- [ ] **Step 5: CLI_REFERENCE.md — reflow (one conjunction).** Replace "…`git push -u origin task/{{.ID}}` (the task
  branch, for the PR) **and** `deliver`/`cancel` run `git push origin main`…" → "…`git push -u origin task/{{.ID}}`
  (the task branch), **then opens/updates the PR** (`gh pr create`, idempotent — needs `gh`+`jq`; body from
  `docs/superpowers/pr/{{.ID}}.md` when present, else generated; t27); `deliver`/`cancel` run `git push origin
  main`…".

- [ ] **Step 6: CLI_REFERENCE.ru.md — mirror Step 5.** "…`git push -u origin task/{{.ID}}` (ветка задачи), **затем
  открывает/обновляет PR** (`gh pr create`, идемпотентно — нужны `gh`+`jq`; тело из `docs/superpowers/pr/{{.ID}}.md`
  если есть, иначе сгенерированное; t27); `deliver`/`cancel` — `git push origin main`…".

- [ ] **Step 7: Verify no stale "manual gh pr create" remains + commit.**

Run: `grep -rn "gh pr create" AGENTS.md DESIGN.md DESIGN.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md`
Expected: every hit now frames `gh pr create` as what **mtt runs automatically** (or the guarded status-description substring) — **none** as a "remaining manual step". Also run `make check` (habit) and eyeball EN↔RU equivalence.

```bash
git add AGENTS.md DESIGN.md DESIGN.ru.md CLI_REFERENCE.md CLI_REFERENCE.ru.md
git commit -m "t27: docs — approve mechanizes gh pr create (EN/RU, both DESIGN blocks)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: t27's own PR-body artifact (exercises the `--body-file` path on the first real run)

**Files:**
- Create: `docs/superpowers/pr/t27.md`

**Interfaces:** consumed by the Task-1 post at approve time (`--body-file docs/superpowers/pr/{{.ID}}.md`).

Rationale: `mtt approve t27` will be the **first live run** of the mechanized post (the branch's working-tree config already carries it). Writing `docs/superpowers/pr/t27.md` makes that run take the rich `--body-file` branch — dogfooding the artifact path — and gives t27 a proper PR body. If the command misbehaves, approve exits 5 and the PR is opened by hand (safe).

- [ ] **Step 1: Write the PR body** (`docs/superpowers/pr/t27.md`): a concise markdown body — what/why (finish flow mechanization; approve opens the PR), the config-only design (3rd post, idempotent open-if-none, title via `mtt show --json`, hybrid body), the gh+jq dependency + exit-5 contract, the two adversarial review rounds (spec DECLINE→fix), and that t40 was spun off. (First-person prose; this file IS the PR body.)

- [ ] **Step 2: Commit**

```bash
git add docs/superpowers/pr/t27.md
git commit -m "t27: PR-body artifact (rich body for its own PR)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

- **Spec coverage:** D1 config-only → Task 1 (config + test only). D2 open-PR existence test → canonical command `.[].number`. D3 title read `// empty` → canonical command. D4 hybrid body → canonical command `if test -f … --body-file … else --body …`. D5 gh/jq/mtt-on-PATH + exit-5 → inherited (post-failure contract, unchanged) + Task 2 (AGENTS.md failure enumeration broadened to PR-open). D6 both types → Task 1 edits both edges. D7 `approved` description keeps `gh pr create` → Task 1 Step 3 + guard unchanged. D8 guard len 3 → Task 1; docs (incl. the **two** DESIGN "Shipped" blocks EN/RU + the `docs/superpowers/pr/` artifact convention) → Task 2, verified by the Step-7 grep. t40 deferred → not in plan. ✓ all covered.
- **Placeholder scan:** none — every step has exact strings/commands.
- **Type consistency:** `cmdPrCreate` defined in Task 1 Step 1 and referenced in the same step's approve case; the config scalar in Step 3 is the same canonical string. Byte-match enforced by the guard (Step 4).

## Risks / notes for the implementer

- **Byte-match is the #1 failure mode.** After Step 3, if Step 4 still fails, diff the config scalar against `cmdPrCreate` character-by-character (em-dash, spaces, double quotes). They must be identical.
- **First real run = t27's own approve.** Expected; safe (exit 5 → open by hand). Do NOT pre-open t27's PR by hand before approve, or the idempotency check will (correctly) skip and you won't exercise `gh pr create`.
- **`mtt` on `$PATH`** must be the freshly built binary (the post calls `mtt show --json`).
