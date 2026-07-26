---
title: Docs-guard scope + the fenced-example validation gotcha
tags:
    - docs
    - tests
priority: medium
created: "2026-07-26T05:54:11Z"
updated: "2026-07-26T05:54:11Z"
---
The t42 docs-guard test (`internal/cli/docs_guard_test.go`, in `make check`) guards two drift classes:
documented-command existence (registry <-> CLI_REFERENCE) and a hardcoded README version badge. It does
NOT guard **structural markdown validity** (glued fences / glued YAML list items in fenced examples) nor
**EN<->RU prose parity**, and its phantom-token scan deliberately excludes FLOW_GUIDE (it teaches custom
flows). So a docs sweep still needs a manual full-file pass for those.

Non-obvious gotcha that bit t42: when validating a fenced code example, do NOT extract it with a non-greedy
regex like ```` ```yaml\n(.*?)``` ````. If the closing fence is glued onto the last content line
(`...cancelled}` immediately followed by three backticks), the non-greedy `.*?` stops at that glued fence and
captures the VALID remainder, so `yaml.safe_load` reports "parses OK" while the rendered doc is broken (the
block never closes; following prose is swallowed). Extract CommonMark-correctly instead: from the opening
`` ```lang `` line to the next line that is *exactly* a bare fence. The adversarial impl review caught the
glued fence that this false-positive had hidden.
