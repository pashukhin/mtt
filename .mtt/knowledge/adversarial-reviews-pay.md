---
title: 'Two adversarial subagent reviews per task: the evidence'
tags:
    - process
    - tests
priority: high
created: "2026-07-23T07:58:05Z"
updated: "2026-07-23T07:58:05Z"
---
Every spec+plan pair sent to an adversarial subagent review has produced at least one real,
empirically-verified defect a self-review missed. Keep the loop: spec review, then plan review, each
instructed to REFUTE claims by running probes, not by reading prose. Catches on record:
- fail-open shell gate (! mtt list ... | grep -q . passes when mtt is missing; also t31's
  ! git diff form converting exit 128 into a pass) -> fail-closed shapes only;
- YAML quoting traps (double-quoted gate breaks \., a leading ! becomes a yaml tag) -> single-quote
  command scalars, assert exact strings in the guard test;
- a plan saying "create test file X" when X exists with tests (Write would silently drop them - grep
  before creating; append, no package/import header);
- cobra validates Args BEFORE RunE (a fixed-arity validator rejects bulk forms before classification);
- t31's clean-tree gate would have deadlocked its own task (.mtt dirtied by mtt note add is swept by
  the post-commit AFTER the gate) - the reviewer caught the ordering.
Reviews check FORM empirically; humans sign off at the *_human_review statuses.
