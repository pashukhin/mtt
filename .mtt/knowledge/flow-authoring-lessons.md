---
title: 'Authoring flow configs: the hard-won rules'
tags:
    - flow
priority: medium
created: "2026-07-23T07:59:10Z"
updated: "2026-07-23T07:59:10Z"
---
- Descriptions are load-bearing (the flow IS the runbook printed at each status): guard-test them like
  commands - Config.Validate runs on add/types, NEVER on Load or the move path, so the repo guard test
  asserting EXACT strings is the sole protection against silent YAML mangling.
- Single-quote command scalars: a double-quoted scalar eats backslashes (\.mtt breaks), a bare leading
  ! parses as a yaml tag and vanishes.
- Gates must be fail-closed: out=$(cmd) && test -z "$out" (an operational failure lands in the error
  branch). NEVER negate a command (! cmd) to express "no diff/no output" - negation converts exit 128 /
  missing-binary into a PASS (caught twice: the s009 self-ref gate, the t31 changelog gate draft).
- Commands run PRE-write: an edge that must land its state write elsewhere switches the tree first
  (git switch main on deliver/cancel) and re-guards with test -f after the switch.
- Gates see the caller's working tree and env; order gates cheap-first.
- Invariant-rejection fixtures must isolate exactly ONE violation (a self-loop edge is a clean
  isolator); verify the fixture fails on the intended invariant, not a neighboring one.
- e2e proves the MECHANISM with generic POSIX commands (touch/false), not git; precise semantics
  (reverse order, best-effort) are unit tests against the fake runner.
