---
title: Release mechanics + launch plan essentials
tags:
    - release
priority: medium
refs:
    - kind: task
      id: t60
created: "2026-07-23T07:58:37Z"
updated: "2026-07-23T07:58:37Z"
---
- Version = git-describe stamp (t30): no version constant to bump per change; semver decided at tag
  time; RELEASING.md is the runbook; cadence is batched and on-demand, never per-PR.
- Launch trigger (decided 2026-07-10): a tagged release + the dogfood proof - do NOT wait for an
  external tracker adapter. Dogfood IS the launch asset: "mtt's own development passes through its own
  gates", demoed by a real history excerpt with checks/who/why.
- Launch assets in priority order: (1) the copy-pasteable AGENTS.md adoption snippet in README;
  (2) a "how mtt gates its own development" write-up with one honest what-it-does-not-do section
  (cooperative discipline, not a jail; --no-run exists and is signed); (3) README positioning surface
  (vs-harness-hooks section, refreshed scan, gate-naming tagline); (4) Show-HN / r/ClaudeAI leading
  with enforcement ("your agent can't say done until the gate is green"), never "task tracker".
- Wave-2 signals: build the GitHub Issues adapter on recurring real "can it write to my tracker?"
  asks; if "how is this different from hooks?" dominates, iterate the POSITIONING, not features.
- The channel is one-shot: known warts (invisible gate cause, broken documented pattern) burn the
  launch - polish the checklist, timebox it, cut scope rather than slip.
