---
title: 'Release scope 1.0: Variant A (the sharp fuse), polish-only'
tags:
    - process
    - release
priority: high
refs:
    - kind: note
      id: positioning-vs-beads
created: "2026-07-27T15:50:27Z"
updated: "2026-07-27T15:50:27Z"
---
# Release scope 1.0 — Variant A: the sharp fuse, polish-only

Decided 2026-07-27 in the pre-t60 "did we build a monster?" reflection.

**Course locked: Variant A.** mtt 1.0 is a small, sharp tool with one job — the fuse between a coding agent (and its human) and the word "done". NOT a do-everything platform (Variant B). This matches the wedge already stated in the positioning-vs-beads note; the drift was in DESIGN / CLI_REFERENCE / roadmap sprawl, not in the positioning.

**Core criterion (what belongs in the core).** A feature stays in the core only if it strengthens EITHER (a) enforcement of "done" (an agent cannot lie), OR (b) human legibility of the true process state (a human can look and see what is still an idea, what is blocked, what is two steps from release). The enforced typed flow is ONE artifact with TWO faces — machine (cannot-lie) and human (honest map). Views over the enforced flow (show / list / roadmap / tree) are IN; a separate planner / scheduler / Gantt is OUT. (This is the correction to the narrower "reinforces done" criterion — legibility for the human-in-the-loop is half the point.)

**Deferred, not done — the suite.** The long-term shape is a small family of focused tools sharing one codebase (tasks / knowledge / agent-setup), ideally one multi-call binary. This is POST-1.0. The finer split of task-flow from task-management was reconsidered and dropped: they share config and domain, so they are one tool; the only natural seam is tasks vs knowledge. The ports are already separate, so any future split stays cheap — spend zero release effort on it. Dropping the split also drops the one thing it forced into the release (pre-carving command namespaces).

**Frozen for 1.0 (kept, not grown; never deleted):** the knowledge base beyond notes + refs + prime (search, versioning, note-side hashtags), self-update, agent-scaffolding. Prefer freeze to amputation for working shipped code.

**Not in 1.0 at all:** remote / URL templates, external-tracker adapters (GitHub Issues etc.), any storage-engine change (a separate think-item records that storage stays plain YAML — a wedge asset — and the only real gap, multi-writer concurrency, is fixed behind the port, never by swapping YAML).

**The release is POLISH-ONLY.** Pre-release work set (tag `release`): pitch to README, an example-flow gallery, the hashtag toggle (t69), doc-hygiene (t63). Acceptance: a new user lives the two-paragraph pitch in ~10 minutes via a cold start (init -> add -> a blocked transition -> roadmap). Only then write the Medium article — a polished-surface article on a still-sprawling surface would sell the wrong product.

See the complexity-budget and pitch notes.
