# internal/fsutil

Shared filesystem durability primitive. Nothing domain-specific lives here.

## Responsibilities

- `AtomicWrite(path, data, fallbackMode)` — temp-file + fsync + rename + parent-dir fsync. Perm policy:
  **preserve** an existing non-empty target's mode, else `fallbackMode` (a zero-byte target counts as absent).
  `MkdirAll` of the parent is the caller's job.
- `SyncDir(dir)` — best-effort parent-dir fsync (tolerates `ErrUnsupported`/`EINVAL`/Windows).

## Boundaries

- No `pkg/mtt` domain types, no `.mtt/` layout knowledge, no business rules — a pure durability helper.
- Extracted from the yaml adapter (was its private `atomicWrite`/`syncDir`); consumers: the yaml adapter
  (task/note/config/audit writes) and `internal/scaffold` (agent settings files).
- File-IO here carries an inline `// #nosec G304` (paths are locally resolved, not network input) — the
  yaml-adapter golangci exclusion does not reach this package.
