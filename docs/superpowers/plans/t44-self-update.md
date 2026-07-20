# mtt self-update (t44) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `mtt self-update` — replace the running binary with the latest published GitHub release (asset + SHA256 verify), falling back to `go install` when no verifiable asset matches the platform.

**Architecture:** Hexagonal. Three new driven ports in `pkg/mtt` (`ReleaseSource`, `BinaryReplacer`, `GoInstaller`) isolate the three non-hermetic effects (HTTP, self-replace, `go install`). A pure `core.SelfUpdater` (`Prepare` → a determinate `Plan`; `Apply` → fetch/verify/replace or go-install) holds all logic and is tested with fakes. Adapters: `internal/adapter/github` (HTTP, injectable doer) and `internal/adapter/installer` (platform replacer + go-installer). The CLI wires the real adapters and renders text/JSON.

**Tech Stack:** Go 1.23+, cobra CLI, `golang.org/x/mod/semver` (new), stdlib `net/http`/`crypto/sha256`, `testscript` (txtar e2e), table-driven unit tests.

## Global Constraints

- **Spec of record:** `docs/superpowers/specs/t44-self-update.md`. Every decision (D1–D10) is binding.
- **TDD:** red → green → refactor. Failing test first, watch it fail, then implement. `make check` (gofmt + vet + golangci-lint v2 + `go test -race -cover` + build) green before every commit.
- **Layering:** `core` imports **no** `adapter/*` and **no** `internal/cli`; `core` never reads `os.Executable`/`PATH`/network (all injected). `pkg/mtt` domain carries **no** yaml/json tags. Storage/effects only through ports.
- **No network in tests.** `ReleaseSource`, `BinaryReplacer`, `GoInstaller` are faked in unit tests; the github adapter is tested with a fake `httpDoer` (no socket). The real download + self-replace are verified only by the **manual real-binary smoke** (Task 9 / `impl_review`).
- **Determinism (B2):** `Prepare` returns an error **only** when `src.Latest(ctx)` fails; every version/asset outcome is a determinate `Plan` state — `UpdateAvailable` / `NoUpdate` / `Undetermined`, with `Via ∈ {asset, go-install, none}`.
- **Asset naming (verbatim from `make release`):** `mtt_<tag>_<goos>_<goarch>`, `+ ".exe"` iff `goos=="windows"`. Checksums asset is named exactly `SHA256SUMS` (format `<hex64>␠␠<name>`). The asset path needs **both** the platform asset **and** `SHA256SUMS`.
- **Version reuse (t30):** current version = the CLI's `resolveVersion()`; comparison via `semver` (a dev/bare-SHA current is not valid semver → `Undetermined` unless `--force`).
- **Exit codes:** success / `NoUpdate` / any `--check-only` resolvable state → **0**; genuine failures + apply-path refusals (`Undetermined`-sans-`--force`, `Via:none`) → **1**. No new taxonomy code (`exitCode`'s default `1` covers it).
- **`--json` schema (pinned):** `{current, latest, update_available, updated, via, asset, path, reason, error}`, `via ∈ {"","asset","go-install","none"}`.
- **Module path:** `github.com/pashukhin/mtt/cmd/mtt` (the go-install target); owner/repo `pashukhin/mtt`.
- **Docs bilingual** (EN + RU) where applicable: `DESIGN`, `CLI_REFERENCE`. Grep all parallel occurrences before editing.

---

## File structure

**Create:**
- `pkg/mtt/release.go` — `Release`/`ReleaseAsset` value types + the 3 ports.
- `internal/core/selfupdate.go` — `SelfUpdater` (`Prepare`/`Apply`), `Plan`/`Result`/`UpdateState`/`UpdateVia`, pure `assetName`/`verifyChecksum`/`isNewer`/`Orderable`, consts.
- `internal/core/selfupdate_test.go` — pure-helper + `Prepare` + `Apply` tests (fakes).
- `internal/adapter/github/github.go` — `Source` (`ReleaseSource` over HTTP, injectable `httpDoer`).
- `internal/adapter/github/github_test.go` — fake-doer parse/fetch tests.
- `internal/adapter/github/CLAUDE.md`.
- `internal/adapter/installer/replace.go` — `replacer` struct + `NewReplacer` + `NewGoInstaller` constructors.
- `internal/adapter/installer/replace_unix.go` (`//go:build !windows`) — Unix `Replace`.
- `internal/adapter/installer/replace_windows.go` (`//go:build windows`) — Windows `Replace`.
- `internal/adapter/installer/goinstall.go` — `goInstaller` (`GoInstaller`).
- `internal/adapter/installer/installer_test.go` — Unix replace + go-install (fakes) tests.
- `internal/adapter/installer/CLAUDE.md`.
- `internal/cli/selfupdate.go` — `newSelfUpdateCmd` + `selfUpdateJSON` + render helpers.
- `internal/cli/selfupdate_test.go` — JSON-view + short-circuit unit tests.
- `internal/cli/testdata/scripts/selfupdate.txt` — hermetic e2e (usage + dev refusal).

**Modify:**
- `internal/cli/root.go` — register `newSelfUpdateCmd()`.
- `go.mod` / `go.sum` — add `golang.org/x/mod`.
- `pkg/mtt/CLAUDE.md`, `internal/core/CLAUDE.md`, `internal/cli/CLAUDE.md` — new ports/usecase/command.
- `docs/architecture/model.go` — note the 3 ports + `SelfUpdater`.
- `CLI_REFERENCE.md` ↔ `.ru.md`, `DESIGN.md` ↔ `.ru.md`, `RELEASING.md`, `CHANGELOG.md`.

---

## Task 1: dep + pure helpers (`isNewer`, `Orderable`, `assetName`)

**Files:**
- Create: `internal/core/selfupdate.go`
- Test: `internal/core/selfupdate_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces:
  - `func Orderable(v string) bool` — true iff `v` is valid SemVer (so it can be compared).
  - `func isNewer(latest, current string) bool` — true iff both valid SemVer and `latest > current`.
  - `func assetName(tag, goos, goarch string) string` — `mtt_<tag>_<goos>_<goarch>[.exe]`.
  - consts `selfUpdateModule = "github.com/pashukhin/mtt/cmd/mtt"`, `checksumsAsset = "SHA256SUMS"`.

- [ ] **Step 1: Add the dependency**

Run:
```bash
go get golang.org/x/mod@latest && go mod tidy
```
Expected: `golang.org/x/mod` appears in `go.mod`'s `require` block; `go.sum` updated.

- [ ] **Step 2: Write the failing test** — `internal/core/selfupdate_test.go`:

```go
package core

import "testing"

func TestOrderable(t *testing.T) {
	cases := map[string]bool{
		"v0.9.0": true, "v0.9.0-5-gf7a03cc": true, "v0.9.0-5-gf7a03cc-dirty": true,
		"dev": false, "6bf290d": false, "": false,
	}
	for in, want := range cases {
		if got := Orderable(in); got != want {
			t.Fatalf("Orderable(%q) = %v want %v", in, got, want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.9.0", "v0.9.0-5-gf7a03cc", true}, // release > its pre-release
		{"v0.9.0", "v0.9.0", false},           // equal
		{"v0.9.0", "v1.0.0", false},           // older
		{"v0.9.0", "dev", false},              // current unorderable
		{"v0.10.0", "v0.9.0", true},
	}
	for _, c := range cases {
		if got := isNewer(c.latest, c.current); got != c.want {
			t.Fatalf("isNewer(%q,%q) = %v want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	if got := assetName("v0.9.0", "linux", "amd64"); got != "mtt_v0.9.0_linux_amd64" {
		t.Fatalf("linux: %q", got)
	}
	if got := assetName("v0.9.0", "windows", "amd64"); got != "mtt_v0.9.0_windows_amd64.exe" {
		t.Fatalf("windows: %q", got)
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `go test ./internal/core/ -run 'Orderable|IsNewer|AssetName'`
Expected: FAIL — undefined `Orderable`/`isNewer`/`assetName`.

- [ ] **Step 4: Implement** — create `internal/core/selfupdate.go`:

```go
package core

import (
	"fmt"

	"golang.org/x/mod/semver"
)

const (
	// selfUpdateModule is the go-install target for the fallback path.
	selfUpdateModule = "github.com/pashukhin/mtt/cmd/mtt"
	// checksumsAsset is the exact name of the checksums file in a release.
	checksumsAsset = "SHA256SUMS"
)

// Orderable reports whether v is valid SemVer and can therefore be compared. A
// dev build ("dev") or a bare commit SHA ("6bf290d") is not orderable.
func Orderable(v string) bool { return semver.IsValid(v) }

// isNewer reports whether latest is a strictly newer SemVer than current. A
// non-orderable current (or latest) yields false (the caller handles that case).
func isNewer(latest, current string) bool {
	if !semver.IsValid(latest) || !semver.IsValid(current) {
		return false
	}
	return semver.Compare(latest, current) > 0
}

// assetName mirrors `make release`: mtt_<tag>_<goos>_<goarch>, plus ".exe" on
// Windows.
func assetName(tag, goos, goarch string) string {
	name := fmt.Sprintf("mtt_%s_%s_%s", tag, goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/core/ -run 'Orderable|IsNewer|AssetName'`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/core/selfupdate.go internal/core/selfupdate_test.go
git commit -m "t44: core self-update pure helpers (isNewer/Orderable/assetName) + x/mod/semver dep"
```

---

## Task 2: pure `verifyChecksum`

**Files:**
- Modify: `internal/core/selfupdate.go`
- Test: `internal/core/selfupdate_test.go` (extend)

**Interfaces:**
- Produces: `func verifyChecksum(name string, assetBytes, sha256sums []byte) error` — parses `SHA256SUMS`, finds `name`, compares `sha256.Sum256(assetBytes)` (hex, case-insensitive). Absent name / mismatch → error.

- [ ] **Step 1: Write the failing test** — append to `internal/core/selfupdate_test.go`:

```go
import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func sums(name string, data []byte, extra ...string) []byte {
	sum := sha256.Sum256(data)
	lines := []string{fmt.Sprintf("%s  %s", hex.EncodeToString(sum[:]), name)}
	lines = append(lines, extra...)
	return []byte(strings.Join(lines, "\n") + "\n")
}

func TestVerifyChecksum(t *testing.T) {
	asset := []byte("the-binary-bytes")
	name := "mtt_v0.9.0_linux_amd64"

	if err := verifyChecksum(name, asset, sums(name, asset)); err != nil {
		t.Fatalf("match must pass: %v", err)
	}
	// one-byte change -> mismatch
	if err := verifyChecksum(name, []byte("the-binary-byteX"), sums(name, asset)); err == nil {
		t.Fatal("mismatch must error")
	}
	// name absent from SHA256SUMS -> error
	if err := verifyChecksum("mtt_v0.9.0_darwin_arm64", asset, sums(name, asset)); err == nil {
		t.Fatal("absent name must error")
	}
	// garbage / malformed sums (no usable line for name) -> error
	if err := verifyChecksum(name, asset, []byte("garbage-no-columns\n")); err == nil {
		t.Fatal("malformed sums must error")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/core/ -run 'VerifyChecksum'`
Expected: FAIL — undefined `verifyChecksum`.

- [ ] **Step 3: Implement** — append to `internal/core/selfupdate.go`:

```go
import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	// (keep "fmt" and "golang.org/x/mod/semver" from Task 1)
)

// verifyChecksum recomputes the SHA-256 of assetBytes and checks it against the
// line for name in a sha256sum-format SHA256SUMS ("<hex>  <name>"). An absent
// name or a mismatch is an error — the caller MUST verify before any replace.
func verifyChecksum(name string, assetBytes, sha256sums []byte) error {
	want, ok := findChecksum(name, sha256sums)
	if !ok {
		return fmt.Errorf("asset %q not listed in %s", name, checksumsAsset)
	}
	sum := sha256.Sum256(assetBytes)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %q: got %s, want %s", name, got, want)
	}
	return nil
}

// findChecksum returns the hex digest recorded for name, if present. It tolerates
// the sha256sum "binary mode" '*' name prefix; malformed lines are skipped.
func findChecksum(name string, sha256sums []byte) (string, bool) {
	sc := bufio.NewScanner(bytes.NewReader(sha256sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		if strings.TrimPrefix(fields[1], "*") == name {
			return fields[0], true
		}
	}
	return "", false
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/core/ -run 'VerifyChecksum'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/selfupdate.go internal/core/selfupdate_test.go
git commit -m "t44: core verifyChecksum (SHA256SUMS parse + compare, verify-before-replace)"
```

---

## Task 3: ports (`pkg/mtt/release.go`) + `SelfUpdater.Prepare`

**Files:**
- Create: `pkg/mtt/release.go`
- Modify: `internal/core/selfupdate.go` (types + `Prepare`)
- Test: `internal/core/selfupdate_test.go` (extend — a fake `ReleaseSource`)

**Interfaces:**
- Produces (`pkg/mtt`):
  - `type ReleaseAsset struct { Name, URL string }`
  - `type Release struct { Tag string; Assets []ReleaseAsset }`
  - `type ReleaseSource interface { Latest(ctx context.Context) (Release, error); Fetch(ctx context.Context, url string) ([]byte, error) }`
  - `type BinaryReplacer interface { Replace(path string, newBinary []byte) error }`
  - `type GoInstaller interface { Install(ctx context.Context, module, version string) (path string, err error) }`
- Produces (`core`):
  - `type UpdateState string` (`UpdateAvailable`/`NoUpdate`/`Undetermined`)
  - `type UpdateVia string` (`viaUnset=""`/`ViaAsset="asset"`/`ViaGoInstall="go-install"`/`ViaNone="none"`)
  - `type Plan struct { Current, Latest string; State UpdateState; Via UpdateVia; Tag, AssetName, AssetURL, ChecksumsURL, Reason string }`
  - `type SelfUpdater struct{}` + `func NewSelfUpdater() *SelfUpdater`
  - `func (u *SelfUpdater) Prepare(ctx context.Context, current, goos, goarch string, goAvailable, force bool, src mtt.ReleaseSource) (Plan, error)`

- [ ] **Step 1: Write the failing test** — append to `internal/core/selfupdate_test.go` (add a fake source):

```go
import (
	"context"
	"errors"
	// ...existing
	"github.com/pashukhin/mtt/pkg/mtt"
)

type fakeSource struct {
	rel     mtt.Release
	latErr  error
	fetched map[string][]byte
	fetchErr error
}

func (f *fakeSource) Latest(context.Context) (mtt.Release, error) { return f.rel, f.latErr }
func (f *fakeSource) Fetch(_ context.Context, url string) ([]byte, error) {
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	return f.fetched[url], nil
}

func relWith(tag string, names ...string) mtt.Release {
	r := mtt.Release{Tag: tag}
	for _, n := range names {
		r.Assets = append(r.Assets, mtt.ReleaseAsset{Name: n, URL: "https://dl/" + n})
	}
	return r
}

func TestPrepareStates(t *testing.T) {
	u := NewSelfUpdater()
	full := relWith("v0.9.0", "mtt_v0.9.0_linux_amd64", checksumsAsset)

	// pre-release current + asset present -> UpdateAvailable via asset
	p, err := u.Prepare(context.Background(), "v0.9.0-3-gabc", "linux", "amd64", true, false, &fakeSource{rel: full})
	if err != nil || p.State != UpdateAvailable || p.Via != ViaAsset || p.AssetName != "mtt_v0.9.0_linux_amd64" {
		t.Fatalf("asset update: %+v err=%v", p, err)
	}
	// equal current -> NoUpdate
	if p, _ := u.Prepare(context.Background(), "v0.9.0", "linux", "amd64", true, false, &fakeSource{rel: full}); p.State != NoUpdate {
		t.Fatalf("equal: %+v", p)
	}
	// equal + force -> UpdateAvailable
	if p, _ := u.Prepare(context.Background(), "v0.9.0", "linux", "amd64", true, true, &fakeSource{rel: full}); p.State != UpdateAvailable {
		t.Fatalf("force equal: %+v", p)
	}
	// dev current, no force -> Undetermined (+reason)
	if p, _ := u.Prepare(context.Background(), "dev", "linux", "amd64", true, false, &fakeSource{rel: full}); p.State != Undetermined || p.Reason == "" {
		t.Fatalf("dev: %+v", p)
	}
	// dev current + force -> UpdateAvailable
	if p, _ := u.Prepare(context.Background(), "dev", "linux", "amd64", true, true, &fakeSource{rel: full}); p.State != UpdateAvailable {
		t.Fatalf("dev force: %+v", p)
	}
}

func TestPrepareViaSelection(t *testing.T) {
	u := NewSelfUpdater()
	// platform absent, Go present -> go-install
	rel := relWith("v0.9.0", "mtt_v0.9.0_linux_amd64", checksumsAsset)
	if p, _ := u.Prepare(context.Background(), "dev", "linux", "riscv64", true, true, &fakeSource{rel: rel}); p.Via != ViaGoInstall {
		t.Fatalf("go-install: %+v", p)
	}
	// platform absent, no Go -> Via none (+reason), still UpdateAvailable
	if p, _ := u.Prepare(context.Background(), "dev", "linux", "riscv64", false, true, &fakeSource{rel: rel}); p.State != UpdateAvailable || p.Via != ViaNone || p.Reason == "" {
		t.Fatalf("via none: %+v", p)
	}
	// asset present but SHA256SUMS missing -> same branch (go-install / none)
	relNoSums := relWith("v0.9.0", "mtt_v0.9.0_linux_amd64")
	if p, _ := u.Prepare(context.Background(), "dev", "linux", "amd64", true, true, &fakeSource{rel: relNoSums}); p.Via != ViaGoInstall {
		t.Fatalf("no-sums+go: %+v", p)
	}
	if p, _ := u.Prepare(context.Background(), "dev", "linux", "amd64", false, true, &fakeSource{rel: relNoSums}); p.Via != ViaNone {
		t.Fatalf("no-sums+noGo: %+v", p)
	}
}

func TestPrepareLatestError(t *testing.T) {
	u := NewSelfUpdater()
	if _, err := u.Prepare(context.Background(), "v0.9.0", "linux", "amd64", true, false, &fakeSource{latErr: errors.New("boom")}); err == nil {
		t.Fatal("Latest() failure must propagate as a Prepare error")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/core/ -run 'Prepare'`
Expected: FAIL — `mtt.Release`/`mtt.ReleaseSource` and `Prepare`/state consts undefined.

- [ ] **Step 3: Implement the ports** — create `pkg/mtt/release.go`:

```go
package mtt

import "context"

// ReleaseAsset is one downloadable file attached to a release.
type ReleaseAsset struct {
	Name string
	URL  string
}

// Release is the published-release metadata self-update needs — its tag and its
// downloadable assets. Provider-agnostic; an adapter maps its own API into it.
type Release struct {
	Tag    string
	Assets []ReleaseAsset
}

// ReleaseSource is the driven port for discovering and downloading a release. The
// github adapter implements it over the HTTP API; tests fake it (no network).
type ReleaseSource interface {
	// Latest returns the newest published release.
	Latest(ctx context.Context) (Release, error)
	// Fetch downloads the bytes at url (an asset or SHA256SUMS).
	Fetch(ctx context.Context, url string) ([]byte, error)
}

// BinaryReplacer atomically swaps the executable at path with newBinary. The bytes
// are ALREADY verified (the caller checks the checksum before calling). Platform
// implementations are side-effecting and not hermetically testable.
type BinaryReplacer interface {
	Replace(path string, newBinary []byte) error
}

// GoInstaller installs module@version through the Go toolchain (the fallback when
// no verifiable asset matches the platform); it returns the installed binary path.
type GoInstaller interface {
	Install(ctx context.Context, module, version string) (path string, err error)
}
```

- [ ] **Step 4: Implement the state types + `Prepare`** — append to `internal/core/selfupdate.go`:

```go
import (
	"context"
	// keep existing imports
	"github.com/pashukhin/mtt/pkg/mtt"
)

// UpdateState is the determinate outcome of Prepare (never a hard error for a
// resolvable release).
type UpdateState string

const (
	UpdateAvailable UpdateState = "update-available"
	NoUpdate        UpdateState = "no-update"
	Undetermined    UpdateState = "undetermined"
)

// UpdateVia is how an available update would be applied. viaUnset is used for
// NoUpdate/Undetermined; ViaNone means "a newer release exists but no install
// method on this platform".
type UpdateVia string

const (
	viaUnset     UpdateVia = ""
	ViaAsset     UpdateVia = "asset"
	ViaGoInstall UpdateVia = "go-install"
	ViaNone      UpdateVia = "none"
)

// Plan is Prepare's determinate decision.
type Plan struct {
	Current      string
	Latest       string
	State        UpdateState
	Via          UpdateVia
	Tag          string
	AssetName    string
	AssetURL     string
	ChecksumsURL string
	Reason       string // populated for Undetermined and Via:none
}

// SelfUpdater computes and applies a self-update. All effects are injected ports.
type SelfUpdater struct{}

// NewSelfUpdater builds the usecase.
func NewSelfUpdater() *SelfUpdater { return &SelfUpdater{} }

// Prepare resolves the latest release and decides what (if anything) to do. It
// returns an error ONLY when src.Latest fails; every other outcome is a state.
func (u *SelfUpdater) Prepare(ctx context.Context, current, goos, goarch string, goAvailable, force bool, src mtt.ReleaseSource) (Plan, error) {
	rel, err := src.Latest(ctx)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve latest release: %w", err)
	}
	p := Plan{Current: current, Latest: rel.Tag, Tag: rel.Tag}

	switch {
	case !Orderable(current):
		if !force {
			p.State = Undetermined
			p.Reason = fmt.Sprintf("cannot determine current version %q; re-run with --force to update to %s", current, rel.Tag)
			return p, nil
		}
	case isNewer(rel.Tag, current):
		// update
	default: // latest <= current
		if !force {
			p.State = NoUpdate
			return p, nil
		}
	}

	// An update should be applied — pick the install method.
	p.State = UpdateAvailable
	an := assetName(rel.Tag, goos, goarch)
	assetURL, hasAsset := findAsset(rel, an)
	sumsURL, hasSums := findAsset(rel, checksumsAsset)
	switch {
	case hasAsset && hasSums:
		p.Via = ViaAsset
		p.AssetName, p.AssetURL, p.ChecksumsURL = an, assetURL, sumsURL
	case goAvailable:
		p.Via = ViaGoInstall
	case !hasAsset:
		p.Via = ViaNone
		p.Reason = fmt.Sprintf("no asset %q in release %s and no Go toolchain to build from source", an, rel.Tag)
	default: // asset present, checksums missing, no Go
		p.Via = ViaNone
		p.Reason = fmt.Sprintf("release %s has no %s (unverifiable) and no Go toolchain to build from source", rel.Tag, checksumsAsset)
	}
	return p, nil
}

// findAsset returns the URL of the asset named name, if present.
func findAsset(rel mtt.Release, name string) (string, bool) {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a.URL, true
		}
	}
	return "", false
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/core/ ./pkg/mtt/ -run 'Prepare|Release'`
Expected: PASS. Then `go build ./...` to confirm the ports compile.

- [ ] **Step 6: Commit**

```bash
git add pkg/mtt/release.go internal/core/selfupdate.go internal/core/selfupdate_test.go
git commit -m "t44: pkg/mtt release ports + core SelfUpdater.Prepare (determinate Plan)"
```

---

## Task 4: `SelfUpdater.Apply`

**Files:**
- Modify: `internal/core/selfupdate.go` (`Result`, `Apply`)
- Test: `internal/core/selfupdate_test.go` (extend — fake replacer + fake installer)

**Interfaces:**
- Produces:
  - `type Result struct { Tag string; Via UpdateVia; Path string }`
  - `func (u *SelfUpdater) Apply(ctx context.Context, p Plan, src mtt.ReleaseSource, replacer mtt.BinaryReplacer, installer mtt.GoInstaller, targetPath string) (Result, error)`
  - asset: `Fetch(AssetURL)` + `Fetch(ChecksumsURL)` → `verifyChecksum` → `Replace(targetPath, bytes)`; **no `Replace` on a verify failure**. go-install: `Install(ctx, selfUpdateModule, Tag)` → `Result.Path`.

- [ ] **Step 1: Write the failing test** — append to `internal/core/selfupdate_test.go`:

```go
type fakeReplacer struct {
	path  string
	bytes []byte
	calls int
	err   error
}

func (f *fakeReplacer) Replace(path string, b []byte) error {
	f.calls++
	f.path, f.bytes = path, b
	return f.err
}

type fakeInstaller struct {
	module, version string
	path            string
	err             error
}

func (f *fakeInstaller) Install(_ context.Context, module, version string) (string, error) {
	f.module, f.version = module, version
	return f.path, f.err
}

func TestApplyAsset(t *testing.T) {
	u := NewSelfUpdater()
	asset := []byte("new-binary")
	name := "mtt_v0.9.0_linux_amd64"
	src := &fakeSource{fetched: map[string][]byte{
		"https://dl/asset": asset,
		"https://dl/sums":  sums(name, asset),
	}}
	p := Plan{State: UpdateAvailable, Via: ViaAsset, Tag: "v0.9.0", AssetName: name, AssetURL: "https://dl/asset", ChecksumsURL: "https://dl/sums"}

	rep := &fakeReplacer{}
	res, err := u.Apply(context.Background(), p, src, rep, &fakeInstaller{}, "/usr/local/bin/mtt")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if rep.calls != 1 || rep.path != "/usr/local/bin/mtt" || string(rep.bytes) != "new-binary" {
		t.Fatalf("replacer got path=%q bytes=%q calls=%d", rep.path, rep.bytes, rep.calls)
	}
	if res.Via != ViaAsset || res.Path != "/usr/local/bin/mtt" {
		t.Fatalf("result: %+v", res)
	}

	// checksum mismatch -> error AND Replace NOT called
	badSrc := &fakeSource{fetched: map[string][]byte{
		"https://dl/asset": []byte("tampered"),
		"https://dl/sums":  sums(name, asset), // sums for the ORIGINAL asset
	}}
	rep2 := &fakeReplacer{}
	if _, err := u.Apply(context.Background(), p, badSrc, rep2, &fakeInstaller{}, "/x"); err == nil {
		t.Fatal("mismatch must error")
	}
	if rep2.calls != 0 {
		t.Fatal("Replace must NOT be called on a verify failure")
	}
}

func TestApplyGoInstall(t *testing.T) {
	u := NewSelfUpdater()
	inst := &fakeInstaller{path: "/home/u/go/bin/mtt"}
	p := Plan{State: UpdateAvailable, Via: ViaGoInstall, Tag: "v0.9.0"}
	res, err := u.Apply(context.Background(), p, &fakeSource{}, &fakeReplacer{}, inst, "/ignored")
	if err != nil {
		t.Fatal(err)
	}
	if inst.module != "github.com/pashukhin/mtt/cmd/mtt" || inst.version != "v0.9.0" {
		t.Fatalf("install args: %q %q", inst.module, inst.version)
	}
	if res.Via != ViaGoInstall || res.Path != "/home/u/go/bin/mtt" {
		t.Fatalf("result: %+v", res)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/core/ -run 'Apply'`
Expected: FAIL — undefined `Apply`/`Result`.

- [ ] **Step 3: Implement** — append to `internal/core/selfupdate.go`:

```go
// Result reports what Apply did.
type Result struct {
	Tag  string
	Via  UpdateVia
	Path string
}

// Apply performs the plan. Asset: download asset + SHA256SUMS, verify, replace —
// verification precedes any write. go-install: shell the toolchain. Only called by
// the CLI for an UpdateAvailable plan with a concrete Via.
func (u *SelfUpdater) Apply(ctx context.Context, p Plan, src mtt.ReleaseSource, replacer mtt.BinaryReplacer, installer mtt.GoInstaller, targetPath string) (Result, error) {
	switch p.Via {
	case ViaAsset:
		asset, err := src.Fetch(ctx, p.AssetURL)
		if err != nil {
			return Result{}, fmt.Errorf("download asset %q: %w", p.AssetName, err)
		}
		checks, err := src.Fetch(ctx, p.ChecksumsURL)
		if err != nil {
			return Result{}, fmt.Errorf("download %s: %w", checksumsAsset, err)
		}
		if err := verifyChecksum(p.AssetName, asset, checks); err != nil {
			return Result{}, err
		}
		if err := replacer.Replace(targetPath, asset); err != nil {
			return Result{}, fmt.Errorf("replace %s: %w", targetPath, err)
		}
		return Result{Tag: p.Tag, Via: ViaAsset, Path: targetPath}, nil
	case ViaGoInstall:
		path, err := installer.Install(ctx, selfUpdateModule, p.Tag)
		if err != nil {
			return Result{}, fmt.Errorf("go install %s@%s: %w", selfUpdateModule, p.Tag, err)
		}
		return Result{Tag: p.Tag, Via: ViaGoInstall, Path: path}, nil
	default:
		return Result{}, fmt.Errorf("no install method: %s", p.Reason)
	}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/core/ -run 'Apply'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/selfupdate.go internal/core/selfupdate_test.go
git commit -m "t44: core SelfUpdater.Apply (verify-before-replace; go-install path)"
```

---

## Task 5: github adapter (`ReleaseSource`)

**Files:**
- Create: `internal/adapter/github/github.go`, `internal/adapter/github/CLAUDE.md`
- Test: `internal/adapter/github/github_test.go`

**Interfaces:**
- Consumes: `mtt.Release`, `mtt.ReleaseAsset` (Task 3).
- Produces:
  - `func New() *Source` — real client (no global `Timeout`; per-op context deadlines).
  - `Source` implements `mtt.ReleaseSource`.
  - (unexported, test-seam) `func newWithDoer(d httpDoer) *Source`.
  - consts `apiTimeout = 15*time.Second`, `downloadTimeout = 3*time.Minute`, `latestURL`.

- [ ] **Step 1: Write the failing test** — `internal/adapter/github/github_test.go`:

```go
package github

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeDoer struct{ resp map[string]string }

func (f fakeDoer) Do(req *http.Request) (*http.Response, error) {
	body, ok := f.resp[req.URL.String()]
	code := http.StatusOK
	if !ok {
		code = http.StatusNotFound
	}
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}

func TestLatestParses(t *testing.T) {
	json := `{"tag_name":"v0.9.0","assets":[
		{"name":"mtt_v0.9.0_linux_amd64","browser_download_url":"https://dl/lin"},
		{"name":"SHA256SUMS","browser_download_url":"https://dl/sums"}]}`
	s := newWithDoer(fakeDoer{resp: map[string]string{latestURL: json}})
	rel, err := s.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Tag != "v0.9.0" || len(rel.Assets) != 2 || rel.Assets[0].URL != "https://dl/lin" {
		t.Fatalf("parsed: %+v", rel)
	}
}

func TestFetch(t *testing.T) {
	s := newWithDoer(fakeDoer{resp: map[string]string{"https://dl/x": "BYTES"}})
	b, err := s.Fetch(context.Background(), "https://dl/x")
	if err != nil || string(b) != "BYTES" {
		t.Fatalf("fetch: %q err=%v", b, err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/adapter/github/`
Expected: FAIL — package/symbols undefined.

- [ ] **Step 3: Implement** — `internal/adapter/github/github.go`:

```go
// Package github implements mtt.ReleaseSource over the GitHub Releases HTTP API.
// The HTTP client is injectable (an httpDoer) so tests run without a socket. Per-
// operation context deadlines bound the API probe and the asset download
// separately; the default client sets no global Timeout and does not override
// redirect/proxy behavior (so browser_download_url redirects and HTTP(S)_PROXY
// keep working).
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

const (
	latestURL       = "https://api.github.com/repos/pashukhin/mtt/releases/latest"
	apiTimeout      = 15 * time.Second
	downloadTimeout = 3 * time.Minute
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Source is the GitHub-backed ReleaseSource.
type Source struct{ doer httpDoer }

// New returns a Source over the default HTTP client (no global Timeout — the
// per-operation context governs; redirects/proxy left at their defaults).
func New() *Source { return &Source{doer: &http.Client{}} }

func newWithDoer(d httpDoer) *Source { return &Source{doer: d} }

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Latest returns the newest release.
func (s *Source) Latest(ctx context.Context) (mtt.Release, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return mtt.Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := s.doer.Do(req)
	if err != nil {
		return mtt.Release{}, fmt.Errorf("GET latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return mtt.Release{}, fmt.Errorf("GET latest release: unexpected status %d", resp.StatusCode)
	}
	var gr ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return mtt.Release{}, fmt.Errorf("decode release: %w", err)
	}
	rel := mtt.Release{Tag: gr.TagName}
	for _, a := range gr.Assets {
		rel.Assets = append(rel.Assets, mtt.ReleaseAsset{Name: a.Name, URL: a.URL})
	}
	return rel, nil
}

// Fetch downloads the bytes at url.
func (s *Source) Fetch(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: unexpected status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
```

- [ ] **Step 4: Write the CLAUDE.md** — `internal/adapter/github/CLAUDE.md`:

```markdown
# internal/adapter/github

Implements `mtt.ReleaseSource` over the GitHub Releases HTTP API
(`repos/pashukhin/mtt/releases/latest`). Maps the API JSON (`tag_name`,
`assets[].{name, browser_download_url}`) into `mtt.Release`.

## Boundaries
- The HTTP client is an injectable `httpDoer` — tests use a fake (no socket; the
  "no network in tests" rule). `New()` uses the default `*http.Client`.
- Per-operation context deadlines: `apiTimeout` (metadata) vs `downloadTimeout`
  (assets). No global `Client.Timeout`; `CheckRedirect`/`Transport.Proxy` left at
  defaults so `browser_download_url` redirects and `HTTP(S)_PROXY` work.
- No auth in v1 (unauthenticated 60 req/hr/IP); `GH_TOKEN` support is deferred.
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/adapter/github/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/github/
git commit -m "t44: github adapter — ReleaseSource over releases/latest (injectable doer)"
```

---

## Task 6: installer adapter (`BinaryReplacer` + `GoInstaller`)

**Files:**
- Create: `internal/adapter/installer/replace.go`, `replace_unix.go`, `replace_windows.go`, `goinstall.go`, `CLAUDE.md`
- Test: `internal/adapter/installer/installer_test.go`

**Interfaces:**
- Consumes: `mtt.BinaryReplacer`, `mtt.GoInstaller` (Task 3).
- Produces:
  - `func NewReplacer() mtt.BinaryReplacer` (platform impl chosen by build tag).
  - `func NewGoInstaller() mtt.GoInstaller`.
  - (test seams, unexported) `goInstaller{run, gobin}` fields.

- [ ] **Step 1: Write the failing test** — `internal/adapter/installer/installer_test.go` (Unix-only run guard + go-install with fakes):

```go
package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReplaceUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix replace path")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "mtt")
	if err := os.WriteFile(path, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := NewReplacer().Replace(path, []byte("NEWBINARY")); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "NEWBINARY" {
		t.Fatalf("content: %q", got)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode: %v", info.Mode())
	}
	// no leftover temp files in the dir
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("leftover temp: %v", entries)
	}
}

func TestGoInstallerArgs(t *testing.T) {
	var gotName string
	var gotArgs []string
	g := &goInstaller{
		run: func(_ context.Context, name string, args ...string) error {
			gotName, gotArgs = name, args
			return nil
		},
		gobin: func(context.Context) (string, error) { return "/home/u/go/bin", nil },
	}
	path, err := g.Install(context.Background(), "github.com/pashukhin/mtt/cmd/mtt", "v0.9.0")
	if err != nil {
		t.Fatal(err)
	}
	if gotName != "go" || len(gotArgs) != 3 || gotArgs[2] != "github.com/pashukhin/mtt/cmd/mtt@v0.9.0" {
		t.Fatalf("argv: %s %v", gotName, gotArgs)
	}
	if path != "/home/u/go/bin/mtt"+exeSuffix() {
		t.Fatalf("path: %q", path)
	}
	// run error propagates
	gErr := &goInstaller{run: func(context.Context, string, ...string) error { return errors.New("x") }, gobin: g.gobin}
	if _, err := gErr.Install(context.Background(), "m", "v1"); err == nil {
		t.Fatal("run error must propagate")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/adapter/installer/`
Expected: FAIL — package/symbols undefined.

- [ ] **Step 3: Implement the constructor + go-installer** — `internal/adapter/installer/replace.go`:

```go
// Package installer applies a self-update: a platform-specific BinaryReplacer
// (atomic on Unix; rename-then-swap on Windows) and a GoInstaller that shells the
// Go toolchain. Both are side-effecting; the CLI wires them, and unit tests fake
// them (the real replace is verified by the manual real-binary smoke).
package installer

import "github.com/pashukhin/mtt/pkg/mtt"

// replacer is the platform BinaryReplacer (method in replace_unix.go /
// replace_windows.go).
type replacer struct{}

// NewReplacer returns the platform binary replacer.
func NewReplacer() mtt.BinaryReplacer { return &replacer{} }
```

`internal/adapter/installer/goinstall.go`:

```go
package installer

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pashukhin/mtt/pkg/mtt"
)

// goInstaller installs module@version via the Go toolchain. run/gobin are seams
// for hermetic tests.
type goInstaller struct {
	run   func(ctx context.Context, name string, args ...string) error
	gobin func(ctx context.Context) (string, error)
}

// NewGoInstaller returns a GoInstaller over the real toolchain.
func NewGoInstaller() mtt.GoInstaller {
	return &goInstaller{run: defaultRun, gobin: defaultGobin}
}

func (g *goInstaller) Install(ctx context.Context, module, version string) (string, error) {
	if err := g.run(ctx, "go", "install", module+"@"+version); err != nil {
		return "", err
	}
	dir, err := g.gobin(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mtt"+exeSuffix()), nil
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func defaultRun(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = nil, nil // let go install be quiet; errors surface via Run()
	return cmd.Run()
}

// defaultGobin resolves the go bin dir: `go env GOBIN` if set, else GOPATH/bin.
func defaultGobin(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "go", "env", "GOBIN", "GOPATH").Output()
	if err != nil {
		return "", fmt.Errorf("go env: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) >= 1 && strings.TrimSpace(lines[0]) != "" {
		return strings.TrimSpace(lines[0]), nil // GOBIN
	}
	if len(lines) >= 2 && strings.TrimSpace(lines[1]) != "" {
		return filepath.Join(strings.TrimSpace(lines[1]), "bin"), nil // GOPATH/bin
	}
	return "", fmt.Errorf("cannot resolve go bin dir")
}
```

- [ ] **Step 4: Implement the platform replacers**

`internal/adapter/installer/replace_unix.go`:

```go
//go:build !windows

package installer

import (
	"fmt"
	"os"
	"path/filepath"
)

// Replace writes newBinary to a same-dir temp file (so the rename is atomic on one
// filesystem), preserves the target's mode, then renames over the target. The
// running process keeps its open inode. A permission failure surfaces from the
// temp-create (attempt-and-surface, not a racy stat precheck).
func (r *replacer) Replace(path string, newBinary []byte) error {
	dir := filepath.Dir(path)
	mode := os.FileMode(0o755)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, ".mtt-update-*")
	if err != nil {
		return fmt.Errorf("cannot write %s (create temp): %w", dir, err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(newBinary); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename over %s: %w", path, err)
	}
	ok = true
	return nil
}
```

`internal/adapter/installer/replace_windows.go`:

```go
//go:build windows

package installer

import (
	"fmt"
	"os"
)

// Replace swaps a running Windows .exe: a running image can be renamed but not
// overwritten. Remove any stale ".old" (rename won't overwrite), move the running
// exe aside, write the new bytes; on failure, roll the old one back. The fresh
// ".old" is left for best-effort cleanup on a later run.
//
// NOTE: unverified in CI (no Windows runner) — see the spec's D6 recorded risk.
func (r *replacer) Replace(path string, newBinary []byte) error {
	old := path + ".old"
	_ = os.Remove(old)
	if err := os.Rename(path, old); err != nil {
		return fmt.Errorf("move running exe aside (%s): %w", path, err)
	}
	if err := os.WriteFile(path, newBinary, 0o755); err != nil {
		_ = os.Rename(old, path) // rollback
		return fmt.Errorf("write new binary %s: %w", path, err)
	}
	return nil
}
```

- [ ] **Step 5: Write the CLAUDE.md** — `internal/adapter/installer/CLAUDE.md`:

```markdown
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
```

- [ ] **Step 6: Run to verify it passes**

Run: `go test ./internal/adapter/installer/ && GOOS=windows go build ./internal/adapter/installer/`
Expected: PASS, and the Windows build compiles (the `//go:build windows` file type-checks).

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/installer/
git commit -m "t44: installer adapter — Unix/Windows BinaryReplacer + GoInstaller"
```

---

## Task 7: CLI `mtt self-update`

**Files:**
- Create: `internal/cli/selfupdate.go`
- Modify: `internal/cli/root.go` (register)
- Test: `internal/cli/selfupdate_test.go`, `internal/cli/testdata/scripts/selfupdate.txt`

**Interfaces:**
- Consumes: `resolveVersion()`, `jsonFlag`, `writeJSON`, `core.NewSelfUpdater`, `core.Orderable`, `core.Plan`/`Result`/state consts, `github.New`, `installer.NewReplacer`/`NewGoInstaller`.
- Produces: `func newSelfUpdateCmd() *cobra.Command`; `type selfUpdateJSON struct{…}`; `func toSelfUpdateJSON(...) selfUpdateJSON` (pure, unit-tested).

- [ ] **Step 1: Write the failing unit test** — `internal/cli/selfupdate_test.go`:

```go
package cli

import (
	"testing"

	"github.com/pashukhin/mtt/internal/core"
)

func TestToSelfUpdateJSONCheckOnly(t *testing.T) {
	p := core.Plan{Current: "v0.8.0", Latest: "v0.9.0", State: core.UpdateAvailable, Via: core.ViaAsset, AssetName: "mtt_v0.9.0_linux_amd64"}
	j := toSelfUpdateJSON(p, core.Result{}, false, nil)
	if !j.UpdateAvailable || j.Updated || j.Via != "asset" || j.Latest != "v0.9.0" {
		t.Fatalf("check-only json: %+v", j)
	}
	// applied
	r := core.Result{Tag: "v0.9.0", Via: core.ViaAsset, Path: "/usr/local/bin/mtt"}
	j = toSelfUpdateJSON(p, r, true, nil)
	if !j.Updated || j.Path != "/usr/local/bin/mtt" {
		t.Fatalf("applied json: %+v", j)
	}
	// via none carries the reason
	pn := core.Plan{Current: "dev", Latest: "v0.9.0", State: core.UpdateAvailable, Via: core.ViaNone, Reason: "no asset"}
	j = toSelfUpdateJSON(pn, core.Result{}, false, nil)
	if j.Via != "none" || j.Reason == "" {
		t.Fatalf("via none json: %+v", j)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run 'ToSelfUpdateJSON'`
Expected: FAIL — undefined `toSelfUpdateJSON`/`selfUpdateJSON`.

- [ ] **Step 3: Implement** — `internal/cli/selfupdate.go`:

```go
package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/adapter/github"
	"github.com/pashukhin/mtt/internal/adapter/installer"
	"github.com/pashukhin/mtt/internal/core"
)

// selfUpdateJSON is the pinned --json shape.
type selfUpdateJSON struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
	Updated         bool   `json:"updated"`
	Via             string `json:"via"`
	Asset           string `json:"asset,omitempty"`
	Path            string `json:"path,omitempty"`
	Reason          string `json:"reason,omitempty"`
	Error           string `json:"error,omitempty"`
}

// toSelfUpdateJSON builds the view from a plan (+ result if applied). err, when
// non-nil, populates the error field (the object still renders on a failure path).
func toSelfUpdateJSON(p core.Plan, r core.Result, applied bool, err error) selfUpdateJSON {
	j := selfUpdateJSON{
		Current:         p.Current,
		Latest:          p.Latest,
		UpdateAvailable: p.State == core.UpdateAvailable,
		Updated:         applied,
		Via:             string(p.Via),
		Asset:           p.AssetName,
		Reason:          p.Reason,
	}
	if applied {
		j.Via = string(r.Via)
		j.Path = r.Path
	}
	if err != nil {
		j.Error = err.Error()
	}
	return j
}

// newSelfUpdateCmd builds `mtt self-update`.
func newSelfUpdateCmd() *cobra.Command {
	var checkOnly, force bool
	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update the installed mtt binary to the latest release",
		Long: "Download the latest published release asset, verify its SHA-256, and atomically\n" +
			"replace the running binary. Falls back to `go install` when no verifiable asset\n" +
			"matches this platform. --check-only reports availability without writing.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			current := resolveVersion()

			// Hermetic short-circuit: an unorderable current with neither --force nor
			// --check-only always refuses — decide it BEFORE any network call.
			if !force && !checkOnly && !core.Orderable(current) {
				refusal := fmt.Errorf("cannot determine the current version (%q); re-run with --force to update anyway", current)
				if jsonFlag(cmd) {
					_ = writeJSON(cmd.OutOrStdout(), toSelfUpdateJSON(core.Plan{Current: current}, core.Result{}, false, refusal))
				}
				return refusal
			}

			target, err := os.Executable()
			if err != nil {
				return fmt.Errorf("locate running binary: %w", err)
			}
			if resolved, err := filepath.EvalSymlinks(target); err == nil {
				target = resolved
			}
			_, goErr := exec.LookPath("go")
			src := github.New()
			updater := core.NewSelfUpdater()

			plan, err := updater.Prepare(cmd.Context(), current, runtime.GOOS, runtime.GOARCH, goErr == nil, force, src)
			if err != nil {
				if jsonFlag(cmd) {
					_ = writeJSON(cmd.OutOrStdout(), toSelfUpdateJSON(core.Plan{Current: current}, core.Result{}, false, err))
				}
				return err
			}

			if checkOnly {
				return renderSelfUpdate(cmd, plan, core.Result{}, false, nil)
			}

			switch plan.State {
			case core.NoUpdate:
				return renderSelfUpdate(cmd, plan, core.Result{}, false, nil)
			case core.Undetermined:
				refusal := errors.New(plan.Reason)
				return renderSelfUpdate(cmd, plan, core.Result{}, false, refusal)
			default: // UpdateAvailable
				if plan.Via == core.ViaNone {
					return renderSelfUpdate(cmd, plan, core.Result{}, false, errors.New(plan.Reason))
				}
				res, err := updater.Apply(cmd.Context(), plan, src, installer.NewReplacer(), installer.NewGoInstaller(), target)
				if err != nil {
					return renderSelfUpdate(cmd, plan, core.Result{}, false, err)
				}
				return renderSelfUpdate(cmd, plan, res, true, nil)
			}
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check-only", false, "report whether an update is available; write nothing")
	cmd.Flags().BoolVar(&force, "force", false, "update even from a dev build or when already on the latest")
	return cmd
}

// renderSelfUpdate prints text (or JSON) for a plan/result. A non-nil err makes it
// return that err (→ exit 1) after emitting the JSON object / a stderr message.
func renderSelfUpdate(cmd *cobra.Command, p core.Plan, r core.Result, applied bool, err error) error {
	if jsonFlag(cmd) {
		if werr := writeJSON(cmd.OutOrStdout(), toSelfUpdateJSON(p, r, applied, err)); werr != nil {
			return werr
		}
		return err
	}
	out := cmd.OutOrStdout()
	switch {
	case err != nil:
		return err // Execute() prints "error: <msg>" to stderr, exit 1
	case applied:
		if r.Via == core.ViaGoInstall {
			fmt.Fprintf(out, "updated to %s via go install → %s\n", r.Tag, r.Path)
			if r.Path != "" {
				fmt.Fprintf(out, "note: the updated binary is at %s (ensure it is the mtt on your PATH)\n", r.Path)
			}
		} else {
			fmt.Fprintf(out, "updated %s → %s\n", p.Current, r.Tag)
		}
	case p.State == core.NoUpdate:
		fmt.Fprintf(out, "already up to date (%s)\n", p.Current)
	case p.State == core.UpdateAvailable && p.Via == core.ViaNone:
		fmt.Fprintf(out, "update available (%s), but %s\n", p.Latest, p.Reason)
	case p.State == core.UpdateAvailable:
		fmt.Fprintf(out, "update available: %s → %s (via %s)\n", p.Current, p.Latest, p.Via)
	case p.State == core.Undetermined:
		fmt.Fprintf(out, "%s\n", p.Reason)
	}
	return nil
}
```

- [ ] **Step 4: Register the command** — in `internal/cli/root.go`, add `newSelfUpdateCmd()` to the `root.AddCommand(...)` list (append after `newPrimeCmd()`):

```go
	root.AddCommand(newVersionCmd(), newInitCmd(), newTypesCmd(), newAddCmd(), newShowCmd(),
		newListCmd(), newEditCmd(), newTreeCmd(), newDepCmd(), newReadyCmd(), newStatusCmd(),
		newUseCmd(), newRmCmd(), newRoadmapCmd(), newTagCmd(), newTagsCmd(), newDoCmd(),
		newNoteCmd(), newRefCmd(), newCheckCmd(), newPrimeCmd(), newSelfUpdateCmd())
```

- [ ] **Step 5: Write the e2e script** — `internal/cli/testdata/scripts/selfupdate.txt` (hermetic: the test binary reports version "dev", so a plain `self-update` short-circuits before any network; `--check-only`/`--force` are NOT exercised here since they would call the network):

```
# usage: unexpected arg
! mtt self-update bogus
! stderr 'panic'

# dev build, no --force -> refuse before any network (hermetic), exit 1
! mtt self-update
stderr 'cannot determine the current version'

# same, --json: the object is emitted with error set, exit 1
! mtt self-update --json
stdout '"error"'
stdout '"current": "dev"'
```

- [ ] **Step 6: Run to verify tests pass**

Run: `go test ./internal/cli/ -run 'ToSelfUpdateJSON|TestScripts/selfupdate'`
Expected: PASS. (If the testscript harness runs all scripts under one test, use `go test ./internal/cli/ -run 'Script'` and confirm `selfupdate.txt` passes.)

- [ ] **Step 7: Full gate**

Run: `make check`
Expected: PASS (fmt, vet, lint, race tests, build).

- [ ] **Step 8: Commit**

```bash
git add internal/cli/selfupdate.go internal/cli/selfupdate_test.go internal/cli/testdata/scripts/selfupdate.txt internal/cli/root.go
git commit -m "t44: CLI mtt self-update (check-only/force/json; hermetic dev-refusal short-circuit)"
```

---

## Task 8: docs + CLAUDE.md sync

**Files:**
- Modify: `CLI_REFERENCE.md`, `CLI_REFERENCE.ru.md`, `DESIGN.md`, `DESIGN.ru.md`, `RELEASING.md`, `CHANGELOG.md`, `docs/architecture/model.go`, `pkg/mtt/CLAUDE.md`, `internal/core/CLAUDE.md`, `internal/cli/CLAUDE.md`.

**Interfaces:** none (docs). Grep parallel occurrences (EN + RU) before editing.

- [ ] **Step 1: `CLI_REFERENCE.md` ↔ `.ru.md`** — add a `mtt self-update` entry near the end of the command reference (after `mtt prime`). Document: purpose; `--check-only` (report, exit 0), `--force` (dev/re-install), `--json` schema `{current,latest,update_available,updated,via,asset,path,reason,error}`; the asset+SHA256 primary path; the `go install` fallback and its GOBIN caveat; exit 1 on failure/refusal. Mirror verbatim in `.ru.md`.

- [ ] **Step 2: `DESIGN.md` ↔ `.ru.md`** — under the packaging/release material add a short **Self-update** subsection: mechanism (asset primary + go-install fallback), the verify-before-replace invariant, the platform-replace note (Unix atomic rename; Windows rename-then-swap, unverified-in-CI), and the three ports (`ReleaseSource`/`BinaryReplacer`/`GoInstaller`) in the hexagon map. Mirror in `.ru.md`. **Grep for existing "self-update"/"release"/"SHA256" occurrences first** — update every parallel "Shipped"/packaging block, EN + RU.

- [ ] **Step 3: `RELEASING.md`** — add one line: the published assets + `SHA256SUMS` are what `mtt self-update` consumes (closing the loop).

- [ ] **Step 4: `CHANGELOG.md`** — under `[Unreleased]` → **Added**: `mtt self-update — update the installed binary to the latest release (asset + SHA256 verify, or go install fallback); --check-only / --force / --json.`

- [ ] **Step 5: `docs/architecture/model.go`** — add a commented block noting the new ports (`ReleaseSource`/`BinaryReplacer`/`GoInstaller`) as contract and `SelfUpdater` as a `core` usecase (like other usecases). Match the file's existing comment style.

- [ ] **Step 6: CLAUDE.md files:**
  - `pkg/mtt/CLAUDE.md` — note `release.go`: the 3 self-update ports + `Release`/`ReleaseAsset` value types.
  - `internal/core/CLAUDE.md` — `SelfUpdater` (`Prepare` determinate `Plan` → `Apply`), pure `assetName`/`verifyChecksum`/`isNewer`/`Orderable`; verify-before-replace.
  - `internal/cli/CLAUDE.md` — `self-update` (wires github+installer adapters, resolves current/target/goos/goarch/goAvailable; hermetic dev-refusal short-circuit).

- [ ] **Step 7: Verify gate + docs build**

Run: `make check`
Expected: PASS (`docs/architecture/model.go` compiles; no test breakage).

- [ ] **Step 8: Commit**

```bash
git add CLI_REFERENCE.md CLI_REFERENCE.ru.md DESIGN.md DESIGN.ru.md RELEASING.md CHANGELOG.md docs/architecture/model.go pkg/mtt/CLAUDE.md internal/core/CLAUDE.md internal/cli/CLAUDE.md
git commit -m "t44: docs — self-update in CLI_REFERENCE/DESIGN/RELEASING/CHANGELOG + CLAUDE.md sync"
```

---

## Task 9: real-binary smoke (manual, `impl_review` acceptance — AC-9)

**Not a code task.** This is the acceptance the spec mandates (the real download + self-replace are not covered by `go test`). Run it on `impl_review`, on a Linux host, against the real `v0.9.0` release.

- [ ] **Step 1: Build a local (pre-release-stamped) binary**

Run: `make build && ./bin/mtt version`
Expected: a `git describe` stamp like `v0.9.0-N-g<sha>` (valid SemVer, orders below `v0.9.0`).

- [ ] **Step 2: Copy it somewhere writable and self-update**

```bash
cp ./bin/mtt /tmp/mtt-smoke && /tmp/mtt-smoke self-update --check-only
/tmp/mtt-smoke self-update
/tmp/mtt-smoke version
```
Expected: `--check-only` reports `update available … v0.9.0` (exit 0); `self-update` downloads `mtt_v0.9.0_linux_amd64`, verifies SHA-256, replaces `/tmp/mtt-smoke`; `version` then prints `v0.9.0`.

- [ ] **Step 3: Idempotence + force**

```bash
/tmp/mtt-smoke self-update        # already up to date (exit 0)
/tmp/mtt-smoke self-update --force # re-installs v0.9.0
```
Expected: first prints "already up to date (v0.9.0)"; `--force` re-installs.

- [ ] **Step 4: Record the smoke result** in the `impl_review` notes / PR body (`docs/superpowers/pr/t44.md`), since CI cannot run it.

---

## Self-Review (run against the spec `docs/superpowers/specs/t44-self-update.md`)

- **Spec coverage:** D1 mechanism → Tasks 3/4/6/7; D2 ports → Task 3; D3 github adapter/timeouts → Task 5; D4 version states → Tasks 1/3; D5 asset+checksum → Tasks 1/2/3; D6 replacers → Task 6; D7 go-install → Tasks 3/4/6; D8 CLI/flags/exit/json → Task 7; D9 Prepare/Apply → Tasks 3/4; D10 deps → Task 1. AC-1..8 map to Tasks 1–7 unit/e2e; AC-9 → Task 9; AC-10 (docs + `make check`) → Task 8 + every task's gate.
- **Type consistency:** `Plan`/`Result`/`UpdateState`/`UpdateVia` defined in Task 3/4 and consumed unchanged in Task 7; `Prepare(ctx, current, goos, goarch, goAvailable, force, src)` and `Apply(ctx, p, src, replacer, installer, targetPath)` signatures match spec D9 and Task 7's call sites; `ViaAsset="asset"`/`ViaGoInstall="go-install"`/`ViaNone="none"` match the pinned `--json` `via` vocabulary.
- **Testing-approach refinement (noted):** the spec's AC-8 listed `--check-only --json` on a dev build as an e2e case; because any path that calls `Latest()` touches the network, that case is covered at the **core** layer (`TestPrepareStates` dev → `Undetermined`) plus the **CLI unit** `toSelfUpdateJSON` view, while the **hermetic testscript** covers the no-network dev-refusal (via the CLI short-circuit) + usage errors. The spec's intent (dev handling is tested) is met without breaking "No network in tests".
