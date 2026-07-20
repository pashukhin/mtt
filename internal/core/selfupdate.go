package core

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/pashukhin/mtt/pkg/mtt"
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

// UpdateState is the determinate outcome of Prepare (never a hard error for a
// resolvable release).
type UpdateState string

const (
	UpdateAvailable UpdateState = "update-available"
	NoUpdate        UpdateState = "no-update"
	Undetermined    UpdateState = "undetermined"
)

// UpdateVia is how an available update would be applied. The zero value ("") is
// used for NoUpdate/Undetermined; ViaNone means "a newer release exists but no
// install method on this platform".
type UpdateVia string

const (
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
