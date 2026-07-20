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
