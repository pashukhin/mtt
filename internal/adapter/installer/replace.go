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
