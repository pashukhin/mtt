---
title: git/GitHub integration traps (verified live)
tags:
    - dx
priority: medium
created: "2026-07-23T07:59:10Z"
updated: "2026-07-23T07:59:10Z"
---
- GitHub squash-merge takes the subject FROM THE COMMIT on single-commit PRs unless the repo sets
  squash_merge_commit_title=PR_TITLE (this repo flipped it; verified via gh api). Any convention keyed
  to "the PR title reaches the squash subject" depends on that setting.
- Branch protection on main would break the deliver/cancel post-push (exit 5) - the delivery tail
  assumes direct push (t33 tracks team semantics).
- git switch from an unborn HEAD exits 128; a fresh e2e repo needs one commit before switching.
- go get pkg@latest can raise the go directive floor of go.mod (a compat break for downstream Go
  toolchains): pin a floor-compatible version explicitly, then go mod tidy, and re-check go.mod still
  says the intended floor.
- gh pr create is made idempotent by guarding on gh pr list --head ... --state open (the approve post
  pattern); body from docs/superpowers/pr/<id>.md when present.
