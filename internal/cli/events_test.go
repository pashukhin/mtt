package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/core"
)

// TestRenderPostRecoveryEmptyRemaining pins the spec-§7 rendering rule: an
// empty Remaining (e.g. a failed audit append has no user-runnable recovery)
// prints the saved-marker line ONLY — never an empty "finish by hand:" block.
func TestRenderPostRecoveryEmptyRemaining(t *testing.T) {
	cmd := &cobra.Command{}
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)

	renderPostRecovery(cmd, &core.PostActionError{Cause: "append failed"}, mutationSavedLine)
	out := errBuf.String()
	if !strings.Contains(out, mutationSavedLine) {
		t.Fatalf("saved line missing: %q", out)
	}
	if strings.Contains(out, "finish the finalization by hand") {
		t.Fatalf("empty Remaining must omit the recovery list: %q", out)
	}

	errBuf.Reset()
	renderPostRecovery(cmd, &core.PostActionError{Cause: "x", Remaining: []string{"git push"}}, mutationSavedLine)
	out = errBuf.String()
	if !strings.Contains(out, "finish the finalization by hand:") || !strings.Contains(out, "  git push") {
		t.Fatalf("remaining not rendered: %q", out)
	}

	errBuf.Reset()
	renderPostRecovery(cmd, nil, mutationSavedLine)
	if errBuf.Len() != 0 {
		t.Fatalf("nil error must render nothing: %q", errBuf.String())
	}
}
