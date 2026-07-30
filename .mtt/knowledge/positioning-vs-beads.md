---
title: 'Positioning: the wedge, the 2026 landscape, the channel'
tags:
    - docs
    - release
priority: high
created: "2026-07-23T07:58:37Z"
updated: "2026-07-30T09:09:26Z"
---
Wedge (bet #1, verified un-copied as of 2026-07-09): config-driven PER-TYPE flows + BLOCKING
shell-command transition gates in a local zero-footprint CLI. Nobody else combines them: beads has
custom-but-GLOBAL statuses and bd gate = async wait primitives (approval/timer/CI), not command-gated
transitions; Backlog.md has DoD CHECKLISTS the agent ticks (pitch: "everyone has the DoD vocabulary -
mtt makes DoD executable"); Task Master a fixed enum; Claude Code Tasks + TaskCompleted hook = one
global hook, home-dir state, Claude-only, gates only "done".
- The #1 2026 objection is "my harness has a completion hook": answer table - per task type (bugfix !=
  refactor), flow as committed+reviewed REPO DATA not personal settings, harness-portable (any CLI
  agent), gates every lifecycle edge not just done, append-only history with check results.
- Problem validation numbers (cite sparingly, one per pitch): 75% rework issue (claude-code#25305),
  19/19 false "all tests pass" (BSWEN 2026-06), reward-hack exclusion drops SWE-bench Pro 87.1->73.0
  (Cursor 2026-06).
- Channel: an AGENTS.md snippet, not the binary (beads and Backlog.md both grew that way). The channel
  punishes bloat and confusing errors - zero-footprint and crisp errors are RETENTION features.
- Bet #2 (external tracker as store-of-record) stays roadmap material, never headline; adapters follow
  demand (GitHub Issues first, only on real asks).


## Pilot verdict (2026-07, first real beads -> mtt migration; treat as directional — one session, one project, migrator has investment bias)

- **Wedge confirmed in the field, and reframed honestly.** The adopter moved a real polyglot repo off beads specifically for executable-enforced DoD, and it paid off. Crucial nuance for the pitch: mtt did NOT invent their gates — they already had them (Make/CI/AGENTS.md prose). mtt RELOCATED the enforcement into the tracker so gates can't be skipped and status reflects them. **The value is consolidation + enforcement, not a new capability.** Lead with that framing.
- **"Not a better beads — a different tool."** mtt wins DECISIVELY iff you want the process itself enforced — especially with agents, who silently skip anything not mechanically gated. If you only want to track issues, beads -> mtt is a lateral move + a maturity regression + an authoring tax, not worth it. Position as a gated control plane, NOT a tracker replacement.
- **Say the maturity regression out loud (don't hide it).** vs a mature tracker you LOSE today: batteries-included time-to-value; a hygiene layer (stale/orphans/preflight/lint/doctor, workflow templates, a human-decision flag, defer/supersede, acceptance/design fields); free-text search; import/export; multi-writer concurrency; bulk commit-hygiene. MOST are DELIBERATE non-goals per our positioning (we don't chase beads' richness). The few REAL gaps are already tracked: search (t6), import/export (t67/t68), onboarding time-to-value (t73), commit-firehose under bulk (t72/t13), concurrency (t10/t33).
- **Authoring tax is the sharpest adoption barrier / retention risk on the AGENTS.md-snippet channel.** mtt is useless until you author a flow; the built-in default is trivial and the flagship is a starting point to adapt. Levers: better starter templates (t62 shipped) + guided onboarding (t73). Never sell it as drop-in.
- **Store trade, honestly:** plain-YAML-as-truth (inspectable, diffable, no DB/export dance) is a genuine win over beads' embedded Dolt DB + sync — but it is merge-conflict-prone under concurrent writers (single-writer story; t10/t33).


---
HEAD-TO-HEAD UPDATE (2026-07-30) — verified against the bd 1.1.2 BINARY + an empirical 4-role beads control arm (note beads-head-to-head-and-pitch-lead; journals EXPERIMENT-contact-energy*). Corrects the earlier doc-based read:
- Statuses: beads has 7 built-in + categories AND CUSTOM statuses (config status.custom) — "custom-but-global" holds, NOT a fixed set. Gates are async external-wait (human/timer/gh:pr/gh:run/bead), enforced at bd close, polled — "not command-gated transitions" confirmed.
- beads is a RICH tool: its OWN knowledge/memory (remember/recall + bd prime), comments, epics/swarm, audit, integrations, batch create. So memory/KB pairing and context recovery are NOT mtt differentiators — beads matches them.
- CONCURRENCY IS BEADS' STRENGTH, not just "our gap": hash IDs + Dolt cell-merge + merge-slot + worktree + federation — built for parallel multi-agent, where mtt's plain-YAML store is weak (t10/t33). State honestly.
- EMPIRICAL result: beads + a git commit-msg hook (RED: convention) + a spec/impl bead split ACHIEVED the same enforced, verifiable, 0-bypass test-first as mtt. So the wedge is NOT "only mtt can enforce order". The surviving, verified differentiator is INTEGRATION + COST: mtt makes the same guarantee ONE declarative, tracker-integrated, lifecycle-verb-driven thing at ~1/3 the mechanical ceremony (one git substrate); beads reconstructs it as decoupled hooks + conventions + a 2x backlog + two-store sync (and here did not finish the game). Both arms' agents independently confirmed the split ("keep the gate; the tracker only suggests").
- One place mtt's plain-text-YAML-as-source genuinely wins (agents felt it): beads' Dolt-canonical / JSONL-export duality is "inverted for a git-native team".
