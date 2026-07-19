package demo

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCodingFlowDemo builds mtt, runs demo/coding-flow.sh against it in a temp
// dir, and asserts the walkthrough reached `done` for each coding type and that
// each of the three deliberate gate blocks actually fired.
func TestCodingFlowDemo(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(thisFile)) // .../demo -> repo root

	mttBin := filepath.Join(t.TempDir(), "mtt")
	build := exec.Command("go", "build", "-o", mttBin, "./cmd/mtt")
	build.Dir = repoRoot
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build mtt: %v", err)
	}

	script := filepath.Join(repoRoot, "demo", "coding-flow.sh")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(),
		"MTT_BIN="+mttBin,
		"MTT_DIR=", "MTT_ROLE=", "MTT_BY=demo",
	)
	out, err := cmd.CombinedOutput()
	t.Logf("demo output:\n%s", out)
	if err != nil {
		t.Fatalf("demo script failed: %v", err)
	}

	got := string(out)
	for _, marker := range []string{"feature: done", "bugfix: done", "refactor: done"} {
		if !strings.Contains(got, marker) {
			t.Errorf("missing marker %q", marker)
		}
	}
	if n := strings.Count(got, "blocked as expected"); n != 3 {
		t.Errorf("want 3 blocked gates, got %d", n)
	}
}
