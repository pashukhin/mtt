---
title: testscript/e2e conventions that bite
tags:
    - tests
priority: medium
created: "2026-07-23T07:58:37Z"
updated: "2026-07-23T07:58:37Z"
---
- Assertions must anchor, not substring-match: 't1  task  \[tbd\]', (?m)^id$ for line-oriented output;
  a loose substring matches vacuously. stdout regexp is whole-output.
- No shell pipes: model a|b as exec a -> cp stdout f -> stdin f -> exec b - (stdin resets per command).
- Wall-clock timestamps tie at second resolution: e2e asserts ordering RELATIONSHIPS ((?s)t1.*t2),
  never absolute positions; exact order is a unit test with a fixed clock.
- A gated e2e config ships as txtar -- gated.yaml -- cp-ed over .mtt/config.yaml, and must be a VALID
  flow: mtt add runs Config.Validate, a 2-status initial->terminal flow dies at the first add - use
  initial->active->terminal even for a minimal demo.
- git in testscript: guard with [exec:git], git init -b main + user.name/email + one --allow-empty
  commit (git switch from an unborn HEAD exits 128); assert an unborn branch via git symbolic-ref
  --short HEAD, not git branch --list.
- Gate-output needles must be OUTPUT-only: the runner echoes commands on progress lines, so a needle
  that substrings the command text proves nothing - use computed output (echo $((13*13)) -> 169).
- A blocked-move e2e must pin the CAUSE on stderr: with require active, ! exec alone cannot tell an
  attribution exit-2 from a gate exit-3.
- txtar support files unpack into $WORK; after cd proj reference them as $WORK/file.
