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

func TestReplaceUnixErrorSurfaces(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix replace path")
	}
	// A target whose directory does not exist: the same-dir temp-create fails and
	// the error surfaces cleanly (attempt-and-surface — no racy stat precheck, no
	// panic). AC-7's "original untouched on failure" is otherwise STRUCTURAL — the
	// Unix impl never writes the target until the final atomic rename.
	err := NewReplacer().Replace(filepath.Join(t.TempDir(), "no-such-dir", "mtt"), []byte("x"))
	if err == nil {
		t.Fatal("missing dir must surface an error, not panic")
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
	if path != filepath.Join("/home/u/go/bin", "mtt"+exeSuffix()) {
		t.Fatalf("path: %q", path)
	}
	// run error propagates
	gErr := &goInstaller{run: func(context.Context, string, ...string) error { return errors.New("x") }, gobin: g.gobin}
	if _, err := gErr.Install(context.Background(), "m", "v1"); err == nil {
		t.Fatal("run error must propagate")
	}
}
