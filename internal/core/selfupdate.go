package core

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

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
