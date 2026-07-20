package core

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
