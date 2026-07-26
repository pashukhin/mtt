---
title: 'Encoding shell commands into config JSON: SetEscapeHTML(false) + one serializer'
tags:
    - go
    - gotcha
    - json
priority: medium
created: "2026-07-26T00:09:39Z"
updated: "2026-07-26T00:09:39Z"
---
Go's encoding/json HTML-escapes <, >, & by default, so json.Marshal/MarshalIndent turns a shell command like `mtt prime 2>/dev/null` into `mtt prime 2>/dev/null`. When you emit config JSON that carries shell metacharacters (redirects `>`/`<`, a `&&` chain), use a json.Encoder with SetEscapeHTML(false) (+ SetIndent) — it keeps the literal bytes. Encoder.Encode also appends a trailing newline (do not double-add).

Companion idempotency rule for a map-based merge-and-rewrite: encoding/json sorts object keys alphabetically, so a re-run only converges to a byte-identical "unchanged" if the CREATE path and the MERGE path share ONE serializer building the same map[string]any. A struct/literal on create + map-marshal on merge would reorder keys and report "merged" forever.

Both were caught by the t52 adversarial plan review (round 2) against the .claude/settings.json scaffold, not by make check — a golden/idempotency test that asserts the exact literal bytes (incl. `2>/dev/null`) pins them.