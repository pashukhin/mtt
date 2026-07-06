package cli

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/core"
)

func newAttributionCmd() *cobra.Command {
	c := &cobra.Command{}
	c.Flags().String("role", "", "")
	c.Flags().String("by", "", "")
	c.Flags().String("who", "", "")
	c.Flags().String("why", "", "")
	return c
}

func TestResolveAttributionPrecedence(t *testing.T) {
	// flag beats env beats author default
	t.Setenv("MTT_BY", "envuser")
	c := newAttributionCmd()
	_ = c.Flags().Set("by", "flaguser")
	if _, by, _, err := resolveAttribution(c, "author"); err != nil || by != "flaguser" {
		t.Fatalf("by = %q, err = %v, want flaguser (flag wins)", by, err)
	}

	// env beats author default
	c = newAttributionCmd()
	if _, by, _, err := resolveAttribution(c, "author"); err != nil || by != "envuser" {
		t.Fatalf("by = %q, err = %v, want envuser (env over author)", by, err)
	}

	// author default when flag+env empty
	t.Setenv("MTT_BY", "")
	c = newAttributionCmd()
	if _, by, _, err := resolveAttribution(c, "author"); err != nil || by != "author" {
		t.Fatalf("by = %q, err = %v, want author (config.local fallback)", by, err)
	}
}

func TestResolveAttributionWhoAliasesByAndWhy(t *testing.T) {
	t.Setenv("MTT_BY", "")
	c := newAttributionCmd()
	_ = c.Flags().Set("who", "alice")
	_ = c.Flags().Set("why", "start work")
	_, by, why, err := resolveAttribution(c, "")
	if err != nil || by != "alice" || why != "start work" {
		t.Fatalf("by = %q, why = %q, err = %v; want alice / start work", by, why, err)
	}
}

func TestResolveAttributionWhoByMutuallyExclusive(t *testing.T) {
	c := newAttributionCmd()
	_ = c.Flags().Set("who", "alice")
	_ = c.Flags().Set("by", "bob")
	if _, _, _, err := resolveAttribution(c, ""); err == nil {
		t.Fatal("setting both --who and --by must error")
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
		{core.ErrMissingAttribution, 2},
	}
	for _, c := range cases {
		if got := exitCode(c.err); got != c.want {
			t.Fatalf("exitCode(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}
