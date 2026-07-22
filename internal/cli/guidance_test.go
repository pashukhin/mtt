package cli

import (
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestFormatNextMoves(t *testing.T) {
	got := formatNextMoves([]mtt.Transition{
		{To: "done", Description: "quality gate"},
		{To: "cancelled"},
	}, "t1", "task")
	want := "done (quality gate) · cancelled"
	if got != want {
		t.Fatalf("formatNextMoves = %q, want %q", got, want)
	}
	if got := formatNextMoves(nil, "t1", "task"); got != "" {
		t.Fatalf("formatNextMoves(nil) = %q, want empty", got)
	}
}
