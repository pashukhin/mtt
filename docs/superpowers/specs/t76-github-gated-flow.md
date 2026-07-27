# t76 — GitHub-gated flow (decision record)

**Type:** task · **Tags:** adapter, release · **Blocks:** t60 (public release) · **Enabler:** t40

Decision record from the 2026-07-27 brainstorm on "external-tracker adapters for the public
release" (prompted by the pilot adopter's suggestion that a free, popular tracker — GitHub Issues —
would make the release more useful, validated by dogfooding on ourselves).

## Problem

The release (t60) would be more adoptable if mtt's executable gates could apply to a tracker teams
already use (GitHub Issues first). The DESIGN "open question — which flow is authoritative" had to be
resolved before any such integration. The naive framing was a binary: **(A)** mtt owns the flow, the
tracker is a data mirror, vs **(B)** adopt the tracker's native workflow as the flow.

## The spectrum (reframing the crux)

There are two extremes and a continuum between:

- **External tracker has a rich flow of its own**, which mtt can *use* (with or without hanging gates
  on its edges).
- **External tracker has nothing** — mtt runs standalone (today's local-YAML case).

Where a user sits on that spectrum is **their choice**, and mtt should support the useful range. What
mtt *owes as code* is only the far extreme (read a tracker's live flow / store tasks only there).

## Decision trail (why this scope, honestly)

1. **We cannot add vertices to the tracker's status graph.** GitHub's frame is `open ⇄ closed`
   (`state_reason` completed/not_planned/reopened). We can only drive its coarse edges and hang gates
   on *our* side — not invent intermediate GitHub statuses.
2. **A quotient/refinement mapping (mtt's fine statuses → GitHub's coarse states) was considered and
   rejected as premature.** It works in theory (persist the fine status in a label, lift via
   `state_reason`, project on write) but is real machinery for little near-term value.
3. **The clean model is two linked entities, not a status mapping.** The GitHub issue keeps its native
   model, untouched; a **rich mtt task (local YAML, full gated flow)** references it; the issue's
   **close is the terminal action of the gating task**; drift (issue closed while its gating task is
   not done) is **detected and complained about**, not enforced.
4. **Crucially — this is already achievable with mtt's existing primitives.** Gate/`post:` commands are
   shell; they can call `mtt` (self-calls) and `gh`. The pilot already proved gates-call-`mtt` (its
   load-bearing "all children done" gate via `mtt list --parent <id> --json`). Even capturing a freshly
   created issue number back into the task is a one-line `post:` shell scalar
   (`u=$(gh issue create …); mtt ref add {{.ID}} url:$u`). So the useful middle of the spectrum needs
   **near-zero new Go**.
5. **Therefore: validate-then-build, not build-an-adapter.** Building a bespoke GitHub adapter (or a
   `mtt tt` command family) up front is premature when the composition already works. Prove it by
   dogfooding; let the evidence decide what, if anything, to build.

## What this task IS (scope)

1. **Dogfood the compositional GitHub-gating pattern on this repo** — using existing primitives + `gh`,
   near-zero new Go — to validate the release story: *"put executable mtt gates on the close of your
   GitHub issues."* Capture where it is clunky.
2. **Package the proven pattern as a `github-gated` flagship init template** (the way the git-flow
   flagship packaged git integration), so adopters do not hand-roll the `gh`-shelling gates.
3. **Let the dogfood evidence decide** whether any bespoke code (a `mtt tt {take,status,link,…}` sugar
   family, or a helper) is warranted — do **not** build it up front. If the config-only pattern is too
   clunky even with t40, that finding justifies the code; otherwise the template is the deliverable.

## What this task is NOT (deferred, deliberate)

- **True GitHub-as-store-of-record** (issues live *only* in GitHub; `mtt roadmap`/`list` read them
  live; the tracker supplies the flow) — the external-store bet. Stays **t9**, demand-driven. Needs a
  real read/write adapter + the composition-root port-selection seam (overlaps t61); out of the release.
- **Projects v2 reflection** (mirroring the fine mtt status onto a Projects board Status field for a
  richer human view), **Trello**, **bidirectional reconcile**, and the **universal-graph** framing —
  all later / demand-driven.

## Enforcement honesty (state it in the docs)

mtt's gate bites only where mtt is the **choke point** for the transition. With local YAML,
`mtt <status>` is the only mover — the gate always bites. With an external tracker that has its own UI,
a human/agent can move the issue **in the GitHub UI**, bypassing mtt (no local hook intercepts it). So:
**mtt gates the AGENT** (it works through mtt); a human closing an issue in the GitHub UI bypasses the
gates. The honest contract is **detect + complain** ("issue #N closed but its gating task <id> is not
done — resolve it"), never a claim to *enforce*. Server-side enforcement (a GitHub Action / branch-
protection-style reject of an ungated transition) is a heavier, separate layer, explicitly out of scope.

## Where the history lives (a non-problem here)

Unlike a store-of-record adapter, this model keeps the append-only `history` with gate-check results
in the **mtt task (YAML)**, where it already lives. GitHub's timeline never needs to hold mtt's check
results. This is a concrete advantage of the two-linked-entities model over quotient/store-of-record.

## Enabler: t40 (not a blocker)

The pattern's only real friction is the pilot's #1 finding — a gate/`post:` command cannot see its own
task's fields (title/tags/parent/children), forcing re-query (`mtt show --json {{.ID}}`) and shell-outs.
**t40** (richer gate/post context — full task in the env + children-with-statuses) removes that
friction. It is an **ergonomic enabler, not a hard blocker** (the pilot worked around it); the release's
GitHub story is gated on t40's ergonomics, not on a GitHub adapter. t40 is already high-priority.

## Dogfood plan (intent; mechanics -> planning)

Validate on this repo without disrupting the live dogfood flow: model a real GitHub issue as an mtt
task carrying a `github-gated` flow whose edges (a) spawn/relate the gated work, (b) block the terminal
until the gating condition passes (an `mtt` self-call check), (c) `gh issue close` on the terminal, and
(d) surface drift on a status read. Run at least one real end-to-end pass, record the clunky spots
(especially any that t40 would fix), and distil the config into the `github-gated` template. The
planning step details the exact flow config, the `gh` calls, and how the dogfood is scoped so it does
not perturb `.mtt/`.

## Acceptance

- A working `github-gated` init template committed (installable like the other flagship templates),
  demonstrably gating a GitHub issue's close on an mtt task's flow using only existing primitives + `gh`.
- At least one real end-to-end dogfood pass, with the clunk/friction documented — and an explicit,
  evidence-backed decision on whether any bespoke code (`mtt tt`) is warranted (with the finding, either
  way).
- Docs state the enforcement honesty (gate-the-agent; UI-close bypasses; detect+complain) and point at
  t40 as the ergonomic enabler.
- `make check` green; CHANGELOG updated; t9 (true store adapter) and the deferred items recorded as
  out-of-scope, not silently dropped.
