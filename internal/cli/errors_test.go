package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pashukhin/mtt/internal/core"
)

// (No pkg/mtt import: taskNotFound takes an untyped string constant, so mtt.TaskID
// is never named here — importing pkg/mtt would be an unused-import compile error.)

func TestExitHint(t *testing.T) {
	attrib := fmt.Errorf("%w: who, why", core.ErrMissingAttribution)
	if h := exitHint(attrib); !strings.Contains(h, "MTT_BY") || !strings.Contains(h, "--why") {
		t.Fatalf("attribution hint missing who/why setup: %q", h)
	}
	if h := exitHint(taskNotFound("t9")); !strings.Contains(h, "mtt roadmap") {
		t.Fatalf("not-found hint should point at discovery: %q", h)
	}
	// No bleed: a post-action error and an invalid-transition are handled with context.
	if h := exitHint(&core.PostActionError{Cause: "x"}); h != "" {
		t.Fatalf("PostActionError must get no generic hint, got %q", h)
	}
	if h := exitHint(core.ErrInvalidTransition); h != "" {
		t.Fatalf("invalid-transition must get no generic hint, got %q", h)
	}
	if h := exitHint(errors.New("boom")); h != "" {
		t.Fatalf("unrelated error must get no hint, got %q", h)
	}
}
