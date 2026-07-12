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

### D2 — Idempotent create-if-absent, no update-on-re-approve

`gh pr view <branch> >/dev/null 2>&1 || gh pr create …` — open the PR only if none exists. `approve` re-fires
after `decline → impl_fix → submit → approve`, and re-opening/-updating would either error or clobber
hand/UI edits to the PR body. So: **create if absent, otherwise leave the existing PR untouched.**

### D3 — Title read (not templatized)

`--title "{{.ID}}: $(mtt show {{.ID}} --json | jq -r .title)"`. `{{.Title}}` is deliberately **not** in the
placeholder whitelist (`core.expandCommands` rejects it — `expand_test.go`), and t27 does not widen it; the
title is **read** at run time via read-only `mtt show --json` (SEC2-safe: read-only mtt, never a transition).
The `{{.ID}}: ` prefix is load-bearing — the `deliver` gate greps `^{{.ID}}: ` on the squash subject, and the
GitHub squash-merge default subject is the PR title.

### D4 — PR body: hybrid artifact-or-fallback

- **If `docs/superpowers/pr/{{.ID}}.md` exists** → `gh pr create … --body-file docs/superpowers/pr/{{.ID}}.md`.
  The agent writes rich prose there (the "why", review rounds, verification — the value a template can't give),
  mirroring the existing `docs/superpowers/{specs,plans}/{{.ID}}-*.md` artifacts and keeping the prose under
  agent control.
- **Else** → a minimal generated `--body` (e.g. `Automated PR for {{.ID}} — see: mtt show {{.ID}}`).

The artifact is **optional** — no `ls` gate on it (unlike the spec/plan submit edges). Chores (no spec/plan)
simply fall to the generated body; when they warrant rich prose the agent writes the file. This is the
resolution of the task's open question (options a–e in the task description): **hybrid (b)+(e)**, config-only.

Draft command (final quoting finalized in the plan; single-quoted YAML scalar, so no `'` inside; **no
backticks** in the fallback body — they are shell command-substitution):

```
gh pr view task/{{.ID}} >/dev/null 2>&1 || { t="{{.ID}}: $(mtt show {{.ID}} --json | jq -r .title)"; if test -f docs/superpowers/pr/{{.ID}}.md; then gh pr create --base main --head task/{{.ID}} --title "$t" --body-file docs/superpowers/pr/{{.ID}}.md; else gh pr create --base main --head task/{{.ID}} --title "$t" --body "Automated PR for {{.ID}} — see: mtt show {{.ID}}"; fi; }
```

### D5 — New runtime dependencies: `gh` + `jq`; failure → exit 5

The mechanized `post:` now requires `gh` (authenticated) and `jq`. `gh` was already assumed (the human ran
it); `jq` is new to the flow. Any failure (missing binary, unauth, network, `gh` API error) makes the `post:`
fail → `core.ErrPostAction` → CLI **exit 5**, the move **kept** (status persisted, branch pushed) — the human
finishes by opening the PR by hand. Identical to the existing push-failure contract; no new failure semantics.

### D6 — Scope: both types

Applies to task and chore approve edges alike. Artifact directory `docs/superpowers/pr/` is symmetric with
`specs/`/`plans/`.

### D7 — `approved` status description update (+ its guard)

The `approved` status description currently instructs the human to run `gh pr create …`. After mechanization it
must say the PR is opened/updated **automatically** (human just merges; after squash-merge run `mtt deliver`).
`TestRepoDogfoodConfig` pins `Contains(approved.Description, "gh pr create")` — that assertion is updated in
lockstep (the description still names `gh pr create` as *what mtt runs*, or the guard is repointed to a new
stable substring; decided in the plan).

### D8 — Test guard + docs

- `TestRepoDogfoodConfig`: the approve case now expects `post` **length 3** — `[cmdPostCommit, cmdPushBranch,
  cmdPrCreate]` — on both types (a new pinned `cmdPrCreate` constant, byte-matching the config), plus D7's
  description assertion.
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
