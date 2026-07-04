package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runRoot(t *testing.T, args ...string) error {
	t.Helper()
	root := NewRootCmd()
	root.SetOut(new(strings.Builder))
	root.SetErr(new(strings.Builder))
	root.SetArgs(args)
	return root.Execute()
}

func TestInitCommand(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runRoot(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".mtt", "config.yaml")); err != nil {
		t.Fatalf("config not created: %v", err)
	}
	if err := runRoot(t, "init"); err == nil {
		t.Fatal("re-init without --force should fail")
	}
	if err := runRoot(t, "init", "--force", "--template", "coding"); err != nil {
		t.Fatalf("force init: %v", err)
	}
}

// chdir switches to dir for the duration of the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}
