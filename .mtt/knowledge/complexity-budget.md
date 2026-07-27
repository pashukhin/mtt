---
title: 'Complexity budget: what may enter the core'
tags:
    - process
priority: high
created: "2026-07-27T15:49:34Z"
updated: "2026-07-27T15:49:34Z"
---
# Complexity budget — what may enter the core

Adopted 2026-07-27 (pre-t60 reflection), adapted from an external review. Governance to stop the core from re-monstering after 1.0.

1. **Core-or-extension test.** A new capability enters the core only if it strengthens enforcement of "done" OR human legibility of the true state (the release-scope-1.0 criterion). Otherwise it lives as an extension / hook / sibling tool.
2. **No second way.** Do not add a second syntactic form for an operation that already has one without a measured ergonomic win.
3. **One graph in, one out.** A new graph or relationship in the domain model must remove or simplify an existing one — do not accumulate parallel structures.
4. **Docs separate real from planned.** The CLI reference must not describe target / parked surface next to the implemented API.
5. **Prefer freeze to amputation** for working, shipped, low-maintenance code (e.g. self-update): stop growing it, but do not rip it out for purity — that is itself anti-lean.
6. **The two-page test.** Each tool must be explainable — and its everyday use runnable — from a two-page essential spec. If a normal task needs the user to know about the current-pointer + sugar forms + refs + prime + roles + bulk selectors + event pipelines + config layering all at once, the abstraction has stopped hiding complexity.
