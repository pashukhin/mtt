---
title: 'Doc-sync: archival design-record vs live user-facing catalog'
tags:
    - docs
    - process
priority: medium
created: "2026-07-28T16:46:01Z"
updated: "2026-07-28T16:46:01Z"
---
When a change adds a member to a set that several docs enumerate (a new template, subcommand, flag,
adapter), don't reflexively update every mention. Grep the set-name across the whole repo, then classify
each hit:

- **Live user-facing catalog** — a place a reader consults to find the CURRENT set: FLOW_GUIDE §12
  "Neighbours", CLI_REFERENCE, README, templates/README. These MUST gain the new member or they read as a
  stale/incomplete index. Update them (both languages).
- **Archival design-record** — DESIGN.md's per-increment "Shipped (tN)" prose. It records what task N
  delivered and stays true-but-non-exhaustive as the set grows later. Retrofitting a later task's addition
  is revisionism, and it fights c23 (DESIGN unload). Leave it.
- **Specific-template reference / test fixture** — a doc line or test that names ONE member for a concrete
  example (e.g. `hierarchy.yaml` in a testscript), not "the set". Leave it.

The distinguishing test is NOT the "Shipped (tN)" label — FLOW_GUIDE §12 is also a "Shipped (t62)" block
yet is a live catalog we update. The test is: does a reader use this to find the current set (→ update), or
is it an archival design-record (→ leave)?

Also future-proof doc-comments that enumerate a set (Go package/test comments): reword them generically
("the hosted templates/*.yaml") so they never drift as the set grows, instead of appending each new name.

Surfaced in t78 (added `templates/docs.yaml` to the hosted-example set) — the adversarial reviews caught
both a missed live catalog (CLI_REFERENCE) and, initially, a wrong instinct to also edit DESIGN. Same hazard
family as the parallel-occurrence trap (grep ALL matches of a changed fact across EN+RU docs).
