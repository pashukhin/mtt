---
title: 'Tag conventions: backlog, think, and the thematic vocabulary'
tags:
    - process
priority: high
created: "2026-07-23T07:58:05Z"
updated: "2026-07-23T07:58:05Z"
---
- backlog = not in the live queue. Every deferred task carries it; PROMOTING a task = mtt tag rm <id>
  backlog. The live queue = open tasks minus backlog; mtt roadmap is the "what next" view (low-priority
  backlog sinks), mtt list --tag backlog the backlog view.
- think = design-open item (usually "Think:"-titled): brainstorm before implementing; drop the tag once
  the design is decided.
- Thematic tags are a deliberately SMALL vocabulary - currently core, flow, sec, tests, perf, dx, ux,
  kb, adapter, demo, multiagent, release, docs. Pick from the existing set before inventing; discover
  the live set with mtt tags (--all for every task, --json for {tag,count}).
- Caveat: #hashtags in titles/descriptions auto-become tags - never put # in a title unless you mean
  it (a "#2" in a migration title once minted a spurious "2" tag).
