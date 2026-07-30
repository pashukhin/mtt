---
title: beads head-to-head verified (2026-07-30) + the pitch lead it grounds
tags:
    - docs
    - release
priority: high
refs:
    - kind: note
      id: positioning-mechanism-not-methodology
    - kind: note
      id: positioning-vs-beads
created: "2026-07-30T04:33:13Z"
updated: "2026-07-30T05:03:44Z"
---
CORRECTED 2026-07-30 against the INSTALLED binary — now **bd 1.1.2** (local updated from 1.0.2 and re-verified; the 1.0.2→1.1.2 changelog is entirely Dolt storage/migration hardening, no task-lifecycle change, so every conclusion below holds). This SUPERSEDES this note's original doc-based version, which was WRONG in several places — verifying against docs alone materially oversold mtt vs beads. (positioning-vs-beads was actually more accurate: "custom-but-global statuses; bd gate = async wait, not command-gated" — both confirmed against the binary.)

WHAT bd 1.1.2 ACTUALLY IS (a rich, mature tool — not the minimal tracker the doc snapshot implied):
- Statuses: 7 built-in (open/in_progress/blocked/deferred/closed/pinned/hooked) with categories, AND CUSTOM statuses configurable for multi-step pipelines (`bd config set status.custom "awaiting_review,awaiting_testing,..."`). NOT a fixed 5-status set. Statuses are global (not per-type).
- Types: 9 built-in (task/bug/feature/chore/epic/decision/spike/story/milestone) + custom types (`types.custom`).
- Knowledge/memory: beads HAS remember/recall/memories/forget + `bd prime` (AI workflow context) + comment/comments + note. So "a KB/prime pairing a tracker lacks" was WRONG — beads has all of it.
- Concurrency: hash IDs, Dolt cell-level merge, merge-slot (serialized-conflict gates), worktree, federation — beads is BUILT for parallel multi-agent; STRONGER than mtt here.
- Also: epic/swarm (hierarchy), audit (append-only JSONL), preflight/onboard/quickstart, jira/linear/ado integrations, sql, export/import, gc/memory-decay.

WHAT SURVIVES AS THE mtt WEDGE (verified against the 1.1.2 binary — and it is NARROW, ONE thing):
- The only structural thing beads cannot do: **gate a status transition on a SYNCHRONOUS LOCAL SHELL COMMAND, per task type.** beads' enforcement is (a) async external-wait gates (human/timer/gh:pr/gh:run/bead) created from a "formula step gate field", checked at close; and (b) git hooks at commit/push time. Neither runs a local command AT a task's transition. Config namespaces are export/jira/linear/github/custom/status/doctor — there is NO gate/transition/dod config.
- So the two things beads cannot enforce:
  1. ORDER — "a failing test exists BEFORE you take THIS task into work" (a red gate on tbd→implementing). A git pre-commit hook gates the COMMIT not the task's start; a gh:run gate is external + at-close. Neither expresses test-before-code.
  2. Executable per-type Definition-of-Done AS A TRANSITION CONDITION — feature runs red+green, chore green-only, design doc-check — hard, local, synchronous gates on the edge.
- This is EXACTLY the one irreducible value all four arm-C sessions independently named. Field evidence and the binary agree: the wedge is the command-gated per-type transition, nothing more.

WHAT DOES NOT SURVIVE (drop from the pitch — beads matches or beats):
- "In-repo memory / KB / prime pairing" — beads has memory+prime+comments+notes. NOT a differentiator.
- Multi-agent / parallel concurrency — beads is STRONGER; never claim it for mtt.

WHAT IS A MODEST-BUT-REAL EDGE (keep, honestly small):
- Plain-text-as-source: since bd v1.0.5 the JSONL auto-export is OPT-IN, so DEFAULT beads state is the Dolt DB — opaque without the tool. mtt's YAML-per-task IS the source (cat/grep/diff/edit directly), no DB, no export step. Real legibility/simplicity edge, but modest (beads can enable JSONL export).

PITCH (corrected, verified against 1.1.2; final wording TBD by maintainer):
- The wedge is ONE thing: **mtt gates transitions with a synchronous local shell command, per type — enforcing what beads structurally cannot: test-before-code (ORDER) and an executable per-type Definition-of-Done as a hard transition condition.** Everything else beads matches or beats (concurrency: beads wins). Narrow is correct — it is the "sharp fuse", and it is exactly what real users independently found irreducible. Secondary, honest-and-small: YAML-is-the-source plain-text legibility.
- Targeting unchanged: ROI scales with the number of context-breaks.

LESSON (process): verify competitor claims against the INSTALLED, CURRENT tool, never docs or a stale local build. The empirical beads arm (EXPERIMENT-contact-energy-beads.md, now run against bd 1.1.2) matters MORE now: beads is a serious, feature-matched competitor, so the head-to-head is a real test of the one surviving wedge — measure whether test-first is actually ENFORCED+VERIFIABLE in mtt and merely hoped-for in beads.
