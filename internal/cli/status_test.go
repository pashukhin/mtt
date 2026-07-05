package cli

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/core"
)

func TestResolveRoleByPrecedence(t *testing.T) {
	newCmd := func() *cobra.Command {
		c := &cobra.Command{}
		c.Flags().String("role", "", "")
		c.Flags().String("by", "", "")
		return c
	}

	// flag beats env beats author default
	t.Setenv("MTT_BY", "envuser")
	c := newCmd()
	_ = c.Flags().Set("by", "flaguser")
	if _, by := resolveRoleBy(c, "author"); by != "flaguser" {
		t.Fatalf("by = %q, want flaguser (flag wins)", by)
	}

	// env beats author default
	c = newCmd()
	if _, by := resolveRoleBy(c, "author"); by != "envuser" {
		t.Fatalf("by = %q, want envuser (env over author)", by)
	}

	// author default when flag+env empty
	t.Setenv("MTT_BY", "")
	c = newCmd()
	if _, by := resolveRoleBy(c, "author"); by != "author" {
		t.Fatalf("by = %q, want author (config.local fallback)", by)
	}
}

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
