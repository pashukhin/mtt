package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDirFlagAndEnvResolveProject(t *testing.T) {
	proj := t.TempDir()
	other := t.TempDir() // sibling temp dir, no .mtt ancestor

	// init the project via --dir, from an unrelated cwd
	chdir(t, other)
	if _, _, err := runOut(t, "--dir", proj, "init", "--template", "hierarchy"); err != nil {
		t.Fatalf("init --dir: %v", err)
	}
	if _, _, err := runOut(t, "--dir", proj, "add", "--type", "epic", "build auth"); err != nil {
		t.Fatalf("add --dir: %v", err)
	}
	out, _, err := runOut(t, "--dir", proj, "show", "e1")
	if err != nil || !strings.Contains(out, "e1") {
		t.Fatalf("show --dir: out=%q err=%v", out, err)
	}

	// MTT_DIR env resolves the same project (cwd still `other`, has no .mtt)
	t.Setenv("MTT_DIR", proj)
	out, _, err = runOut(t, "show", "e1")
	if err != nil || !strings.Contains(out, "e1") {
		t.Fatalf("show via MTT_DIR: out=%q err=%v", out, err)
	}

	// --dir without .mtt errors (flag overrides env)
	if _, _, err := runOut(t, "--dir", other, "show", "e1"); err == nil {
		t.Fatal("--dir without .mtt should error")
	}
}

// TestJSONFlag covers the jsonFlag helper directly: no command consumes --json
// yet (that lands with list/show JSON output in later tasks), but the helper
// itself is part of this task's produced surface.
func TestJSONFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	cmd.Flags().Bool("json", false, "")
	if jsonFlag(cmd) {
		t.Fatal("expected jsonFlag to default to false")
	}
	if err := cmd.Flags().Set("json", "true"); err != nil {
		t.Fatal(err)
	}
	if !jsonFlag(cmd) {
		t.Fatal("expected jsonFlag to be true after --json set")
	}
}
