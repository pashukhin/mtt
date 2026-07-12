# t27 — mechanize `gh pr create` on approve

Status: design spec (decision record) for t27. Input: the s009 c1 / flow-mechanization retro (c1 automated the
branch push; the ONLY remaining manual git step is opening the PR) plus this session's dogfood — four chores
(c3/c5/c6/c4) each had their PR opened by hand. Goal: **the `approve → approved` edge opens/updates the PR
itself**, config-only, so the human's remaining job is to merge.

Everything is mechanical EXCEPT the PR body; the body was the "think" this task carried. Decision below.

## Decisions

### D1 — Config-only: a third `post:` action on the approve edge (no production Go)

Both `approve` edges (`impl_review → approved`, task **and** chore) gain a **third** `post:` entry, after the
existing `[git add .mtt && commit, git push -u origin task/<id>]`. It runs **after** the push (the branch must
be on the remote for `gh pr create --head`). The mechanism rides entirely on surfaces that already exist —
`post:` shell, the already-whitelisted `{{.ID}}`, read-only `mtt show --json`, `jq`, `gh` — so **no `core` /
`pkg/mtt` / adapter / CLI code changes**. `gh pr create` belongs in `post:` (not the `commands:` gate): the
gate stays offline, network already lives in `post:` (the `git push`).

### D2 — Idempotent create-if-**open**, no update-on-re-approve

Open the PR only when **no OPEN PR** exists for the branch. **Not** `gh pr view <branch>` — that matches a PR
in *any* state (open/closed/merged), so after `decline → impl_fix → … → approve` where the prior PR was
**closed**, it would find the closed PR, short-circuit, and **silently skip** creation (no PR, `approve` exits
0 — no signal). Use a state-filtered existence test:

```
[ -n "$(gh pr list --head task/{{.ID}} --state open --json number --jq ".[].number")" ] || { …create… }
```

`--jq ".[].number"` **iterates** — empty output on `[]` → create; the number(s) → skip. (`.[0].number` would
print the literal `null` on an empty array — `null.number` → `null` — and wrongly skip forever; verified.)
`approve` re-fires after `decline → impl_fix → submit → approve`; re-opening or updating an existing open PR
would clobber hand/UI edits, so: **create iff no open PR, else leave it.** Double-quoting the jq expressions
keeps the whole command a single-quoted YAML scalar (matching the existing posts).

### D3 — Title read (not templatized)

`--title "{{.ID}}: $(mtt show {{.ID}} --json | jq -r ".title // empty")"`. `{{.Title}}` is deliberately **not**
in the placeholder whitelist (`core.expandCommands` rejects it — `expand_test.go`), and t27 does not widen it;
the title is **read** at run time via read-only `mtt show --json` (SEC2-safe: read-only mtt, never a
transition). `// empty` matters: `Title` is `omitempty`, so a task carrying only a description yields `t27: `
(honest) instead of the literal `t27: null`. Titles are assumed **single-line** (a newline survives shell
capture as one argument but GitHub rejects a multi-line title). The `{{.ID}}: ` prefix is load-bearing — the
`deliver` gate greps `^{{.ID}}: ` on the squash subject (a `:` inside the title body is fine — the anchor is
prefix-only), and GitHub's squash-merge default subject is the PR title. Shell-injection-safe: the title is
captured into a variable via command substitution and passed as `--title "$t"` (double-quoted expansion is not
re-scanned), so `"`/`$`/backticks in a title are inert — verified.

### D4 — PR body: hybrid artifact-or-fallback

