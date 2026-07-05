package exec

import (
	"testing"
	"time"
)

func TestRunAllPass(t *testing.T) {
	checks, err := NewRunner(t.TempDir(), time.Minute).Run([]string{"true", "true"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(checks) != 2 || checks[0].Exit != 0 || checks[1].Exit != 0 {
		t.Fatalf("checks = %+v", checks)
	}
}

func TestRunStopsAtFirstNonZero(t *testing.T) {
	checks, err := NewRunner(t.TempDir(), time.Minute).Run([]string{"true", "false", "true"})
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
	_, err := NewRunner(t.TempDir(), time.Millisecond).Run([]string{"sleep 1"})
	if err == nil {
		t.Fatalf("want a timeout error, got nil")
	}
}
