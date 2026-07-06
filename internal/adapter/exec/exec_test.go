package exec

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func TestRunAllPass(t *testing.T) {
	checks, err := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard).Run([]string{"true", "true"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(checks) != 2 || checks[0].Exit != 0 || checks[1].Exit != 0 {
		t.Fatalf("checks = %+v", checks)
	}
}

func TestRunStopsAtFirstNonZero(t *testing.T) {
	checks, err := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard).Run([]string{"true", "false", "true"})
	if err != nil {
		t.Fatalf("Run: %v (non-zero exit is data, not an error)", err)
	}
	if len(checks) != 2 {
		t.Fatalf("ran %d commands, want to stop after 2", len(checks))
	}
	if checks[0].Exit != 0 || checks[1].Exit == 0 {
		t.Fatalf("checks = %+v", checks)
	}
	if checks[1].Cmd != "false" {
		t.Fatalf("failed cmd = %q, want false", checks[1].Cmd)
	}
}

func TestRunTimeout(t *testing.T) {
	_, err := NewRunner(t.TempDir(), time.Millisecond, io.Discard, io.Discard).Run([]string{"sleep 1"})
	if err == nil {
		t.Fatalf("want a timeout error, got nil")
	}
}

func TestRunStreamsProgressAndSeparatesOutput(t *testing.T) {
	// The command text ("echo $((3+4))") deliberately does not contain its output
	// ("7"), so we can assert the two streams stay separate.
	var prog, out bytes.Buffer
	checks, err := NewRunner(t.TempDir(), time.Minute, &prog, &out).Run([]string{"echo $((3+4))", "true"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(checks) != 2 {
		t.Fatalf("checks = %+v", checks)
	}
	p := prog.String()
	for _, want := range []string{"▶ echo $((3+4))", "✓ echo $((3+4)) (exit 0,", "▶ true", "✓ true (exit 0,"} {
		if !strings.Contains(p, want) {
			t.Fatalf("progress missing %q:\n%s", want, p)
		}
	}
	// command output ("7\n") goes to cmdOut, not to progress. Checking "7\n" (with
	// the echo's trailing newline) avoids colliding with a "7ms" elapsed in progress.
	if !strings.Contains(out.String(), "7\n") {
		t.Fatalf("cmdOut missing command stdout:\n%s", out.String())
	}
	if strings.Contains(p, "7\n") {
		t.Fatalf("progress leaked command output:\n%s", p)
	}
}

func TestRunProgressMarksFailure(t *testing.T) {
	var prog bytes.Buffer
	_, err := NewRunner(t.TempDir(), time.Minute, &prog, io.Discard).Run([]string{"false"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(prog.String(), "✗ false (exit 1,") {
		t.Fatalf("progress missing failure mark:\n%s", prog.String())
	}
}
