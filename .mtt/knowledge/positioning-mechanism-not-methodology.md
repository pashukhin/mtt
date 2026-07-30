---
title: 'Positioning fork resolved (pilot-2): carry the mechanism + examples, not a methodology'
tags:
    - docs
    - process
    - release
priority: high
refs:
    - kind: note
      id: complexity-budget
    - kind: note
      id: positioning-vs-beads
    - kind: note
      id: release-scope
created: "2026-07-30T02:15:44Z"
updated: "2026-07-30T02:15:44Z"
---
Decided 2026-07-30 (pilot-2 reflection, pre-t60). Refines positioning-vs-beads and release-scope; governed by complexity-budget. Resolves the "narrow mechanism vs flow+standards" fork.

THE FORK. Either (1) mtt is a narrow mechanism — you build gates around an agent; we ship EXAMPLES (not templates) + first-class reference docs, and carry ONLY the mechanism downstream. Or (2) we invest in flow + templates + a shipped process/standards layer (which today does not exist — "everyone does as they like") and carry procedures/standards downstream too.

DECISION: Variant 1, sharpened. Downstream we carry ONLY: the mechanism (per-type flows, blocking shell gates, post/events, attribution), human LEGIBILITY of the true state, first-class reference docs, and a curated gallery of EXAMPLES. We do NOT build or own a methodology. This repo's own heavy dogfood flow is ONE example among examples — never "the recommended mtt flow".

WHY (this is the already-staked position, not a new lean):
- The wedge IS the mechanism (positioning-vs-beads): mtt RELOCATES enforcement, it does not supply gates. Value = consolidation + enforcement, not a new capability.
- Complexity-budget test #1: a capability enters core only if it strengthens enforcement of "done" OR legibility of true state. Process/standards/templates do neither -> extension/examples, not core. The channel (AGENTS.md snippet) punishes bloat.
- Pitch: "a state machine YOU define / YOUR shell commands / not a drop-in", aimed at a developer working THROUGH coding agents — the agent authors, the human supervises. That target is exactly who Variant 1 fits.
- NEW pilot-2 evidence (greenfield, not a migration): a fresh agent AUTHORED a sophisticated fit-for-purpose flow (red-first gate, self-contained gate scripts) from the mechanism + CONFIG_REFERENCE + examples alone, never touching source. Every friction found was a mechanism-edge or a doc gap (attribution default, bare-sh gate env, red-gate exit contract, empty-state, version stamp) — none was "missing process". Mechanism + good docs + examples were sufficient.

WHY NOT Variant 2 (steelman + rebuttal): a methodology clones harder and reads richer — BUT it is a WORSE moat: softer, higher-maintenance, an endless contestable opinion-surface that ties mtt's fate to OUR process opinions rather than the mechanism, and the channel punishes exactly that bloat. Cross-project uniformity was never a goal; the mechanism already standardizes WITHIN a project by enforcing the flow YOU author (this repo proves rigorous standards are authored IN mtt, not shipped BY it).

THE ONE COST WE OWN: the authoring tax / blank page. Mitigate WITHIN Variant 1 — excellent reference docs (t83 paid off) + curated examples + onboarding (t73) — never by shipping a methodology. Prefer the framing EXAMPLES (learn + steal) over TEMPLATES (adopt + depend): a "template" invites the drop-in expectation the pitch rejects and forces us to own a canonical flow.

FLIP CONDITION (re-open only if): the real target adopter turns out to be teams/humans who CANNOT author a flow and want an imposed process. The current pitch explicitly excludes that user.

ROADMAP CONSEQUENCES:
- t79 stops being a fork: ship EXAMPLES of toolchain-neutral (non-Make) gates + a no-code companion as teaching artifacts (gallery t78), NOT an official adopted template. Lighter; fine as fast-follow.
- The red-gate exit-code contract (t86 item 1) is a MECHANISM fix (the gate asks "is there a failing test?", not "is it failing correctly") + a doc line — legitimately core.
- t86's process guidance (verified-vs-assumed, criteria over prescriptions, record recurring frictions) stays OPTIONAL learning prose in FLOW_GUIDE — never a normative standard mtt enforces.
- t84/t85/t81/t80 are pure mechanism/legibility polish — in.
- A shipped methodology/standards layer: not built. t82 (full mtt config surface) stays post-release/feature.
