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
