package cli

import (
	"errors"
	"testing"

	"github.com/pashukhin/mtt/internal/core"
)

func TestExitCode(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{nil, 0},
		{errors.New("boom"), 1},
		{core.ErrBlocked, 3},
		{errors.New("wrap: " + core.ErrInvalidTransition.Error()), 1}, // plain string does not match
		{core.ErrInvalidTransition, 6},
	}
	for _, c := range cases {
		if got := exitCode(c.err); got != c.want {
			t.Fatalf("exitCode(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}
