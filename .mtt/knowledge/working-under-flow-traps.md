---
title: 'Working under the dogfood flow: traps and recoveries'
tags:
    - flow
    - process
priority: high
created: "2026-07-23T07:58:37Z"
updated: "2026-07-23T07:58:37Z"
---
- Commit code/docs to the task branch BEFORE mtt submit/approve - flow post-hooks auto-commit ONLY
  .mtt. Since t31 this is machine-enforced (clean-tree gate, .mtt excluded); the gate text names the fix.
- Backlog items mtt add-ed to LOCAL MAIN while a task is in flight: before deliver, reset --hard
  origin/main and cherry-pick the post-branch SHAs - deliver greps local main for the squash subject
  and pushes main, so a diverged local main ships stray commits. Prefer adding queue items on the task
  branch (they merge via the PR) or push main immediately.
- Gates inherit the caller's environment: an exported MTT_DIR leaks through a make check gate into the
  testscript suite and reds it deterministically. Prefer cwd discovery + config.local author over
  MTT_DIR/MTT_BY exports.
- Post-action failure (commit/push/PR-open) keeps the move and exits 5; finish by hand - the exit-5
  message prints the recovery commands (t28).
- --no-run skips ALL commands on the edge including context switches: on deliver/cancel it skips
  git switch main, so the terminal write strands on the current branch (t32 caveat). It always demands
  --who and --why (exit 2 otherwise) - a signed, audited bypass, not a convenience.
