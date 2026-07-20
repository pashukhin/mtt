# internal/adapter/installer

Applies a self-update. Two effects, both behind `pkg/mtt` ports:

- **`BinaryReplacer`** (`NewReplacer`) — build-tagged: Unix (`replace_unix.go`)
  writes a same-dir temp + atomic `rename`; Windows (`replace_windows.go`) does
  rename-then-swap (remove stale `.old` → rename running exe aside → write new →
  rollback on failure). **The Windows path is unverified in CI** (no runner);
  isolated here, exercised in unit tests only on Unix.
- **`GoInstaller`** (`NewGoInstaller`) — shells `go install <module>@<tag>` and
  resolves the installed path from `go env GOBIN`/`GOPATH/bin`. `run`/`gobin` are
  seams so tests assert the argv without executing the toolchain.

No verification here — the core `SelfUpdater` checks the SHA-256 before Replace.
