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
updated: "2026-07-30T04:33:13Z"
---
Verified beads against its current docs (2026-07-30) to ground the pitch before t60/the article. Final article wording is TBD (deferred by the maintainer); this note records the reasoning in detail so it is not lost.

BEADS MODEL (docs, 2026-07-30; sources: github.com/steveyegge/beads README + docs site + AGENT_INSTRUCTIONS.md):
- FIXED GLOBAL statuses: open, in_progress, blocked, closed, deferred. No custom statuses, no per-issue-type lifecycles. (A couple of built-in types exist, e.g. a `message` type with an ephemeral lifecycle, but no user-authored per-type flow.)
- Gates = special ISSUES that block dependent work (via the dependency graph) until an EXTERNAL async condition is met: human approval, `timer`, `gh:pr` (PR merge), `gh:run` (CI run), cross-rig `bead`. Enforced at `bd close`; checked by POLLING (`bd gate check` via cron/CI/hook); `--force` bypasses. There is NO synchronous local shell-command gate on a transition, and NO gate on the `start` edge.
- Store: Dolt/SQLite + JSONL synced via git; hash IDs (bd-a1b2) + cell-level merge — engineered for multi-agent / multi-branch concurrency. Also has "semantic memory decay" (summarizes old closed tasks).

WHAT THIS CONFIRMS (our differentiator holds AND sharpens):
- The mtt wedge is NOT "beads can't gate" (it can) but the FORM: synchronous local shell-command gate on ANY per-type edge, including `start`. Only that expresses ORDER (a failing test exists BEFORE code) and per-type work distinction (feature=red gate, chore=none, design=doc-only). beads' fixed-5-statuses + async-external-wait-gate-at-close cannot express either. Field-grounded: all four arm-C sessions independently named the order gate as the single irreducible value.
- Corollary: even "green/done can't lie" is NOT unique — beads also enforces at bd close (gh:run/gh:pr). So the pitch must NOT lead on the green "done" gate; the arm-C sessions unanimously called that the weakest, most replicable part.

WHERE BEADS IS STRONGER (state honestly; do NOT claim these for mtt):
- Multi-agent / parallel / concurrent writers: beads is BUILT for it (hash IDs, cell-level merge, native branching). mtt's one-file-per-task YAML is the known-weak axis (single-writer; t10/t33). This reverses the intuition that "mtt is better for multi-agent" — on true concurrency, beads leads.
- Memory: beads is also in-repo (Dolt/JSONL) and survives context loss; it even has memory-decay. So "in-repo memory" is NOT a clean mtt win — it is a TRADE.

THE PITCH LEAD (verified; final wording TBD):
- LEAD: mtt gates transitions with a synchronous shell command PER TYPE — including `start` — so it can enforce what a fixed-status async-wait model (beads) and result-only hooks (CI/pre-commit) cannot: ORDER (test-before-code) and the distinction between kinds of work.
- SECOND (honest trade, not a headline): state is plain text in the working tree (cat/grep/diff, no tool, no export) plus a knowledge pairing (notes/refs/prime) a tracker lacks — traded against beads' DB robustness, memory-decay, and concurrency.
- SILENT: parallel multi-agent concurrency — beads leads there; keep it out of the mtt pitch until our concurrency axis (t10/t33) is tested.
- TARGETING refinement (from arm C, unanimous): mtt's ROI scales with the NUMBER OF CONTEXT-BREAKS (many fresh agent sessions), not team or project size. Sharper than "a developer working through coding agents".

TWO SMALL CORRECTIONS TO FOLD INTO positioning-vs-beads later:
1. beads statuses are FIXED (open/in_progress/blocked/closed/deferred), not "custom-but-global".
2. beads gates enforce at `bd close` and are POLLED (bd gate check); add that they are external-wait/dependency-shaped.
3. Add explicitly: concurrency is beads' STRENGTH and mtt's weak axis (the note frames it only as "our gap").

NEXT: an empirical control arm (beads) on the same GAME.md, same 4-role structure, no stress condition — protocol in EXPERIMENT-contact-energy-beads.md. It tests the LEAD claim (does the order gate actually produce verifiable test-first that beads cannot) and honestly measures authoring-tax + (untested) concurrency.
