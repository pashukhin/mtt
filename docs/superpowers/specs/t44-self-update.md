# mtt self-update — update the installed binary in place (`t44`)

Status: spec (decision record). Type: task (`t44`). Branch: `task/t44`. Depends on: `t30` (versioning, terminal).

## Context / problem

Since `v0.9.0` mtt ships prebuilt binaries attached to a GitHub Release (`RELEASING.md`,
`.github/workflows/release.yml`): `make release VERSION=vX.Y.Z` cross-compiles the 5-platform matrix into
`dist/mtt_vX.Y.Z_<os>_<arch>[.exe]` + `dist/SHA256SUMS`, and a `v*`-tag run publishes them. A user who
installed a binary (downloaded an asset, or `go install …/cmd/mtt@vX.Y.Z`) has **no in-tool way to move to
the next release** — they must re-find the release page or re-run `go install`. `t44` closes that loop:
`mtt self-update` fetches the latest published release, **verifies it against `SHA256SUMS`**, and atomically
replaces the running binary — with a `go install` fallback when the platform has no published asset.

This is only meaningful now that `v0.9.0` is tagged and published (the primary asset+checksum path had nothing
to fetch before). `t44` depends on `t30`: the version machinery it reuses — `resolveVersion()` (ldflags →
`runtime/debug` module version → `"dev"`) — is `t30`'s, and the asset naming / `SHA256SUMS` format it consumes
is the release workflow's.

Non-negotiable constraints from AGENTS/DESIGN this design must satisfy:

- **Hexagon.** `cli → core → port ← adapter`; ports (interfaces) + domain types live in the public `pkg/mtt`;
  `core` imports no adapter and no network/OS-replace code.
- **No network in tests.** The GitHub HTTP call and the actual binary self-replace are both **side-effecting
  and non-hermetic**; they must sit behind ports that are **faked** in unit tests. The real self-replace and
  the real download are verified **manually** (a real-binary smoke on `impl_review`), never in `go test`.
- **TDD, SOLID, DRY, KISS.** Pure, tested logic (version compare, checksum verify, asset selection, the update
  *plan*) is separated from the two irreducible side effects (HTTP fetch, file replace / `go install`).

## User stories

Primary user = the coding **agent** (and the human maintainer) who has an installed `mtt` and wants the newest
release without leaving the terminal.

- **US1** — As a user, update my installed mtt to the latest release in one command, with the download
  integrity-checked. `mtt self-update`
- **US2** — As a user, ask whether an update is available without changing anything (scriptable/CI).
  `mtt self-update --check-only`
- **US3** — As a user on a dev build (or already on latest), be told clearly rather than silently no-op'd or
  wrongly downgraded — and be able to force the fetch when I mean it. `mtt self-update --force`
- **US4** — As a user on a platform with no published asset but a Go toolchain, still update via the source
  install path. (automatic fallback → `go install …/cmd/mtt@<latest>`)
- **US5** — As an agent, drive it from structured output. `mtt self-update --check-only --json`

## Decisions

### D1 — Mechanism: asset+SHA256 **primary**, `go install` **fallback** (both in v1)

- **Primary — release asset + checksum.** Resolve the latest release, map `runtime.GOOS/GOARCH` to the asset
  name `mtt_<tag>_<goos>_<goarch>[.exe]`, download that asset **and** `SHA256SUMS`, verify the asset's SHA-256
  against its `SHA256SUMS` line, then atomically replace the running binary. This is verifiable (we control the
  checksum) and needs no toolchain.
