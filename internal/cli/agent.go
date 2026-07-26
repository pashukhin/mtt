package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pashukhin/mtt/internal/scaffold"
)

// newAgentCmd builds `mtt agent` — the onboarding group for agent-facing
// artifacts: `hooks` (config; t52) and `docs` (how-to-use-mtt runbook; t46).
func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Scaffold agent-facing configuration and docs for coding agents",
	}
	cmd.AddCommand(newAgentHooksCmd(), newAgentDocsCmd())
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
			results, err := scaffold.Run(root, scaffold.HookTargets())
			if err != nil {
				return err
			}
			return reportScaffold(cmd, results, hookScaffoldNote)
		},
	}
}

// scaffoldJSON is the machine-readable per-target scaffold result.
type scaffoldJSON struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Action string `json:"action"`
}

// toScaffoldJSON maps results to the non-null JSON view (empty slice, not null).
func toScaffoldJSON(results []scaffold.Result) []scaffoldJSON {
	out := make([]scaffoldJSON, 0, len(results))
	for _, r := range results {
		out = append(out, scaffoldJSON{Name: r.Name, Path: r.Path, Action: string(r.Action)})
	}
	return out
}

// reportScaffold renders results (shared by `agent hooks`, `agent docs`, `init`).
// JSON: the non-null array. Human: one line per target + `note` on stderr when
// anything changed (note "" suppresses it).
func reportScaffold(cmd *cobra.Command, results []scaffold.Result, note string) error {
	if jsonFlag(cmd) {
		return writeJSON(cmd.OutOrStdout(), toScaffoldJSON(results))
	}
	changed := false
	for _, r := range results {
		if r.Action != scaffold.Unchanged {
			changed = true
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s %s\n", r.Name, r.Action, r.Path); err != nil {
			return err
		}
	}
	if changed && note != "" {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), note)
	}
	return nil
}

// hookScaffoldNote is printed by `agent hooks` / `init` when a hook file changed.
const hookScaffoldNote = "→ a SessionStart/PreCompact hook now runs `mtt prime` at session start; review the settings file."

// docScaffoldNote is printed by `agent docs` / `init` when a doc file changed.
const docScaffoldNote = "→ a `Working under mtt` runbook was scaffolded into AGENTS.md (+ CLAUDE.md/GEMINI.md pointers); review it."

// newAgentDocsCmd builds `mtt agent docs` — scaffold a project-agnostic "Working
// under mtt" runbook (AGENTS.md) + pointer stubs (CLAUDE.md, GEMINI.md).
// Idempotent; re-runnable.
func newAgentDocsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "docs",
		Short: "Scaffold generic how-to-use-mtt agent docs (AGENTS.md runbook + CLAUDE.md/GEMINI.md pointers)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRoot(cmd)
			if err != nil {
				return err
			}
			results, err := scaffold.Run(root, scaffold.DocTargets())
			if err != nil {
				return err
			}
			return reportScaffold(cmd, results, docScaffoldNote)
		},
	}
}
