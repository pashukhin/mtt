package cli

import (
	"testing"

	"github.com/pashukhin/mtt/pkg/mtt"
)

func TestFormatNextMoves(t *testing.T) {
	got := formatNextMoves([]mtt.Transition{
		{To: "done", Description: "quality gate"},
		{To: "cancelled"},
	})
	want := "done (quality gate) · cancelled"
	if got != want {
		t.Fatalf("formatNextMoves = %q, want %q", got, want)
	}
	if got := formatNextMoves(nil); got != "" {
		t.Fatalf("formatNextMoves(nil) = %q, want empty", got)
	}
}
