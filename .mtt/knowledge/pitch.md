---
title: 'Pitch 1.0: why you need mtt (two paragraphs + who it is not for)'
tags:
    - docs
    - release
priority: high
created: "2026-07-27T15:50:31Z"
updated: "2026-07-30T09:09:22Z"
---
# Pitch 1.0 — why you need mtt

Working draft (2026-07-27). Two paragraphs, plus the honest coda. Source for the README pitch (task P1 / the release-scope note). Aimed at the Variant-A user: a developer working through coding agents. It does NOT work for a "human wants a Gantt planner" user — and that is the correct signal that the higher entry cost is justified.

**The pain.** You already have a definition of done: tests, lint, review, "spec before code". It lives as prose in AGENTS.md, in Make targets, in CI. Prose does not stop an agent — it hopes. And an agent silently skips whatever is not mechanically checked, and writes "done" where done is not (in one measurement, 19 of 19 false "all tests pass"). The problem is not that you lack a definition of done — it is that nothing enforces it inside the tracker itself.

**What mtt does.** mtt turns a task's lifecycle into a small state machine you define per task type (a bugfix is not a refactor), and every transition runs your own shell commands as blocking gates: checks fail, the transition does not happen, and the status cannot lie. The flow is committed repo data, not personal settings; it works with any CLI agent; every transition is written to append-only history with its check results. A local, zero-footprint Go CLI. And because the status cannot lie, when YOU look — at show / list / roadmap — you see a true map of where everything actually is: what is still an idea, what is blocked, what is two steps from release. In one line: "done" is not what the agent declares — it is what the tool refuses to record until it is earned.

**Honest coda (do not hide it).**
- The entry cost is higher than a simpler tracker's — and that is the price, not a flaw. mtt is useless until you author a flow. It is not a drop-in: author your definition of done once, and it is enforced from then on.
- Who it is NOT for: if you just want a to-do list, use something lighter — moving to mtt would be a lateral step plus an authoring tax. mtt wins decisively only if you want the process itself enforced — especially under agents, who silently skip anything not mechanically gated.


---
REFINEMENT (post beads head-to-head, 2026-07-30) — fold in when finalizing for the article (t60). Two greenfield pilots + a 4-role beads control arm sharpened this:
- CRISP irreducible = the ORDER gate: mtt gates a task's START on a local check (a failing test must exist BEFORE code). CI / pre-commit / hooks-on-result see only the OUTCOME ("green") and cannot see order — the one thing all eight pilot agents independently named as not reducible to other tooling.
- Category reframe: for coding agents mtt is a CONSTRAINT on the agent ("it closes doors", "it refuses"), not a to-do list. A tracker suggests; mtt's gate refuses. That is the sell.
- Targeting: ROI scales with the number of CONTEXT-BREAKS (many fresh agent sessions handing off), not team/project size. For a solo human on a small project it is overhead — say so; it is the correct signal.
- Honest-coda addition: you CAN hand-roll test-first enforcement with git hooks + commit conventions — a competing agent-tracker (beads) makes you do exactly that. mtt's difference is that the same guarantee is ONE declarative thing INSIDE the tracker, driven by one lifecycle verb per task — status cannot lie, no two-store hand-sync. Same enforcement, a fraction of the machinery. (Evidence: note beads-head-to-head-and-pitch-lead.)
