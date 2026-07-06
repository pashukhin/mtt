package exec

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestRunAllPass(t *testing.T) {
	checks, err := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard).Run([]mtt.Command{{Run: "true"}, {Run: "true"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(checks) != 2 || checks[0].Exit != 0 || checks[1].Exit != 0 {
		t.Fatalf("checks = %+v", checks)
	}
}

func TestRunStopsAtFirstNonZero(t *testing.T) {
	checks, err := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard).Run([]mtt.Command{{Run: "true"}, {Run: "false"}, {Run: "true"}})
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
	_, err := NewRunner(t.TempDir(), time.Millisecond, io.Discard, io.Discard).Run([]mtt.Command{{Run: "sleep 1"}})
	if err == nil {
		t.Fatalf("want a timeout error, got nil")
	}
}

func TestRunStreamsProgressAndSeparatesOutput(t *testing.T) {
	// The command text ("echo $((3+4))") deliberately does not contain its output
	// ("7"), so we can assert the two streams stay separate.
	var prog, out bytes.Buffer
	checks, err := NewRunner(t.TempDir(), time.Minute, &prog, &out).Run([]mtt.Command{{Run: "echo $((3+4))"}, {Run: "true"}})
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

func TestRunPerCommandTimeoutOverridesGlobal(t *testing.T) {
	// Global is generous; a tight per-command timeout must fire first.
	_, err := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard).
		Run([]mtt.Command{{Run: "sleep 1", Timeout: 20 * time.Millisecond}})
	if err == nil {
		t.Fatal("want a per-command timeout error, got nil")
	}
}

func TestRunFallsBackToGlobalTimeout(t *testing.T) {
	// No per-command timeout -> the (tight) global applies and fires.
	_, err := NewRunner(t.TempDir(), 20*time.Millisecond, io.Discard, io.Discard).
		Run([]mtt.Command{{Run: "sleep 1"}})
	if err == nil {
		t.Fatal("want a global timeout error, got nil")
	}
}

func TestRunProgressMarksFailure(t *testing.T) {
	var prog bytes.Buffer
	_, err := NewRunner(t.TempDir(), time.Minute, &prog, io.Discard).Run([]mtt.Command{{Run: "false"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(prog.String(), "✗ false (exit 1,") {
		t.Fatalf("progress missing failure mark:\n%s", prog.String())
	}
}

func TestCompensateBestEffortRunsAll(t *testing.T) {
	var prog bytes.Buffer
	// The middle compensator fails (exit 1); best-effort must still run the last.
	checks := NewRunner(t.TempDir(), time.Minute, &prog, io.Discard).
		Compensate([]mtt.Command{{Run: "true"}, {Run: "false"}, {Run: "true"}})
	if len(checks) != 3 {
		t.Fatalf("ran %d compensators, want all 3 (best-effort)", len(checks))
	}
	if checks[0].Exit != 0 || checks[1].Exit == 0 || checks[2].Exit != 0 {
		t.Fatalf("checks = %+v", checks)
	}
	if !strings.Contains(prog.String(), "↩ compensating (3 commands)") {
		t.Fatalf("progress missing the compensation header:\n%s", prog.String())
	}
}

func TestCompensateEmptyIsNoOp(t *testing.T) {
	var prog bytes.Buffer
	if checks := NewRunner(t.TempDir(), time.Minute, &prog, io.Discard).Compensate(nil); checks != nil {
		t.Fatalf("checks = %+v, want nil", checks)
	}
	if prog.Len() != 0 {
		t.Fatalf("empty compensation should print nothing:\n%s", prog.String())
	}
}

func TestCompensateHonorsPerCommandTimeout(t *testing.T) {
	// A tight per-command timeout on a compensator fires; best-effort records -1.
	checks := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard).
		Compensate([]mtt.Command{{Run: "sleep 1", Timeout: 20 * time.Millisecond}})
	if len(checks) != 1 || checks[0].Exit != -1 {
		t.Fatalf("checks = %+v, want a single -1 (timed-out) check", checks)
	}
}
