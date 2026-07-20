package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

type fakeSource struct {
	rel      mtt.Release
	latErr   error
	fetched  map[string][]byte
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