- **If `docs/superpowers/pr/{{.ID}}.md` exists** → `gh pr create … --body-file docs/superpowers/pr/{{.ID}}.md`.
  The agent writes rich prose there (the "why", review rounds, verification — the value a template can't give),
  mirroring the existing `docs/superpowers/{specs,plans}/{{.ID}}-*.md` artifacts and keeping the prose under
  agent control.
- **Else** → a minimal generated `--body` (e.g. `Automated PR for {{.ID}} — see: mtt show {{.ID}}`).

The artifact is **optional** — no `ls` gate on it (unlike the spec/plan submit edges). Chores (no spec/plan)
simply fall to the generated body; when they warrant rich prose the agent writes the file. This is the
resolution of the task's open question (options a–e in the task description): **hybrid (b)+(e)**, config-only.

Draft command (the plan pins the byte-exact string that the guard matches; single-quoted YAML scalar, so no
`'` inside — jq expressions are **double-quoted**; **no backticks** in the fallback body — they are shell
command-substitution). Validated in scratch (existence semantics, nested `$( )` quoting, special-char title):

```
[ -n "$(gh pr list --head task/{{.ID}} --state open --json number --jq ".[].number")" ] || { t="{{.ID}}: $(mtt show {{.ID}} --json | jq -r ".title // empty")"; if test -f docs/superpowers/pr/{{.ID}}.md; then gh pr create --base main --head task/{{.ID}} --title "$t" --body-file docs/superpowers/pr/{{.ID}}.md; else gh pr create --base main --head task/{{.ID}} --title "$t" --body "Automated PR for {{.ID}} — see: mtt show {{.ID}}"; fi; }
```

### D5 — New runtime dependencies: `gh` + `jq`; failure → exit 5

The mechanized `post:` now requires `gh` (authenticated) and `jq`, and calls the **`mtt` binary on `$PATH`**
(`mtt show --json`, not in-process code — a stale installed `mtt` lacking `--json`/`.title` would corrupt the
title; the dogfood assumption is a current built `mtt` on `$PATH`, same as every other `mtt …` the flow runs).
`gh` was already assumed (the human ran it); `jq` is new to the flow. Any failure (missing binary, unauth,
network, `gh` API error) makes the `post:` fail → `core.ErrPostAction` → CLI **exit 5**, the move **kept**
(status persisted, branch pushed) — the human finishes by opening the PR by hand. Identical to the existing
push-failure contract; no new failure semantics.

### D6 — Scope: both types

Applies to task and chore approve edges alike. Artifact directory `docs/superpowers/pr/` is symmetric with
`specs/`/`plans/`.

### D7 — `approved` status description update (+ its guard)

The `approved` status description currently instructs the human to run `gh pr create …`. After mechanization it
says mtt **runs `gh pr create` automatically** — the human just merges; after the squash-merge run `mtt
deliver`. **Decision (pinned): keep the literal `gh pr create` substring** in the reworded description (it now
names *what mtt runs for you*), so `TestRepoDogfoodConfig`'s `Contains(approved.Description, "gh pr create")`
guard needs **no new anchor** — only the description text changes, on both types.

### D8 — Test guard + docs

- `TestRepoDogfoodConfig`: the approve case now expects `post` **length 3** — `[cmdPostCommit, cmdPushBranch,
  cmdPrCreate]` — on both types (a new pinned `cmdPrCreate` constant, byte-matching the config). The existing
  `Contains(approved.Description, "gh pr create")` assertion is **unchanged** (D7 keeps the substring); only the
  description text changes.
- Docs (EN/RU where bilingual): AGENTS.md ("Moves auto-commit / auto-push" bullet → approve also opens the PR;
  the gh+jq dependency), DESIGN.md/DESIGN.ru.md + CLI_REFERENCE.md/.ru (the c1 auto-push note → also PR-open),
  and the `docs/superpowers/pr/` artifact convention. `internal/adapter/yaml/CLAUDE.md` if the guard note needs it.

## Alternatives considered (from the task's a–e)

- **(a) whitelist `{{.Description}}`** → body = task description. Rejected: description ≠ a rich PR body, and it
  widens the deliberately-narrow 4-field whitelist (t5/t21).
- **(c) synthesize body from `mtt show --json` + spec/plan links** → static template, no rich prose. Folded into
  D4's fallback (kept minimal to keep the config scalar reviewable; rich prose is the artifact's job).
- **(d) `mtt approve --pr-body-file …` flag** → threads a caller value into the post context. Rejected **for
  now** as a one-off; the general problem (a per-invocation channel into the placeholder context beyond
  `{ID,Type,From,To}`) is **spun off to t40** (backlog). t27 stays config-only.
- **(e) minimal template only, edit PR by hand** → chosen as the *fallback* half of D4, not the whole answer.

## Out of scope (deferred)

- **t40** — generalized argument-passing into the gate/post placeholder pipeline (the `--pr-body-file` flag and
  its generalization). t27 must not add a new CLI flag or placeholder.
- Updating an existing PR's body on re-approve (D2: create-if-absent only).

## Risks

- **Gnarly `post:` scalar.** A multi-statement shell with `mtt show | jq`, an `if/else`, and `gh` is more than
  the one-line posts elsewhere. Mitigation: keep it readable (title in a var, explicit `if/else`), avoid
  backticks, review it "like a Makefile" (SEC2). The plan pins the exact string and the guard byte-matches it.
- **First real run is t27's own approve.** On branch `task/t27` the working-tree config already carries the new
  post, so `mtt approve t27` is the first live execution (self-dogfood, like c3's deliver). If it misbehaves →
  exit 5, PR opened by hand — safe.
- **`gh`/`jq` absence** reddens approve (exit 5) where before it was a silent manual step. Accepted and
  documented (D5); the move is kept, so it is recoverable, not destructive.
