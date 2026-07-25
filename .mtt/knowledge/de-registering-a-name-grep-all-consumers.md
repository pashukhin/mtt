---
title: 'Removing a built-in name/token: grep every consumer form, not just the surface string'
tags:
    - process
    - tests
priority: medium
created: "2026-07-25T13:45:27Z"
updated: "2026-07-25T13:45:27Z"
---
When de-registering a value callers pass as a bare token — a built-in `--template` name, a flag value, a status/type name — a grep for the SURFACE string (e.g. `--template hierarchy`) is BLIND to the forms that only break at compile/test time:

- Go tests pass CLI flags as SEPARATE args: `runRoot(t, "init", "--template", "hierarchy")` — no `--template hierarchy` substring exists.
- Adapter/API tests call directly: `Init(root, "hierarchy", …)`, and table loops like `[]string{"default","coding","hierarchy"}`.

An adversarial plan review caught this in t62 as a near-BLOCKER: the `--template hierarchy` grep undercounted the migration by ~9 Go test files (incl. a shared `initHierarchy` helper behind ~20 tests) that would have reddened `make check` on de-embed.

**How to apply:** enumerate consumers by the VALUE-IN-QUOTES (`"hierarchy"`, `"coding"`) AND the API entry points, sweeping `*_test.go` too — not just the CLI string. Chokepoint the migration through shared test helpers (one `initHierarchy` edit covered ~20 tests). This is the code/test analogue of the docs "grep all parallel occurrences" discipline. Second-order lesson: a text/template file (`{{.Name}}`) is NOT valid standalone YAML — when moving it to a verbatim path, de-template it to a literal first.