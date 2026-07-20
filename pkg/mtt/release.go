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
