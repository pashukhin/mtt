# internal/scaffold

Scaffolds agent-facing artifacts (config/hooks **and** docs) for a project. Onboarding infrastructure —
NOT tasks/knowledge domain.

## Responsibilities

- `Target` (seam) — a file + a pure `Merge(existing, exists) → (bytes, Action)` + a `Name`. Two producers:
  `HookTargets()` (t52; agent-tool config) and `DocTargets()` (t46; agent docs), both consumed by `Run`.
- `Run(root, targets)` — the thin IO edge (read → Merge → `fsutil.AtomicWrite`), write-as-you-go; only
  `os.ErrNotExist` marks a file absent.

### Hooks facet (t52)

- `claudeTarget.Merge` (pure) — additive, idempotent, no-clobber merge of `.claude/settings.json`:
  `SessionStart`+`PreCompact` → `mtt prime 2>/dev/null || true`, plus a read-only `permissions.allow`. One
  `json.Encoder` (`SetEscapeHTML(false)`, 2-space indent) — alphabetical keys, literal `2>/dev/null`.
  Only `claude` is registered; codex/gemini deferred until their config-hook formats are verified.

### Doc facet (t46)

- `DocTargets()` (`docs.go`) returns the agent-doc targets — `AGENTS.md` (the project-agnostic "Working under
  mtt" runbook) + `CLAUDE.md`/`GEMINI.md` (pointer stubs). Each `docTarget.Merge` delegates to the pure
  `blockMerge`: a `<!-- mtt:begin -->…<!-- mtt:end -->` block created / appended / **regenerated** (never
  clobbering surrounding content; a malformed marker pair refuses). Content is `//go:embed`-ed from `docdata/`
  (backticks preclude a Go raw literal).

## Boundaries

- No `pkg/mtt` domain, no `.mtt/` storage, no port. Writes via `internal/fsutil`.
- Hook presence is a word-boundary `mtt prime` prefix; a malformed / type-incompatible existing file is
  refused, never clobbered; a null/empty container is treated as absent. Doc merge preserves all content
  outside the markers.
