---
title: Where the history lives (orientation breadcrumb)
tags:
    - docs
priority: low
created: "2026-07-23T07:59:10Z"
updated: "2026-07-23T07:59:10Z"
---
mtt bootstrapped itself over sessions s001-s009 (2026-07-03..11): contract -> YAML store -> hierarchy
-> dependencies -> gated flow -> attribution/current/structured commands/rollback -> tags, priorities,
batch -> release prep -> s009 self-host (flow v2: verified delivery tail, two types, auto-commit/push/
PR posts). Since s009 every product change is an mtt task on a task/<id> branch.
- t31 (2026-07-23) retired the pre-mtt apparatus: TASKS.md (the bootstrap plan), sessions/*.md (the
  narrative archive), NEXT_SESSION.md (the handoff primer), and delivered-task artifacts under
  docs/superpowers/. Git history keeps all of them; nothing was lost, only de-canonized.
- Where things live now: queue = mtt roadmap; knowledge = mtt notes (mtt prime at session start);
  architecture = DESIGN.md; rules = AGENTS.md; per-task artifacts = docs/superpowers/{specs,plans,pr}/
  <id>-*.md for OPEN tasks only (delivered ones are deleted - git has them).
