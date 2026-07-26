package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/scaffold"
)

// newAgentCmd builds `mtt agent` — the onboarding group for agent-facing
// artifacts (t52: `hooks`; t46 later adds `docs`).
func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Scaffold agent-facing configuration for coding-agent harnesses",
	}
	cmd.AddCommand(newAgentHooksCmd())
	return cmd
}

// newAgentHooksCmd builds `mtt agent hooks` — scaffold settings + hooks
// (SessionStart/PreCompact -> mtt prime) + a read-only permissions allowlist for
// every registered harness (currently Claude Code). Idempotent; re-runnable.
func newAgentHooksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hooks",
		Short: "Scaffold agent settings + hooks (SessionStart/PreCompact -> mtt prime) for registered harnesses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			results, err := scaffold.Run(root, scaffold.Registry())
			if err != nil {
				return err
			}
			return reportScaffold(cmd, results)
		},
	}
}

// scaffoldJSON is the machine-readable per-harness scaffold result.
type scaffoldJSON struct {
	Harness string `json:"harness"`
	Path    string `json:"path"`
	Action  string `json:"action"`
}

// toScaffoldJSON maps results to the non-null JSON view (empty slice, not null).
func toScaffoldJSON(results []scaffold.Result) []scaffoldJSON {
	out := make([]scaffoldJSON, 0, len(results))
	for _, r := range results {
		out = append(out, scaffoldJSON{Harness: r.Harness, Path: r.Path, Action: string(r.Action)})
	}
	return out
}

// reportScaffold renders the results (shared by `agent hooks` and `init`).
// Human: one line per harness + a single note when anything changed. JSON: the
// non-null result array.
func reportScaffold(cmd *cobra.Command, results []scaffold.Result) error {
	if jsonFlag(cmd) {
		return writeJSON(cmd.OutOrStdout(), toScaffoldJSON(results))
	}
	changed := false
	for _, r := range results {
		if r.Action != scaffold.Unchanged {
			changed = true
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s %s\n", r.Harness, r.Action, r.Path); err != nil {
			return err
		}
	}
	if changed {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(),
			"→ a SessionStart/PreCompact hook now runs `mtt prime` at session start; review the settings file.")
	}
	return nil
}