- **Fallback — `go install`.** When the resolved release has **no asset** matching the current platform (the
  computed name is absent from the release's asset list — e.g. a platform outside the 5-entry matrix) **and** a
  Go toolchain is on `PATH`, fall back to `go install github.com/pashukhin/mtt/cmd/mtt@<latest-tag>` (pinned to
  the resolved tag, **not** `@latest`, so we install exactly the version we resolved). Integrity there is the
  Go module checksum database, not our `SHA256SUMS`.
- **Rejected — `go install` as primary** (task frames asset as primary; needs Go; no checksum we control) and
  **asset-only, drop the fallback** (leaves no-asset platforms with only a manual path). Fallback is automatic,
  not a flag (KISS); a `--via` selector is YAGNI.
- **`go install` caveat (documented, not worked around):** `go install` writes to `GOBIN`/`$GOPATH/bin`, which
  may differ from the running binary's location. The fallback therefore installs the new version to the Go bin
  dir and **reports that path**; it does not pretend to have replaced an asset-installed binary elsewhere. This
  is honest and correct for the common case (a `go install`-ed mtt lives in `GOBIN`, so it is replaced in
  place).

### D2 — Ports: `ReleaseSource`, `BinaryReplacer`, `GoInstaller` (in `pkg/mtt`)

Three driven ports isolate the three non-hermetic effects; the `core` usecase depends only on these interfaces
(as it does on `TaskStore`/`KnowledgeStore`/`AuditStore`).

```go
// pkg/mtt (contract)
type ReleaseAsset struct { Name, URL string }
type Release       struct { Tag string; Assets []ReleaseAsset }

type ReleaseSource interface {
    Latest(ctx context.Context) (Release, error)     // GET releases/latest
    Fetch(ctx context.Context, url string) ([]byte, error) // download an asset / SHA256SUMS
}

type BinaryReplacer interface {
    // Replace atomically swaps the executable at path with newBinary (mode preserved).
    Replace(path string, newBinary []byte) error
}

type GoInstaller interface {
    // Install runs the toolchain install of module@version; returns the installed binary path.
    Install(ctx context.Context, module, version string) (path string, err error)
}
```

- These are **the only** new contract additions. `Release`/`ReleaseAsset` are plain domain-adjacent value
  types (no serialization tags — the adapter maps GitHub JSON into them).
- **Rejected — depend on a self-update library** (`inconshreveable/go-update`) for the replace: it would pull a
  transitive dependency for ~30 lines of platform code we can own and isolate behind `BinaryReplacer`
  (dep-minimalism, AGENTS). Noted as prior art for the Windows swap technique; not a dependency.

### D3 — GitHub adapter: `releases/latest`, injectable HTTP doer, no socket in tests

- **`internal/adapter/github`** implements `ReleaseSource` over
  `GET https://api.github.com/repos/pashukhin/mtt/releases/latest` (`Accept: application/vnd.github+json`),
  reading `tag_name` and `assets[].{name, browser_download_url}` into `Release`. `Fetch` GETs an asset URL and
  returns the bytes.
- **Testability without network:** the adapter takes an injectable **`httpDoer`** (`interface{ Do(*http.Request)
  (*http.Response, error) }`, satisfied by `*http.Client`). Tests inject a fake doer returning canned JSON /
  asset bytes — **no sockets, honoring "No network in tests" literally** (the repo has no `httptest` usage; this
  keeps it that way). The default client carries a timeout (a package constant); the whole call is
  `context`-bounded.
- The repo/owner (`pashukhin/mtt`) is a package constant derived from the module path; not user-configurable in
  v1 (YAGNI).

### D4 — Version resolution & comparison (reuse `t30`; `x/mod/semver`)

- **Current version** is resolved by the **CLI** via the existing `resolveVersion()` and **passed into** the
  core `Plan` as a string (core imports no `internal/cli`; stays pure and platform-injectable). `runtime.GOOS/
  GOARCH` are likewise passed in.
- **Comparison** uses `golang.org/x/mod/semver` (see D10) — canonical SemVer ordering:
  - Current is **valid SemVer** and `latest > current` → update. `latest <= current` → **no-op** ("already up
    to date"), unless `--force`.
  - Current is **not valid SemVer** (e.g. `"dev"` from a plain `go build`) → **refuse** without `--force`
    ("cannot determine the current version; re-run with --force to update to `<latest>` anyway"); `--force`
    proceeds.
  - **Locally-built binaries update cleanly without `--force`:** `make build` stamps `git describe --tags`
    (e.g. `v0.9.0-5-gf7a03cc`), which is **valid SemVer** and, by the pre-release rule, **orders *below***
    `v0.9.0` — so a dev checkout ahead of the tag sees the release as newer and updates. This is exactly the
    smoke path (a locally built mtt → published `v0.9.0`).
- **`--force`** bypasses **both** guards (invalid-current refusal **and** the not-newer no-op): it always
  installs the resolved **latest** — a re-install when equal, and the explicit override when the current version
  can't be ordered. (A "downgrade" is a non-case: `latest` *is* the highest published tag; without `--force`
  mtt never installs a non-newer version.)

### D5 — Asset selection + checksum verify (pure, before any write)

- **`assetName(tag, goos, goarch)`** (pure) → `mtt_<tag>_<goos>_<goarch>` + `.exe` when `goos == "windows"`,
  mirroring `make release` exactly. The name is looked up in the **release's actual asset list** (not a
  hardcoded matrix); absent → the go-install fallback (D1) or, if no Go, an actionable error.
