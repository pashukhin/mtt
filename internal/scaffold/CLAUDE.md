# internal/scaffold

Scaffolds agent-facing configuration (hooks + permissions) for supported harnesses. Onboarding
infrastructure — NOT tasks/knowledge domain.

## Responsibilities

- `Harness` (seam) + `Registry()` — the extension point. Only `claude` is registered; codex/gemini are
  deferred until their config-hook formats are verified (no guessed config is written).
- `claudeHarness.Merge` (pure) — additive, idempotent, no-clobber merge of `.claude/settings.json`:
  `SessionStart`+`PreCompact` → `mtt prime 2>/dev/null || true`, plus a read-only `permissions.allow`. One
  `json.Encoder` (`SetEscapeHTML(false)`, 2-space indent) — alphabetical keys, literal `2>/dev/null`.
- `Run(root, harnesses)` — the thin IO edge (read → Merge → `fsutil.AtomicWrite`), write-as-you-go.

## Boundaries

- No `pkg/mtt` domain, no `.mtt/` storage, no port. Writes via `internal/fsutil`.
- Presence is a word-boundary `mtt prime` prefix; malformed / type-incompatible existing files are refused,
  never clobbered; a null/empty container is treated as absent.
