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
	checks, err := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard, 0).Run([]mtt.Command{{Run: "true"}, {Run: "true"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(checks) != 2 || checks[0].Exit != 0 || checks[1].Exit != 0 {
		t.Fatalf("checks = %+v", checks)
	}
}

func TestRunStopsAtFirstNonZero(t *testing.T) {
	checks, err := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard, 0).Run([]mtt.Command{{Run: "true"}, {Run: "false"}, {Run: "true"}})
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
	_, err := NewRunner(t.TempDir(), time.Millisecond, io.Discard, io.Discard, 0).Run([]mtt.Command{{Run: "sleep 1"}})
	if err == nil {
		t.Fatalf("want a timeout error, got nil")
	}
}

func TestRunOperationalFailureRecordsFailingCheckLast(t *testing.T) {
	// CONTRACT (core.Runner): on an operational failure, Run returns the checks so
	// far with the FAILING command's Check as the LAST element (Exit -1). core's
	// compensation index (len(checks)-1) relies on it — so assert it here.
	checks, err := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard, 0).
		Run([]mtt.Command{{Run: "true"}, {Run: "sleep 1", Timeout: 20 * time.Millisecond}})
	if err == nil {
		t.Fatal("want an operational (timeout) error")
	}
	if len(checks) != 2 {
		t.Fatalf("checks = %+v, want the succeeded command + the failing one", checks)
	}
	if checks[0].Exit != 0 {
		t.Fatalf("first check = %+v, want the succeeded command (exit 0)", checks[0])
	}
	if last := checks[len(checks)-1]; last.Cmd != "sleep 1" || last.Exit != -1 {
		t.Fatalf("last check = %+v, want {Cmd: sleep 1, Exit: -1}", last)
	}
}

func TestRunStreamsProgressAndSeparatesOutput(t *testing.T) {
	// The command text ("echo $((3+4))") deliberately does not contain its output
	// ("7"), so we can assert the two streams stay separate.
	var prog, out bytes.Buffer
	checks, err := NewRunner(t.TempDir(), time.Minute, &prog, &out, 0).Run([]mtt.Command{{Run: "echo $((3+4))"}, {Run: "true"}})
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
	_, err := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard, 0).
		Run([]mtt.Command{{Run: "sleep 1", Timeout: 20 * time.Millisecond}})
	if err == nil {
		t.Fatal("want a per-command timeout error, got nil")
	}
}

func TestRunFallsBackToGlobalTimeout(t *testing.T) {
	// No per-command timeout -> the (tight) global applies and fires.
	_, err := NewRunner(t.TempDir(), 20*time.Millisecond, io.Discard, io.Discard, 0).
		Run([]mtt.Command{{Run: "sleep 1"}})
	if err == nil {
		t.Fatal("want a global timeout error, got nil")
	}
}

func TestRunProgressMarksFailure(t *testing.T) {
	var prog bytes.Buffer
	_, err := NewRunner(t.TempDir(), time.Minute, &prog, io.Discard, 0).Run([]mtt.Command{{Run: "false"}})
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
	checks := NewRunner(t.TempDir(), time.Minute, &prog, io.Discard, 0).
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
	if checks := NewRunner(t.TempDir(), time.Minute, &prog, io.Discard, 0).Compensate(nil); checks != nil {
		t.Fatalf("checks = %+v, want nil", checks)
	}
	if prog.Len() != 0 {
		t.Fatalf("empty compensation should print nothing:\n%s", prog.String())
	}
}

func TestCompensateHonorsPerCommandTimeout(t *testing.T) {
	// A tight per-command timeout on a compensator fires; best-effort records -1.
	checks := NewRunner(t.TempDir(), time.Minute, io.Discard, io.Discard, 0).
		Compensate([]mtt.Command{{Run: "sleep 1", Timeout: 20 * time.Millisecond}})
	if len(checks) != 1 || checks[0].Exit != -1 {
		t.Fatalf("checks = %+v, want a single -1 (timed-out) check", checks)
	}
}

func TestTailBufferKeepsLastLines(t *testing.T) {
	b := &tailBuffer{max: 2}
	_, _ = io.WriteString(b, "one\ntwo\nthree\n")
	if got := b.lines(); len(got) != 2 || got[0] != "two" || got[1] != "three" {
		t.Fatalf("lines = %v, want [two three]", got)
	}
}

func TestTailBufferKeepsTrailingPartialLine(t *testing.T) {
	b := &tailBuffer{max: 3}
	_, _ = io.WriteString(b, "a\nb\nno-newline-tail")
	got := b.lines()
	if len(got) == 0 || got[len(got)-1] != "no-newline-tail" {
		t.Fatalf("lines = %v, want the trailing partial line included", got)
	}
}

func TestRunEchoesFailingCommandTail(t *testing.T) {
	var prog bytes.Buffer
	// Output is hidden (cmdOut=Discard) but tailLines=10 -> the failing command's
	// OUTPUT (169, absent from the command text) is echoed under the ✗ line.
	_, err := NewRunner(t.TempDir(), time.Minute, &prog, io.Discard, 10).
		Run([]mtt.Command{{Run: "echo $((13*13)); exit 1"}})
	if err != nil {
		t.Fatalf("Run: %v (non-zero is data)", err)
	}
	if !strings.Contains(prog.String(), "169") {
		t.Fatalf("progress must echo the failing command's output tail:\n%s", prog.String())
	}
}

func TestRunDoesNotEchoTailOnSuccessOrWhenDisabled(t *testing.T) {
	var prog bytes.Buffer
	// A succeeding command never echoes its output (hidden-by-default holds).
	// 121 = 11*11 appears only in output, so its absence proves no echo.
	_, _ = NewRunner(t.TempDir(), time.Minute, &prog, io.Discard, 10).
		Run([]mtt.Command{{Run: "echo $((11*11))"}})
	if strings.Contains(prog.String(), "121") {
		t.Fatalf("a succeeding command must not echo output:\n%s", prog.String())
	}
	// tailLines=0 disables the tail even on failure (484 = 22*22, output-only).
	prog.Reset()
	_, _ = NewRunner(t.TempDir(), time.Minute, &prog, io.Discard, 0).
		Run([]mtt.Command{{Run: "echo $((22*22)); exit 1"}})
	if strings.Contains(prog.String(), "484") {
		t.Fatalf("tailLines=0 must disable the tail:\n%s", prog.String())
	}
}