- **`verifyChecksum(assetName, assetBytes, sha256sumsBytes)`** (pure) parses `SHA256SUMS` lines
  (`<hex64>␠␠<name>`, the `sha256sum` format), finds `assetName`, compares a freshly computed
  `sha256.Sum256(assetBytes)` (hex, case-insensitive). Name absent from `SHA256SUMS` → error ("asset not listed
  in SHA256SUMS"); hash mismatch → error. **Verification happens on the full in-memory buffer before the
  replacer is ever called** — a mismatch aborts with the original binary intact. (Binaries are ~7 MB; buffering
  in memory is fine, and avoids a download-to-temp-then-verify dance.)

### D6 — Atomic self-replace: Unix rename-over; Windows rename-then-swap

- **Target path** = `filepath.EvalSymlinks(os.Executable())` (replace the real file a shim points at).
- **Writability precheck:** if the target's directory is not writable → a clear error ("cannot write
  `<dir>` — re-run with adequate permissions or update manually"), **no auto-`sudo`**.
- **Unix (`//go:build !windows`):** write `newBinary` to a temp file **in the same directory** (`O_EXCL`),
  `chmod` it to the target's current mode, `fsync`, then `os.Rename(temp, target)` — atomic on one filesystem;
  the running process keeps its open inode. Any pre-rename failure removes the temp; target untouched.
- **Windows (`//go:build windows`):** a running `.exe` can be **renamed** but not overwritten/deleted. So
  `os.Rename(target, target+".old")` (moves the running image aside — permitted), then write `newBinary` to
  `target`; on failure, rename `.old` back. Leave `.old` for best-effort cleanup on a later run (the running
  process still holds it open). This is the standard swap technique.
- **Verification status:** the Windows path is **implemented but not verifiable in this environment** (no
  Windows host/CI runner). It is isolated behind `BinaryReplacer`, exercised in unit tests only via the fake,
  and flagged for real verification on a Windows host before it is trusted. Recorded risk (maintainer chose
  Unix + Windows over Unix-only).
- `BinaryReplacer` is selected by build tag at construction; the plan pins the exact package/file layout.

### D7 — `go install` fallback path

- Triggered only when D5's asset lookup misses **and** `exec.LookPath("go")` succeeds. `GoInstaller.Install`
  runs `go install github.com/pashukhin/mtt/cmd/mtt@<tag>` and returns the resulting binary path (probe
  `go env GOBIN` then `$GOPATH/bin`). No asset/checksum step (Go's module sum DB is the integrity mechanism).
- The command reports: `installed <tag> via go install → <path>` plus, when `<path>` differs from the running
  binary, a one-line note that the updated binary is at the Go bin dir.
- Missing asset **and** no Go → actionable error listing both facts (exit 1).

### D8 — CLI surface, flags, exit codes

```
mtt self-update [--check-only] [--force] [--json]
```

- **`mtt self-update`** — the full plan+apply (D1–D7).
- **`--check-only`** — run `Plan` only (resolve + compare + asset/fallback selection); print `current`,
  `latest`, and whether an update is available; **write nothing**. **Exit 0** regardless (availability is in the
  output, not the exit code — deliberately *unlike* `mtt check`'s exit 7, because `self-update --check-only` is
  an informational query, not a repo-integrity gate). Recorded decision.
- **`--force`** — D4 semantics (bypass invalid-current refusal and the not-newer no-op).
- **`--json`** — structured output (every mtt command honors `--json`, the `t45` discipline):
  `{current, latest, update_available, updated, asset, via}` where `via ∈ {"asset","go-install"}` and
  `updated`/`asset` are populated on an applied update (empty/false under `--check-only` or a no-op).
- **Thin CLI** (`internal/cli/selfupdate.go`): resolve current (`resolveVersion()`), target
  (`EvalSymlinks(os.Executable())`), `runtime.GOOS/GOARCH`; construct the github adapter (real `http.Client` +
  timeout), the platform `BinaryReplacer`, the `GoInstaller`; call `core.SelfUpdater`; render text or JSON.
- **Exit codes:** success / no-op / check-only → **0**; every failure (network/API, checksum mismatch, no
  asset + no Go, dev build without `--force`, target not writable, replace failure) → **1** with an actionable
  message. No new taxonomy code (this command is not a gate). `--json` on an error still exits 1.

### D9 — Core usecase `SelfUpdater`: `Plan` then `Apply`

- **`Plan(ctx, current, goos, goarch string, force bool, src ReleaseSource) (Plan, error)`** — pure but for the
  one `src.Latest(ctx)` call: resolve latest, compare (D4), select asset or fallback (D5/D7). Returns a `Plan`
  describing the decision (`NoUpdate` / `Via: asset|goInstall` / tag / asset name+URL / checksums URL). Used
  directly by `--check-only`.
- **`Apply(ctx, plan, src, replacer, installer, targetPath) (Result, error)`** — asset: `src.Fetch(asset)` +
  `src.Fetch(SHA256SUMS)` → `verifyChecksum` → `replacer.Replace(targetPath, bytes)`; go-install:
  `installer.Install(module, tag)`. Never replaces on a verify failure.
- Pure helpers (`assetName`, `verifyChecksum`, `isNewer`) are unit-tested standalone. The usecase is tested with
  fake `ReleaseSource`/`BinaryReplacer`/`GoInstaller`.

### D10 — Dependencies

- **`golang.org/x/mod/semver`** (new, direct) for version comparison — the canonical SemVer implementation (the
  same logic the `go` command uses), tiny and pure-Go; avoids hand-rolling pre-release ordering (the exact edge
  case D4 relies on: `v0.9.0-5-g… < v0.9.0`). Justified per AGENTS ("justify any new dependency"). **Rejected —
  hand-rolled `vX.Y.Z` compare:** simpler dep tree but re-implements pre-release ordering, which is subtle and
  security-adjacent (deciding "is this newer"). Recorded; open to reversal at spec review.
- **`net/http`** (stdlib) for the github adapter.

## Scope

**In:** the three ports (`ReleaseSource`/`BinaryReplacer`/`GoInstaller`) in `pkg/mtt`; the github HTTP adapter
(injectable doer); the platform `BinaryReplacer` (Unix + Windows) and the `GoInstaller`; the core `SelfUpdater`
(`Plan`/`Apply`) + pure helpers (`assetName`/`verifyChecksum`/`isNewer`); `mtt self-update` with
`--check-only`/`--force`/`--json`; unit tests (core + adapters, all hermetic) + the non-network CLI e2e cases;
the real-binary smoke as an `impl_review` acceptance step; docs sync.

**Out:**
- **Windows real verification** — implemented, but proven only on a Windows host later (isolated, unit-faked).
- **Auto/background update** (session-start nag, `prime`-style prompt) → follow-up.
- **Pinning an arbitrary version** (`self-update` targets *latest* only; `go install …@vX.Y.Z` is the manual
  escape for a specific version).
- **A user-facing `--rollback` command** (the `.old`/temp is internal safety, not a surface).
- **GPG/signature verification** (SHA-256 from the release is v1 integrity; signing is future hardening).
- **URL/owner configurability** and **`--via` selector** → YAGNI.

## Acceptance criteria

1. **Plan (unit):** current `v0.9.0-3-gabc` (valid, pre-release) + latest `v0.9.0` → update via **asset**;
   current `v0.9.0` + latest `v0.9.0` → **no-op** (unless `--force`); current `"dev"` + latest `v0.9.0` →
   **refuse** without `--force`, **update** with `--force`; latest older than current → no-op without `--force`.
2. **Asset selection (unit):** `assetName("v0.9.0","linux","amd64") == "mtt_v0.9.0_linux_amd64"`;
   `…,"windows","amd64" == "mtt_v0.9.0_windows_amd64.exe"`; a platform absent from the release's asset list →
   **go-install** when Go present, else an actionable error.
3. **Checksum verify (unit):** matching bytes → ok; a one-byte change → error and **`Replace` is never called**;
   asset name absent from `SHA256SUMS` → error; malformed `SHA256SUMS` line → error.
4. **Apply asset (unit, fakes):** fake `ReleaseSource` serves asset + `SHA256SUMS`; on success the fake
   `BinaryReplacer` receives `(targetPath, assetBytes)`; on checksum mismatch it receives nothing.
5. **Apply go-install (unit, fake):** no matching asset + Go present → fake `GoInstaller.Install` called with
   `("github.com/pashukhin/mtt/cmd/mtt", "<tag>")`; returned path surfaced in the result.
6. **github adapter (unit, fake doer):** canned `releases/latest` JSON → `Release{Tag, Assets}` parsed;
   `Fetch(url)` returns the canned bytes; **no socket opened**.
7. **Unix replacer (unit, temp dir):** replacing a throwaway file swaps its contents and preserves mode;
   a same-dir temp is used (atomic rename), original untouched on an injected write failure.
8. **CLI e2e (testscript, hermetic):** `self-update --json` on a **dev** build without `--force` → refusal
   message, exit 1, valid JSON error; flag/usage errors; `--check-only --json` shape on the dev-refusal path.
   (The happy path needs network → not e2e; covered by AC-9.)
9. **Real-binary smoke (manual, `impl_review`):** build mtt locally (`make build` → a `v0.9.0-N-g…` stamp),
   run `./bin/mtt self-update` → it resolves `v0.9.0`, downloads the linux asset, verifies SHA-256, replaces
   the binary, and the replaced binary prints `mtt version` → `v0.9.0`. Re-running → "already up to date"
   (exit 0). `--force` re-installs. This is the proof the primary path works end-to-end.
10. `make check` green. Docs synced (below).

## Testing approach

- **Unit (`internal/core`, hermetic, table-driven):** `Plan` (AC-1/2), `verifyChecksum` (AC-3), `isNewer`,
  `assetName`; `Apply` asset + go-install with fakes (AC-4/5) — asserting **no replace on verify failure**.
- **Unit (`internal/adapter/github`):** fake `httpDoer` → JSON parse + asset URL selection + `Fetch` (AC-6). No
  sockets.
- **Unit (`internal/adapter/…` replacer):** Unix temp-dir replace + mode preservation + failure leaves original
  (AC-7); the Windows impl compiles under its build tag but is not run here (recorded).
- **e2e (`internal/cli`, testscript):** only the **no-network** cases — dev-refusal, usage errors, `--json`
  error/shape (AC-8). No network, per AGENTS.
- **Manual smoke (`impl_review`):** AC-9 against the real `v0.9.0` release.

## Docs to sync (docs-sync judgment, `impl_review`)

Grep **all** parallel occurrences (EN + RU) before editing — the "parallel occurrences" trap.

- **`CLI_REFERENCE.md ↔ .ru.md`:** a new `mtt self-update` entry (flags, `--check-only` exit-0 semantics,
  `--force`, the go-install fallback + its GOBIN caveat, integrity via `SHA256SUMS`).
- **`DESIGN.md ↔ .ru.md`:** a short "Self-update" note under the packaging/release material — mechanism
  (asset primary + go-install fallback), verify-before-replace invariant, the platform-replace note (Unix
  atomic rename; Windows rename-then-swap, unverified), and the three ports in the hexagon map.
- **`RELEASING.md`:** one line closing the loop — the published assets + `SHA256SUMS` are what `mtt self-update`
  consumes.
- **`CHANGELOG.md`** `[Unreleased]` → **Added:** `mtt self-update`.
- **`docs/architecture/model.go`:** the new ports (`ReleaseSource`/`BinaryReplacer`/`GoInstaller`) noted as
  contract, `SelfUpdater` as a `core` usecase.
- **CLAUDE.md files:** new adapter package(s) (`github`, and the replacer/go-installer package); `internal/core`
  (`SelfUpdater`, pure helpers); `internal/cli` (`self-update`); `pkg/mtt` (the new ports). Keep each thin.
- **`AGENTS.md`:** no new flow rule expected; touch only if a convention changes (it should not).

## Sequencing & tracking (process, not code)

`t44` is `speccing` on `task/t44`. This document is the `speccing` deliverable. Next: commit it, run an
adversarial subagent **spec review**, address findings, then `spec_human_review` (maintainer sign-off) →
`planning` (writing-plans) → `plan_review` → `plan_human_review` → TDD `implementing` → `impl_review`
(including the AC-9 real-binary smoke) → `approved` (auto PR) → merge → `deliver`.
