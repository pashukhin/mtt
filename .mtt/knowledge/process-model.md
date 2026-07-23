---
title: 'Product axis, not process: what mtt tracks here'
tags:
    - flow
    - process
priority: high
created: "2026-07-23T07:58:05Z"
updated: "2026-07-23T07:58:05Z"
---
mtt tracks the PRODUCT axis: a task is a unit of product change; the 15-status task flow is one
task's maturation (spec -> plan -> TDD -> reviews -> delivery), which may span several work sessions.
Sessions/phases (how we work) are process - ephemeral, executed, never queued; modeling them as
tracked items was the s009 category error, caught by asking "what are we actually tracking?".
- Two types, chosen by design-openness: task (design OPEN - spec+plan artifacts, each reviewed) vs
  chore (design ALREADY FIXED elsewhere - impl stage only). A chore whose diff contains undocumented
  design decisions must be declined and recreated as a task (the impl_review description polices this).
- A task leaves the queue only over a flow edge: deliver (verified "done = in main") or cancel --why.
  mtt rm erases the record - it is for mistakes, never closure. "Done directly" (work landed on main,
  task rm-ed - the t43 precedent) is the anti-pattern this note exists to kill: already-decided work
  becomes a chore instead.
- The flow prints its own instructions at every status (descriptions are the runbook); trust
  mtt roadmap / mtt show over any memory of the process.
