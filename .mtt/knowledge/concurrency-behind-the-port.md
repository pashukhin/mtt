---
title: 'Concurrency: git-native default behind the port; a server is an adapter, never the core'
tags:
    - core
    - process
    - think
priority: high
refs:
    - kind: note
      id: beads-head-to-head-and-pitch-lead
    - kind: note
      id: complexity-budget
    - kind: note
      id: positioning-mechanism-not-methodology
    - kind: note
      id: storage-engine-behind-port-think
created: "2026-07-30T12:47:43Z"
updated: "2026-07-30T12:47:43Z"
---
Decided 2026-07-30 (post beads head-to-head + concurrency design discussion). Refines storage-engine-behind-port-think. Prevents the "just build mtt-server" temptation from resurfacing every time the ID-race / lost-update problem is hit.

THE PULL. Real concurrency needs atomicity + idempotency from the store. A server/DB (or embedded Dolt — beads' choice) trivializes ID-minting, lost-update, cross-clone and polyrepo at once — the honest pull toward an "mtt-server". BUT a server ENDS the wedge: it is Variant B (release-scope rejected it), it is the heaviness the pilot agents disliked in beads' Dolt, and it kills the AGENTS.md-snippet adoption channel (nobody stands up a server to try a tool). The head-to-head just proved light+integrated beats heavy — a server contradicts that conclusion the day after we recorded it.

THE grep NUANCE. "A server still lets you query, just differently" is half true: an API replaces QUERYING, but loses the PROPERTIES that ARE the wedge — state lives WITH the code (same commit / PR diff), offline, zero-infra, readable by ANY tool without an API. Those are properties, not "a different grep".

DECISION:
- Concurrency is solved BEHIND the TaskStore port. The DEFAULT store stays plain-YAML-in-git (the wedge, a retention asset). Build + debug the git-native concurrency on it NOW.
- git-native mechanism (default; MONOREPO / one shared git store): an atomic CLAIM via a git-ref-CAS command hung on the EXISTING transition pre-gate (the start edge) — mtt stays git-agnostic, the command does the CAS. The claim SERIALIZES same-entity writes, so we NEVER need cell-merge — this is how we stay light where beads went to Dolt. The claim IS the assignee (t3 resurrected as the claim primitive, not tracker-richness). Comments (t2) deferred: claim + deps cover double-work + order; add a channel only if that proves insufficient.
- The HARD git-native piece is the ID-minting race across clones (t74): a pre-add gate doing git-ref-CAS on a counter (network round-trip per create), OR deferred minting (ID on merge-to-main), OR monotonic non-reusable ids. Reversing "CRUD post-only" (release-scope) is on the table ONLY for this. If it is too painful for someone, that is their signal to use a server/external adapter — a topology choice, not a reason to make the server the default.
- POLYREPO is OUT of the git-native mechanism (no shared git substrate to coordinate on). Kept in mind, deferred: needs a shared state-branch/repo (t33) or an external store. Not chased now.
- SERVER / external store (GitHub Issues t76, Jira, a hypothetical mtt-server) = an OPTIONAL adapter behind the port, for teams needing server-grade concurrency or polyrepo. NEVER the core. The server path is potentially ahead of us (via adapters t9/t76) — but the git-way is tried and debugged FIRST.

FLIP CONDITION: if we deliberately decide mtt's future is a multi-agent PLATFORM for teams (not the sharp fuse for a dev-through-agents), a server makes sense — but that is an explicit change of audience + product, taken with eyes open, never entered through the ID-race. Today all positioning says: sharp fuse.

Backlog: t10 (lost-update/concurrency cluster), t33 (state visibility across branches / claim), t74 (monotonic ids), t3 (assignee = claim), t2 (comments, deferred), t9/t76 (external-store adapters).
